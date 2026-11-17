package cii

import (
	"strconv"
	"strings"

	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// This file holds the serialization of the extensions: the elements of a
// document that the model does not carry, kept verbatim (see
// invoice.Extension).

// nsPrefix returns the conventional prefix of a CII namespace, or "".
func nsPrefix(space string) string {
	switch space {
	case NamespaceRSM:
		return "rsm"
	case NamespaceRAM:
		return "ram"
	case NamespaceUDT:
		return "udt"
	case NamespaceQDT:
		return "qdt"
	}
	return ""
}

// appendElement appends n and its content as XML. CII elements get the
// conventional prefixes, declared on the root of the documents written by
// Marshal; other elements keep their prefix and get a namespace declaration.
// Attributes in other namespaces than the element's (xsi:…) are not kept.
// With indent, the element starts on a new line at the given depth and its
// children are indented below it.
func appendElement(b []byte, n *xmltree.Node, indent bool, depth int) []byte {
	if indent {
		b = appendIndent(b, depth)
	}
	name, decl := elementName(n)
	b = append(b, '<')
	b = append(b, name...)
	if decl != "" {
		b = append(b, ' ')
		b = append(b, decl...)
	}
	for _, a := range n.Attrs {
		if a.Prefix != "" && a.Prefix != "xml" {
			continue
		}
		b = append(b, ' ')
		if a.Prefix != "" {
			b = append(b, a.Prefix...)
			b = append(b, ':')
		}
		b = append(b, a.Name...)
		b = append(b, '=', '"')
		b = appendEscaped(b, a.Value, true)
		b = append(b, '"')
	}
	b = append(b, '>')
	if len(n.Children) == 0 {
		b = appendEscaped(b, n.Text, false)
	} else {
		for _, c := range n.Children {
			b = appendElement(b, c, indent, depth+1)
		}
		if indent {
			b = appendIndent(b, depth)
		}
	}
	b = append(b, '<', '/')
	b = append(b, name...)
	return append(b, '>')
}

// elementName returns the qualified name of n and, for an element outside
// the CII namespaces, the declaration of its namespace.
func elementName(n *xmltree.Node) (name, decl string) {
	if p := nsPrefix(n.Space); p != "" {
		return p + ":" + n.Name, ""
	}
	if n.Prefix != "" {
		return n.Prefix + ":" + n.Name, "xmlns:" + n.Prefix + `="` + string(appendEscaped(nil, n.Space, true)) + `"`
	}
	if n.Space != "" {
		return n.Name, `xmlns="` + string(appendEscaped(nil, n.Space, true)) + `"`
	}
	return n.Name, ""
}

func appendIndent(b []byte, depth int) []byte {
	n := 1 + 2*depth
	if n > len(indentation) {
		n = len(indentation)
	}
	return append(b, indentation[:n]...)
}

// extensionWrapper declares the CII prefixes around an extension so that it
// can be parsed again.
const (
	extensionWrapperStart = `<x xmlns:rsm="` + NamespaceRSM + `" xmlns:ram="` + NamespaceRAM +
		`" xmlns:udt="` + NamespaceUDT + `" xmlns:qdt="` + NamespaceQDT + `">`
	extensionWrapperEnd = `</x>`
)

// parseExtension parses the XML of an element extension.
func parseExtension(e *invoice.Extension) (*xmltree.Node, error) {
	root, err := xmltree.Parse([]byte(extensionWrapperStart + e.XML + extensionWrapperEnd))
	if err != nil {
		return nil, err
	}
	if len(root.Children) != 1 {
		return nil, errBadExtension
	}
	return root.Children[0], nil
}

type extensionError string

func (e extensionError) Error() string { return string(e) }

const errBadExtension = extensionError("cii: extension is not a single element")

// splitAttrPath splits the path of an attribute extension into the path of
// the parent of the element that carries it, the local name of that element
// and its occurrence number (1 for the first child of that name).
func splitAttrPath(path string) (parent, name string, occ int) {
	parent, name = "", path
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		parent, name = path[:i], path[i+1:]
	}
	occ = 1
	if i := strings.IndexByte(name, '['); i >= 0 && strings.HasSuffix(name, "]") {
		if n, err := strconv.Atoi(name[i+1 : len(name)-1]); err == nil && n > 0 {
			occ = n
		}
		name = name[:i]
	}
	return parent, name, occ
}
