package peppol

import (
	"errors"
	"iter"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// XPath 2.0 helpers. Elements are matched by local name; values are compared
// as the reference implementation compares them: general comparisons on the
// raw string value, normalize-space on XML white space only.

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

// first returns the first element matching path below n, or nil. Where the
// reference would stop on an XPath type error (several values given to a
// single-valued function), the rules use the first value.
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

// anyEq reports whether an element matching path has the string value v
// (path = 'v').
func anyEq(n *xmltree.Node, path, v string) bool {
	for c := range all(n, path) {
		if sv(c) == v {
			return true
		}
	}
	return false
}

// anyAttrIn reports whether an element matching path has the attribute attr
// with one of the values vs (path/@attr = ('a', 'b', …)).
func anyAttrIn(n *xmltree.Node, path, attr string, vs ...string) bool {
	for c := range all(n, path) {
		if a, ok := c.LookupAttr(attr); ok && in(a, vs...) {
			return true
		}
	}
	return false
}

// firstAttr returns normalize-space(path/@attr): the first attribute attr
// found on the elements matching path, "" when none has it.
func firstAttr(n *xmltree.Node, path, attr string) string {
	for c := range all(n, path) {
		if a, ok := c.LookupAttr(attr); ok {
			return norm(a)
		}
	}
	return ""
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

// text returns n/text(): the character data directly inside n ("" for nil).
func text(n *xmltree.Node) string {
	if n == nil {
		return ""
	}
	return n.Text
}

// isSpace reports whether c is XML white space.
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// trim removes leading and trailing XML white space (the whitespace facet
// applied when a string is cast to a number).
func trim(s string) string {
	i, j := 0, len(s)
	for i < j && isSpace(s[i]) {
		i++
	}
	for j > i && isSpace(s[j-1]) {
		j--
	}
	return s[i:j]
}

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

// length returns string-length(s): its number of characters.
func length(s string) int { return utf8.RuneCountInString(s) }

// substring implements substring(s, start, n) for integer arguments: the
// characters at the positions p (from 1) with start <= p < start + n.
func substring(s string, start, n int) string {
	end := start + n
	var b strings.Builder
	p := 0
	for _, r := range s {
		p++
		if p >= end {
			break
		}
		if p >= start {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// substringFrom implements substring(s, start): the characters from
// position start (from 1).
func substringFrom(s string, start int) string {
	p := 0
	for i := range s {
		p++
		if p >= start {
			return s[i:]
		}
	}
	return ""
}

// allDigits reports whether s is not empty and made of the ASCII digits 0-9
// only (the regular expression ^[0-9]+$).
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// digits reports whether s has exactly n ASCII digits (^[0-9]{n}$).
func digits(s string, n int) bool { return len(s) == n && allDigits(s) }

// letters reports whether s holds only ASCII letters, as the schematron
// tests by removing the letters with translate() and checking that nothing
// is left. The empty string qualifies.
func letters(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i] | 0x20; c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

// castableInteger implements s castable as xs:integer: an optional sign and
// ASCII digits, surrounding white space ignored.
func castableInteger(s string) bool {
	s = trim(s)
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	return allDigits(s)
}

// double implements the cast of a string to xs:double, as number() and the
// general comparisons with a number do. It reports false when s is not a
// valid xs:double literal.
func double(s string) (float64, bool) {
	s = trim(s)
	switch s {
	case "INF":
		return math.Inf(1), true
	case "-INF":
		return math.Inf(-1), true
	case "NaN":
		return math.NaN(), true
	}
	m, exp, hasExp := strings.Cut(strings.ReplaceAll(s, "E", "e"), "e")
	if m != "" && (m[0] == '+' || m[0] == '-') {
		m = m[1:]
	}
	ip, fp, _ := strings.Cut(m, ".")
	if ip == "" && fp == "" || ip != "" && !allDigits(ip) || fp != "" && !allDigits(fp) {
		return 0, false
	}
	if hasExp {
		if exp != "" && (exp[0] == '+' || exp[0] == '-') {
			exp = exp[1:]
		}
		if !allDigits(exp) {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return f, true
}

// number implements number(s): the xs:double value of s, NaN when s is not
// a number.
func number(s string) float64 {
	if f, ok := double(s); ok {
		return f
	}
	return math.NaN()
}

// isNumber reports whether string(number(s)) != 'NaN'.
func isNumber(s string) bool { return !math.IsNaN(number(s)) }

// dec implements xs:decimal(n) on an element. It reports false when n is nil
// or its value is not a decimal: the reference would stop with an error, and
// the rules that need the value are then skipped.
func dec(n *xmltree.Node) (*big.Rat, bool) {
	if n == nil {
		return nil, false
	}
	d, err := invoice.ParseDecimal(sv(n))
	if err != nil {
		return nil, false
	}
	return rat(d), true
}

// rat returns the exact value of d.
func rat(d invoice.Decimal) *big.Rat {
	den := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d.Scale())), nil)
	return new(big.Rat).SetFrac(big.NewInt(d.Coef()), den)
}

// within reports whether |a - b| <= slack (the u:slack function of the
// schematron).
func within(a, b, slack *big.Rat) bool {
	d := new(big.Rat).Sub(a, b)
	return d.Abs(d).Cmp(slack) <= 0
}

// parentIs reports whether the parent of n has the local name name.
func parentIs(n *xmltree.Node, name string) bool { return n.Parent != nil && n.Parent.Name == name }
