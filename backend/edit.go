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
	"github.com/wansing/perspective/core"
)

// We use multiple forms because having multiple submit buttons is tricky.
// (There is no definition which one is used if pressing enter.)
var editTmpl = tmpl(`{{ Breadcrumbs .Selected true }}

	<div class="mb-3">
		{{ .Selected.Class.Name }} &middot; ID: {{ .Selected.ID }} &middot; Workflow: <em>{{ WorkflowLink .State.Workflow }}</em>

		{{ if ne .SelectedVersion.VersionNo 0 }}
			&middot; Version: {{ .SelectedVersion.VersionNo }} ({{ FormatTs .SelectedVersion.TsChanged }})
			&middot; Workflow group: <em><strong>{{ .State.WorkflowGroup.Name }}</strong></em>

			{{ with .State.ReleaseToGroup }}
				&middot;
				<form style="display: inline;" action="{{ $.Prefix }}release/{{ $.SelectedVersion.VersionNo }}{{ $.Selected.Location }}" method="post" enctype="multipart/form-data">
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
				<form style="display: inline;" action="{{ $.Prefix }}revoke/{{ $.SelectedVersion.VersionNo }}{{ $.Selected.Location }}" method="post" enctype="multipart/form-data">
					<button type="submit" class="btn btn-sm btn-secondary" id="revoke_button">Revoke</button>
					<!-- might delete old versions -->
				</form>
				to <em>{{ .Name }}</em>
			{{ end }}
		{{ end }}
	</div>

	{{ if ne .SelectedVersion.VersionNo .Selected.MaxVersionNo }}
		<div class="alert alert-warning">
			You are editing an old version: {{ .SelectedVersion.VersionNo }} of {{ .Selected.MaxVersionNo }}.
			<a id="edit_latest_link" href="edit/{{ .Selected.MaxVersionNo }}{{ .Selected.Location }}">
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
						<option {{ if eq .ID $.WorkflowGroupID -}} selected {{- end }} value="{{ .ID }}">{{ .Name }}</option>
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
			<h2>{{ $.Selected.Class.Name }}</h2>
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
	*context
	Selected        *core.Node
	SelectedVersion core.DBVersion
	State           *core.ReleaseState
	Content         string
	VersionNote     string
	WorkflowGroupID int // recommended workflow group if the content is edited
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

	for _, v := range versions {

		w.WriteString(`
			<tr`)

		if v.VersionNo() == data.SelectedVersion.VersionNo() {
			w.WriteString(` class="table-active"`)
		}

		w.WriteString(`>
			<td>` + strconv.Itoa(v.VersionNo()) + `</td>
			<td>` + FormatTs(v.TsChanged()) + `</td>
			<td>` + html.EscapeString(v.VersionNote()) + `</td>
			<td>
		`)

		// not taking groups from the workflow because the workflow might have changed in the meantime, not containing the group any more
		if grp, err := data.db.GetGroupOrReaders(v.WorkflowGroupID()); err == nil {
			w.WriteString(html.EscapeString(grp.Name()))
		}

		w.WriteString(`
			<td>
				<a href="edit/` + strconv.Itoa(v.VersionNo()) + data.Selected.Location() + `">Open</a>
			</td>
		`)

		w.WriteString(`</td>
			</tr>`)
	}

	return template.HTML(w.String()), nil
}

func edit(w http.ResponseWriter, req *http.Request, ctx *context, params httprouter.Params) error {

	selected, err := ctx.Open(params.ByName("path"))
	if err != nil {
		return err
	}

	versionNo, _ := strconv.Atoi(params.ByName("version"))
	if versionNo == 0 {
		versionNo = selected.MaxVersionNo()
	}

	var selectedVersion core.DBVersion = core.NoVersion{} // easier than checking for nil in this whole file
	if versionNo != 0 {
		selectedVersion, err = selected.GetVersion(versionNo)
		if err != nil {
			return err
		}
	}

	state, err := selected.ReleaseState(selectedVersion, ctx.User)
	if err != nil {
		return err
	}

	if !state.CanEditNode() {
		return ErrAuth
	}

	var content = selectedVersion.Content()

	var versionNote string
	if selectedVersion.VersionNo() != selected.MaxVersionNo() {
		versionNote = fmt.Sprintf("modified version %d of %d", selectedVersion.VersionNo(), selected.MaxVersionNo())
	}

	var workflowGroupID int
	if sg := state.SuggestedSaveGroup(); sg != nil {
		workflowGroupID = (*sg).ID()
	}

	if req.Method == http.MethodPost {

		content = req.PostFormValue("content")
		versionNote = req.PostFormValue("version_note")

		workflowGroupID, err = strconv.Atoi(req.PostFormValue("workflow_group"))
		if err != nil {
			return err
		}
		if !state.IsSaveGroup(workflowGroupID) {
			return fmt.Errorf("invalid workflow group id: %d", workflowGroupID)
		}

		var deleteFiles = req.Form["deleteFiles[]"]
		var uploadFiles = req.MultipartForm.File["upload[]"]
		defer req.MultipartForm.RemoveAll()

		if err = doEdit(ctx, selected, selectedVersion, content, versionNote, ctx.User.Name(), workflowGroupID, deleteFiles, uploadFiles); err == nil {
			ctx.SeeOther("/edit/%d%s", 0 /* evaluates to max version number, might be racey */, selected.Location())
			return nil
		} else {
			ctx.Danger(err)
			// keep user input, don't redirect
		}
	}

	return editTmpl.Execute(w, &editData{
		context:         ctx,
		Selected:        selected,
		SelectedVersion: selectedVersion,
		State:           state,
		Content:         content,
		VersionNote:     versionNote,
		WorkflowGroupID: workflowGroupID,
	})
}

func doEdit(ctx *context, selected *core.Node, selectedVersion core.DBVersion, content, versionNote, username string, workflowGroupID int, deleteFiles []string, uploadFiles []*multipart.FileHeader) error {

	// delete files

	for _, name := range deleteFiles {
		if strings.Contains(content, "("+name+")") { // markdown syntax for image and href
			ctx.Danger(fmt.Errorf("%s has not been deleted because it is referenced in the content", name))
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

	if content != selectedVersion.Content() {
		if err := ctx.db.Edit(selected, selectedVersion, content, versionNote, username, workflowGroupID); err != nil {
			return err
		}
	}

	return nil
}
