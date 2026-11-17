package cii

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// This file holds a small structural XSD checker used by the tests: it loads
// the Factur-X schemas of a profile (directory given by the FX_XSD
// environment variable, with one sub-directory per profile: MINIMUM, BASICWL,
// BASIC, EN16931, EXTENDED) and checks that a document follows them: element
// order, cardinalities, allowed and required attributes, no children in
// simple content. Facets of simple types are not checked.

var xsdPrefixes = map[string]string{
	"rsm": NamespaceRSM,
	"ram": NamespaceRAM,
	"udt": NamespaceUDT,
	"qdt": NamespaceQDT,
}

type xsdType struct {
	simple bool
	choice bool
	elems  []xsdElem
	attrs  map[string]bool // attribute name → required
}

type xsdElem struct {
	name, ns string
	typ      string // "ns|name" key of a named type, "" for inline or builtin
	inline   *xsdType
	min, max int // max < 0: unbounded
}

type xsdSchema struct {
	types map[string]*xsdType
}

func xsdDirFor(p invoice.Profile) string {
	switch p {
	case invoice.ProfileMinimum:
		return "MINIMUM"
	case invoice.ProfileBasicWL:
		return "BASICWL"
	case invoice.ProfileBasic:
		return "BASIC"
	case invoice.ProfileExtended, invoice.ProfileExtendedCTCFR:
		return "EXTENDED"
	}
	return "EN16931"
}

var (
	xsdMu    sync.Mutex
	xsdCache = map[string]*xsdSchema{}
)

// loadXSD returns the schema of a profile, or nil when FX_XSD is not set.
func loadXSD(t testing.TB, p invoice.Profile) *xsdSchema {
	t.Helper()
	root := os.Getenv("FX_XSD")
	if root == "" {
		return nil
	}
	dir := filepath.Join(root, xsdDirFor(p))
	xsdMu.Lock()
	defer xsdMu.Unlock()
	if s, ok := xsdCache[dir]; ok {
		return s
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.xsd"))
	if len(files) == 0 {
		t.Fatalf("no XSD in %s", dir)
	}
	s := &xsdSchema{types: map[string]*xsdType{}}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := xmltree.Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		tns := doc.Attr("targetNamespace")
		for _, c := range doc.Children {
			switch c.Name {
			case "complexType":
				s.types[tns+"|"+c.Attr("name")] = parseComplexType(c, tns)
			case "simpleType":
				s.types[tns+"|"+c.Attr("name")] = &xsdType{simple: true}
			}
		}
	}
	xsdCache[dir] = s
	return s
}

func typeKey(qname string) string {
	prefix, local, ok := strings.Cut(qname, ":")
	if !ok {
		return qname
	}
	if prefix == "xs" {
		return "" // builtin simple type
	}
	return xsdPrefixes[prefix] + "|" + local
}

func parseComplexType(n *xmltree.Node, tns string) *xsdType {
	t := &xsdType{attrs: map[string]bool{}}
	for _, c := range n.Children {
		switch c.Name {
		case "sequence", "choice":
			t.choice = c.Name == "choice"
			for _, e := range c.Children {
				if e.Name != "element" {
					continue
				}
				el := xsdElem{name: e.Attr("name"), ns: tns, typ: typeKey(e.Attr("type")), min: 1, max: 1}
				if v, ok := e.LookupAttr("minOccurs"); ok {
					el.min, _ = strconv.Atoi(v)
				}
				if v, ok := e.LookupAttr("maxOccurs"); ok {
					if v == "unbounded" {
						el.max = -1
					} else {
						el.max, _ = strconv.Atoi(v)
					}
				}
				if ct := e.Elem("complexType"); ct != nil {
					el.inline = parseComplexType(ct, tns)
				}
				t.elems = append(t.elems, el)
			}
		case "simpleContent":
			t.simple = true
			for _, ext := range c.Children {
				for _, a := range ext.Children {
					if a.Name == "attribute" {
						t.attrs[a.Attr("name")] = a.Attr("use") == "required"
					}
				}
			}
		case "attribute":
			t.attrs[c.Attr("name")] = c.Attr("use") == "required"
		}
	}
	return t
}

// check validates a document against the schema and returns the violations.
func (s *xsdSchema) check(root *xmltree.Node) []string {
	var errs []string
	if root.Name != "CrossIndustryInvoice" || root.Space != NamespaceRSM {
		return []string{"root is not rsm:CrossIndustryInvoice"}
	}
	s.checkNode(root, s.types[NamespaceRSM+"|CrossIndustryInvoiceType"], "/rsm:CrossIndustryInvoice", &errs)
	return errs
}

