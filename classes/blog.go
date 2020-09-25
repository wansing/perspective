//+build ignore

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
				{{with .Node.Tags}}
					&middot; Tags:
					{{range $i, $tag := .}}
						{{- if $i}},{{end}}
						{{$tag -}}
					{{end}}
				{{end}}
				{{with .Node.Timestamps}}
					&middot; findet statt am:
					{{range .}}
						{{$.Request.FormatDateTime .}}
					{{end}}
				{{end}}
			</div>
		{{end}}

		{{if .Node.Next}}
			{{template "metadata" .T.Next}}
			{{.Get "body"}}
			<div class="blog-blogentry-back">
				<a onclick="javascript:window.history.back(); return false;" href="{{.Link}}">Zurück</a>
			</div>
		{{else}}
			{{range .T.Children}}
				<div class="blog-blogentry">
					{{template "metadata" .}}
					<div class="blog-blogentry-teaser">
						{{.Body}}
						{{if .Cut}}
							<p class="blog-blogentry-more">
								<a href="{{.Link}}">{{$.T.ReadMore}}</a>
							</p>
						{{end}}
					</div>
				</div>
			{{end}}
			<div class="blog-pagelinks">
				{{range .T.PageLinks}}
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
	core.Base
	Children  []*blogChild
	Next      blogNode
	page      int // starting with 1
	PageLinks []template.HTML
	pages     int
	perPage   int
	ReadMore  string
	tmpl      *template.Template
}

// Node plus Request, so we can localize or internationalize things
type blogNode struct {
	*core.Node
	*core.Request
}

type blogChild struct {
	blogNode
	Body template.HTML
	Cut  bool
}

func (t *Blog) AdditionalSlugs() []string {
	if t.page > 1 {
		return []string{"page", strconv.Itoa(t.page)}
	}
	return nil
}

func (t *Blog) Do(r *core.Route) error {

	// take segment page/123 from queue before calling Recurse

	if r.Queue.PopIf("page") {
		pageStr, _ := r.Queue.Pop()
		t.page, _ = strconv.Atoi(pageStr.Key)
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
		t.pages = 1
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

	if r.Node.Next != nil {
		t.Next = blogNode{
			Node:    r.Node.Next,
			Request: r.Request,
		}
	} else {

		// Populate Children. This is not a func on Blog because we need stuff from Route for the localization.

		children, err := r.Node.GetReleasedChildren(r.User, core.ChronologicallyDesc, t.perPage, (t.page-1)*t.perPage)
		if err != nil {
			return err
		}

		for _, child := range children {

			// render body

			childRoute := &core.Route{
				Node:    child,
				Request: r.Request,
				Queue:   core.NewQueue(""),
			}

			if err := child.Do(childRoute); err != nil {
				return err
			}

			body, cut := util.CutMore(string(childRoute.Get("body")))

			bodyBytes, err := ioutil.ReadAll(
				util.AnchorHeading(
					strings.NewReader(body),
					fmt.Sprintf(`<a href="%s" class="%s" id="%s">`, child.Link(), "blog-blogentry-headline", child.Slug()),
				),
			)
			if err != nil {
				return err
			}

			t.Children = append(t.Children, &blogChild{
				blogNode: blogNode{
					Node:    child,
					Request: r.Request,
				},
				Body: template.HTML(bodyBytes),
				Cut:  cut,
			})
		}
	}

	t.PageLinks = util.PageLinks(
		t.page,
		t.pages,
		func(page int, name string) string {
			return `<a href="` + r.Node.Link() + `/page/` + strconv.Itoa(page) + `">` + name + `</a>`
		},
		func(page int, name string) string {
			return `<span>` + strconv.Itoa(page) + `</span>`
		},
	)

	buf := &bytes.Buffer{}
	if err := t.tmpl.Execute(buf, r); err != nil {
		return err
	}
	r.Set("body", buf.String())

	return nil
}
