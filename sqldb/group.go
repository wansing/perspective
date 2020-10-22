package sqldb

import (
	"database/sql"
	"errors"

	"github.com/wansing/perspective/core"
)

type group struct {
	db            *GroupDB // required for lazy loading
	id            int
	name          string
	members       map[int]interface{} // user id => struct{}
	membersLoaded bool                // lazy loading
}

func (g *group) ID() int {
	return g.id
}

func (g *group) Name() string {
	return g.name
}

func (g *group) HasMember(u core.DBUser) (bool, error) {
	if members, err := g.Members(); err == nil {
		_, ok := members[u.ID()]
		return ok, nil
	} else {
		return false, err
	}
}

func (g *group) Members() (map[int]interface{}, error) {

	if !g.membersLoaded {

		g.members = make(map[int]interface{})

		rows, err := g.db.members.Query(g.id)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var userID int
			if err = rows.Scan(&userID); err != nil {
				return nil, err
			}
			g.members[userID] = struct{}{}
		}

		g.membersLoaded = true
	}

	return g.members, nil
}

type GroupDB struct {
	*sql.DB
	delete     *sql.Stmt
	get        *sql.Stmt
	getAll     *sql.Stmt
	getByName  *sql.Stmt
	getOf      *sql.Stmt
	insert     *sql.Stmt
	join       *sql.Stmt
	leave      *sql.Stmt
	leaveUsers *sql.Stmt
	members    *sql.Stmt
}

func NewGroupDB(db *sql.DB) *GroupDB {

	db.Exec(`
		CREATE TABLE IF NOT EXISTS grp (
			id INTEGER PRIMARY KEY,
			name varchar(64) NOT NULL,
			UNIQUE(name)
		);
		CREATE TABLE IF NOT EXISTS membership (
			grp int(11) NOT NULL,
			usr int(11) NOT NULL,
			PRIMARY KEY (grp, usr)
		);`)

	var groupDB = &GroupDB{}
	groupDB.DB = db
	groupDB.delete = mustPrepare(db, "DELETE FROM grp WHERE id = ?")
	groupDB.get = mustPrepare(db, "SELECT name FROM grp WHERE id = ? LIMIT 1")
	groupDB.getAll = mustPrepare(db, "SELECT id, name FROM grp ORDER BY name LIMIT ? OFFSET ?")
	groupDB.getByName = mustPrepare(db, "SELECT id FROM grp WHERE name = ? LIMIT 1")
	groupDB.getOf = mustPrepare(db, "SELECT grp.id, grp.name FROM grp, membership WHERE grp.id = membership.grp AND membership.usr = ? ORDER BY grp.name")
	groupDB.insert = mustPrepare(db, "INSERT INTO grp (name) VALUES (?)")
	groupDB.join = mustPrepare(db, "INSERT INTO membership (grp, usr) VALUES (?, ?)")
	groupDB.leave = mustPrepare(db, "DELETE FROM membership WHERE grp = ? AND usr = ?")
	groupDB.leaveUsers = mustPrepare(db, "DELETE FROM membership WHERE grp = ?")
	groupDB.members = mustPrepare(db, "SELECT usr FROM membership WHERE grp = ?")
	return groupDB
}

func (db *GroupDB) Writeable() bool {
	return true
}

func (db *GroupDB) Delete(g core.DBGroup) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Stmt(db.leaveUsers).Exec(g.ID())
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Stmt(db.delete).Exec(g.ID())
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *GroupDB) GetGroup(id int) (core.DBGroup, error) {
	var g = &group{
		db: db,
		id: id,
	}
	return g, db.get.QueryRow(id).Scan(&g.name)
}

func (db *GroupDB) GetGroupByName(name string) (core.DBGroup, error) {
	var g = &group{
		db:   db,
		name: name,
	}
	return g, db.getByName.QueryRow(name).Scan(&g.id)
}

func (db *GroupDB) getMultiple(stmt *sql.Stmt, args ...interface{}) ([]core.DBGroup, error) {

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups = []core.DBGroup{}

	for rows.Next() {
		var id int
		var name string
		err = rows.Scan(&id, &name)
		if err != nil {
			return nil, err
		}
		groups = append(groups, &group{
			db:   db,
			id:   id,
			name: name,
		})
	}

	return groups, nil
}

func (db *GroupDB) GetAllGroups(limit, offset int) ([]core.DBGroup, error) {
	return db.getMultiple(db.getAll, limit, offset)
}

func (db *GroupDB) GetGroupsOf(u core.DBUser) ([]core.DBGroup, error) {
	return db.getMultiple(db.getOf, u.ID())
}

func (db *GroupDB) InsertGroup(name string) error {
	_, err := db.insert.Exec(name)
	return err
}

func (db *GroupDB) Join(g core.DBGroup, user core.DBUser) error {

	if user.ID() == 0 {
		return errors.New("can't add all users")
	}

	_, err := db.join.Exec(g.ID(), user.ID())
	if err != nil {
		return err
	}

	if grp := g.(*group); grp.members != nil { // if members are loaded
		grp.members[user.ID()] = struct{}{}
	}
	return nil
}

func (db *GroupDB) Leave(g core.DBGroup, user core.DBUser) error {

	if user.ID() == 0 {
		return errors.New("can't remove all users")
	}

	_, err := db.leave.Exec(g.ID(), user.ID())
	if err != nil {
		return err
	}

	if grp := g.(*group); grp.members != nil { // if members are loaded
		delete(grp.members, user.ID())
	}
	return nil
}
