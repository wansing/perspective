package backend

import (
	"errors"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/auth"
)

var workflowsTmpl = tmpl(`<h1>Workflows</h1>

	<ul>
		{{ range .GetAllWorkflows }}
			<li>{{ WorkflowLinkLong . }}</li>
		{{ end }}
	</ul>

	<h2>Create Workflow</h2>

	<form method="post" class="form-inline">
		<div class="form-group">
			<input class="form-control" name="workflow_name" placeholder="Workflow name">
			<button type="submit" class="btn btn-primary mx-sm-3" name="submit_add">Create workflow</button>
		</div>
	</form>`)

type workflowsData struct {
	*Route
}

func (data *workflowsData) GetAllWorkflows() ([]*auth.Workflow, error) {
	return data.db.Auth.GetAllWorkflows(1000, 0) // assuming there are not more than 1k workflows
}

func workflows(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	if !r.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	if req.Method == http.MethodPost {

		newWorkflowName := strings.TrimSpace(req.PostFormValue("workflow_name"))

		if newWorkflowName == "" {
			return errors.New("missing workflow name")
		}

		if err := r.db.Auth.InsertWorkflow(newWorkflowName); err != nil {
			return err
		}

		r.Success("workflow %s has been created", newWorkflowName)
		r.SeeOther("/workflow")
		return nil
	}

	return workflowsTmpl.Execute(w, &workflowsData{
		Route: r,
	})
}
