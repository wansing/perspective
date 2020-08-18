package sqldb

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/wansing/perspective/auth"
	"golang.org/x/crypto/bcrypt"
)

var ErrAuth = errors.New("authentication failed")

func clean(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	return name
}

func generate(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

type user struct {
	id   int
	name string
	hash []byte // should contain salt, can be empty, compare must fail if it's empty
}

func (u *user) compare(password string) bool {
	return bcrypt.CompareHashAndPassword(u.hash, []byte(password)) == nil
}

func (u *user) Id() int {
	return u.id
}

func (u *user) Name() string {
	return u.name
}

type UserDB struct {
	*sql.DB
	delete      *sql.Stmt
	get         *sql.Stmt
	getAll      *sql.Stmt
	getByName   *sql.Stmt
	insert      *sql.Stmt
	login       *sql.Stmt
	setPassword *sql.Stmt
}

func NewUserDB(db *sql.DB) *UserDB {

	db.Exec(
		`CREATE TABLE IF NOT EXISTS usr (
			id INTEGER PRIMARY KEY,
			mail varchar(128) NOT NULL,
			hash BLOB(128) NOT NULL,
			UNIQUE(mail)
		);`)

	var userDB = &UserDB{}
	userDB.DB = db
	userDB.delete = mustPrepare(db, "DELETE FROM usr WHERE id = ?")
	userDB.get = mustPrepare(db, "SELECT mail, hash FROM usr WHERE id = ? LIMIT 1")
	userDB.getAll = mustPrepare(db, "SELECT id, mail FROM usr ORDER BY mail LIMIT ? OFFSET ?")
	userDB.getByName = mustPrepare(db, "SELECT id, hash FROM usr WHERE mail = ? LIMIT 1")
	userDB.insert = mustPrepare(db, "INSERT INTO usr (mail, hash) VALUES (?, '')")
	userDB.login = mustPrepare(db, "SELECT id, hash FROM usr WHERE mail = ?")
	userDB.setPassword = mustPrepare(db, "UPDATE usr SET hash = ? WHERE id = ?")
	return userDB
}

func (db *UserDB) Writeable() bool {
	return true
}

func (db *UserDB) ChangePassword(u auth.DBUser, old, new string) error {
	if !u.(*user).compare(old) {
		return ErrAuth
	}
	return db.SetPassword(u, new)
}

func (db *UserDB) Delete(u auth.DBUser) error {
	_, err := db.delete.Exec(u.Id())
	return err
}

// Get may return sql.ErrNoRows, because we can not compare the returned auth.DBUser to nil.
func (db *UserDB) GetUser(id int) (auth.DBUser, error) {
	var u = &user{
		id: id,
	}
	err := db.get.QueryRow(id).Scan(&u.name, &u.hash)
	return u, err
}

// Get may return sql.ErrNoRows, because we can not compare the returned auth.DBUser to nil.
func (db *UserDB) GetUserByName(name string) (auth.DBUser, error) {
	var u = &user{
		name: name,
	}
	err := db.getByName.QueryRow(name).Scan(&u.id, &u.hash)
	return u, err
}

func (db *UserDB) GetAllUsers(limit, offset int) ([]auth.DBUser, error) {

	var all = []auth.DBUser{}

	rows, err := db.getAll.Query(limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var u = &user{}
		err = rows.Scan(&u.id, &u.name)
		if err != nil {
			return nil, err
		}
		all = append(all, u)
	}

	return all, nil
}

func (db *UserDB) InsertUser(name string) (auth.DBUser, error) {
	name = clean(name)
	res, err := db.insert.Exec(name)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	return &user{
		id:   int(id),
		name: name,
	}, err
}

func (db *UserDB) LoginUser(name, password string) (auth.DBUser, error) {

	name = clean(name)

	var u = &user{}

	err := db.login.QueryRow(name).Scan(&u.id, &u.hash)
	if err == sql.ErrNoRows {
		return nil, ErrAuth // user not found
	}
	if err != nil {
		return nil, err
	}

	if !u.compare(password) {
		return nil, ErrAuth // wrong password
	}

	return u, nil
}

func (db *UserDB) SetPassword(u auth.DBUser, password string) error {

	if password == "" {
		return errors.New("no password given")
	}

	if u.Id() == 0 {
		return errors.New("can't set password of user 0")
	}

	hash, err := generate(password)
	if err != nil {
		return err
	}

	_, err = db.setPassword.Exec(hash, u.Id())
	if err != nil {
		return err
	}

	u.(*user).hash = hash
	return nil
}
