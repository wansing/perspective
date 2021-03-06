package backend

import (
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/julienschmidt/httprouter"
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
	*context
}

// for view only
type rule struct {
	Url        string
	Group      core.DBGroup
	Permission core.Permission
}

func (data *rulesData) WorkflowAssignments() (result []struct {
	Workflow     *core.Workflow
	ChildrenOnly bool
	InternalUrl  string
}) {

	rawEditors, err := data.db.GetAllWorkflowAssignments()
	if err != nil {
		return
	}

	for nodeID, nodeMap := range rawEditors {
		for childrenOnly, workflow := range nodeMap {

			var row = struct {
				Workflow     *core.Workflow
				ChildrenOnly bool
				InternalUrl  string
			}{
				Workflow:     workflow,
				ChildrenOnly: childrenOnly,
			}

			row.InternalUrl, err = data.db.InternalPathByNodeID(nodeID)
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

	for nodeID, groupMap := range rawRules {

		for groupID, permInt := range groupMap {

			url, err := data.db.InternalPathByNodeID(nodeID)
			if err != nil {
				return nil, err
			}

			grp, err := data.db.GetGroup(groupID)
			if err != nil {
				return nil, fmt.Errorf("error getting group %d: %v", groupID, err)
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
				return result[i].Group.ID() < result[j].Group.ID()
			}
			return result[i].Url < result[j].Url
		},
	)

	return result, nil
}

func rules(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	if !ctx.IsRootAdmin() {
		return errors.New("unauthorized")
	}

	return rulesTmpl.Execute(w, &rulesData{
		context: ctx,
	})
}
