package core

type DBGroup interface {
	ID() int
	Name() string
	HasMember(u DBUser) (bool, error)
	Members() (map[int]interface{}, error)
}

type GroupDB interface {
	Delete(g DBGroup) error
	GetAllGroups(limit, offset int) ([]DBGroup, error)
	GetGroup(id int) (DBGroup, error)
	GetGroupByName(name string) (DBGroup, error)
	GetGroupsOf(u DBUser) ([]DBGroup, error)
	InsertGroup(name string) error
	Join(g DBGroup, u DBUser) error
	Leave(g DBGroup, u DBUser) error
	Writeable() bool
}

// GetGroup shadows AuthDB.GroupDB.GetGroup.
func (c *CoreDB) GetGroup(id int) (DBGroup, error) {
	if id == 0 {
		return AllUsers{}, nil
	}
	return c.GroupDB.GetGroup(id)
}

func (c *CoreDB) GetGroupOrReaders(id int) (DBGroup, error) {
	if id == 0 {
		return Readers{}, nil
	}
	return c.GroupDB.GetGroup(id)
}
