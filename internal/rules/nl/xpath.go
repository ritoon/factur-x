package nl

import (
	"iter"
	"strconv"
	"strings"

	"github.com/ritoon/zerechnung/internal/xmltree"
)

// step is one location step of a rule context: an element of namespace
// space and local name name, or any element when name is "*".
type step struct{ space, name string }

func (s step) is(n *xmltree.Node) bool {
	return s.name == "*" || n.Name == s.name && n.Space == s.space
}

// pattern is the context of a schematron rule, a location path without
// predicate: relative ("ram:SellerTradeParty/ram:PostalTradeAddress"), it
// matches an element and its ancestors from the last step backwards;
// absolute ("/*/rsm:ExchangedDocument/ram:TypeCode"), the first step must
// also be the root element.
type pattern struct {
	steps    []step
	absolute bool
}

// compile parses a context written with the prefixes of ns. It panics on an
// unknown prefix: the contexts are constants of this package.
func compile(path string, ns map[string]string) pattern {
	var p pattern
	if rest, ok := strings.CutPrefix(path, "/"); ok {
		p.absolute, path = true, rest
	}
	for s := range strings.SplitSeq(path, "/") {
		if s == "*" {
			p.steps = append(p.steps, step{name: "*"})
			continue
		}
		prefix, local, ok := strings.Cut(s, ":")
		uri, known := ns[prefix]
		if !ok || !known {
			panic("nl: bad context step " + s)
		}
		p.steps = append(p.steps, step{space: uri, name: local})
	}
	return p
}

// match reports whether n matches the pattern.
func (p pattern) match(n *xmltree.Node) bool {
	c := n
	for i := len(p.steps) - 1; i >= 0; i-- {
		if c == nil || !p.steps[i].is(c) {
			return false
		}
		c = c.Parent
	}
	return !p.absolute || c == nil
}

// all yields the elements matching a path of local names below n, in
// document order (an XPath relative location path).
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

// descendants yields the elements matching path whose first step is any
// descendant of root, in document order (//a/b/c).
func descendants(root *xmltree.Node, path string) iter.Seq[*xmltree.Node] {
	head, rest, _ := strings.Cut(path, "/")
	return func(yield func(*xmltree.Node) bool) {
		stop := false
		root.Walk(func(n *xmltree.Node) bool {
			if stop {
				return false
			}
			if n.Name != head {
				return true
			}
			if rest == "" {
				stop = !yield(n)
			} else {
				stop = !walk(n, rest, yield)
			}
			return !stop
		})
	}
}

// first returns the first element of seq, or nil.
func first(seq iter.Seq[*xmltree.Node]) *xmltree.Node {
	for n := range seq {
		return n
	}
	return nil
}

// list collects seq.
func list(seq iter.Seq[*xmltree.Node]) []*xmltree.Node {
	var out []*xmltree.Node
	for n := range seq {
		out = append(out, n)
	}
	return out
}

// exists reports whether an element matches path below n.
func exists(n *xmltree.Node, path string) bool { return first(all(n, path)) != nil }

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

// isSpace reports whether c is XML white space, which normalize-space()
// strips.
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// norm implements XPath normalize-space().
func norm(s string) string {
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

// anyEq reports whether one of nodes has the string value v (the general
// comparison nodes = 'v').
func anyEq(nodes []*xmltree.Node, v string) bool {
	for _, n := range nodes {
		if sv(n) == v {
			return true
		}
	}
	return false
}

// anyNe reports whether one of nodes has a string value other than v (the
// general comparison nodes != 'v', false for no node).
func anyNe(nodes []*xmltree.Node, v string) bool {
	for _, n := range nodes {
		if sv(n) != v {
			return true
		}
	}
	return false
}

// anyNonBlank reports whether one of nodes has a value that is not blank
// (nodes/normalize-space(.) != "").
func anyNonBlank(nodes []*xmltree.Node) bool {
	for _, n := range nodes {
		if nsv(n) != "" {
			return true
		}
	}
	return false
}

// schemes reports whether the schemeID attributes of nodes, joined with
// spaces, contain one of ids as a word: contains(concat(' ',
// string-join(nodes/@schemeID, ' '), ' '), ' id ').
func schemes(nodes []*xmltree.Node, ids ...string) bool {
	var vals []string
	for _, n := range nodes {
		if v, ok := n.LookupAttr("schemeID"); ok {
			vals = append(vals, v)
		}
	}
	joined := " " + strings.Join(vals, " ") + " "
	for _, id := range ids {
		if strings.Contains(joined, " "+id+" ") {
			return true
		}
	}
	return false
}

// trimXML removes leading and trailing XML white space, as the casts of an
// untyped value do.
func trimXML(s string) string {
	i, j := 0, len(s)
	for i < j && isSpace(s[i]) {
		i++
	}
	for j > i && isSpace(s[j-1]) {
		j--
	}
	return s[i:j]
}

// decimalSign casts s to xs:decimal and returns the sign of the value (-1, 0
// or 1). ok is false when s is not in the lexical space of xs:decimal: an
// optional sign, digits with at most one decimal point, no exponent.
func decimalSign(s string) (sign int, ok bool) {
	s = trimXML(s)
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	digits, dot, nonZero := 0, false, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '.' && !dot:
			dot = true
		case c >= '0' && c <= '9':
			digits++
			nonZero = nonZero || c != '0'
		default:
			return 0, false
		}
	}
	switch {
	case digits == 0:
		return 0, false
	case !nonZero:
		return 0, true
	case neg:
		return -1, true
	}
	return 1, true
}

// double casts s to xs:double, as an untyped value compared with a number
// is. ok is false when s is not in the lexical space of xs:double.
func double(s string) (f float64, ok bool) {
	s = trimXML(s)
	switch s {
	case "INF", "+INF", "-INF", "NaN":
		f, _ = strconv.ParseFloat(s, 64)
		return f, true
	}
	mant, exp, hasExp := strings.Cut(strings.ReplaceAll(s, "E", "e"), "e")
	if hasExp {
		e := strings.TrimPrefix(strings.TrimPrefix(exp, "+"), "-")
		if e == "" || strings.Trim(e, "0123456789") != "" {
			return 0, false
		}
	}
	if _, ok := decimalSign(mant); !ok || mant != trimXML(mant) {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		if ne, isNum := err.(*strconv.NumError); isNum && ne.Err == strconv.ErrRange {
			return f, true // ±Inf or 0, as the cast rounds
		}
		return 0, false
	}
	return f, true
}

// numEq evaluates the general comparison nodes = v for a number v: each
// value is cast to xs:double. ok is false when a value does not cast, which
// stops the reference implementation with an error.
func numEq(nodes []*xmltree.Node, vs ...float64) (eq, ok bool) {
	for _, n := range nodes {
		f, castOK := double(sv(n))
		if !castOK {
			return false, false
		}
		for _, v := range vs {
			if f == v {
				eq = true
			}
		}
	}
	return eq, true
}

// emptyLeaf reports whether n has no child element and a blank string value
// (not(*) and not(normalize-space())).
func emptyLeaf(n *xmltree.Node) bool {
	if len(n.Children) > 0 {
		return false
	}
	for i := 0; i < len(n.Text); i++ {
		if !isSpace(n.Text[i]) {
			return false
		}
	}
	return true
}
