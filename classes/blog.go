package classes

// TODO rss/atom feed with pagination https://stackoverflow.com/questions/1301392/pagination-in-feeds-like-atom-and-rss

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"math"
	"strconv"
	"strings"

	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/util"
	"gopkg.in/ini.v1"
)

func init() {

	var tmpl = template.Must(template.New("").Parse(`

		{{define "metadata"}}
			<div class="blog-blogentry-date">
				geschrieben am {{.Request.FormatDateTime .Node.TsCreated}}
				{{with .NodeVersion.Tags}}
					&middot; Tags:
					{{range $i, $tag := .}}
						{{- if $i}},{{end}}
						{{$tag -}}
					{{end}}
				{{end}}
				{{with .NodeVersion.Timestamps}}
					&middot; findet statt am:
					{{range .}}
						{{$.Request.FormatDateTime .}}
					{{end}}
				{{end}}
			</div>
		{{end}}

		{{if .Next}}
			{{template "metadata" .Next}}
			{{.Query.Get "body"}}
			<div class="blog-blogentry-back">
				<a onclick="javascript:window.history.back(); return false;" href="{{.Query.Node.Link}}">Zurück</a>
			</div>
		{{else}}
			{{range .Children}}
				<div class="blog-blogentry">
					{{template "metadata" .}}
					<div class="blog-blogentry-teaser">
						{{.Body}}
						{{if .Cut}}
							<p class="blog-blogentry-more">
								<a href="{{.Query.Node.Link}}">{{$.ReadMore}}</a>
							</p>
						{{end}}
					</div>
				</div>
			{{end}}
			<div class="blog-pagelinks">
				{{range .PageLinks}}
					{{.}}
				{{end}}
			</div>
		{{end}}`))

	Register(func() core.Class {
		return &Blog{
			tmpl: tmpl,
		}
	})
}

type Blog struct {
	tmpl *template.Template
}

func (*Blog) Code() string {
	return "blog"
}

func (*Blog) Name() string {
	return "Blog"
}

func (*Blog) Info() string {
	return ""
}

func (*Blog) FeaturedChildClasses() []string {
	return []string{"markdown"}
}

func (*Blog) SelectOrder() core.Order {
	return core.ChronologicallyDesc
}

type blogData struct {
	page     int // starting with 1
	pages    int
	perPage  int
	ReadMore string
	Query    *core.Query
}

// Node plus Request, so we can localize or internationalize things
type blogNode struct {
	core.NodeVersion
	*core.Request
}

type blogChild struct {
	blogNode
	Body template.HTML
	Cut  bool
}

func (data *blogData) PageLinks() []template.HTML {
	return util.PageLinks(
		data.page,
		data.pages,
		func(page int, name string) string {
			return `<a href="` + data.Query.Node.Link() + `/page/` + strconv.Itoa(page) + `">` + name + `</a>`
		},
		func(page int, name string) string {
			return `<span>` + strconv.Itoa(page) + `</span>`
		},
	)
}

func (data *blogData) Next() *blogNode {
	if len(data.Query.Next) > 0 {
		return &blogNode{
			NodeVersion: data.Query.Next[0],
			Request:     data.Query.Request,
		}
	}
	return nil
}

func (data *blogData) Children() ([]*blogChild, error) {

	children, err := data.Query.Node.GetReleasedChildren(data.Query.User, core.ChronologicallyDesc, data.perPage, (data.page-1)*data.perPage)
	if err != nil {
		return nil, err
	}

	var result = make([]*blogChild, 0, len(children))

	for _, child := range children {

		// render body

		childQuery := &core.Query{
			Node:    child.Node,
			Version: child.Version,
			Request: data.Query.Request,
			Queue:   core.NewQueue(""),
		}

		if err := childQuery.Run(); err != nil {
			return nil, err
		}

		body, cut := util.CutMore(string(childQuery.Get("body")))

		bodyBytes, err := ioutil.ReadAll(
			util.AnchorHeading(
				strings.NewReader(body),
				fmt.Sprintf(`<a href="%s" class="%s" id="%s">`, child.Link(), "blog-blogentry-headline", child.Slug()),
			),
		)
		if err != nil {
			return nil, err
		}

		result = append(result, &blogChild{
			blogNode: blogNode{
				NodeVersion: child,
				Request:     data.Query.Request,
			},
			Body: template.HTML(bodyBytes),
			Cut:  cut,
		})
	}

	return result, nil
}

func (t *Blog) Run(r *core.Query) error {

	var data = &blogData{
		Query: r,
	}

	// take segment page/123 from queue before calling Recurse

	if r.Queue.PopIf("page") {
		pageStr, _ := r.Queue.Pop()
		data.page, _ = strconv.Atoi(pageStr)
		if data.page == 1 {
			r.Set(
				"head",
				fmt.Sprintf(`<link rel="canonical" href="%s" />%s`, r.Node.Link(), r.Get("head")),
			)
		}
	}

	if err := r.Recurse(); err != nil {
		return err
	}

	// parse content as ini

	var cfg, err = ini.Load([]byte(r.Content()))
	if err != nil {
		return err
	}
	var config = cfg.Section("").KeysHash()

	data.perPage, _ = strconv.Atoi(config["per-page"])
	if data.perPage <= 0 {
		data.perPage = 10
	}

	if childrenCount, err := r.Node.CountReleasedChildren(); err == nil {
		data.pages = int(math.Ceil(float64(childrenCount) / float64(data.perPage)))
	} else {
		return err
	}

	if data.page < 1 {
		data.page = 1
	}

	if data.page > data.pages {
		data.page = data.pages
	}

	data.ReadMore = config["readmore"]
	if data.ReadMore == "" {
		data.ReadMore = "Read more"
	}

	if data.page > 1 {
		r.Node.AddSlugs = []string{"page", strconv.Itoa(data.page)}
	}

	buf := &bytes.Buffer{}
	if err := t.tmpl.Execute(buf, data); err != nil {
		return err
	}
	r.Set("body", buf.String())

	return nil
}
