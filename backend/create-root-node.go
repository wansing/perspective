package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var createRootNodeTmpl = tmpl(`<h1>Create root node</h1>

	<p>There is no root node yet.</p>

	<form method="post">
		<input type="submit" class="btn btn-primary" name="create" value="Create root node">
	</form>`)

func createRootNode(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	if !ctx.IsRootAdmin() {
		return errors.New("you need root admin permission")
	}

	if req.PostFormValue("create") != "" {
		if err := ctx.db.NodeDB.InsertNode(0, core.RootSlug, "html"); err == nil {
			ctx.SeeOther("/choose/1/")
			return nil
		} else {
			ctx.Danger(err)
		}
	}

	return createRootNodeTmpl.Execute(w, ctx)
}
