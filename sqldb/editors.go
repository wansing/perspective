package sqldb

import (
	"database/sql"
)

type EditorsDB struct {
	db       *sql.DB
	assign   *sql.Stmt
	get      *sql.Stmt
	getAll   *sql.Stmt
	unassign *sql.Stmt
}

func NewEditorsDB(db *sql.DB) *EditorsDB {

	db.Exec(`
		CREATE TABLE IF NOT EXISTS element_workflow (
			elementId int(11) NOT NULL,
			childrenOnly bool NOT NULL, -- whether this affects only the children
			workflowId int(11) NOT NULL,
			PRIMARY KEY (elementId, childrenOnly)
		);`)

	var editorsDB = &EditorsDB{}
	editorsDB.db = db
	editorsDB.assign = mustPrepare(db, "INSERT OR IGNORE INTO element_workflow (elementId, childrenOnly, workflowId) VALUES (?, ?, ?)")
	editorsDB.get = mustPrepare(db, "SELECT workflowId FROM element_workflow WHERE elementId = ? AND childrenOnly = ? LIMIT 1")
	editorsDB.getAll = mustPrepare(db, "SELECT elementId, childrenOnly, workflowId FROM element_workflow")
	editorsDB.unassign = mustPrepare(db, "DELETE FROM element_workflow WHERE elementId = ? AND childrenOnly = ?") // "LIMIT 1" is not working in SQLite
	return editorsDB
}

func (e *EditorsDB) AssignWorkflowId(nodeId int, childrenOnly bool, workflowId int) error {
	_, err := e.assign.Exec(nodeId, childrenOnly, workflowId)
	return err
}

func (e *EditorsDB) GetAssignedWorkflowId(nodeId int, childrenOnly bool) (int, error) {
	var workflowId int
	err := e.get.QueryRow(nodeId, childrenOnly).Scan(&workflowId)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return workflowId, err
}

func (e *EditorsDB) GetAllWorkflowAssignments() (map[int]map[bool]int, error) {
	res, err := e.getAll.Query()
	if err != nil {
		return nil, err
	}
	var all = make(map[int]map[bool]int)
	for res.Next() {
		var nodeId, workflowId int
		var childrenOnly bool
		if err = res.Scan(&nodeId, &childrenOnly, &workflowId); err != nil {
			return nil, err
		}
		if _, ok := all[nodeId]; !ok {
			all[nodeId] = make(map[bool]int)
		}
		all[nodeId][childrenOnly] = workflowId
	}
	return all, nil
}

func (e *EditorsDB) UnassignWorkflow(nodeId int, childrenOnly bool) error {
	_, err := e.unassign.Exec(nodeId, childrenOnly)
	return err
}
