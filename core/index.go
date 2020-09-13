package core

//import "github.com/wansing/perspective/auth"

// An IndexDB stores timestamps and tags which are defined in node content.
type IndexDB interface {
	SetTags(parentId int, nodeId int, nodeTsChanged int64, tags []string) error
	SetTimestamps(parentId int, nodeId int, timestamps []int64) error
	RecentChildrenByTag(parentId int, now int64, tag string, limit, offset int) ([]int, error)   // uses tsChanged of max released version
	UpcomingChildrenByTag(parentId int, now int64, tag string, limit, offset int) ([]int, error) // uses timestamp
}

// SetTags calls IndexDB.SetTags using n.ParentId(), n.Id() and n.TsChanged().
// Ensure that you set tsChanged before.
func (n *Node) SetTags(tags []string) error {
	v, err := n.GetReleasedVersion()
	if err != nil {
		return err
	}
	return n.db.SetTags(n.ParentId(), n.Id(), v.TsChanged(), tags)
}

func (n *Node) SetTimestamps(timestamps []int64) error {
	return n.db.SetTimestamps(n.ParentId(), n.Id(), timestamps)
}

func (n *Node) RecentChildrenByTag(user *User, now int64, tag string, limit, offset int) ([]*Node, error) {
	return n.childrenByTag(n.db.RecentChildrenByTag, user, now, tag, limit, offset)
}

func (n *Node) UpcomingChildrenByTag(user *User, now int64, tag string, limit, offset int) ([]*Node, error) {
	return n.childrenByTag(n.db.UpcomingChildrenByTag, user, now, tag, limit, offset)
}

func (n *Node) childrenByTag(f func(id int, now int64, tag string, limit, offset int) ([]int, error), user *User, now int64, tag string, limit, offset int) ([]*Node, error) {
	var nodeIds, err = f(n.Id(), now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	var nodes = make([]*Node, 0, len(nodeIds))
	for _, nodeId := range nodeIds {
		dbNode, err := n.db.GetNodeById(nodeId)
		if err != nil {
			return nil, err
		}
		child := n.db.NewNode(n, dbNode)
		if err := user.RequirePermission(Read, child); err != nil {
			continue
		}
		nodes = append(nodes, child)
	}
	return nodes, nil
}
