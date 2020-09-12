package backend

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/auth"
	"github.com/wansing/perspective/core"
)

// We use multiple forms because having multiple submit buttons is tricky.
// (There is no definition which one is used if pressing enter.)
var editTmpl = tmpl(`{{ Breadcrumbs .Selected true }}

	<div>
		{{ .Selected.Class.Name }} &middot; ID: {{ .Selected.Id }} &middot; Workflow: <em>{{ WorkflowLink .State.Workflow }}</em> &middot; Workflow group: <em><strong>{{ .State.WorkflowGroup.Name }}</strong></em>

		{{ with .State.ReleaseToGroup }}
			&middot;
			<form style="display: inline;" action="{{ $.Prefix }}release{{ $.Selected.HrefPath }}:{{ $.Selected.VersionNo }}" method="post" enctype="multipart/form-data">
				<button type="submit" class="btn btn-sm btn-secondary" id="release_button">Release</button>
				<!-- might delete old versions -->
			</form>
			to <em>{{ .Name }}</em>
		{{ end }}

		{{ if and .State.RevokeToGroup .State.ReleaseToGroup }}
			or
		{{ end }}

		{{ with .State.RevokeToGroup }}
			&middot;
			<form style="display: inline;" action="{{ $.Prefix }}revoke{{ $.Selected.HrefPath }}:{{ $.Selected.VersionNo }}" method="post" enctype="multipart/form-data">
				<button type="submit" class="btn btn-sm btn-secondary" id="revoke_button">Revoke</button>
				<!-- might delete old versions -->
			</form>
			to <em>{{ .Name }}</em>
		{{ end }}

	</div>

	<h1>Edit version {{ FormatTs .Selected.TsChanged }}</h1>

	{{ if ne .Selected.VersionNo .Selected.MaxVersionNo }}
		<div class="alert alert-warning">
			You are editing an old version: {{ .Selected.VersionNo }} of {{ .Selected.MaxVersionNo }}.
			<a id="edit_latest_link" href="edit{{ .Selected.HrefPath }}:{{ .Selected.MaxVersionNo }}">
				Edit the latest version instead.
			</a>
		</div>
	{{ end }}

	<form method="post" enctype="multipart/form-data">

		<div class="form-group">
			<textarea class="form-control" id="content" name="content" onchange="changed();">{{ .Content }}</textarea>
		</div>

		<div class="form-group row">

			<div class="col-lg-7">
				<input class="form-control" type="input" id="version" name="version_note" placeholder="Versionsnotiz" maxlength="1000" value="{{ .VersionNote }}">
			</div>

			<div class="col-lg-3">
				<select class="form-control" name="workflow_group">
					<optgroup label="Workflow group">

					{{ range .State.SaveGroups }}
						<option {{ if eq .Id $.WorkflowGroupId -}} selected {{- end }} value="{{ .Id }}">{{ .Name }}</option>
					{{ end }}

					</optgroup>
				</select>
			</div>

			<div class="col-lg-2">
				<button type="submit" class="btn btn-primary form-control">Save</button>
			</div>
		</div>

		<a class="collapse-link">
			<h2>Version History</h2>
		</a>

		<div>
			<p>Only the content is versioned.</p>
			<div class="table-responsive-sm">
				<table class="table table-sm">
					<thead>
						<tr>
							<th>No</th>
							<th>Time</th>
							<th>Version note</th>
							<th>Workflow group</th>
							<th>Load</th>
						</tr>
					</thead>
					<tbody>
						{{ .VersionHistory }}
					</tbody>
				</table>
			</div>
		</div>

		<input type="hidden" name="MAX_FILE_SIZE" value="4000000" />

		{{ $Files := .GetFiles }}

		<a class="collapse-link">
			<h2>{{ with $Files }}{{ len . }} {{ end }}Files</h2>
		</a>

		<div><!-- collapses -->

			<div class="form-group">
				<label for="upload-input">Upload files</label>
				<input type="file" class="form-control-file" name="upload[]" id="upload-input" multiple>
				<p class="mt-2"><a href="#" onclick="document.getElementById('upload-input').value = ''; return false;">Reset upload form</a></p>
			</div>

			{{ with $Files }}

				<div class="table-responsive-sm">
					<table class="table table-sm">
						<thead>
							<tr>
								<th>Name</th>
								<th>Size</th>
								<th>Delete</th>
							</tr>
						</thead>
						<tbody>
							{{ range . }}
								<tr>
									<td><a href="#" onclick="insertAtCursorPosition({{ .Name }}); return false;">{{ .Name }}</a></td>
									<td>{{ .Size }}</td>
									<td><input type="checkbox" title="Delete" id="{{ .Name }}" name="deleteFiles[]" value="{{ .Name }}" /></td>
								</tr>
							{{ end }}
						</tbody>
					</table>
				</div>

			{{ end }}

		</div>
	</form>

	{{ with .Info }}
		<a class="collapse-link">
			<h2>Information on the class {{ $.Selected.Class.Name }}</h2>
		</a>
		<div>
			{{ . }}
		</div>
	{{ end }}

	<script type="text/javascript">

		function insertAtCursorPosition(filename) {

			var ext = filename.split('.').pop().toLowerCase();
			var base = filename.slice(0, -(ext.length+1))

			switch(ext) {
				case "gif":
				case "jpg":
				case "jpeg":
				case "png":
				case "svg":
				case "webp":
					content = '![' + base + '](' + filename + ')'
				default:
					content = '[' + base + '](' + filename + ')'
			}

			textarea = document.getElementById('content');

			if(typeof textarea != 'object'){
				return false;
			}
			if(document.selection) {
				textarea.focus();
				range = document.selection.createRange();
				range.text = content;
				range.select();
			}
			else if(textarea.selectionStart || textarea.selectionStart == '0') {
				posStart = textarea.selectionStart;
				posEnd = textarea.selectionEnd;
				textarea.value = textarea.value.substring(0, posStart) + content + textarea.value.substring(posEnd, textarea.value.length);
				textarea.focus();
				textarea.selectionStart = textarea.selectionEnd = posStart + content.length;
			}
			else {
				textarea.value += content;
				textarea.focus();
			}

			changed();
		}

		// allow tabs and multi-line indentation

		tabOverride.set(document.getElementsByTagName('textarea'));

		var hasChanged = false;

		function changed() {

			if(hasChanged) {
				return;
			}
			else {
				hasChanged = true;
			}

			var releaseButton = document.getElementById('release_button');

			if(releaseButton && releaseButton.disabled == false) {
				releaseButton.disabled = true;
				releaseButton.style.opacity = 0.3;
			}

			var revokeButton = document.getElementById('revoke_button');

			if(revokeButton && revokeButton.disabled == false) {
				revokeButton.disabled = true;
				revokeButton.style.opacity = 0.3;
			}

			var editLatestLink = document.getElementById('edit_latest_link')

			if(editLatestLink) {
				editLatestLink.onclick = function() { return confirm('Änderungern verwerfen?'); };
			}
		}

		function setTsNow(idString, value) {
			var now = new Date();
			var nowString = now.getDate() + '.' + (now.getMonth() + 1) + '.' + now.getFullYear() + ' '
				+ now.getHours() + ':' + now.getMinutes() + ':' + now.getSeconds();
			document.getElementById(idString).value = nowString;
		}

		function resetTs(idString) {
			node = document.getElementById(idString);
			node.value = node.defaultValue;
		}

	</script>`)

