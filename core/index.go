package core

// An IndexDB stores timestamps and tags which are defined in node content.
type IndexDB interface {
	AddTags(parentId int, nodeId int, nodeTsChanged int64, tags []string) error
	AddTimestamps(parentId int, nodeId int, timestamps []int64) error
	ClearIndex(nodeId int) error
	RecentChildrenByTag(parentId int, now int64, tag string, limit, offset int) ([]int, error)   // uses tsChanged of max released version
	UpcomingChildrenByTag(parentId int, now int64, tag string, limit, offset int) ([]int, error) // uses timestamp
}

// AddTags calls IndexDB.AddTags using n.ParentId(), n.Id() and n.TsChanged().
// Ensure that you modify tsChanged before.
func (n *Node) AddTags(tags []string) error {
	return n.db.AddTags(n.ParentId(), n.Id(), n.TsChanged(), tags)
}

func (n *Node) AddTimestamps(timestamps []int64) error {
	return n.db.AddTimestamps(n.ParentId(), n.Id(), timestamps)
}

func (n *Node) ClearIndex() error {
	return n.db.ClearIndex(n.Id())
}

func (n *Node) RecentChildrenByTag(now int64, tag string, limit, offset int) ([]DBNode, error) {
	var nodeIds, err = n.db.RecentChildrenByTag(n.Id(), now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	var nodes = make([]DBNode, len(nodeIds))
	for i, nodeId := range nodeIds {
		nodes[i], err = n.db.GetReleasedNodeById(nodeId)
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func (n *Node) UpcomingChildrenByTag(now int64, tag string, limit, offset int) ([]DBNode, error) {
	nodeIds, err := n.db.UpcomingChildrenByTag(n.Id(), now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	var nodes = make([]DBNode, len(nodeIds))
	for i, nodeId := range nodeIds {
		nodes[i], err = n.db.GetReleasedNodeById(nodeId)
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}
