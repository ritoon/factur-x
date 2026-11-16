package pdf

import (
	"math"
	"sort"
	"strconv"
)

// AppendObject appends the serialization of o to b. Streams cannot be
// serialized inline and are written as null.
func AppendObject(b []byte, o Object) []byte {
	switch v := o.(type) {
	case nil:
		return append(b, "null"...)
	case Bool:
		if v {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case Int:
		return strconv.AppendInt(b, int64(v), 10)
	case Real:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return append(b, '0')
		}
		return strconv.AppendFloat(b, f, 'f', -1, 64)
	case Name:
		return appendName(b, v)
	case String:
		return appendString(b, v)
	case Ref:
		b = strconv.AppendUint(b, uint64(v.Num), 10)
		b = append(b, ' ')
		b = strconv.AppendUint(b, uint64(v.Gen), 10)
		return append(b, " R"...)
	case Array:
		b = append(b, '[')
		for i, x := range v {
			if i > 0 {
				b = append(b, ' ')
			}
			b = AppendObject(b, x)
		}
		return append(b, ']')
	case Dict:
		b = append(b, "<<"...)
		for _, kv := range v {
			if kv.Val == nil {
				continue
			}
			b = appendName(b, kv.Key)
			b = append(b, ' ')
			b = AppendObject(b, kv.Val)
		}
		return append(b, ">>"...)
	}
	return append(b, "null"...)
}

func appendName(b []byte, n Name) []byte {
	b = append(b, '/')
	const hexdigits = "0123456789ABCDEF"
	for i := 0; i < len(n); i++ {
		c := n[i]
		if c < 0x21 || c > 0x7E || c == '#' || class[c] != regular {
			b = append(b, '#', hexdigits[c>>4], hexdigits[c&15])
			continue
		}
		b = append(b, c)
	}
	return b
}

// appendString writes a literal string when s is printable ASCII, a
// hexadecimal string otherwise.
func appendString(b []byte, s String) []byte {
	printable := true
	for _, c := range s {
		if c < 0x20 || c > 0x7E {
			printable = false
			break
		}
	}
	if !printable {
		const hexdigits = "0123456789ABCDEF"
		b = append(b, '<')
		for _, c := range s {
			b = append(b, hexdigits[c>>4], hexdigits[c&15])
		}
		return append(b, '>')
	}
	b = append(b, '(')
	for _, c := range s {
		if c == '(' || c == ')' || c == '\\' {
			b = append(b, '\\')
		}
		b = append(b, c)
	}
	return append(b, ')')
}

// Update collects new and replaced objects and appends them to the original
// file as an incremental update (ISO 32000-1, 7.5.6): the original bytes are
// kept unchanged, which preserves existing signatures.
type Update struct {
	r    *Reader
	next uint32
	objs []pending
}

type pending struct {
	ref  Ref
	body []byte // serialized object, without "n g obj" / "endobj"
}

// NewUpdate starts an incremental update of the document read by r.
func NewUpdate(r *Reader) *Update {
	u := &Update{r: r}
	next := int64(1)
	if size, ok := r.trailer.Int("Size"); ok && size > next {
		next = size
	}
	for _, s := range r.sections {
		if m := s.maxNum(); m+1 > next {
			next = m + 1
		}
	}
	if r.rec != nil && int64(r.rec.maxNum)+1 > next {
		next = int64(r.rec.maxNum) + 1
	}
	if next > math.MaxUint32-16 {
		next = math.MaxUint32 - 16
	}
	u.next = uint32(next)
	return u
}

func (s *section) maxNum() int64 {
	var m int64
	for _, ss := range s.subs {
		if v := int64(ss.first) + int64(ss.count) - 1; v > m {
			m = v
		}
	}
	for n := range s.lenient {
		if int64(n) > m {
			m = int64(n)
		}
	}
	if s.xs != nil {
		for i := 0; i+1 < len(s.xs.index); i += 2 {
			if v := int64(s.xs.index[i]) + int64(s.xs.index[i+1]) - 1; v > m {
				m = v
			}
		}
	}
	if s.hybrid != nil {
		if v := s.hybrid.maxNum(); v > m {
			m = v
		}
	}
	return m
}

// Alloc reserves a new object number.
func (u *Update) Alloc() Ref {
	ref := Ref{Num: u.next}
	u.next++
	return ref
}

// Put writes object o under ref, which is either allocated by Alloc or an
// existing object of the document (a new version of it).
func (u *Update) Put(ref Ref, o Object) {
	u.objs = append(u.objs, pending{ref: ref, body: AppendObject(nil, o)})
}

// PutStream writes a stream object under ref. /Length is set from data.
func (u *Update) PutStream(ref Ref, d Dict, data []byte) {
	d = d.Clone()
	d.Set("Length", Int(len(data)))
	body := make([]byte, 0, len(data)+len(d)*16+32)
	body = AppendObject(body, d)
	body = append(body, "\nstream\n"...)
	body = append(body, data...)
	body = append(body, "\nendstream"...)
	u.objs = append(u.objs, pending{ref: ref, body: body})
}

