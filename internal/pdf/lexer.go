package pdf

import (
	"bytes"
	"errors"
	"math"
	"strconv"
)

// Character classes (ISO 32000-1, 7.2.2).
const (
	regular = iota
	white
	delim
)

var class = func() (c [256]uint8) {
	for _, b := range []byte{0, '\t', '\n', '\f', '\r', ' '} {
		c[b] = white
	}
	for _, b := range []byte("()<>[]{}/%") {
		c[b] = delim
	}
	return
}()

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isRegular(c byte) bool { return class[c] == regular }

// maxDepth bounds the nesting of arrays and dictionaries.
const maxDepth = 256

var (
	errEOF       = errors.New("pdf: unexpected end of data")
	errSyntax    = errors.New("pdf: syntax error")
	errTooDeep   = errors.New("pdf: objects nested too deeply")
	errNotObject = errors.New("pdf: no object header at offset")
)

// keyword is a bare keyword met where an object was expected ("endobj",
// "stream", "R"...). It never escapes the parser.
type keyword string

// parser reads objects from b. r, when set, resolves indirect stream
// lengths.
type parser struct {
	b     []byte
	pos   int
	r     *Reader
	depth int
}

// skip skips white space and comments.
func (p *parser) skip() {
	b := p.b
	for p.pos < len(b) {
		c := b[p.pos]
		if class[c] == white {
			p.pos++
			continue
		}
		if c == '%' {
			for p.pos < len(b) && b[p.pos] != '\n' && b[p.pos] != '\r' {
				p.pos++
			}
			continue
		}
		return
	}
}

// word returns the run of regular characters at the current position.
func (p *parser) word() []byte {
	start := p.pos
	for p.pos < len(p.b) && isRegular(p.b[p.pos]) {
		p.pos++
	}
	return p.b[start:p.pos]
}

// uint reads an unsigned decimal integer; ok is false when there is none.
func (p *parser) uint() (int64, bool) {
	start := p.pos
	var n int64
	for p.pos < len(p.b) && isDigit(p.b[p.pos]) {
		if n > (math.MaxInt64-9)/10 {
			return 0, false
		}
		n = n*10 + int64(p.b[p.pos]-'0')
		p.pos++
	}
	return n, p.pos > start
}

// object reads the next object. Bare keywords are returned as keyword.
func (p *parser) object() (Object, error) {
	p.skip()
	if p.pos >= len(p.b) {
		return nil, errEOF
	}
	c := p.b[p.pos]
	switch {
	case c == '/':
		p.pos++
		return p.name(), nil
	case c == '(':
		return p.literal()
	case c == '<':
		if p.pos+1 < len(p.b) && p.b[p.pos+1] == '<' {
			return p.dict()
		}
		return p.hex()
	case c == '[':
		return p.array()
	case isDigit(c) || c == '+' || c == '-' || c == '.':
		return p.number(), nil
	case c == ']' || c == '>' || c == ')' || c == '{' || c == '}':
		p.pos++
		return keyword(p.b[p.pos-1 : p.pos]), nil
	}
	w := p.word()
	switch string(w) {
	case "true":
		return Bool(true), nil
	case "false":
		return Bool(false), nil
	case "null":
		return nil, nil
	}
	if len(w) == 0 {
		p.pos++
		return nil, errSyntax
	}
	return keyword(w), nil
}

// name reads a name after the slash.
func (p *parser) name() Name {
	w := p.word()
	if bytes.IndexByte(w, '#') < 0 {
		return internName(w)
	}
	out := make([]byte, 0, len(w))
	for i := 0; i < len(w); i++ {
		if w[i] == '#' && i+2 < len(w) && isHex(w[i+1]) && isHex(w[i+2]) {
			out = append(out, unhex(w[i+1])<<4|unhex(w[i+2]))
			i += 2
			continue
		}
		out = append(out, w[i])
	}
	return Name(out)
}

