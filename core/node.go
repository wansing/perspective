package core

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/wansing/perspective/auth"
	"github.com/wansing/perspective/upload"
)

var ErrUnauthorized = errors.New("unauthorized")

type DBVersionStub interface {
	VersionNo() int // ascending
	VersionNote() string
	TsChanged() int64
	WorkflowGroupId() int // one from the workflow, or zero if the version finished its workflow and is visible to all who have the right to read it
}

type DBNode interface {

	// node
	Id() int
	ParentId() int
	Slug() string
	ClassName() string
	TsCreated() int64
	MaxVersionNo() int
	MaxWGZeroVersionNo() int // The number of the latest version with workflow_group == 0, else zero. Redundant, but helpful.

	// version
	DBVersionStub
	Content() string
	Versions() ([]DBVersionStub, error)

	CountChildren() (int, error)
	CountReleasedChildren(minTsCreated int64) (int, error)
	GetChildren(order Order, limit, offset int) ([]DBNode, error)
}

type NodeDB interface {
	AddVersion(n DBNode, content, versionNote string) error
	DeleteNode(n DBNode) error
	GetParentAndSlug(id int) (parentId int, slug string, err error)
	GetLatestNode(parentId int, slug string) (DBNode, error)                 // latest version, error if node does not exist, empty version if no version exists
	GetReleasedNode(parentId int, slug string) (DBNode, error)               // latest released version or error
	GetReleasedNodeById(id int) (DBNode, error)                              // latest released version or error
	GetVersionNode(parentId int, slug string, versionNo int) (DBNode, error) // specific version or error
	InsertNode(parentId int, slug string, class string) error
	IsNotFound(err error) bool
	SetClass(n DBNode, className string) error
	SetParent(n DBNode, parent DBNode) error
	SetSlug(n DBNode, slug string) error
	SetWorkflowGroup(n DBNode, groupId int) error // sets workflow group id of the current version
}

type Node struct {
	DBNode
	Instance
	Class      *Class
	db         *CoreDB
	Parent     *Node // parent in node hierarchy, required for permission checking, nil if node is root
	Prev       *Node // predecessor in route, nil if node is root (or included, which makes it the root of a route)
	Next       *Node // successor in route, nil if node is leaf
	pushed     bool  // whether the slug of this node had been pushed to the queue
	localVars  map[string]string
	tags       []string
	timestamps []int64

	overrideContent bool
	content         string
}

func (db *CoreDB) newNode(dbNode DBNode) (*Node, error) {

	var node = &Node{}
	node.db = db
	node.DBNode = dbNode
	node.localVars = make(map[string]string)

	var ok bool
	node.Class, ok = db.ClassRegistry.Get(dbNode.ClassName())
	if !ok {
		return nil, fmt.Errorf("class %s not found", dbNode.ClassName())
	}

	node.Instance = node.Class.Create()
	node.Instance.SetNode(node) // connect them the other way round

	return node, nil
}

// Content shadows Node.DBNode.Content.
func (n *Node) Content() string {
	if n.overrideContent {
		return n.content
	} else {
		return n.DBNode.Content()
	}
}

func (n *Node) SetContent(newContent string) {
	n.content = newContent
	n.overrideContent = true
}

// Leaf returns the last node of the relation which is created by Node.Next.
//
// It won't work properly when called before Recurse.
func (n *Node) Leaf() *Node {
	if n != nil {
		for n.Next != nil {
			n = n.Next
		}
	}
	return n
}

// Root returns the last node of the relation which is created by Node.Prev.
//
// It returns the root of a route, not the root of the tree.
func (n *Node) Root() *Node {
	if n != nil {
		for n.Prev != nil {
			n = n.Prev
		}
	}
	return n
}

func (n *Node) isInMainRoute() bool {
	return n.Root().Id() == 1 // id of root node, hardcoded
}

// GetAssignedRules returns all access rules which are assigned with the receiver node.
func (n *Node) GetAssignedRules() (map[auth.Group]Permission, error) {
	var rawRules, err = n.db.GetAccessRules(n.Id())
	if err != nil {
		return nil, err
	}
	var rules = make(map[auth.Group]Permission)
	for groupId, permInt := range rawRules {
		var group, err = n.db.Auth.GetGroup(groupId)
		if err != nil {
			return nil, err
		}
		var perm = Permission(permInt)
		if !perm.Valid() {
			return nil, errors.New("invalid permission value")
		}
		rules[group] = perm
	}
	return rules, nil
}

