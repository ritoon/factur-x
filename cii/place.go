package cii

import (
	"strings"

	"github.com/ritoon/factur-x/internal/schema"
	"github.com/ritoon/factur-x/invoice"
)

// This file puts the extensions of an invoice (see invoice.Extension) back
// into the document written by Marshal.
//
// When the invoice holds extensions, the writer tracks the open elements
// (frames) and the children written in each. The writer declares, right after
// the start tag of the element of a value that has an Extensions field, the
// extensions of that value (hold). When an element is closed, the extensions
// whose path designates it are inserted among its children at the position
// that the schema of the target profile gives them; the containers that the
// writer did not write are created. An extension that the target schema does
// not allow there, or that would repeat an element beyond its maximum number
// of occurrences, is reported as lost.

// frame is an element being written.
type frame struct {
	local   string      // local name
	typ     schema.Type // type in the target schema (invalid when unknown)
	at      int         // offset of the start tag, indentation included
	nameEnd int         // offset right after the name in the start tag
	body    int         // offset right after the start tag
	kids    []kid       // children written so far, in document order
	holder  int         // index of the frame of the nearest holder, -1 if none

	// For the element of a value with extensions:
	exts    []invoice.Extension
	placed  []bool
	pending int
}

// kid is a child element written in a frame.
type kid struct {
	local   string
	at      int // offset of the start tag, indentation included
	nameEnd int // offset right after the name in the start tag
	pos     int // position in the schema of the parent, -1 if unknown
}

// trackStart records the start tag of a child element written at offset at
// (indentation included), whose name ends at nameEnd, and opens its frame
// when it is a container.
func (w *writer) trackStart(name string, at, nameEnd int, container bool) {
	local := name
	if i := strings.IndexByte(name, ':'); i >= 0 {
		local = name[i+1:]
	}
	var typ schema.Type
	holder := -1
	if n := len(w.frames); n > 0 {
		p := &w.frames[n-1]
		typ = p.typ.Child(local)
		pos := -1
		if typ.Valid() {
			pos = typ.Pos()
		}
		p.kids = append(p.kids, kid{local: local, at: at, nameEnd: nameEnd, pos: pos})
		holder = p.holder
	} else {
		typ = schema.Root(w.target)
	}
	if !container {
		return
	}
	if cap(w.frames) > len(w.frames) {
		// Reuse the child list of a frame closed earlier at this depth.
		w.frames = w.frames[:len(w.frames)+1]
		f := &w.frames[len(w.frames)-1]
		*f = frame{kids: f.kids[:0]}
	} else {
		w.frames = append(w.frames, frame{})
	}
	f := &w.frames[len(w.frames)-1]
	f.local, f.typ, f.at, f.nameEnd, f.holder = local, typ, at, nameEnd, holder
}

// hold declares the extensions of the value whose element was just started.
func (w *writer) hold(exts []invoice.Extension) {
	if !w.track || len(exts) == 0 {
		return
	}
	i := len(w.frames) - 1
	f := &w.frames[i]
	f.holder, f.exts, f.pending = i, exts, len(exts)
	f.placed = make([]bool, len(exts))
}

// closeFrame inserts the extensions designating the innermost open element,
// before its end tag is written. popFrame or dropFrame closes the frame
// then.
func (w *writer) closeFrame() {
	fi := len(w.frames) - 1
	if hi := w.frames[fi].holder; hi >= 0 && w.frames[hi].pending > 0 && w.concerns(hi, fi) {
		rel := w.relPath(hi, fi)
		w.placeAttrs(fi, hi, rel)
		w.placeElements(fi, hi, rel)
		w.placeWrappers(fi, hi, rel)
		if hi == fi {
			h := &w.frames[hi]
			for i := range h.exts {
				if !h.placed[i] {
					w.loseExt(hi, i)
				}
			}
		}
	}
}

// popFrame closes the frame of the innermost open element.
func (w *writer) popFrame() { w.frames = w.frames[:len(w.frames)-1] }

// dropFrame closes the frame of an element that endOpt removes, and forgets
// the element in its parent.
func (w *writer) dropFrame() {
	w.frames = w.frames[:len(w.frames)-1]
	if n := len(w.frames); n > 0 {
		p := &w.frames[n-1]
		p.kids = p.kids[:len(p.kids)-1]
	}
}

