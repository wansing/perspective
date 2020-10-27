package core

import (
	"strings"
)

// A Queue stores the slugs that have not been processed yet.
// We don't care whether there is trailing slash, but some http.Handlers do (used in core.Handler) and redirect accordingly, so we must keep that information.
type Queue struct {
	Slugs         []string
	TrailingSlash bool
}

// NewQueue creates a queue from the given path.
// Subsequent slashes are collapsed.
func NewQueue(path string) *Queue {
	slugs := strings.FieldsFunc(path, func(c rune) bool {
		return c == 47 // slash
	})
	var q = &Queue{
		Slugs:         slugs,
		TrailingSlash: len(slugs) > 0 && strings.HasSuffix(path, "/"),
	}
	return q
}

func (q *Queue) HasPrefix(slug string) bool {
	return q.Len() > 0 && q.Slugs[0] == slug
}

func (q *Queue) IsEmpty() bool {
	return q.Len() == 0
}

func (q *Queue) Len() int {
	return len(q.Slugs)
}

func (q *Queue) Pop() (slug string, ok bool) {
	if q.Len() > 0 {
		slug, q.Slugs = q.Slugs[0], q.Slugs[1:]
		ok = true
	}
	return
}

func (q *Queue) PopIf(slug string) bool {
	if q.Len() > 0 && q.Slugs[0] == slug {
		q.Pop()
		return true
	}
	return false
}

func (q *Queue) push(slug string) interface{} {
	q.Slugs = append([]string{slug}, q.Slugs...)
	return nil
}

func (q *Queue) String() string {
	var str = "/" + strings.Join(q.Slugs, "/")
	if q.TrailingSlash && !strings.HasSuffix(str, "/") {
		str = str + "/"
	}
	return str
}
