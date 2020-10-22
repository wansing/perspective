package upload

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type Store interface {
	Folder(nodeID int) Folder
	HMAC(nodeID int, filename string, w int, h int, ts int64) string
	ServeHTTP(writer http.ResponseWriter, req *http.Request) // implementations will use HMAC and ParseUploadUrl
}

// ParseUrl parses an url like "foo.jpg" or "100/foo.jpg?w=400&h=200".
func ParseUrl(store Store, defaultFolder Folder, u *url.URL) (isUpload bool, uploadLocation Folder, filename string, resize bool, w, h int, ts int64, sig []byte, err error) {

	dir, filename := path.Split(u.Path)

	// strip slashes from dir

	dir = strings.Trim(dir, "/")

	// u.Path like "foo.jpg"

	if dir == "" && defaultFolder != nil {
		var has bool
		has, err = defaultFolder.HasFile(filename)
		if err != nil {
			return
		}
		if has {
			isUpload = true
			uploadLocation = defaultFolder
		}
	}

	// u.Path like "123/foo.jpg"

	if dir != "" {
		if nodeID, err := strconv.Atoi(dir); nodeID > 0 && err == nil {
			isUpload = true
			uploadLocation = store.Folder(nodeID)
		}
	}

	// process u.Query()

	if isUpload {

		filename = strings.TrimSpace(filename)
		if filename == "" {
			isUpload = false
			return
		}

		// search for query keys w and h

		if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
			w, _ = strconv.Atoi(u.Query().Get("w"))
			h, _ = strconv.Atoi(u.Query().Get("h"))
			resize = w != 0 || h != 0
		}

		// other parameters

		ts, _ = strconv.ParseInt(u.Query().Get("ts"), 10, 64)
		sig = []byte(u.Query().Get("sig"))
	}

	return
}
