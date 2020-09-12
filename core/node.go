package core

import (
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/wansing/perspective/auth"
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

type DBVersionStub interface {
	TsChanged() int64
	VersionNo() int // ascending
	VersionNote() string
	WorkflowGroupId() int // one from the workflow, or zero if the version finished its workflow and is visible to all who have the right to read it
}

type DBVersion interface {
	DBVersionStub
	Content() string
}

type DBNodeVersionStub interface {
	DBNode
	DBVersionStub
}

type DBNodeVersion interface {
	DBNode
	DBVersion
}

type NodeDB interface {
	AddVersion(n DBNode, content, versionNote string, workflowGroupId int) error
	CountChildren(n DBNode) (int, error)
	CountReleasedChildren(n DBNode, minTsCreated int64) (int, error)
	DeleteNode(n DBNode) error
	GetChildren(n DBNode, order Order, limit, offset int) ([]DBNode, error)
	GetNode(parentId int, slug string) (DBNode, error)
	GetParentAndSlug(id int) (parentId int, slug string, err error)
	GetReleasedChildren(n DBNode, order Order, limit, offset int) ([]DBNodeVersion, error)
	GetReleasedNodeById(id int) (DBNode, DBVersion, error) // latest released version or error
	GetVersion(parent DBNode, versionNo int) (DBVersion, error)
	InsertNode(parentId int, slug string, class string) error
	IsNotFound(err error) bool
	SetClass(n DBNode, className string) error
	SetParent(n DBNode, parent DBNode) error
	SetSlug(n DBNode, slug string) error
	SetWorkflowGroup(n DBNodeVersionStub, groupId int) error // sets workflow group id of the current version
	Versions(n DBNode) ([]DBVersionStub, error)
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
	DBVersion
	Instance
	Class      *Class
	db         *CoreDB
	Parent     *Node // parent in node hierarchy, required for permission checking, nil if node is root
	Prev       *Node // predecessor in route, nil if node is root (or included, which makes it the root of a route)
	Next       *Node // successor in route, nil if node is leaf
	Tags       []string
	Timestamps []int64
	pushed     bool // whether the slug of this node had been pushed to the queue
	localVars  map[string]string

	overrideContent bool
	content         string
}

// NewNode creates a Node. You must set Prev and Next on your own.
func (c *CoreDB) NewNode(parent *Node, dbNode DBNode, dbVersion DBVersion) (*Node, error) {

	var n = &Node{}
	n.db = c
	n.DBNode = dbNode
	n.DBVersion = dbVersion
	n.localVars = make(map[string]string)
	n.Parent = parent

	var ok bool
	n.Class, ok = c.ClassRegistry.Get(dbNode.ClassName())
	if !ok {
		// TODO node.Class = NotFoundClass...
		return n, fmt.Errorf("class %s not found", dbNode.ClassName())
	}

	n.Instance = n.Class.Create()
	n.Instance.SetNode(n) // connect them the other way round

	return n, nil
}

// Content shadows Node.DBNode.Content.
func (n *Node) Content() string {
	if n.overrideContent {
		return n.content
	} else {
		return n.DBVersion.Content()
	}
}

func (n *Node) CountChildren() (int, error) {
	return n.db.CountChildren(n)
}

func (n *Node) CountReleasedChildren(minTsCreated int64) (int, error) {
	return n.db.CountReleasedChildren(n, minTsCreated)
}

// Id shadows DBNode.Id.
// If n (e.g. parent) is nil, it returns zero.
func (n *Node) Id() int {
	if n != nil {
		return n.DBNode.Id()
	}
	return 0
}

func (n *Node) GetChildren(user auth.User, order Order, limit, offset int) ([]DBNode, error) {
	var children, err = n.db.GetChildren(n, order, limit, offset)
	if err != nil {
		return nil, err
	}
	var result = make([]DBNode, 0, len(children))
	for _, c := range children {
		node, err := n.db.NewNode(n, c, &NoVersion{})
		if err != nil {
			return nil, err
		}
		if err := node.RequirePermission(Read, user); err != nil {
			continue
		}
		result = append(result, node)
	}
	return result, nil
}

func (n *Node) GetReleasedChildren(user auth.User, order Order, limit, offset int) ([]*Node, error) {
	var children, err = n.db.GetReleasedChildren(n, order, limit, offset)
	if err != nil {
		return nil, err
	}
	var result = make([]*Node, 0, len(children))
	for _, c := range children {
		node, err := n.db.NewNode(n, c, c) // TODO ok? c is DBNodeVersion
		if err != nil {
			return nil, err
		}
		if err := node.RequirePermission(Read, user); err != nil {
			continue
		}
		result = append(result, node)
	}
	return result, nil
}

func (n *Node) SetContent(newContent string) {
	n.content = newContent
	n.overrideContent = true
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

func (n *Node) Versions() ([]DBVersionStub, error) {
	return n.db.Versions(n)
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
func (c *CoreDB) Edit(n *Node, newContent, newVersionNote, username string, workflowGroupId int) error {
	if n.Content() != newContent {
		if err := c.AddVersion(n.DBNode, newContent, fmt.Sprintf("[%s] %s", username, strings.TrimSpace(newVersionNote)), workflowGroupId); err != nil {
			return err
		}
	}
	return nil
}

// GetLocal returns the value of a local variable as HTML.
func (n *Node) GetLocal(varName string) template.HTML {
	return template.HTML(n.localVars[varName])
}

// GetLocal returns the value of a local variable as a string.
func (n *Node) GetLocalStr(varName string) string {
	return n.localVars[varName]
}

// SetLocal sets a local variable.
func (n *Node) SetLocal(name, value string) interface{} {
	n.localVars[name] = value
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

	if err := c.NodeDB.SetWorkflowGroup(n, newWorkflowGroup); err != nil {
		return err
	}

	if oldMaxWGZeroVersionNo != n.MaxWGZeroVersionNo() { // if maxWGZeroVersionNo has changed

		var err error
		n.DBVersion, err = c.GetVersion(n, n.MaxWGZeroVersionNo())
		if err != nil {
			return err
		}

		var tmpRoute = &Route{
			Request: newDummyRequest(),
			Queue:   NewQueue(""),
			Node:    n,
		}

		if err := n.Do(tmpRoute); err != nil {
			return err
		}

		if err := n.SetTags(n.Tags); err != nil {
			return err
		}

		if err := n.SetTimestamps(n.Timestamps); err != nil {
			return err
		}
	}

	return nil
}
