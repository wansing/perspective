package sqldb

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wansing/perspective/core"
)

type versionStub struct {
	versionNo       int
	versionNote     string
	tsChanged       int64
	workflowGroupId int
}

func (v *versionStub) VersionNo() int {
	return v.versionNo
}

func (v *versionStub) VersionNote() string {
	return v.versionNote
}

func (v *versionStub) TsChanged() int64 {
	return v.tsChanged
}

func (v *versionStub) WorkflowGroupId() int {
	return v.workflowGroupId
}

type node struct {
	db *NodeDB

	// node
	id                 int
	parentId           int
	slug               string
	className          string
	tsCreated          int64
	maxVersionNo       int
	maxWGZeroVersionNo int

	// version
	versionStub
	content string
}

func (e *node) Id() int {
	return e.id
}

func (e *node) ParentId() int {
	return e.parentId
}

func (e *node) Slug() string {
	return e.slug
}

func (e *node) ClassName() string {
	return e.className
}

func (e *node) TsCreated() int64 {
	return e.tsCreated
}

func (e *node) MaxVersionNo() int {
	return e.maxVersionNo
}

func (e *node) MaxWGZeroVersionNo() int {
	return e.maxWGZeroVersionNo
}

func (e *node) Content() string {
	return e.content
}

func (e *node) CountChildren() (int, error) {
	var count int
	return count, e.db.countChildren.QueryRow(e.id).Scan(&count)
}

func (e *node) CountReleasedChildren(minTsCreated int64) (int, error) {
	var count int
	return count, e.db.countReleased.QueryRow(e.id, minTsCreated).Scan(&count)
}

