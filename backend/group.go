package backend

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/auth"
)

var groupTmpl = tmpl(`<h1>Group &raquo;{{ .Selected.Name }}&laquo;</h1>

	<h2>Members</h2>

	<ul>
		{{ range .Members }}
			<li>{{ UserLink . }}</li>
		{{ else }}
			No members.
		{{ end }}
	</ul>

	<h2>Add member</h2>

	<form method="post" class="form-inline">
		<div class="form-group">
			<input type="number" class="form-control" name="user_id" placeholder="User ID">
			<button type="submit" class="btn btn-primary mx-sm-3" name="submit_add">Add user to group</button>
		</div>
	</form>`)

type groupData struct {
	*Route
	Selected auth.Group
}

func (data *groupData) Members() ([]auth.User, error) {

	var memberIds, err = data.Selected.Members()
	if err != nil {
		return nil, err
	}

	var members = []auth.User{}
	for memberId := range memberIds { // map: group id -> interface{}
		member, err := data.db.Auth.GetUser(memberId)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, nil
}

func group(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	if !r.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	selectedId, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		return err
	}

	selected, err := r.db.Auth.GetGroup(selectedId)
	if err != nil {
		return err
	}

	if req.Method == http.MethodPost {

		if addUserId := req.PostFormValue("user_id"); addUserId != "" {

			userId, err := strconv.Atoi(addUserId)
			if err != nil {
				r.Danger(err)
				return nil
			}

			user, err := r.db.Auth.GetUser(userId)
			if err != nil {
				return err
			}

			if err = r.db.Auth.GroupDB.Join(selected, user); err != nil {
				return err
			}

			r.Success("user %s has been added to group %s", user.Name(), selected.Name())
			r.SeeOther("/group/%d", selected.Id())
			return nil
		}
	}

	return groupTmpl.Execute(w, &groupData{
		Route:    r,
		Selected: selected,
	})
}
