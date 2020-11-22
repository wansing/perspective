package core

import (
	"errors"
	"fmt"

	"github.com/wansing/perspective/upload"
)

var ErrUnauthorized = errors.New("unauthorized")

type DBNode interface {
	ID() int
	ParentID() int
	Slug() string
	ClassCode() string
	TsCreated() int64
	MaxVersionNo() int
	MaxWGZeroVersionNo() int // The number of the latest version with workflow_group == 0, else zero. Redundant, but helpful.
}

type DBNodeVersion interface {
	DBNode
	DBVersion
}

type NodeDB interface {
	AddVersion(n DBNode, content, versionNote string, workflowGroupID int) error
	CountChildren(id int) (int, error)
	CountReleasedChildren(id int) (int, error)
	DeleteNode(n DBNode) error
	GetChildren(id int, order Order, limit, offset int) ([]DBNodeVersion, error) // version part can be empty, exists just because it makes caching easier
	GetNodeByID(id int) (DBNode, error)
	GetNodeBySlug(parentID int, slug string) (DBNode, error)
	GetReleasedChildren(id int, order Order, limit, offset int) ([]DBNodeVersion, error)
	GetVersion(id int, versionNo int) (DBVersion, error)
	InsertNode(parentID int, slug string, class string) error
	IsNotFound(err error) bool
	SetClass(n DBNode, classCode string) error
	SetParent(n DBNode, parent DBNode) error
	SetSlug(n DBNode, slug string) error
	SetWorkflowGroup(n DBNode, v DBVersionStub, groupID int) error // sets workflow group id of the current version
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

func (NoVersion) WorkflowGroupID() int {
	return 0
}

type NodeVersion struct {
	*Node
	*Version
}

// Node is independent from Route.
type Node struct {
	DBNode
	Instance
	Parent *Node // parent in node hierarchy or nil, required for permission checking

	db *CoreDB // TODO omit
}

// NewNode creates a Node.
func (c *CoreDB) NewNode(parent *Node, dbNode DBNode) *Node {

	var n = &Node{}
	n.db = c
	n.DBNode = dbNode
	n.Instance = n.Class().Create()
	n.Parent = parent

	return n
}

func (n *Node) Class() *Class {
	class, ok := n.db.ClassRegistry.Get(n.ClassCode())
	if !ok {
		class = &Class{
			Create: func() Instance {
				return &NOP{}
			},
			Name: "Unknown",
			Code: n.ClassCode(),
		}
	}
	return class
}

func (n *Node) GetVersion(versionNo int) (*Version, error) {
	v, err := n.db.GetVersion(n.ID(), versionNo)
	if err != nil {
		return nil, fmt.Errorf("node %s, version %d: %w", n.Location(), versionNo, err)
	}
	return NewVersion(v), nil
}

func (n *Node) CountChildren() (int, error) {
	return n.db.CountChildren(n.ID())
}

func (n *Node) CountReleasedChildren() (int, error) {
	return n.db.CountReleasedChildren(n.ID())
}

func (n *Node) Depth() int {
	var depth = 0
	for n != nil {
		depth++
		n = n.Parent
	}
	return depth
}

// ID shadows DBNode.ID. If the receiver is nil, it returns zero.
func (n *Node) ID() int {
	if n != nil {
		return n.DBNode.ID()
	}
	return 0
}

func (n *Node) GetChildren(user DBUser, order Order, limit, offset int) ([]*Node, error) {
	var children, err = n.db.GetChildren(n.ID(), order, limit, offset)
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

func (n *Node) GetReleasedChildren(user DBUser, order Order, limit, offset int) ([]NodeVersion, error) {
	var children, err = n.db.GetReleasedChildren(n.ID(), order, limit, offset)
	if err != nil {
		return nil, err
	}
	var result = make([]NodeVersion, 0, len(children))
	for _, c := range children {
		node := n.db.NewNode(n, c)
		if err := node.RequirePermission(Read, user); err != nil {
			continue
		}
		version := NewVersion(c)
		result = append(result, NodeVersion{
			Node:    node,
			Version: version,
		})
	}
	return result, nil
}

// GetAssignedRules returns all access rules which are assigned with the receiver node.
func (n *Node) GetAssignedRules() (map[DBGroup]Permission, error) {
	var rawRules, err = n.db.GetAccessRules(n.ID())
	if err != nil {
		return nil, err
	}
	var rules = make(map[DBGroup]Permission)
	for groupID, permInt := range rawRules {
		var group, err = n.db.GetGroup(groupID)
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
	var workflowID, err = n.db.GetAssignedWorkflowID(n.ID(), childrenOnly)
	if err != nil {
		return nil, err
	}
	if workflowID == 0 {
		return nil, nil
	}
	return n.db.GetWorkflow(workflowID)
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
	return n.db.Uploads.Folder(n.ID())
}

func (n *Node) HMAC(nodeID int, filename string, w int, h int, ts int64) string {
	return n.db.Uploads.HMAC(nodeID, filename, w, h, ts)
}

func (n *Node) String() string {
	return n.Slug()
}

func (n *Node) Versions() ([]DBVersionStub, error) {
	return n.db.Versions(n.ID())
}

// AddChild adds a child node to the receiver node.
// It does not care for duplicated slugs, the database must prevent them.
func (c *CoreDB) AddChild(n *Node, slug, classCode string) error {
	if _, ok := c.ClassRegistry.Get(classCode); !ok {
		return fmt.Errorf("class %s not found", classCode)
	}
	return c.InsertNode(n.DBNode.ID(), slug, classCode)
}
