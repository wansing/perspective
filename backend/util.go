package backend

import (
	"bytes"
	"html/template"
	"io"
	"time"

	"github.com/wansing/perspective/core"
)

func Breadcrumbs(e *core.Node, linkLast bool) template.HTML {

	var buf = &bytes.Buffer{}

	buf.WriteString(`<nav aria-label="breadcrumb" style="margin-top: 1rem;"><ol class="breadcrumb">`)

	for i := e.Root(); i != e.Next; i = i.Next {
		var isLast = (i == e)
		buf.WriteString(`<li class="breadcrumb-item`)
		if isLast {
			buf.WriteString(` active`)
		}
		buf.WriteString(`">`)
		if !isLast || linkLast {
			buf.WriteString(`<a href="choose/1` + i.HrefPath() + `">`)
		}
		buf.WriteString(i.Slug())
		if !isLast || linkLast {
			buf.WriteString(`</a>`)
		}
		buf.WriteString(`</li>`)
	}

	buf.WriteString(`</ol></nav>`)

	return template.HTML(buf.String())
}

func optionClass(reg core.ClassRegistry, w io.StringWriter, code string, selectedCode string) {
	if class, ok := reg.Get(code); ok {
		w.WriteString(`<option `)
		if class.Code == selectedCode {
			w.WriteString(`selected `)
		}
		w.WriteString(`value="` + class.Code + `">` + class.Code + ": " + class.Name + `</option>`)
	}
}

func FormatTs(ts int64) string {
	// ignores the user timezone
	return time.Unix(ts, 0).Format("_2.1.2006 15:04:05")
}

// SelectChildClass writes one or two optgroup tags.
func SelectChildClass(reg core.ClassRegistry, e *core.Node, selectedCode string) template.HTML {

	w := &bytes.Buffer{}

	if e != nil && len(e.Class.FeaturedChildClasses) > 0 {
		w.WriteString(`<optgroup label="Featured">`)
		for _, code := range e.Class.FeaturedChildClasses {
			optionClass(reg, w, code, selectedCode)
		}
		w.WriteString(`</optgroup>`)
	}

	w.WriteString(`<optgroup label="All">`)
	for _, code := range reg.All() {
		optionClass(reg, w, code, selectedCode)
	}
	w.WriteString(`</optgroup>`)

	return template.HTML(w.String())
}
