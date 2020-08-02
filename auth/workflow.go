package auth

import (
	"bytes"
	"errors"
)

type DBWorkflow interface {
	Id() int
	Name() string
	Groups() ([]int, error) // can be empty
}

type WorkflowDB interface {
	Delete(w DBWorkflow) error
	GetAllWorkflows(limit, offset int) ([]DBWorkflow, error)
	GetWorkflow(id int) (DBWorkflow, error)
	InsertWorkflow(name string) error
	UpdateWorkflow(w DBWorkflow, groups []int) error
	Writeable() bool
}

// Workflow wraps DBWorkflow and caches its Groups.
type Workflow struct {
	DBWorkflow
	groupDB      GroupDB
	groups       []Group
	groupsLoaded bool
}

// Groups shadows Workflow.DBWorkflow.Groups.
func (w *Workflow) Groups() ([]Group, error) {

	if !w.groupsLoaded {

		groupIds, err := w.DBWorkflow.Groups()
		if err != nil {
			return nil, err
		}

		for _, groupId := range groupIds {
			group, err := w.groupDB.GetGroup(groupId)
			if err != nil {
				return nil, err
			}
			w.groups = append(w.groups, group)
		}
	}

	return w.groups, nil
}

func (w *Workflow) String() string {
	var buf bytes.Buffer
	buf.WriteString(w.Name())
	groups, _ := w.Groups()
	if len(groups) > 0 {
		buf.WriteString(" (")
		for i, group := range groups {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(group.Name())
		}
		buf.WriteString(")")
	}
	return buf.String()
}

// GetAllWorkflows shadows AuthDB.WorkflowDB.GetAllWorkflows.
func (a *AuthDB) GetAllWorkflows(limit, offset int) ([]*Workflow, error) {
	var workflows, err = a.WorkflowDB.GetAllWorkflows(limit, offset)
	var result = make([]*Workflow, len(workflows))
	for i := range workflows {
		result[i] = &Workflow{
			DBWorkflow: workflows[i],
			groupDB:    a.GroupDB,
		}
	}
	return result, err
}

// GetWorkflow shadows AuthDB.WorkflowDB.GetWorkflow.
func (a *AuthDB) GetWorkflow(id int) (*Workflow, error) {
	var w, err = a.WorkflowDB.GetWorkflow(id)
	return &Workflow{
		DBWorkflow: w,
		groupDB:    a.GroupDB,
	}, err
}

// UpdateWorkflow shadows AuthDB.WorkflowDB.UpdateWorkflow.
func (a *AuthDB) UpdateWorkflow(w DBWorkflow, groupIds []int) error {
	for _, groupId := range groupIds {
		if groupId == 0 {
			return errors.New("all users is not allowed in workflow")
		}
	}
	return a.WorkflowDB.UpdateWorkflow(w, groupIds)
}