// internName avoids allocating the most common names.
func internName(w []byte) Name {
	switch string(w) {
	case "Type":
		return "Type"
	case "Subtype":
		return "Subtype"
	case "Length":
		return "Length"
	case "Filter":
		return "Filter"
	case "FlateDecode":
		return "FlateDecode"
	case "DecodeParms":
		return "DecodeParms"
	case "Names":
		return "Names"
	case "Kids":
		return "Kids"
	case "Limits":
		return "Limits"
	case "EmbeddedFiles":
		return "EmbeddedFiles"
	case "EmbeddedFile":
		return "EmbeddedFile"
	case "Filespec":
		return "Filespec"
	case "F":
		return "F"
	case "UF":
		return "UF"
	case "EF":
		return "EF"
	case "Desc":
		return "Desc"
	case "AF":
		return "AF"
	case "AFRelationship":
		return "AFRelationship"
	case "Alternative":
		return "Alternative"
	case "Data":
		return "Data"
	case "Source":
		return "Source"
	case "Params":
		return "Params"
	case "ModDate":
		return "ModDate"
	case "CreationDate":
		return "CreationDate"
	case "Size":
		return "Size"
	case "CheckSum":
		return "CheckSum"
	case "Root":
		return "Root"
	case "Info":
		return "Info"
	case "ID":
		return "ID"
	case "Prev":
		return "Prev"
	case "Catalog":
		return "Catalog"
	case "Pages":
		return "Pages"
	case "Page":
		return "Page"
	case "Metadata":
		return "Metadata"
	case "XML":
		return "XML"
	case "OutputIntents":
		return "OutputIntents"
	case "Count":
		return "Count"
	case "Parent":
		return "Parent"
	case "Resources":
		return "Resources"
	case "MediaBox":
		return "MediaBox"
	case "Contents":
		return "Contents"
	case "Font":
		return "Font"
	case "XObject":
		return "XObject"
	case "ProcSet":
		return "ProcSet"
	case "Producer":
		return "Producer"
	case "Creator":
		return "Creator"
	case "Title":
		return "Title"
	case "Author":
		return "Author"
	case "ObjStm":
		return "ObjStm"
	case "XRef":
		return "XRef"
	case "N":
		return "N"
	case "First":
		return "First"
	case "W":
		return "W"
	case "Index":
		return "Index"
	case "Predictor":
		return "Predictor"
	case "Columns":
		return "Columns"
	case "text/xml":
		return "text/xml"
	}
	return Name(w)
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}

// literal reads a literal string; the result aliases the input when it holds
// no escape and no carriage return.
func (p *parser) literal() (Object, error) {
	b := p.b
	start := p.pos + 1
	// Fast path: no backslash, no CR, no nested parenthesis.
	for i := start; i < len(b); i++ {
		switch b[i] {
		case ')':
			p.pos = i + 1
			return String(b[start:i:i]), nil
		case '\\', '(', '\r':
			return p.literalSlow()
		}
	}
	p.pos = len(b)
	return String(b[start:len(b):len(b)]), nil
}

func (p *parser) literalSlow() (Object, error) {
	b := p.b
	i := p.pos + 1
	out := make([]byte, 0, 32)
	level := 1
	for i < len(b) {
		c := b[i]
		i++
		switch c {
		case '(':
			level++
		case ')':
			level--
			if level == 0 {
				p.pos = i
				return String(out), nil
			}
		case '\r':
			// An unescaped end of line is read as a line feed.
			if i < len(b) && b[i] == '\n' {
				i++
			}
			c = '\n'
		case '\\':
			if i >= len(b) {
				break
			}
			e := b[i]
			i++
			switch e {
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			case '\r':
				if i < len(b) && b[i] == '\n' {
					i++
				}
				continue
			case '\n':
				continue
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for k := 0; k < 2 && i < len(b) && b[i] >= '0' && b[i] <= '7'; k++ {
						v = v*8 + int(b[i]-'0')
						i++
					}
					c = byte(v)
				} else {
					c = e // \( \) \\ and unknown escapes
				}
			}
		}
		out = append(out, c)
	}
	p.pos = len(b)
	return String(out), nil
}

// hex reads a hexadecimal string.
func (p *parser) hex() (Object, error) {
	b := p.b
	i := p.pos + 1
	out := make([]byte, 0, 16)
	var hi byte
	half := false
	for i < len(b) {
		c := b[i]
		i++
		if c == '>' {
			p.pos = i
			if half {
				out = append(out, hi<<4)
			}
			return String(out), nil
		}
		if !isHex(c) {
			if class[c] == white {
				continue
			}
			// Tolerate garbage: skip it.
			continue
		}
		if half {
			out = append(out, hi<<4|unhex(c))
			half = false
		} else {
			hi = unhex(c)
			half = true
		}
	}
	p.pos = len(b)
	return String(out), nil
}

