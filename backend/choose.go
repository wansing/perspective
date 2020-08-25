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
					<td>{{ .Selected.Id }}</td>
				</tr>
				<tr class="table-light">
					<td colspan="4" style="border-top: 0; text-align: center;">
						<a class="btn btn-sm btn-primary" href="{{ HrefBackend "edit" .Selected }}">Edit</a>
						<a class="btn btn-sm btn-primary" href="{{ HrefBackend "class" .Selected }}">Set class</a>
						{{ if CanCreate .User .Selected }}
							<a class="btn btn-sm btn-primary" href="{{ HrefBackend "create" .Selected }}">Create</a>
						{{ end }}
						{{ if and .Selected.Parent (CanRemove .User .Selected) }}
							<a class="btn btn-sm btn-primary" href="{{ HrefBackend "rename" .Selected }}">Rename</a>
							<a class="btn btn-sm btn-primary" href="{{ HrefBackend "move" .Selected }}">Move</a>
							<a class="btn btn-sm btn-primary" href="{{ HrefBackend "delete" .Selected }}">Delete</a>
						{{ end }}
						{{ if CanAdmin .User .Selected }}
							<a class="btn btn-sm btn-primary" href="{{ HrefBackend "access" .Selected }}">Access rules</a>
						{{ end }}
					</td>
				</tr>

				{{ if .Children }}
					<tr>
						<th colspan="4">Children</th>
					</tr>
				{{ end }}

				{{ range .Children }}
					<tr>
						<td>{{ $.WorkflowIndicator . }} </td>
						<td>
							<a class="btn btn-sm btn-secondary" href="{{ HrefChoose $.Selected 0 }}/{{ .Slug }}">{{ .Slug }}</a>
						</td>
						<td>{{ .ClassName }}</td>
						<td>{{ .Id }}</td>
					</tr>
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
	*Route
	page     int
	Selected *core.Node
}

func (data *chooseData) Children() ([]core.DBNode, error) {
	return data.Selected.GetChildren(data.Selected.Class.SelectOrder, SelectPerPage, (data.page-1)*SelectPerPage)
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
			return fmt.Sprintf(`<li class="page-item"><a class="page-link" href="%s">%s</a></li>`, hrefBackend(fmt.Sprintf("choose/page/%d", page), data.Selected), name)
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

func choose(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	page, err := strconv.Atoi(params.ByName("page"))
	if err != nil {
		page = 1
	}

	path := params.ByName("path")
	selected, err := r.Open(path)
	if err != nil {
		if r.db.IsNotFound(err) && path == "/" && r.IsRootAdmin() {
			r.SeeOther("/create-root-node")
			return nil
		} else {
			return err
		}
	}

	if err := selected.RequirePermission(core.Read, r.User); err != nil {
		return err
	}

	return chooseTmpl.Execute(w, &chooseData{
		Route:    r,
		page:     page,
		Selected: selected,
	})
}
