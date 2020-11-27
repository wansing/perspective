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
type queryContextKey struct{}

var ErrNotFound = errors.New("not found")

// A Query is the execution of one queue, which can be the main queue or an included queue.
type Query struct {
	*Queue
	*Request
	*Node                      // current node
	*Version                   // version of current node
	Next     []NodeVersion     // executed nodes
	RootURL  string            // "/" for main query, "/" or include base for included queries, TODO unused at the moment
	Vars     map[string]string // node output
	VarDepth map[string]int    // must be stored for each var separately
}

// QueryFromContext gets a Query from the given context. It can panic.
func QueryFromContext(ctx context.Context) *Query {
	return ctx.Value(queryContextKey{}).(*Query)
}

// Include executes a command, starting in a base node (default "."), and returns a variable (default "body").
//
//  Include(command)
//  Include(base, command)
//  Include(base, command, varName)
//
// Example:
//
//  {{.Include "/legal" "disclaimer" "body"}}
func (r *Query) Include(args ...string) (template.HTML, error) {

	var baseLocation, command, varName string
	switch len(args) {
	case 1:
		baseLocation = "."
		command = args[0]
		varName = "body"
	case 2:
		baseLocation = args[0]
		command = args[1]
		varName = "body"
	case 3:
		baseLocation = args[0]
		command = args[1]
		varName = args[2]
	default:
		return template.HTML(""), errors.New("Include: wrong number of arguments")
	}

	if !path.IsAbs(baseLocation) {
		baseLocation = path.Join(r.Node.Location(), baseLocation)
	}

	if r.includes == nil { // in dummy requests
		return template.HTML(""), nil
	}

	if _, ok := r.includes[baseLocation][command]; !ok {

		// base

		base, err := r.Open(baseLocation) // returns the leaf
		if err != nil {
			return template.HTML(""), err
		}
		if base == nil {
			return template.HTML(""), ErrNotFound
		}
		// base could be cached, but cache invalidation might be difficult

		// command

		includeQuery := &Query{
			Node:    base,
			Request: r.Request,
			Queue:   NewQueue(command),
		}

		if err = includeQuery.Recurse(); err != nil {
			return template.HTML(""), err
		}

		// cache vars

		if _, ok := r.includes[baseLocation]; !ok {
			r.includes[baseLocation] = make(map[string]map[string]string)
		}

		r.includes[baseLocation][command] = includeQuery.Vars
	}

	if v, ok := r.includes[baseLocation][command][varName]; ok {
		return template.HTML(v), nil
	} else {
		return template.HTML(""), fmt.Errorf("include %s: var %s %w", command, varName, ErrNotFound)
	}
}

// Recurse takes the next slug from the queue, creates the corresponding node and executes it.
func (r *Query) Recurse() error {

	// make this function idempotent

	if r.recursed == nil {
		r.recursed = make(map[int]interface{})
	}

	if _, ok := r.recursed[r.Node.ID()]; ok {
		return nil // already done
	}
	r.recursed[r.Node.ID()] = struct{}{}

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
		n, err = r.Request.db.GetNodeBySlug(r.Node, "default")
		if err != nil {
			return nil // no problem, it was just a try
		}
	} else {
		var slug, _ = r.Queue.Pop()
		n, err = r.Request.db.GetNodeBySlug(r.Node, slug)
		if err != nil {
			// restore queue and resort to default
			r.Queue.push(slug)
			n, err = r.Request.db.GetNodeBySlug(r.Node, "default")
			if err != nil {
				return fmt.Errorf("pop (%d, %s): %w", r.Node.ID(), slug, err) // neither slug nor default were found
			}
		}
	}

	n.Parent = r.Node

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
func (r *Query) Get(varName string) template.HTML {

	var val, _ = r.Vars[varName]
	delete(r.Vars, varName)

	if r.IsHTML() {
		return template.HTML(val)
	} else {
		return template.HTML("") // the return value will be thrown away anyway
	}
}

// Set sets a variable if it is empty or if the current node is deeper than the origin of the existing value.
func (r *Query) Set(name, value string) {

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
