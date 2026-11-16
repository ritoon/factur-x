package cii

import (
	"encoding/base64"
	"unicode/utf8"

	"github.com/ritoon/factur-x/invoice"
)

// writer appends XML to a byte slice. Element names are written as given
// (with their rsm/ram/udt/qdt prefix).
type writer struct {
	b      []byte
	depth  int
	indent bool
	lvl    level

	losses []string
}

// mark records the position of a start tag so that an element left empty can
// be removed again (see endOpt).
type mark struct {
	at   int // length of the buffer before the start tag
	body int // length of the buffer right after the start tag
}

// indentation is sliced to write a newline followed by the indentation of
// the current depth.
const indentation = "\n                                                                                "

func (w *writer) newline() {
	if !w.indent {
		return
	}
	n := 1 + 2*w.depth
	if n > len(indentation) {
		n = len(indentation)
	}
	w.b = append(w.b, indentation[:n]...)
}

// start writes the start tag of an element and returns its mark.
func (w *writer) start(name string) mark {
	at := len(w.b)
	w.newline()
	w.b = append(w.b, '<')
	w.b = append(w.b, name...)
	w.b = append(w.b, '>')
	w.depth++
	return mark{at: at, body: len(w.b)}
}

// end writes the end tag of an element that has child elements.
func (w *writer) end(name string) {
	w.depth--
	w.newline()
	w.b = append(w.b, '<', '/')
	w.b = append(w.b, name...)
	w.b = append(w.b, '>')
}

// endOpt closes an element, or removes it when nothing was written inside.
func (w *writer) endOpt(name string, m mark) {
	if len(w.b) == m.body {
		w.depth--
		w.b = w.b[:m.at]
		return
	}
	w.end(name)
}

// openLeaf writes "<name" so that attributes can follow.
func (w *writer) openLeaf(name string) {
	w.newline()
	w.b = append(w.b, '<')
	w.b = append(w.b, name...)
}

// closeLeaf writes ">value</name>".
func (w *writer) closeLeaf(name, value string) {
	w.b = append(w.b, '>')
	w.b = appendEscaped(w.b, value, false)
	w.b = append(w.b, '<', '/')
	w.b = append(w.b, name...)
	w.b = append(w.b, '>')
}

// attr writes ` name="value"`.
func (w *writer) attr(name, value string) {
	w.b = append(w.b, ' ')
	w.b = append(w.b, name...)
	w.b = append(w.b, '=', '"')
	w.b = appendEscaped(w.b, value, true)
	w.b = append(w.b, '"')
}

// leafReq writes a text element, even when value is empty.
func (w *writer) leafReq(name, value string) {
	w.openLeaf(name)
	w.closeLeaf(name, value)
}

// leaf writes a text element when value is not empty.
func (w *writer) leaf(name, value string) {
	if value != "" {
		w.leafReq(name, value)
	}
}

// leafAttr writes a text element with an optional attribute (omitted when
// empty). Nothing is written when value and attrValue are both empty.
func (w *writer) leafAttr(name, attrName, attrValue, value string) {
	if value == "" && attrValue == "" {
		return
	}
	w.openLeaf(name)
	if attrValue != "" {
		w.attr(attrName, attrValue)
	}
	w.closeLeaf(name, value)
}

// decReq writes a decimal element, even when d is absent.
func (w *writer) decReq(name string, d invoice.Decimal) {
	w.openLeaf(name)
	w.b = append(w.b, '>')
	w.b = d.Append(w.b)
	w.b = append(w.b, '<', '/')
	w.b = append(w.b, name...)
	w.b = append(w.b, '>')
}

// dec writes a decimal element when d is set.
func (w *writer) dec(name string, d invoice.Decimal) {
	if d.IsSet() {
		w.decReq(name, d)
	}
}

// decAttr writes a decimal element with an optional attribute, when d is set.
func (w *writer) decAttr(name, attrName, attrValue string, d invoice.Decimal) {
	if !d.IsSet() {
		return
	}
	w.openLeaf(name)
	if attrValue != "" {
		w.attr(attrName, attrValue)
	}
	w.b = append(w.b, '>')
	w.b = d.Append(w.b)
	w.b = append(w.b, '<', '/')
	w.b = append(w.b, name...)
	w.b = append(w.b, '>')
}

