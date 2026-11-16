// Package schema checks the structure of a CII invoice against the XML
// schema of its Factur-X profile: allowed elements, order, cardinalities,
// attributes, namespaces and simple types.
//
// The schemas of the five Factur-X profiles (MINIMUM, BASIC WL, BASIC,
// EN 16931, EXTENDED) are compiled ahead of time into the tables of
// tables_gen.go by the generator in ./gen, so validation needs no schema
// engine and does not allocate while the document is valid. Types that are
// identical in several profiles are shared.
//
// XRECHNUNG invoices are checked against the EN 16931 schema and
// EXTENDED-CTC-FR invoices against the EXTENDED schema. An invoice whose
// profile is unknown is checked against the EXTENDED schema, the most
// permissive, with a warning.
package schema

//go:generate go run ./gen -o tables_gen.go ${FX_XSD_DIR}

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// Rule identifiers.
const (
	RuleProfile          = "XSD-PROFILE"           // unknown profile: checked against EXTENDED (warning)
	RuleRoot             = "XSD-ROOT"              // the root element is not rsm:CrossIndustryInvoice
	RuleNamespace        = "XSD-NAMESPACE"         // element in the wrong namespace
	RuleUnexpected       = "XSD-UNEXPECTED"        // element not allowed in its parent
	RuleOrder            = "XSD-ORDER"             // element allowed in its parent, but misplaced
	RuleMissing          = "XSD-MISSING"           // required element missing
	RuleCount            = "XSD-COUNT"             // element repeated more than allowed
	RuleAttribute        = "XSD-ATTRIBUTE"         // attribute not declared
	RuleAttributeMissing = "XSD-ATTRIBUTE-MISSING" // required attribute missing
	RuleType             = "XSD-TYPE"              // value not valid for its simple type
	RuleText             = "XSD-TEXT"              // text in an element that only holds elements
)

// Catalog lists the rules implemented by this package.
var Catalog = []rules.RuleInfo{
	{ID: RuleProfile, Severity: rules.Warning, Description: "The profile (BT-24) is unknown: the structure is checked against the EXTENDED schema, the most permissive."},
	{ID: RuleRoot, Severity: rules.Error, Description: "The root element shall be rsm:CrossIndustryInvoice."},
	{ID: RuleNamespace, Severity: rules.Error, Description: "Each element shall be in the namespace defined by the schema (rsm, ram, udt or qdt)."},
	{ID: RuleUnexpected, Severity: rules.Error, Description: "Each element shall be allowed in its parent by the schema of the profile."},
	{ID: RuleOrder, Severity: rules.Error, Description: "Child elements shall appear in the order defined by the schema."},
	{ID: RuleMissing, Severity: rules.Error, Description: "Elements required by the schema (minOccurs > 0) shall be present."},
	{ID: RuleCount, Severity: rules.Error, Description: "An element shall not occur more often than allowed by the schema (maxOccurs)."},
	{ID: RuleAttribute, Severity: rules.Error, Description: "Only the attributes declared by the schema are allowed."},
	{ID: RuleAttributeMissing, Severity: rules.Error, Description: "Attributes required by the schema shall be present."},
	{ID: RuleType, Severity: rules.Error, Description: "Element and attribute values shall match their simple type (xs:decimal, xs:boolean, xs:base64Binary, enumerations, lengths, patterns)."},
	{ID: RuleText, Severity: rules.Error, Description: "Elements whose content is made of elements shall not contain text."},
}

// simpleKind is the lexical space checked for a simple type.
type simpleKind uint8

const (
	kString  simpleKind = iota // xs:string, xs:token and other unconstrained string types
	kDecimal                   // xs:decimal
	kInteger                   // xs:integer and its derived types
	kBoolean                   // xs:boolean
	kBase64                    // xs:base64Binary
	kDate                      // xs:date
)

// contentKind is the content model of a complex type.
type contentKind uint8

const (
	contentEmpty    contentKind = iota // no element, no text
	contentElements                    // a sequence or a choice of elements
	contentSimple                      // text of a simple type, with attributes
)

// unbounded is the maxOccurs value meaning "unbounded".
const unbounded = 255

// Profiles of the tables, from the smallest to the largest.
const (
	pMinimum = iota
	pBasicWL
	pBasic
	pEN16931
	pExtended
	numProfiles
)

var profileNames = [numProfiles]string{"MINIMUM", "BASIC WL", "BASIC", "EN 16931", "EXTENDED"}

