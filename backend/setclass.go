package backend

import (
	"html/template"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var setClassTmpl = tmpl(`<h1>Set class of {{ .Selected.Location }}</h1>

		<p>
			<a class="btn btn-secondary" href="choose/1{{ .Selected.Location }}">Cancel</a>
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
	*context
	Selected     *core.Node
	NewClassCode string
}

func (data *setClassData) SelectChildClass() template.HTML {
	var featuredChildClasses []string
	if data.Selected.Parent != nil {
		featuredChildClasses = data.Selected.Parent.Class().FeaturedChildClasses()
	}
	return SelectChildClass(data.db.ClassRegistry, featuredChildClasses, data.NewClassCode)
}

func setClass(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selected, err := ctx.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	var newClassCode = selected.Class().Code()

	// check permission

	if err := selected.RequirePermission(core.Remove, ctx.User); err != nil {
		return err
	}

	// set class

	if req.Method == http.MethodPost {
		newClassCode = req.PostFormValue("class")
		if err = ctx.db.SetClass(selected, newClassCode); err == nil {
			ctx.SeeOther("/choose/1%s", selected.Location())
			return nil
		} else {
			ctx.Danger(err)
		}
	}

	return setClassTmpl.Execute(w, &setClassData{
		context:      ctx,
		NewClassCode: newClassCode,
		Selected:     selected,
	})
}
