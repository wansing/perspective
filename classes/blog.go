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

	tmpl := template.Must(template.New("").Parse(`

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
			{{.Route.Get "body"}}
			<div class="blog-blogentry-back">
				<a onclick="javascript:window.history.back(); return false;" href="{{.Route.Node.Link}}">Zurück</a>
			</div>
		{{else}}
			{{range .Children}}
				<div class="blog-blogentry">
					{{template "metadata" .}}
					<div class="blog-blogentry-teaser">
						{{.Body}}
						{{if .Cut}}
							<p class="blog-blogentry-more">
								<a href="{{.Route.Node.Link}}">{{$.ReadMore}}</a>
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

	Register(&core.Class{
		Create: func() core.Instance {
			return &Blog{
				page: 1,
				tmpl: tmpl,
			}
		},
		Name:                 "Blog",
		Code:                 "blog",
		Info:                 ``,
		SelectOrder:          core.ChronologicallyDesc,
		FeaturedChildClasses: []string{"markdown"},
	})
}

type Blog struct {
	page      int // starting with 1
	pages     int
	perPage   int
	ReadMore  string
	Route     *core.Route
	tmpl      *template.Template
}

func (t *Blog) PageLinks() []template.HTML {
	return util.PageLinks(
		t.page,
		t.pages,
		func(page int, name string) string {
			return `<a href="` + t.Route.Node.Link() + `/page/` + strconv.Itoa(page) + `">` + name + `</a>`
		},
		func(page int, name string) string {
			return `<span>` + strconv.Itoa(page) + `</span>`
		},
	)
}

// Node plus Request, so we can localize or internationalize things
type blogNode struct {
	core.NodeVersion
	*core.Request
}

func (t *Blog) Next() *blogNode {
	if len(t.Route.Next) > 0 {
		return &blogNode{
			NodeVersion: t.Route.Next[0],
			Request:     t.Route.Request,
		}
	}
	return nil
}

type blogChild struct {
	blogNode
	Body template.HTML
	Cut  bool
}

func (t *Blog) Children() ([]*blogChild, error) {

	children, err := t.Route.Node.GetReleasedChildren(t.Route.User, core.ChronologicallyDesc, t.perPage, (t.page-1)*t.perPage)
	if err != nil {
		return nil, err
	}

	var result = make([]*blogChild, 0, len(children))

	for _, child := range children {

		// render body

		childRoute := &core.Route{
			Node:    child.Node,
			Version: child.Version,
			Request: t.Route.Request,
			Queue:   core.NewQueue(""),
		}

		if err := child.Do(childRoute); err != nil {
			return nil, err
		}

		body, cut := util.CutMore(string(childRoute.Get("body")))

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
				Request:     t.Route.Request,
			},
			Body: template.HTML(bodyBytes),
			Cut:  cut,
		})
	}

	return result, nil
}

func (t *Blog) AddSlugs() []string {
	if t.page > 1 {
		return []string{"page", strconv.Itoa(t.page)}
	}
	return nil
}

func (t *Blog) Do(r *core.Route) error {

	t.Route = r

	// take segment page/123 from queue before calling Recurse

	if r.Queue.PopIf("page") {
		pageStr, _ := r.Queue.Pop()
		t.page, _ = strconv.Atoi(pageStr)
		if t.page == 1 {
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
	var data = cfg.Section("").KeysHash()

	t.perPage, _ = strconv.Atoi(data["per-page"])
	if t.perPage <= 0 {
		t.perPage = 10
	}

	if childrenCount, err := r.Node.CountReleasedChildren(); err == nil {
		t.pages = int(math.Ceil(float64(childrenCount) / float64(t.perPage)))
	} else {
		return err
	}

	if t.page < 1 {
		t.page = 1
	}

	if t.page > t.pages {
		t.page = t.pages
	}

	t.ReadMore = data["readmore"]
	if t.ReadMore == "" {
		t.ReadMore = "Read more"
	}

	buf := &bytes.Buffer{}
	if err := t.tmpl.Execute(buf, t); err != nil {
		return err
	}
	r.Set("body", buf.String())

	return nil
}
