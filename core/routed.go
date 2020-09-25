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
		w.Header().Set("Location", w.Node.Leaf().HrefPath()+location)
	}
	w.writer.WriteHeader(statusCode)
}

// Routed adapts an http.Handler to our system. It should not have child nodes because it passes the whole queue to the handler.
type Handler struct {
	Base
	http.Handler
}

// Empties the queue, creates an http.Request struct and an http.ResponseWriter, calls Routed.Handler.ServeHTTP and writes the result to the "body" variable.
func (t *Handler) Do(r *Route) error {

	var path = r.Queue.String()
	r.Queue = nil // clear queue

	// no need to call r.Recurse because the queue is empty anyway

	var req = r.request.Clone(
		context.WithValue(
			r.request.Context(),
			routeContextKey{},
			r,
		),
	)

	if u, err := url.Parse(path); err == nil {
		req.URL = u
	} else {
		return err
	}

	var writer = &responseWriter{Route: r}

	t.Handler.ServeHTTP(writer, req)

	r.Set("body", writer.String())
	return nil
}
