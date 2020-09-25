package backend

import (
	"errors"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var groupsTmpl = tmpl(`<h1>Groups</h1>

	<ul>
		{{ range .Groups }}
			<li>{{ GroupLink . }}</li>
		{{ end }}
	</ul>

	<h2>Create Group</h2>

	<form method="post" class="form-inline">
		<div class="form-group">
			<input class="form-control" name="group_name" placeholder="Group name">
			<button type="submit" class="btn btn-primary mx-sm-3" name="submit_add">Create group</button>
		</div>
	</form>`)

type groupsData struct {
	*Route
}

func (data *groupsData) Groups() ([]core.DBGroup, error) {
	return data.db.GetAllGroups(10000, 0) // assuming there are not more than 10k groups
}

func groups(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	if !r.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	if req.Method == http.MethodPost {

		newGroupName := strings.TrimSpace(req.PostFormValue("group_name"))

		if newGroupName == "" {
			return errors.New("missing name")
		}

		if err := r.db.InsertGroup(newGroupName); err != nil {
			return err
		}

		r.Success("group %s has been created", newGroupName)
		r.SeeOther("/groups")
		return nil
	}

	return groupsTmpl.Execute(w, &groupsData{
		Route: r,
	})
}
