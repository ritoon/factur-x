package xmltree

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The scanner below reads the document in one pass over the input bytes. It
// accepts exactly the documents that encoding/xml (strict mode, RawToken)
// accepts and builds the same tree as the encoding/xml loop it replaced; the
// differential fuzzing in fuzz_test.go checks it. Consequences worth knowing:
//
//   - DOCTYPE and other <!...> declarations are skipped, never interpreted:
//     no entity is defined, no external resource is read.
//   - Only the five predefined entities and character references are
//     expanded; any other reference is an error.
//   - Character data and attribute values must be UTF-8 characters of the
//     XML Char production; "\r\n" and "\r" become "\n" (not in references).
//   - Element and attribute names follow the XML 1.0 (4th edition) tables of
//     encoding/xml (see names_gen.go); a name holds at most one colon.
//   - An XML declaration ("<?xml ...?>") may switch the encoding of the rest
//     of the input to ISO-8859-1 or windows-1252.

const xmlNamespace = "http://www.w3.org/XML/1998/namespace"

type parser struct {
	data []byte // input, replaced by its UTF-8 conversion after an encoding switch
	pos  int

	line    int // line number of data[linePos]
	linePos int

	root  *Node
	stack []frame   // open elements
	ns    []binding // namespace declarations in scope
	kids  []*Node   // children of the open elements, contiguous per element
	text  []byte    // character data of the open elements, contiguous per element
	attrs []Attr    // attributes of the start tag being read
	buf   []byte    // decoded attribute value or text outside the root

	// Slabs from which nodes, child lists and attribute lists are carved.
	nodes    []Node
	children []*Node
	attrSlab []Attr
}

// frame is an open element.
type frame struct {
	node *Node
	ns   int // len(parser.ns) before the declarations of the element
	kids int // len(parser.kids) when the element started
	text int // len(parser.text) when the element started
	// raw is the first character data of the element, left in the input
	// while it is the only one (the common case: no copy before the string).
	raw    []byte
	hasRaw bool
}

type binding struct {
	prefix, uri string
}

// Parse parses an XML document and returns its root element.
//
// DOCTYPE declarations are ignored and no entity other than the five
// predefined ones is expanded. Documents declared in ISO-8859-1 or
// windows-1252 are converted to UTF-8; other non-UTF-8 encodings are refused.
func Parse(data []byte) (*Node, error) {
	p := parser{data: bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), line: 1}
	return p.parse()
}

func (p *parser) parse() (*Node, error) {
	for p.pos < len(p.data) {
		var err error
		switch {
		case p.data[p.pos] != '<':
			err = p.charData(false)
		case p.pos+1 >= len(p.data):
			err = p.eof(p.pos)
		case p.data[p.pos+1] == '/':
			err = p.endTag()
		case p.data[p.pos+1] == '?':
			err = p.procInst()
		case p.data[p.pos+1] == '!':
			err = p.bang()
		default:
			err = p.startTag()
		}
		if err != nil {
			return nil, err
		}
	}
	if p.root == nil {
		return nil, errors.New("xml: no root element")
	}
	if len(p.stack) != 0 {
		return nil, fmt.Errorf("xml: unexpected end of document inside <%s>", p.stack[len(p.stack)-1].node.Name)
	}
	return p.root, nil
}

// lineAt returns the line of data[i]. Lines are counted lazily, so i must not
// go backwards (except for error messages, which get the last known line).
func (p *parser) lineAt(i int) int {
	if i > p.linePos {
		p.line += bytes.Count(p.data[p.linePos:i], []byte{'\n'})
		p.linePos = i
	}
	return p.line
}

func (p *parser) errorf(i int, format string, args ...any) error {
	return fmt.Errorf("xml: line %d: %s", p.lineAt(i), fmt.Sprintf(format, args...))
}

func (p *parser) eof(i int) error { return p.errorf(i, "unexpected end of document") }

