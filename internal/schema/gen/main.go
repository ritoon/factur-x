// Command gen compiles the XML schemas of the Factur-X profiles into the
// compact Go tables used by package schema.
//
// Usage:
//
//	go run ./gen [-o tables_gen.go] <dir>
//
// dir is searched recursively for the main schema of each profile: a file
// whose name ends in _MINIMUM.xsd, _BASICWL.xsd (or _BASIC-WL.xsd),
// _BASIC.xsd, _EN16931.xsd or _EXTENDED.xsd, such as
// "Factur-X_1.09.2_EN16931.xsd" in the official Factur-X package. The schemas
// it imports are read from the paths given by their schemaLocation.
//
// Only the subset of XML Schema used by the Factur-X schemas is supported:
// named and anonymous complex types holding one sequence or one choice of
// element particles, simple content extensions with attributes, and simple
// types restricting a built-in type with enumeration, pattern and length
// facets. Anything else stops the generator with an error, so that a new
// version of the schemas never compiles silently into wrong tables.
//
// Complex types that are structurally identical in several profiles are
// emitted once and shared.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const nsXS = "http://www.w3.org/2001/XMLSchema"

// profiles, in the order of the generated tables (and of the profile bit
// masks). The token is the file name suffix that identifies the main schema.
var profiles = []struct {
	name   string // display name
	tokens []string
}{
	{"MINIMUM", []string{"MINIMUM"}},
	{"BASIC WL", []string{"BASICWL", "BASIC-WL", "BASIC_WL"}},
	{"BASIC", []string{"BASIC"}},
	{"EN 16931", []string{"EN16931"}},
	{"EXTENDED", []string{"EXTENDED"}},
}

// Conventional prefixes of the CII namespaces, used in display names.
var prefixes = map[string]string{
	"urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100":                       "rsm",
	"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100": "ram",
	"urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100":                        "udt",
	"urn:un:unece:uncefact:data:standard:QualifiedDataType:100":                          "qdt",
}

