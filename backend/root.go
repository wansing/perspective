package backend

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func root(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {
	if r.LoggedIn() {
		r.SeeOther("/choose/1/") // trailing slash seems to be relevant
	} else {
		r.SeeOther("/login")
	}
	return nil
}
