package auth

type DBUser interface {
	Id() int
	Name() string // can be email address
}

type UserDB interface {
	ChangePassword(u DBUser, old, new string) error
	Delete(u DBUser) error
	GetUser(id int) (DBUser, error)
	GetAllUsers(limit, offset int) ([]DBUser, error)
	InsertUser(name string) error
	LoginUser(name, password string) (DBUser, error)
	SetPassword(u DBUser, password string) error
	Writeable() bool
}

type User DBUser

// GetAllUsers shadows AuthDB.UserDB.GetAllUsers.
func (a *AuthDB) GetAllUsers(limit, offset int) ([]User, error) {
	users, err := a.UserDB.GetAllUsers(limit, offset)
	result := make([]User, len(users))
	for i := range users {
		result[i] = users[i]
	}
	return result, err
}
