package classes

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/util"
)

var rawTemplateFuncs = template.FuncMap{
	"more": func() template.HTML {
		return template.HTML(util.CutMoreStr)
	},
}

func init() {
	Register(func() core.Class {
		return Raw{}
	})
}

// Raw parses the user-defined content into templates. Variables can be set using {{define}}.
type Raw struct {
	// if we stored templateFuncs here, it would be harder for HTML to embed Raw
}

func (Raw) Code() string {
	return "raw"
}

func (Raw) Name() string {
	return "Raw HTML document"
}

func (Raw) Info() string {
	return ""
}

func (Raw) FeaturedChildClasses() []string {
	return nil
}

func (Raw) SelectOrder() core.Order {
	return core.AlphabeticallyAsc
}

func (raw Raw) Run(r *core.Query) error {
	var data = &rawData{
		Queue: r.Queue,
		query: r,
	}
	return raw.ParseAndExecute(r, data)
}

func (Raw) ParseAndExecute(r *core.Query, data interface{}) error {

	// parse and execute the user content into templates

	parsed, err := template.New("body").Funcs(rawTemplateFuncs).Parse(r.Content())
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

type rawData struct {
	*core.Queue             // exposed to the user content
	query       *core.Query // not exported, unavailable in user-defined templates
}

func (data *rawData) Get(varName string) template.HTML {
	return data.query.Get(varName)
}

func (data *rawData) Include(args ...string) (template.HTML, error) {
	return data.query.Include(args...)
}

func (data *rawData) Recurse() error {
	return data.query.Recurse()
}

func (data *rawData) Set(name, value string) {
	data.query.Set(name, value)
}

func (data *rawData) GetGlobal(varName string) template.HTML {
	return data.query.GetGlobal(varName)
}

func (data *rawData) HasGlobal(varName string) bool {
	return data.query.HasGlobal(varName)
}

func (data *rawData) SetGlobal(varName string, value string) interface{} {
	return data.query.SetGlobal(varName, value)
}

func (data *rawData) Tag(tags ...string) interface{} {
	return data.query.Version.Tag(tags...)
}

func (data *rawData) Ts(dates ...string) interface{} {
	return data.query.Version.Ts(dates...)
}
