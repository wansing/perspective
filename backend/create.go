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
	*context
	Selected *core.Node // parent
	Class    string
	Slug     string
}

// wrapper for template
func (data *createData) SelectChildClass() template.HTML {
	return SelectChildClass(data.db.ClassRegistry, data.Selected.Class().FeaturedChildClasses(), data.Class)
}

func create(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selected, err := ctx.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	// check permission

	if err = selected.RequirePermission(core.Create, ctx.User); err != nil {
		return err
	}

	// POST

	if req.Method == http.MethodPost {
		if err := ctx.db.AddChild(selected, req.PostFormValue("slug"), req.PostFormValue("class")); err == nil {
			ctx.SeeOther("/choose/1%s", selected.Location())
			return nil
		} else {
			ctx.Danger(err)
		}
	}

	return createTmpl.Execute(w, &createData{
		context:  ctx,
		Selected: selected,
	})
}
