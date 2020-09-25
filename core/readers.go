package core

// Readers implements Group and represents everyone with read permission. It is similar to AllUsers and is used when dealing with edit permissions.
type Readers struct {
	AllUsers
}

func (Readers) Name() string {
	return "everyone with read permission"
}

func (Readers) HasMember(DBUser) (bool, error) {
	return false, nil
}
