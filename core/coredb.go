package core

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/wansing/perspective/filestore"
	"github.com/wansing/perspective/upload"
	"github.com/wansing/perspective/util"
)

type CoreDB struct {
	AccessDB
	ClassRegistry
	EditorsDB
	GroupDB
	IndexDB
	NodeDB
	UserDB
	WorkflowDB
	SessionManager *scs.SessionManager
	Uploads        upload.Store

	HMACSecret string  // exported because main sets it
	SqlDB      *sql.DB // required for some classes
}

func (c *CoreDB) Init(sessionStore scs.Store, cookiePath string) error {

	if c.HMACSecret == "" {
		var err error
		c.HMACSecret, err = util.RandomString32()
		if err == nil {
			log.Println("generating random HMAC secret")
		} else {
			return fmt.Errorf("error generating random HMAC secret: %v")
		}
	}

	c.SessionManager = scs.New()
	c.SessionManager.Store = sessionStore
	c.SessionManager.Cookie.Path = cookiePath + "/"         // 'The default value is "/". Passing the empty string "" will result in it being set to the path that the cookie was issued from.'
	c.SessionManager.Cookie.Persist = false                 // Don't store cookie across browser sessions. Required for GDPR cookie consent exemption criterion B. https://ec.europa.eu/justice/article-29/documentation/opinion-recommendation/files/2012/wp194_en.pdf
	c.SessionManager.Cookie.SameSite = http.SameSiteLaxMode // good CSRF protection if HTTP GET doesn't modify anything
	c.SessionManager.Cookie.Secure = false                  // else running on localhost or behind a http proxy fails
	c.SessionManager.IdleTimeout = 12 * time.Hour
	c.SessionManager.Lifetime = 720 * time.Hour

	resizer, err := filestore.FindResizer()
	if err == nil {
		fmt.Printf("using JPEG resizer: %s\n", resizer.Name())
	} else {
		return err
	}

	c.Uploads = &filestore.Store{
		CacheDir:   "./cache",
		UploadDir:  "./uploads",
		HMACSecret: []byte(c.HMACSecret),
		Resizer:    resizer,
	}

	return nil
}

// AddAccessRule shadows AccessDB.InsertAccessRule.
func (c *CoreDB) AddAccessRule(e *Node, groupID int, perm Permission) error {
	var group, err = c.GroupDB.GetGroup(groupID)
	if err != nil {
		return err
	}
	return c.AccessDB.InsertAccessRule(e.ID(), group.ID(), int(perm))
}

// RemoveAccessRule shadows AccessDB.RemoveAccessRule.
func (c *CoreDB) RemoveAccessRule(e *Node, groupID int) error {
	// not checking if the group exists because not a lot can go wrong
	return c.AccessDB.RemoveAccessRule(e.ID(), groupID)
}

// Edit adds a version to the receiver node.
func (c *CoreDB) Edit(n *Node, v DBVersion, newContent, newVersionNote, username string, workflowGroupID int) error {
	if v.Content() != newContent {
		if err := c.AddVersion(n.DBNode, newContent, fmt.Sprintf("[%s] %s", username, strings.TrimSpace(newVersionNote)), workflowGroupID); err != nil {
			return err
		}
	}
	return nil
}

// GetAllWorkflowAssignments shadows EditorsDB.GetAllWorkflowAssignments.
func (c *CoreDB) GetAllWorkflowAssignments() (map[int]map[bool]*Workflow, error) {
	var base, err = c.EditorsDB.GetAllWorkflowAssignments()
	if err != nil {
		return nil, err
	}
	var all = make(map[int]map[bool]*Workflow)
	for nodeID, entry := range base {
		if _, ok := all[nodeID]; !ok {
			all[nodeID] = make(map[bool]*Workflow)
		}
		for childrenOnly, workflowID := range entry {
			workflow, err := c.GetWorkflow(workflowID)
			if err != nil {
				return nil, err
			}
			all[nodeID][childrenOnly] = workflow
		}
	}
	return all, nil
}

func (c *CoreDB) GetNodeBySlug(parent *Node, slug string) (*Node, error) {
	dbNode, err := c.NodeDB.GetNodeBySlug(parent.ID(), slug)
	if err != nil {
		return nil, err
	}
	return c.NewNode(parent, dbNode), nil
}

// InternalUrlByNodeID determines the internal path of the node with the given id.
func (c *CoreDB) InternalPathByNodeID(id int) (string, error) {
	return c.internalPathByNodeID(id, 16)
}

func (c *CoreDB) internalPathByNodeID(id int, maxDepth int) (string, error) {
	var slugs = []string{}
	for {
		if maxDepth--; maxDepth < 0 {
			return "", errors.New("too deep")
		}
		if id == 1 { // root
			break
		}
		n, err := c.GetNodeByID(id)
		if err != nil {
			return "", err
		}
		slugs = append([]string{n.Slug()}, slugs...)
		id = n.ParentID()
	}

	return "/" + strings.Join(slugs, "/"), nil
}

