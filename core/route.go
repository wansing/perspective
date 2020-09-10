package core

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/wansing/perspective/util"
)

const RootId = 1        // id of the root node
const RootSlug = "root" // slug of the root node

// see https://golang.org/pkg/context/#WithValue
type routeContextKey struct{}

var ErrNotFound = errors.New("not found")

// A route processes one queue, which can be the main queue or an included queue.
//
// The route is passed to the content template.
//
// For usage in templates, funcs on Route must return "one return value (of any type) or two return values, the second of which is an error."
type Route struct {
	*Queue
	*Request
	*Node                      // current node, used by template funcs only
	Vars     map[string]string // results of template execution
	VarDepth map[string]int    // must be stored for each var separately
}

// RouteFromContext gets a Route from the given context. It can panic.
func RouteFromContext(ctx context.Context) *Route {
	return ctx.Value(routeContextKey{}).(*Route)
}

// T returns the instance of the current node.
func (r *Route) T() interface{} {
	return r.Node.Instance
}

// Example: {{ .Include "/stuff" "footer" "body" }}
//
// basePath can be "."
func (r *Route) Include(basePath, command, varName string) (template.HTML, error) {
	basePath = r.Node.MakeAbsolute(basePath)
	var v, err = r.include(basePath, command, varName)
	return template.HTML(v), err
}

func (r *Route) include(basePath, command, varName string) (string, error) {

	if r.includes == nil { // in dummy requests
		return "", nil
	}

	if _, ok := r.includes[basePath][command]; !ok {

		// base

		base, err := r.Open(basePath) // returns the leaf
		if err != nil {
			return "", err
		}
		if base == nil {
			return "", ErrNotFound
		}
		// base could be cached, but cache invalidation might be difficult

		// command

		includeRoute := &Route{
			Request: r.Request,
			Queue:   NewQueue(command),
		}

		if err = includeRoute.pop(base, nil); err != nil {
			return "", err
		}

		// cache vars

		if _, ok := r.includes[basePath]; !ok {
			r.includes[basePath] = make(map[string]map[string]string)
		}

		r.includes[basePath][command] = includeRoute.Vars
	}

	if v, ok := r.includes[basePath][command][varName]; ok {
		return v, nil
	} else {
		return "", fmt.Errorf("including %s: var %s %w", command, varName, ErrNotFound)
	}
}

// IncludeBody calls Include(base, command, "body").
func (r *Route) IncludeBody(base, command string) (template.HTML, error) {
	return r.Include(base, command, "body")
}

// IncludeChild calls Include(".", command, varName).
func (r *Route) IncludeChild(command, varName string) (template.HTML, error) {
	return r.Include(".", command, varName)
}

// IncludeChildBody calls Include(".", command, "body").
func (r *Route) IncludeChildBody(command string) (template.HTML, error) {
	return r.Include(".", command, "body")
}

// Recurse takes the next slug from the queue, creates the corresponding node and executes its templates.
//
// It must be called explicitly in the user content because some things (global templates, push) should be done before and some things (like output) should be done after calling Recurse.
//
// Because Recurse can be called in a template, r.Node must be set.
func (r *Route) Recurse() error {
	return r.pop(r.Node, r.Node)
}

// idempotent if prev != nil
func (r *Route) pop(parent, prev *Node) error {

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

	if prev != nil && prev.Next != nil {
		// Recurse has already been called
		return nil
	}

	if r.Queue.Len() == 0 {
		return nil
	}

	// get node

	var parentId = 0 // default parent id is always zero, because Node.Parent refers to the tree hierarchy
	if parent != nil {
		parentId = parent.Id()
	}

	var slug = (*r.Queue)[0].Key
	var versionNo = (*r.Queue)[0].Version
	(*r.Queue) = (*r.Queue)[1:]

	var err error
	var n *Node

	if versionNo == DefaultVersion {
		n, err = r.Request.db.GetReleasedNode(parent, slug)
	} else {
		n, err = r.Request.db.GetVersionNode(parent, slug, versionNo)
	}
	if err != nil {
		return fmt.Errorf("recurse (%d, %s): %w", parentId, slug, err) // %w wraps err
	}

	n.Parent = parent
	n.Prev = prev

	if prev != nil {
		prev.Next = n
	}

	if err := n.RequirePermission(Read, r.User); err != nil {
		return err
	}

	defer func(old *Node) {
		r.Node = old
	}(r.Node) // r.Node can be nil

	r.Node = n // for template execution

	if err := n.Do(r); err != nil {
		return err
	}

	return nil
}

// Get returns the value of a variable and clears it.
//
// If IsHTML is false (i.e. the content type is not HTML), it returns an empty string as the return value will be thrown away anyway.
func (r *Route) Get(varName string) template.HTML {

	var val, _ = r.Vars[varName]
	delete(r.Vars, varName)

	if r.IsHTML() {
		return template.HTML(val)
	} else {
		return template.HTML("") // the return value will be thrown away anyway
	}
}

// Set sets a variable if it is empty or if the current node is deeper than the origin of the existing value.
func (r *Route) Set(name, value string) {

	if !r.IsHTML() && r.Vars[name] != "" {
		// don't overwrite content, which is probably JSON data or so
		return
	}

	if r.Vars == nil {
		r.Vars = make(map[string]string)
	}

	if r.VarDepth == nil {
		r.VarDepth = make(map[string]int)
	}

	if r.Vars[name] == "" || r.Node.Depth() > r.VarDepth[name] { // set if old value is empty (e.g. has been fetched using Get) or if the new value comes from a deeper node
		r.Vars[name] = value
		r.VarDepth[name] = r.Node.Depth()
	}
}

// MakeIncludeOnly returns an error if the current node is in the main route.
// The effect is that the node can only be used as an include,
// but not be viewed directly.
func (r *Route) MakeIncludeOnly() (interface{}, error) {
	if r.Node.Root().Id() == RootId {
		return nil, ErrNotFound
	}
	return nil, nil
}

// Tags applies one or more tags to the current node.
func (r *Route) Tag(tags ...string) interface{} {
	r.Node.Tags = append(r.Node.Tags, tags...)
	return nil
}

// Ts applies one or more timestamps to the current node.
// Arguments are parsed with util.ParseTime.
func (r *Route) Ts(dates ...string) interface{} {
	for _, dateStr := range dates {
		if ts, err := util.ParseTime(dateStr); err == nil {
			r.Node.Timestamps = append(r.Node.Timestamps, ts)
		}
	}
	return nil
}
