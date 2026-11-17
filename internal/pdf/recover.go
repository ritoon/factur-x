package pdf

import (
	"bytes"
	"math"
	"sort"
)

// recovery is an object index rebuilt by scanning the whole file for
// "n g obj" headers, used when the cross-reference data is missing, damaged
// or points to wrong offsets.
type recovery struct {
	objs    map[uint32]recEntry
	trailer Dict
	maxNum  uint32
}

// recEntry locates an object found by the scan. pos orders definitions:
// a later definition replaces an earlier one, as an incremental update would.
type recEntry struct {
	pos   int64
	off   int64 // offset of the object header (direct objects)
	inStm bool
	stm   uint32 // object stream number (compressed objects)
	idx   uint32 // index in the object stream
}

// recovery returns the scanned index, building it on first use.
func (r *Reader) recovery() *recovery {
	if r.rec != nil {
		return r.rec
	}
	rec := &recovery{objs: map[uint32]recEntry{}}
	r.rec = rec
	data := r.data

	type stmAt struct {
		num uint32
		pos int64
		st  *Stream
	}
	type dictAt struct {
		pos int64
		d   Dict
	}
	var stms []stmAt
	var trailers []dictAt

	for i := 0; i < len(data); {
		k := bytes.Index(data[i:], []byte("obj"))
		if k < 0 {
			break
		}
		at := i + k
		i = at + 3
		if at+3 < len(data) && isRegular(data[at+3]) {
			continue
		}
		off, num, ok := objHeaderBefore(data, at)
		if !ok {
			continue
		}
		o, end := scanObject(data, at+3)
		if end < 0 {
			continue
		}
		rec.objs[num] = recEntry{pos: int64(off), off: int64(off)}
		if num > rec.maxNum {
			rec.maxNum = num
		}
		if st, ok := o.(*Stream); ok {
			switch t, _ := st.Dict.Name("Type"); t {
			case "ObjStm":
				stms = append(stms, stmAt{num, int64(off), st})
			case "XRef":
				trailers = append(trailers, dictAt{int64(off), st.Dict})
			}
		}
		if end > i {
			i = end
		}
	}

	for i := 0; i < len(data); {
		k := bytes.Index(data[i:], []byte("trailer"))
		if k < 0 {
			break
		}
		at := i + k
		i = at + 7
		p := parser{b: data, pos: i, r: r}
		if o, err := p.object(); err == nil {
			if d, ok := o.(Dict); ok {
				trailers = append(trailers, dictAt{int64(at), d})
			}
		}
	}
	sort.SliceStable(trailers, func(a, b int) bool { return trailers[a].pos < trailers[b].pos })
	for _, t := range trailers {
		for _, key := range []Name{"Root", "Info", "ID", "Encrypt"} {
			if v := t.d.Get(key); v != nil {
				rec.trailer.Set(key, v)
			}
		}
	}

	// Objects stored in object streams, unless a later direct definition
	// exists.
	for _, s := range stms {
		os := r.decodeObjStm(s.st)
		if os == nil {
			continue
		}
		if r.stms == nil {
			r.stms = map[uint32]*objStm{}
		}
		r.stms[s.num] = os
		for idx, n := range os.nums {
			if e, ok := rec.objs[n]; ok && e.pos > s.pos {
				continue
			}
			rec.objs[n] = recEntry{pos: s.pos, inStm: true, stm: s.num, idx: uint32(idx)}
			if n > rec.maxNum {
				rec.maxNum = n
			}
		}
	}
	return rec
}

// objHeaderBefore checks that the "obj" keyword at index at is preceded by
// "num gen" and returns the offset of num.
func objHeaderBefore(b []byte, at int) (off int, num uint32, ok bool) {
	j := at - 1
	for j >= 0 && class[b[j]] == white {
		j--
	}
	if j == at-1 {
		return 0, 0, false
	}
	end := j + 1
	for j >= 0 && isDigit(b[j]) {
		j--
	}
	if j+1 == end || end-j-1 > 5 || j < 0 || class[b[j]] != white {
		return 0, 0, false
	}
	for j >= 0 && class[b[j]] == white {
		j--
	}
	end = j + 1
	for j >= 0 && isDigit(b[j]) {
		j--
	}
	if j+1 == end || end-j-1 > 10 || (j >= 0 && isRegular(b[j])) {
		return 0, 0, false
	}
	var n uint64
	for _, c := range b[j+1 : end] {
		n = n*10 + uint64(c-'0')
	}
	if n == 0 || n > math.MaxUint32 {
		return 0, 0, false
	}
	return j + 1, uint32(n), true
}

// scanObject parses the object body starting at pos (just after "obj")
// without resolving anything, and returns the object and the offset where
// it ends (-1 when it cannot be parsed).
func scanObject(b []byte, pos int) (Object, int) {
	p := parser{b: b, pos: pos}
	o, err := p.object()
	if err != nil {
		return nil, -1
	}
	if _, ok := o.(keyword); ok {
		return nil, p.pos
	}
	d, ok := o.(Dict)
	if !ok {
		return o, p.pos
	}
	p.skip()
	if !bytes.HasPrefix(b[p.pos:], []byte("stream")) {
		return d, p.pos
	}
	length := int64(-1)
	if n, ok := d.Get("Length").(Int); ok {
		length = int64(n)
	}
	start, end := streamExtent(b, p.pos, length)
	st := &Stream{Dict: d, Raw: b[start:end:end]}
	next := end
	if i := bytes.Index(b[end:], []byte("endstream")); i >= 0 && i < 16 {
		next = end + i + 9
	}
	return st, next
}

// rebuild switches the reader to the scanned index.
func (r *Reader) rebuild() {
	r.recOnly = true
	r.cache = nil
	rec := r.recovery()
	t := rec.trailer.Clone()
	if _, ok := r.Resolve(t.Get("Root")).(Dict); !ok {
		// Look for the catalog itself, the latest one in the file.
		best, bestPos := uint32(0), int64(-1)
		for num, e := range rec.objs {
			if e.pos < bestPos {
				continue
			}
			if d, ok := r.Object(num).(Dict); ok {
				if typ, _ := d.Name("Type"); typ == "Catalog" {
					best, bestPos = num, e.pos
				}
			}
		}
		if bestPos >= 0 {
			t.Set("Root", Ref{Num: best})
		}
	}
	r.trailer = t
}