// concerns reports whether an extension of holder frame hi that is not
// placed yet lies in frame fi or below it, without building the path of fi.
func (w *writer) concerns(hi, fi int) bool {
	if hi == fi {
		return true // pending > 0
	}
	h := &w.frames[hi]
	for i := range h.exts {
		if h.placed[i] {
			continue
		}
		p := h.exts[i].Path
		ok := true
		for j := hi + 1; j <= fi && ok; j++ {
			if j > hi+1 {
				if ok = p != "" && p[0] == '/'; ok {
					p = p[1:]
				}
			}
			l := w.frames[j].local
			if ok = strings.HasPrefix(p, l); ok {
				p = p[len(l):]
			}
		}
		if ok && (p == "" || p[0] == '/') {
			return true
		}
	}
	return false
}

// relPath returns the path of frame fi relative to the holder frame hi.
func (w *writer) relPath(hi, fi int) string {
	if hi == fi {
		return ""
	}
	var b strings.Builder
	for i := hi + 1; i <= fi; i++ {
		if i > hi+1 {
			b.WriteByte('/')
		}
		b.WriteString(w.frames[i].local)
	}
	return b.String()
}

// loseExt reports extension i of holder frame hi as lost.
func (w *writer) loseExt(hi, i int) {
	h := &w.frames[hi]
	h.placed[i] = true
	h.pending--
	var b strings.Builder
	b.WriteString("EXT ")
	for j := 1; j <= hi; j++ {
		b.WriteString(w.frames[j].local)
		b.WriteByte('/')
	}
	b.WriteString(h.exts[i].String())
	w.lose(b.String())
}

func (w *writer) done(hi, i int) {
	h := &w.frames[hi]
	h.placed[i] = true
	h.pending--
}

// placeAttrs inserts the attribute extensions of holder hi that belong to
// frame fi (path rel) or to one of its children.
func (w *writer) placeAttrs(fi, hi int, rel string) {
	h := &w.frames[hi]
	for i := range h.exts {
		e := &h.exts[i]
		if h.placed[i] || !e.IsAttr() {
			continue
		}
		f := &w.frames[fi]
		if e.Path == rel {
			// An attribute of the element itself.
			if w.extOK && f.typ.HasAttr(e.Attr) && !hasAttr(w.b[f.at:f.body], e.Attr) {
				n := w.insertAttr(f.body-1, e.Attr, e.Value)
				f.body += n
				for k := range f.kids {
					f.kids[k].at += n
					f.kids[k].nameEnd += n
				}
				w.done(hi, i)
			} else {
				w.loseExt(hi, i)
			}
			continue
		}
		parent, name, occ := splitAttrPath(e.Path)
		if parent != rel {
			continue
		}
		k := -1
		for j, seen := range f.kids {
			if seen.local == name {
				if occ--; occ == 0 {
					k = j
					break
				}
			}
		}
		if k < 0 {
			continue // not written: lost when the holder closes
		}
		end := len(w.b)
		if k+1 < len(f.kids) {
			end = f.kids[k+1].at
		}
		if tag := startTag(w.b[f.kids[k].at:end]); w.extOK && f.typ.Child(name).HasAttr(e.Attr) && !hasAttr(tag, e.Attr) {
			n := w.insertAttr(f.kids[k].at+len(tag), e.Attr, e.Value)
			for j := k + 1; j < len(f.kids); j++ {
				f.kids[j].at += n
				f.kids[j].nameEnd += n
			}
			w.done(hi, i)
		} else {
			w.loseExt(hi, i)
		}
	}
}

// startTag returns the start tag at the beginning of b.
func startTag(b []byte) []byte {
	for i, c := range b {
		if c == '>' {
			return b[:i]
		}
	}
	return b
}

// hasAttr reports whether a start tag carries the attribute name.
func hasAttr(tag []byte, name string) bool {
	s := string(startTag(tag))
	for i := 0; ; {
		j := strings.Index(s[i:], " "+name+"=")
		if j < 0 {
			return false
		}
		if quotes := strings.Count(s[:i+j], `"`); quotes%2 == 0 {
			return true
		}
		i += j + 1
	}
}

// insertAttr inserts ` name="value"` at offset at, the end of a start tag,
// and returns its length.
func (w *writer) insertAttr(at int, name, value string) int {
	s := len(w.b)
	w.attr(name, value)
	n := len(w.b) - s
	w.moveTail(s, at)
	return n
}

// moveTail moves the bytes written from offset s to offset p (p <= s),
// shifting the bytes in between.
func (w *writer) moveTail(s, p int) {
	if p >= s {
		return
	}
	w.tmp = append(w.tmp[:0], w.b[s:]...)
	copy(w.b[p+len(w.tmp):], w.b[p:s])
	copy(w.b[p:], w.tmp)
}

