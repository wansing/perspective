package backend

import (
	"html/template"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var createTmpl = tmpl(`<h1>Create node below {{ .Selected.Location }}</h1>

	<p>
		<a class="btn btn-secondary" href="choose/1{{ .Selected.Location }}">Cancel</a>
	</p>

	<form method="post">
		<div class="form-row">
			<div class="col-md-7">
				<input class="form-control" name="slug" placeholder="Slug" value="{{ .Slug }}" onkeyup="javascript:normalizeSlug(this);">
			</div>
			<div class="col-md-3">
				<select class="form-control" name="class">
					{{ .SelectChildClass }}
				</select>
			</div>
			<div class="col-md-2">
				<button type="submit" class="btn btn-primary" name="create">Create</button>
			</div>
		</div>
	</form>`)

type createData struct {
	*Route
	Selected *core.Node // parent
	Class    string
	Slug     string
}

// wrapper for template
func (data *createData) SelectChildClass() template.HTML {
	return SelectChildClass(data.db.ClassRegistry, data.Selected, data.Class)
}

func create(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	// check permission

	if err = selected.RequirePermission(core.Create, r.User); err != nil {
		return err
	}

	// POST

	if req.Method == http.MethodPost {
		if err := r.db.AddChild(selected, req.PostFormValue("slug"), req.PostFormValue("class")); err == nil {
			r.SeeOther("/choose/1%s", selected.Location())
			return nil
		} else {
			r.Danger(err)
		}
	}

	return createTmpl.Execute(w, &createData{
		Route:    r,
		Selected: selected,
	})
}
