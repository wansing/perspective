package upload

import (
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"
)

type Store interface {
	Folder(nodeID int) Folder
	HMAC(nodeID int, filename string, w int, h int, ts int64) string
	ServeHTTP(writer http.ResponseWriter, req *http.Request) // implementations will use HMAC and ParseUrl
}

// ParseUrl parses an url like "foo.jpg" or "bar/baz/foo.jpg?w=400&h=200".
func ParseUrl(u *url.URL) (path string, filename string, resize bool, w, h int, ts int64, sig []byte) {

	path, filename = pathpkg.Split(u.Path)
	path = strings.Trim(path, "/")
	filename = strings.TrimSpace(filename)

	w, _ = strconv.Atoi(u.Query().Get("w"))
	h, _ = strconv.Atoi(u.Query().Get("h"))
	resize = w != 0 || h != 0

	ts, _ = strconv.ParseInt(u.Query().Get("ts"), 10, 64)
	sig = []byte(u.Query().Get("sig"))

	return
}
