package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var moveTmpl = tmpl(`<h1>Move {{ .Selected.HrefPath }}</h1>

	<p>
		<a class="btn btn-secondary" href="choose/1{{ .Selected.HrefPath }}">Cancel</a>
	</p>

	<form method="post">
		<div class="form-group row">
			<label class="col-sm-2 col-form-label">Current location</label>
			<div class="col-sm-10">
				<input class="form-control-plaintext" readonly value="{{ .Selected.Parent.HrefPath }}">
			</div>
		</div>
		<div class="form-group row">
			<label class="col-sm-2 col-form-label">New location</label>
			<div class="col-sm-10">
				<input class="form-control" name="parentUrl" value="{{ .ParentUrl }}">
			</div>
		</div>
		<div class="form-group row">
			<label class="col-sm-2 col-form-label">Slug</label>
			<div class="col-sm-10">
				<input class="form-control-plaintext" readonly value="{{ .Selected.Slug }}">
			</div>
		</div>
		<button type="submit" class="btn btn-primary">Move</button>
	</form>`)

type moveData struct {
	*Route
	ParentUrl string
	Selected  *core.Node
}

func move(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	// check delete permission

	if selected.Parent == nil {
		return errors.New("can't move root")
	}

	if err = r.User.RequirePermission(core.Remove, selected.Parent); err != nil {
		return err
	}

	var parentUrl = selected.Parent.HrefPath() // default value

	// move

	if req.Method == http.MethodPost {

		parentUrl = req.PostFormValue("parentUrl")

		newParent, err := r.Open(selected.MakeAbsolute(parentUrl))
		if err != nil {
			return err
		}

		// check create permission

		if err = r.User.RequirePermission(core.Create, newParent); err != nil {
			return err
		}

		if err = r.db.SetParent(selected, newParent); err == nil {
			r.SeeOther("/choose/1%s", selected.HrefPath())
			return nil
		} else {
			r.Danger(err)
		}
	}

	return moveTmpl.Execute(w, &moveData{
		Route:     r,
		ParentUrl: parentUrl,
		Selected:  selected,
	})
}