// startTag reads "<name attr='value'...>" or "<name .../>" at p.pos.
func (p *parser) startTag() error {
	d := p.data
	lt := p.pos
	i := lt + 1
	j := nameEnd(d, i)
	if j >= len(d) {
		return p.eof(j)
	}
	prefix, local, ok := p.qname(d[i:j])
	if !ok {
		return p.errorf(i, "invalid element name %q", d[i:j])
	}

	nsStart := len(p.ns)
	p.attrs = p.attrs[:0]
	empty := false
	for i = j; ; {
		i = skipSpace(d, i)
		if i >= len(d) {
			return p.eof(i)
		}
		if c := d[i]; c == '>' {
			i++
			break
		} else if c == '/' {
			if i+1 >= len(d) {
				return p.eof(i + 1)
			}
			if d[i+1] != '>' {
				return p.errorf(i, "expected /> in element")
			}
			empty = true
			i += 2
			break
		}
		j = nameEnd(d, i)
		if j >= len(d) {
			return p.eof(j)
		}
		aname := d[i:j]
		aprefix, alocal, ok := p.qname(aname)
		if !ok {
			return p.errorf(i, "invalid attribute name %q", aname)
		}
		i = skipSpace(d, j)
		if i >= len(d) {
			return p.eof(i)
		}
		if d[i] != '=' {
			return p.errorf(i, "attribute %s without value", aname)
		}
		i = skipSpace(d, i+1)
		if i >= len(d) {
			return p.eof(i)
		}
		q := d[i]
		if q != '"' && q != '\'' {
			return p.errorf(i, "unquoted attribute value")
		}
		p.pos = i + 1
		v, raw, err := p.chars(p.buf[:0], q)
		if err != nil {
			return err
		}
		if !raw {
			p.buf = v[:0]
		}
		value := intern(&values, v)
		i = p.pos
		switch {
		case aprefix == "" && alocal == "xmlns":
			p.ns = append(p.ns, binding{"", value})
		case aprefix == "xmlns":
			p.ns = append(p.ns, binding{alocal, value})
		default:
			p.attrs = append(p.attrs, Attr{Name: alocal, Prefix: aprefix, Value: value})
		}
	}
	p.pos = i

	n := p.newNode()
	n.Name, n.Prefix, n.Line = local, prefix, p.lineAt(lt)
	if len(p.attrs) > 0 {
		n.Attrs = p.attrList(p.attrs)
	}
	if len(p.stack) > 0 {
		n.Parent = p.stack[len(p.stack)-1].node
		p.kids = append(p.kids, n)
	} else if p.root != nil {
		return errors.New("xml: more than one root element")
	} else {
		p.root = n
	}
	if len(p.stack) >= MaxDepth {
		return p.errorf(lt, "elements nested more than %d levels deep", MaxDepth)
	}
	n.Space = p.lookup(prefix)
	if prefix != "" && n.Space == "" {
		return p.errorf(lt, "undeclared prefix %q", prefix)
	}
	if empty {
		p.ns = p.ns[:nsStart]
		return nil
	}
	p.stack = append(p.stack, frame{node: n, ns: nsStart, kids: len(p.kids), text: len(p.text)})
	return nil
}

func (p *parser) lookup(prefix string) string {
	for i := len(p.ns) - 1; i >= 0; i-- {
		if p.ns[i].prefix == prefix {
			return p.ns[i].uri
		}
	}
	if prefix == "xml" {
		return xmlNamespace
	}
	return ""
}

// endTag reads "</name>" at p.pos and closes the current element.
func (p *parser) endTag() error {
	d := p.data
	i := p.pos + 2
	j := nameEnd(d, i)
	if j >= len(d) {
		return p.eof(j)
	}
	if len(p.stack) == 0 {
		return p.errorf(i, "unexpected end tag </%s>", d[i:j])
	}
	f := &p.stack[len(p.stack)-1]
	n := f.node
	if !sameName(d[i:j], n.Prefix, n.Name) {
		return p.errorf(i, "element <%s> closed by </%s>", qualified(n.Prefix, n.Name), d[i:j])
	}
	k := skipSpace(d, j)
	if k >= len(d) {
		return p.eof(k)
	}
	if d[k] != '>' {
		return p.errorf(k, "invalid characters in end tag </%s>", d[i:j])
	}
	p.pos = k + 1

	switch {
	case f.hasRaw:
		n.Text = intern(&values, f.raw)
	case len(p.text) > f.text:
		n.Text = intern(&values, p.text[f.text:])
		p.text = p.text[:f.text]
	}
	if len(p.kids) > f.kids {
		n.Children = p.childList(p.kids[f.kids:])
		p.kids = p.kids[:f.kids]
	}
	p.ns = p.ns[:f.ns]
	p.stack = p.stack[:len(p.stack)-1]
	return nil
}

// sameName reports whether the name of an end tag is prefix:local.
func sameName(b []byte, prefix, local string) bool {
	if prefix == "" {
		return string(b) == local
	}
	return len(b) == len(prefix)+1+len(local) && string(b[:len(prefix)]) == prefix &&
		b[len(prefix)] == ':' && string(b[len(prefix)+1:]) == local
}

func qualified(prefix, local string) string {
	if prefix == "" {
		return local
	}
	return prefix + ":" + local
}