// Folder returns the upload.Folder which stores uploaded files for the node.
func (n *Node) Folder() upload.Folder {
	return n.db.Uploads.Folder(n.Id())
}

func (n *Node) WorkflowGroup() (auth.Group, error) {
	return n.db.Auth.GetGroup(n.WorkflowGroupId())
}

func (n *Node) IsPushed() bool {
	return n.pushed
}

func (n *Node) HMAC(nodeId int, filename string, w int, h int, ts int64) string {
	return n.db.Uploads.HMAC(nodeId, filename, w, h, ts)
}

func (n *Node) ParseUploadUrl(u *url.URL) (isUpload bool, uploadLocation upload.Folder, filename string, resize bool, w, h int, ts int64, sig []byte, err error) {
	return upload.ParseUrl(n.db.Uploads, n.db.Uploads.Folder(n.Id()), u)
}

func (n *Node) String() string {
	return n.Slug()
}

// AddChild adds a child node to the receiver node.
// It does not care for duplicated slugs, the database must prevent them.
func (c *CoreDB) AddChild(n *Node, slug, className string) error {
	if _, ok := c.ClassRegistry.Get(className); !ok {
		return fmt.Errorf("class %s not found", className)
	}
	return c.InsertNode(n.DBNode.Id(), slug, className)
}

// DeleteNode shadows CoreDB.NodeDB.DeleteNode.
func (c *CoreDB) DeleteNode(n *Node) error {
	if n.ParentId() == 0 {
		return errors.New("can't delete root node")
	}
	return c.NodeDB.DeleteNode(n)
}

// Edit adds a version to the receiver node.
func (c *CoreDB) Edit(n *Node, newContent, newVersionNote, username string) error {
	if n.Content() != newContent {
		if err := c.AddVersion(n.DBNode, newContent, fmt.Sprintf("[%s] %s", username, strings.TrimSpace(newVersionNote))); err != nil {
			return err
		}
	}
	return nil
}

// SetClass shadows CoreDB.NodeDB.SetClass.
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

// SetParent shadows CoreDB.NodeDB.SetParent.
func (c *CoreDB) SetParent(n *Node, newParent *Node) error {

	if n.Parent == nil {
		return errors.New("can't move root node")
	}

	// newParent can't be below this
	for newAncestor := newParent; newAncestor != nil; newAncestor = newAncestor.Parent {
		if newAncestor.Id() == n.Id() {
			return errors.New("can't move node below itself")
		}
	}

	// skip if new parent is current parent
	if newParent.Id() == n.Parent.Id() {
		return nil
	}

	if err := c.NodeDB.SetParent(n, newParent); err != nil {
		return err
	}

	n.Parent = newParent
	return nil
}

// SetSlug shadows CoreDB.NodeDB.SetSlug.
// It does not care for duplicated slugs, the database must prevent them.
func (c *CoreDB) SetSlug(n *Node, slug string) error {
	slug = NormalizeSlug(slug)
	if slug == "" {
		return errors.New("slug can't be empty")
	}
	return c.NodeDB.SetSlug(n, slug)
}

// SetWorkflowGroup shadows CoreDB.NodeDB.SetWorkflowGroup.
func (c *CoreDB) SetWorkflowGroup(n *Node, newWorkflowGroup int) error {

	if n.WorkflowGroupId() == newWorkflowGroup {
		return nil
	}

	var oldMaxWGZeroVersionNo = n.MaxWGZeroVersionNo()

	if err := c.NodeDB.SetWorkflowGroup(n.DBNode, newWorkflowGroup); err != nil {
		return err
	}

	if oldMaxWGZeroVersionNo != n.MaxWGZeroVersionNo() { // if maxWGZeroVersionNo has changed

		var parentId = 0
		if n.Parent != nil {
			parentId = n.Parent.Id()
		}

		var err error
		n, err = c.GetReleasedNode(parentId, n.Slug())
		if err != nil {
			return err
		}

		var tmpRoute = newDummyRoute()
		tmpRoute.current = n
		if err := tmpRoute.parseAndExecuteTemplates(); err != nil {
			return err
		}

		if err := n.ClearIndex(); err != nil {
			return err
		}

		if err := n.AddTags(n.tags); err != nil {
			return err
		}

		if err := n.AddTimestamps(n.timestamps); err != nil {
			return err
		}
	}

	return nil
}
