package core

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"runtime/debug"
)

const RootID = 1        // id of the root node
const RootSlug = "root" // slug of the root node

// see https://golang.org/pkg/context/#WithValue
type routeContextKey struct{}

var ErrNotFound = errors.New("not found")

// A Route executes one queue, which can be the main queue or an included queue.
//
// For usage in templates, funcs on Route must return "one return value (of any type) or two return values, the second of which is an error."
type Route struct {
	*Queue
	*Request
	*Node                        // current node
	*Version                     // version of current node
	Next     []NodeVersion       // executed nodes
	Recursed map[int]interface{} // avoids double recursion
	RootURL  string              // "/" for main route, "/" or include base for included routes, TODO use it
	Vars     map[string]string   // node output
	VarDepth map[string]int      // must be stored for each var separately
}

// RouteFromContext gets a Route from the given context. It can panic.
func RouteFromContext(ctx context.Context) *Route {
	return ctx.Value(routeContextKey{}).(*Route)
}

// Example: {{ .Include "/stuff" "footer" "body" }}
//
// baseLocation can be "."
func (r *Route) Include(baseLocation, command, varName string) (template.HTML, error) {
	if !path.IsAbs(baseLocation) {
		baseLocation = path.Join(r.Node.Location(), baseLocation)
	}
	var v, err = r.include(baseLocation, command, varName)
	return template.HTML(v), err
}

func (r *Route) include(baseLocation, command, varName string) (string, error) {

	if r.includes == nil { // in dummy requests
		return "", nil
	}

	if _, ok := r.includes[baseLocation][command]; !ok {

		// base

		base, err := r.Open(baseLocation) // returns the leaf
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

		if err = includeRoute.pop(base); err != nil {
			return "", err
		}

		// cache vars

		if _, ok := r.includes[baseLocation]; !ok {
			r.includes[baseLocation] = make(map[string]map[string]string)
		}

		r.includes[baseLocation][command] = includeRoute.Vars
	}

	if v, ok := r.includes[baseLocation][command][varName]; ok {
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
	return r.pop(r.Node)
}

func (r *Route) pop(parent *Node) error {

	// make this function idempotent

	if r.Recursed == nil {
		r.Recursed = make(map[int]interface{})
	}

	if _, ok := r.Recursed[parent.ID()]; ok {
		return nil // already done
	}
	r.Recursed[parent.ID()] = struct{}{}

	// When the template engine recovers from a panic, it displays an 404 error and logs the panic message.
	// This displays the panic message and logs a stack trace.

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

	// get node

	var n *Node
	var err error

	if r.Queue.Len() == 0 {
		// try default
		n, err = r.Request.db.GetNodeBySlug(parent, "default")
		if err != nil {
			return nil // no problem, it was just a try
		}
	} else {
		var slug, _ = r.Queue.Pop()
		n, err = r.Request.db.GetNodeBySlug(parent, slug)
		if err != nil {
			// restore queue and resort to default
			r.Queue.push(slug)
			n, err = r.Request.db.GetNodeBySlug(parent, "default")
			if err != nil {
				return fmt.Errorf("pop (%d, %s): %w", parent.ID(), slug, err) // neither slug nor default were found
			}
		}
	}

	n.Parent = parent

	// check permission

	if err := n.RequirePermission(Read, r.User); err != nil {
		return err
	}

	// get version

	v, err := n.GetVersion(n.MaxWGZeroVersionNo()) // could use specific version for preview
	if err != nil {
		return err
	}

	// setup context

	var oldNode = r.Node
	var oldVersion = r.Version

	r.Node = n
	r.Version = v

	// run class code

	if err := n.Do(r); err != nil {
		r.Danger(err)
	}

	r.Node = oldNode
	r.Version = oldVersion
	r.Next = append([]NodeVersion{{n, v}}, r.Next...)

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
