package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var deleteTmpl = tmpl(`<h1>Delete {{ .Selected.HrefPath }}</h1>

	<p>
		<a class="btn btn-secondary" href="choose/1{{ .Selected.HrefPath }}">Cancel</a>
	</p>

	<form method="post">
		<input type="submit" class="btn btn-primary" name="delete" value="Delete">
	</form>`)

type deleteData struct {
	*Route
	Selected *core.Node
}

func del(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	// check permission

	if selected.Parent == nil {
		return errors.New("can't delete root")
	}

	if err = r.User.RequirePermission(core.Remove, selected.Parent); err != nil {
		return err
	}

	// delete

	if req.PostFormValue("delete") != "" {
		if err := r.db.DeleteNode(selected); err == nil {
			r.SeeOther("/choose/1%s", selected.Parent.HrefPath())
			return nil
		} else {
			r.Danger(err)
		}
	}

	return deleteTmpl.Execute(w, &deleteData{
		Route:    r,
		Selected: selected,
	})
}
