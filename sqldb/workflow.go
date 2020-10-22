package sqldb

import (
	"database/sql"
	"fmt"

	"github.com/wansing/perspective/core"
)

type workflow struct {
	db           *WorkflowDB // required for lazy loading
	id           int
	name         string
	groups       []int // without 0
	groupsLoaded bool  // lazy loading
}

func (w *workflow) ID() int {
	return w.id
}

func (w *workflow) Name() string {
	return w.name
}

func (w *workflow) Groups() ([]int, error) {

	if !w.groupsLoaded {

		w.groups = []int{}

		rows, err := w.db.groups.Query(w.id)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var groupID int
			if err = rows.Scan(&groupID); err != nil {
				return nil, err
			}
			w.groups = append(w.groups, groupID)
		}

		w.groupsLoaded = true
	}

	return w.groups, nil
}

func (db *WorkflowDB) Delete(w core.DBWorkflow) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Stmt(db.clear).Exec(w.ID())
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Stmt(db.delete).Exec(w.ID())
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

type WorkflowDB struct {
	*sql.DB
	clear  *sql.Stmt
	delete *sql.Stmt
	get    *sql.Stmt
	getAll *sql.Stmt
	groups *sql.Stmt
	insert *sql.Stmt
	push   *sql.Stmt
}

func NewWorkflowDB(db *sql.DB) *WorkflowDB {

	db.Exec(`
		CREATE TABLE IF NOT EXISTS workflow (
			workflowId INTEGER PRIMARY KEY,
			workflowName varchar(32) NOT NULL,
			UNIQUE (workflowName)
		);
		CREATE TABLE IF NOT EXISTS workflow_position (
			workflowId int(11) NOT NULL,
			position int(11) NOT NULL,
			groupId int(11) NOT NULL,
			PRIMARY KEY (workflowId,position)
		);`)

	var workflowDB = &WorkflowDB{}
	workflowDB.DB = db
	workflowDB.clear = mustPrepare(db, "DELETE FROM workflow_position WHERE workflowId = ?")
	workflowDB.delete = mustPrepare(db, "DELETE FROM workflow WHERE workflowId = ?")
	workflowDB.get = mustPrepare(db, "SELECT workflowName FROM workflow WHERE workflowId = ? LIMIT 1")
	workflowDB.getAll = mustPrepare(db, "SELECT workflowId, workflowName FROM workflow ORDER BY workflowName LIMIT ? OFFSET ?")
	workflowDB.groups = mustPrepare(db, "SELECT groupId FROM workflow_position WHERE workflowId = ? ORDER BY position")
	workflowDB.insert = mustPrepare(db, "INSERT INTO workflow (workflowName) VALUES (?)")
	workflowDB.push = mustPrepare(db, "INSERT INTO workflow_position (workflowId, position, groupId) VALUES (?, ?, ?)")
	return workflowDB
}

func (db *WorkflowDB) Writeable() bool {
	return true
}

func (db *WorkflowDB) GetWorkflow(id int) (core.DBWorkflow, error) {
	var w = &workflow{
		db: db,
		id: id,
	}
	return w, db.get.QueryRow(id).Scan(&w.name)
}

func (db *WorkflowDB) GetAllWorkflows(limit, offset int) ([]core.DBWorkflow, error) {

	rows, err := db.getAll.Query(limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all = []core.DBWorkflow{}

	for rows.Next() {
		var id int
		var name string
		err = rows.Scan(&id, &name)
		if err != nil {
			return nil, err
		}
		all = append(all, &workflow{
			db:   db,
			id:   id,
			name: name,
		})
	}

	return all, nil
}

func (db *WorkflowDB) InsertWorkflow(name string) error {
	_, err := db.insert.Exec(name)
	return err
}

func (db *WorkflowDB) UpdateWorkflow(w core.DBWorkflow, groups []int) error {

	for _, group := range groups {
		if group <= 0 {
			return fmt.Errorf("invalid group id %d", group)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Stmt(db.clear).Exec(w.ID())
	if err != nil {
		tx.Rollback()
		return err
	}

	for position, group := range groups {
		_, err = tx.Stmt(db.push).Exec(w.ID(), position, group)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