type editData struct {
	*Route
	Selected        *core.Node
	State           *auth.ReleaseState
	Content         string
	VersionNote     string
	WorkflowGroupId int // recommended workflow group if the content is edited
}

func (data *editData) GetFiles() ([]os.FileInfo, error) {
	return data.Selected.Folder().Files()
}

func (data *editData) Info() template.HTML {
	return data.Selected.Class.InfoHTML()
}

func (data *editData) VersionHistory() (template.HTML, error) {

	w := &bytes.Buffer{}

	var versions, err = data.Selected.Versions()
	if err != nil {
		return template.HTML(""), err
	}

	for _, version := range versions {

		w.WriteString(`
			<tr`)

		if version.VersionNo() == data.Selected.VersionNo() {
			w.WriteString(` class="table-active"`)
		}

		w.WriteString(`>
			<td>` + strconv.Itoa(version.VersionNo()) + `</td>
			<td>` + FormatTs(version.TsChanged()) + `</td>
			<td>` + html.EscapeString(version.VersionNote()) + `</td>
			<td>
		`)

		// not taking groups from the workflow because the workflow might have changed in the meantime, not containing the group any more
		if grp, err := data.db.Auth.GetGroupOrReaders(version.WorkflowGroupId()); err == nil {
			w.WriteString(html.EscapeString(grp.Name()))
		}

		w.WriteString(`
			<td>
				<a href="edit` + data.Selected.HrefPath() + ":" + strconv.Itoa(version.VersionNo()) + `">Open</a>
			</td>
		`)

		w.WriteString(`</td>
			</tr>`)
	}

	return template.HTML(w.String()), nil
}

