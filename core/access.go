package core

type AccessDB interface {
	GetAccessRules(nodeId int) (map[int]int, error)  // group id -> permission
	GetAllAccessRules() (map[int]map[int]int, error) // node id -> (group id -> permission)
	InsertAccessRule(nodeId int, groupId int, perm int) error
	RemoveAccessRule(nodeId int, groupId int) error
}
