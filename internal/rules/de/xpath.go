package de

import (
	"iter"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ritoon/zerechnung/internal/xmltree"
)

// all yields the elements matching a path of local names below n, in
// document order, exploring every branch (like an XPath location path).
func all(n *xmltree.Node, path string) iter.Seq[*xmltree.Node] {
	return func(yield func(*xmltree.Node) bool) {
		walk(n, path, yield)
	}
}

func walk(n *xmltree.Node, path string, yield func(*xmltree.Node) bool) bool {
	if n == nil {
		return true
	}
	seg, rest, _ := strings.Cut(path, "/")
	for _, c := range n.Children {
		if c.Name != seg {
			continue
		}
		if rest == "" {
			if !yield(c) {
				return false
			}
		} else if !walk(c, rest, yield) {
			return false
		}
	}
	return true
}

// first returns the first element matching path below n, or nil.
func first(n *xmltree.Node, path string) *xmltree.Node {
	for c := range all(n, path) {
		return c
	}
	return nil
}

// exists reports whether an element matches path.
func exists(n *xmltree.Node, path string) bool { return first(n, path) != nil }

// count returns the number of elements matching path.
func count(n *xmltree.Node, path string) int {
	i := 0
	for range all(n, path) {
		i++
	}
	return i
}

// nonBlank reports whether an element matching path has a non-blank value
// (path[boolean(normalize-space(.))]).
func nonBlank(n *xmltree.Node, path string) bool {
	for c := range all(n, path) {
		if nsv(c) != "" {
			return true
		}
	}
	return false
}

// anyEq reports whether an element matching path has the string value v
// (the XPath general comparison path = 'v').
func anyEq(n *xmltree.Node, path, v string) bool {
	for c := range all(n, path) {
		if sv(c) == v {
			return true
		}
	}
	return false
}

// anyIn reports whether an element matching path has one of the string
// values vs (path = ('a', 'b', …)).
func anyIn(n *xmltree.Node, path string, vs ...string) bool {
	for c := range all(n, path) {
		if in(sv(c), vs...) {
			return true
		}
	}
	return false
}

// anyAttrEq reports whether an element matching path has the attribute attr
// with value v (path/@attr = 'v').
func anyAttrEq(n *xmltree.Node, path, attr, v string) bool {
	for c := range all(n, path) {
		if a, ok := c.LookupAttr(attr); ok && a == v {
			return true
		}
	}
	return false
}

// sv returns the XPath string value of n: the concatenated character data of
// the element and its descendants, untrimmed ("" for nil).
func sv(n *xmltree.Node) string {
	if n == nil {
		return ""
	}
	if len(n.Children) == 0 {
		return n.Text
	}
	var b strings.Builder
	appendSV(&b, n)
	return b.String()
}

func appendSV(b *strings.Builder, n *xmltree.Node) {
	b.WriteString(n.Text)
	for _, c := range n.Children {
		appendSV(b, c)
	}
}

// isSpace reports whether c is XML white space (the class \s of XPath
// regular expressions, and the characters normalize-space() strips).
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// norm implements XPath normalize-space(): XML white space is trimmed and
// every run of it is replaced by a single space.
func norm(s string) string {
	clean := true
	for i := 0; i < len(s); i++ {
		if isSpace(s[i]) && (s[i] != ' ' || i == 0 || i == len(s)-1 || s[i+1] == ' ') {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	b := make([]byte, 0, len(s))
	sp := false
	for i := 0; i < len(s); i++ {
		if isSpace(s[i]) {
			sp = len(b) > 0
			continue
		}
		if sp {
			b = append(b, ' ')
			sp = false
		}
		b = append(b, s[i])
	}
	return string(b)
}

// nsv returns normalize-space(n).
func nsv(n *xmltree.Node) string { return norm(sv(n)) }

// attr returns the value of the attribute name of n ("" when n is nil or the
// attribute is absent).
func attr(n *xmltree.Node, name string) string {
	if n == nil {
		return ""
	}
	return n.Attr(name)
}

// in reports whether s is one of vs (s = ('a', 'b', …)).
func in(s string, vs ...string) bool {
	for _, v := range vs {
		if s == v {
			return true
		}
	}
	return false
}

// isDigit matches the XPath regular expression \d: any Unicode decimal digit.
func isDigit(r rune) bool { return r >= '0' && r <= '9' || r > utf8.RuneSelf && unicode.IsDigit(r) }

// hasAncestor reports whether an ancestor of n has the local name name.
func hasAncestor(n *xmltree.Node, name string) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Name == name {
			return true
		}
	}
	return false
}

// parentIs reports whether the parent of n has the local name name.
func parentIs(n *xmltree.Node, name string) bool { return n.Parent != nil && n.Parent.Name == name }