func edit(w http.ResponseWriter, req *http.Request, r *Route, params httprouter.Params) error {

	selected, err := r.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	state, err := selected.ReleaseState(r.User)
	if err != nil {
		return err
	}

	if !state.CanEdit() {
		return ErrAuth
	}

	var content = selected.Content()

	var versionNote string
	if selected.VersionNo() != selected.MaxVersionNo() {
		versionNote = fmt.Sprintf("reverted to version %d", selected.VersionNo())
	}

	var workflowGroupId int
	if sg := state.SuggestedSaveGroup(); sg != nil {
		workflowGroupId = (*sg).Id()
	}

	if req.Method == http.MethodPost {

		content = req.PostFormValue("content")
		versionNote = req.PostFormValue("version_note")

		workflowGroupId, err = strconv.Atoi(req.PostFormValue("workflow_group"))
		if err != nil {
			return err
		}
		if !state.IsSaveGroup(workflowGroupId) {
			return fmt.Errorf("invalid workflow group id: %d", workflowGroupId)
		}

		var deleteFiles = req.Form["deleteFiles[]"]
		var uploadFiles = req.MultipartForm.File["upload[]"]
		defer req.MultipartForm.RemoveAll()

		if err = doEdit(r, selected, content, versionNote, r.User.Name(), workflowGroupId, deleteFiles, uploadFiles); err == nil {
			r.SeeOther("/edit%s", selected.HrefPath())
			return nil
		} else {
			r.Danger(err)
			// keep user input, don't redirect
		}
	}

	return editTmpl.Execute(w, &editData{
		Route:           r,
		Selected:        selected,
		State:           state,
		Content:         content,
		VersionNote:     versionNote,
		WorkflowGroupId: workflowGroupId,
	})
}

func doEdit(r *Route, selected *core.Node, content, versionNote, username string, workflowGroupId int, deleteFiles []string, uploadFiles []*multipart.FileHeader) error {

	// delete files

	for _, name := range deleteFiles {
		if strings.Contains(content, "("+name+")") { // markdown syntax for image and href
			r.Danger(fmt.Errorf("%s has not been deleted because it is referenced in the content", name))
			continue
		}
		if err := selected.Folder().Delete(name); err != nil {
			return err
		}
	}

	// upload files (MultipartReader geht nicht, weil die Form schon geparst wurde. Deshalb diese Lösung, die mit temporären Dateien arbeitet.)

	for _, fileheader := range uploadFiles {
		file, err := fileheader.Open()
		if err != nil {
			return err
		}
		if err = selected.Folder().Upload(fileheader.Filename, file); err != nil {
			return err
		}
	}

	// edit content (versioned)

	if content != selected.Content() {
		if err := r.db.Edit(selected, content, versionNote, username, workflowGroupId); err != nil {
			return err
		}
	}
	/*
		if workflowGroupId != selected.WorkflowGroupId() {
			if err := r.db.SetWorkflowGroup(selected, workflowGroupId); err != nil {
				return err
			}
		}
	*/
	return nil
}