// ctype is a compiled element type.
type ctype struct {
	name       string // type name, with the conventional prefix ("ram:TradePartyType")
	content    contentKind
	choice     bool  // the elements form a choice rather than a sequence
	min, max   uint8 // occurrences of the choice
	simple     simpleKind
	facets     uint16 // index in facetTable, 0 = none
	simpleName string // name of the simple type, for messages
	first, n   uint16 // particles[first : first+n]
	afirst, an uint16 // attributes[afirst : afirst+an]
	profiles   uint8  // bit mask of the profiles using this type
}

// particle is a child element of a type.
type particle struct {
	name     string
	ns       uint8  // index in namespaces
	typ      uint16 // index in types
	min, max uint8  // max is unbounded for maxOccurs="unbounded"
}

// attribute is an attribute declaration.
type attribute struct {
	name     string
	kind     simpleKind
	facets   uint16
	required bool
}

// facets restrict a simple type.
type facets struct {
	enum           []string
	patterns       []string
	minLen, maxLen int // -1 = none
	collapse       bool
}

// facetRegexps holds the compiled patterns of facetTable.
var facetRegexps = func() [][]*regexp.Regexp {
	out := make([][]*regexp.Regexp, len(facetTable))
	for i, f := range facetTable {
		for _, p := range f.patterns {
			out[i] = append(out[i], regexp.MustCompile("^(?:"+p+")$"))
		}
	}
	return out
}()

// profileIndex returns the schema used for an invoice profile.
func profileIndex(p invoice.Profile) int {
	switch p {
	case invoice.ProfileMinimum:
		return pMinimum
	case invoice.ProfileBasicWL:
		return pBasicWL
	case invoice.ProfileBasic:
		return pBasic
	case invoice.ProfileEN16931, invoice.ProfileXRechnung:
		return pEN16931
	}
	return pExtended
}

// Version returns the version of the Factur-X schemas the tables were
// generated from.
func Version() string { return schemaVersion }

// Check validates the structure of c.Doc against the schema of c.Doc.Profile
// and records the findings in c.Report.
func Check(c *rules.Checker) {
	if c.Doc == nil || c.Doc.Root == nil {
		return
	}
	p := profileIndex(c.Doc.Profile)
	if c.Doc.Profile == invoice.ProfileUnknown {
		at := c.Doc.Context.Find("GuidelineSpecifiedDocumentContextParameter/ID")
		if at == nil {
			at = c.Doc.Root
		}
		c.Fail(RuleProfile, rules.Warning, at,
			"unknown profile (BT-24 %q): the structure is checked against the EXTENDED schema, the most permissive", c.Doc.SpecificationID())
	}
	v := validator{c: c, p: p}
	r := &roots[p]
	root := c.Doc.Root
	if root.Name != r.name {
		c.Fail(RuleRoot, rules.Error, root, "the root element is %s, expected rsm:%s", display(root), r.name)
		return
	}
	if root.Space != namespaces[r.ns] {
		c.Fail(RuleNamespace, rules.Error, root, "%s is in namespace %q, expected %q", display(root), root.Space, namespaces[r.ns])
	}
	v.element(root, &types[r.typ])
}

type validator struct {
	c *rules.Checker
	p int // profile index
}

func (v *validator) fail(rule string, n *xmltree.Node, format string, args ...any) {
	v.c.Fail(rule, rules.Error, n, format, args...)
}

// element validates n, an element of type t.
func (v *validator) element(n *xmltree.Node, t *ctype) {
	if len(n.Attrs) > 0 || t.an > 0 {
		v.attributes(n, t)
	}
	if t.content == contentSimple {
		if len(n.Children) > 0 {
			v.fail(RuleUnexpected, n.Children[0], "%s is not allowed: %s holds a value, not elements", display(n.Children[0]), display(n))
		}
		if t.simple != kString || t.facets != 0 {
			if msg := checkValue(n.Text, t.simple, t.facets); msg != "" {
				v.fail(RuleType, n, "%s: value %q %s %s", display(n), short(n.Text), msg, typeLabel(t.simpleName, t.simple))
			}
		}
		return
	}
	if !blank(n.Text) {
		v.fail(RuleText, n, "%s holds elements only, text %q is not allowed", display(n), short(n.Text))
	}
	if t.choice {
		v.choice(n, t)
	} else {
		v.sequence(n, t)
	}
}