func (e *node) getChildren(stmt *sql.Stmt, args ...interface{}) ([]core.DBNode, error) {

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children = []core.DBNode{}

	for rows.Next() {
		var child = &node{
			db: e.db,
		}
		err := rows.Scan(&child.id, &child.parentId, &child.slug, &child.className, &child.tsCreated, &child.maxVersionNo, &child.maxWGZeroVersionNo, &child.versionNo, &child.versionNote, &child.content, &child.tsChanged, &child.workflowGroupId)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	return children, nil
}

func (e *node) GetReleasedChildren(order core.Order, limit, offset int) ([]core.DBNode, error) {
	switch order {
	case core.AlphabeticallyAsc:
		return e.getChildren(e.db.getReleasedChildrenAlphabetically, e.id, limit, offset)
	case core.ChronologicallyDesc:
		return e.getChildren(e.db.getReleasedChildrenChronologicallyDesc, e.id, limit, offset)
	default:
		return nil, fmt.Errorf("unknown order %d", order)
	}
}

func (e *node) Versions() ([]core.DBVersionStub, error) {

	rows, err := e.db.versions.Query(e.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions = []core.DBVersionStub{}

	for rows.Next() {
		var version = &versionStub{}
		err := rows.Scan(&version.versionNo, &version.versionNote, &version.tsChanged, &version.workflowGroupId)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, nil
}

type NodeDB struct {
	*sql.DB
	calculateMWGZV                         *sql.Stmt
	countChildren                          *sql.Stmt
	countReleased                          *sql.Stmt
	getReleasedChildrenAlphabetically      *sql.Stmt
	getReleasedChildrenChronologicallyDesc *sql.Stmt
	getLatest                              *sql.Stmt
	getNode                                *sql.Stmt
	getNodeById                            *sql.Stmt
	getParentAndSlug                       *sql.Stmt
	getVersion                             *sql.Stmt
	insertNode                             *sql.Stmt
	insertVersion                          *sql.Stmt
	removeNode                             *sql.Stmt
	removeVersion                          *sql.Stmt
	setClass                               *sql.Stmt
	setMaxVersion                          *sql.Stmt
	setMWGZV                               *sql.Stmt
	setParent                              *sql.Stmt
	setSlug                                *sql.Stmt
	setWorkflowGroup                       *sql.Stmt
	versions                               *sql.Stmt
}

func NewNodeDB(db *sql.DB) *NodeDB {

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS element (
			id INTEGER PRIMARY KEY,
			parentId int(11) NOT NULL,
			slug varchar(64) NOT NULL,
			class varchar(64) NOT NULL,
			ts_created int(11) NOT NULL,
			maxWGZeroVersion int(11) NOT NULL,
			maxVersion int(11) NOT NULL,
			UNIQUE (parentId, slug)
		);
		CREATE TABLE IF NOT EXISTS version (
			id int(11) NOT NULL, /* node id */
			versionNr int(11) NOT NULL DEFAULT '0' /* auto_increment for compound primary key works only with MyISAM, which does not support transactions */,
			versionNote varchar(128) NOT NULL,
			content mediumtext NOT NULL,
			ts_changed INTEGER NOT NULL,
			workflow_group int(11) NOT NULL DEFAULT '0',
			PRIMARY KEY (id, versionNr)
		);
		`)
	if err != nil {
		panic(err)
	}

	var nodeDB = &NodeDB{}
	nodeDB.DB = db
	nodeDB.calculateMWGZV = mustPrepare(db, "SELECT COALESCE(max(versionNr), 0) FROM version WHERE version.id = ? AND version.workflow_group = 0")
	nodeDB.countChildren = mustPrepare(db, "SELECT COUNT(1) FROM element WHERE parentId = ?")
	nodeDB.countReleased = mustPrepare(db, "SELECT COUNT(1) FROM element WHERE parentId = ? AND maxWGZeroVersion > 0 AND ts_created > ?")
	nodeDB.getReleasedChildrenAlphabetically = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion, v.versionNr, v.versionNote, v.content, v.ts_changed, v.workflow_group FROM element e, version v WHERE e.parentId = ? AND e.id = v.id AND v.versionNr = e.maxWGZeroVersion ORDER BY slug LIMIT ? OFFSET ?")
	nodeDB.getReleasedChildrenChronologicallyDesc = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion, v.versionNr, v.versionNote, v.content, v.ts_changed, v.workflow_group FROM element e, version v WHERE e.parentId = ? AND e.id = v.id AND v.versionNr = e.maxWGZeroVersion ORDER BY ts_created DESC LIMIT ? OFFSET ?")
	nodeDB.getLatest = mustPrepare(db, "SELECT versionNr, versionNote, content, ts_changed, workflow_group FROM version WHERE id = ? ORDER BY versionNr DESC LIMIT 1") // in contrast to getVersion(e.maxVersion), this works if there is no version yet
	nodeDB.getNode = mustPrepare(db, "SELECT id, parentId, slug, class, ts_created, maxVersion, maxWGZeroVersion FROM element WHERE parentId = ? AND slug = ? LIMIT 1")
	nodeDB.getNodeById = mustPrepare(db, "SELECT id, parentId, slug, class, ts_created, maxVersion, maxWGZeroVersion FROM element WHERE id = ? LIMIT 1")
	nodeDB.getParentAndSlug = mustPrepare(db, "SELECT parentId, slug FROM element WHERE id = ?")
	nodeDB.getVersion = mustPrepare(db, "SELECT versionNr, versionNote, content, ts_changed, workflow_group FROM version WHERE id = ? AND versionNr = ? LIMIT 1")
	nodeDB.insertNode = mustPrepare(db, "INSERT INTO element (parentId, slug, class, ts_created, maxVersion, maxWGZeroVersion) VALUES (?, ?, ?, ?, ?, ?)")
	nodeDB.insertVersion = mustPrepare(db, "INSERT INTO version (id, versionNr, versionNote, content, ts_changed, workflow_group) VALUES (?, ?, ?, ?, ?, ?)")
	nodeDB.removeNode = mustPrepare(db, "DELETE FROM element WHERE id = ?")
	nodeDB.removeVersion = mustPrepare(db, "DELETE FROM version WHERE id = ?")
	nodeDB.setClass = mustPrepare(db, "UPDATE element SET class = ? WHERE id = ?")
	nodeDB.setMaxVersion = mustPrepare(db, "UPDATE element SET maxVersion = ? WHERE id = ?")
	nodeDB.setMWGZV = mustPrepare(db, "UPDATE element SET maxWGZeroVersion = ? WHERE id = ?")
	nodeDB.setParent = mustPrepare(db, "UPDATE element SET parentId = ? WHERE id = ?")
	nodeDB.setSlug = mustPrepare(db, "UPDATE element SET slug = ? WHERE id = ?")
	nodeDB.setWorkflowGroup = mustPrepare(db, "UPDATE version SET workflow_group = ? WHERE id = ? AND versionNr = ?")
	nodeDB.versions = mustPrepare(db, "SELECT versionNr, versionNote, ts_changed, workflow_group FROM version WHERE id = ? ORDER BY versionNr DESC")
	return nodeDB
}

func (db *NodeDB) AddVersion(e core.DBNode, content, versionNote string) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// we assume that e.maxVersionNo is up to date

	e.(*node).maxVersionNo++

	_, err = tx.Stmt(db.setMaxVersion).Exec(e.MaxVersionNo(), e.Id())
	if err != nil {
		tx.Rollback()
		return err
	}

	e.(*node).content = content
	e.(*node).tsChanged = time.Now().Unix()
	e.(*node).versionNo = e.MaxVersionNo()
	e.(*node).versionNote = versionNote

	if _, err := tx.Stmt(db.insertVersion).Exec(e.Id(), e.VersionNo(), e.VersionNote(), e.Content(), e.TsChanged(), e.WorkflowGroupId()); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *NodeDB) DeleteNode(e core.DBNode) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// copied from CountChildren()
	var childrenCount int
	if err := tx.Stmt(db.countChildren).QueryRow(e.Id()).Scan(&childrenCount); err == nil {
		if childrenCount > 0 {
			return errors.New("can't delete node with child nodes")
		}
	} else {
		return err
	}

	_, err = tx.Stmt(db.removeNode).Exec(e.Id())
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Stmt(db.removeVersion).Exec(e.Id())
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// may return sql.ErrNoRows
// we could do a database join instead of getting node and version separately, but that would triple some code
func (db *NodeDB) get(parentId int, slug string) (*node, error) {
	var e = &node{
		db: db,
	}
	return e, db.getNode.QueryRow(parentId, slug).Scan(&e.id, &e.parentId, &e.slug, &e.className, &e.tsCreated, &e.maxVersionNo, &e.maxWGZeroVersionNo)
}

func (db *NodeDB) getById(id int) (*node, error) {
	var e = &node{
		db: db,
	}
	return e, db.getNodeById.QueryRow(id).Scan(&e.id, &e.parentId, &e.slug, &e.className, &e.tsCreated, &e.maxVersionNo, &e.maxWGZeroVersionNo)
}

func (db *NodeDB) GetParentAndSlug(id int) (parentId int, slug string, err error) {
	return parentId, slug, db.getParentAndSlug.QueryRow(id).Scan(&parentId, &slug)
}

func (db *NodeDB) GetLatestNode(parentId int, slug string) (core.DBNode, error) {
	var e, err = db.get(parentId, slug)
	if err != nil {
		return nil, err
	}
	err = db.getLatest.QueryRow(e.id).Scan(&e.versionNo, &e.versionNote, &e.content, &e.tsChanged, &e.workflowGroupId)
	if err == sql.ErrNoRows {
		err = nil // return empty version, see core/node.go
	}
	return e, err
}

func (db *NodeDB) GetReleasedNode(parentId int, slug string) (core.DBNode, error) {
	var e, err = db.get(parentId, slug)
	if e == nil || err != nil {
		return e, err
	}
	// relies on correct maxWGZeroVersionNo
	return e, db.getVersion.QueryRow(e.id, e.maxWGZeroVersionNo).Scan(&e.versionNo, &e.versionNote, &e.content, &e.tsChanged, &e.workflowGroupId)
}

func (db *NodeDB) GetReleasedNodeById(id int) (core.DBNode, error) {
	var e, err = db.getById(id)
	if e == nil || err != nil {
		return e, err
	}
	// relies on correct maxWGZeroVersionNo
	return e, db.getVersion.QueryRow(e.id, e.maxWGZeroVersionNo).Scan(&e.versionNo, &e.versionNote, &e.content, &e.tsChanged, &e.workflowGroupId)
}

func (db *NodeDB) GetVersionNode(parentId int, slug string, versionNo int) (core.DBNode, error) {
	var e, err = db.get(parentId, slug)
	if err != nil {
		return nil, err
	}
	return e, db.getVersion.QueryRow(e.id, versionNo).Scan(&e.versionNo, &e.versionNote, &e.content, &e.tsChanged, &e.workflowGroupId)
}

func (db *NodeDB) InsertNode(parentId int, slug string, className string) error {
	_, err := db.insertNode.Exec(parentId, slug, className, time.Now().Unix(), 0, 0)
	return err
}

func (db *NodeDB) IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func (db *NodeDB) SetClass(e core.DBNode, className string) error {
	_, err := db.setClass.Exec(className, e.Id())
	if err == nil {
		e.(*node).className = className
	}
	return err
}

func (db *NodeDB) SetParent(e core.DBNode, parent core.DBNode) error {
	_, err := db.setParent.Exec(parent.Id(), e.Id())
	return err
}

func (db *NodeDB) SetSlug(e core.DBNode, slug string) error {
	_, err := db.setSlug.Exec(slug, e.Id())
	if err == nil {
		e.(*node).slug = slug
	}
	return err
}

func (db *NodeDB) SetWorkflowGroup(e core.DBNode, groupId int) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// update version

	_, err = tx.Stmt(db.setWorkflowGroup).Exec(groupId, e.Id(), e.VersionNo())
	if err != nil {
		tx.Rollback()
		return err
	}

	// always update node (because a version might have been revoked)

	var newMWGZV int

	if err := tx.Stmt(db.calculateMWGZV).QueryRow(e.Id()).Scan(&newMWGZV); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.Stmt(db.setMWGZV).Exec(newMWGZV, e.Id()); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	e.(*node).maxWGZeroVersionNo = newMWGZV
	e.(*node).workflowGroupId = groupId

	return nil
}
