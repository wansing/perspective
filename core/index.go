package core

// An IndexDB stores timestamps and tags which are defined in node content.
type IndexDB interface {
	SetTags(parentID int, nodeID int, nodeTsChanged int64, tags []string) error
	SetTimestamps(parentID int, nodeID int, timestamps []int64) error
	RecentChildrenByTag(parentID int, now int64, tag string, limit, offset int) ([]int, error)   // uses tsChanged of max released version
	UpcomingChildrenByTag(parentID int, now int64, tag string, limit, offset int) ([]int, error) // uses timestamp
}

// SetTags calls IndexDB.SetTags using n.ParentID(), n.ID() and n.TsChanged().
// Ensure that you set tsChanged before.
func (n *Node) SetTags(tags []string) error {
	v, err := n.GetVersion(n.MaxWGZeroVersionNo())
	if err != nil {
		return err
	}
	return n.db.SetTags(n.ParentID(), n.ID(), v.TsChanged(), tags)
}

func (n *Node) SetTimestamps(timestamps []int64) error {
	return n.db.SetTimestamps(n.ParentID(), n.ID(), timestamps)
}

func (n *Node) RecentChildrenByTag(user DBUser, now int64, tag string, limit, offset int) ([]*Node, error) {
	return n.childrenByTag(n.db.RecentChildrenByTag, user, now, tag, limit, offset)
}

func (n *Node) UpcomingChildrenByTag(user DBUser, now int64, tag string, limit, offset int) ([]*Node, error) {
	return n.childrenByTag(n.db.UpcomingChildrenByTag, user, now, tag, limit, offset)
}

func (n *Node) childrenByTag(f func(id int, now int64, tag string, limit, offset int) ([]int, error), user DBUser, now int64, tag string, limit, offset int) ([]*Node, error) {
	var nodeIDs, err = f(n.ID(), now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	var nodes = make([]*Node, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		dbNode, err := n.db.GetNodeByID(nodeID)
		if err != nil {
			return nil, err
		}
		child := n.db.NewNode(n, dbNode)
		if err := child.RequirePermission(Read, user); err != nil {
			continue
		}
		nodes = append(nodes, child)
	}
	return nodes, nil
}
