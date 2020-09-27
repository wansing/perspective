package backend

import (
	"errors"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var usersTmpl = tmpl(`<h1>Users</h1>

	<ul>
		{{ range .Users }}
			<li>{{ UserLink . }}</li>
		{{ end }}
	</ul>

	<h2>Create User</h2>

	<form method="post" class="form-inline">
		<div class="form-group">
			<input type="email" class="form-control" name="mail_user" placeholder="Email address">
			<button type="submit" class="btn btn-primary mx-sm-3" name="submit_add">Create user</button>
		</div>
	</div>`)

type usersData struct {
	*context
}

func (data *usersData) Users() ([]core.DBUser, error) {
	return data.db.GetAllUsers(100000, 0) // assuming there are not more than 100k users
}

func users(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	if !ctx.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	if req.Method == http.MethodPost {

		newUserMail := strings.TrimSpace(req.PostFormValue("mail_user"))

		if newUserMail == "" {
			return errors.New("missing email address")
		}

		if _, err := ctx.db.InsertUser(newUserMail); err != nil {
			return err
		}

		ctx.Success("user %s has been created", newUserMail)
		ctx.SeeOther("/users")
		return nil
	}

	return usersTmpl.Execute(w, &usersData{
		context: ctx,
	})
}
