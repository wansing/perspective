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
func (n *Node) whereami(external bool, versionFunc func(*Node) int) []string {

	if n == nil {
		return []string{}
	}

	// recurse

	var where = n.Parent.whereami(external, versionFunc)

	// URL segment

	if !external || (external && !n.IsPushed()) {

		var segment = n.Slug()

		if n.Parent == nil { // omit root slug
			segment = ""
		}

		if versionFunc != nil {
			if versionNo := versionFunc(n); versionNo != 0 {
				segment = fmt.Sprintf("%s:%d", segment, versionNo)
			}
		}

		if segment != "" { // true for root iff it has a non-default versionNo
			where = append(where, segment)
		}
	}

	if external {
		where = append(where, n.ExternalURLSegments()...)
	}

	return where
}

// Href returns the location of the node as a string.
// Like Queue.String(), it returns like "/foo:42/bar" or "/".
//
// Usually params are omitted and pushed slugs are included.
// Pass external = true to invert this behavior.
//
// If a versionFunc is supplied, it is called to include version information.
func (n *Node) Href(external bool, versionFunc func(*Node) int) string {

	var href string

	if segments := n.whereami(external, versionFunc); len(segments) > 0 {
		href += "/" + strings.Join(segments, "/")
	}

	if href == "" {
		href = "/"
	}

	return href
}

// href for visitors viewing the site
var preferVersionNo = func(n *Node) int {
	if n.VersionNo() != n.MaxWGZeroVersionNo() {
		return n.VersionNo()
	}
	return 0
}

// HrefView calls Href with a versionFunc that appends a version number
// if and only if it differs from the MaxWGZeroVersionNo.
func (n *Node) HrefView() string {
	return n.Href(true, preferVersionNo)
}

// HrefPath calls Href with default values. Its result represents the database structure.
func (n *Node) HrefPath() string {
	return n.Href(false, nil)
}

// If p is relative, MakeAbsolute prepends the HrefPath of the receiver.
// Then p is cleaned and returned.
func (n *Node) MakeAbsolute(p string) string {
	if !path.IsAbs(p) {
		p = n.HrefPath() + "/" + p
	}
	return path.Clean(p)
}
