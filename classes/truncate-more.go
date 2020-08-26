package classes

import (
	"strings"

	"github.com/wansing/perspective/core"
)

// TruncateMore wraps an Instance:
//  e.Instance = TruncateMore{e.Instance}
type TruncateMore struct {
	core.Instance
}

// OnExecute modifies results["body"], discarding everything including and after "<!-- more -->".
//
// Golang's template parser removes HTML comments. core.ContentFuncs defines the "{{more}}" function which creates the required comment.
func (t TruncateMore) OnExecute(r *core.Route) error {

	var body = string(r.Get("body"))
	if index := strings.Index(body, "<!-- more -->"); index >= 0 {
		body = body[:index]
	}
	r.Set("body", body)

	return t.Instance.OnExecute(r)
}
