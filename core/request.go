package core

import (
	"encoding/gob"
	"fmt"
	"html/template"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/wansing/perspective/auth"
)

type Notification struct {
	Message string
	Style   string
}

func init() {
	gob.Register([]Notification{}) // required for storing Notifications in a session
}

// A Request is created by CoreDB.NewRequest.
type Request struct {
	db   *CoreDB // unexported, so it can't be accessed in templates
	User auth.User

	// http
	writer  http.ResponseWriter
	request *http.Request

	// content
	globals   map[string]string                       // must be writable by includes, so they can require js/css libraries
	includes  map[string]map[string]map[string]string // base path => command => resultName => value
	templates map[string]*template.Template           // global templates

	// robustness
	statusWritten bool
	watchdog      int
}

// NewRequest creates a Request with the given http.ResponseWriter and http.Request.
// If a user is logged in, it sets Request.User.
func (c *CoreDB) NewRequest(w http.ResponseWriter, httpreq *http.Request) *Request {

	var req = &Request{
		db:        c,
		writer:    w,
		request:   httpreq,
		globals:   make(map[string]string),
		includes:  make(map[string]map[string]map[string]string),
		templates: make(map[string]*template.Template),
	}

	if uid := c.SessionManager.GetInt(httpreq.Context(), "uid"); uid != 0 {
		u, err := c.Auth.GetUser(uid)
		if u != nil && err == nil {
			req.User = u
		}
		// ignore errors
	}

	return req
}

func newDummyRequest() *Request {
	return &Request{
		writer:    nil,
		request:   nil,
		globals:   make(map[string]string),
		includes:  nil, // include checks that
		templates: make(map[string]*template.Template),
	}
}

// Danger adds a "danger" notification to the session.
func (req *Request) Danger(err error) {
	req.addNotification(err.Error(), "danger")
}

// Success adds a "success" notification to the session.
func (req *Request) Success(format string, args ...interface{}) {
	req.addNotification(fmt.Sprintf(format, args...), "success")
}

// style should be a bootstrap alert style without the leading "alert-"
func (req *Request) addNotification(message, style string) {
	notifications, _ := req.db.SessionManager.Get(req.request.Context(), "notifications").([]Notification)
	notifications = append(notifications, Notification{message, style})
	req.db.SessionManager.Put(req.request.Context(), "notifications", notifications)
}

// RenderNotification removes all notifications from the session
// and renders them into an HTML string.
// If the HTTP status had already been written, it does nothing.
func (req *Request) RenderNotifications() template.HTML {
	var r string
	if !req.statusWritten {
		notifications, _ := req.db.SessionManager.Pop(req.request.Context(), "notifications").([]Notification)
		for _, n := range notifications {
			r += `<div class="alert alert-` + n.Style + ` mt-3" role="alert">` + n.Message + `</div>`
		}
	}
	return template.HTML(r)
}

// Destroys the session (which means re-setting the cookie with zero lifetime) if the session has been modified and is empty now.
func (req *Request) Cleanup() {
	sessMan := req.db.SessionManager
	if sessMan.Status(req.request.Context()) == scs.Modified && len(sessMan.Keys(req.request.Context())) == 0 {
		_ = sessMan.Destroy(req.request.Context())
	}
}

// SeeOther sets the HTTP header to redirect to an URL.
func (req *Request) SeeOther(format string, args ...interface{}) {
	if req.statusWritten {
		return
	}
	var url = fmt.Sprintf(format, args...)
	http.Redirect(req.writer, req.request, url, http.StatusSeeOther)
	req.statusWritten = true
}

// Login tries to log in a user. On success, the user id is stored in the session.
func (req *Request) Login(mail string, enteredPass string) error {
	if req.LoggedIn() {
		return nil
	}
	if u, err := req.db.Auth.LoginUser(mail, enteredPass); err == nil {
		req.User = u
	} else {
		return err // is ErrAuth if mail or enteredPass is wrong
	}
	req.Success("Welcome %s!", req.User.Name())
	req.db.SessionManager.Put(req.request.Context(), "uid", req.User.Id())
	return nil
}

func (req *Request) LoggedIn() bool {
	return req.User != nil
}

// Logout removes the user id from the session and calls req.Cleanup().
func (req *Request) Logout() {
	if req.LoggedIn() {
		req.db.SessionManager.Remove(req.request.Context(), "uid")
	}
	req.Cleanup()
}

// Open creates a new Route from the given path, setting latest = true and leaving Execute = false.
// It returns the leaf of the route.
func (req *Request) Open(path string) (*Node, error) {
	route, err := NewRoute(req, path)
	if err != nil {
		return nil, err
	}
	route.latest = true
	if err = route.RootRecurse(); err != nil {
		return nil, err
	}
	return route.root.Leaf(), nil
}

// GetGlobal returns the value of a global variable.
func (req *Request) GetGlobal(varName string) template.HTML {
	return template.HTML(req.globals[varName])
}

// HasGlobal returns whether a global variable with the given name exists.
func (req *Request) HasGlobal(varName string) bool {
	_, ok := req.globals[varName]
	return ok
}

// SetGlobal sets a global variable to a given value.
func (req *Request) SetGlobal(varName string, value string) interface{} {
	req.globals[varName] = value
	return nil
}

// IsHTML returns true if the Content-Type field of the header
// of the embedded http.ResponseWriter is empty or set to "text/html".
func (req *Request) IsHTML() bool {

	if req.writer == nil { // in dummy requests
		return false
	}

	switch req.writer.Header().Get("Content-Type") {
	case "":
		return true
	case "text/html":
		return true
	default:
		return false
	}
}
