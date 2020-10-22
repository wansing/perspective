package backend

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/core"
)

var ErrAuth = errors.New("unauthorized")

// we need the CoreDB in the backend
type context struct {
	*core.Request
	Prefix string // with trailing slash
	db     *core.CoreDB
}

func (ctx *context) GroupsWriteable() bool {
	return ctx.db.GroupDB.Writeable()
}

func (ctx *context) UsersWriteable() bool {
	return ctx.db.UserDB.Writeable()
}

func (ctx *context) WorkflowsWriteable() bool {
	return ctx.db.WorkflowDB.Writeable()
}

func middleware(db *core.CoreDB, prefix string, requireLoggedIn bool, f func(http.ResponseWriter, *http.Request, *context, httprouter.Params) error) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {

		// similar to the code in func main

		var request = db.NewRequest(w, req)

		var ctx = &context{
			Prefix:  prefix + "/backend/",
			Request: request,
			db:      db,
		}
		defer ctx.Cleanup()

		if requireLoggedIn && !ctx.LoggedIn() {
			ctx.SeeOther("/")
			return
		}

		ctx.SetGlobal("include-bootstrap-4-css", "true")

		if err := f(w, req, ctx, params); err != nil {
			// probably no template has been executed, so execute error template
			errorTmpl.Execute(w, struct {
				*context
				Err error
			}{
				context: ctx,
				Err:     err,
			})
		}
	}
}

var errorTmpl = tmpl(`
	<div class="alert alert-danger" role="alert">
		{{ .Err }}
	</div>`)

func NewBackendRouter(db *core.CoreDB, prefix string) http.Handler {

	var router = httprouter.New()

	var GETAndPOST = func(path string, handle httprouter.Handle) {
		router.GET(path, handle)
		router.POST(path, handle)
	}

	// public
	router.GET("/", middleware(db, prefix, false, root))
	GETAndPOST("/login", middleware(db, prefix, false, login))

	// private
	GETAndPOST("/access/*path", middleware(db, prefix, true, access))
	router.GET("/choose/:page/*path", middleware(db, prefix, true, choose)) // "/choose/1/" will work, "/choose/1" won't. GET("/choose/:page") would match everyhing.
	GETAndPOST("/class/*path", middleware(db, prefix, true, setClass))
	GETAndPOST("/create/*path", middleware(db, prefix, true, create))
	GETAndPOST("/create-root-node", middleware(db, prefix, true, createRootNode))
	GETAndPOST("/delete/*path", middleware(db, prefix, true, del))
	GETAndPOST("/edit/:version/*path", middleware(db, prefix, true, edit))
	GETAndPOST("/groups", middleware(db, prefix, true, groups))
	GETAndPOST("/group/:id", middleware(db, prefix, true, group))
	router.GET("/logout", middleware(db, prefix, true, logout))
	GETAndPOST("/move/*path", middleware(db, prefix, true, move))
	router.POST("/release/:version/*path", middleware(db, prefix, true, release))
	GETAndPOST("/rename/*path", middleware(db, prefix, true, rename))
	router.POST("/revoke/:version/*path", middleware(db, prefix, true, revoke))
	router.GET("/rules", middleware(db, prefix, true, rules))
	router.GET("/users", middleware(db, prefix, true, users))
	GETAndPOST("/user/:id", middleware(db, prefix, true, user))
	GETAndPOST("/workflows", middleware(db, prefix, true, workflows))
	GETAndPOST("/workflow/:id", middleware(db, prefix, true, workflow))

	return router
}

func tmpl(text string) *template.Template {
	t := template.Must(backendTmpl.Clone())
	t = template.Must(t.Parse(`{{ define "content" }}` + text + `{{ end }}`))
	return t
}

