package core

import (
	"encoding/gob"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"golang.org/x/text/language"
)

type Notification struct {
	Message string
	Style   string
}

func init() {
	gob.Register([]Notification{}) // required for storing Notifications in a session
}

var langMatcher = language.NewMatcher([]language.Tag{
	language.AmericanEnglish, // default
	language.German,
})

var monthNamesDe = strings.NewReplacer(
	"January", "Januar",
	"February", "Februar",
	"March", "März",
	"May", "Mai",
	"June", "Juni",
	"July", "Juli",
	"October", "Oktober",
	"December", "Dezember",
)

// A Request is created by CoreDB.NewRequest.
type Request struct {
	db   *CoreDB // unexported, so templates can't access it
	Path string  // absolute, without trailing slash
	User DBUser  // must not be nil

	// http
	request *http.Request
	writer  http.ResponseWriter

	// content
	includes  map[string]map[string]map[string]string // base => command => resultName => value, execute every (base, command) just once
	Templates map[string]*template.Template           // global templates, tailored to class "raw", TODO move to context or so
	vars      map[string]string                       // global variables

	// robustness
	recursed      map[int]interface{} // avoid double recursion and infinite loops
	statusWritten bool
	watchdog      int

	// caching
	language language.Tag
}

// NewRequest creates a Request with the given http.ResponseWriter and http.Request.
// If a user is logged in, it sets Request.User.
func (c *CoreDB) NewRequest(w http.ResponseWriter, httpreq *http.Request) *Request {

	var req = &Request{
		db:        c,
		User:      Guest{},
		request:   httpreq,
		writer:    w,
		includes:  make(map[string]map[string]map[string]string),
		Templates: make(map[string]*template.Template),
		vars:      make(map[string]string),
	}

	req.language, _ = language.MatchStrings(langMatcher, httpreq.Header.Get("Accept-Language"))

	if uid := c.SessionManager.GetInt(httpreq.Context(), "uid"); uid != 0 {
		u, err := c.UserDB.GetUser(uid)
		if u != nil && err == nil {
			req.User = u
		}
		// ignore errors
	}

	req.Path = "/" + strings.Trim(httpreq.URL.Path, "/")

	return req
}

func newDummyRequest() *Request {
	return &Request{
		User:      Guest{},
		includes:  nil, // Include checks that
		Templates: make(map[string]*template.Template),
		vars:      make(map[string]string),
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
	if u, err := req.db.LoginUser(mail, enteredPass); err == nil {
		req.User = u
	} else {
		return err // is ErrAuth if mail or enteredPass is wrong
	}
	req.Success("Welcome %s!", req.User.Name())
	req.db.SessionManager.Put(req.request.Context(), "uid", req.User.ID())
	return nil
}

func (req *Request) LoggedIn() bool {
	return req.User.ID() != 0
}

// Logout removes the user id from the session and calls req.Cleanup().
func (req *Request) Logout() {
	if req.LoggedIn() {
		req.db.SessionManager.Remove(req.request.Context(), "uid")
	}
	req.Cleanup()
}

// Open calls CoreDB.Open.
func (req *Request) Open(path string) (*Node, error) {
	return req.db.Open(req.User, nil, NewQueue("/"+RootSlug+path))
}

// GetGlobal returns the value of a global variable.
func (req *Request) GetGlobal(varName string) string {
	return req.vars[varName]
}

// SetGlobal sets a global variable.
func (req *Request) SetGlobal(varName string, value string) {
	req.vars[varName] = value
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

// IsRootAdmin returns true if the user has admin permission for the root node.
func (req *Request) IsRootAdmin() bool {
	// node id 1 is more robust than Node.Parent.Parent..., which relies on the consistency of the Parent field
	if err := req.db.requireRule(Admin, RootID, req.User, nil); err == nil {
		return true
	} else {
		return false
	}
}

func (req *Request) FormatDateTime(ts int64) string {
	b, _ := req.language.Base()
	switch b.String() {
	case "de":
		return monthNamesDe.Replace(time.Unix(ts, 0).Format("2. January 2006 15:04 Uhr"))
	default:
		return time.Unix(ts, 0).Format("January 2, 2006 3:04 PM")
	}
}
