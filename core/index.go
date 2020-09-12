package core

import "github.com/wansing/perspective/auth"

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
	return n.db.SetTags(n.ParentId(), n.Id(), n.TsChanged(), tags)
}

func (n *Node) SetTimestamps(timestamps []int64) error {
	return n.db.SetTimestamps(n.ParentId(), n.Id(), timestamps)
}

func (n *Node) RecentChildrenByTag(user auth.User, now int64, tag string, limit, offset int) ([]*Node, error) {
	var nodeIds, err = n.db.RecentChildrenByTag(n.Id(), now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	var result = make([]*Node, 0, len(nodeIds))
	for _, nodeId := range nodeIds {
		dbNode, dbVersion, err := n.db.GetReleasedNodeById(nodeId)
		if err != nil {
			return nil, err
		}
		node, err := n.db.NewNode(n, dbNode, dbVersion)
		if err != nil {
			return nil, err
		}
		if err := node.RequirePermission(Read, user); err != nil {
			continue
		}
		result = append(result, node)
	}
	return result, nil
}

func (n *Node) UpcomingChildrenByTag(now int64, tag string, limit, offset int) ([]DBNode, error) {
	nodeIds, err := n.db.UpcomingChildrenByTag(n.Id(), now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	var nodes = make([]DBNode, len(nodeIds))
	for i, nodeId := range nodeIds {
		nodes[i], _, err = n.db.GetReleasedNodeById(nodeId) // TODO GetReleasedNodeById should return DBNode only
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}
