package sqldb

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/wansing/perspective/auth"
	"github.com/wansing/perspective/util"
)

var ErrAuth = errors.New("authentication failed")

func clean(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	return name
}

func hash(salt string, password string) string {
	var hash = sha256.Sum256([]byte(password + salt))
	return hex.EncodeToString(hash[:])
}

type user struct {
	id   int
	name string
	salt string
	pass string // hash
}

func (u *user) hash(password string) string {
	return hash(u.salt, password)
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
	getAll      *sql.Stmt
	get         *sql.Stmt
	insert      *sql.Stmt
	login       *sql.Stmt
	setPassword *sql.Stmt
}

func NewUserDB(db *sql.DB) *UserDB {

	db.Exec(
		`CREATE TABLE IF NOT EXISTS usr (
			id INTEGER PRIMARY KEY,
			mail varchar(128) NOT NULL,
			salt varchar(64) NOT NULL,
			password varchar(64) NOT NULL,
			UNIQUE(mail)
		);`)

	var userDB = &UserDB{}
	userDB.DB = db
	userDB.delete = mustPrepare(db, "DELETE FROM usr WHERE id = ?")
	userDB.get = mustPrepare(db, "SELECT mail FROM usr WHERE id = ? LIMIT 1")
	userDB.getAll = mustPrepare(db, "SELECT id, mail, salt FROM usr ORDER BY mail LIMIT ? OFFSET ?")
	userDB.insert = mustPrepare(db, "INSERT INTO usr (mail) VALUES (?)") // empty password field should be safe because no hash value equals it
	userDB.login = mustPrepare(db, "SELECT id, salt, password FROM usr WHERE mail = ?")
	userDB.setPassword = mustPrepare(db, "UPDATE usr SET salt = ?, password = ? WHERE id = ?")
	return userDB
}

func (db *UserDB) Writeable() bool {
	return true
}

func (db *UserDB) ChangePassword(u auth.DBUser, old, new string) error {
	if u.(*user).hash(old) != u.(*user).pass {
		return ErrAuth
	}
	return db.SetPassword(u, new)
}

func (db *UserDB) Delete(u auth.DBUser) error {
	_, err := db.delete.Exec(u.Id())
	return err
}

// Get may return sql.ErrNoRows, because we can not compare the returned storage.User to nil.
func (db *UserDB) GetUser(id int) (auth.DBUser, error) {
	var u = &user{
		id: id,
	}
	err := db.get.QueryRow(id).Scan(&u.name)
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
		err = rows.Scan(&u.id, &u.name, &u.salt)
		if err != nil {
			return nil, err
		}
		all = append(all, u)
	}

	return all, nil
}

func (db *UserDB) InsertUser(name string) error {
	name = clean(name)
	_, err := db.insert.Exec(name)
	return err
}

func (db *UserDB) LoginUser(name, password string) (auth.DBUser, error) {

	name = clean(name)

	var u = &user{}

	err := db.login.QueryRow(name).Scan(&u.id, &u.salt, &u.pass)
	if err == sql.ErrNoRows {
		return nil, ErrAuth // user not found
	}
	if err != nil {
		return nil, err
	}

	if u.hash(password) != u.pass {
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

	salt, err := util.RandomString32()
	if err != nil {
		return err
	}

	_, err = db.setPassword.Exec(salt, hash(salt, password), u.Id())
	if err != nil {
		return err
	}

	u.(*user).salt = salt
	return nil
}
