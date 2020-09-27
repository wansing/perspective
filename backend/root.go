package backend

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func root(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {
	if ctx.LoggedIn() {
		ctx.SeeOther("/choose/1/") // trailing slash seems to be relevant
	} else {
		ctx.SeeOther("/login")
	}
	return nil
}
