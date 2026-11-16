package xmltree

import (
	"bytes"
	"fmt"
	"hash/maphash"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

//go:generate go run ./gen -o names_gen.go

// ASCII classes of name characters.
const (
	inName    = 1 << iota // part of a name run: [A-Za-z0-9_:.-] and every byte >= 0x80
	nameStart             // may start a name: [A-Za-z_:]
	nameChar              // may follow the first character: [A-Za-z0-9_:.-]
)

var nameClass [256]uint8

func init() {
	for c := range 256 {
		var k uint8
		switch {
		case c >= 0x80:
			k = inName
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', c == '_', c == ':':
			k = inName | nameStart | nameChar
		case '0' <= c && c <= '9', c == '.', c == '-':
			k = inName | nameChar
		}
		nameClass[c] = k
	}
}

// nameEnd returns the end of the name run starting at d[i]: the bytes up to
// the first ASCII byte that cannot appear in a name. The run is checked by
// validName.
func nameEnd(d []byte, i int) int {
	for i < len(d) && nameClass[d[i]]&inName != 0 {
		i++
	}
	return i
}

// validName reports whether b is an XML name.
func validName(b []byte) bool {
	if len(b) == 0 || nameClass[b[0]]&nameStart == 0 && b[0] < utf8.RuneSelf {
		return false
	}
	for i := 0; i < len(b); {
		c := b[i]
		if c < utf8.RuneSelf {
			if nameClass[c]&nameChar == 0 {
				return false
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if i == 0 && !inRanges(nameStartRanges[:], r) || i > 0 && !inRanges(nameCharRanges[:], r) {
			return false
		}
		i += size
	}
	return true
}

type runeRange struct{ lo, hi uint16 }

func inRanges(rs []runeRange, r rune) bool {
	if r > 0xFFFF {
		return false
	}
	lo, hi := 0, len(rs)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		switch {
		case rune(rs[m].hi) < r:
			lo = m + 1
		case rune(rs[m].lo) > r:
			hi = m
		default:
			return true
		}
	}
	return false
}

// qname checks the name of an element or attribute and splits it into prefix
// and local name. A name holds at most one colon; a colon at either end is
// part of the local name.
func (p *parser) qname(b []byte) (prefix, local string, ok bool) {
	if !validName(b) {
		return "", "", false
	}
	c := bytes.IndexByte(b, ':')
	switch {
	case c < 0:
		return "", intern(&names, b), true
	case bytes.IndexByte(b[c+1:], ':') >= 0:
		return "", "", false
	case c == 0 || c == len(b)-1:
		return "", intern(&names, b), true
	}
	return intern(&names, b[:c]), intern(&names, b[c+1:]), true
}

// internTable is a lossy, lock-free cache of short strings shared by all
// parses: element and attribute names, and short values (codes, amounts,
// dates, indentation), repeat from one invoice to the next. A slot holds the
// last string stored in it; a lookup only returns a string equal to the bytes
// asked for.
type internTable struct {
	max   int
	slots [4096]atomic.Pointer[string]
}

var (
	names      = internTable{max: 64}
	values     = internTable{max: 16}
	internSeed = maphash.MakeSeed()
)

// intern returns string(b), shared with earlier calls when possible.
func intern(t *internTable, b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) > t.max {
		return string(b)
	}
	slot := &t.slots[maphash.Bytes(internSeed, b)%uint64(len(t.slots))]
	if s := slot.Load(); s != nil && *s == string(b) {
		return *s
	}
	s := string(b)
	slot.Store(&s)
	return s
}

// decodeCharset converts data from the encoding declared by an XML
// declaration to UTF-8. It returns nil when data needs no conversion, and an
// error for the encodings not supported (other than UTF-8, US-ASCII,
// ISO-8859-1 and windows-1252).
func decodeCharset(label string, data []byte) ([]byte, error) {
	switch strings.ToLower(label) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return nil, nil
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1", "l1", "windows-1252", "cp1252":
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
		return out, nil
	}
	return nil, fmt.Errorf("unsupported XML encoding %q", label)
}

// win1252 maps bytes 0x80–0x9F of windows-1252 to Unicode (0 = undefined).
var win1252 = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}
