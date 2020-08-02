package util

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var ErrNilNode = errors.New("HTML node is nil")

// CreateDomTree reads from a reader and parses the content into an html.Node.
// It returns a body node.
func CreateDomTree(bodyReader io.Reader) (*html.Node, error) {

	parsed, err := html.ParseFragment(
		io.MultiReader(
			strings.NewReader("<body>"),
			bodyReader,
			strings.NewReader("</body>"),
		),
		&html.Node{
			Type:     html.ElementNode,
			DataAtom: atom.Html,
			Data:     "html",
		},
	)

	if err == nil {
		return parsed[1], nil // [0] is head, [1] is body, we want the body node
	} else {
		return nil, err
	}
}

func renderDomTree(root *html.Node, buf *bytes.Buffer) error {
	if root == nil || buf == nil {
		return nil
	}
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		err := html.Render(buf, node)
		if err != nil {
			return err
		}
	}
	return nil
}

// RenderDomTreeToString renders an html.Node into a string.
// If an error occurs, the error string is returned.
func RenderDomTreeToString(root *html.Node) string {

	if root == nil {
		return ""
	}

	buf := &bytes.Buffer{}
	err := renderDomTree(root, buf)

	if err == nil {
		return buf.String()
	} else {
		return err.Error()
	}
}

// ForEachDomNode calls a task func for each node, including root.
// It recurses (pre-order) if and only if the task returns true.
//
// The task might replace the node, so its NextSibling might change.
func ForEachDomNode(root *html.Node, task func(*html.Node) (bool, error)) error {

	if root == nil {
		return ErrNilNode
	}

	recurse, err := task(root)
	if err != nil {
		return err
	}
	if !recurse {
		return nil
	}

	for child := root.FirstChild; child != nil; {

		nextSiblingBackup := child.NextSibling // backup because the task might modify child.NextSibling

		err = ForEachDomNode(child, task)
		if err != nil {
			return err
		}

		child = nextSiblingBackup
	}

	return nil
}
