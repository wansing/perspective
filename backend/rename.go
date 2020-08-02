package backend

import (
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var renameTmpl = tmpl(`<h1>Rename {{ .Selected.HrefPath }}</h1>

		<p>
			<a class="btn btn-secondary" href="{{ HrefChoose .Selected 0 }}">Cancel</a>
		</p>

		<form method="post">
			<div class="form-group row">
				<label class="col-sm-2 col-form-label">Location</label>
				<div class="col-sm-10">
					<input class="form-control-plaintext" readonly value="{{ .Selected.Parent.HrefPath }}">
				</div>
			</div>
			<div class="form-group row">
				<label class="col-sm-2 col-form-label">Current slug</label>
				<div class="col-sm-10">
					<input class="form-control-plaintext" readonly value="{{ .Selected.Slug }}">
				</div>
			</div>
			<div class="form-group row">
				<label class="col-sm-2 col-form-label">New slug</label>
				<div class="col-sm-10">
					<input class="form-control" name="slug" value="{{ .NewSlug }}" onkeyup="javascript:normalizeSlug(this);">
				</div>
			</div>
			<button type="submit" class="btn btn-primary">Rename</button>
		</form>`)

type renameData struct {
	*Route
	Selected *core.Node
	NewSlug  string
}

func rename(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	var newSlug = selected.Slug()

	// check permission

	if err = selected.RequirePermission(core.Remove, r.User); err != nil {
		return err
	}

	if selected.Parent == nil {
		return errors.New("can't rename root")
	}

	// rename

	if req.Method == http.MethodPost {

		newSlug = req.PostFormValue("slug")

		if err = r.db.SetSlug(selected, newSlug); err == nil {
			r.SeeOther(hrefChoose(selected, 0))
			return nil
		} else {
			r.Danger(err)
		}
	}

	return renameTmpl.Execute(w, &renameData{
		Route:    r,
		NewSlug:  newSlug,
		Selected: selected,
	})
}
