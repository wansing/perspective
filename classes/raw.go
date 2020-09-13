package classes

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/util"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Raw{}
		},
		Name: "Raw HTML document",
		Code: "raw",
	})
}

var RawTemplateFuncs = template.FuncMap{
	"more": func() template.HTML {
		return template.HTML(util.CutMoreStr)
	},
}

// Raw parses the content as templates. Variables can be set using {{define}}.
type Raw struct {
	core.Base
}

func (t *Raw) Do(r *core.Route) error {

	// parse and execute the user content into templates

	parsed, err := template.New("body").Funcs(RawTemplateFuncs).Parse(r.Content())
	if err != nil {
		return err
	}

	var globalTemplates []*template.Template
	var localTemplates []*template.Template

	for _, pt := range parsed.Templates() {
		if util.IsFirstUpper(pt.Name()) {
			globalTemplates = append(globalTemplates, pt)
		} else {
			localTemplates = append(localTemplates, pt)
		}
	}

	// parsed templates are still associated to each other, so it's enough to add the old global templates to one of the new localTemplates

	for _, oldGlobal := range r.Templates {
		_, err = localTemplates[0].AddParseTree(oldGlobal.Name(), oldGlobal.Tree)
		if err != nil {
			return err
		}
	}

	// now add new globalTemplates to r.Templates

	for _, newGlobal := range globalTemplates {
		r.Templates[newGlobal.Name()], err = newGlobal.Clone()
		if err != nil {
			return err
		}
	}

	// execute local templates

	for _, lt := range localTemplates {
		buf := &bytes.Buffer{}
		if err := lt.Execute(buf, r); err != nil { // recursion is done here
			return fmt.Errorf("%s: %v", r.Node.HrefPath(), err)
		}
		r.Set(lt.Name(), buf.String())
	}

	r.Recurse() // Recurse is idempotent here. This call is in case the user content forgot it. This might mess up the output, but is still better than not recursing at all.

	return nil
}