// Bytes returns the original file followed by the update. trailer holds the
// entries of the new trailer other than /Size and /Prev (normally /Root,
// /Info and /ID). The cross-reference section is a stream when the newest
// section of the original is one, a classic table otherwise. When the
// original cross-reference data was damaged, the new section indexes every
// object of the file and has no /Prev.
func (u *Update) Bytes(trailer Dict) []byte {
	data := u.r.data
	size := len(data) + 256
	for _, p := range u.objs {
		size += len(p.body) + 40
	}
	out := make([]byte, 0, size+len(u.objs)*24+256)
	out = append(out, data...)
	if n := len(out); n > 0 && out[n-1] != '\n' && out[n-1] != '\r' {
		out = append(out, '\n')
	}

	entries := map[uint32]entry{}
	for _, p := range u.objs {
		entries[p.ref.Num] = entry{typ: 1, gen: p.ref.Gen, off: int64(len(out))}
		out = strconv.AppendUint(out, uint64(p.ref.Num), 10)
		out = append(out, ' ')
		out = strconv.AppendUint(out, uint64(p.ref.Gen), 10)
		out = append(out, " obj\n"...)
		out = append(out, p.body...)
		out = append(out, "\nendobj\n"...)
	}

	full := u.r.Repaired()
	stream := u.r.XrefStream() && !full
	if full {
		// Index every object found in the file; the new objects win.
		rec := u.r.recovery()
		for num, e := range rec.objs {
			if _, ok := entries[num]; ok {
				continue
			}
			if e.inStm {
				entries[num] = entry{typ: 2, off: int64(e.stm), idx: e.idx}
				stream = true
			} else {
				entries[num] = entry{typ: 1, off: e.off}
			}
		}
		entries[0] = entry{typ: 0, gen: 65535}
	}

	t := Dict{}
	maxNum := uint32(0)
	for n := range entries {
		maxNum = max(maxNum, n)
	}
	var xref Ref
	if stream {
		xref = u.Alloc()
		maxNum = max(maxNum, xref.Num)
	}
	sz := int64(maxNum) + 1
	if old, ok := u.r.trailer.Int("Size"); ok && old > sz && !full {
		sz = old
	}
	t.Set("Size", Int(sz))
	for _, kv := range trailer {
		t.Set(kv.Key, kv.Val)
	}
	if !full && u.r.startxref >= 0 {
		t.Set("Prev", Int(u.r.startxref))
	}

	xoff := int64(len(out))
	if stream {
		entries[xref.Num] = entry{typ: 1, off: xoff}
		out = appendXrefStream(out, xref, entries, t)
	} else {
		out = appendXrefTable(out, entries, t)
	}
	out = append(out, "startxref\n"...)
	out = strconv.AppendInt(out, xoff, 10)
	out = append(out, "\n%%EOF\n"...)
	return out
}

// runs groups sorted object numbers into contiguous subsections.
func runs(entries map[uint32]entry) (nums []uint32, subs [][2]uint32) {
	nums = make([]uint32, 0, len(entries))
	for n := range entries {
		nums = append(nums, n)
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })
	for i := 0; i < len(nums); {
		j := i + 1
		for j < len(nums) && nums[j] == nums[j-1]+1 {
			j++
		}
		subs = append(subs, [2]uint32{nums[i], uint32(j - i)})
		i = j
	}
	return nums, subs
}

func appendXrefTable(out []byte, entries map[uint32]entry, t Dict) []byte {
	nums, subs := runs(entries)
	out = append(out, "xref\n"...)
	k := 0
	for _, s := range subs {
		out = strconv.AppendUint(out, uint64(s[0]), 10)
		out = append(out, ' ')
		out = strconv.AppendUint(out, uint64(s[1]), 10)
		out = append(out, '\n')
		for i := uint32(0); i < s[1]; i++ {
			e := entries[nums[k]]
			k++
			off, gen, typ := e.off, int64(e.gen), byte('n')
			if e.typ == 0 {
				off, typ = 0, 'f'
			}
			out = appendPadded(out, off, 10)
			out = append(out, ' ')
			out = appendPadded(out, gen, 5)
			out = append(out, ' ', typ, '\r', '\n')
		}
	}
	out = append(out, "trailer\n"...)
	out = AppendObject(out, t)
	return append(out, '\n')
}

func appendPadded(b []byte, v int64, width int) []byte {
	var buf [20]byte
	s := strconv.AppendInt(buf[:0], v, 10)
	for i := len(s); i < width; i++ {
		b = append(b, '0')
	}
	return append(b, s...)
}

func appendXrefStream(out []byte, xref Ref, entries map[uint32]entry, t Dict) []byte {
	nums, subs := runs(entries)
	var max2, max3 int64
	for _, e := range entries {
		f2, f3 := e.off, int64(e.gen)
		if e.typ == 2 {
			f3 = int64(e.idx)
		}
		max2, max3 = max(max2, f2), max(max3, f3)
	}
	w2, w3 := byteWidth(max2), byteWidth(max3)
	rowLen := 1 + w2 + w3
	data := make([]byte, 0, len(nums)*rowLen)
	for _, n := range nums {
		e := entries[n]
		f2, f3 := e.off, int64(e.gen)
		switch e.typ {
		case 0:
			f2 = 0
		case 2:
			f3 = int64(e.idx)
		}
		data = append(data, e.typ)
		data = appendBE(data, f2, w2)
		data = appendBE(data, f3, w3)
	}
	idx := make(Array, 0, 2*len(subs))
	for _, s := range subs {
		idx = append(idx, Int(s[0]), Int(s[1]))
	}
	d := Dict{{"Type", Name("XRef")}}
	d = append(d, t...)
	d.Set("Index", idx)
	d.Set("W", Array{Int(1), Int(w2), Int(w3)})
	d.Set("Length", Int(len(data)))
	out = strconv.AppendUint(out, uint64(xref.Num), 10)
	out = append(out, " 0 obj\n"...)
	out = AppendObject(out, d)
	out = append(out, "\nstream\n"...)
	out = append(out, data...)
	return append(out, "\nendstream\nendobj\n"...)
}

func byteWidth(v int64) int {
	n := 1
	for v > 0xff {
		v >>= 8
		n++
	}
	return n
}

func appendBE(b []byte, v int64, w int) []byte {
	for i := w - 1; i >= 0; i-- {
		b = append(b, byte(v>>(8*i)))
	}
	return b
}
