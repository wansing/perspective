package sqldb

import (
	"database/sql"
)

type AccessDB struct {
	db     *sql.DB
	get    *sql.Stmt
	getAll *sql.Stmt
	insert *sql.Stmt
	remove *sql.Stmt
}

func NewAccessDB(db *sql.DB) *AccessDB {

	db.Exec(`
		CREATE TABLE IF NOT EXISTS access (
			elementId int(11) NOT NULL,
			groupId int(11) NOT NULL,
			permission int(11) NOT NULL,
			PRIMARY KEY (elementId, groupId)
		);`)

	var accessDB = &AccessDB{}
	accessDB.db = db
	accessDB.get = mustPrepare(db, "SELECT groupId, permission FROM access WHERE elementId = ?")
	accessDB.getAll = mustPrepare(db, "SELECT elementId, groupId, permission FROM access")
	accessDB.insert = mustPrepare(db, "INSERT OR IGNORE INTO access (elementId, groupId, permission) VALUES (?, ?, ?)")
	accessDB.remove = mustPrepare(db, "DELETE FROM access WHERE elementId = ? AND groupId = ?")
	return accessDB
}

func (e *AccessDB) GetAccessRules(nodeID int) (map[int]int, error) {
	res, err := e.get.Query(nodeID)
	if err != nil {
		return nil, err
	}
	var rules = map[int]int{}
	for res.Next() {
		var groupID, perm int
		if err = res.Scan(&groupID, &perm); err != nil {
			return nil, err
		}
		rules[groupID] = perm
	}
	return rules, nil
}

func (e *AccessDB) GetAllAccessRules() (map[int]map[int]int, error) {
	res, err := e.getAll.Query()
	if err != nil {
		return nil, err
	}
	var all = make(map[int]map[int]int)
	for res.Next() {
		var nodeID, groupID, perm int
		if err = res.Scan(&nodeID, &groupID, &perm); err != nil {
			return nil, err
		}
		if _, ok := all[nodeID]; !ok {
			all[nodeID] = make(map[int]int)
		}
		all[nodeID][groupID] = perm
	}
	return all, nil
}

func (e *AccessDB) InsertAccessRule(nodeID int, groupID int, perm int) error {
	_, err := e.insert.Exec(nodeID, groupID, perm)
	return err
}

func (e *AccessDB) RemoveAccessRule(nodeID int, groupID int) error {
	_, err := e.remove.Exec(nodeID, groupID)
	return err
}
