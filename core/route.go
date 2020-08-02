package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"runtime/debug"
	"unicode"
	"unicode/utf8"

	"github.com/wansing/perspective/util"
	"gopkg.in/ini.v1"
)

// see https://golang.org/pkg/context/#WithValue
type routeContextKey struct{}

var ErrNotFound = errors.New("not found")

// A route processes one queue, which can be the main queue or an included queue.
//
// A route is passed to the content template.
//
// For usage in templates, funcs on Route must return "one return value (of any type) or two return values, the second of which is an error."
type Route struct {
	*Queue
	*Request
	Execute bool
	results map[string]string // results of template execution
	latest  bool
	root    *Node

	// only used during recursion
	calledRecurse map[int]interface{} // node id -> interface{}, mapping exists if node has called Recurse
	current       *Node
}

// RouteFromContext gets a Route from the given context. It can panic.
func RouteFromContext(ctx context.Context) *Route {
	return ctx.Value(routeContextKey{}).(*Route)
}

// NewRoute creates an empty Route. It creates a Queue from the given path.
func NewRoute(request *Request, path string) (*Route, error) {
	var queue, err = NewQueue(path)
	if err != nil {
		return nil, err
	}
	return &Route{
		Request:       request,
		Queue:         queue,
		results:       make(map[string]string),
		calledRecurse: make(map[int]interface{}),
	}, nil
}

func newDummyRoute() *Route {
	var queue, _ = NewQueue("")
	return &Route{
		Queue:         queue,
		Request:       newDummyRequest(),
		results:       make(map[string]string),
		calledRecurse: make(map[int]interface{}),
	}
}

func (r *Route) Current() *Node {
	return r.current
}

// ContentAsIni parses the content of the current node according to the INI syntax.
func (r *Route) ContentAsIni() (map[string]string, error) {
	cfg, err := ini.Load([]byte(r.current.Content()))
	if err != nil {
		return nil, err
	}
	return cfg.Section("").KeysHash(), nil
}

// T returns the instance of the current node.
func (r *Route) T() interface{} {
	return r.current.Instance
}

// Prefix returns the HrefPath of the leaf.
// If the HrefPath is "/", Prefix returns an empty string.
func (r *Route) Prefix() string {
	var prefix = r.root.Leaf().HrefPath()
	if prefix == "/" {
		prefix = ""
	}
	return prefix
}

// Example: {{ .Include "/stuff" "footer" "body" }}
//
// basePath can be "."
func (r *Route) Include(basePath, command, resultName string) (template.HTML, error) {
	var result, err = r.include(basePath, command, resultName)
	return template.HTML(result), err
}

func (r *Route) include(basePath, command, resultName string) (string, error) {

	if r.includes == nil { // in dummy requests
		return "", nil
	}

	basePath = r.current.MakeAbsolute(basePath)

	if _, ok := r.includes[basePath][command]; !ok {

		// base

		var base *Node // do not modify this!
		var err error

		if basePath == r.current.HrefPath() {
			base = r.current
		} else {
			base, err = r.Request.Open(basePath) // returns the leaf
			if err != nil {
				return "", err
			}
			if base == nil {
				return "", ErrNotFound
			}
			// base could be cached, but cache invalidation might be difficult
		}

		// command

		includeRoute, err := NewRoute(r.Request, command)
		if err != nil {
			return "", err
		}

		includeRoute.Execute = true

		err = includeRoute.recurse(base, nil)
		if err != nil {
			return "", err
		}

		// cache results

		if _, ok := r.includes[basePath]; !ok {
			r.includes[basePath] = make(map[string]map[string]string)
		}

		r.includes[basePath][command] = includeRoute.results
	}

	if result, ok := r.includes[basePath][command][resultName]; ok {
		return result, nil
	} else {
		return "", ErrNotFound
	}
}

// IncludeBody calls Include(base, command, "body").
func (r *Route) IncludeBody(base, command string) (template.HTML, error) {
	return r.Include(base, command, "body")
}

