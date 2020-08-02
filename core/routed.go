package core

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
)

// responseWriter implements http.ResponseWriter.
type responseWriter struct {
	*Route
	*bytes.Buffer
}

// Header forwards to the real ResponseWriter.
func (w *responseWriter) Header() http.Header {
	return w.writer.Header()
}

// Write writes to the embedded buffer.
func (w *responseWriter) Write(b []byte) (int, error) {
	// redirect to the buffer
	if w.Buffer == nil {
		w.Buffer = &bytes.Buffer{}
	}
	return w.Buffer.Write(b)
}

// WriteHeader forwards to the real ResponseWriter.
// It prepends the path, so subrouters like Routed.Handler do HTTP redirects easily (e.g. to a cleaned version of the path, or to a version with removed or added trailing slashes).
func (w *responseWriter) WriteHeader(statusCode int) {
	if location := w.Header().Get("Location"); location != "" {
		w.Header().Set("Location", w.root.Leaf().HrefPath()+location)
	}
	w.writer.WriteHeader(statusCode)
}

// Routed adapts an http.Handler to our system. It should not have child nodes because it passes the whole queue to the handler.
type Routed struct {
	Base
	http.Handler
	path string // transfers the router path from OnPrepare to OnExecute
}

// OnPrepare just clears the queue, saving its string representation (path) for later.
func (t *Routed) OnPrepare(r *Route) error {
	t.path = r.Queue.String()
	r.Queue.Clear()
	return nil
}

// OnExecute is called immediately after OnPrepare because OnPrepare cleared the queue.
// It creates an http.Request struct and an http.ResponseWriter, calls Routed.Handler.ServeHTTP
// and writes the result to the "body" variable.
func (t *Routed) OnExecute(r *Route) error {

	var req = r.request.Clone(
		context.WithValue(
			r.request.Context(),
			routeContextKey{},
			r,
		),
	)

	u, err := url.Parse(t.path)
	if err != nil {
		return err
	}
	req.URL = u

	var w = &responseWriter{Route: r}

	t.Handler.ServeHTTP(w, req)

	r.Set("body", w.String()) // "body" variable should be empty before because Routed can not have child nodes
	return nil
}