// sequence validates the children of n against the sequence of t.
//
// The schemas satisfy the unique particle attribution constraint and the
// generator guarantees that local names are unique in a content model, so
// children are matched greedily by local name. When a child does not fit,
// the error is classified (missing, misplaced, unexpected) and matching
// resumes so that a single mistake gives a single finding.
func (v *validator) sequence(n *xmltree.Node, t *ctype) {
	ps := particles[t.first : t.first+t.n]
	pi, cnt := 0, 0 // current particle and its occurrences so far
	kids := n.Children
	for k, ch := range kids {
		if pi < len(ps) && ps[pi].name == ch.Name {
			cnt++
			v.matched(n, ch, &ps[pi], cnt)
			continue
		}
		j := pi + 1
		for j < len(ps) && ps[j].name != ch.Name {
			j++
		}
		if j < len(ps) {
			// Moving forward: the particles skipped must be optional, unless
			// they appear later (they are then reported as misplaced).
			for s := pi; s < j; s++ {
				have := 0
				if s == pi {
					have = cnt
				}
				if have < int(ps[s].min) && !hasChild(kids[k+1:], ps[s].name) {
					v.fail(RuleMissing, n, "%s is missing in %s (expected before %s)", ps[s].display(), display(n), ps[j].display())
				}
			}
			pi, cnt = j, 1
			v.matched(n, ch, &ps[pi], 1)
			continue
		}
		if e := find(ps[:min(pi, len(ps))], ch.Name); e >= 0 {
			// Name the first preceding sibling that must follow ch.
			before := ps[pi].display()
			for _, prev := range kids[:k] {
				if find(ps, prev.Name) > e {
					before = display(prev)
					break
				}
			}
			v.fail(RuleOrder, ch, "%s is not allowed at this position in %s: it must come before %s; expected here: %s",
				display(ch), display(n), before, expected(ps, pi, cnt))
			v.namespace(ch, &ps[e])
			v.element(ch, &types[ps[e].typ])
			continue
		}
		v.unexpected(n, t, ch, expected(ps, pi, cnt))
	}
	for s := pi; s < len(ps); s++ {
		have := 0
		if s == pi {
			have = cnt
		}
		if have < int(ps[s].min) {
			v.fail(RuleMissing, n, "%s is missing in %s", ps[s].display(), display(n))
		}
	}
}

// choice validates the children of n against the choice of t. Consecutive
// children matching the same alternative, within its maxOccurs, form one
// occurrence of the choice.
func (v *validator) choice(n *xmltree.Node, t *ctype) {
	ps := particles[t.first : t.first+t.n]
	occ, alt, cnt := 0, -1, 0
	for _, ch := range n.Children {
		j := find(ps, ch.Name)
		if j < 0 {
			v.unexpected(n, t, ch, alternatives(ps))
			continue
		}
		if j == alt && (ps[j].max == unbounded || cnt < int(ps[j].max)) {
			cnt++
		} else {
			if alt >= 0 && cnt < int(ps[alt].min) {
				v.fail(RuleMissing, n, "%s is missing in %s", ps[alt].display(), display(n))
			}
			alt, cnt = j, 1
			occ++
			if t.max != unbounded && occ > int(t.max) {
				v.fail(RuleCount, ch, "%s: only %d of %s allowed in %s", display(ch), t.max, alternatives(ps), display(n))
			}
		}
		v.namespace(ch, &ps[j])
		v.element(ch, &types[ps[j].typ])
	}
	if alt >= 0 && cnt < int(ps[alt].min) {
		v.fail(RuleMissing, n, "%s is missing in %s", ps[alt].display(), display(n))
	}
	if occ < int(t.min) {
		v.fail(RuleMissing, n, "%s is missing in %s", alternatives(ps), display(n))
	}
}

// matched validates ch, the cnt-th occurrence of particle p in n.
func (v *validator) matched(n, ch *xmltree.Node, p *particle, cnt int) {
	if p.max != unbounded && cnt > int(p.max) {
		if p.max == 1 {
			v.fail(RuleCount, ch, "%s may occur only once in %s", display(ch), display(n))
		} else {
			v.fail(RuleCount, ch, "%s may occur at most %d times in %s", display(ch), p.max, display(n))
		}
	}
	v.namespace(ch, p)
	v.element(ch, &types[p.typ])
}

func (v *validator) namespace(ch *xmltree.Node, p *particle) {
	if ch.Space != namespaces[p.ns] {
		v.fail(RuleNamespace, ch, "%s is in namespace %q, expected %q (%s)", display(ch), ch.Space, namespaces[p.ns], p.display())
	}
}

