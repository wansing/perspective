package classes

import (
	"fmt"
	pkghtml "html"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wansing/perspective/core"
	"github.com/wansing/perspective/util"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Html{}
		},
		Name: "HTML document",
		Code: "html",
	})
}

var delimiters = regexp.MustCompile("{{.+?}}") // +? prefer fewer

type Html struct {
	core.Base
}

func (t *Html) OnPrepare(r *core.Route) error {

	var e = t.Node

	domtree, err := util.CreateDomTree(strings.NewReader(e.Content()))
	if err != nil {
		return err
	}

	// rewrite a-href and img-src
	//
	// See "func (e *Node) parseUploadUrl" for how the URL type (upload or node) is determined.

	// Ambiguities: Does <a href="2019/foo.jpg"> link to an uploaded file or an node?
	//
	// Answer:
	// - uploads and nodes share a namespace
	// - node slugs must not contain dots

	err = util.ForEachDomNode(domtree, func(node *html.Node) (bool, error) {

		// return (but keep iterating) if not an a/img ElementNode

		if node.Type != html.ElementNode {
			return true, nil
		}

		if node.DataAtom != atom.A && node.DataAtom != atom.Img {
			return true, nil
		}

		// Process upload filenames via a-href or img-src, like `foo.jpg` or `100/foo.jpg?w=400&h=200`.
		// We often call return here, but that's okay because we're at the end of the function.

		// search for href/src attribute

		var attrIdx int = -1

		if node.DataAtom == atom.A {
			for i, attr := range node.Attr {
				if node.DataAtom == atom.A && strings.ToLower(attr.Key) == "href" {
					attrIdx = i
					break
				}
			}
		}

		if node.DataAtom == atom.Img {
			for i, attr := range node.Attr {
				if node.DataAtom == atom.Img && strings.ToLower(attr.Key) == "src" {
					attrIdx = i
					break
				}
			}
		}

		// skip if href/src not found

		if attrIdx < 0 {
			return true, nil
		}

		// skip node if value is unparseable

		u, err := url.Parse(node.Attr[attrIdx].Val)
		if err != nil {
			return true, nil
		}

		// skip node if the url contains explicit Opaque, Scheme, User or Host

		if u.Opaque != "" || u.Scheme != "" || u.User != nil || u.Host != "" {
			return true, nil
		}

		// check if u is an upload

		isUpload, location, filename, resize, w, h, _, _, err := e.ParseUploadUrl(u)
		if err != nil {
			return true, err
		}

		if isUpload {

			if resize {

				// CSS attribute "style"

				styleAttr := "width: auto; height: auto;"

				if w != 0 {
					styleAttr += " max-width: " + strconv.Itoa(w) + "px;"
				}

				if h != 0 {
					styleAttr += " max-height: " + strconv.Itoa(h) + "px;"
				}

				node.Attr = append(node.Attr, html.Attribute{Key: "style", Val: styleAttr})

				// restore w and h and append timestamp and signature to filename

				ts := time.Now().Unix()

				filename += fmt.Sprintf("?w=%d&h=%d&ts=%d&sig=%s", w, h, ts, e.HMAC(location.NodeId(), filename, w, h, ts))
			}

			// always prepend upload folder

			node.Attr[attrIdx].Val = fmt.Sprintf("/upload/%d/%s", location.NodeId(), filename)

			return true, nil

		} else {

			// It's not an upload. atom.Img wouldn't make sense.

			if node.DataAtom == atom.A {

				// post-process u.Path
				//
				// hrefs shall be relative to the containing impression (not to the current leaf), so we must make them absolute
				//
				// See path.Clean for path guidelines. HrefView must comply with them.

				u.Path = strings.TrimSpace(u.Path)

				switch u.Path {
				case "":
					if u.Fragment == "" { // don't touch href="#foo"
						if node.FirstChild != nil {
							u.Path = path.Join(e.HrefView(), core.NormalizeSlug(node.FirstChild.Data)) // href from content
						}
					}
				case ".":
					u.Path = e.HrefView() // href to impression which contains it
				default:
					if u.Path[0] != '/' { // don't touch absolute href
						u.Path = path.Join(e.HrefView(), u.Path) // make path absolute
					}
				}

				node.Attr[attrIdx].Val = u.String()

				// Determine if the link is active
				//
				// # Example leaf "/foo/bar"
				//
				// ## Active paths
				//
				// * /foo
				// * /foo/bar
				//
				// ## Non-active paths
				//
				// * / (because it would be always active)
				// * /foo/bar/baz
				// * /foo/other
				//
				// # Conclusion
				//
				// path must be a prefix of leaf
				//
				// # Example leaf "/"
				//
				// ## Active paths
				//
				// * /

				leaf := e.Leaf().HrefView()

				active := strings.HasPrefix(leaf, u.Path)

				// stricter rule for root node

				root := e.Root().HrefView()

				if active && u.Path == root {
					active = leaf == root
				}

				if active {

					// add class "active"

					classFound := false

					for i := range node.Attr {

						if strings.ToLower(node.Attr[i].Key) != "class" {
							continue
						}

						node.Attr[i].Val = node.Attr[i].Val + " active"
						classFound = true
						break
					}

					// attribute "class" not found, add 'class="active"''

					if !classFound {
						node.Attr = append(node.Attr, html.Attribute{Key: "class", Val: "active"})
					}
				}
			}
		}

		return true, err
	})
	if err != nil {
		return err
	}

	// restore quotation marks etc within template instructions {{ }}
	// regex is not multi-line because text/template writes: "Except for raw strings, actions may not span newlines, although comments can."

	var result = delimiters.ReplaceAllStringFunc(
		util.RenderDomTreeToString(domtree),
		pkghtml.UnescapeString,
	)

	e.SetContent(result)
	return nil
}
