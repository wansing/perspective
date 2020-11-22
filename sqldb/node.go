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
	workflowGroupID int
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

func (v *versionStub) WorkflowGroupID() int {
	return v.workflowGroupID
}

type node struct {
	id                 int
	parentID           int
	slug               string
	classCode          string
	tsCreated          int64
	maxVersionNo       int
	maxWGZeroVersionNo int
	version
}

func (e *node) ID() int {
	return e.id
}

func (e *node) ParentID() int {
	return e.parentID
}

func (e *node) Slug() string {
	return e.slug
}

func (e *node) ClassCode() string {
	return e.classCode
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
	getNodeByID                            *sql.Stmt
	getNodeBySlug                          *sql.Stmt
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
	nodeDB.countReleased = mustPrepare(db, "SELECT COUNT(1) FROM element WHERE parentId = ? AND maxWGZeroVersion > 0")

	nodeDB.getChildrenAlphabetically = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion FROM element e WHERE e.parentId = ? ORDER BY slug LIMIT ? OFFSET ?")
	nodeDB.getChildrenChronologicallyDesc = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion FROM element e WHERE e.parentId = ? ORDER BY ts_created DESC LIMIT ? OFFSET ?")

	nodeDB.getReleasedChildrenAlphabetically = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion, v.versionNr, v.versionNote, v.content, v.ts_changed, v.workflow_group FROM element e, version v WHERE e.parentId = ? AND e.id = v.id AND v.versionNr = e.maxWGZeroVersion ORDER BY slug LIMIT ? OFFSET ?")
	nodeDB.getReleasedChildrenChronologicallyDesc = mustPrepare(db, "SELECT e.id, e.parentId, e.slug, e.class, e.ts_created, e.maxVersion, e.maxWGZeroVersion, v.versionNr, v.versionNote, v.content, v.ts_changed, v.workflow_group FROM element e, version v WHERE e.parentId = ? AND e.id = v.id AND v.versionNr = e.maxWGZeroVersion ORDER BY ts_created DESC LIMIT ? OFFSET ?")
	nodeDB.getNodeByID = mustPrepare(db, "SELECT id, parentId, slug, class, ts_created, maxVersion, maxWGZeroVersion FROM element WHERE id = ? LIMIT 1")
	nodeDB.getNodeBySlug = mustPrepare(db, "SELECT id, parentId, slug, class, ts_created, maxVersion, maxWGZeroVersion FROM element WHERE parentId = ? AND slug = ? LIMIT 1")
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

func (db *NodeDB) AddVersion(e core.DBNode, content, versionNote string, workflowGroupID int) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	var tsChanged = time.Now().Unix()
	var versionNo = e.MaxVersionNo() + 1 // we assume that e.MaxVersionNo() is up to date

	if _, err := tx.Stmt(db.insertVersion).Exec(e.ID(), versionNo, versionNote, content, tsChanged, workflowGroupID); err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Stmt(db.setMaxVersion).Exec(versionNo, e.ID())
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

func (db *NodeDB) CountChildren(id int) (int, error) {
	var count int
	return count, db.countChildren.QueryRow(id).Scan(&count)
}

func (db *NodeDB) CountReleasedChildren(id int) (int, error) {
	var count int
	return count, db.countReleased.QueryRow(id).Scan(&count)
}

func (db *NodeDB) DeleteNode(e core.DBNode) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// copied from CountChildren()
	var childrenCount int
	if err := tx.Stmt(db.countChildren).QueryRow(e.ID()).Scan(&childrenCount); err == nil {
		if childrenCount > 0 {
			return errors.New("can't delete node with child nodes")
		}
	} else {
		return err
	}

	_, err = tx.Stmt(db.removeNode).Exec(e.ID())
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Stmt(db.removeVersion).Exec(e.ID())
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *NodeDB) GetNodeByID(id int) (core.DBNode, error) {
	var n = &node{}
	return n, db.getNodeByID.QueryRow(id).Scan(&n.id, &n.parentID, &n.slug, &n.classCode, &n.tsCreated, &n.maxVersionNo, &n.maxWGZeroVersionNo)
}

