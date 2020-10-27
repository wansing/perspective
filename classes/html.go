package classes

import (
	"fmt"
	pkghtml "html"
	"io"
	"net/url"
	pathpkg "path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/upload"
	"github.com/wansing/perspective/util"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &HTML{}
		},
		Name: "HTML document",
		Code: "html",
	})
}

var delimiters = regexp.MustCompile("{{.+?}}") // +? prefer fewer

type HTML struct {
	Raw
}

// Do rewrites some HTML code and then calls Raw.Do().
//
// The order is crucial.
// If templates were processed first, then embedded content would be rewritten as well.
// Now instead, HTML rewriting must take care not to modify template instructions.
func (t *HTML) Do(r *core.Route) error {

	rewritten, err := rewriteHTML(r.Request.Path, r.Node, strings.NewReader(r.Content()))
	if err != nil {
		return err
	}

	r.SetContent(rewritten)

	return t.Raw.Do(r)
}

func rewriteHTML(reqPath string, node *core.Node, input io.Reader) (string, error) {

	domtree, err := util.CreateDomTree(input)
	if err != nil {
		return "", err
	}

	// rewrite a-href and img-src

	err = util.ForEachDomNode(domtree, func(domNode *html.Node) (bool, error) {

		// return (but keep iterating) if not an a/img ElementNode

		if domNode.Type != html.ElementNode {
			return true, nil
		}

		if domNode.DataAtom != atom.A && domNode.DataAtom != atom.Img {
			return true, nil
		}

		// Process upload filenames via a-href or img-src, like `foo.jpg` or `100/foo.jpg?w=400&h=200`.
		// We often call return here, but that's okay because we're at the end of the function.

		// search for href/src attribute

		var attrIdx int = -1

		if domNode.DataAtom == atom.A {
			for i, attr := range domNode.Attr {
				if domNode.DataAtom == atom.A && strings.ToLower(attr.Key) == "href" {
					attrIdx = i
					break
				}
			}
		}

		if domNode.DataAtom == atom.Img {
			for i, attr := range domNode.Attr {
				if domNode.DataAtom == atom.Img && strings.ToLower(attr.Key) == "src" {
					attrIdx = i
					break
				}
			}
		}

		// skip if href/src not found

		if attrIdx < 0 {
			return true, nil
		}

		// skip domNode if value is unparseable

		u, err := url.Parse(domNode.Attr[attrIdx].Val)
		if err != nil {
			return true, nil
		}

		// skip domNode if the url contains explicit Opaque, Scheme, User or Host

		if u.Opaque != "" || u.Scheme != "" || u.User != nil || u.Host != "" {
			return true, nil
		}

		// check if u is an upload
		//
		// Ambiguities in hrefs: Does <a href="2019/foo.jpg"> link to an uploaded file or an node? We consider it an upload if the filename contains a dot.

		path, filename, resize, w, h, _, _ := upload.ParseUrl(u)

		if strings.Contains(filename, ".") {

			var nodeID int

			if path == "" {
				nodeID = node.ID()
			} else {
				nodeID, err = strconv.Atoi(path)
				if err != nil {
					return true, nil
				}
			}

			if resize {

				// CSS attribute "style"

				styleAttr := "width: auto; height: auto;"

				if w != 0 {
					styleAttr += " max-width: " + strconv.Itoa(w) + "px;"
				}

				if h != 0 {
					styleAttr += " max-height: " + strconv.Itoa(h) + "px;"
				}

				domNode.Attr = append(domNode.Attr, html.Attribute{Key: "style", Val: styleAttr})

				// restore w and h and append timestamp and signature to filename

				ts := time.Now().Unix()

				filename += fmt.Sprintf("?w=%d&h=%d&ts=%d&sig=%s", w, h, ts, node.HMAC(nodeID, filename, w, h, ts))
			}

			// always prepend upload folder

			domNode.Attr[attrIdx].Val = fmt.Sprintf("/upload/%d/%s", nodeID, filename)

			return true, nil

		} else {

			// It's not an upload. atom.Img wouldn't make sense.

			if domNode.DataAtom == atom.A {

				// post-process u.Path
				//
				// hrefs shall be relative to the containing impression (not to the current leaf), so we must make them absolute
				//
				// See path.Clean for path guidelines. Link() must comply with them.

				u.Path = strings.TrimSpace(u.Path)

				switch u.Path {
				case "":
					if u.Fragment == "" { // don't touch href="#foo"
						if domNode.FirstChild != nil {
							u.Path = pathpkg.Join(node.Link(), core.NormalizeSlug(domNode.FirstChild.Data)) // href from content
						}
					}
				case ".":
					u.Path = node.Link() // href to impression which contains it
				default:
					if !pathpkg.IsAbs(u.Path) { // don't touch absolute href
						u.Path = pathpkg.Join(node.Link(), u.Path) // make path absolute
					}
				}

				domNode.Attr[attrIdx].Val = u.String()

				// Determine if the link is active
				//
				// Example request: "/foo/bar"
				// Active hrefs: "/foo", "/foo/bar"
				// Inactive hrefs: "/" (because it would be always active), "/foo/bar/baz", "/foo/other"
				// Assumption:
				// - href is active iff it is a prefix of request
				// - except route "/", which is active for href "/" only

				active := strings.HasPrefix(reqPath, u.Path)

				// stricter rule for root node

				if active && u.Path == "/" {
					active = reqPath == "/"
				}

				if active {

					// add class "active"

					classFound := false

					for i := range domNode.Attr {

						if strings.ToLower(domNode.Attr[i].Key) != "class" {
							continue
						}

						domNode.Attr[i].Val = domNode.Attr[i].Val + " active"
						classFound = true
						break
					}

					// attribute "class" not found, add 'class="active"''

					if !classFound {
						domNode.Attr = append(domNode.Attr, html.Attribute{Key: "class", Val: "active"})
					}
				}
			}
		}

		return true, err
	})
	if err != nil {
		return "", err
	}

	// restore quotation marks etc within template instructions {{ }}
	// regex is not multi-line because text/template writes: "Except for raw strings, actions may not span newlines, although comments can."

	var result = delimiters.ReplaceAllStringFunc(
		util.RenderDomTreeToString(domtree),
		pkghtml.UnescapeString,
	)

	return result, nil
}
