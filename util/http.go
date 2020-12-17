package util

import (
	"net/http"
	"strings"
)

type prefixedResponseWriter struct {
	http.ResponseWriter
	prefix string // without trailing slash
}

// WriteHeader shadows and calls http.ResponseWriter.WriteHeader.
func (w prefixedResponseWriter) WriteHeader(statusCode int) {
	// modify Location header, absolute locations only
	if w.prefix != "" {
		if location := w.Header().Get("Location"); len(location) > 0 && location[0] == '/' {
			w.Header().Set("Location", w.prefix+location)
		}
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func HandlePrefix(mux *http.ServeMux, prefix string, handler http.Handler) {
	prefix = strings.TrimSuffix(prefix, "/")
	mux.Handle(
		prefix+"/", // http mux needs trailing slash
		http.StripPrefix(
			prefix,
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w = &prefixedResponseWriter{w, prefix}
					handler.ServeHTTP(w, r)
				},
			),
		),
	)
}
