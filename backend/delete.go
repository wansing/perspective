package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var deleteTmpl = tmpl(`<h1>Delete {{ .Selected.Location }}</h1>

	<p>
		<a class="btn btn-secondary" href="choose/1{{ .Selected.Location }}">Cancel</a>
	</p>

	<form method="post">
		<input type="submit" class="btn btn-primary" name="delete" value="Delete">
	</form>`)

type deleteData struct {
	*context
	Selected *core.Node
}

func del(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selected, err := ctx.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	// check permission

	if selected.Parent == nil {
		return errors.New("can't delete root")
	}

	if err = selected.Parent.RequirePermission(core.Remove, ctx.User); err != nil {
		return err
	}

	// delete

	if req.PostFormValue("delete") != "" {
		if err := ctx.db.DeleteNode(selected); err == nil {
			ctx.SeeOther("/choose/1%s", selected.Parent.Location())
			return nil
		} else {
			ctx.Danger(err)
		}
	}

	return deleteTmpl.Execute(w, &deleteData{
		context:  ctx,
		Selected: selected,
	})
}
