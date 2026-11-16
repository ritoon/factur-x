package pdf

import (
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// pdfDoc maps the bytes of PDFDocEncoding that differ from ISO 8859-1
// (ISO 32000-1, Annex D.2). 0 marks an undefined code.
var pdfDoc = map[byte]rune{
	0x18: 0x02D8, 0x19: 0x02C7, 0x1A: 0x02C6, 0x1B: 0x02D9, 0x1C: 0x02DD, 0x1D: 0x02DB, 0x1E: 0x02DA, 0x1F: 0x02DC,
	0x7F: 0,
	0x80: 0x2022, 0x81: 0x2020, 0x82: 0x2021, 0x83: 0x2026, 0x84: 0x2014, 0x85: 0x2013, 0x86: 0x0192, 0x87: 0x2044,
	0x88: 0x2039, 0x89: 0x203A, 0x8A: 0x2212, 0x8B: 0x2030, 0x8C: 0x201E, 0x8D: 0x201C, 0x8E: 0x201D, 0x8F: 0x2018,
	0x90: 0x2019, 0x91: 0x201A, 0x92: 0x2122, 0x93: 0xFB01, 0x94: 0xFB02, 0x95: 0x0141, 0x96: 0x0152, 0x97: 0x0160,
	0x98: 0x0178, 0x99: 0x017D, 0x9A: 0x0131, 0x9B: 0x0142, 0x9C: 0x0153, 0x9D: 0x0161, 0x9E: 0x017E, 0x9F: 0,
	0xA0: 0x20AC, 0xAD: 0,
}

// Text decodes a PDF text string: UTF-16BE with a byte order mark, UTF-8
// with a byte order mark (PDF 2.0), or PDFDocEncoding. A UTF-16LE byte order
// mark, which some writers produce, is also accepted.
func Text(s []byte) string {
	switch {
	case len(s) >= 2 && s[0] == 0xFE && s[1] == 0xFF:
		return utf16String(s[2:], true)
	case len(s) >= 2 && s[0] == 0xFF && s[1] == 0xFE:
		return utf16String(s[2:], false)
	case len(s) >= 3 && s[0] == 0xEF && s[1] == 0xBB && s[2] == 0xBF:
		return strings.ToValidUTF8(string(s[3:]), "�")
	}
	ascii := true
	for _, c := range s {
		if c >= 0x7F || (c < 0x20 && c >= 0x18) {
			ascii = false
			break
		}
	}
	if ascii {
		return string(s)
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, c := range s {
		if r, ok := pdfDoc[c]; ok {
			if r == 0 {
				r = utf8.RuneError
			}
			b.WriteRune(r)
		} else {
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

func utf16String(s []byte, be bool) string {
	u := make([]uint16, 0, len(s)/2)
	for i := 0; i+1 < len(s); i += 2 {
		if be {
			u = append(u, uint16(s[i])<<8|uint16(s[i+1]))
		} else {
			u = append(u, uint16(s[i+1])<<8|uint16(s[i]))
		}
	}
	// Drop language escape sequences (U+001B ... U+001B).
	out := u[:0]
	for i := 0; i < len(u); i++ {
		if u[i] == 0x1B {
			j := i + 1
			for j < len(u) && u[j] != 0x1B {
				j++
			}
			i = j
			continue
		}
		out = append(out, u[i])
	}
	return string(utf16.Decode(out))
}

// EncodeText encodes s as a PDF text string: unchanged when it is printable
// ASCII, UTF-16BE with a byte order mark otherwise.
func EncodeText(s string) String {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] >= 0x7F {
			ascii = false
			break
		}
	}
	if ascii {
		return String(s)
	}
	u := utf16.Encode([]rune(s))
	out := make([]byte, 2, 2+2*len(u))
	out[0], out[1] = 0xFE, 0xFF
	for _, c := range u {
		out = append(out, byte(c>>8), byte(c))
	}
	return out
}

// ParseDate parses a PDF date ("D:YYYYMMDDHHmmSSOHH'mm'"). It is lenient:
// the "D:" prefix, trailing fields and apostrophes are optional, and "Z" may
// be followed by an offset. ok is false when the year is missing or a field
// is out of range.
func ParseDate(s string) (t time.Time, ok bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "D:")
	i := 0
	num := func(n int) (int, bool) {
		if i+n > len(s) {
			return 0, false
		}
		v := 0
		for k := 0; k < n; k++ {
			c := s[i+k]
			if c < '0' || c > '9' {
				return 0, false
			}
			v = v*10 + int(c-'0')
		}
		i += n
		return v, true
	}
	year, ok := num(4)
	if !ok {
		return time.Time{}, false
	}
	f := [5]int{1, 1, 0, 0, 0} // month, day, hour, minute, second
	for k := range f {
		v, ok := num(2)
		if !ok {
			break
		}
		f[k] = v
	}
	month, day, hour, minute, sec := f[0], f[1], f[2], f[3], f[4]
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || minute > 59 || sec > 60 {
		return time.Time{}, false
	}
	if sec == 60 {
		sec = 59
	}
	loc := time.UTC
	if i < len(s) {
		sign := s[i]
		i++
		switch sign {
		case '+', '-':
			oh, ok := num(2)
			if !ok {
				break
			}
			if i < len(s) && s[i] == '\'' {
				i++
			}
			om, _ := num(2)
			if oh > 23 || om > 59 {
				return time.Time{}, false
			}
			offset := oh*3600 + om*60
			if sign == '-' {
				offset = -offset
			}
			if offset != 0 {
				loc = time.FixedZone("", offset)
			}
		case 'Z', 'z':
		}
	}
	t = time.Date(year, time.Month(month), day, hour, minute, sec, 0, loc)
	if t.Day() != day {
		return time.Time{}, false // 31 February
	}
	return t, true
}

// FormatDate formats t as a PDF date, "D:20261116143000+01'00'" ("Z" for
// UTC). Sub-second precision is dropped.
func FormatDate(t time.Time) string {
	b := make([]byte, 0, 24)
	b = append(b, "D:"...)
	b = t.AppendFormat(b, "20060102150405")
	_, off := t.Zone()
	if off == 0 {
		return string(append(b, 'Z'))
	}
	if off < 0 {
		b = append(b, '-')
		off = -off
	} else {
		b = append(b, '+')
	}
	h, m := off/3600, off%3600/60
	b = append(b, byte('0'+h/10), byte('0'+h%10), '\'', byte('0'+m/10), byte('0'+m%10), '\'')
	return string(b)
}
