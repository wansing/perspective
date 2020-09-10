package core

import (
	pathpkg "path"
	"strconv"
	"strings"
)

const DefaultVersion = 0 // latest version or latest release, depending on where it's used

type QueueSegment struct {
	Key     string // must not be empty
	Version int    // default value: 0
	pushed  bool
}

type Queue []QueueSegment

func NewQueue(path string) *Queue {

	fields := strings.FieldsFunc(path, func(c rune) bool {
		return c == 47 // slash
	})

	queue := &Queue{}

	for _, key := range fields {
		var version int
		if colon := strings.Index(key, ":"); colon != -1 {
			version, _ = strconv.Atoi(key[colon+1:])
			key = key[:colon]
		}
		key = strings.TrimSpace(key)
		*queue = append(*queue, QueueSegment{Key: key, Version: version})
	}

	// prepend RootSlug

	if pathpkg.IsAbs(path) {
		if queue.Len() > 0 && (*queue)[0].Key == "" { // like ":2/foo"
			(*queue)[0].Key = RootSlug
		} else {
			queue.Push(RootSlug)
		}
	}

	return queue
}

func (q *Queue) HasPrefix(key string) bool {
	return q.Len() > 0 && (*q)[0].Key == key
}

func (q *Queue) IsEmpty() bool {
	return len(*q) == 0
}

func (q *Queue) Len() int {
	return len(*q)
}

func (q *Queue) Pop() (segment QueueSegment, ok bool) {
	if q.Len() > 0 {
		segment, (*q) = (*q)[0], (*q)[1:]
		ok = true
	}
	return
}

func (q *Queue) PopIf(key string) bool {
	if q.Len() > 0 && (*q)[0].Key == key {
		q.Pop()
		return true
	}
	return false
}

func (q *Queue) Push(slug string) interface{} {
	*q = append([]QueueSegment{{Key: slug, Version: DefaultVersion, pushed: true}}, *q...)
	return nil
}

func (q *Queue) PushIfEmpty(slug string) interface{} {
	if q.Len() == 0 {
		q.Push(slug)
	}
	return nil
}

func (q *Queue) PushIfNotEmpty(slug string) interface{} {
	if q.Len() > 0 {
		q.Push(slug)
	}
	return nil
}

// String returns a textual representation like "/foo:42/bar", or "/" if the queue is empty.
func (q *Queue) String() string {
	var result = &strings.Builder{}
	for _, segment := range *q {
		result.WriteString("/")
		result.WriteString(segment.Key)
		if segment.Version != 0 {
			result.WriteString(":")
			result.WriteString(strconv.Itoa(segment.Version))
		}
	}
	if result.Len() == 0 {
		return "/"
	}
	return result.String()
}