// charData reads a run of character data (cdata false) or a CDATA section
// whose "<![CDATA[" has been consumed (cdata true).
func (p *parser) charData(cdata bool) error {
	if len(p.stack) == 0 {
		out, raw, err := p.readText(p.buf[:0], cdata)
		if err != nil {
			return err
		}
		if !raw {
			p.buf = out[:0]
		}
		if len(bytes.TrimSpace(out)) > 0 {
			return p.errorf(p.pos, "text outside the root element")
		}
		return nil
	}
	f := &p.stack[len(p.stack)-1]
	if f.hasRaw {
		p.text = append(p.text, f.raw...)
		f.raw, f.hasRaw = nil, false
	}
	before := len(p.text)
	out, raw, err := p.readText(p.text, cdata)
	if err != nil {
		return err
	}
	switch {
	case !raw:
		p.text = out
	case before == f.text:
		f.raw, f.hasRaw = out, true
	default:
		p.text = append(p.text, out...)
	}
	return nil
}

func (p *parser) readText(dst []byte, cdata bool) ([]byte, bool, error) {
	if cdata {
		return p.cdata(dst)
	}
	return p.chars(dst, 0)
}

// Byte classes of character data: 0 for bytes copied as is, 1 for bytes that
// need a closer look.
var textClass, attrClass, cdataClass [256]uint8

func init() {
	for c := range 256 {
		special := uint8(0)
		if c < 0x20 && c != '\t' && c != '\n' || c >= 0x80 {
			special = 1
		}
		cdataClass[c] = special
		textClass[c] = special
		attrClass[c] = special
	}
	for _, c := range []byte("<&]\r") {
		textClass[c] = 1
	}
	for _, c := range []byte("<&\r\"'") {
		attrClass[c] = 1
	}
	cdataClass['\r'] = 1
}

// chars reads character data at p.pos: up to the next '<' or the end of the
// input (quote 0), or up to the closing quote, which is consumed (attribute
// value). When the data needs no decoding it is returned as a sub-slice of the
// input with raw true and dst is left alone; otherwise the decoded data is
// appended to dst.
func (p *parser) chars(dst []byte, quote byte) (out []byte, raw bool, err error) {
	d := p.data
	class := &textClass
	if quote != 0 {
		class = &attrClass
	}
	start := p.pos
	from := -1 // start of the run not yet copied to dst; -1 while raw
	i := start
	for {
		for i < len(d) && class[d[i]] == 0 {
			i++
		}
		if i >= len(d) {
			if quote != 0 {
				return nil, false, p.eof(i)
			}
			break
		}
		c := d[i]
		if c == '<' {
			if quote != 0 {
				return nil, false, p.errorf(i, "unescaped < in attribute value")
			}
			break
		}
		if quote != 0 && c == quote {
			p.pos = i + 1
			return p.flush(dst, d, start, from, i)
		}
		switch {
		case c == '&':
			dst = appendRun(dst, d, start, from, i)
			if dst, i, err = p.reference(dst, i); err != nil {
				return nil, false, err
			}
			from = i
		case c == '\r':
			dst = append(appendRun(dst, d, start, from, i), '\n')
			if i++; i < len(d) && d[i] == '\n' {
				i++
			}
			from = i
		case c == ']':
			if i+2 < len(d) && d[i+1] == ']' && d[i+2] == '>' {
				return nil, false, p.errorf(i, "unescaped ]]> in text")
			}
			i++
		case c == '"' || c == '\'':
			i++ // the other quote, in an attribute value
		case c < utf8.RuneSelf:
			return nil, false, p.errorf(i, "illegal character code %U", rune(c))
		default:
			size, err := p.char(d, i)
			if err != nil {
				return nil, false, err
			}
			i += size
		}
	}
	p.pos = i
	return p.flush(dst, d, start, from, i)
}

// flush ends a scan of character data from d[start] to d[end]: from is the
// start of the run not yet copied to dst, -1 when nothing was decoded.
func (p *parser) flush(dst, d []byte, start, from, end int) ([]byte, bool, error) {
	if from < 0 {
		return d[start:end], true, nil
	}
	return append(dst, d[from:end]...), false, nil
}

// appendRun appends to dst the data not yet copied before d[i].
func appendRun(dst, d []byte, start, from, i int) []byte {
	if from < 0 {
		from = start
	}
	return append(dst, d[from:i]...)
}

// char checks the multi-byte character at d[i] and returns its size.
func (p *parser) char(d []byte, i int) (int, error) {
	r, size := utf8.DecodeRune(d[i:])
	if r == utf8.RuneError && size == 1 {
		return 0, p.errorf(i, "invalid UTF-8")
	}
	if r == 0xFFFE || r == 0xFFFF {
		return 0, p.errorf(i, "illegal character code %U", r)
	}
	return size, nil
}

