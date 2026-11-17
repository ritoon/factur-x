// Package xmlenc holds the XML escaping shared by the CII and UBL writers.
package xmlenc

import "unicode/utf8"

// AppendEscaped appends s escaped as XML character data (or as an attribute
// value when attr is set). Characters that XML 1.0 cannot represent and
// invalid UTF-8 are replaced by U+FFFD. Carriage returns are written as
// character references so that they survive end-of-line normalization.
func AppendEscaped(b []byte, s string, attr bool) []byte {
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
