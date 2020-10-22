package core

import (
	"errors"
)

type DBUser interface {
	ID() int
	Name() string // can be email address
}

type UserDB interface {
	ChangePassword(u DBUser, old, new string) error
	Delete(u DBUser) error
	GetUser(id int) (DBUser, error)
	GetUserByName(name string) (DBUser, error)
	GetAllUsers(limit, offset int) ([]DBUser, error)
	InsertUser(name string) (DBUser, error)
	LoginUser(name, password string) (DBUser, error)
	SetPassword(u DBUser, password string) error
	Writeable() bool
}

var ErrEmptyPassword = errors.New("refusing to set empty password")

// shadows UserDB.SetPassword
func (c *CoreDB) SetPassword(u DBUser, password string) error {
	if password == "" {
		return ErrEmptyPassword
	}
	return c.UserDB.SetPassword(u, password)
}

// RequirePermission returns an error if the given user does not have the given permission on the node.
func (n *Node) RequirePermission(perm Permission, u DBUser) error {
	return n.requirePermission(perm, u, nil)
}

// RequirePermissionRules returns all access rules under which the given user has the given permission on the node:
//
//     node id -> (group id -> interface{})
//
// If the user does not have the required permission, an error is returned.
//
// Edit permission implies read permission, but edit permission is modeled through workflows and not through rules.
// Thus the function may return (nil, nil).
func (n *Node) RequirePermissionRules(perm Permission, u DBUser) (map[int]map[int]interface{}, error) {
	rules := make(map[int]map[int]interface{})
	return rules, n.requirePermission(perm, u, &rules)
}

// requirePermission calls requirePermissionRecursive. On failure, it makes use of the convention that "edit" implies "read".
func (n *Node) requirePermission(perm Permission, u DBUser, permittingRules *map[int]map[int]interface{}) error {

	if err := n.requirePermissionRecursive(perm, u, permittingRules); err == nil {
		return nil
	}

	// every member of any workflow group can read

	if perm == Read {

		workflow, err := n.GetWorkflow()
		if err != nil {
			return err
		}

		groups, err := workflow.Groups()
		if err != nil {
			return err
		}

		for _, group := range groups {
			hasMember, err := group.HasMember(u)
			if err != nil {
				return err
			}
			if hasMember {
				return nil // authorized
			}
		}
	}

	return ErrUnauthorized
}

func (n *Node) requirePermissionRecursive(perm Permission, u DBUser, permittingRules *map[int]map[int]interface{}) error {

	if n == nil {
		return ErrUnauthorized
	}

	if err := n.db.requireRule(perm, n.ID(), u, permittingRules); err == nil {
		return nil
	}

	// recursion
	if n.Parent != nil {
		if err := n.Parent.requirePermissionRecursive(perm, u, permittingRules); err == nil {
			return nil
		}
	}

	return ErrUnauthorized
}

// ReleaseState returns the ReleaseState which describes the relation between a node, a version and a user.
func (n *Node) ReleaseState(v DBVersion, u DBUser) (*ReleaseState, error) {
	var workflow, err = n.GetWorkflow()
	if err != nil {
		return nil, err
	}
	return GetReleaseState(workflow, v.WorkflowGroupID(), u)
}
