package classes

import (
	//	"bytes"
	//	"golang.org/x/net/html"
	//	"golang.org/x/net/html/atom"
	//	"math"
	//	"strconv"
	//	"strings"
	"html/template"

	"github.com/wansing/perspective/core"
	//	"github.com/wansing/perspective/util"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Blog{
				page:     1,
				perPage:  3,
				readmore: "Read more",
			}
		},
		Name: "Blog",
		Code: "blog",
		Info: `
		<p>You can set the parameters: <tt>per-page</tt> and <tt>readmore</tt>.</p>
		<p>You can embed the parameters <tt>page</tt>, <tt>pages</tt>, <tt>page-url</tt> and <tt>pagelinks</tt>.</p>
		<p>
			Example:
			<code>
				[Back to page {{ .GetLocal "page" }} of {{ .GetLocal "pages" }}]({{ .GetLocal "page-url" }})
				<br>
				{{ .GetLocal "pagelinks" }}
			</code>
		</p>`,
		SelectOrder:          core.ChronologicallyDesc,
		FeaturedChildClasses: []string{"markdown"},
	})
}

type Blog struct {
	Markdown
	page     int // starting with 1
	pages    int
	perPage  int    // parameter:per-page
	readmore string // parameter:readmore
	Children []template.HTML
}

func (t *Blog) OnPrepare(r *core.Route) error {

	var offset = (t.page-1)*t.perPage

	children, err := t.Node.GetReleasedChildrenNodes(core.ChronologicallyDesc, t.perPage, offset)
	if err != nil {
		return err
	}

	for _, c := range children {

		// maybe move into GetReleasedChildrenNodes?
		if err := c.RequirePermission(core.Read, r.User); err != nil {
			continue
		}

		c.Instance = TruncateMore{c.Instance}

		childRoute, err := core.NewRoute(r.Request, core.NewQueue(""))
		if err != nil {
			return err
		}

		if err = childRoute.Execute(c); err != nil {
			return err
		}

		t.Children = append(t.Children, template.HTML(childRoute.Get("body")))
	}

	t.Node.SetContent(
		`{{ range .T.Children }}
			<div>
				{{ . }}
			</div>
		{{ end }}`,
	)
	return nil
}
