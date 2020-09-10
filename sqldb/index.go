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
	clearTag      *sql.Stmt
	clearTs       *sql.Stmt
	insertTag     *sql.Stmt
	insertTs      *sql.Stmt
	recentByTag   *sql.Stmt
	upcomingByTag *sql.Stmt
}

func NewIndexDB(db *sql.DB) *IndexDB {

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS element_tag (
		  parentId int(11) NOT NULL, /* redundant, but helpful */
		  elementId int(11) NOT NULL,
		  versionTsChanged INTEGER NOT NULL, /* redundant, denormalized copy of ts_changed of maxWGZeroVersion of element */
		  tag varchar(32) NOT NULL,
		  PRIMARY KEY (elementId, tag)
		);
		CREATE TABLE IF NOT EXISTS element_ts (
		  parentId int(11) NOT NULL, /* redundant, but helpful */
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
	indexDB.clearTag = mustPrepare(db, "DELETE FROM element_tag WHERE elementId = ?")
	indexDB.clearTs = mustPrepare(db, "DELETE FROM element_ts WHERE elementId = ?")
	indexDB.insertTag = mustPrepare(db, "INSERT INTO element_tag (parentId, elementId, versionTsChanged, tag) VALUES (?, ?, ?, ?)")
	indexDB.insertTs = mustPrepare(db, "INSERT INTO element_ts (parentId, elementId, ts) VALUES (?, ?, ?)")
	indexDB.recentByTag = mustPrepare(db, "SELECT elementId FROM element_tag WHERE parentId = ? AND versionTsChanged <= ? AND tag = ? ORDER BY versionTsChanged DESC LIMIT ? OFFSET ?")
	indexDB.upcomingByTag = mustPrepare(db, "SELECT element_ts.elementId FROM element_tag, element_ts WHERE element_ts.parentId = ? AND element_ts.elementId = element_tag.elementId AND element_ts.ts >= ? AND element_tag.tag = ? ORDER BY ts ASC LIMIT ? OFFSET ?")
	return indexDB
}

func (db *IndexDB) SetTags(parentId int, nodeId int, nodeTsChanged int64, tags []string) error {

	tx, err := db.Begin() // faster than independent inserts in SQLite
	if err != nil {
		return err
	}

	if _, err := tx.Stmt(db.clearTag).Exec(nodeId); err != nil {
		return err
	}

	var stmt = tx.Stmt(db.insertTag)

	for _, tag := range tags {

		tag = cleanTag(tag)
		if tag == "" {
			continue
		}

		if _, err := stmt.Exec(parentId, nodeId, nodeTsChanged, tag); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (db *IndexDB) SetTimestamps(parentId int, nodeId int, timestamps []int64) error {

	tx, err := db.Begin() // faster than independent inserts in SQLite
	if err != nil {
		return err
	}

	if _, err := tx.Stmt(db.clearTs).Exec(nodeId); err != nil {
		return err
	}

	var stmt = tx.Stmt(db.insertTs)

	for _, ts := range timestamps {
		if _, err := stmt.Exec(parentId, nodeId, ts); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (db *IndexDB) RecentChildrenByTag(parentId int, now int64, tag string, limit, offset int) ([]int, error) {

	tag = cleanTag(tag)
	if tag == "" {
		return nil, errors.New("no tag")
	}

	rows, err := db.recentByTag.Query(parentId, now, tag, limit, offset)
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

func (db *IndexDB) UpcomingChildrenByTag(parentId int, now int64, tag string, limit, offset int) ([]int, error) {

	tag = cleanTag(tag)
	if tag == "" {
		return nil, errors.New("no tag")
	}

	rows, err := db.upcomingByTag.Query(parentId, now, tag, limit, offset)
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
