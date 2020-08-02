package core

// An EditorsDB assigns workflows to nodes.
// A node can bear zero or one workflows.
type EditorsDB interface {
	AssignWorkflowId(nodeId int, childrenOnly bool, workflowId int) error
	GetAssignedWorkflowId(nodeId int, childrenOnly bool) (int, error) // zero if not assigned
	GetAllWorkflowAssignments() (map[int]map[bool]int, error)         // node id -> childrenOnly -> workflow id
	UnassignWorkflow(nodeId int, childrenOnly bool) error
}