func main() {
	out := flag.String("o", "tables_gen.go", "output file")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gen [-o tables_gen.go] <directory of the Factur-X XSD files>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 || flag.Arg(0) == "" {
		flag.Usage()
		os.Exit(2)
	}
	src, err := generate(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

// generate compiles the schemas found in dir and returns the Go source.
func generate(dir string) ([]byte, error) {
	mains, version, err := findMains(dir)
	if err != nil {
		return nil, err
	}
	c := newCompiler()
	for i, path := range mains {
		if err := c.compileProfile(i, path); err != nil {
			return nil, fmt.Errorf("%s: %w", profiles[i].name, err)
		}
	}
	return c.emit(version)
}

// findMains returns the main schema of each profile.
func findMains(dir string) ([]string, string, error) {
	mains := make([]string, len(profiles))
	var version string
	verRE := regexp.MustCompile(`(\d+\.\d+(?:\.\d+)*)`)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".xsd") {
			return nil
		}
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		upper := strings.ToUpper(base)
		for i, p := range profiles {
			for _, tok := range p.tokens {
				if !strings.HasSuffix(upper, "_"+tok) {
					continue
				}
				if mains[i] != "" && mains[i] != path {
					return fmt.Errorf("two main schemas for %s: %s and %s", p.name, mains[i], path)
				}
				mains[i] = path
				if v := verRE.FindString(base); v != "" && newerVersion(v, version) {
					version = v
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	for i, m := range mains {
		if m == "" {
			return nil, "", fmt.Errorf("no main schema for profile %s in %s", profiles[i].name, dir)
		}
	}
	return mains, version, nil
}

// newerVersion reports whether the dotted version v is greater than cur
// ("" being lower than any version). The tables take the highest version
// of the main schemas: a corrigendum such as Factur-X 1.09.2 only changes
// some profiles, and the schemas of the others may keep the file names of
// the release they come from.
func newerVersion(v, cur string) bool {
	if cur == "" {
		return true
	}
	a, b := strings.Split(v, "."), strings.Split(cur, ".")
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x, _ = strconv.Atoi(a[i])
		}
		if i < len(b) {
			y, _ = strconv.Atoi(b[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// xnode is a generic XML element of a schema document.
type xnode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []*xnode   `xml:",any"`
}

func (n *xnode) attr(name string) string {
	for _, a := range n.Attrs {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// is reports whether n is the XML Schema element local.
func (n *xnode) is(local string) bool {
	return n.XMLName.Space == nsXS && n.XMLName.Local == local
}

// schemaDoc is one parsed schema file.
type schemaDoc struct {
	path              string
	tns               string
	ns                map[string]string // prefixes declared on xs:schema
	qualifiedElements bool
}

type qname struct{ space, local string }

func (q qname) String() string {
	if p, ok := prefixes[q.space]; ok {
		return p + ":" + q.local
	}
	if q.space == nsXS {
		return "xs:" + q.local
	}
	return "{" + q.space + "}" + q.local
}

// decl is a named schema component with the document that defines it.
type decl struct {
	doc  *schemaDoc
	node *xnode
}

// profileSchemas holds the components of one profile.
type profileSchemas struct {
	docs     map[string]*schemaDoc
	complex  map[qname]decl
	simple   map[qname]decl
	elements map[qname]decl
	files    []string
}

func (ps *profileSchemas) load(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, ok := ps.docs[abs]; ok {
		return nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	var root xnode
	if err := xml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if !root.is("schema") {
		return fmt.Errorf("%s: root element is not xs:schema", path)
	}
	doc := &schemaDoc{path: abs, tns: root.attr("targetNamespace"), ns: map[string]string{}}
	doc.qualifiedElements = root.attr("elementFormDefault") == "qualified"
	if f := root.attr("attributeFormDefault"); f == "qualified" {
		return fmt.Errorf("%s: attributeFormDefault=qualified is not supported", path)
	}
	for _, a := range root.Attrs {
		switch {
		case a.Name.Space == "xmlns":
			doc.ns[a.Name.Local] = a.Value
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			doc.ns[""] = a.Value
		}
	}
	ps.docs[abs] = doc
	ps.files = append(ps.files, abs)
	for _, c := range root.Children {
		name := qname{doc.tns, c.attr("name")}
		switch {
		case c.is("import"), c.is("include"):
			loc := c.attr("schemaLocation")
			if loc == "" {
				return fmt.Errorf("%s: import of %s without schemaLocation", path, c.attr("namespace"))
			}
			if err := ps.load(filepath.Join(filepath.Dir(abs), loc)); err != nil {
				return err
			}
		case c.is("complexType"):
			ps.complex[name] = decl{doc, c}
		case c.is("simpleType"):
			ps.simple[name] = decl{doc, c}
		case c.is("element"):
			ps.elements[name] = decl{doc, c}
		case c.is("annotation"):
		default:
			return fmt.Errorf("%s: unsupported top-level xs:%s", path, c.XMLName.Local)
		}
	}
	return nil
}

// resolve turns a QName written in doc into a namespace and local name.
func (doc *schemaDoc) resolve(s string) (qname, error) {
	prefix, local, ok := strings.Cut(s, ":")
	if !ok {
		prefix, local = "", s
	}
	uri, found := doc.ns[prefix]
	if !found {
		if prefix == "" {
			return qname{"", local}, nil
		}
		return qname{}, fmt.Errorf("%s: undeclared prefix in %q", doc.path, s)
	}
	return qname{uri, local}, nil
}

// Simple type kinds, as named in package schema.
const (
	kString  = "kString"
	kDecimal = "kDecimal"
	kInteger = "kInteger"
	kBoolean = "kBoolean"
	kBase64  = "kBase64"
	kDate    = "kDate"
)

// builtins maps the supported built-in types to their kind. The string-like
// types accept any value (their white space facet does not restrict the
// lexical space).
var builtins = map[string]string{
	"string": kString, "normalizedString": kString, "token": kString, "anyURI": kString,
	"NMTOKEN": kString, "Name": kString, "NCName": kString, "language": kString, "ID": kString,
	"decimal": kDecimal,
	"integer": kInteger, "long": kInteger, "int": kInteger, "short": kInteger,
	"nonNegativeInteger": kInteger, "positiveInteger": kInteger,
	"boolean":      kBoolean,
	"base64Binary": kBase64,
	"date":         kDate,
}

// collapsing reports whether white space is collapsed for the built-in type
// (it matters for enumeration and length facets).
var collapsing = map[string]bool{"token": true, "NMTOKEN": true, "Name": true, "NCName": true, "language": true, "ID": true, "anyURI": true}

// facets of a simple type.
type gFacets struct {
	enum     []string
	pattern  []string
	min, max int // lengths; -1 = none
	collapse bool
}

func (f *gFacets) empty() bool {
	return f == nil || (len(f.enum) == 0 && len(f.pattern) == 0 && f.min < 0 && f.max < 0)
}

func (f *gFacets) key() string {
	if f.empty() {
		return ""
	}
	return fmt.Sprintf("%q/%q/%d/%d/%t", f.enum, f.pattern, f.min, f.max, f.collapse)
}

// gSimple is a resolved simple type.
type gSimple struct {
	name   string // display name of the type, for messages
	kind   string
	facets *gFacets
}

// Content kinds, as named in package schema.
const (
	contentEmpty    = "contentEmpty"
	contentElements = "contentElements"
	contentSimple   = "contentSimple"
)

type gPart struct {
	name     string
	ns       string
	typ      *gType
	min, max int // max -1 = unbounded
}

type gAttr struct {
	name     string
	simple   gSimple
	required bool
}

type gType struct {
	name     string // display name
	content  string
	choice   bool
	min, max int // occurrences of the choice
	simple   gSimple
	parts    []gPart
	attrs    []gAttr
	profiles uint8
	id       int
	key      string
}

type compiler struct {
	byKey      map[string]*gType
	types      []*gType
	roots      []gPart
	files      [][]string // per profile
	namespaces []string
	inProgress map[string]bool
}

func newCompiler() *compiler {
	return &compiler{byKey: map[string]*gType{}, inProgress: map[string]bool{}}
}

func (c *compiler) compileProfile(p int, path string) error {
	ps := &profileSchemas{docs: map[string]*schemaDoc{}, complex: map[qname]decl{}, simple: map[qname]decl{}, elements: map[qname]decl{}}
	if err := ps.load(path); err != nil {
		return err
	}
	c.files = append(c.files, ps.files)
	pc := &profileCompiler{c: c, ps: ps, p: p, memo: map[qname]*gType{}}
	root, ok := ps.elements[qname{"urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100", "CrossIndustryInvoice"}]
	if !ok {
		return errors.New("no global element rsm:CrossIndustryInvoice")
	}
	part, err := pc.element(root.doc, root.node, true)
	if err != nil {
		return err
	}
	c.roots = append(c.roots, part)
	return nil
}

type profileCompiler struct {
	c    *compiler
	ps   *profileSchemas
	p    int
	memo map[qname]*gType
}

// intern returns the shared copy of t, registering it when it is new.
func (pc *profileCompiler) intern(t *gType) *gType {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%t|%d|%d|%s|%s|%s", t.name, t.content, t.choice, t.min, t.max, t.simple.kind, t.simple.name, t.simple.facets.key())
	for _, a := range t.attrs {
		fmt.Fprintf(&b, "|@%s:%s:%s:%s:%t", a.name, a.simple.kind, a.simple.name, a.simple.facets.key(), a.required)
	}
	for _, p := range t.parts {
		fmt.Fprintf(&b, "|%s{%s}#%d:%d:%d", p.name, p.ns, p.typ.id, p.min, p.max)
	}
	t.key = b.String()
	if s, ok := pc.c.byKey[t.key]; ok {
		s.profiles |= 1 << pc.p
		return s
	}
	t.profiles = 1 << pc.p
	t.id = len(pc.c.types)
	pc.c.types = append(pc.c.types, t)
	pc.c.byKey[t.key] = t
	return t
}

func (pc *profileCompiler) addNamespace(ns string) {
	for _, n := range pc.c.namespaces {
		if n == ns {
			return
		}
	}
	pc.c.namespaces = append(pc.c.namespaces, ns)
}

// occurs reads minOccurs and maxOccurs.
func occurs(n *xnode) (min, max int, err error) {
	min, max = 1, 1
	if s := n.attr("minOccurs"); s != "" {
		if min, err = strconv.Atoi(s); err != nil || min < 0 || min > 254 {
			return 0, 0, fmt.Errorf("unsupported minOccurs %q", s)
		}
	}
	if s := n.attr("maxOccurs"); s != "" {
		if s == "unbounded" {
			max = -1
		} else if max, err = strconv.Atoi(s); err != nil || max < 1 || max > 254 {
			return 0, 0, fmt.Errorf("unsupported maxOccurs %q", s)
		}
	}
	if max >= 0 && max < min {
		return 0, 0, fmt.Errorf("maxOccurs %d < minOccurs %d", max, min)
	}
	return min, max, nil
}

// element compiles an element declaration (global or local) into a particle.
func (pc *profileCompiler) element(doc *schemaDoc, n *xnode, global bool) (gPart, error) {
	if ref := n.attr("ref"); ref != "" {
		q, err := doc.resolve(ref)
		if err != nil {
			return gPart{}, err
		}
		d, ok := pc.ps.elements[q]
		if !ok {
			return gPart{}, fmt.Errorf("unknown element %s", q)
		}
		part, err := pc.element(d.doc, d.node, true)
		if err != nil {
			return gPart{}, err
		}
		part.min, part.max, err = occurs(n)
		return part, err
	}
	name := n.attr("name")
	if name == "" {
		return gPart{}, fmt.Errorf("%s: element without name", doc.path)
	}
	ns := ""
	if global || doc.qualifiedElements || n.attr("form") == "qualified" {
		ns = doc.tns
	}
	if n.attr("form") == "unqualified" {
		ns = ""
	}
	if ns != "" {
		pc.addNamespace(ns)
	}
	part := gPart{name: name, ns: ns}
	var err error
	part.min, part.max, err = occurs(n)
	if err != nil {
		return gPart{}, fmt.Errorf("element %s: %w", name, err)
	}
	display := qname{ns, name}.String()
	if typ := n.attr("type"); typ != "" {
		q, err := doc.resolve(typ)
		if err != nil {
			return gPart{}, err
		}
		part.typ, err = pc.typeByName(q)
		if err != nil {
			return gPart{}, fmt.Errorf("element %s: %w", display, err)
		}
		return part, nil
	}
	for _, c := range n.Children {
		switch {
		case c.is("complexType"):
			part.typ, err = pc.complexType(doc, c, "type of "+display)
		case c.is("simpleType"):
			var s gSimple
			s, err = pc.simpleType(doc, c, "type of "+display)
			if err == nil {
				part.typ = pc.intern(&gType{name: "type of " + display, content: contentSimple, simple: s})
			}
		case c.is("annotation"):
			continue
		default:
			err = fmt.Errorf("unsupported xs:%s", c.XMLName.Local)
		}
		if err != nil {
			return gPart{}, fmt.Errorf("element %s: %w", display, err)
		}
	}
	if part.typ == nil {
		// xs:anyType is not supported: every Factur-X element has a type.
		return gPart{}, fmt.Errorf("element %s has no type", display)
	}
	return part, nil
}

// typeByName compiles the named type q (complex, simple or built-in).
func (pc *profileCompiler) typeByName(q qname) (*gType, error) {
	if t, ok := pc.memo[q]; ok {
		return t, nil
	}
	key := fmt.Sprint(pc.p, q)
	if pc.c.inProgress[key] {
		return nil, fmt.Errorf("recursive type %s is not supported", q)
	}
	pc.c.inProgress[key] = true
	defer delete(pc.c.inProgress, key)
	var (
		t   *gType
		err error
	)
	if d, ok := pc.ps.complex[q]; ok {
		t, err = pc.complexType(d.doc, d.node, q.String())
	} else {
		var s gSimple
		s, err = pc.simpleByName(q)
		if err == nil {
			t = pc.intern(&gType{name: q.String(), content: contentSimple, simple: s})
		}
	}
	if err != nil {
		return nil, err
	}
	pc.memo[q] = t
	return t, nil
}

// complexType compiles a complex type definition.
func (pc *profileCompiler) complexType(doc *schemaDoc, n *xnode, name string) (*gType, error) {
	if n.attr("mixed") == "true" {
		return nil, fmt.Errorf("%s: mixed content is not supported", name)
	}
	t := &gType{name: name, content: contentEmpty, min: 1, max: 1}
	for _, c := range n.Children {
		switch {
		case c.is("annotation"):
		case c.is("sequence"), c.is("choice"):
			if t.content != contentEmpty || len(t.parts) > 0 {
				return nil, fmt.Errorf("%s: several model groups", name)
			}
			if err := pc.group(doc, c, t); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		case c.is("simpleContent"):
			if err := pc.simpleContent(doc, c, t); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		case c.is("attribute"):
			a, err := pc.attribute(doc, c)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if a != nil {
				t.attrs = append(t.attrs, *a)
			}
		default:
			return nil, fmt.Errorf("%s: unsupported xs:%s", name, c.XMLName.Local)
		}
	}
	return pc.intern(t), nil
}

// group compiles the top-level sequence or choice of a complex type.
func (pc *profileCompiler) group(doc *schemaDoc, g *xnode, t *gType) error {
	min, max, err := occurs(g)
	if err != nil {
		return err
	}
	t.choice = g.is("choice")
	if !t.choice && (min != 1 || max != 1) {
		return errors.New("repeated or optional sequences are not supported")
	}
	t.min, t.max = min, max
	seen := map[string]bool{}
	for _, c := range g.Children {
		switch {
		case c.is("annotation"):
		case c.is("element"):
			part, err := pc.element(doc, c, false)
			if err != nil {
				return err
			}
			if seen[part.name] {
				// Children are matched by local name at run time.
				return fmt.Errorf("element name %s used twice in one content model", part.name)
			}
			seen[part.name] = true
			t.parts = append(t.parts, part)
		default:
			return fmt.Errorf("nested xs:%s is not supported", c.XMLName.Local)
		}
	}
	if len(t.parts) > 0 {
		t.content = contentElements
	}
	return nil
}

// simpleContent compiles an xs:simpleContent extension.
func (pc *profileCompiler) simpleContent(doc *schemaDoc, n *xnode, t *gType) error {
	t.content = contentSimple
	for _, c := range n.Children {
		switch {
		case c.is("annotation"):
		case c.is("extension"):
			base, err := doc.resolve(c.attr("base"))
			if err != nil {
				return err
			}
			if _, ok := pc.ps.complex[base]; ok {
				bt, err := pc.typeByName(base)
				if err != nil {
					return err
				}
				if bt.content != contentSimple {
					return fmt.Errorf("simple content extension of %s, which has no simple content", base)
				}
				t.simple = bt.simple
				t.attrs = append(t.attrs, bt.attrs...)
			} else {
				s, err := pc.simpleByName(base)
				if err != nil {
					return err
				}
				t.simple = s
			}
			for _, a := range c.Children {
				switch {
				case a.is("annotation"):
				case a.is("attribute"):
					ga, err := pc.attribute(doc, a)
					if err != nil {
						return err
					}
					if ga != nil {
						t.attrs = append(t.attrs, *ga)
					}
				default:
					return fmt.Errorf("unsupported xs:%s in extension", a.XMLName.Local)
				}
			}
		default:
			return fmt.Errorf("unsupported simple content xs:%s", c.XMLName.Local)
		}
	}
	return nil
}

// attribute compiles an attribute declaration; prohibited attributes give nil.
func (pc *profileCompiler) attribute(doc *schemaDoc, n *xnode) (*gAttr, error) {
	if n.attr("ref") != "" {
		return nil, errors.New("attribute references are not supported")
	}
	name := n.attr("name")
	a := &gAttr{name: name, simple: gSimple{name: "xs:anySimpleType", kind: kString}}
	switch n.attr("use") {
	case "required":
		a.required = true
	case "prohibited":
		return nil, nil
	case "", "optional":
	default:
		return nil, fmt.Errorf("attribute %s: unsupported use %q", name, n.attr("use"))
	}
	if typ := n.attr("type"); typ != "" {
		q, err := doc.resolve(typ)
		if err != nil {
			return nil, err
		}
		if a.simple, err = pc.simpleByName(q); err != nil {
			return nil, fmt.Errorf("attribute %s: %w", name, err)
		}
	}
	for _, c := range n.Children {
		switch {
		case c.is("annotation"):
		case c.is("simpleType"):
			s, err := pc.simpleType(doc, c, "type of @"+name)
			if err != nil {
				return nil, fmt.Errorf("attribute %s: %w", name, err)
			}
			a.simple = s
		default:
			return nil, fmt.Errorf("attribute %s: unsupported xs:%s", name, c.XMLName.Local)
		}
	}
	return a, nil
}

// simpleByName resolves a named simple type or a built-in type.
func (pc *profileCompiler) simpleByName(q qname) (gSimple, error) {
	if q.space == nsXS {
		kind, ok := builtins[q.local]
		if !ok {
			return gSimple{}, fmt.Errorf("unsupported built-in type xs:%s", q.local)
		}
		return gSimple{name: "xs:" + q.local, kind: kind, facets: &gFacets{min: -1, max: -1, collapse: collapsing[q.local]}}, nil
	}
	d, ok := pc.ps.simple[q]
	if !ok {
		if _, isComplex := pc.ps.complex[q]; isComplex {
			return gSimple{}, fmt.Errorf("%s is a complex type, a simple type is expected", q)
		}
		return gSimple{}, fmt.Errorf("unknown type %s", q)
	}
	return pc.simpleType(d.doc, d.node, q.String())
}

// simpleType compiles a simple type definition (a restriction of a built-in
// or named simple type).
func (pc *profileCompiler) simpleType(doc *schemaDoc, n *xnode, name string) (gSimple, error) {
	for _, c := range n.Children {
		switch {
		case c.is("annotation"):
		case c.is("restriction"):
			base, err := doc.resolve(c.attr("base"))
			if err != nil {
				return gSimple{}, err
			}
			bs, err := pc.simpleByName(base)
			if err != nil {
				return gSimple{}, err
			}
			f := &gFacets{min: -1, max: -1}
			if bs.facets != nil {
				g := *bs.facets
				g.enum = append([]string(nil), g.enum...)
				g.pattern = append([]string(nil), g.pattern...)
				f = &g
			}
			var enum []string
			for _, fc := range c.Children {
				v := fc.attr("value")
				switch {
				case fc.is("annotation"):
				case fc.is("enumeration"):
					enum = append(enum, v)
				case fc.is("pattern"):
					if strings.Contains(v, `\i`) || strings.Contains(v, `\c`) || strings.Contains(v, `\I`) || strings.Contains(v, `\C`) {
						return gSimple{}, fmt.Errorf("%s: pattern %q uses XML Schema name escapes", name, v)
					}
					if _, err := regexp.Compile("^(?:" + v + ")$"); err != nil {
						return gSimple{}, fmt.Errorf("%s: pattern %q: %w", name, v, err)
					}
					f.pattern = append(f.pattern, v)
				case fc.is("length"), fc.is("minLength"), fc.is("maxLength"):
					l, err := strconv.Atoi(v)
					if err != nil || l < 0 {
						return gSimple{}, fmt.Errorf("%s: bad %s %q", name, fc.XMLName.Local, v)
					}
					if !fc.is("maxLength") {
						f.min = l
					}
					if !fc.is("minLength") {
						f.max = l
					}
				case fc.is("whiteSpace"):
					f.collapse = v == "collapse"
				default:
					return gSimple{}, fmt.Errorf("%s: unsupported facet xs:%s", name, fc.XMLName.Local)
				}
			}
			if len(enum) > 0 {
				f.enum = enum
			}
			return gSimple{name: name, kind: bs.kind, facets: f}, nil
		default:
			return gSimple{}, fmt.Errorf("%s: unsupported xs:%s (only restrictions are supported)", name, c.XMLName.Local)
		}
	}
	return gSimple{}, fmt.Errorf("%s: simple type without restriction", name)
}

// emit writes the Go source of the tables.
func (c *compiler) emit(version string) ([]byte, error) {
	// Stable order: by name, then by the lowest profile using the type.
	order := append([]*gType(nil), c.types...)
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].name != order[j].name {
			return order[i].name < order[j].name
		}
		return lowBit(order[i].profiles) < lowBit(order[j].profiles)
	})
	newID := make(map[*gType]int, len(order))
	for i, t := range order {
		newID[t] = i
	}
	sort.Strings(c.namespaces)
	nsIndex := map[string]int{"": 0}
	for i, ns := range c.namespaces {
		nsIndex[ns] = i + 1
	}

	// Facets, deduplicated; index 0 means none.
	var facetList []*gFacets
	facetIndex := map[string]int{"": 0}
	facetOf := func(f *gFacets) int {
		k := f.key()
		if i, ok := facetIndex[k]; ok {
			return i
		}
		facetList = append(facetList, f)
		facetIndex[k] = len(facetList)
		return len(facetList)
	}

	var (
		b         bytes.Buffer
		parts     bytes.Buffer
		attrs     bytes.Buffer
		nParts    int
		nAttrs    int
		maxOccurs = func(m int) string {
			if m < 0 {
				return "unbounded"
			}
			return strconv.Itoa(m)
		}
	)
	fmt.Fprintf(&b, "// Code generated by internal/schema/gen from the Factur-X %s XML schemas; DO NOT EDIT.\n\n", version)
	b.WriteString("package schema\n\n")
	b.WriteString("// Source schemas (SHA-256):\n//\n")
	for p, files := range c.files {
		fmt.Fprintf(&b, "//\t%s:\n", profiles[p].name)
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(data)
			fmt.Fprintf(&b, "//\t\t%s %s\n", hex.EncodeToString(sum[:8]), filepath.Base(f))
		}
	}
	fmt.Fprintf(&b, "\n// schemaVersion is the version of the Factur-X schemas compiled into the tables.\nconst schemaVersion = %q\n\n", version)
	b.WriteString("// namespaces lists the element namespaces; particle.ns indexes it (0 = no namespace).\n")
	b.WriteString("var namespaces = [...]string{\"\",\n")
	for _, ns := range c.namespaces {
		fmt.Fprintf(&b, "%q,\n", ns)
	}
	b.WriteString("}\n\n")

	b.WriteString("// types holds the content model of every element type, shared between\n// profiles when identical; profiles is a bit mask of the profiles using it.\n")
	b.WriteString("var types = [...]ctype{\n")
	for i, t := range order {
		firstPart, firstAttr := nParts, nAttrs
		for _, p := range t.parts {
			fmt.Fprintf(&parts, "{%q, %d, %d, %d, %s}, // %d\n", p.name, nsIndex[p.ns], newID[p.typ], p.min, maxOccurs(p.max), nParts)
			nParts++
		}
		for _, a := range t.attrs {
			fmt.Fprintf(&attrs, "{%q, %s, %d, %t},\n", a.name, a.simple.kind, facetOf(a.simple.facets), a.required)
			nAttrs++
		}
		fmt.Fprintf(&b, "{name: %q, content: %s", t.name, t.content)
		if t.choice {
			fmt.Fprintf(&b, ", choice: true, min: %d, max: %s", t.min, maxOccurs(t.max))
		}
		if t.content == contentSimple {
			fmt.Fprintf(&b, ", simple: %s", t.simple.kind)
			if fi := facetOf(t.simple.facets); fi != 0 {
				fmt.Fprintf(&b, ", facets: %d", fi)
			}
			if t.simple.kind != kString {
				fmt.Fprintf(&b, ", simpleName: %q", t.simple.name)
			}
		}
		if len(t.parts) > 0 {
			fmt.Fprintf(&b, ", first: %d, n: %d", firstPart, len(t.parts))
		}
		if len(t.attrs) > 0 {
			fmt.Fprintf(&b, ", afirst: %d, an: %d", firstAttr, len(t.attrs))
		}
		fmt.Fprintf(&b, ", profiles: %#02x}, // %d\n", t.profiles, i)
	}
	b.WriteString("}\n\n")
	b.WriteString("// particles lists the child elements of the types: name, namespace, type, minOccurs, maxOccurs.\n")
	b.WriteString("var particles = [...]particle{\n")
	b.Write(parts.Bytes())
	b.WriteString("}\n\n")
	b.WriteString("// attributes lists the attributes of the types: name, kind, facets, required.\n")
	b.WriteString("var attributes = [...]attribute{\n")
	b.Write(attrs.Bytes())
	b.WriteString("}\n\n")
	b.WriteString("// facetTable lists the facets of the restricted simple types; index 0 is none.\n")
	b.WriteString("var facetTable = [...]facets{{},\n")
	for _, f := range facetList {
		b.WriteString("{")
		if len(f.enum) > 0 {
			fmt.Fprintf(&b, "enum: %#v, ", f.enum)
		}
		if len(f.pattern) > 0 {
			fmt.Fprintf(&b, "patterns: %#v, ", f.pattern)
		}
		fmt.Fprintf(&b, "minLen: %d, maxLen: %d, collapse: %t},\n", f.min, f.max, f.collapse)
	}
	b.WriteString("}\n\n")
	b.WriteString("// roots holds the root element of each profile, in profile order.\n")
	b.WriteString("var roots = [numProfiles]particle{\n")
	for p, r := range c.roots {
		fmt.Fprintf(&b, "{%q, %d, %d, 1, 1}, // %s\n", r.name, nsIndex[r.ns], newID[r.typ], profiles[p].name)
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting generated code: %w\n%s", err, b.Bytes())
	}
	return src, nil
}

func lowBit(m uint8) int {
	for i := 0; i < 8; i++ {
		if m&(1<<i) != 0 {
			return i
		}
	}
	return 8
}
