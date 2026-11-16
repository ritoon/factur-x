package xmltree

// The encoding/xml implementation of Parse that the scanner of parse.go
// replaced, kept as the reference of the differential tests: parseStd must
// accept the same documents as Parse and build the same trees.

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// parseStd parses an XML document with encoding/xml.
//
// DOCTYPE declarations are ignored and no entity other than the five
// predefined ones is expanded. Documents declared in ISO-8859-1 or
// windows-1252 are converted to UTF-8; other non-UTF-8 encodings are refused.
func parseStd(data []byte) (*Node, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = charsetReaderStd

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
			if len(stack) >= MaxDepth {
				return nil, fmt.Errorf("xml: line %d: elements nested more than %d levels deep", line, MaxDepth)
			}
			stack = append(stack, scope{node: n, ns: ns})
			n.Space = lookup(n.Prefix)
			if n.Prefix != "" && n.Space == "" {
				return nil, fmt.Errorf("xml: line %d: undeclared prefix %q", line, n.Prefix)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("xml: line %d: unexpected end tag </%s>", line, qnameStd(t.Name))
			}
			top := stack[len(stack)-1].node
			if top.Name != t.Name.Local || top.Prefix != t.Name.Space {
				return nil, fmt.Errorf("xml: line %d: element <%s> closed by </%s>", line, qnameStd(xml.Name{Space: top.Prefix, Local: top.Name}), qnameStd(t.Name))
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

func qnameStd(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return n.Space + ":" + n.Local
}

// charsetReaderStd converts single-byte Latin encodings to UTF-8.
func charsetReaderStd(label string, input io.Reader) (io.Reader, error) {
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
