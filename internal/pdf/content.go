package pdf

import (
	"bytes"
	"errors"
)

// ErrInlineImage is returned by ScanContent when the data of an inline image
// has no end (no EI operator after ID).
var ErrInlineImage = errors.New("pdf: inline image without EI")

// maxOperands bounds the operands kept for an operator: no operator of ISO
// 32000-1 takes more than a few dozen (scn with a DeviceN colour space of 32
// colourants and a pattern name).
const maxOperands = 64

// ScanContent reads the operators of a content stream (ISO 32000-1, 7.8.2)
// and calls fn with each operator and its operands. An inline image (BI,
// ID, data, EI) is reported once, as the operator "BI" with a single
// operand: the dictionary of its parameters, with the keys as written,
// abbreviated or not. Comments are skipped. The streams of a page are
// scanned as one: the caller joins them, with white space between them.
//
// Scanning stops at the end of data, at the first error fn returns, which
// ScanContent returns, or at an inline image without end (ErrInlineImage).
// Syntax errors in operands are skipped, as viewers do.
func ScanContent(data []byte, fn func(op string, args []Object) error) error {
	p := &parser{b: data}
	var args []Object
	for {
		o, err := p.object()
		switch {
		case errors.Is(err, errEOF):
			return nil
		case err != nil:
			// An operand that cannot be read: skip it, keep the others.
			continue
		}
		kw, isOp := o.(keyword)
		if !isOp {
			if len(args) < maxOperands {
				args = append(args, o)
			}
			continue
		}
		op := string(kw)
		switch op {
		case "]", ">", ")", "{", "}":
			// Stray delimiters: not operators.
			continue
		case "BI":
			d, err := p.inlineImage()
			if err != nil {
				return err
			}
			if err := fn("BI", []Object{d}); err != nil {
				return err
			}
		default:
			if err := fn(op, args); err != nil {
				return err
			}
		}
		args = args[:0]
	}
}

// inlineImage reads the parameters of an inline image after BI, and skips
// its data up to the EI operator.
func (p *parser) inlineImage() (Dict, error) {
	var d Dict
	for {
		o, err := p.object()
		if errors.Is(err, errEOF) {
			return d, ErrInlineImage
		}
		if err != nil {
			continue
		}
		if kw, ok := o.(keyword); ok {
			if kw == "ID" {
				break
			}
			continue
		}
		key, ok := o.(Name)
		if !ok {
			continue
		}
		v, err := p.object()
		if errors.Is(err, errEOF) {
			return d, ErrInlineImage
		}
		if _, ok := v.(keyword); ok {
			// "ID" (or another keyword) where a value was expected.
			if v == keyword("ID") {
				break
			}
			continue
		}
		d.Set(key, v)
	}
	end, ok := inlineDataEnd(p.b, p.pos)
	if !ok {
		return d, ErrInlineImage
	}
	p.pos = end
	return d, nil
}

// inlineDataEnd returns the offset after the EI operator that ends the data
// of an inline image, the ID operator ending at pos. A single white-space
// character follows ID, then the data; EI ends it: the first one preceded
// by white space and followed by white space, a delimiter or the end of the
// stream.
func inlineDataEnd(b []byte, pos int) (int, bool) {
	if pos < len(b) && class[b[pos]] == white {
		pos++
	}
	for i := pos; ; {
		k := bytes.Index(b[i:], []byte("EI"))
		if k < 0 {
			return 0, false
		}
		k += i
		before := k == pos || class[b[k-1]] == white
		after := k+2 == len(b) || class[b[k+2]] != regular
		if before && after {
			return k + 2, true
		}
		i = k + 1
	}
}

// ContentSyntax checks the operands of a content stream as Syntax checks
// the tokens of an object, and calls fn for each deviation: hexadecimal
// strings (6.1.6) and the limits on numbers, strings and names (6.1.13).
// The data of inline images is skipped. The issues have no object number.
func ContentSyntax(data []byte, fn func(SyntaxIssue)) {
	l := lint{b: data, end: len(data), fn: fn}
	for {
		kw, _ := l.scan()
		switch kw {
		case "":
			return
		case "ID":
			end, ok := inlineDataEnd(data, l.pos)
			if !ok {
				return
			}
			l.pos = end
		}
	}
}
