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
	vars     map[string]string // node output
	varDepth map[string]int    // must be stored for each var separately
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
func (q *Query) Include(args ...string) (template.HTML, error) {

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
		baseLocation = path.Join(q.Node.Location(), baseLocation)
	}

	if q.includes == nil { // in dummy requests
		return template.HTML(""), nil
	}

	if _, ok := q.includes[baseLocation][command]; !ok {

		// base

		base, err := q.Open(baseLocation) // returns the leaf
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
			Request: q.Request,
			Queue:   NewQueue(command),
		}

		if err = includeQuery.Recurse(); err != nil {
			return template.HTML(""), err
		}

		// cache vars

		if _, ok := q.includes[baseLocation]; !ok {
			q.includes[baseLocation] = make(map[string]map[string]string)
		}

		q.includes[baseLocation][command] = includeQuery.vars
	}

	if v, ok := q.includes[baseLocation][command][varName]; ok {
		return template.HTML(v), nil
	} else {
		return template.HTML(""), fmt.Errorf("include %s: var %s %w", command, varName, ErrNotFound)
	}
}

// Recurse takes the next slug from the queue, creates the corresponding node and runs it.
func (q *Query) Recurse() error {

	// make this function idempotent

	if q.recursed == nil {
		q.recursed = make(map[int]interface{})
	}

	if _, ok := q.recursed[q.Node.ID()]; ok {
		return nil // already done
	}
	q.recursed[q.Node.ID()] = struct{}{}

	if q.watchdog++; q.watchdog > 1000 {
		return fmt.Errorf("watchdog reached %d", q.watchdog)
	}

	// get node

	var n *Node
	var err error

	if q.Queue.Len() == 0 {
		// try default
		n, err = q.Request.db.GetNodeBySlug(q.Node, "default")
		if err != nil {
			return nil // no problem, it was just a try
		}
	} else {
		var slug, _ = q.Queue.Pop()
		n, err = q.Request.db.GetNodeBySlug(q.Node, slug)
		if err != nil {
			// restore queue and resort to default
			q.Queue.push(slug)
			n, err = q.Request.db.GetNodeBySlug(q.Node, "default")
			if err != nil {
				return fmt.Errorf("get node %s/%s: %w", q.Node.Location(), slug, err) // neither slug nor default were found
			}
		}
	}

	n.Parent = q.Node

	// check permission

	if err := n.RequirePermission(Read, q.User); err != nil {
		return err
	}

	// get version

	v, err := n.GetVersion(n.MaxWGZeroVersionNo()) // could use specific version for preview
	if err != nil {
		return err
	}

	// setup context and run the new node

	var oldNode = q.Node
	var oldVersion = q.Version

	q.Node = n
	q.Version = v

	if err := q.Run(); err != nil {
		q.Danger(err)
	}

	q.Node = oldNode
	q.Version = oldVersion
	q.Next = append([]NodeVersion{{n, v}}, q.Next...)

	return nil
}

// Runs q.Node.
func (q *Query) Run() error {

	if q.watchdog++; q.watchdog > 1000 {
		return fmt.Errorf("watchdog reached %d", q.watchdog)
	}

	// recover before rootTemplate does, displays the panic message and log a stack trace

	defer func() {
		if val := recover(); val != nil {
			q.Set("body", fmt.Sprintf("<pre>%s</pre>", val))
			log.Printf("panic: %s\n%s", val, string(debug.Stack()))
			q.writer.WriteHeader(http.StatusInternalServerError)
		}
	}()

	return q.Node.Class().Run(q)
}

// Get returns the value of a variable. If q.IsHTML is true, then the value is cleared.
func (q *Query) Get(varName string) template.HTML {
	var val, _ = q.vars[varName]
	if q.IsHTML() {
		delete(q.vars, varName)
	}
	return template.HTML(val) // if not HTML, then the return value might be thrown away in other nodes, but the root handler still needs it
}

// Set sets a variable if it is empty or if the current node is deeper than the origin of the existing value.
func (q *Query) Set(name, value string) {

	if !q.IsHTML() && q.vars[name] != "" {
		// don't overwrite content, which is probably JSON data or so
		return
	}

	if q.vars == nil {
		q.vars = make(map[string]string)
	}

	if q.varDepth == nil {
		q.varDepth = make(map[string]int)
	}

	if q.vars[name] == "" || q.Node.Depth() > q.varDepth[name] { // set if old value is empty (e.g. has been fetched using Get) or if the new value comes from a deeper node
		q.vars[name] = value
		q.varDepth[name] = q.Node.Depth()
	}
}
