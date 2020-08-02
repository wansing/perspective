package core

// Higher permissions include lower permissions.
type Permission int

const (
	None   Permission = 1
	Read   Permission = 100
	Create Permission = 200 // create descendant nodes
	Remove Permission = 400 // remove descendant nodes (but don't delete this node)
	Admin  Permission = 500 // edit access rules of this node
)

func (p Permission) String() string {
	switch p {
	case None:
		return "none"
	case Read:
		return "read"
	case Create:
		return "create"
	case Remove:
		return "remove"
	case Admin:
		return "admin"
	}
	return "unknown"
}

func (p Permission) Valid() bool {
	switch p {
	case None:
		return true
	case Read:
		return true
	case Create:
		return true
	case Remove:
		return true
	case Admin:
		return true
	default:
		return false
	}
}
