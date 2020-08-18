package auth

import (
	"errors"
)

type AuthDB struct {
	GroupDB
	UserDB
	WorkflowDB
}

var ErrEmptyPassword = errors.New("refusing to set empty password")

// shadows AuthDB.UserDB.SetPassword
func (a *AuthDB) SetPassword(u User, password string) error {
	if password == "" {
		return ErrEmptyPassword
	}
	return a.UserDB.SetPassword(u, password)
}
