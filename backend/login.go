package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

var ErrLogin = errors.New("wrong username or password")

var loginTmpl = tmpl(`<h1>Login</h1>
	<form method="post" style="max-width: 20rem; margin: auto;">
		<div class="form-group">
			<label>E-Mail</label>
			<input type="text" class="form-control" name="email" value="{{ .Email }}" required autofocus>
		</div>
		<div class="form-group">
			<label>Password</label>
			<input type="password" class="form-control" name="password" required>
		</div>
		<div class="form-group">
			<button type="submit" class="btn btn-primary" name="login">Login</button>
		</div>
	</form>`)

type loginData struct {
	*Route
	Email string
}

func login(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	var email string

	if req.Method == http.MethodPost {

		email = req.PostFormValue("email")
		password := req.PostFormValue("password")

		err := r.Login(email, password)
		if err == nil {
			r.SeeOther("/")
			return nil
		} else {
			r.Danger(ErrLogin)
			// keep POST data for email field
		}
	}

	return loginTmpl.Execute(w, &loginData{
		Route: r,
		Email: email,
	})
}