func (s *xsdSchema) checkNode(n *xmltree.Node, t *xsdType, path string, errs *[]string) {
	if t == nil || t.simple {
		if len(n.Children) > 0 {
			*errs = append(*errs, fmt.Sprintf("%s: unexpected child <%s> in simple content", path, n.Children[0].Name))
		}
		var attrs map[string]bool
		if t != nil {
			attrs = t.attrs
		}
		for _, a := range n.Attrs {
			if _, ok := attrs[a.Name]; !ok {
				*errs = append(*errs, fmt.Sprintf("%s: attribute %s not allowed", path, a.Name))
			}
		}
		for name, req := range attrs {
			if _, ok := n.LookupAttr(name); req && !ok {
				*errs = append(*errs, fmt.Sprintf("%s: missing attribute %s", path, name))
			}
		}
		return
	}
	for _, a := range n.Attrs {
		if _, ok := t.attrs[a.Name]; !ok {
			*errs = append(*errs, fmt.Sprintf("%s: attribute %s not allowed", path, a.Name))
		}
	}
	kids := n.Children
	if t.choice {
		if len(kids) != 1 {
			*errs = append(*errs, fmt.Sprintf("%s: choice expects exactly one child, got %d", path, len(kids)))
			return
		}
		for _, el := range t.elems {
			if kids[0].Name == el.name && kids[0].Space == el.ns {
				s.checkNode(kids[0], s.elemType(el), path+"/"+el.name, errs)
				return
			}
		}
		*errs = append(*errs, fmt.Sprintf("%s: unexpected child <%s>", path, kids[0].Name))
		return
	}
	i := 0
	for _, el := range t.elems {
		count := 0
		for i < len(kids) && kids[i].Name == el.name && kids[i].Space == el.ns {
			s.checkNode(kids[i], s.elemType(el), path+"/"+el.name, errs)
			count++
			i++
		}
		if count < el.min {
			*errs = append(*errs, fmt.Sprintf("%s: missing <%s>", path, el.name))
		}
		if el.max >= 0 && count > el.max {
			*errs = append(*errs, fmt.Sprintf("%s: <%s> occurs %d times, at most %d", path, el.name, count, el.max))
		}
	}
	for ; i < len(kids); i++ {
		*errs = append(*errs, fmt.Sprintf("%s: unexpected or misplaced <%s>", path, kids[i].Name))
	}
}

func (s *xsdSchema) elemType(el xsdElem) *xsdType {
	if el.inline != nil {
		return el.inline
	}
	if el.typ == "" {
		return nil
	}
	return s.types[el.typ]
}

// checkXSD validates out against the XSD of profile p when FX_XSD is set.
func checkXSD(t testing.TB, p invoice.Profile, out []byte) {
	t.Helper()
	s := loadXSD(t, p)
	if s == nil {
		return
	}
	root, err := xmltree.Parse(out)
	if err != nil {
		t.Fatalf("output is not well-formed: %v", err)
	}
	if errs := s.check(root); len(errs) > 0 {
		if len(errs) > 10 {
			errs = append(errs[:10], fmt.Sprintf("… %d more", len(errs)-10))
		}
		t.Errorf("profile %v: output does not follow the XSD:\n  %s", p, strings.Join(errs, "\n  "))
	}
}

func TestXSDCheckerDetectsDisorder(t *testing.T) {
	s := loadXSD(t, invoice.ProfileEN16931)
	if s == nil {
		t.Skip("FX_XSD not set")
	}
	data, err := os.ReadFile("../testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		t.Fatal(err)
	}
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if errs := s.check(root); len(errs) > 0 {
		t.Fatalf("reference document rejected: %v", errs)
	}
	// Swap two header elements: the checker must notice.
	tx := root.Find("SupplyChainTradeTransaction")
	k := tx.Children
	k[len(k)-1], k[len(k)-2] = k[len(k)-2], k[len(k)-1]
	if errs := s.check(root); len(errs) == 0 {
		t.Fatal("misplaced elements not detected")
	}
	// A larger profile must be rejected by the schema of a smaller one.
	for _, c := range []struct{ out, schema invoice.Profile }{
		{invoice.ProfileExtended, invoice.ProfileEN16931},
		{invoice.ProfileEN16931, invoice.ProfileBasic},
		{invoice.ProfileBasic, invoice.ProfileBasicWL},
		{invoice.ProfileBasicWL, invoice.ProfileMinimum},
	} {
		out, err := Marshal(fullInvoice(), &MarshalOptions{Profile: c.out})
		if err != nil {
			t.Fatal(err)
		}
		root, err := xmltree.Parse(out)
		if err != nil {
			t.Fatal(err)
		}
		if errs := loadXSD(t, c.schema).check(root); len(errs) == 0 {
			t.Errorf("%v output accepted by the %v schema", c.out, c.schema)
		}
	}
}
