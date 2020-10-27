package filestore

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/wansing/perspective/upload"
)

// implements upload.Folder
type Folder struct {
	store  *Store
	nodeID int
}

func (f Folder) stattPattern(w, h int, filename string) string {
	return fmt.Sprintf("%s/%d_%d_%d_%s", f.store.CacheDir, f.nodeID, w, h, filename)
}

func (f Folder) stattPatternAll(filename string) string {
	return fmt.Sprintf("%s/%d_*_*_%s", f.store.CacheDir, f.nodeID, filename)
}

func (f Folder) uploadsFs() string {
	return fmt.Sprintf(f.store.UploadDir+"/%d/", f.nodeID)
}

func (f Folder) Delete(filename string) error {

	filename, err := upload.CleanFilename(filename)
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(f.uploadsFs(), filename))
	if err != nil {
		return err
	}

	cacheds, err := filepath.Glob(f.stattPatternAll(filename))
	if err != nil {
		return err
	}

	for _, cached := range cacheds {
		err = os.Remove(cached)
		if err != nil {
			return err
		}
	}

	_ = os.Remove(f.uploadsFs()) // try to remove folder, works only if the folder is empty
	return nil
}

func (f Folder) NodeID() int {
	return f.nodeID
}

func (f Folder) Files() ([]os.FileInfo, error) {
	files, err := ioutil.ReadDir(f.uploadsFs())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // assuming the folder was deleted because it was empty
		} else {
			return nil, err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	return files, nil
}

func (f Folder) HasFile(filename string) (bool, error) {
	filename, err := upload.CleanFilename(filename)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(filepath.Join(f.uploadsFs(), filename)); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (f Folder) Upload(filename string, src io.Reader) error {

	filename, err := upload.CleanFilename(filename)
	if err != nil {
		return err
	}

	err = os.MkdirAll(f.uploadsFs(), 0755) // 755 is required if the webserver runs as a different user
	if err != nil {
		return err
	}

	has, err := f.HasFile(filename)
	if err != nil {
		return err
	}
	if has {
		return errors.New("file already exists")
	}

	dst, err := os.Create(filepath.Join(f.uploadsFs(), filename)) //  creates or truncates the named file, umask 0666
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// implements upload.Store
type Store struct {
	CacheDir   string // will contain just files
	UploadDir  string // will contain folders whose names are node ids
	HMACSecret []byte
	Resizer    JPEGResizer
}

func (s *Store) Folder(nodeID int) upload.Folder {
	return &Folder{
		store:  s,
		nodeID: nodeID,
	}
}

func (s *Store) HMAC(nodeID int, filename string, w int, h int, ts int64) string {
	return upload.HMAC(s.HMACSecret, nodeID, filename, w, h, ts)
}

func (s *Store) ServeHTTP(writer http.ResponseWriter, req *http.Request) {

	path, filename, resize, w, h, ts, sig := upload.ParseUrl(req.URL) // req.URL seems to be always relative

	var nodeID, err = strconv.Atoi(path)
	if err != nil {
		http.NotFound(writer, req)
		return
	}
	var location = s.Folder(nodeID).(*Folder)

	original := location.uploadsFs() + filename

	// serve original file if resizing is not requested

	if !resize {
		http.ServeFile(writer, req, original)
		return
	}

	// HMAC to avoid DoS attacks, deny access if timestamp is older than one day

	if !hmac.Equal([]byte(s.HMAC(location.nodeID, filename, w, h, ts)), sig) {
		http.NotFound(writer, req)
		return
	}

	if ts+86400 < time.Now().Unix() {
		http.NotFound(writer, req)
		return
	}

	// requested filename reflects the parsed URL

	requested := location.stattPattern(w, h, filename)

	// create requested file (as a symlink to the canonical filename if required)

	if _, err := os.Stat(requested); os.IsNotExist(err) {

		// get original dimensions and assemble canonical filename (with real width and height)

		originalFile, err := os.Open(original)
		if err != nil {
			http.NotFound(writer, req)
			return
		}

		originalImage, _, err := image.DecodeConfig(originalFile)
		if err != nil {
			http.NotFound(writer, req)
			return
		}

		// URL parameters w and h are taken as max, no distortion allowed

		var scalingRatio float32 = 1.0

		if w != 0 {
			scalingRatio = float32(w) / float32(originalImage.Width)
		}

		if h != 0 {
			scalingRatioH := float32(h) / float32(originalImage.Height)
			if scalingRatio > scalingRatioH {
				scalingRatio = scalingRatioH
			}
		}

		if scalingRatio >= 1.0 {

			// don't scale up, symlink to original image instead

			err := os.Symlink(original, requested)
			if err != nil {
				http.NotFound(writer, req)
				return
			}

		} else {

			w = int(float32(originalImage.Width) * scalingRatio)
			h = int(float32(originalImage.Height) * scalingRatio)

			// w and h are always genuine (and especially non-zero) now

			var canonical = location.stattPattern(w, h, filename) // with real width and height

			// resize

			if _, err := os.Stat(canonical); os.IsNotExist(err) { // if canonical file doesn't exist
				if err := s.Resizer.Resize(original, canonical, w, h); err != nil {
					log.Printf("error resizing: %v", err)
				}
			}

			// symlink canonical filename to requested filename, if necessary

			if canonical != requested {
				err := os.Symlink(canonical, requested)
				if err != nil {
					http.NotFound(writer, req)
					return
				}
			}
		}
	}

	http.ServeFile(writer, req, requested)
	return
}

type JPEGResizer interface {
	Name() string
	Resize(original, resized string, width, height int) error
}

type ImageMagick struct{}

func (ImageMagick) Name() string {
	return "ImageMagick"
}

func (ImageMagick) Resize(original, resized string, width, height int) error {
	resizeArg := fmt.Sprintf("%dx%d>", width, height)
	args := []string{original, "-resize", resizeArg, "-quality", "85", resized} // ">" means "Only Shrink Larger Images"
	return exec.Command("convert", args...).Run()
}

type Vips struct{}

func (Vips) Name() string {
	return "vips"
}

// JPEG Quality: https://github.com/libvips/libvips/issues/571#issuecomment-268031545
// vips resize vs vips thumbnail: https://github.com/libvips/libvips/issues/571#issuecomment-270430879
func (Vips) Resize(original, resized string, width, height int) error {
	args := []string{"thumbnail", original, resized + `[Q=85]`, strconv.Itoa(width), "--height", strconv.Itoa(height), "--size", "down"}
	return exec.Command("vips", args...).Run()
}

func FindResizer() (JPEGResizer, error) {
	if _, err := exec.LookPath("vips"); err == nil {
		return Vips{}, nil
	} else if _, err := exec.LookPath("convert"); err == nil {
		return ImageMagick{}, nil
	} else {
		return nil, errors.New("no JPEG resizer found")
	}
}