// dateIn writes <name><inner format="102">CCYYMMDD</inner></name>. inner is
// udt:DateTimeString, udt:DateString or qdt:DateTimeString. An absent date
// gives an empty inner element.
func (w *writer) dateIn(name, inner string, d invoice.Date) {
	w.start(name)
	w.openLeaf(inner)
	w.b = append(w.b, ` format="102">`...)
	if !d.IsZero() {
		w.b = appendDate102(w.b, d)
	}
	w.b = append(w.b, '<', '/')
	w.b = append(w.b, inner...)
	w.b = append(w.b, '>')
	w.end(name)
}

// dateTime writes a udt:DateTimeType element when d is set.
func (w *writer) dateTime(name string, d invoice.Date) {
	if !d.IsZero() {
		w.dateIn(name, "udt:DateTimeString", d)
	}
}

// indicator writes <name><udt:Indicator>true|false</udt:Indicator></name>.
func (w *writer) indicator(name string, v bool) {
	w.start(name)
	w.openLeaf("udt:Indicator")
	if v {
		w.b = append(w.b, ">true</udt:Indicator>"...)
	} else {
		w.b = append(w.b, ">false</udt:Indicator>"...)
	}
	w.end(name)
}

// binary writes a base64 binary object with its mimeCode and filename.
func (w *writer) binary(name string, data []byte, mime, filename string) {
	w.openLeaf(name)
	w.attr("mimeCode", mime)
	w.attr("filename", filename)
	w.b = append(w.b, '>')
	w.b = base64.StdEncoding.AppendEncode(w.b, data)
	w.b = append(w.b, '<', '/')
	w.b = append(w.b, name...)
	w.b = append(w.b, '>')
}

// lose records a business term that the target profile cannot carry.
func (w *writer) lose(term string) {
	for _, t := range w.losses {
		if t == term {
			return
		}
	}
	w.losses = append(w.losses, term)
}

// text writes a text element when value is not empty and allowed, and
// records term as lost when it is not allowed.
func (w *writer) text(allowed bool, name, value, term string) {
	if value == "" {
		return
	}
	if !allowed {
		w.lose(term)
		return
	}
	w.leafReq(name, value)
}

// amount is like text for decimals.
func (w *writer) amount(allowed bool, name string, d invoice.Decimal, term string) {
	if !d.IsSet() {
		return
	}
	if !allowed {
		w.lose(term)
		return
	}
	w.decReq(name, d)
}

// date is like text for udt:DateTimeType elements.
func (w *writer) date(allowed bool, name string, d invoice.Date, term string) {
	if d.IsZero() {
		return
	}
	if !allowed {
		w.lose(term)
		return
	}
	w.dateIn(name, "udt:DateTimeString", d)
}

// appendDate102 appends d as CCYYMMDD.
func appendDate102(b []byte, d invoice.Date) []byte {
	y, m, day := d.Year(), int(d.Month()), d.Day()
	if y < 0 || y > 9999 {
		y = 0
	}
	return append(b,
		byte('0'+y/1000), byte('0'+y/100%10), byte('0'+y/10%10), byte('0'+y%10),
		byte('0'+m/10%10), byte('0'+m%10),
		byte('0'+day/10%10), byte('0'+day%10))
}

// appendEscaped appends s escaped as XML character data (or as an attribute
// value when attr is set). Characters that XML 1.0 cannot represent and
// invalid UTF-8 are replaced by U+FFFD. Carriage returns are written as
// character references so that they survive end-of-line normalization.
func appendEscaped(b []byte, s string, attr bool) []byte {
	last := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			var esc string
			switch c {
			case '&':
				esc = "&amp;"
			case '<':
				esc = "&lt;"
			case '>':
				esc = "&gt;"
			case '"':
				if attr {
					esc = "&quot;"
				}
			case '\t':
				if attr {
					esc = "&#x9;"
				}
			case '\n':
				if attr {
					esc = "&#xA;"
				}
			case '\r':
				esc = "&#xD;"
			default:
				if c < 0x20 {
					esc = "�"
				}
			}
			if esc != "" {
				b = append(b, s[last:i]...)
				b = append(b, esc...)
				last = i + 1
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || r == 0xFFFE || r == 0xFFFF {
			b = append(b, s[last:i]...)
			b = append(b, "�"...)
			last = i + size
		}
		i += size
	}
	return append(b, s[last:]...)
}
