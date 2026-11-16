// Package xmltree is a small read-only XML tree used to parse and validate
// CII invoices.
//
// Elements are matched by local name: the prefix used in the document does not
// matter, and paths given to Find, FindAll and Value may carry a prefix
// ("ram:ID") or not ("ID"). The namespace URI of each element is still
// resolved, so callers can check it when it matters (the document root).
package xmltree

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Node is an XML element.
type Node struct {
	Name     string  // local name
	Space    string  // namespace URI, resolved from the xmlns declarations in scope
	Prefix   string  // prefix used in the document ("" when none)
	Attrs    []Attr  // attributes, in document order (namespace declarations excluded)
	Text     string  // character data directly inside the element, untrimmed
	Children []*Node // child elements, in document order
	Parent   *Node
	Line     int // line of the start tag, 1-based
}

// Attr is an XML attribute.
type Attr struct {
	Name   string // local name
	Prefix string
	Value  string
}

// Parse parses an XML document and returns its root element.
//
// DOCTYPE declarations are ignored and no entity other than the five
// predefined ones is expanded. Documents declared in ISO-8859-1 or
// windows-1252 are converted to UTF-8; other non-UTF-8 encodings are refused.
func Parse(data []byte) (*Node, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = charsetReader

	type scope struct {
		node *Node
		ns   map[string]string // declarations made on this element
	}
	var (
		stack []scope
		root  *Node
	)
	lookup := func(prefix string) string {
		for i := len(stack) - 1; i >= 0; i-- {
			if uri, ok := stack[i].ns[prefix]; ok {
				return uri
			}
		}
		if prefix == "xml" {
			return "http://www.w3.org/XML/1998/namespace"
		}
		return ""
	}
	for {
		line, _ := d.InputPos()
		tok, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &Node{Name: t.Name.Local, Prefix: t.Name.Space, Line: line}
			var ns map[string]string
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					if ns == nil {
						ns = map[string]string{}
					}
					ns[""] = a.Value
				case a.Name.Space == "xmlns":
					if ns == nil {
						ns = map[string]string{}
					}
					ns[a.Name.Local] = a.Value
				default:
					n.Attrs = append(n.Attrs, Attr{Name: a.Name.Local, Prefix: a.Name.Space, Value: a.Value})
				}
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1].node
				n.Parent = p
				p.Children = append(p.Children, n)
			} else if root != nil {
				return nil, errors.New("xml: more than one root element")
			} else {
				root = n
			}
			stack = append(stack, scope{node: n, ns: ns})
			n.Space = lookup(n.Prefix)
			if n.Prefix != "" && n.Space == "" {
				return nil, fmt.Errorf("xml: line %d: undeclared prefix %q", line, n.Prefix)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("xml: line %d: unexpected end tag </%s>", line, qname(t.Name))
			}
			top := stack[len(stack)-1].node
			if top.Name != t.Name.Local || top.Prefix != t.Name.Space {
				return nil, fmt.Errorf("xml: line %d: element <%s> closed by </%s>", line, qname(xml.Name{Space: top.Prefix, Local: top.Name}), qname(t.Name))
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if len(bytes.TrimSpace(t)) > 0 {
					return nil, fmt.Errorf("xml: line %d: text outside the root element", line)
				}
				continue
			}
			top := stack[len(stack)-1].node
			if top.Text == "" {
				top.Text = string(t)
			} else {
				top.Text += string(t)
			}
		}
	}
	if root == nil {
		return nil, errors.New("xml: no root element")
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("xml: unexpected end of document inside <%s>", stack[len(stack)-1].node.Name)
	}
	return root, nil
}

func qname(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return n.Space + ":" + n.Local
}

