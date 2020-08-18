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
	"github.com/wansing/perspective/auth"
	"github.com/wansing/perspective/filestore"
	"github.com/wansing/perspective/upload"
	"github.com/wansing/perspective/util"
)

type CoreDB struct {
	AccessDB
	ClassRegistry
	EditorsDB
	NodeDB
	IndexDB

	Auth           auth.AuthDB
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
		CacheDir:    "./cache",
		UploadDir:   "./uploads",
		HMACSecret:  []byte(c.HMACSecret),
		Resizer:     resizer,
	}

	return nil
}

// AddAccessRule calls CoreDB.AccessDB.InsertAccessRule.
func (c *CoreDB) AddAccessRule(e *Node, groupId int, perm Permission) error {
	var group, err = c.Auth.GetGroup(groupId)
	if err != nil {
		return err
	}
	return c.AccessDB.InsertAccessRule(e.Id(), group.Id(), int(perm))
}

// RemoveAccessRule shadows CoreDB.AccessDB.RemoveAccessRule.
func (c *CoreDB) RemoveAccessRule(e *Node, groupId int) error {
	// not checking if the group exists because not a lot can go wrong
	return c.AccessDB.RemoveAccessRule(e.Id(), groupId)
}

// GetAllWorkflowAssignments shadows CoreDB.EditorsDB.GetAllWorkflowAssignments.
func (c *CoreDB) GetAllWorkflowAssignments() (map[int]map[bool]*auth.Workflow, error) {
	var base, err = c.EditorsDB.GetAllWorkflowAssignments()
	if err != nil {
		return nil, err
	}
	var all = make(map[int]map[bool]*auth.Workflow)
	for nodeId, entry := range base {
		if _, ok := all[nodeId]; !ok {
			all[nodeId] = make(map[bool]*auth.Workflow)
		}
		for childrenOnly, workflowId := range entry {
			workflow, err := c.Auth.GetWorkflow(workflowId)
			if err != nil {
				return nil, err
			}
			all[nodeId][childrenOnly] = workflow
		}
	}
	return all, nil
}

// GetLatestNode shadows CoreDB.NodeDB.GetLatestNode.
func (c *CoreDB) GetLatestNode(parentId int, slug string) (*Node, error) {
	node, err := c.NodeDB.GetLatestNode(parentId, slug)
	if err != nil {
		return nil, err
	}
	return c.newNode(node)
}

// GetReleasedNode shadows CoreDB.NodeDB.GetReleasedNode.
func (c *CoreDB) GetReleasedNode(parentId int, slug string) (*Node, error) {
	node, err := c.NodeDB.GetReleasedNode(parentId, slug)
	if err != nil {
		return nil, err
	}
	return c.newNode(node)
}

// GetVersionNode shadows CoreDB.NodeDB.GetVersionNode.
func (c *CoreDB) GetVersionNode(parentId int, slug string, versionNo int) (*Node, error) {
	node, err := c.NodeDB.GetVersionNode(parentId, slug, versionNo)
	if err != nil {
		return nil, err
	}
	return c.newNode(node)
}

// InternalUrlByNodeId determines the internal path of the node with the given id.
func (c *CoreDB) InternalPathByNodeId(id int) (string, error) {
	return c.internalPathByNodeId(id, 16)
}

func (c *CoreDB) internalPathByNodeId(id int, maxDepth int) (string, error) {
	var slugs = []string{}
	for {
		if maxDepth--; maxDepth < 0 {
			return "", errors.New("too deep")
		}
		if id == 1 { // root
			break
		}
		parentId, slug, err := c.GetParentAndSlug(id)
		if err != nil {
			return "", err
		}
		slugs = append([]string{slug}, slugs...)
		id = parentId
	}

	return "/" + strings.Join(slugs, "/"), nil
}

// requirePermissionById checks if a node with a given id has a rule which gives permission to the user.
// If permittingRules is not nil, then it is populated.
func (c *CoreDB) requirePermissionById(required Permission, nodeId int, u auth.User, permittingRules *map[int]map[int]interface{}) error {

	if u == nil && required > Read {
		return ErrUnauthorized
	}

	// don't check for Edit because that is not a Permission

	groups, err := c.Auth.GetGroupsOf(u)
	if err != nil {
		return err
	}
	groups = append(groups, auth.AllUsers{})

	nodeRules, err := c.GetAccessRules(nodeId)
	if err != nil {
		return err
	}

	for _, group := range groups {
		if myPermission, ok := nodeRules[group.Id()]; ok {
			var myPerm = Permission(myPermission)
			if !myPerm.Valid() {
				return errors.New("invalid permission")
			}
			if myPerm >= required {
				if permittingRules != nil {
					if (*permittingRules)[nodeId] == nil {
						(*permittingRules)[nodeId] = make(map[int]interface{})
					}
					(*permittingRules)[nodeId][group.Id()] = struct{}{}
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
