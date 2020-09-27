package backend

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func logout(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {
	ctx.Logout()
	ctx.Success("Goodbye")
	ctx.SeeOther("/login")
	return nil
}
