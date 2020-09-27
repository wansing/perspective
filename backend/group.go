package backend

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
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
	*context
	Selected core.DBGroup
}

func (data *groupData) Members() ([]core.DBUser, error) {

	var memberIds, err = data.Selected.Members()
	if err != nil {
		return nil, err
	}

	var members = []core.DBUser{}
	for memberId := range memberIds { // map: group id -> interface{}
		member, err := data.db.GetUser(memberId)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return members, nil
}

func group(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	if !ctx.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	selectedId, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		return err
	}

	selected, err := ctx.db.GetGroup(selectedId)
	if err != nil {
		return err
	}

	if req.Method == http.MethodPost {

		if addUserId := req.PostFormValue("user_id"); addUserId != "" {

			userId, err := strconv.Atoi(addUserId)
			if err != nil {
				ctx.Danger(err)
				return nil
			}

			user, err := ctx.db.GetUser(userId)
			if err != nil {
				return err
			}

			if err = ctx.db.GroupDB.Join(selected, user); err != nil {
				return err
			}

			ctx.Success("user %s has been added to group %s", user.Name(), selected.Name())
			ctx.SeeOther("/group/%d", selected.Id())
			return nil
		}
	}

	return groupTmpl.Execute(w, &groupData{
		context:  ctx,
		Selected: selected,
	})
}
