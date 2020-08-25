package backend

import (
	"html/template"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var setClassTmpl = tmpl(`<h1>Set class of {{ .Selected.HrefPath }}</h1>

		<p>
			<a class="btn btn-secondary" href="{{ HrefChoose .Selected 0 }}">Cancel</a>
		</p>

		<form method="post">
			<div class="form-group row">
				<label class="col-sm-2 col-form-label">Class</label>
				<div class="col-sm-10">
					<select class="form-control" name="class">
						{{ .SelectChildClass }}
					</select>
				</div>
			</div>
			<button type="submit" class="btn btn-primary">Set class</button>
		</form>`)

type setClassData struct {
	*Route
	Selected *core.Node
	NewClass string
}

func (data *setClassData) SelectChildClass() template.HTML {
	return SelectChildClass(data.db.ClassRegistry, data.Selected.Parent, data.NewClass)
}

func setClass(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	var newClass = selected.Class.Code

	// check permission

	if err := selected.RequirePermission(core.Remove, r.User); err != nil {
		return err
	}

	// set class

	if req.Method == http.MethodPost {
		newClass = req.PostFormValue("class")
		if err = r.db.SetClass(selected, newClass); err == nil {
			r.SeeOther(hrefChoose(selected, 0))
			return nil
		} else {
			r.Danger(err)
		}
	}

	return setClassTmpl.Execute(w, &setClassData{
		Route:    r,
		NewClass: newClass,
		Selected: selected,
	})
}
