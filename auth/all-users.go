package auth

import (
	"errors"
)

// AllUsers implements Group and represents all users, including the public.
type AllUsers struct{}

func (AllUsers) Id() int {
	return 0
}

func (AllUsers) Name() string {
	return "all users"
}

func (AllUsers) Join(DBUser) error {
	return errors.New("can't join")
}

func (AllUsers) Leave(DBUser) error {
	return errors.New("can't leave")
}

func (AllUsers) HasMember(DBUser) (bool, error) {
	return true, nil
}

func (AllUsers) Members() (map[int]interface{}, error) {
	return nil, errors.New("not available")
}

func (AllUsers) Delete() error {
	return errors.New("not available")
}
