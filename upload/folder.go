package upload

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// one Folder for one node
type Folder interface {
	Delete(filename string) error
	NodeID() int
	Files() ([]os.FileInfo, error)
	HasFile(filename string) (bool, error)
	Upload(filename string, src io.Reader) error
}

func CleanFilename(filename string) (string, error) {
	filename = filepath.Base(filename)
	filename = strings.TrimSpace(filename)
	if strings.Contains(filename, "/") || strings.Contains(filename, `\`) {
		return "", errors.New("filename contains a slash")
	}
	if filename == "" {
		return "", errors.New("filename is empty")
	}
	return filename, nil
}

// Creates an HMAC of a resized uploaded file. Store implementations can use it to prevent DoS attacks on image resizing.
func HMAC(secret []byte, nodeID int, filename string, w int, h int, ts int64) string {

	buf := make([]byte, 32)
	binary.PutVarint(buf[0:], int64(nodeID))
	binary.PutVarint(buf[8:], ts)
	binary.PutVarint(buf[16:], int64(w))
	binary.PutVarint(buf[24:], int64(h))
	buf = append(buf, []byte(filename)...)

	hash := hmac.New(sha256.New, secret)
	hash.Write(buf)
	return base64.URLEncoding.EncodeToString(hash.Sum(nil))
}
