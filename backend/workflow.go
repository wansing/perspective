package backend

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var workflowTmpl = tmpl(`<h1>Workflow &raquo;{{ .Selected.Name }}&laquo;</h1>

	<form method="post">

		{{ range $i, $e := .Selected.Groups }}
			<div class="form-group">
				<select class="form-control" name="groups[]">
					<option value=""></option>
					{{ range $.AllGroups }}
						<option {{ if eq (index $.Selected.Groups $i).ID .ID }}selected="selected"{{ end }} value="{{ .ID }}">{{ .Name }}</option>
					{{ end }}
				</select>
			</div>
		{{ end }}

		<div class="form-group">
			<select class="form-control" name="groups[]">
				<option value="" selected="selected"></option>
				{{ range $.AllGroups }}
					<option value="{{ .ID }}">{{ .Name }}</option>
				{{ end }}
			</select>
		</div>

		<button class="btn btn-primary" type="submit">Save</button>
	</form>

	<a href="workflow/{{ .Selected.ID }}/delete">`)

type workflowData struct {
	*context
	Selected *core.Workflow
}

func (data *workflowData) AllGroups() ([]core.DBGroup, error) {
	return data.db.GetAllGroups(10000, 0) // assuming there are not more than 10k groups
}

func workflow(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	if !ctx.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	selectedID, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		return err
	}

	selected, err := ctx.db.GetWorkflow(selectedID)
	if err != nil {
		return err
	}

	if req.Method == http.MethodPost {

		req.ParseForm()

		var groupIDs = []int{}
		for _, groupIDStr := range req.PostForm["groups[]"] {
			if groupIDStr == "" {
				continue
			}
			groupID, err := strconv.Atoi(groupIDStr)
			if err != nil {
				return err
			}
			groupIDs = append(groupIDs, groupID)
		}

		if err := ctx.db.UpdateWorkflow(selected.DBWorkflow, groupIDs); err != nil {
			return err
		}

		ctx.Success("workflow %s has been updated", selected.Name())
		ctx.SeeOther("/workflow/%d", selected.ID())
		return nil
	}

	return workflowTmpl.Execute(w, &workflowData{
		context:  ctx,
		Selected: selected,
	})
}
