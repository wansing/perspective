package backend

import (
	"errors"
	"net/http"
	"path"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var moveTmpl = tmpl(`<h1>Move {{ .Selected.Location }}</h1>

	<p>
		<a class="btn btn-secondary" href="choose/1{{ .Selected.Location }}">Cancel</a>
	</p>

	<form method="post">
		<div class="form-group row">
			<label class="col-sm-2 col-form-label">Current location</label>
			<div class="col-sm-10">
				<input class="form-control-plaintext" readonly value="{{ .Selected.Parent.Location }}">
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
	*context
	ParentUrl string
	Selected  *core.Node
}

func move(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selected, err := ctx.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	// check delete permission

	if selected.Parent == nil {
		return errors.New("can't move root")
	}

	if err = selected.Parent.RequirePermission(core.Remove, ctx.User); err != nil {
		return err
	}

	var parentUrl = selected.Parent.Location() // default value

	// move

	if req.Method == http.MethodPost {

		parentUrl = req.PostFormValue("parentUrl")
		if !path.IsAbs(parentUrl) {
			parentUrl = path.Join(selected.Location(), parentUrl)
		}

		newParent, err := ctx.Open(parentUrl)
		if err != nil {
			return err
		}

		// check create permission

		if err = newParent.RequirePermission(core.Create, ctx.User); err != nil {
			return err
		}

		if err = ctx.db.SetParent(selected, newParent); err == nil {
			ctx.SeeOther("/choose/1%s", selected.Location())
			return nil
		} else {
			ctx.Danger(err)
		}
	}

	return moveTmpl.Execute(w, &moveData{
		context:   ctx,
		ParentUrl: parentUrl,
		Selected:  selected,
	})
}
