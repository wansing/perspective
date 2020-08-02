package core

import (
	"fmt"
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

func NewQueue(path string) (*Queue, error) {

	// dots are not allowed
	if strings.Contains(path, ".") {
		return nil, fmt.Errorf("path contains a dot: %s", path)
	}

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

	return queue, nil
}

func (q *Queue) Clear() {
	(*q) = nil
}

func (q *Queue) HasPrefix(key string) bool {
	return q.Len() > 0 && (*q)[0].Key == key
}

func (q *Queue) Len() int {
	return len(*q)
}

func (q *Queue) Pop() (segment QueueSegment, ok bool) {
	if q.Len() > 0 && (*q)[0].Key != "" {
		segment, (*q) = (*q)[0], (*q)[1:]
		ok = true
	}
	return
}

// PopInt removes and returns the key of the first queue item if and only if it is an integer.
// Use the boolean return value to distinguish between zero and non-integer.
func (q *Queue) PopInt() (val int, ok bool) {
	if q.Len() == 0 {
		return
	}
	val, err := strconv.Atoi((*q)[0].Key)
	if err != nil {
		return
	}
	(*q) = (*q)[1:]
	ok = true
	return
}

// PopIf removes and returns the first queue item if and only if it equals the given key.
func (q *Queue) PopIf(key string) (segment QueueSegment, ok bool) {
	if q.Len() >= 1 && (*q)[0].Key == key {
		segment, (*q) = (*q)[0], (*q)[1:]
		ok = true
	}
	return
}

// PopIfVal removes and returns the second queue item if and only if the first item equals the given key.
func (q *Queue) PopIfVal(key string) (segment QueueSegment, ok bool) {
	if q.Len() >= 2 && (*q)[0].Key == key {
		segment, *q = (*q)[1], (*q)[2:]
		ok = true
	}
	return
}

func (q *Queue) Push(slug string) interface{} {
	*q = append([]QueueSegment{{Key: slug, Version: DefaultVersion, pushed: true}}, *q...)
	return nil
}

func (q *Queue) PushIfEmpty(slug string) interface{} {
	if len(*q) == 0 {
		q.Push(slug)
	}
	return nil
}

func (q *Queue) PushIfNotEmpty(slug string) interface{} {
	if len(*q) > 0 {
		q.Push(slug)
	}
	return nil
}

// String returns a textual representation like "/foo:42/bar", or "/" if the queue is empty.
func (q *Queue) String() string {
	result := ""
	for _, segment := range *q {
		result = result + "/" + segment.Key
		if segment.Version != 0 {
			result = result + ":" + strconv.Itoa(segment.Version)
		}
	}
	if result == "" {
		result = "/"
	}
	return result
}
