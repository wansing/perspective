package core

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
	groups       []DBGroup
	groupsLoaded bool
}

// Groups shadows DBWorkflow.Groups.
func (w *Workflow) Groups() ([]DBGroup, error) {

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

// GetAllWorkflows shadows WorkflowDB.GetAllWorkflows.
func (c *CoreDB) GetAllWorkflows(limit, offset int) ([]*Workflow, error) {
	var workflows, err = c.WorkflowDB.GetAllWorkflows(limit, offset)
	var result = make([]*Workflow, len(workflows))
	for i := range workflows {
		result[i] = &Workflow{
			DBWorkflow: workflows[i],
			groupDB:    c.GroupDB,
		}
	}
	return result, err
}

// GetWorkflow shadows WorkflowDB.GetWorkflow.
func (c *CoreDB) GetWorkflow(id int) (*Workflow, error) {
	var w, err = c.WorkflowDB.GetWorkflow(id)
	return &Workflow{
		DBWorkflow: w,
		groupDB:    c.GroupDB,
	}, err
}

// UpdateWorkflow shadows WorkflowDB.UpdateWorkflow.
func (c *CoreDB) UpdateWorkflow(w DBWorkflow, groupIds []int) error {
	for _, groupId := range groupIds {
		if groupId == 0 {
			return errors.New("all users is not allowed in workflow")
		}
	}
	return c.WorkflowDB.UpdateWorkflow(w, groupIds)
}
