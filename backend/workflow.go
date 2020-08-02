package backend

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/auth"
)

var workflowTmpl = tmpl(`<h1>Workflow &raquo;{{ .Selected.Name }}&laquo;</h1>

	<form method="post">

		{{ range $i, $e := .Selected.Groups }}
			<div class="form-group">
				<select class="form-control" name="groups[]">
					<option value=""></option>
					{{ range $.AllGroups }}
						<option {{ if eq (index $.Selected.Groups $i).Id .Id }}selected="selected"{{ end }} value="{{ .Id }}">{{ .Name }}</option>
					{{ end }}
				</select>
			</div>
		{{ end }}

		<div class="form-group">
			<select class="form-control" name="groups[]">
				<option value="" selected="selected"></option>
				{{ range $.AllGroups }}
					<option value="{{ .Id }}">{{ .Name }}</option>
				{{ end }}
			</select>
		</div>

		<button class="btn btn-primary" type="submit">Save</button>
	</form>

	<a href="/backend/workflow/{{ .Selected.Id }}/delete">`)

type workflowData struct {
	*Route
	Selected *auth.Workflow
}

func (data *workflowData) AllGroups() ([]auth.Group, error) {
	return data.db.Auth.GetAllGroups(10000, 0) // assuming there are not more than 10k groups
}

func workflow(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	if !r.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	selectedId, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		return err
	}

	selected, err := r.db.Auth.GetWorkflow(selectedId)
	if err != nil {
		return err
	}

	if req.Method == http.MethodPost {

		req.ParseForm()

		var groupIds = []int{}
		for _, groupIdStr := range req.PostForm["groups[]"] {
			if groupIdStr == "" {
				continue
			}
			groupId, err := strconv.Atoi(groupIdStr)
			if err != nil {
				return err
			}
			groupIds = append(groupIds, groupId)
		}

		if err := r.db.Auth.UpdateWorkflow(selected.DBWorkflow, groupIds); err != nil {
			return err
		}

		r.Success("workflow %s has been updated", selected.Name())
		r.SeeOther("/workflow/%d", selected.Id())
		return nil
	}

	return workflowTmpl.Execute(w, &workflowData{
		Route:    r,
		Selected: selected,
	})
}
