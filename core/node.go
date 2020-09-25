package core

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/wansing/perspective/upload"
)

var ErrUnauthorized = errors.New("unauthorized")

type DBNode interface {
	Id() int
	ParentId() int
	Slug() string
	ClassName() string
	TsCreated() int64
	MaxVersionNo() int
	MaxWGZeroVersionNo() int // The number of the latest version with workflow_group == 0, else zero. Redundant, but helpful.
}

type DBNodeVersion interface {
	DBNode
	DBVersion
}

type NodeDB interface {
	AddVersion(n DBNode, content, versionNote string, workflowGroupId int) error
	CountChildren(id int) (int, error)
	CountReleasedChildren(id int) (int, error)
	DeleteNode(n DBNode) error
	GetChildren(id int, order Order, limit, offset int) ([]DBNodeVersion, error) // version part can be empty, exists just because it makes caching easier
	GetNodeById(id int) (DBNode, error)
	GetNodeBySlug(parentId int, slug string) (DBNode, error)
	GetReleasedChildren(id int, order Order, limit, offset int) ([]DBNodeVersion, error)
	GetVersion(id int, versionNo int) (DBVersion, error)
	InsertNode(parentId int, slug string, class string) error
	IsNotFound(err error) bool
	SetClass(n DBNode, className string) error
	SetParent(n DBNode, parent DBNode) error
	SetSlug(n DBNode, slug string) error
	SetWorkflowGroup(n DBNode, v DBVersionStub, groupId int) error // sets workflow group id of the current version
	Versions(id int) ([]DBVersionStub, error)
}

type NoVersion struct{}

func (NoVersion) Content() string {
	return ""
}

func (NoVersion) TsChanged() int64 {
	return 0
}

func (NoVersion) VersionNo() int {
	return 0
}

func (NoVersion) VersionNote() string {
	return ""
}

func (NoVersion) WorkflowGroupId() int {
	return 0
}

type Node struct {
	DBNode
	Instance
	Class      *Class
	Parent     *Node // parent in node hierarchy, required for permission checking, nil if node is root
	Prev       *Node // predecessor in route, nil if node is root (or included, which makes it the root of a route)
	Next       *Node // successor in route, nil if node is leaf
	Tags       []string
	Timestamps []int64

	db     *CoreDB
	pushed bool // whether the slug of this node had been pushed to the queue
}

// NewNode creates a Node. You must set Prev and Next on your own.
func (c *CoreDB) NewNode(parent *Node, dbNode DBNode) *Node {

	var n = &Node{}
	n.db = c
	n.DBNode = dbNode
	n.Parent = parent

	var ok bool
	n.Class, ok = c.ClassRegistry.Get(dbNode.ClassName())
	if !ok {
		n.Class = &Class{
			Create: func() Instance {
				return &Base{} // Base won't reveal the content to the viewer
			},
			Name: "unknown",
			Code: dbNode.ClassName(),
		}
	}

	n.Instance = n.Class.Create()

	return n
}

func (n *Node) GetLatestVersion() (DBVersion, error) {
	return n.db.GetVersion(n.Id(), n.MaxVersionNo())
}

func (n *Node) GetReleasedVersion() (DBVersion, error) {
	return n.db.GetVersion(n.Id(), n.MaxWGZeroVersionNo())
}

func (n *Node) GetVersion(versionNo int) (DBVersion, error) {
	v, err := n.db.GetVersion(n.Id(), versionNo)
	if err != nil {
		err = fmt.Errorf("version %d of node %d: %w", versionNo, n.Id(), err)
	}
	return v, err
}

func (n *Node) CountChildren() (int, error) {
	return n.db.CountChildren(n.Id())
}

func (n *Node) CountReleasedChildren() (int, error) {
	return n.db.CountReleasedChildren(n.Id())
}

func (n *Node) Depth() int {
	var depth = 0
	for n != nil {
		depth++
		n = n.Prev
	}
	return depth
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

// Id shadows DBNode.Id. If the receiver is nil, it returns zero.
func (n *Node) Id() int {
	if n != nil {
		return n.DBNode.Id()
	}
	return 0
}

func (n *Node) GetChildren(user DBUser, order Order, limit, offset int) ([]*Node, error) {
	return n.getChildren(n.db.GetChildren, user, order, limit, offset)
}

func (n *Node) GetReleasedChildren(user DBUser, order Order, limit, offset int) ([]*Node, error) {
	return n.getChildren(n.db.GetReleasedChildren, user, order, limit, offset)
}

func (n *Node) getChildren(f func(id int, order Order, limit, offset int) ([]DBNodeVersion, error), user DBUser, order Order, limit, offset int) ([]*Node, error) {
	var children, err = f(n.Id(), order, limit, offset)
	if err != nil {
		return nil, err
	}
	var result = make([]*Node, 0, len(children))
	for _, c := range children {
		node := n.db.NewNode(n, c)
		if err := node.RequirePermission(Read, user); err != nil {
			continue
		}
		result = append(result, node)
	}
	return result, nil
}

// GetAssignedRules returns all access rules which are assigned with the receiver node.
func (n *Node) GetAssignedRules() (map[DBGroup]Permission, error) {
	var rawRules, err = n.db.GetAccessRules(n.Id())
	if err != nil {
		return nil, err
	}
	var rules = make(map[DBGroup]Permission)
	for groupId, permInt := range rawRules {
		var group, err = n.db.GetGroup(groupId)
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

// GetAssignedWorkflow returns the workflow which is directly assigned to the node, if any.
func (n *Node) GetAssignedWorkflow(childrenOnly bool) (*Workflow, error) {
	var workflowId, err = n.db.GetAssignedWorkflowId(n.Id(), childrenOnly)
	if err != nil {
		return nil, err
	}
	if workflowId == 0 {
		return nil, nil
	}
	return n.db.GetWorkflow(workflowId)
}

// GetWorkflow returns the workflow which applies (but is not necessarily directly assigned) to the node.
func (n *Node) GetWorkflow() (*Workflow, error) {

	// check impression (with childrenOnly == false)

	workflow, err := n.GetAssignedWorkflow(false)
	if err != nil {
		return nil, err
	}
	if workflow != nil {
		return workflow, nil
	}

	if n.Parent != nil {

		// check parent node with childrenOnly == true

		workflow, err = n.Parent.GetAssignedWorkflow(true)
		if err != nil {
			return nil, err
		}
		if workflow != nil {
			return workflow, nil
		}

		// else recurse to parent

		return n.Parent.GetWorkflow()
	}

	return nil, errors.New("no workflow")
}

// Folder returns the upload.Folder which stores uploaded files for the node.
func (n *Node) Folder() upload.Folder {
	return n.db.Uploads.Folder(n.Id())
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

func (n *Node) Versions() ([]DBVersionStub, error) {
	return n.db.Versions(n.Id())
}

// AddChild adds a child node to the receiver node.
// It does not care for duplicated slugs, the database must prevent them.
func (c *CoreDB) AddChild(n *Node, slug, className string) error {
	if _, ok := c.ClassRegistry.Get(className); !ok {
		return fmt.Errorf("class %s not found", className)
	}
	return c.InsertNode(n.DBNode.Id(), slug, className)
}
