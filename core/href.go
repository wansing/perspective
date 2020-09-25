package core

import (
	"fmt"
	"path"
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

// recursive
func (n *Node) whereami(external bool, versionNo ...int) []string {

	if n == nil {
		return []string{}
	}

	// recurse

	var where = n.Parent.whereami(external, versionNo...)

	// URL segment

	if !external || (external && !n.IsPushed()) {

		var segment = n.Slug()

		if n.Parent == nil { // omit root slug
			segment = ""
		}

		for i := 0; i < len(versionNo)-1; i = i + 2 {
			if versionNo[i] == n.Id() {
				segment = fmt.Sprintf("%s:%d", segment, versionNo[i+1])
			}
		}

		if segment != "" { // true for root iff it has a non-default versionNo
			where = append(where, segment)
		}
	}

	if external {
		where = append(where, n.AdditionalSlugs()...)
	}

	return where
}

// Href returns the location of the node as a string.
// Like Queue.String(), it returns like "/foo:42/bar" or "/".
//
// Usually params are omitted and pushed slugs are included.
// Pass external = true to invert this behavior.
func (n *Node) Href(external bool, versionNo ...int) string {

	var href string

	if segments := n.whereami(external, versionNo...); len(segments) > 0 {
		href += "/" + strings.Join(segments, "/")
	}

	if href == "" {
		href = "/"
	}

	return href
}

func (n *Node) HrefPath(versionNo ...int) string {
	return n.Href(false, versionNo...)
}

func (n *Node) HrefView(versionNo ...int) string {
	return n.Href(true, versionNo...)
}

// If p is relative, MakeAbsolute prepends the HrefPath of the receiver.
// Then p is cleaned and returned.
func (n *Node) MakeAbsolute(p string) string {
	if !path.IsAbs(p) {
		p = n.HrefPath() + "/" + p
	}
	return path.Clean(p)
}
