package core

import (
	"regexp"
	"strings"
)

// doesn't contain the dot
var slugRegex *regexp.Regexp = regexp.MustCompile(`[^a-z0-9]+`)

// Normalizes a slug. Especially, dots are removed.
// Almost identical to the javascript function "normalizeSlug".
func NormalizeSlug(slug string) string {

	slug = strings.ToLower(strings.TrimSpace(slug))
	slug = slugRegex.ReplaceAllString(slug, `-`)

	// in addition to the javascript function, remove trailing dashes
	for len(slug) > 0 && slug[len(slug)-1] == '-' {
		slug = slug[:len(slug)-1]
	}

	return slug
}

func (n *Node) Location() string {

	var nodes = []*Node{} // reversed
	for p := n; p != nil; p = p.Parent {
		nodes = append(nodes, p)
	}

	var slugs = []string{}

	for i := len(nodes) - 2; i >= 0; i-- { // -2, omit root slug
		slugs = append(slugs, nodes[i].Slug())
	}

	return "/" + strings.Join(slugs, "/")
}

func (n *Node) Link() string {

	var nodes = []*Node{} // reversed
	for p := n; p != nil; p = p.Parent {
		nodes = append(nodes, p)
	}

	var slugs = []string{}

	for i := len(nodes) - 1; i >= 0; i-- {
		if segment := nodes[i].Slug(); nodes[i].Parent != nil && segment != "default" { // neither root nor "default"
			slugs = append(slugs, segment)
		}
		slugs = append(slugs, nodes[i].AddSlugs()...)
	}

	return "/" + strings.Join(slugs, "/")
}