// IncludeChild calls Include(".", command, resultName).
func (r *Route) IncludeChild(command, resultName string) (template.HTML, error) {
	return r.Include(".", command, resultName)
}

// IncludeChildBody calls Include(".", command, "body").
func (r *Route) IncludeChildBody(command string) (template.HTML, error) {
	return r.Include(".", command, "body")
}

// RootRecurse prepends "/root" to the queue and calls Recurse.
func (r *Route) RootRecurse() error {

	if r.Queue.Len() > 0 && (*r.Queue)[0].Key == "" { // like ":2/foo"
		(*r.Queue)[0].Key = "root"
	} else {
		r.Queue.Push("root")
	}

	return r.Recurse()
}

// Recurse takes the next slug from the queue, creates the corresponding node and executes its templates.
//
// It must be called explicitly in the user content because some things (global templates, push) should be done before and some things (like output) should be done after calling Recurse.
func (r *Route) Recurse() error {
	return r.recurse(r.current, r.current)
}

func (r *Route) recurse(parent, prev *Node) error {

	// When the template engine recovers from a panic, it displays an 404 error and logs the panic message.
	// This approach displays the panic message and logs a stack trace.
	defer func() {
		if val := recover(); val != nil {
			r.Set("body", fmt.Sprintf("<pre>%s</pre>", val))
			log.Printf("panic: %s\n%s", val, string(debug.Stack()))
			r.writer.WriteHeader(http.StatusInternalServerError)
		}
	}()

	if r.watchdog++; r.watchdog > 1000 {
		return fmt.Errorf("watchdog reached %d", r.watchdog)
	}

	// don't recurse twice
	if r.current != nil {
		if _, ok := r.calledRecurse[r.current.Id()]; ok {
			log.Printf("[%s] recurse had already been called", r.current.Slug())
			return nil
		}
		r.calledRecurse[r.current.Id()] = struct{}{}
	}

	if r.Queue.Len() == 0 {
		return nil
	}

	// store node in r.current

	var parentId = 0 // default parent id is always zero, because Node.Parent refers to the tree hierarchy
	if parent != nil {
		parentId = parent.Id()
	}

	var slug = (*r.Queue)[0].Key
	var currentVersionNr = (*r.Queue)[0].Version
	(*r.Queue) = (*r.Queue)[1:]

	var err error

	if currentVersionNr == DefaultVersion {
		if r.latest {
			r.current, err = r.db.GetLatestNode(parentId, slug)
		} else {
			r.current, err = r.db.GetReleasedNode(parentId, slug)
		}
	} else {
		r.current, err = r.db.GetVersionNode(parentId, slug, currentVersionNr)
	}
	if err != nil {
		return fmt.Errorf("error getting node (%d, %s): %v", parentId, slug, err)
	}

	r.current.Parent = parent
	r.current.Prev = prev

	if r.root == nil {
		r.root = r.current
	}

	// set prev and next
	if prev != nil {
		prev.Next = r.current
		r.current.Prev = prev
	}

	if err := r.current.RequirePermission(Read, r.User); err != nil {
		return err
	}

	if r.Execute {

		err = r.current.OnPrepare(r)
		if err != nil {
			r.Danger(err)
		}

		err = r.parseAndExecuteTemplates()
		if err != nil {
			r.Danger(err)
		}

		// Call Recurse if the user content didn't. This might mess up the output, but is still better than not recursing at all.

		if _, ok := r.calledRecurse[r.current.Id()]; !ok {
			log.Printf("[%s] calling recurse manually", r.current.Slug())
			r.Recurse()
		}

		// OnPrepare is only executed if we're expecting HTML
		if r.IsHTML() {
			err = r.current.OnExecute(r)
			if err != nil {
				r.Danger(err)
			}
		}
	} else {
		// don't call hooks and don't execute user content, so we must recurse ourselves
		r.Recurse()
	}

	// restore r.current
	r.current = prev

	return nil
}

func isGlobal(templateName string) bool {
	firstRune, _ := utf8.DecodeRuneInString(templateName)
	return unicode.IsUpper(firstRune)
}

