package backend

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var userTmpl = tmpl(`<h1>User &raquo;{{ .Selected.Name }}&laquo;</h1>

	<h2>Groups</h2>

	<ul>
		{{ range .Groups }}
			<li>{{ GroupLink . }}</li>
		{{ end }}
	</ul>

	<h2>Change Password</h2>

	<form method="post">

		<div class="form-group row">
			<label class="col-sm-6 col-form-label">Current password</label>
			<div class="col-sm-6">
				<input type="password" class="form-control" name="old">
			</div>
		</div>

		<div class="form-group row">
			<label class="col-sm-6 col-form-label">New password</label>
			<div class="col-sm-6">
				<input type="password" class="form-control" name="new1">
			</div>
		</div>

		<div class="form-group row">
			<label class="col-sm-6 col-form-label">Repeat new password</label>
			<div class="col-sm-6">
				<input type="password" class="form-control" name="new2">
			</div>
		</div>

		<button type="submit" class="btn btn-primary" name="submit_add">Change password</button>

	</form>`)

type userData struct {
	*context
	Selected core.DBUser
}

func (data *userData) Groups() ([]core.DBGroup, error) {
	return data.db.GetGroupsOf(data.Selected)
}

func user(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selectedID, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		return err
	}

	selected, err := ctx.db.GetUser(selectedID)
	if err != nil {
		return err
	}

	if !(ctx.IsRootAdmin() || selected.ID() == ctx.User.ID()) {
		return errors.New("unauthorized")
	}

	if req.Method == http.MethodPost {

		var new1 = req.PostFormValue("new1")
		var new2 = req.PostFormValue("new2")

		if new1 != new2 {
			return errors.New("new passwords don't match")
		}

		if strings.TrimSpace(new1) == "" {
			return errors.New("new password is empty") // we could use zxcvbn instead, or leave it to the UserDB
		}

		if err = ctx.db.ChangePassword(selected, req.PostFormValue("old"), new1); err != nil {
			return err
		}

		ctx.Success("password of %s has been changed", selected.Name())
		ctx.SeeOther("/user/%d", selected.ID())
		return nil
	}

	return userTmpl.Execute(w, &userData{
		context:  ctx,
		Selected: selected,
	})
}
