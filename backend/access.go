package backend

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var accessTmpl = tmpl(`<h1>Access Rules of {{ .Selected.Location }}</h1>

	<p>
		<a class="btn btn-secondary" href="choose/1{{ .Selected.Location }}">Cancel</a>
	</p>

	<form method="post">

		<h2>Workflows (edit)</h2>

		<div class="form-group row">
			<label class="col-md-8 col-form-label">Assign workflow to this and descendant nodes</label>
			<div class="col-md-4">
				<select class="form-control" name="workflow">
					{{ .WriteWorkflowOptions (.Selected.GetAssignedWorkflow false) }}
				</select>
			</div>
		</div>

		<div class="form-group row">
			<label class="col-md-8 col-form-label">Assign workflow to descendant nodes only</label>
			<div class="col-md-4">
				<select class="form-control" name="childrenWorkflow">
					{{ .WriteWorkflowOptions (.Selected.GetAssignedWorkflow true) }}
				</select>
			</div>
		</div>

		<h2>Group permissions</h2>

		<table class="table">

			<tr>
				<th>Group</th>
				<th>Permission</th>
				<th>Delete</th>
			</tr>

			{{ range $group, $permission := .Selected.GetAssignedRules }}

				<tr>
					<td>{{ $group.Name }}</td>
					<td>{{ $permission.String }}</td>
					<td><input type="checkbox" name="remove[]" value="{{ $group.Id }}"></td>
				</tr>

			{{ end }}

			<tr>
				<td>
					<select class="form-control" name="group">
						<option value=""></option>
						<option value="0">All Users</option>

						{{ range $.AllGroups }}
							<option value="{{ .Id }}">{{ .Name }}</option>
						{{ end }}

					</select>
				</td>
				<td>
					<select class="form-control" name="permission">
						<option value=""></option>
						<option value="` + strconv.Itoa(int(core.None)) + `">none</option>
						<option value="` + strconv.Itoa(int(core.Read)) + `">read</option>
						<option value="` + strconv.Itoa(int(core.Create)) + `">create</option>
						<option value="` + strconv.Itoa(int(core.Remove)) + `">remove</option>
						<option value="` + strconv.Itoa(int(core.Admin)) + `">admin</option>2
					</select>
				</td>
				<td></td>
			</tr>
		</table>

		<p>
			<button type="submit" class="btn btn-primary" name="save">Apply</button>
		</p>

	</form>`)

type accessData struct {
	*context
	Selected *core.Node
}

func (t *accessData) AllGroups() ([]core.DBGroup, error) {
	return t.db.GetAllGroups(100000, 0) // assuming there are not more than 100k groups
}

func (t *accessData) WriteWorkflowOptions(selectedWorkflow *core.Workflow) (template.HTML, error) {

	buf := &bytes.Buffer{}

	// no workflow assignment

	buf.WriteString(`<option value="0"`)
	if selectedWorkflow == nil {
		buf.WriteString(` selected`)
	}
	buf.WriteString(`></option>`)

	// workflows

	allWorkflows, err := t.db.GetAllWorkflows(10000, 0) // assuming there are not more than 10k workflows
	if err != nil {
		return template.HTML(""), err
	}

	for _, workflow := range allWorkflows {
		buf.WriteString(`<option value="` + strconv.Itoa(workflow.Id()) + `"`)
		if selectedWorkflow != nil && (*selectedWorkflow).Id() == workflow.Id() {
			buf.WriteString(` selected`)
		}
		buf.WriteString(`>` + workflow.Name() + `</option>`)
	}

	return template.HTML(buf.String()), nil
}

func access(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selected, err := ctx.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	err = selected.RequirePermission(core.Admin, ctx.User)
	if err != nil {
		return err
	}

	// POST

	if req.Method == http.MethodPost {

		// workflow

		workflowId, err := strconv.Atoi(req.PostFormValue("workflow"))
		if err != nil {
			return err
		}

		if workflowId == 0 {
			if err = ctx.db.UnassignWorkflow(selected, false); err != nil {
				return err
			}
		} else {
			if err = ctx.db.AssignWorkflow(selected, false, workflowId); err != nil {
				return err
			}
		}

		// children workflow

		childrenWorkflowId, err := strconv.Atoi(req.PostFormValue("childrenWorkflow"))
		if err != nil {
			return err
		}

		if childrenWorkflowId == 0 {
			if err = ctx.db.UnassignWorkflow(selected, true); err != nil {
				return err
			}
		} else {
			if err = ctx.db.AssignWorkflow(selected, true, childrenWorkflowId); err != nil {
				return err
			}
		}

		// build RemoveRules

		removeRules := make(map[int]interface{})

		for _, groupIdString := range req.PostForm["remove[]"] {
			groupId, err := strconv.Atoi(groupIdString)
			if err != nil {
				return err
			}
			removeRules[groupId] = struct{}{}
		}

		// anti-lockout

		myAdminRules, err := selected.RequirePermissionRules(core.Admin, ctx.User)
		if err != nil {
			return err
		}

		var mySelectedAdminRules = myAdminRules[selected.Id()]
		if len(myAdminRules) == len(mySelectedAdminRules) {
			// all of my admin rules apply to this node, none of them apply to any of its ancestors
			for groupId, _ := range removeRules {
				// simulate removal
				delete(mySelectedAdminRules, groupId)
			}
			if len(mySelectedAdminRules) == 0 {
				// there would be no admin rules left for this node
				return errors.New("you can't lock yourself out")
			}
		}

		// process removeRules

		for removeGroupId := range removeRules {
			err = ctx.db.RemoveAccessRule(selected, removeGroupId)
			if err != nil {
				return fmt.Errorf("error removing rule: %v", err)
			}
		}

		// add (group, permission)

		addGroupIdStr := req.PostFormValue("group")
		addPermissionStr := req.PostFormValue("permission")

		if addGroupIdStr != "" && addPermissionStr != "" {

			addGroupId, err := strconv.Atoi(addGroupIdStr)
			if err != nil {
				return err
			}

			addPermission, err := strconv.Atoi(addPermissionStr)
			if err != nil {
				return err
			}

			err = ctx.db.AddAccessRule(selected, addGroupId, core.Permission(addPermission))
			if err != nil {
				return fmt.Errorf("error adding rule: %v", err)
			}
		}

		ctx.SeeOther("/access%s", selected.Location())
		return nil
	}

	return accessTmpl.Execute(w, &accessData{
		context:  ctx,
		Selected: selected,
	})
}
