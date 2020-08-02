package auth

type DBGroup interface {
	Id() int
	Name() string
	HasMember(u DBUser) (bool, error)
	Members() (map[int]interface{}, error)
}

type GroupDB interface {
	Delete(g DBGroup) error
	GetAllGroups(limit, offset int) ([]DBGroup, error)
	GetGroup(id int) (DBGroup, error)
	GetGroupsOf(u DBUser) ([]DBGroup, error)
	InsertGroup(name string) error
	Join(g DBGroup, u DBUser) error
	Leave(g DBGroup, u DBUser) error
	Writeable() bool
}

type Group DBGroup

// GetGroup shadows AuthDB.GroupDB.GetGroup.
func (a *AuthDB) GetGroup(id int) (Group, error) {
	if id == 0 {
		return AllUsers{}, nil
	}
	return a.GroupDB.GetGroup(id)
}

func (a *AuthDB) GetGroupOrReaders(id int) (Group, error) {
	if id == 0 {
		return Readers{}, nil
	}
	return a.GroupDB.GetGroup(id)
}

// GetGroupsOf shadows AuthDB.GroupDB.GetGroupsOf.
func (a *AuthDB) GetGroupsOf(u User) ([]Group, error) {
	if u == nil {
		return nil, nil
	}
	groups, err := a.GroupDB.GetGroupsOf(u)
	result := make([]Group, len(groups))
	for i := range groups {
		result[i] = groups[i]
	}
	return result, err
}

// GetAllGroups shadows AuthDB.GroupDB.GetAllGroups.
func (a *AuthDB) GetAllGroups(limit, offset int) ([]Group, error) {
	groups, err := a.GroupDB.GetAllGroups(limit, offset)
	result := make([]Group, len(groups))
	for i := range groups {
		result[i] = groups[i]
	}
	return result, err
}