// cdata reads the content of a CDATA section, p.pos being just after
// "<![CDATA[". The result is as for chars.
func (p *parser) cdata(dst []byte) ([]byte, bool, error) {
	d := p.data
	start := p.pos
	k := bytes.Index(d[start:], []byte("]]>"))
	if k < 0 {
		return nil, false, p.errorf(start, "unexpected end of document in CDATA section")
	}
	end := start + k
	p.pos = end + 3
	from := -1
	for i := start; ; {
		for i < end && cdataClass[d[i]] == 0 {
			i++
		}
		if i >= end {
			break
		}
		switch c := d[i]; {
		case c == '\r':
			dst = append(appendRun(dst, d, start, from, i), '\n')
			if i++; i < end && d[i] == '\n' {
				i++
			}
			from = i
		case c < utf8.RuneSelf:
			return nil, false, p.errorf(i, "illegal character code %U", rune(c))
		default:
			size, err := p.char(d, i)
			if err != nil {
				return nil, false, err
			}
			i += size
		}
	}
	return p.flush(dst, d, start, from, end)
}

// reference expands the entity or character reference at d[i] ('&') into
// dst and returns the index after its ';'.
func (p *parser) reference(dst []byte, i int) ([]byte, int, error) {
	d := p.data
	j := i + 1
	if j >= len(d) {
		return nil, 0, p.eof(j)
	}
	if d[j] != '#' {
		k := nameEnd(d, j)
		if k >= len(d) {
			return nil, 0, p.eof(k)
		}
		var c byte
		if d[k] == ';' {
			switch string(d[j:k]) {
			case "lt":
				c = '<'
			case "gt":
				c = '>'
			case "amp":
				c = '&'
			case "apos":
				c = '\''
			case "quot":
				c = '"'
			}
		}
		if c == 0 {
			return nil, 0, p.errorf(i, "invalid character entity %s", d[i:min(k+1, len(d))])
		}
		return append(dst, c), k + 1, nil
	}
	j++
	if j >= len(d) {
		return nil, 0, p.eof(j)
	}
	base := rune(10)
	if d[j] == 'x' {
		base = 16
		if j++; j >= len(d) {
			return nil, 0, p.eof(j)
		}
	}
	var r rune
	digits, tooBig := j, false
	for ; j < len(d); j++ {
		c := d[j]
		var v rune
		switch {
		case '0' <= c && c <= '9':
			v = rune(c - '0')
		case base == 16 && 'a' <= c && c <= 'f':
			v = rune(c-'a') + 10
		case base == 16 && 'A' <= c && c <= 'F':
			v = rune(c-'A') + 10
		default:
			v = -1
		}
		if v < 0 {
			break
		}
		if !tooBig {
			if r = r*base + v; r > utf8.MaxRune {
				tooBig = true
			}
		}
	}
	if j >= len(d) {
		return nil, 0, p.eof(j)
	}
	if d[j] != ';' || j == digits || tooBig {
		return nil, 0, p.errorf(i, "invalid character reference %s", d[i:j])
	}
	if 0xD800 <= r && r <= 0xDFFF {
		r = utf8.RuneError // as string(rune(r)) does
	}
	if !inCharRange(r) {
		return nil, 0, p.errorf(i, "illegal character code %U", r)
	}
	return utf8.AppendRune(dst, r), j + 1, nil
}

// inCharRange reports whether r is allowed by the XML Char production.
func inCharRange(r rune) bool {
	return r == 0x09 || r == 0x0A || r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}

// procInst reads a processing instruction at p.pos. An XML declaration is
// checked and may switch the encoding of the rest of the input.
func (p *parser) procInst() error {
	d := p.data
	i := p.pos + 2
	j := nameEnd(d, i)
	if j >= len(d) {
		return p.eof(j)
	}
	if !validName(d[i:j]) {
		return p.errorf(i, "invalid processing instruction target %q", d[i:j])
	}
	k := skipSpace(d, j)
	e := bytes.Index(d[k:], []byte("?>"))
	if e < 0 {
		return p.eof(len(d))
	}
	p.pos = k + e + 2
	if string(d[i:j]) != "xml" {
		return nil
	}
	content := string(d[k : k+e])
	if v := pseudoAttr(content, "version"); v != "" && v != "1.0" {
		return p.errorf(i, "unsupported XML version %q", v)
	}
	enc := pseudoAttr(content, "encoding")
	if enc == "" || strings.EqualFold(enc, "utf-8") {
		return nil
	}
	conv, err := decodeCharset(enc, d[p.pos:])
	if err != nil {
		return fmt.Errorf("xml: %w", err)
	}
	if conv != nil {
		p.lineAt(p.pos)
		p.data, p.pos, p.linePos = conv, 0, 0
	}
	return nil
}

