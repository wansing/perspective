package core

type AccessDB interface {
	GetAccessRules(nodeID int) (map[int]int, error)  // group id -> permission
	GetAllAccessRules() (map[int]map[int]int, error) // node id -> (group id -> permission)
	InsertAccessRule(nodeID int, groupID int, perm int) error
	RemoveAccessRule(nodeID int, groupID int) error
}