// placeElements inserts the element extensions of holder hi whose parent is
// frame fi (path rel).
func (w *writer) placeElements(fi, hi int, rel string) {
	h := &w.frames[hi]
	for i := range h.exts {
		e := &h.exts[i]
		if h.placed[i] || e.IsAttr() || e.Path != rel {
			continue
		}
		// The element is parsed and written again: it is well-formed and
		// follows the indentation of the document.
		n, err := parseExtension(e)
		if err != nil {
			w.loseExt(hi, i)
			continue
		}
		ct := w.frames[fi].typ.Child(n.Name)
		if !w.extOK || !ct.Valid() || ct.Space() != n.Space || w.full(fi, n.Name, ct) {
			w.loseExt(hi, i)
			continue
		}
		s := len(w.b)
		w.b = appendElement(w.b, n, w.indent, w.depth)
		w.insertKid(fi, n.Name, ct.Pos(), s)
		w.done(hi, i)
	}
}

// full reports whether frame fi already holds the maximum number of child
// elements named local.
func (w *writer) full(fi int, local string, ct schema.Type) bool {
	if ct.Max() < 0 {
		return false
	}
	n := 0
	for _, k := range w.frames[fi].kids {
		if k.local == local {
			n++
		}
	}
	return n >= ct.Max()
}

// insertKid moves the child element written from offset s to the end of the
// buffer into its schema position in frame fi, and records it.
func (w *writer) insertKid(fi int, local string, pos, s int) {
	f := &w.frames[fi]
	j := len(f.kids)
	for k := range f.kids {
		if f.kids[k].pos > pos {
			j = k
			break
		}
	}
	p := s
	if j < len(f.kids) {
		p = f.kids[j].at
	}
	n := len(w.b) - s
	w.moveTail(s, p)
	for k := j; k < len(f.kids); k++ {
		f.kids[k].at += n
		f.kids[k].nameEnd += n
	}
	nameEnd := tagNameEnd(w.b, p)
	f.kids = append(f.kids, kid{})
	copy(f.kids[j+1:], f.kids[j:])
	f.kids[j] = kid{local: local, at: p, nameEnd: nameEnd, pos: pos}
}

// tagNameEnd returns the offset right after the name of the first start tag
// found from offset p.
func tagNameEnd(b []byte, p int) int {
	for p < len(b) && b[p] != '<' {
		p++
	}
	for p++; p < len(b); p++ {
		switch b[p] {
		case ' ', '>', '/', '\t', '\n', '\r':
			return p
		}
	}
	return p
}

// placeWrappers creates, in frame fi (path rel), the containers that the
// writer did not write and that hold extensions of holder hi.
func (w *writer) placeWrappers(fi, hi int, rel string) {
	prefix := rel + "/"
	if rel == "" {
		prefix = ""
	}
	for {
		h := &w.frames[hi]
		seg := ""
		for i := range h.exts {
			e := &h.exts[i]
			if h.placed[i] || !strings.HasPrefix(e.Path, prefix) || e.Path == rel {
				continue
			}
			rest := e.Path[len(prefix):]
			if e.IsAttr() && !strings.Contains(rest, "/") {
				continue // an attribute of a child: placed by placeAttrs or lost
			}
			seg, _, _ = strings.Cut(rest, "/")
			break
		}
		if seg == "" {
			return
		}
		ct := w.frames[fi].typ.Child(seg)
		pfx := nsPrefix(ct.Space())
		if !w.extOK || !ct.Valid() || ct.Simple() || pfx == "" || w.full(fi, seg, ct) {
			h := &w.frames[hi]
			for i := range h.exts {
				if p := h.exts[i].Path; !h.placed[i] && (p == prefix+seg || strings.HasPrefix(p, prefix+seg+"/")) {
					w.loseExt(hi, i)
				}
			}
			continue
		}
		s := len(w.b)
		name := pfx + ":" + seg
		m := w.start(name)
		w.endOpt(name, m)
		if len(w.b) > s {
			// start recorded the wrapper as the last child: move it into
			// place.
			f := &w.frames[fi]
			f.kids = f.kids[:len(f.kids)-1]
			w.insertKid(fi, seg, ct.Pos(), s)
		}
		// Whatever could not be placed inside the wrapper is lost.
		h = &w.frames[hi]
		for i := range h.exts {
			if p := h.exts[i].Path; !h.placed[i] && (p == prefix+seg || strings.HasPrefix(p, prefix+seg+"/") || strings.HasPrefix(p, prefix+seg+"[")) {
				w.loseExt(hi, i)
			}
		}
	}
}
