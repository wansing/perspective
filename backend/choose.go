package backend

// currently the edit button is always shown, until we have reliable permission caching

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/util"
)

const SelectPerPage = 20

var chooseTmpl = tmpl(`{{ Breadcrumbs .Selected false }}

	<div class="table-responsive">
		<table class="table">
			<thead>
				<tr>
					<th>Status</th>
					<th>URL</th>
					<th>Class</th>
					<th>ID</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<tr class="table-light">
					<td>{{ .WorkflowIndicator .Selected.DBNode }}</td>
					<td>{{ .Selected.Slug }}</td>
					<td>{{ .Selected.Class.Name }} ({{ .Selected.Class.Code }})</td>
					<td>{{ .Selected.ID }}</td>
				</tr>
				<tr class="table-light">
					<td colspan="4" style="border-top: 0; text-align: center;">
						<a class="btn btn-sm btn-primary" href="edit/0{{ .Selected.Location }}">Edit</a>
						<a class="btn btn-sm btn-primary" href="class{{ .Selected.Location }}">Set class</a>
						{{ if CanCreate .User .Selected }}
							<a class="btn btn-sm btn-primary" href="create{{ .Selected.Location }}">Create</a>
						{{ end }}
						{{ if and .Selected.Parent (CanRemove .User .Selected) }}
							<a class="btn btn-sm btn-primary" href="rename{{ .Selected.Location }}">Rename</a>
							<a class="btn btn-sm btn-primary" href="move{{ .Selected.Location }}">Move</a>
							<a class="btn btn-sm btn-primary" href="delete{{ .Selected.Location }}">Delete</a>
						{{ end }}
						{{ if CanAdmin .User .Selected }}
							<a class="btn btn-sm btn-primary" href="access{{ .Selected.Location }}">Access rules</a>
						{{ end }}
					</td>
				</tr>

				<tr>
					<th colspan="4">Children</th>
				</tr>

				{{ range .Children }}
					<tr>
						<td>{{ $.WorkflowIndicator . }} </td>
						<td>
							<a class="btn btn-sm btn-secondary" href="choose/1{{ $.Selected.Location }}/{{ .Slug }}">{{ .Slug }}</a>
						</td>
						<td>{{ .ClassName }}</td>
						<td>{{ .ID }}</td>
					</tr>
				{{ else }}
					<tr>
						<td colspan="4">none</td>
					<tr>
				{{ end }}
			</tbody>
		</table>
	</div>
	<nav>
		<ul class="pagination justify-content-center">
			{{ range .PageLinks }}
				{{ . }}
			{{ end }}
		</ul>
	</nav>`)

type chooseData struct {
	*context
	page     int
	Selected *core.Node
}

func (data *chooseData) Children() ([]*core.Node, error) {
	return data.Selected.GetChildren(data.Request.User, data.Selected.Class.SelectOrder, SelectPerPage, (data.page-1)*SelectPerPage)
}

func (data *chooseData) PageLinks() []template.HTML {

	pagesTotal := 1

	if childrenCount, err := data.Selected.CountChildren(); err == nil {
		pagesTotal = int(math.Ceil(float64(childrenCount) / SelectPerPage))
	}

	return util.PageLinks(
		data.page,
		pagesTotal,
		func(page int, name string) string {
			return fmt.Sprintf(`<li class="page-item"><a class="page-link" href="choose/%d%s">%s</a></li>`, page, data.Selected.Location(), name)
		},
		func(page int, name string) string {
			return fmt.Sprintf(`<li class="page-item active"><span class="page-link">%d</span></li>`, page)
		},
	)
}

func (*chooseData) WorkflowIndicator(e core.DBNode) template.HTML {

	if e.MaxVersionNo() == 0 {
		return template.HTML(`<span class="alert-inline alert-warning">?</span>`)
	}

	if e.MaxVersionNo() == e.MaxWGZeroVersionNo() {
		return template.HTML(`<span class="alert-inline alert-success" title="The latest version has been released.">&#10003;</span>`)
	}

	return template.HTML(`<span class="alert-inline alert-danger" title="The latest version has not been released yet.">&hellip;</span>`)
}

func choose(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	page, err := strconv.Atoi(params.ByName("page"))
	if err != nil {
		page = 1
	}

	var path = params.ByName("path")
	selected, err := ctx.Open(path)
	if err != nil {
		if ctx.db.IsNotFound(err) && path == "/" && ctx.IsRootAdmin() {
			ctx.SeeOther("/create-root-node")
			return nil
		} else {
			return err
		}
	}

	if err := selected.RequirePermission(core.Read, ctx.User); err != nil {
		return err
	}

	return chooseTmpl.Execute(w, &chooseData{
		context:  ctx,
		page:     page,
		Selected: selected,
	})
}
