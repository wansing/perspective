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

func (e *EditorsDB) AssignWorkflowID(nodeID int, childrenOnly bool, workflowID int) error {
	_, err := e.assign.Exec(nodeID, childrenOnly, workflowID)
	return err
}

func (e *EditorsDB) GetAssignedWorkflowID(nodeID int, childrenOnly bool) (int, error) {
	var workflowID int
	err := e.get.QueryRow(nodeID, childrenOnly).Scan(&workflowID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return workflowID, err
}

func (e *EditorsDB) GetAllWorkflowAssignments() (map[int]map[bool]int, error) {
	res, err := e.getAll.Query()
	if err != nil {
		return nil, err
	}
	var all = make(map[int]map[bool]int)
	for res.Next() {
		var nodeID, workflowID int
		var childrenOnly bool
		if err = res.Scan(&nodeID, &childrenOnly, &workflowID); err != nil {
			return nil, err
		}
		if _, ok := all[nodeID]; !ok {
			all[nodeID] = make(map[bool]int)
		}
		all[nodeID][childrenOnly] = workflowID
	}
	return all, nil
}

func (e *EditorsDB) UnassignWorkflow(nodeID int, childrenOnly bool) error {
	_, err := e.unassign.Exec(nodeID, childrenOnly)
	return err
}