func (r *Route) parseAndExecuteTemplates() error {

	// parse and execute the user content into templates (can contain variable assignments and queue modifications)

	parsed, err := template.New("body").Parse(r.current.Content())
	if err != nil {
		return err
	}

	var globalTemplates []*template.Template
	var localTemplates []*template.Template

	for _, t := range parsed.Templates() {
		if isGlobal(t.Name()) {
			globalTemplates = append(globalTemplates, t)
		} else {
			localTemplates = append(localTemplates, t)
		}
	}

	// parsed templates are still associated to each other, so it's enough to add the old global templates to one of the new localTemplates

	for _, oldGlobal := range r.templates {
		_, err = localTemplates[0].AddParseTree(oldGlobal.Name(), oldGlobal.Tree)
		if err != nil {
			return err
		}
	}

	// now add new globalTemplates to r.templates

	for _, newGlobal := range globalTemplates {
		r.templates[newGlobal.Name()], err = newGlobal.Clone()
		if err != nil {
			return err
		}
	}

	// execute local templates

	for _, t := range localTemplates {
		buf := &bytes.Buffer{}
		err := t.Execute(buf, r) // recursion is done here
		if err != nil {
			return fmt.Errorf("error executing template in %s: %v", r.current, err)
		}
		r.results[t.Name()] = buf.String() // unconditionally, not using Route.Set
	}

	return nil
}

// Get returns the result of a template execution from the previously processed node.
// The stored result is cleared.
//
// If IsHTML is false (i.e. the content type is not HTML),
// it returns an empty string as the return value will be thrown away anyway.
//
func (r *Route) Get(varName string) template.HTML {

	var val, _ = r.results[varName]
	delete(r.results, varName)

	if r.IsHTML() {
		return template.HTML(val)
	} else {
		return template.HTML("") // the return value will be thrown away anyway
	}
}

func (r *Route) VarNames() []string {
	names := make([]string, 0, len(r.results))
	for name := range r.results {
		names = append(names, name)
	}
	return names
}

// GetLocal returns the value of a local variable as HTML.
func (r *Route) GetLocal(varName string) template.HTML {
	return template.HTML(r.current.localVars[varName])
}

// GetLocal returns the value of a local variable as a string.
func (r *Route) GetLocalStr(varName string) string {
	return r.current.localVars[varName]
}

// Set sets the result of a template execution.
// If the content type is HTML, it will refuse to overwrite an existing result.
//
// Set is intended to be used in classes. Node content should use {{define}} instead.
func (r *Route) Set(name, value string) {

	if !r.IsHTML() && r.results[name] != "" {
		// don't overwrite content, which is probably JSON data or so
		return
	}

	if r.results[name] == "" {
		r.results[name] = value
	}
}

// SetLocal sets a local variable.
func (r *Route) SetLocal(name, value string) interface{} {
	r.current.localVars[name] = value
	return nil
}

// MakeIncludeOnly returns an error if the current node is in the main route.
// The effect is that the node can only be used as an include,
// but not be viewed directly.
func (r *Route) MakeIncludeOnly() (interface{}, error) {
	if r.current.isInMainRoute() {
		return nil, ErrNotFound
	}
	return nil, nil
}

// Tags applies one or more tags to the current node.
func (r *Route) Tag(tags ...string) interface{} {
	r.current.tags = append(r.current.tags, tags...)
	return nil
}

// Ts applies one or more timestamps to the current node.
// Arguments are parsed with util.ParseTime.
func (r *Route) Ts(dates ...string) interface{} {
	for _, dateStr := range dates {
		if ts, err := util.ParseTime(dateStr); err == nil {
			r.current.timestamps = append(r.current.timestamps, ts)
		}
	}
	return nil
}

// IsRootAdmin returns true if the user has admin permission for the root node.
func (r *Route) IsRootAdmin() bool {
	// node id 1 is more robust than Node.Parent.Parent..., which relies on the consistency of the Parent field
	if err := r.db.requirePermissionById(Admin, 1, r.User, nil); err == nil {
		return true
	} else {
		return false
	}
}
