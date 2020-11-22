package backend

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/wansing/perspective/core"
)

func Breadcrumbs(node *core.Node, linkLast bool) template.HTML {

	var nodes = []*core.Node{}
	for n := node; n != nil; n = n.Parent {
		nodes = append(nodes, n)
	}

	// reverse
	for i := len(nodes)/2 - 1; i >= 0; i-- {
		opp := len(nodes) - 1 - i
		nodes[i], nodes[opp] = nodes[opp], nodes[i]
	}

	var buf = &bytes.Buffer{}
	buf.WriteString(`<nav aria-label="breadcrumb" style="margin-top: 1rem;"><ol class="breadcrumb">`)

	for _, n := range nodes {
		var isLast = n == node
		buf.WriteString(`<li class="breadcrumb-item`)
		if isLast {
			buf.WriteString(` active`)
		}
		buf.WriteString(`">`)
		if !isLast || linkLast {
			buf.WriteString(`<a href="choose/1` + n.Location() + `">`)
		}
		buf.WriteString(n.Slug())
		if !isLast || linkLast {
			buf.WriteString(`</a>`)
		}
		buf.WriteString(`</li>`)
	}

	buf.WriteString(`</ol></nav>`)

	return template.HTML(buf.String())
}

func FormatTs(ts int64) string {
	// ignores the user's timezone
	return time.Unix(ts, 0).Format("_2.1.2006 15:04:05")
}

// SelectChildClass writes options or optgroups.
func SelectChildClass(reg core.ClassRegistry, featuredChildClasses []string, selectedCode string) template.HTML {

	var b = &strings.Builder{}
	var optgroups = false
	var selected = false

	if len(featuredChildClasses) > 0 {
		optgroups = true
		b.WriteString(`<optgroup label="Featured">`)
		for _, code := range featuredChildClasses {
			if class, ok := reg.Get(code); ok {
				b.WriteString(`<option `)
				if class.Code == selectedCode {
					b.WriteString(`selected `)
					selected = true
				}
				fmt.Fprintf(b, `value="%s">%s: %s</option>`, class.Code, class.Code, class.Name)
			}
		}
		b.WriteString(`</optgroup>`)
	}

	if optgroups {
		b.WriteString(`<optgroup label="All">`)
	}

	for _, code := range reg.All() {
		if class, ok := reg.Get(code); ok {
			b.WriteString(`<option `)
			if !selected && class.Code == selectedCode {
				b.WriteString(`selected `)
			}
			fmt.Fprintf(b, `value="%s">%s: %s</option>`, class.Code, class.Code, class.Name)
		}
	}

	if optgroups {
		b.WriteString(`</optgroup>`)
	}

	return template.HTML(b.String())
}