// pseudoAttr returns the value of name="..." or name='...' in the content of
// an XML declaration, found the way encoding/xml finds it ("" when absent).
func pseudoAttr(content, name string) string {
	key := name + "="
	for rest := content; ; {
		k := strings.Index(rest, key)
		if k < 0 || k+len(key) >= len(rest) {
			return ""
		}
		q := rest[k+len(key)]
		rest = rest[k+len(key)+1:]
		if q == '"' || q == '\'' {
			v, _, ok := strings.Cut(rest, string(q))
			if !ok {
				return ""
			}
			return v
		}
	}
}

// bang reads a comment, a CDATA section or a declaration (<!DOCTYPE ...>)
// at p.pos.
func (p *parser) bang() error {
	d := p.data
	i := p.pos + 2
	if i >= len(d) {
		return p.eof(i)
	}
	switch d[i] {
	case '-':
		if i+1 >= len(d) {
			return p.eof(i + 1)
		}
		if d[i+1] != '-' {
			return p.errorf(i, "invalid sequence <!- not part of <!--")
		}
		i += 2
		k := bytes.Index(d[i:], []byte("--"))
		if k < 0 {
			return p.eof(len(d))
		}
		j := i + k + 2
		if j >= len(d) {
			return p.eof(j)
		}
		if d[j] != '>' {
			return p.errorf(j, `invalid sequence "--" in comment`)
		}
		p.pos = j + 1
		return nil
	case '[':
		const open = "[CDATA["
		if !bytes.HasPrefix(d[i:], []byte(open)) {
			return p.errorf(i, "invalid <![ sequence")
		}
		p.pos = i + len(open)
		return p.charData(true)
	}
	return p.declaration(i + 1)
}

// declaration skips a declaration such as <!DOCTYPE ...>, i being after the
// first character following "<!" (taken as is, as encoding/xml does). Quoted
// strings, nested <...> and comments may contain '>'.
func (p *parser) declaration(i int) error {
	d := p.data
	var quote byte
	depth := 0
	for {
		if i >= len(d) {
			return p.eof(i)
		}
		b := d[i]
		i++
		if quote == 0 && b == '>' && depth == 0 {
			p.pos = i
			return nil
		}
	handle:
		switch {
		case b == quote:
			quote = 0
		case quote != 0:
		case b == '\'' || b == '"':
			quote = b
		case b == '>':
			depth--
		case b == '<':
			for _, want := range []byte("!--") {
				if i >= len(d) {
					return p.eof(i)
				}
				b = d[i]
				i++
				if b != want {
					depth++
					goto handle
				}
			}
			k := bytes.Index(d[i:], []byte("-->"))
			if k < 0 {
				return p.eof(len(d))
			}
			i += k + 3
		}
	}
}

func skipSpace(d []byte, i int) int {
	for i < len(d) {
		switch d[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// newNode returns a zeroed node from the node slab.
func (p *parser) newNode() *Node {
	if len(p.nodes) == cap(p.nodes) {
		p.nodes = make([]Node, 0, slabSize(cap(p.nodes), 1))
	}
	p.nodes = p.nodes[:len(p.nodes)+1]
	return &p.nodes[len(p.nodes)-1]
}

// childList copies kids into the child slab. The result has no spare
// capacity, so that appending to it never overwrites another list.
func (p *parser) childList(kids []*Node) []*Node {
	if cap(p.children)-len(p.children) < len(kids) {
		p.children = make([]*Node, 0, slabSize(cap(p.children), len(kids)))
	}
	a := len(p.children)
	p.children = append(p.children, kids...)
	return p.children[a:len(p.children):len(p.children)]
}

// attrList copies attrs into the attribute slab, as childList does.
func (p *parser) attrList(attrs []Attr) []Attr {
	if cap(p.attrSlab)-len(p.attrSlab) < len(attrs) {
		p.attrSlab = make([]Attr, 0, slabSize(cap(p.attrSlab), len(attrs)))
	}
	a := len(p.attrSlab)
	p.attrSlab = append(p.attrSlab, attrs...)
	return p.attrSlab[a:len(p.attrSlab):len(p.attrSlab)]
}

// slabSize returns the capacity of the slab that follows one of capacity
// prev, holding at least need items: doubling, from 16 up to 1024.
func slabSize(prev, need int) int {
	return max(min(max(2*prev, 16), 1024), need)
}