// requireRule checks if a node with a given id has a rule which gives permission to the user.
// If permittingRules is not nil, then it is populated.
func (c *CoreDB) requireRule(required Permission, nodeID int, u DBUser, permittingRules *map[int]map[int]interface{}) error {

	if u == nil && required > Read {
		return ErrUnauthorized
	}

	// don't check for Edit because that is not a Permission

	var err error
	var groups []DBGroup
	if u != nil {
		groups, err = c.GroupDB.GetGroupsOf(u)
		if err != nil {
			return err
		}
	}
	groups = append(groups, AllUsers{})

	nodeRules, err := c.GetAccessRules(nodeID)
	if err != nil {
		return err
	}

	for _, group := range groups {
		if myPermission, ok := nodeRules[group.ID()]; ok {
			var myPerm = Permission(myPermission)
			if !myPerm.Valid() {
				return errors.New("invalid permission")
			}
			if myPerm >= required {
				if permittingRules != nil {
					if (*permittingRules)[nodeID] == nil {
						(*permittingRules)[nodeID] = make(map[int]interface{})
					}
					(*permittingRules)[nodeID][group.ID()] = struct{}{}
				} else {
					return nil // if permittingRules are not requested, then we're done now
				}
			}
		}
	}

	if permittingRules != nil && len(*permittingRules) >= 1 { // if permittingRules are requested, then at least one is required
		return nil
	}

	return ErrUnauthorized
}

// Open gets a Node from the database. It returns the leaf of the given queue.
// Any version information in the queue is ignored.
func (c *CoreDB) Open(user DBUser, parent *Node, queue *Queue) (*Node, error) {

	if queue.Len() > 16 {
		return nil, errors.New("queue too deep")
	}

	if queue.Len() == 0 {
		return parent, nil // return parent, not nil!
	}

	var parentID = 0
	if parent != nil {
		parentID = parent.ID()
	}

	slug, ok := queue.Pop()
	if !ok {
		return parent, nil // queue is empty
	}

	n, err := c.GetNodeBySlug(parent, slug)
	if err != nil {
		return nil, fmt.Errorf("open (%d, %s): %w", parentID, slug, err) // %w wraps err
	}

	if err := n.RequirePermission(Read, user); err != nil {
		return nil, fmt.Errorf("open (%d, %s): %w", parentID, slug, err)
	}

	return c.Open(user, n, queue)
}

// SetClass shadows NodeDB.SetClass.
func (c *CoreDB) SetClass(n *Node, className string) error {
	className = strings.TrimSpace(className)
	if className == "" {
		return errors.New("class can't be empty")
	}
	if _, ok := c.ClassRegistry.Get(className); !ok {
		return fmt.Errorf("class %s not found", className)
	}
	return c.NodeDB.SetClass(n.DBNode, className)
}

// SetParent shadows NodeDB.SetParent.
func (c *CoreDB) SetParent(n *Node, newParent *Node) error {

	if n.Parent == nil {
		return errors.New("can't move root node")
	}

	// newParent can't be below this
	for newAncestor := newParent; newAncestor != nil; newAncestor = newAncestor.Parent {
		if newAncestor.ID() == n.ID() {
			return errors.New("can't move node below itself")
		}
	}

	// skip if new parent is current parent
	if newParent.ID() == n.Parent.ID() {
		return nil
	}

	if err := c.NodeDB.SetParent(n.DBNode, newParent); err != nil {
		return err
	}

	n.Parent = newParent
	return nil
}

// SetSlug shadows NodeDB.SetSlug.
// It does not care for duplicated slugs, the database must prevent them.
func (c *CoreDB) SetSlug(n *Node, slug string) error {
	slug = NormalizeSlug(slug)
	if slug == "" {
		return errors.New("slug can't be empty")
	}
	return c.NodeDB.SetSlug(n.DBNode, slug)
}

// SetWorkflowGroup shadows NodeDB.SetWorkflowGroup.
func (c *CoreDB) SetWorkflowGroup(n *Node, v *Version, newWorkflowGroup int) error {

	if v.WorkflowGroupID() == newWorkflowGroup {
		return nil
	}

	var oldMaxWGZeroVersionNo = n.MaxWGZeroVersionNo()

	if err := c.NodeDB.SetWorkflowGroup(n.DBNode, v, newWorkflowGroup); err != nil {
		return err
	}

	if oldMaxWGZeroVersionNo != n.MaxWGZeroVersionNo() { // if maxWGZeroVersionNo has changed

		v, err := n.GetVersion(n.MaxWGZeroVersionNo())
		if err != nil {
			return err
		}

		var tmpRequest = newDummyRequest()
		tmpRequest.db = c // required for getNodeBySlug

		var tmpRoute = &Route{
			Request: tmpRequest,
			Queue:   NewQueue(""),
			Node:    n,
			Version: v,
		}

		if err := n.Do(tmpRoute); err != nil {
			return err
		}

		if err := n.SetTags(v.Tags); err != nil {
			return err
		}

		if err := n.SetTimestamps(v.Timestamps); err != nil {
			return err
		}
	}

	return nil
}

// AssignWorkflow shadows EditorsDB.AssignWorkflow.
func (c *CoreDB) AssignWorkflow(n *Node, childrenOnly bool, workflowID int) error {
	return c.EditorsDB.AssignWorkflowID(n.ID(), childrenOnly, workflowID)
}

// UnassignWorkflow shadows EditorsDB.UnassignWorkflow.
func (c *CoreDB) UnassignWorkflow(n *Node, childrenOnly bool) error {
	return c.EditorsDB.UnassignWorkflow(n.ID(), childrenOnly)
}