// unexpected reports ch, a child of n that type t does not declare. When a
// larger profile allows it, the message says so.
func (v *validator) unexpected(n *xmltree.Node, t *ctype, ch *xmltree.Node, want string) {
	if q := v.allowedFrom(t, ch.Name); q >= 0 {
		v.fail(RuleUnexpected, ch, "%s is not allowed in %s in profile %s (it is allowed from profile %s); expected here: %s",
			display(ch), display(n), profileNames[v.p], profileNames[q], want)
		return
	}
	v.fail(RuleUnexpected, ch, "%s is not allowed in %s; expected here: %s", display(ch), display(n), want)
}

// allowedFrom returns the smallest profile above the current one whose
// version of type t declares a child named name, or -1.
func (v *validator) allowedFrom(t *ctype, name string) int {
	for q := v.p + 1; q < numProfiles; q++ {
		for i := range types {
			u := &types[i]
			if u.profiles&(1<<q) == 0 || u.name != t.name {
				continue
			}
			if find(particles[u.first:u.first+u.n], name) >= 0 {
				return q
			}
		}
	}
	return -1
}

// attributes validates the attributes of n against those of t.
func (v *validator) attributes(n *xmltree.Node, t *ctype) {
	as := attributes[t.afirst : t.afirst+t.an]
	for i := range n.Attrs {
		a := &n.Attrs[i]
		if a.Prefix != "" {
			// xmltree does not resolve attribute namespaces: the conventional
			// xsi prefix is trusted for the XML Schema instance attributes.
			if a.Prefix == "xsi" && (a.Name == "schemaLocation" || a.Name == "noNamespaceSchemaLocation" || a.Name == "type") {
				continue
			}
			v.fail(RuleAttribute, n, "attribute %s:%s is not allowed on %s", a.Prefix, a.Name, display(n))
			continue
		}
		d := -1
		for j := range as {
			if as[j].name == a.Name {
				d = j
				break
			}
		}
		if d < 0 {
			v.fail(RuleAttribute, n, "attribute %s is not allowed on %s%s", a.Name, display(n), attrList(as))
			continue
		}
		if as[d].kind != kString || as[d].facets != 0 {
			if msg := checkValue(a.Value, as[d].kind, as[d].facets); msg != "" {
				v.fail(RuleType, n, "attribute %s of %s: value %q %s %s", a.Name, display(n), short(a.Value), msg, typeLabel("", as[d].kind))
			}
		}
	}
	for j := range as {
		if !as[j].required {
			continue
		}
		if _, ok := n.LookupAttr(as[j].name); !ok {
			v.fail(RuleAttributeMissing, n, "attribute %s is required on %s", as[j].name, display(n))
		}
	}
}

// display returns the name of a particle with its conventional prefix.
func (p *particle) display() string {
	if pre := nsPrefix(namespaces[p.ns]); pre != "" {
		return pre + ":" + p.name
	}
	return p.name
}

// display returns the name of an element with the conventional prefix of its
// namespace, or the prefix used in the document.
func display(n *xmltree.Node) string {
	if pre := nsPrefix(n.Space); pre != "" {
		return pre + ":" + n.Name
	}
	if n.Prefix != "" {
		return n.Prefix + ":" + n.Name
	}
	return n.Name
}

func nsPrefix(ns string) string {
	switch ns {
	case rules.NSRsm:
		return "rsm"
	case rules.NSRam:
		return "ram"
	case rules.NSUdt:
		return "udt"
	case rules.NSQdt:
		return "qdt"
	}
	return ""
}

// expected lists the elements that may come next in a sequence, the current
// particle being pi with cnt occurrences.
func expected(ps []particle, pi, cnt int) string {
	var names []string
	for s := pi; s < len(ps); s++ {
		have := 0
		if s == pi {
			have = cnt
		}
		if ps[s].max == unbounded || have < int(ps[s].max) {
			names = append(names, ps[s].display())
		}
		if have < int(ps[s].min) {
			break
		}
	}
	if len(names) == 0 {
		return "no further element"
	}
	return orList(names)
}

func alternatives(ps []particle) string {
	names := make([]string, len(ps))
	for i := range ps {
		names[i] = ps[i].display()
	}
	return orList(names)
}

