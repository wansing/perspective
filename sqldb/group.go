package sqldb

import (
	"database/sql"
	"errors"

	"github.com/wansing/perspective/auth"
)

type group struct {
	db            *GroupDB // required for lazy loading
	id            int
	name          string
	members       map[int]interface{} // user id => struct{}
	membersLoaded bool                // lazy loading
}

func (g *group) Id() int {
	return g.id
}

func (g *group) Name() string {
	return g.name
}

func (g *group) HasMember(u auth.DBUser) (bool, error) {
	if members, err := g.Members(); err == nil {
		_, ok := members[u.Id()]
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
			var userId int
			if err = rows.Scan(&userId); err != nil {
				return nil, err
			}
			g.members[userId] = struct{}{}
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

func (db *GroupDB) Delete(g auth.DBGroup) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Stmt(db.leaveUsers).Exec(g.Id())
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Stmt(db.delete).Exec(g.Id())
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *GroupDB) GetGroup(id int) (auth.DBGroup, error) {
	var g = &group{
		db: db,
		id: id,
	}
	return g, db.get.QueryRow(id).Scan(&g.name)
}

func (db *GroupDB) getMultiple(stmt *sql.Stmt, args ...interface{}) ([]auth.DBGroup, error) {

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups = []auth.DBGroup{}

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

func (db *GroupDB) GetAllGroups(limit, offset int) ([]auth.DBGroup, error) {
	return db.getMultiple(db.getAll, limit, offset)
}

func (db *GroupDB) GetGroupsOf(u auth.DBUser) ([]auth.DBGroup, error) {
	return db.getMultiple(db.getOf, u.Id())
}

func (db *GroupDB) InsertGroup(name string) error {
	_, err := db.insert.Exec(name)
	return err
}

func (db *GroupDB) Join(g auth.DBGroup, user auth.DBUser) error {

	if user.Id() == 0 {
		return errors.New("can't add all users")
	}

	_, err := db.join.Exec(g.Id(), user.Id())
	if err != nil {
		return err
	}

	g.(*group).members[user.Id()] = struct{}{}
	return nil
}

func (db *GroupDB) Leave(g auth.DBGroup, user auth.DBUser) error {

	if user.Id() == 0 {
		return errors.New("can't remove all users")
	}

	_, err := db.leave.Exec(g.Id(), user.Id())
	if err != nil {
		return err
	}

	delete(g.(*group).members, user.Id())
	return nil
}
