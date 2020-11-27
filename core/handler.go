package core

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
)

// responseWriter implements http.ResponseWriter.
type responseWriter struct {
	*bytes.Buffer
	prefix string
	writer http.ResponseWriter
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
// It prepends the path, so subrouters like Handler.Handler do HTTP redirects easily (e.g. to a cleaned version of the path, or to a version with removed or added trailing slashes).
func (w *responseWriter) WriteHeader(statusCode int) {
	// it is easier to modify the Location header now instead of wrapping http.Header
	if location := w.Header().Get("Location"); location != "" {
		// we're not using path.Join because we must keep trailing slash
		var joined = strings.ReplaceAll(w.prefix+location, "//", "/")
		w.Header().Set("Location", joined)
	}
	w.writer.WriteHeader(statusCode)
}

// Handler adapts an http.Handler to our system. It should not have child nodes because it passes the whole queue to the handler.
type Handler struct {
	http.Handler
}

func (t *Handler) AddSlugs() []string {
	return nil
}

// Empties the queue, creates an http.Request struct and an http.ResponseWriter, calls Handler.Handler.ServeHTTP and writes the result to the "body" variable.
func (t *Handler) Do(r *Query) error {

	var path = r.Queue.String()
	r.Queue = &Queue{} // clear queue

	// no need to call r.Recurse because the queue is empty anyway

	var req = r.request.Clone(
		context.WithValue(
			r.request.Context(),
			queryContextKey{},
			r,
		),
	)

	if u, err := url.Parse(path); err == nil {
		req.URL = u
	} else {
		return err
	}

	var writer = &responseWriter{
		prefix: r.Node.Link(),
		writer: r.writer,
	}

	t.Handler.ServeHTTP(writer, req)

	r.Set("body", writer.String())
	return nil
}