var backendTmpl = template.Must(template.New("backend").Parse(`
<!DOCTYPE html>
<html>
	<head>
		<base href="{{.Prefix}}">
		<link rel="stylesheet" type="text/css" href="/assets/bootstrap-4.4.1.min.css">
		<meta charset="utf-8">
		<script src="/assets/taboverride-4.0.3.min.js"></script>
		<title>Backend</title>

		<style>

			/* collapse */

			.collapse-link > *::after {
				content: " \25BE";
			}

			.collapse-link-active > *::after {
				content: " \25B4";
			}

			.collapsed {
				max-height: 0;
				overflow: hidden;
				transition: max-height 0.2s ease-out;
			}

			/* bootstrap enhancements */

			.alert-inline {
				display: inline-block;
				border: 1px solid transparent;
				border-radius: .2rem;
				padding: .15rem .3rem;
			}

			.bg-light, .table-light, .table-light > td, .table-light > th {
				background-color: #f4f5f6 !important;
			}

			.col-form-label {
				text-align: right;
			}

			/* html tags */

			body {
				padding-bottom: 1rem;
			}

			h1 {
				font-size: 1.5rem !important;
				margin: 1rem 0 0.7rem !important;
			}

			h2 {
				font-size: 1.3rem !important;
				margin: 0.2rem 0 0.5rem !important;
			}

			table {
				margin-top: 0.5rem;
				border-bottom: 1px solid #dee2e6;
			}

			textarea {
				tab-size: 4;
				-moz-tab-size: 4;
			}

		</style>
	</head>
	<body>

		{{ if .LoggedIn }}

			<nav class="navbar navbar-expand-md bg-light">
				<ul class="navbar-nav">
					<li class="nav-item">
						<a class="nav-link" href="/" target="_blank">View site</a>
					</li>
					<li class="nav-item">
						<a class="nav-link" href="choose/1/">Nodes</a>
					</li>
					<li class="nav-item">
						<a class="nav-link" href="user/{{ .User.ID }}">{{ .User.Name }}</a>
					</li>

					{{ if .IsRootAdmin }}

						{{ if .GroupsWriteable }}
							<li class="nav-item">
								<a class="nav-link" href="groups">Groups</a>
							</li>
						{{ end }}

						{{ if .UsersWriteable }}
							<li class="nav-item">
								<a class="nav-link" href="users">Users</a>
							</li>
						{{ end }}

						{{ if .WorkflowsWriteable }}
							<li class="nav-item">
								<a class="nav-link" href="workflows">Workflows</a>
							</li>
						{{ end }}

						<li class="nav-item">
							<a class="nav-link" href="rules">Rules</a>
						</li>

					{{ end }}

					<li class="nav-item">
						<a class="nav-link" href="logout">Logout</a>
					</li>
				</ul>
			</nav>

			<script>

				function normalizeSlug(widget) {
					widget.value = widget.value.toLowerCase().replace(/[^a-z0-9]+/g, '-');
				}

			</script>

		{{ end }}

		<div class="container pt-3">
			<div class="starter-template">
				{{ .RenderNotifications }}
				{{ template "content" . }}
			</div>
		</div>

		{{ if .LoggedIn }}

			<script>

				var links = document.getElementsByClassName("collapse-link");

				for(var i = 0; i < links.length; i++) {
					links[i].addEventListener("click", function() {
						this.classList.toggle("collapse-link-active");
						var content = this.nextElementSibling;
						if(content.style.maxHeight) {
							content.style.maxHeight = null;
						} else {
							content.style.maxHeight = content.scrollHeight + "px";
							setTimeout(
								function () {
									content.scrollIntoView(
										{
											behavior: "smooth",
											block: "start"
										}
									);
								},
								200
							);
						}
					});

					links[i].setAttribute("href", "javascript:void(0);");

					links[i].nextElementSibling.className = "collapsed";
				}

				var textareas = document.getElementsByTagName('textarea');

				for(var i = 0; i < textareas.length; i++) {
					textareas[i].setAttribute('style', 'height:' + textareas[i].scrollHeight + 'px;overflow-y:hidden;');
					textareas[i].addEventListener("input", onTextareaInput, false);
				}

				function onTextareaInput() {

					var scrollLeft = window.pageXOffset || (document.documentElement || document.body.parentNode || document.body).scrollLeft;
					var scrollTop  = window.pageYOffset || (document.documentElement || document.body.parentNode || document.body).scrollTop;

					this.style.height = 'auto';
					this.style.height = (this.scrollHeight) + 'px';

					window.scrollTo(scrollLeft, scrollTop);
				}

			</script>

		{{ end }}
	</body>
</html>`)).Funcs(
	template.FuncMap{
		"Breadcrumbs": func(im *core.Node, link bool) template.HTML {
			return Breadcrumbs(im, link)
		},
		"CanAdmin": func(u core.DBUser, n *core.Node) bool {
			return n.RequirePermission(core.Admin, u) == nil
		},
		"CanCreate": func(u core.DBUser, n *core.Node) bool {
			return n.RequirePermission(core.Create, u) == nil
		},
		"CanRemove": func(u core.DBUser, n *core.Node) bool {
			return n.RequirePermission(core.Remove, u) == nil
		},
		"FormatTs": FormatTs,
		"GroupLink": func(group core.DBGroup) template.HTML {
			if group.ID() == 0 { // all users
				return template.HTML(group.Name())
			} else {
				return template.HTML(fmt.Sprintf(`<a href="group/%d">%s</a>`, group.ID(), group.Name()))
			}
		},
		"UserLink": func(user core.DBUser) template.HTML {
			return template.HTML(fmt.Sprintf(`<a href="user/%d">%s</a>`, user.ID(), user.Name()))
		},
		"WorkflowLink": func(w *core.Workflow) template.HTML {
			return template.HTML(fmt.Sprintf(`<a href="workflow/%d">%s</a>`, w.ID(), w.Name()))
		},
		"WorkflowLinkLong": func(w *core.Workflow) template.HTML {
			return template.HTML(fmt.Sprintf(`<a href="workflow/%d">%s</a>`, w.ID(), w.String()))
		},
	},
)
