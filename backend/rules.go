package backend

import (
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/auth"
	"github.com/wansing/perspective/core"
)

var rulesTmpl = tmpl(`<h1>All Access Rules</h1>

	<h2>Edit</h2>

	<table class="table">
		<tr>
			<th>Node</th>
			<th>Children only</th>
			<th>Workflow</th>
		</tr>

		{{ range .WorkflowAssignments }}
			<tr>
				<td><a class="btn btn-sm btn-secondary" href="access{{ .InternalUrl }}">{{ .InternalUrl }}</a></td>
				<td>{{ .ChildrenOnly }}</td>
				<td>{{ if .Workflow }}{{ WorkflowLink .Workflow }}{{ else }}(Workflow not found){{ end }}</td>
			</tr>
		{{ end }}
	</table>

	<h2>Other Permissions</h2>

	<table class="table">
		<tr>
			<th>Node</th>
			<th>Group</th>
			<th>Permission</th>
		</tr>

		{{ range .Rules }}
			<tr>
				<td><a class="btn btn-sm btn-secondary" href="access{{ .Url }}">{{ .Url }}</a></td>
				<td>{{ GroupLink .Group }}</td>
				<td>{{ .Permission.String }}</td>
			</tr>
		{{ end }}
	</table>`)

type rulesData struct {
	*Route
}

// for view only
type rule struct {
	Url        string
	Group      auth.Group
	Permission core.Permission
}

func (data *rulesData) WorkflowAssignments() (result []struct {
	Workflow     *auth.Workflow
	ChildrenOnly bool
	InternalUrl  string
}) {

	rawEditors, err := data.db.GetAllWorkflowAssignments()
	if err != nil {
		return
	}

	for nodeId, nodeMap := range rawEditors {
		for childrenOnly, workflow := range nodeMap {

			var row = struct {
				Workflow     *auth.Workflow
				ChildrenOnly bool
				InternalUrl  string
			}{
				Workflow:     workflow,
				ChildrenOnly: childrenOnly,
			}

			row.InternalUrl, err = data.db.InternalPathByNodeId(nodeId)
			if err != nil {
				return
			}

			result = append(result, row)
		}
	}

	return
}

func (data *rulesData) Rules() ([]rule, error) {

	rawRules, err := data.db.GetAllAccessRules()
	if err != nil {
		return nil, err
	}

	result := make([]rule, 0, len(rawRules))

	for nodeId, groupMap := range rawRules {

		for groupId, permInt := range groupMap {

			url, err := data.db.InternalPathByNodeId(nodeId)
			if err != nil {
				return nil, err
			}

			grp, err := data.db.Auth.GetGroup(groupId)
			if err != nil {
				return nil, fmt.Errorf("error getting group %d: %v", groupId, err)
			}

			perm := core.Permission(permInt)
			if !perm.Valid() {
				return nil, fmt.Errorf("invalid permission: %d", permInt)
			}

			result = append(result, rule{
				url,
				grp,
				perm,
			})
		}
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			if result[i].Url == result[j].Url {
				return result[i].Group.Id() < result[j].Group.Id()
			}
			return result[i].Url < result[j].Url
		},
	)

	return result, nil
}

func rules(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	if !r.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	return rulesTmpl.Execute(w, &rulesData{
		Route: r,
	})
}
