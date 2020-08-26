package backend

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/wansing/perspective/auth"
	"github.com/wansing/perspective/core"
)

var ErrAuth = errors.New("unauthorized")

// we need the CoreDB (which is not exposed to user content) in the backend
type Route struct {
	*core.Route
	db *core.CoreDB
}

func NewRoute(db *core.CoreDB, request *core.Request, path string) (*Route, error) {
	r, err := core.NewRoute(request, core.NewQueue(path))
	return &Route{
		Route: r,
		db:    db,
	}, err
}

func (r *Route) GroupsWriteable() bool {
	return r.db.Auth.GroupDB.Writeable()
}

func (r *Route) UsersWriteable() bool {
	return r.db.Auth.UserDB.Writeable()
}

func (r *Route) WorkflowsWriteable() bool {
	return r.db.Auth.WorkflowDB.Writeable()
}

func middleware(db *core.CoreDB, requireLoggedIn bool, f func(http.ResponseWriter, *http.Request, *Route, httprouter.Params) error) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {

		// similar to the code in func main

		var request = db.NewRequest(w, req)

		var mainRoute, err = NewRoute(db, request, req.URL.Path)
		if err != nil {
			http.NotFound(w, req)
			return
		}
		defer mainRoute.Cleanup()

		if requireLoggedIn && !mainRoute.LoggedIn() {
			mainRoute.SeeOther("/")
			return
		}

		mainRoute.SetGlobal("include-bootstrap-4-css", "true")

		if err := f(w, req, mainRoute, params); err != nil {
			// probably no template has been executed, so execute error template
			errorTmpl.Execute(w, struct{
				*Route
				Err error
			}{
				Route: mainRoute,
				Err:   err,
			})
		}
	}
}

var errorTmpl = tmpl(`
	<div class="alert alert-danger" role="alert">
		{{ .Err }}
	</div>`)

func NewBackendRouter(db *core.CoreDB) http.Handler {

	var router = httprouter.New()

	var GETAndPOST = func(path string, handle httprouter.Handle) {
		router.GET(path, handle)
		router.POST(path, handle)
	}

	// public
	router.GET("/", middleware(db, false, root))
	GETAndPOST("/login", middleware(db, false, login))

	// private
	GETAndPOST("/access/*path", middleware(db, true, access))
	router.GET("/choose/:page/*path", middleware(db, true, choose)) // "/choose/1/" will work, "/choose/1" won't
	GETAndPOST("/class/*path", middleware(db, true, setClass))
	GETAndPOST("/create/*path", middleware(db, true, create))
	GETAndPOST("/create-root-node", middleware(db, true, createRootNode))
	GETAndPOST("/delete/*path", middleware(db, true, del))
	GETAndPOST("/edit/*path", middleware(db, true, edit))
	GETAndPOST("/groups", middleware(db, true, groups))
	GETAndPOST("/group/:id", middleware(db, true, group))
	router.GET("/logout", middleware(db, true, logout))
	GETAndPOST("/move/*path", middleware(db, true, move))
	GETAndPOST("/release/*path", middleware(db, true, release))
	GETAndPOST("/rename/*path", middleware(db, true, rename))
	GETAndPOST("/revoke/*path", middleware(db, true, revoke))
	router.GET("/rules", middleware(db, true, rules))
	router.GET("/users", middleware(db, true, users))
	GETAndPOST("/user/:id", middleware(db, true, user))
	GETAndPOST("/workflows", middleware(db, true, workflows))
	GETAndPOST("/workflow/:id", middleware(db, true, workflow))

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
						<a class="nav-link" href="/backend/choose/1/">Nodes</a>
					</li>
					<li class="nav-item">
						<a class="nav-link" href="/backend/user/{{ .User.Id }}">{{ .User.Name }}</a>
					</li>

					{{ if .IsRootAdmin }}

						{{ if .GroupsWriteable }}
							<li class="nav-item">
								<a class="nav-link" href="/backend/groups">Groups</a>
							</li>
						{{ end }}

						{{ if .UsersWriteable }}
							<li class="nav-item">
								<a class="nav-link" href="/backend/users">Users</a>
							</li>
						{{ end }}

						{{ if .WorkflowsWriteable }}
							<li class="nav-item">
								<a class="nav-link" href="/backend/workflows">Workflows</a>
							</li>
						{{ end }}

						<li class="nav-item">
							<a class="nav-link" href="/backend/rules">Rules</a>
						</li>

					{{ end }}

					<li class="nav-item">
						<a class="nav-link" href="/backend/logout">Logout</a>
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
		"CanAdmin": func(u auth.User, e *core.Node) bool {
			return e.RequirePermission(core.Admin, u) == nil
		},
		"CanCreate": func(u auth.User, e *core.Node) bool {
			return e.RequirePermission(core.Create, u) == nil
		},
		"CanRemove": func(u auth.User, e *core.Node) bool {
			return e.RequirePermission(core.Remove, u) == nil
		},
		"FormatTs": FormatTs,
		"GroupLink": func(group auth.Group) template.HTML {
			if group.Id() == 0 { // all users
				return template.HTML(group.Name())
			} else {
				return template.HTML(fmt.Sprintf(`<a href="/backend/group/%d">%s</a>`, group.Id(), group.Name()))
			}
		},
		"HrefBackend": func(segment string, e *core.Node) string {
			return "/backend" + hrefBackend(segment, e) // prepend /backend because this is for templates
		},
		"HrefBackendVersion": func(action string, e *core.Node, versionNr int) string {
			return "/backend" + hrefBackendVersion(action, e, versionNr) // prepend /backend because this is for templates
		},
		"HrefChoose": func(e *core.Node, page int) string {
			return "/backend" + hrefChoose(e, page)
		},
		"UserLink": func(user auth.User) template.HTML {
			return template.HTML(fmt.Sprintf(`<a href="/backend/user/%d">%s</a>`, user.Id(), user.Name()))
		},
		"WorkflowLink": func(w *auth.Workflow) template.HTML {
			return template.HTML(fmt.Sprintf(`<a href="/backend/workflow/%d">%s</a>`, w.Id(), w.Name()))
		},
		"WorkflowLinkLong": func(w *auth.Workflow) template.HTML {
			return template.HTML(fmt.Sprintf(`<a href="/backend/workflow/%d">%s</a>`, w.Id(), w.String()))
		},
	},
)
