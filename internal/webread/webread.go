// Package webread reads Moodle's own pages.
//
// It exists for what the APIs cannot reach. On a site with mobile web services
// switched off there is no token, and the endpoint a browser session can use
// exposes only a handful of functions — assignments and grades are not among
// them. The pages are the only remaining source.
//
// Reading a page is worse than calling an API in every way, so the rules here
// are stricter to compensate:
//
//   - Read only. Nothing here submits a form or follows an action.
//   - Anchor on what Moodle generates from data, not on what it shows a
//     person. A class built from a database value survives translation; the
//     words beside it do not.
//   - When an anchor is missing, say the page changed shape. Guessing from
//     whatever else is on the page is how a scraper reports a confident wrong
//     answer.
package webread

import (
	"strings"

	"golang.org/x/net/html"
)

// node is a small wrapper over the parsed tree, so the parsers read as
// intent rather than as traversal.
type node struct{ *html.Node }

// parse reads a document.
func parse(markup string) (node, error) {
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return node{}, err
	}
	return node{root}, nil
}

// classes returns the element's class tokens.
func (n node) classes() []string {
	if n.Node == nil || n.Type != html.ElementNode {
		return nil
	}
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			return strings.Fields(attr.Val)
		}
	}
	return nil
}

// hasClass reports whether the element carries a class token exactly.
//
// Exactly, because Moodle's classes share prefixes: "submissionstatustable"
// would otherwise match a search for "submissionstatus".
func (n node) hasClass(want string) bool {
	for _, class := range n.classes() {
		if class == want {
			return true
		}
	}
	return false
}

// classWithPrefix returns the first class token starting with prefix.
//
// This is how a status is read: Moodle builds the token from the raw database
// value, so "submissionstatusdraft" yields "draft" — the same word the web
// service API returns, and one that no translation touches.
func (n node) classWithPrefix(prefix string) string {
	for _, class := range n.classes() {
		if strings.HasPrefix(class, prefix) && class != prefix {
			return strings.TrimPrefix(class, prefix)
		}
	}
	return ""
}

// find returns the first descendant matching the predicate.
func (n node) find(match func(node) bool) (node, bool) {
	if n.Node == nil {
		return node{}, false
	}
	var walk func(*html.Node) (node, bool)
	walk = func(current *html.Node) (node, bool) {
		candidate := node{current}
		if current.Type == html.ElementNode && match(candidate) {
			return candidate, true
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			if found, ok := walk(child); ok {
				return found, ok
			}
		}
		return node{}, false
	}
	return walk(n.Node)
}

// findAll returns every descendant matching the predicate, in document order.
func (n node) findAll(match func(node) bool) []node {
	var found []node
	if n.Node == nil {
		return found
	}
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		candidate := node{current}
		if current.Type == html.ElementNode && match(candidate) {
			found = append(found, candidate)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n.Node)
	return found
}

// byClass matches an element carrying the class token.
func byClass(class string) func(node) bool {
	return func(n node) bool { return n.hasClass(class) }
}

// byTag matches an element by name.
func byTag(name string) func(node) bool {
	return func(n node) bool { return n.Data == name }
}

// text returns the element's visible text, collapsed to single spaces.
func (n node) text() string {
	if n.Node == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			b.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n.Node)
	return strings.Join(strings.Fields(b.String()), " ")
}

// attr returns an attribute value.
func (n node) attr(name string) string {
	if n.Node == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
