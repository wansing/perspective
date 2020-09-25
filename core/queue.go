package core

import (
	"strings"
)

const DefaultVersion = 0 // latest version or latest release, depending on where it's used

type Queue []string

func NewQueue(path string) *Queue {

	fields := strings.FieldsFunc(path, func(c rune) bool {
		return c == 47 // slash
	})

	q := &Queue{RootSlug} // prepend RootSlug
	*q = append(*q, fields...)

	return q
}

func (q *Queue) HasPrefix(slug string) bool {
	return q.Len() > 0 && (*q)[0] == slug
}

func (q *Queue) IsEmpty() bool {
	return len(*q) == 0
}

func (q *Queue) Len() int {
	return len(*q)
}

func (q *Queue) Pop() (slug string, ok bool) {
	if q.Len() > 0 {
		slug, (*q) = (*q)[0], (*q)[1:]
		ok = true
	}
	return
}

func (q *Queue) PopIf(slug string) bool {
	if q.Len() > 0 && (*q)[0] == slug {
		q.Pop()
		return true
	}
	return false
}

func (q *Queue) push(slug string) interface{} {
	*q = append([]string{slug}, *q...)
	return nil
}

func (q *Queue) String() string {
	return "/" + strings.Join(*q, "/")
}
