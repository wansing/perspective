package core

import (
	"github.com/wansing/perspective/auth"
)

type User struct {
	auth.User
}

// RequirePermission returns an error if the given user does not have the given permission on the node.
func (u *User) RequirePermission(perm Permission, n *Node) error {
	return u.requirePermission(perm, n, nil)
}

// RequirePermissionRules returns all access rules under which the given user has the given permission on the node:
//
//     node id -> (group id -> interface{})
//
// If the user does not have the required permission, an error is returned.
//
// Edit permission implies read permission, but edit permission is modeled through workflows and not through rules.
// Thus the function may return (nil, nil).
func (u *User) RequirePermissionRules(perm Permission, n *Node) (map[int]map[int]interface{}, error) {
	rules := make(map[int]map[int]interface{})
	return rules, u.requirePermission(perm, n, &rules)
}

// requirePermission calls requirePermissionRecursive. On failure, it makes use of the convention that "edit" implies "read".
func (u *User) requirePermission(perm Permission, n *Node, permittingRules *map[int]map[int]interface{}) error {

	if err := u.requirePermissionRecursive(perm, n, permittingRules); err == nil {
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

func (u *User) requirePermissionRecursive(perm Permission, n *Node, permittingRules *map[int]map[int]interface{}) error {

	if n == nil {
		return ErrUnauthorized
	}

	if err := n.db.requireRule(perm, n.Id(), u, permittingRules); err == nil {
		return nil
	}

	// recursion
	if n.Parent != nil {
		if err := u.requirePermissionRecursive(perm, n.Parent, permittingRules); err == nil {
			return nil
		}
	}

	return ErrUnauthorized
}

// ReleaseState returns the ReleaseState which describes the relation between a node, a version and a user.
func (u *User) ReleaseState(n *Node, v DBVersion) (*auth.ReleaseState, error) {
	var workflow, err = n.GetWorkflow()
	if err != nil {
		return nil, err
	}
	return auth.GetReleaseState(workflow, v.WorkflowGroupId(), u)
}
