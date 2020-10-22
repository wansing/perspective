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

func (db *IndexDB) SetTags(parentID int, nodeID int, nodeTsChanged int64, tags []string) error {

	tx, err := db.Begin() // faster than independent inserts in SQLite
	if err != nil {
		return err
	}

	if _, err := tx.Stmt(db.clearTag).Exec(nodeID); err != nil {
		return err
	}

	var stmt = tx.Stmt(db.insertTag)

	for _, tag := range tags {

		tag = cleanTag(tag)
		if tag == "" {
			continue
		}

		if _, err := stmt.Exec(parentID, nodeID, nodeTsChanged, tag); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (db *IndexDB) SetTimestamps(parentID int, nodeID int, timestamps []int64) error {

	tx, err := db.Begin() // faster than independent inserts in SQLite
	if err != nil {
		return err
	}

	if _, err := tx.Stmt(db.clearTs).Exec(nodeID); err != nil {
		return err
	}

	var stmt = tx.Stmt(db.insertTs)

	for _, ts := range timestamps {
		if _, err := stmt.Exec(parentID, nodeID, ts); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (db *IndexDB) RecentChildrenByTag(parentID int, now int64, tag string, limit, offset int) ([]int, error) {

	tag = cleanTag(tag)
	if tag == "" {
		return nil, errors.New("no tag")
	}

	rows, err := db.recentByTag.Query(parentID, now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodeIDs = []int{}
	for rows.Next() {
		var nodeID int
		if err = rows.Scan(&nodeID); err != nil {
			return nil, err
		}
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs, nil
}

func (db *IndexDB) UpcomingChildrenByTag(parentID int, now int64, tag string, limit, offset int) ([]int, error) {

	tag = cleanTag(tag)
	if tag == "" {
		return nil, errors.New("no tag")
	}

	rows, err := db.upcomingByTag.Query(parentID, now, tag, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodeIDs = []int{}
	for rows.Next() {
		var nodeID int
		if err = rows.Scan(&nodeID); err != nil {
			return nil, err
		}
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs, nil
}
