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

// Raw parses the user-defined content into templates. Variables can be set using {{define}}.
//
// Raw does not pass the Query to these user-defined templates.
// It wraps some functions instead, making them available in templates.
// Templates might still try to call Do and ParseExecute, but these functions require a Query as an argument.
type Raw struct {
	*core.Queue             // exposed to the user content
	query       *core.Query // not exported, unavailable in user-defined templates
}

func (t *Raw) AddSlugs() []string {
	return nil
}

func (t *Raw) Do(r *core.Query) error {
	return t.ParseAndExecute(r, t)
}

func (t *Raw) ParseAndExecute(r *core.Query, data interface{}) error {

	t.Queue = r.Queue
	t.query = r

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
		if err := lt.Execute(buf, data); err != nil { // recursion is done here
			return fmt.Errorf("%s: %v", r.Node.Location(), err)
		}
		r.Set(lt.Name(), buf.String())
	}

	return r.Recurse() // Recurse is idempotent here. This call is in case the user content forgot it. This might mess up the output, but is still better than not recursing at all.
}

// wrappers for Query functions

func (t *Raw) Get(varName string) template.HTML {
	return t.query.Get(varName)
}

func (t *Raw) Include(args ...string) (template.HTML, error) {
	return t.query.Include(args...)
}

func (t *Raw) Recurse() error {
	return t.query.Recurse()
}

func (t *Raw) Set(name, value string) {
	t.query.Set(name, value)
}

func (t *Raw) GetGlobal(varName string) template.HTML {
	return t.query.GetGlobal(varName)
}

func (t *Raw) HasGlobal(varName string) bool {
	return t.query.HasGlobal(varName)
}

func (t *Raw) SetGlobal(varName string, value string) interface{} {
	return t.query.SetGlobal(varName, value)
}

func (t *Raw) Tag(tags ...string) interface{} {
	return t.query.Version.Tag(tags...)
}

func (t *Raw) Ts(dates ...string) interface{} {
	return t.query.Version.Ts(dates...)
}
