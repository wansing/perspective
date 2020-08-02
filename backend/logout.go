package backend

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func logout(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {
	r.Logout()
	r.Success("Goodbye")
	r.SeeOther("/login")
	return nil
}