// number reads a number; an unsigned integer followed by another one and "R"
// is read as an indirect reference.
func (p *parser) number() Object {
	b := p.b
	start := p.pos
	neg := false
	for p.pos < len(b) && (b[p.pos] == '+' || b[p.pos] == '-') {
		neg = b[p.pos] == '-'
		p.pos++
	}
	signed := p.pos > start
	intStart := p.pos
	n, isInt := p.uint()
	if p.pos < len(b) && b[p.pos] == '.' {
		// Real number.
		p.pos++
		for p.pos < len(b) && isDigit(b[p.pos]) {
			p.pos++
		}
		f, err := strconv.ParseFloat(string(b[intStart:p.pos]), 64)
		if err != nil {
			f = 0
		}
		// Skip trailing garbage glued to the number ("1.2.3", "4-").
		for p.pos < len(b) && isRegular(b[p.pos]) {
			p.pos++
		}
		if neg {
			f = -f
		}
		return Real(f)
	}
	if !isInt {
		// Overflow or lone sign.
		for p.pos < len(b) && isRegular(b[p.pos]) {
			p.pos++
		}
		digits := b[intStart:p.pos]
		if f, err := strconv.ParseFloat(string(digits), 64); err == nil && len(digits) > 0 {
			if neg {
				f = -f
			}
			return Real(f)
		}
		return Int(0)
	}
	// Garbage glued to the integer ("12abc"): skip it.
	if p.pos < len(b) && isRegular(b[p.pos]) {
		for p.pos < len(b) && isRegular(b[p.pos]) {
			p.pos++
		}
		if neg {
			n = -n
		}
		return Int(n)
	}
	if !signed && n <= math.MaxUint32 {
		// Look ahead for "gen R".
		save := p.pos
		p.skip()
		if gen, ok := p.uint(); ok && gen <= math.MaxUint16 {
			p.skip()
			if p.pos < len(b) && b[p.pos] == 'R' && (p.pos+1 == len(b) || !isRegular(b[p.pos+1])) {
				p.pos++
				return Ref{Num: uint32(n), Gen: uint16(gen)}
			}
		}
		p.pos = save
	}
	if neg {
		n = -n
	}
	return Int(n)
}

// array reads an array.
func (p *parser) array() (Object, error) {
	p.pos++ // [
	if p.depth++; p.depth > maxDepth {
		return nil, errTooDeep
	}
	defer func() { p.depth-- }()
	arr := make(Array, 0, 4)
	for {
		p.skip()
		if p.pos >= len(p.b) {
			return arr, nil // truncated: keep what was read
		}
		if p.b[p.pos] == ']' {
			p.pos++
			return arr, nil
		}
		save := p.pos
		o, err := p.object()
		if err == errTooDeep {
			return nil, err
		}
		if err != nil {
			continue
		}
		if k, ok := o.(keyword); ok {
			switch k {
			case "endobj", "stream", "endstream", "obj", ">":
				// Unterminated array: stop before the keyword.
				p.pos = save
				return arr, nil
			}
			o = nil // unknown keyword read as null
		}
		arr = append(arr, o)
	}
}

// dict reads a dictionary.
func (p *parser) dict() (Object, error) {
	p.pos += 2 // <<
	if p.depth++; p.depth > maxDepth {
		return nil, errTooDeep
	}
	defer func() { p.depth-- }()
	d := make(Dict, 0, 8)
	for {
		p.skip()
		if p.pos >= len(p.b) {
			return d, nil
		}
		c := p.b[p.pos]
		if c == '>' {
			if p.pos+1 < len(p.b) && p.b[p.pos+1] == '>' {
				p.pos += 2
			} else {
				p.pos++
			}
			return d, nil
		}
		if c != '/' {
			// Not a key: skip one object (tolerance) or stop on a keyword.
			save := p.pos
			o, err := p.object()
			if err == errTooDeep {
				return nil, err
			}
			if k, ok := o.(keyword); ok && (k == "endobj" || k == "stream" || k == "endstream" || k == "obj") {
				p.pos = save
				return d, nil
			}
			continue
		}
		p.pos++
		key := p.name()
		p.skip()
		if p.pos < len(p.b) && p.b[p.pos] == '>' {
			// Key without value.
			continue
		}
		save := p.pos
		val, err := p.object()
		if err == errTooDeep {
			return nil, err
		}
		if err != nil {
			val = nil
		}
		if k, ok := val.(keyword); ok {
			switch k {
			case "endobj", "stream", "endstream", "obj":
				p.pos = save
				return d, nil
			}
			val = nil
		}
		// A null value is equivalent to an absent entry; the first of
		// duplicate keys wins.
		if val != nil && !d.Has(key) {
			d = append(d, KV{key, val})
		}
	}
}