// Elem returns the first child element with the given local name, or nil.
func (n *Node) Elem(name string) *Node {
	if n == nil {
		return nil
	}
	name = local(name)
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Elems returns the child elements with the given local name.
func (n *Node) Elems(name string) []*Node {
	if n == nil {
		return nil
	}
	name = local(name)
	var out []*Node
	for _, c := range n.Children {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// Find returns the first element matching a relative path of local names
// separated by "/" ("ram:ApplicableHeaderTradeSettlement/ram:InvoiceCurrencyCode"),
// or nil. Every branch is explored: the result is the first match in document
// order. An empty path returns n itself.
func (n *Node) Find(path string) *Node {
	if n == nil {
		return nil
	}
	if path == "" {
		return n
	}
	seg, rest, _ := strings.Cut(path, "/")
	seg = local(seg)
	for _, c := range n.Children {
		if c.Name != seg {
			continue
		}
		if rest == "" {
			return c
		}
		if m := c.Find(rest); m != nil {
			return m
		}
	}
	return nil
}

// FindAll returns every element matching a relative path, in document order.
func (n *Node) FindAll(path string) []*Node {
	if n == nil {
		return nil
	}
	if path == "" {
		return []*Node{n}
	}
	var out []*Node
	n.findAll(path, &out)
	return out
}

func (n *Node) findAll(path string, out *[]*Node) {
	seg, rest, _ := strings.Cut(path, "/")
	seg = local(seg)
	for _, c := range n.Children {
		if c.Name != seg {
			continue
		}
		if rest == "" {
			*out = append(*out, c)
		} else {
			c.findAll(rest, out)
		}
	}
}

// Value returns the trimmed text of the first element matching path, or "".
func (n *Node) Value(path string) string {
	return n.Find(path).Trim()
}

// Exists reports whether an element matches path.
func (n *Node) Exists(path string) bool { return n.Find(path) != nil }

// Count returns the number of elements matching path.
func (n *Node) Count(path string) int { return len(n.FindAll(path)) }

// Trim returns the text of n with leading and trailing white space removed,
// or "" when n is nil.
func (n *Node) Trim() string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Text)
}

// Normalized returns the text of n as XPath normalize-space() would: trimmed,
// with every run of white space replaced by a single space.
func (n *Node) Normalized() string {
	if n == nil {
		return ""
	}
	return strings.Join(strings.Fields(n.Text), " ")
}

// Attr returns the value of the attribute with the given local name, or "".
func (n *Node) Attr(name string) string {
	v, _ := n.LookupAttr(name)
	return v
}

// LookupAttr returns the value of an attribute and whether it is present.
func (n *Node) LookupAttr(name string) (string, bool) {
	if n == nil {
		return "", false
	}
	name = local(name)
	for _, a := range n.Attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// Walk calls fn for n and each descendant, depth first in document order.
// When fn returns false the children of that node are skipped.
func (n *Node) Walk(fn func(*Node) bool) {
	if n == nil || !fn(n) {
		return
	}
	for _, c := range n.Children {
		c.Walk(fn)
	}
}

// Path returns an XPath-like location of n, such as
// "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:IncludedSupplyChainTradeLineItem[2]/ram:AssociatedDocumentLineDocument".
// Prefixes are the conventional CII ones (rsm, ram, udt, qdt) when the
// namespace is known, the document prefix otherwise. A position is added when
// the parent has several children with the same name.
func (n *Node) Path() string {
	if n == nil {
		return ""
	}
	var segs []string
	for c := n; c != nil; c = c.Parent {
		s := c.qualified()
		if p := c.Parent; p != nil {
			idx, total := 0, 0
			for _, sib := range p.Children {
				if sib.Name == c.Name {
					total++
					if sib == c {
						idx = total
					}
				}
			}
			if total > 1 {
				s += "[" + strconv.Itoa(idx) + "]"
			}
		}
		segs = append(segs, s)
	}
	var b strings.Builder
	for i := len(segs) - 1; i >= 0; i-- {
		b.WriteByte('/')
		b.WriteString(segs[i])
	}
	return b.String()
}

// Conventional prefixes of the CII namespaces.
var prefixes = map[string]string{
	"urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100":                       "rsm",
	"urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100": "ram",
	"urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100":                        "udt",
	"urn:un:unece:uncefact:data:standard:QualifiedDataType:100":                          "qdt",
}

func (n *Node) qualified() string {
	if p, ok := prefixes[n.Space]; ok {
		return p + ":" + n.Name
	}
	if n.Prefix != "" {
		return n.Prefix + ":" + n.Name
	}
	return n.Name
}

func local(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// charsetReader converts single-byte Latin encodings to UTF-8.
func charsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1", "l1", "windows-1252", "cp1252":
		data, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		cp1252 := strings.HasSuffix(strings.ToLower(label), "1252")
		out := make([]byte, 0, len(data)+len(data)/8)
		for _, b := range data {
			r := rune(b)
			if cp1252 && b >= 0x80 && b < 0xa0 {
				if m := win1252[b-0x80]; m != 0 {
					r = m
				}
			}
			out = utf8.AppendRune(out, r)
		}
		return bytes.NewReader(out), nil
	}
	return nil, fmt.Errorf("unsupported XML encoding %q", label)
}

// win1252 maps bytes 0x80–0x9F of windows-1252 to Unicode (0 = undefined).
var win1252 = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}
