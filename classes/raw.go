package classes

import (
	"bytes"
	"fmt"
	"html/template"
	"sort"

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

	var global []*template.Template
	var local []*template.Template

	for _, t := range parsed.Templates() { // order is random
		if util.IsFirstUpper(t.Name()) {
			global = append(global, t)
		} else {
			local = append(local, t)
		}
	}

	// parsed templates are associated to each other, so it's enough to add each long-known global template to any local template

	for _, t := range r.Templates {
		_, err = local[0].AddParseTree(t.Name(), t.Tree)
		if err != nil {
			return err
		}
	}

	// now add new global templates to r.Templates, Clone() dissociates them

	for _, t := range global {
		r.Templates[t.Name()], err = t.Clone()
		if err != nil {
			return err
		}
	}

	// sort local templates, "body" must be executed last because it usually contains {{.Recurse}}

	sort.Slice(
		local,
		func(i, j int) bool { // less
			if local[i].Name() == "body" {
				return false // "body" is never less
			}
			if local[j].Name() == "body" {
				return true // everything is less compared to "body"
			}
			return local[i].Name() < local[j].Name()
		},
	)

	// execute local templates

	for _, t := range local {
		buf := &bytes.Buffer{}
		if err := t.Execute(buf, data); err != nil { // recursion is done here
			return fmt.Errorf("%s: %v", r.Node.Location(), err)
		}
		r.Set(t.Name(), buf.String())
	}

	return r.Recurse() // Recurse is idempotent here. This call is in case the user content forgot it. This might mess up the output, but is still better than not recursing at all.
}

type rawData struct {
	*core.Queue             // exposed to the user content
	query       *core.Query // not exported, unavailable in user-defined templates
}

func (data *rawData) Get(name string) template.HTML {
	return template.HTML(data.query.Get(name))
}

func (data *rawData) GetGlobal(name string) template.HTML {
	return template.HTML(data.query.GetGlobal(name))
}

func (data *rawData) Include(args ...string) (template.HTML, error) {
	return data.query.Include(args...)
}

func (data *rawData) Recurse() error {
	return data.query.Recurse()
}

// "Such a method must have one return value (of any type) or two return values, the second of which is an error."
func (data *rawData) Set(name, value string) interface{} {
	data.query.Set(name, value)
	return nil
}

// "Such a method must have one return value (of any type) or two return values, the second of which is an error."
func (data *rawData) SetGlobal(name string, value string) interface{} {
	data.query.SetGlobal(name, value)
	return nil
}

func (data *rawData) Tag(tags ...string) interface{} {
	return data.query.Version.Tag(tags...)
}

func (data *rawData) Ts(dates ...string) interface{} {
	return data.query.Version.Ts(dates...)
}
