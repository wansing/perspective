package backend

import (
	"errors"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
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
	*context
}

func (data *workflowsData) GetAllWorkflows() ([]*core.Workflow, error) {
	return data.db.GetAllWorkflows(1000, 0) // assuming there are not more than 1k workflows
}

func workflows(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	if !ctx.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	if req.Method == http.MethodPost {

		newWorkflowName := strings.TrimSpace(req.PostFormValue("workflow_name"))

		if newWorkflowName == "" {
			return errors.New("missing workflow name")
		}

		if err := ctx.db.InsertWorkflow(newWorkflowName); err != nil {
			return err
		}

		ctx.Success("workflow %s has been created", newWorkflowName)
		ctx.SeeOther("/workflows")
		return nil
	}

	return workflowsTmpl.Execute(w, &workflowsData{
		context: ctx,
	})
}
