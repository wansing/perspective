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
	id                 int
	parentId           int
	slug               string
	className          string
	tsCreated          int64
	maxVersionNo       int
	maxWGZeroVersionNo int
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

type version struct {
	versionStub
	content string
}

func (v *version) Content() string {
	return v.content
}

type nodeVersion struct {
	node
	version
}

type NodeDB struct {
	*sql.DB
	calculateMWGZV                         *sql.Stmt
	countChildren                          *sql.Stmt
	countReleased                          *sql.Stmt
	getChildrenAlphabetically              *sql.Stmt
	getChildrenChronologicallyDesc         *sql.Stmt
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

	nodeDB.getChildrenAlphabetically = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion FROM element e WHERE e.parentId = ? ORDER BY slug LIMIT ? OFFSET ?")
	nodeDB.getChildrenChronologicallyDesc = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion FROM element e WHERE e.parentId = ? ORDER BY ts_created DESC LIMIT ? OFFSET ?")

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

func (db *NodeDB) AddVersion(e core.DBNode, content, versionNote string, workflowGroupId int) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	var tsChanged = time.Now().Unix()
	var versionNo = e.MaxVersionNo() + 1 // we assume that e.MaxVersionNo() is up to date

	if _, err := tx.Stmt(db.insertVersion).Exec(e.Id(), versionNo, versionNote, content, tsChanged, workflowGroupId); err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Stmt(db.setMaxVersion).Exec(versionNo, e.Id())
	if err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if en, ok := e.(*node); ok {
		en.maxVersionNo = versionNo
	}
	if ev, ok := e.(*nodeVersion); ok {
		ev.content = content
		ev.tsChanged = tsChanged
		ev.versionNo = versionNo
		ev.versionNote = versionNote
	}

	return nil
}

func (db *NodeDB) CountChildren(n core.DBNode) (int, error) {
	var count int
	return count, db.countChildren.QueryRow(n.Id()).Scan(&count)
}

func (db *NodeDB) CountReleasedChildren(n core.DBNode, minTsCreated int64) (int, error) {
	var count int
	return count, db.countReleased.QueryRow(n.Id(), minTsCreated).Scan(&count)
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
func (db *NodeDB) GetNode(parentId int, slug string) (core.DBNode, error) {
	var e = &node{}
	return e, db.getNode.QueryRow(parentId, slug).Scan(&e.id, &e.parentId, &e.slug, &e.className, &e.tsCreated, &e.maxVersionNo, &e.maxWGZeroVersionNo)
}

func (db *NodeDB) GetParentAndSlug(id int) (parentId int, slug string, err error) {
	return parentId, slug, db.getParentAndSlug.QueryRow(id).Scan(&parentId, &slug)
}

func (db *NodeDB) GetReleasedNodeById(id int) (core.DBNode, core.DBVersion, error) {
	var n = &node{}
	if err := db.getNodeById.QueryRow(id).Scan(&n.id, &n.parentId, &n.slug, &n.className, &n.tsCreated, &n.maxVersionNo, &n.maxWGZeroVersionNo); err != nil {
		return nil, nil, err
	}
	var v = &version{}
	// relies on correct maxWGZeroVersionNo
	return n, v, db.getVersion.QueryRow(n.id, n.maxWGZeroVersionNo).Scan(&v.versionNo, &v.versionNote, &v.content, &v.tsChanged, &v.workflowGroupId)
}

func (db *NodeDB) GetVersion(n core.DBNode, versionNo int) (core.DBVersion, error) {
	var v = &version{}
	return v, db.getVersion.QueryRow(n.Id(), versionNo).Scan(&v.versionNo, &v.versionNote, &v.content, &v.tsChanged, &v.workflowGroupId)
}

func (db *NodeDB) GetChildren(parent core.DBNode, order core.Order, limit, offset int) ([]core.DBNode, error) {
	switch order {
	case core.AlphabeticallyAsc:
		return db.getChildren(db.getChildrenAlphabetically, parent.Id(), limit, offset)
	case core.ChronologicallyDesc:
		return db.getChildren(db.getChildrenChronologicallyDesc, parent.Id(), limit, offset)
	default:
		return nil, fmt.Errorf("unknown order %d", order)
	}
}

func (db *NodeDB) GetReleasedChildren(parent core.DBNode, order core.Order, limit, offset int) ([]core.DBNodeVersion, error) {
	switch order {
	case core.AlphabeticallyAsc:
		return db.getChildrenNV(db.getReleasedChildrenAlphabetically, parent.Id(), limit, offset)
	case core.ChronologicallyDesc:
		return db.getChildrenNV(db.getReleasedChildrenChronologicallyDesc, parent.Id(), limit, offset)
	default:
		return nil, fmt.Errorf("unknown order %d", order)
	}
}

func (db *NodeDB) getChildren(stmt *sql.Stmt, args ...interface{}) ([]core.DBNode, error) {

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children = []core.DBNode{}

	for rows.Next() {
		var child = &node{
			//db: db,
		}
		err := rows.Scan(&child.id, &child.parentId, &child.slug, &child.className, &child.tsCreated, &child.maxVersionNo, &child.maxWGZeroVersionNo)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	return children, nil
}

func (db *NodeDB) getChildrenNV(stmt *sql.Stmt, args ...interface{}) ([]core.DBNodeVersion, error) {

	rows, err := stmt.Query(args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children = []core.DBNodeVersion{}

	for rows.Next() {
		var child = &nodeVersion{
			//db: db,
		}
		err := rows.Scan(&child.id, &child.parentId, &child.slug, &child.className, &child.tsCreated, &child.maxVersionNo, &child.maxWGZeroVersionNo, &child.versionNo, &child.versionNote, &child.content, &child.tsChanged, &child.workflowGroupId)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	return children, nil
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

func (db *NodeDB) SetWorkflowGroup(e core.DBNodeVersionStub, groupId int) error {

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

	// TODO what if type(e) == version?
	if ev, ok := e.(*nodeVersion); ok {
		ev.maxWGZeroVersionNo = newMWGZV
		ev.workflowGroupId = groupId
	}

	return nil
}

func (db *NodeDB) Versions(n core.DBNode) ([]core.DBVersionStub, error) {

	rows, err := db.versions.Query(n.Id())
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