func orList(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " or " + names[1]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

func attrList(as []attribute) string {
	if len(as) == 0 {
		return " (no attribute allowed)"
	}
	names := make([]string, len(as))
	for i := range as {
		names[i] = as[i].name
	}
	return " (allowed: " + strings.Join(names, ", ") + ")"
}

func find(ps []particle, name string) int {
	for i := range ps {
		if ps[i].name == name {
			return i
		}
	}
	return -1
}

func hasChild(kids []*xmltree.Node, name string) bool {
	for _, k := range kids {
		if k.Name == name {
			return true
		}
	}
	return false
}

func typeLabel(name string, k simpleKind) string {
	if name != "" {
		return name
	}
	switch k {
	case kDecimal:
		return "xs:decimal"
	case kInteger:
		return "xs:integer"
	case kBoolean:
		return "xs:boolean"
	case kBase64:
		return "xs:base64Binary"
	case kDate:
		return "xs:date"
	}
	return "xs:string"
}

// short truncates a value for messages.
func short(s string) string {
	s = trim(s)
	if len(s) > 40 {
		i := 37
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
		return s[:i] + "..."
	}
	return s
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

func blank(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isSpace(s[i]) {
			return false
		}
	}
	return true
}

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

// checkValue checks a value against a simple type. It returns "" when the
// value is valid, else the reason ("is not a valid", "is not one of"...),
// completed by the caller with the type name.
func checkValue(s string, k simpleKind, fi uint16) string {
	ok := true
	switch k {
	case kDecimal:
		ok = validDecimal(trim(s))
	case kInteger:
		ok = validInteger(trim(s))
	case kBoolean:
		switch trim(s) {
		case "true", "false", "1", "0":
		default:
			ok = false
		}
	case kBase64:
		ok = validBase64(s)
	case kDate:
		ok = validDate(trim(s))
	}
	if !ok {
		return "is not a valid"
	}
	if fi == 0 {
		return ""
	}
	return checkFacets(s, fi)
}

func checkFacets(s string, fi uint16) string {
	return facetTable[fi].check(s, facetRegexps[fi])
}

// check checks s against the facets; res are the compiled patterns. It
// returns "" when s is valid, else the reason.
func (f *facets) check(s string, res []*regexp.Regexp) string {
	if f.collapse {
		s = strings.Join(strings.Fields(s), " ")
	}
	if len(f.enum) > 0 {
		found := false
		for _, e := range f.enum {
			if e == s {
				found = true
				break
			}
		}
		if !found {
			return "is not an allowed value (" + strings.Join(f.enum, ", ") + ") of"
		}
	}
	if f.minLen >= 0 || f.maxLen >= 0 {
		l := utf8.RuneCountInString(s)
		if (f.minLen >= 0 && l < f.minLen) || (f.maxLen >= 0 && l > f.maxLen) {
			return "has a length out of the bounds of"
		}
	}
	for _, re := range res {
		if !re.MatchString(s) {
			return "does not match the pattern of"
		}
	}
	return ""
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// validDecimal reports whether s is an xs:decimal lexical value: optional
// sign, digits, optional fraction; at least one digit; no exponent.
func validDecimal(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits, dot := 0, false
	for ; i < len(s); i++ {
		switch c := s[i]; {
		case isDigit(c):
			digits++
		case c == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return digits > 0
}

// validInteger reports whether s is an xs:integer lexical value.
func validInteger(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if i == len(s) {
		return false
	}
	for ; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// validBase64 reports whether s is an xs:base64Binary lexical value: groups
// of four characters of the base64 alphabet, white space allowed anywhere,
// with at most two padding characters at the end.
func validBase64(s string) bool {
	n, pad := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case isSpace(c):
			continue
		case c == '=':
			pad++
			if pad > 2 {
				return false
			}
		case pad > 0:
			return false // data after padding
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', isDigit(c), c == '+', c == '/':
		default:
			return false
		}
		n++
	}
	return n%4 == 0
}

// validDate reports whether s is an xs:date lexical value
// (-?YYYY-MM-DD with an optional time zone).
func validDate(s string) bool {
	if strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	if len(s) < 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for _, i := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if !isDigit(s[i]) {
			return false
		}
	}
	m := int(s[5]-'0')*10 + int(s[6]-'0')
	d := int(s[8]-'0')*10 + int(s[9]-'0')
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return false
	}
	switch tz := s[10:]; {
	case tz == "", tz == "Z":
		return true
	case len(tz) == 6 && (tz[0] == '+' || tz[0] == '-') && tz[3] == ':' &&
		isDigit(tz[1]) && isDigit(tz[2]) && isDigit(tz[4]) && isDigit(tz[5]):
		return true
	}
	return false
}
