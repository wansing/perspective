package core

import (
	"errors"

	"github.com/wansing/perspective/auth"
)

// RequirePermission returns an error if the given user does not have the given permission on the node.
func (n *Node) RequirePermission(perm Permission, u auth.User) error {
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
func (n *Node) RequirePermissionRules(perm Permission, u auth.User) (map[int]map[int]interface{}, error) {
	rules := make(map[int]map[int]interface{})
	return rules, n.requirePermission(perm, u, &rules)
}

// requirePermission calls requirePermissionRecursive. On failure, it makes use of the convention that "edit" implies "read".
func (n *Node) requirePermission(perm Permission, u auth.User, permittingRules *map[int]map[int]interface{}) error {

	if err := n.requirePermissionRecursive(perm, u, permittingRules); err == nil {
		return nil
	}

	// edit implies read

	if perm == Read {
		rs, err := n.ReleaseState(u)
		if err != nil {
			return err
		}
		if rs.CanEdit() {
			return nil // authorized
		}
	}

	return ErrUnauthorized
}

func (n *Node) requirePermissionRecursive(perm Permission, u auth.User, permittingRules *map[int]map[int]interface{}) error {

	if n == nil {
		return ErrUnauthorized
	}

	if err := n.db.requirePermissionById(perm, n.Id(), u, permittingRules); err == nil {
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

// GetWorkflow returns the workflow which applies (but is not necessarily directly assigned) to the node.
func (n *Node) GetWorkflow() (*auth.Workflow, error) {

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

// AssignWorkflow shadows CoreDB.EditorsDB.AssignWorkflow.
func (c *CoreDB) AssignWorkflow(n *Node, childrenOnly bool, workflowId int) error {
	return c.EditorsDB.AssignWorkflowId(n.Id(), childrenOnly, workflowId)
}

// GetAssignedWorkflow returns the workflow which is directly assigned to the node, if any.
func (n *Node) GetAssignedWorkflow(childrenOnly bool) (*auth.Workflow, error) {
	var workflowId, err = n.db.GetAssignedWorkflowId(n.Id(), childrenOnly)
	if err != nil {
		return nil, err
	}
	if workflowId == 0 {
		return nil, nil
	}
	return n.db.Auth.GetWorkflow(workflowId)
}

// AssignWorkflow shadows CoreDB.EditorsDB.UnassignWorkflow.
func (c *CoreDB) UnassignWorkflow(n *Node, childrenOnly bool) error {
	return c.EditorsDB.UnassignWorkflow(n.Id(), childrenOnly)
}

// ReleaseState returns the ReleaseState which describes the relation between the node and the given user.
func (n *Node) ReleaseState(user auth.User) (*auth.ReleaseState, error) {
	var workflow, err = n.GetWorkflow()
	if err != nil {
		return nil, err
	}
	return auth.GetReleaseState(workflow, n.WorkflowGroupId(), user)
}