// may return sql.ErrNoRows
// we could do a database join instead of getting node and version separately, but that would triple some code
func (db *NodeDB) GetNodeBySlug(parentID int, slug string) (core.DBNode, error) {
	var e = &node{}
	return e, db.getNodeBySlug.QueryRow(parentID, slug).Scan(&e.id, &e.parentID, &e.slug, &e.classCode, &e.tsCreated, &e.maxVersionNo, &e.maxWGZeroVersionNo)
}

func (db *NodeDB) GetVersion(id int, versionNo int) (core.DBVersion, error) {
	var v = &version{}
	return v, db.getVersion.QueryRow(id, versionNo).Scan(&v.versionNo, &v.versionNote, &v.content, &v.tsChanged, &v.workflowGroupID)
}

func (db *NodeDB) GetChildren(id int, order core.Order, limit, offset int) ([]core.DBNodeVersion, error) {

	var stmt *sql.Stmt

	switch order {
	case core.AlphabeticallyAsc:
		stmt = db.getChildrenAlphabetically
	case core.ChronologicallyDesc:
		stmt = db.getChildrenChronologicallyDesc
	default:
		return nil, fmt.Errorf("unknown order %d", order)
	}

	rows, err := stmt.Query(id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children = []core.DBNodeVersion{}

	for rows.Next() {
		var child = &node{}
		err := rows.Scan(&child.id, &child.parentID, &child.slug, &child.classCode, &child.tsCreated, &child.maxVersionNo, &child.maxWGZeroVersionNo)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	return children, nil
}

func (db *NodeDB) GetReleasedChildren(id int, order core.Order, limit, offset int) ([]core.DBNodeVersion, error) {

	var stmt *sql.Stmt

	switch order {
	case core.AlphabeticallyAsc:
		stmt = db.getReleasedChildrenAlphabetically
	case core.ChronologicallyDesc:
		stmt = db.getReleasedChildrenChronologicallyDesc
	default:
		return nil, fmt.Errorf("unknown order %d", order)
	}

	rows, err := stmt.Query(id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children = []core.DBNodeVersion{}

	for rows.Next() {
		var child = &nodeVersion{
			//db: db,
		}
		err := rows.Scan(&child.id, &child.parentID, &child.slug, &child.classCode, &child.tsCreated, &child.maxVersionNo, &child.maxWGZeroVersionNo, &child.versionNo, &child.versionNote, &child.content, &child.tsChanged, &child.workflowGroupID)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	return children, nil
}

func (db *NodeDB) InsertNode(parentID int, slug string, classCode string) error {
	_, err := db.insertNode.Exec(parentID, slug, classCode, time.Now().Unix(), 0, 0)
	return err
}

func (db *NodeDB) IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func (db *NodeDB) SetClass(e core.DBNode, classCode string) error {
	_, err := db.setClass.Exec(classCode, e.ID())
	if err == nil {
		e.(*node).classCode = classCode
	}
	return err
}

func (db *NodeDB) SetParent(e core.DBNode, parent core.DBNode) error {
	_, err := db.setParent.Exec(parent.ID(), e.ID())
	return err
}

func (db *NodeDB) SetSlug(e core.DBNode, slug string) error {
	_, err := db.setSlug.Exec(slug, e.ID())
	if err == nil {
		e.(*node).slug = slug
	}
	return err
}

func (db *NodeDB) SetWorkflowGroup(n core.DBNode, v core.DBVersionStub, groupID int) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// update version

	_, err = tx.Stmt(db.setWorkflowGroup).Exec(groupID, n.ID(), v.VersionNo())
	if err != nil {
		tx.Rollback()
		return err
	}

	// always update node (because a version might have been revoked)

	var newMWGZV int

	if err := tx.Stmt(db.calculateMWGZV).QueryRow(n.ID()).Scan(&newMWGZV); err != nil {
		tx.Rollback()
		return err
	}

	if _, err := tx.Stmt(db.setMWGZV).Exec(newMWGZV, n.ID()); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if nn, ok := n.(*node); ok {
		nn.maxWGZeroVersionNo = newMWGZV
	}

	if vv, ok := v.(*version); ok {
		vv.workflowGroupID = groupID
	}

	return nil
}

func (db *NodeDB) Versions(id int) ([]core.DBVersionStub, error) {

	rows, err := db.versions.Query(id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions = []core.DBVersionStub{}

	for rows.Next() {
		var version = &versionStub{}
		err := rows.Scan(&version.versionNo, &version.versionNote, &version.tsChanged, &version.workflowGroupID)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, nil
}
