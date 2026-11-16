package en16931

import (
	"unicode/utf8"

	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// isSpace reports whether c is XML white space, as understood by
// normalize-space(): space, tab, carriage return or line feed.
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// trim removes leading and trailing XML white space.
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

// text returns the text of n, or "" when n is nil. It is the string value
// used by XPath comparisons, untrimmed.
func text(n *xmltree.Node) string {
	if n == nil {
		return ""
	}
	return n.Text
}

// blank reports whether normalize-space(n) is the empty string, true when n
// is nil.
func blank(n *xmltree.Node) bool { return n == nil || trim(n.Text) == "" }

// code returns the value of a coded element as the code list rules see it:
// normalize-space(), and false when the result still contains a space.
func code(s string) (string, bool) {
	v := trim(s)
	for i := 0; i < len(v); i++ {
		if isSpace(v[i]) {
			return v, false
		}
	}
	return v, true
}

// tooManyDecimals reports whether string-length(substring-after(n, '.')) > 2,
// the test of the BR-DEC rules. A nil node has no decimals.
func tooManyDecimals(n *xmltree.Node) bool {
	if n == nil {
		return false
	}
	s := n.Text
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return utf8.RuneCountInString(s[i+1:]) > 2
		}
	}
	return false
}

// dec returns xs:decimal(n). It returns false when n is nil or not a decimal.
func dec(n *xmltree.Node) (invoice.Decimal, bool) {
	if n == nil {
		return invoice.Decimal{}, false
	}
	v, err := invoice.ParseDecimal(n.Text)
	return v, err == nil
}

// boolean returns the value of an xs:boolean (true, false, 1, 0 surrounded by
// white space), and false as second result when s is not a boolean.
func boolean(s string) (value, ok bool) {
	switch trim(s) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	}
	return false, false
}

// chargeIndicator returns the value of the charge indicator of an allowance
// or charge, compared with false() or true() as the schematron does: an
// allowance has an Indicator equal to false (or 0), a charge one equal to
// true (or 1).
func chargeIndicator(ac *xmltree.Node) (isAllowance, isCharge bool) {
	for it, ci := elems(ac, "ChargeIndicator"); ci != nil; ci = it.next() {
		for it, ind := elems(ci, "Indicator"); ind != nil; ind = it.next() {
			if v, ok := boolean(ind.Text); ok {
				if v {
					isCharge = true
				} else {
					isAllowance = true
				}
			}
		}
	}
	return isAllowance, isCharge
}

// round2 returns round(v * 100) div 100: v rounded to two decimals, halves
// towards positive infinity.
func round2(v invoice.Decimal) invoice.Decimal { return v.RoundHalfCeil(2) }

// zero is the decimal 0.
var zero = invoice.NewDecimal(0, 0)

// one is the decimal 1.
var one = invoice.NewDecimal(1, 0)

// percentOf returns round(abs(basis) * rate) div 100, the expected VAT amount
// of BR-S-09 and BR-CO-17.
func percentOf(basis, rate invoice.Decimal) invoice.Decimal {
	r := basis.Abs().Mul(rate).RoundHalfCeil(0)
	return invoice.NewDecimal(r.Coef(), 2)
}

// within reports whether |a - b| < 1 (strict) or <= 1 (inclusive).
func within(a, b invoice.Decimal, inclusive bool) bool {
	lo := a.Sub(one).Cmp(b)
	hi := a.Add(one).Cmp(b)
	if inclusive {
		return lo <= 0 && hi >= 0
	}
	return lo < 0 && hi > 0
}

// childText reports whether n has a child named name whose text is exactly v.
func childText(n *xmltree.Node, name, v string) bool {
	if n == nil {
		return false
	}
	for _, c := range n.Children {
		if c.Name == name && c.Text == v {
			return true
		}
	}
	return false
}

// upperIs reports whether upper-case(s) = v, for an upper-case ASCII v.
func upperIs(s, v string) bool {
	if len(s) != len(v) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c != v[i] {
			return false
		}
	}
	return true
}

// isVAT reports whether upper-case(n/ram:TypeCode) = 'VAT'.
func isVAT(n *xmltree.Node) bool {
	tc := n.Elem("TypeCode")
	return tc != nil && upperIs(tc.Text, "VAT")
}

// first returns the first child of n named name (nil safe).
func first(n *xmltree.Node, name string) *xmltree.Node { return n.Elem(name) }

// exists reports whether a descendant of n matches the path of local names.
func exists(n *xmltree.Node, path ...string) bool {
	if n == nil {
		return false
	}
	if len(path) == 0 {
		return true
	}
	for _, c := range n.Children {
		if c.Name == path[0] && exists(c, path[1:]...) {
			return true
		}
	}
	return false
}

// count returns the number of descendants of n matching the path of local
// names.
func count(n *xmltree.Node, path []string) int {
	if n == nil {
		return 0
	}
	if len(path) == 0 {
		return 1
	}
	total := 0
	for _, c := range n.Children {
		if c.Name == path[0] {
			total += count(c, path[1:])
		}
	}
	return total
}

// find returns the first descendant of n matching the path of local names.
func find(n *xmltree.Node, path ...string) *xmltree.Node {
	if n == nil {
		return nil
	}
	if len(path) == 0 {
		return n
	}
	for _, c := range n.Children {
		if c.Name == path[0] {
			if m := find(c, path[1:]...); m != nil {
				return m
			}
		}
	}
	return nil
}

// quote returns s for a message: trimmed, shortened beyond 60 bytes.
func quote(s string) string {
	s = trim(s)
	if len(s) > 60 {
		i := 57
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
		s = s[:i] + "..."
	}
	return s
}

// children returns the child elements of n (nil safe).
func children(n *xmltree.Node) []*xmltree.Node {
	if n == nil {
		return nil
	}
	return n.Children
}

// cursor iterates over the child elements of a node that have a given name,
// without allocating.
type cursor struct {
	kids []*xmltree.Node
	name string
}

// elems returns a cursor over the child elements of n named name (n may be
// nil), and the first of them:
//
//	for it, c := elems(n, "ID"); c != nil; c = it.next() { ... }
func elems(n *xmltree.Node, name string) (cursor, *xmltree.Node) {
	c := cursor{kids: children(n), name: name}
	first := c.next()
	return c, first
}

// next returns the next element, or nil at the end.
func (c *cursor) next() *xmltree.Node {
	for len(c.kids) > 0 {
		k := c.kids[0]
		c.kids = c.kids[1:]
		if k.Name == c.name {
			return k
		}
	}
	return nil
}

// countElems returns the number of child elements of n named name.
func countElems(n *xmltree.Node, name string) int {
	total := 0
	for _, c := range children(n) {
		if c.Name == name {
			total++
		}
	}
	return total
}
