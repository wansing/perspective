package sqldb

import (
	"database/sql"
	"errors"
	"strings"
)

func cleanTag(tag string) string {
	return strings.TrimSpace(strings.ToLower(tag))
}

type IndexDB struct {
	*sql.DB
	clearTagStmt      *sql.Stmt
	clearTsStmt       *sql.Stmt
	insertTagStmt     *sql.Stmt
	insertTsStmt      *sql.Stmt
	recentByTagStmt   *sql.Stmt
	upcomingByTagStmt *sql.Stmt
}

func NewIndexDB(db *sql.DB) *IndexDB {

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS element_tag (
		  elementId int(11) NOT NULL,
		  versionTsChanged INTEGER NOT NULL, /* redundant, denormalized copy of ts_changed of maxWGZeroVersion of element */
		  tag varchar(32) NOT NULL,
		  PRIMARY KEY (elementId, tag)
		);
		CREATE TABLE IF NOT EXISTS element_ts (
		  elementId int(11) NOT NULL,
		  ts int(11) NOT NULL,
		  PRIMARY KEY (elementId, ts)
		);
		`)
	if err != nil {
		panic(err)
	}

	var indexDB = &IndexDB{}
	indexDB.DB = db
	indexDB.clearTagStmt = mustPrepare(db, "DELETE FROM element_tag WHERE elementId = ?")
	indexDB.clearTsStmt = mustPrepare(db, "DELETE FROM element_ts WHERE elementId = ?")
	indexDB.insertTagStmt = mustPrepare(db, "INSERT INTO element_tag (elementId, versionTsChanged, tag) VALUES (?, ?, ?)")
	indexDB.insertTsStmt = mustPrepare(db, "INSERT INTO element_ts (elementId, ts) VALUES (?, ?)")
	indexDB.recentByTagStmt = mustPrepare(db, "SELECT elementId FROM element_tag WHERE versionTsChanged <= ? AND tag = ? ORDER BY versionTsChanged DESC LIMIT ? OFFSET ?")
	indexDB.upcomingByTagStmt = mustPrepare(db, "SELECT element_ts.elementId FROM element_tag, element_ts WHERE element_ts.elementId = element_tag.elementId AND element_ts.ts >= ? AND element_tag.tag = ? ORDER BY ts ASC LIMIT ? OFFSET ?")
	return indexDB
}

func (db *IndexDB) AddTags(nodeId int, versionTsChanged int64, tags []string) error {

	tx, err := db.Begin() // faster than independent inserts in SQLite
	if err != nil {
		return err
	}

	var stmt = tx.Stmt(db.insertTagStmt)

	for _, tag := range tags {

		tag = cleanTag(tag)
		if tag == "" {
			continue
		}

		_, err := stmt.Exec(nodeId, versionTsChanged, tag)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (db *IndexDB) AddTimestamps(nodeId int, timestamps []int64) error {

	tx, err := db.Begin() // faster than independent inserts in SQLite
	if err != nil {
		return err
	}

	var stmt = tx.Stmt(db.insertTsStmt)

	for _, ts := range timestamps {
		_, err := stmt.Exec(nodeId, ts)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (db *IndexDB) ClearIndex(nodeId int) error {
	if _, err := db.clearTagStmt.Exec(nodeId); err != nil {
		return err
	}
	if _, err := db.clearTsStmt.Exec(nodeId); err != nil {
		return err
	}
	return nil
}

func (db *IndexDB) RecentByTag(now int64, tag string, limit, offset int) ([]int, error) {

	tag = cleanTag(tag)
	if tag == "" {
		return nil, errors.New("no tag")
	}

	rows, err := db.recentByTagStmt.Query(now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodeIds = []int{}
	for rows.Next() {
		var nodeId int
		if err = rows.Scan(&nodeId); err != nil {
			return nil, err
		}
		nodeIds = append(nodeIds, nodeId)
	}
	return nodeIds, nil
}

func (db *IndexDB) UpcomingByTag(now int64, tag string, limit, offset int) ([]int, error) {

	tag = cleanTag(tag)
	if tag == "" {
		return nil, errors.New("no tag")
	}

	rows, err := db.upcomingByTagStmt.Query(now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodeIds = []int{}
	for rows.Next() {
		var nodeId int
		if err = rows.Scan(&nodeId); err != nil {
			return nil, err
		}
		nodeIds = append(nodeIds, nodeId)
	}
	return nodeIds, nil
}
