package core

// An EditorsDB assigns workflows to nodes.
// A node can bear zero or one workflows.
type EditorsDB interface {
	AssignWorkflowID(nodeID int, childrenOnly bool, workflowID int) error
	GetAssignedWorkflowID(nodeID int, childrenOnly bool) (int, error) // zero if not assigned
	GetAllWorkflowAssignments() (map[int]map[bool]int, error)         // node id -> childrenOnly -> workflow id
	UnassignWorkflow(nodeID int, childrenOnly bool) error
}
