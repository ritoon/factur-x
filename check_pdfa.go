package facturx

import (
	"fmt"
	"strconv"
	"time"

	"github.com/ritoon/factur-x/internal/pdf"
	"github.com/ritoon/factur-x/internal/xmp"
)

// Limits of the PDF/A scan.
const (
	maxScanObjects = 200000 // objects visited
	maxScanDepth   = 32     // nesting of page tree nodes, forms, outlines and action chains
	maxReported    = 10     // findings per rule before the rest is summed up
)

// forbiddenActions are the action types PDF/A-2 and PDF/A-3 forbid
// (ISO 19005-3, 6.6.1), with the deprecated set-state and no-op actions.
var forbiddenActions = map[pdf.Name]bool{
	"Launch": true, "Sound": true, "Movie": true, "ResetForm": true, "ImportData": true,
	"Hide": true, "SetOCGState": true, "Rendition": true, "Trans": true, "GoTo3DView": true,
	"JavaScript": true, "SetState": true, "NOP": true,
}

// allowedNamed are the named actions PDF/A allows.
var allowedNamed = map[pdf.Name]bool{"NextPage": true, "PrevPage": true, "FirstPage": true, "LastPage": true}

// Roles in which an object is visited by the scan. The same object may be
// reached in several roles (a widget is both a form field and an
// annotation), each of which is checked once.
const (
	roleNode uint16 = 1 << iota
	roleResources
	roleFont
	roleXObject
	roleStream
	roleAnnot
	roleField
	roleOutline
	roleAction
)

// pdfaScan checks the PDF/A requirements that can be verified without
// rendering the pages: embedded fonts (PDF-31), JavaScript and actions
// (PDF-32 to PDF-34), the other embedded files (PDF-36, PDF-37) and the
// LZWDecode filter (PDF-38). It walks the document catalog, the outlines,
// the form fields and the page tree with the resources of the pages, form
// XObjects, patterns, soft masks, Type 3 fonts and annotation appearances.
// The content streams are not parsed.
type pdfaScan struct {
	c      *container
	k      *containerCheck
	seen   map[pdf.Ref]uint16 // roles in which an indirect object was visited
	visits int
	fonts  map[string]bool // fonts reported, by name and subtype
	files  []*fileSpec     // file specifications of FileAttachment annotations
	counts map[string]int  // findings per rule
}

// pdfa runs the PDF/A scan. invoice is the file specification of the
// invoice attachment, checked by CheckContainer itself; it may be nil.
func (k *containerCheck) pdfa(c *container, invoice *fileSpec) {
	s := &pdfaScan{c: c, k: k, seen: map[pdf.Ref]uint16{}, fonts: map[string]bool{}, counts: map[string]int{}}
	r := c.r
	cat := c.catalog
	if s.present(cat, "AA") {
		s.report("PDF-34", "catalog /AA", "the document catalog has an additional-actions dictionary (/AA)")
	}
	if names, ok := r.Dict(cat.Get("Names")); ok && s.present(names, "JavaScript") {
		s.report("PDF-32", "catalog /Names /JavaScript", "the document has document-level JavaScript (/Names /JavaScript)")
	}
	s.action(cat.Get("OpenAction"), "catalog /OpenAction", 0)
	s.outlines()
	s.fields()
	s.pages()

	s.stream(cat.Get("Metadata"), "catalog /Metadata")
	if intents, ok := r.Resolve(cat.Get("OutputIntents")).(pdf.Array); ok {
		for i, o := range intents {
			if d, ok := r.Dict(o); ok {
				s.stream(d.Get("DestOutputProfile"), fmt.Sprintf("catalog /OutputIntents [%d] /DestOutputProfile", i))
			}
		}
	}
	s.embeddedFiles(invoice)

	for _, id := range []string{"PDF-31", "PDF-32", "PDF-33", "PDF-34", "PDF-36", "PDF-37", "PDF-38"} {
		if n := s.counts[id] - maxReported; n > 0 {
			k.add(id, "", "%d more occurrences not listed", n)
		}
	}
}

// report adds a finding, up to maxReported per rule.
func (s *pdfaScan) report(id, path, format string, args ...any) {
	s.counts[id]++
	if s.counts[id] <= maxReported {
		s.k.add(id, path, format, args...)
	}
}

// visit reports whether o should be visited in role: an indirect object is
// visited once per role, and the scan stops after maxScanObjects visits.
func (s *pdfaScan) visit(o pdf.Object, role uint16) bool {
	if s.visits >= maxScanObjects {
		return false
	}
	if ref, ok := o.(pdf.Ref); ok {
		if s.seen[ref]&role != 0 {
			return false
		}
		s.seen[ref] |= role
	}
	s.visits++
	return true
}

// present reports whether d has a non-null entry key.
func (s *pdfaScan) present(d pdf.Dict, key pdf.Name) bool {
	return s.c.r.Resolve(d.Get(key)) != nil
}

func (s *pdfaScan) name(d pdf.Dict, key pdf.Name) pdf.Name {
	n, _ := s.c.r.Resolve(d.Get(key)).(pdf.Name)
	return n
}

func (s *pdfaScan) text(d pdf.Dict, key pdf.Name) string {
	if t, ok := s.c.r.Resolve(d.Get(key)).(pdf.String); ok {
		return pdf.Text(t)
	}
	return ""
}

// action checks an action and the actions chained to it by /Next.
func (s *pdfaScan) action(o pdf.Object, where string, depth int) {
	if o == nil || depth > maxScanDepth || !s.visit(o, roleAction) {
		return
	}
	r := s.c.r
	if list, ok := r.Resolve(o).(pdf.Array); ok { // /Next may be an array
		for i, x := range list {
			s.action(x, fmt.Sprintf("%s [%d]", where, i), depth+1)
		}
		return
	}
	d, ok := r.Resolve(o).(pdf.Dict)
	if !ok {
		return // a destination
	}
	switch typ := s.name(d, "S"); {
	case forbiddenActions[typ]:
		s.report("PDF-33", where, "the action is a %s action, which PDF/A forbids", typ)
	case typ == "Named":
		if n := s.name(d, "N"); !allowedNamed[n] {
			s.report("PDF-33", where, "the action is the named action %s; PDF/A only allows NextPage, PrevPage, FirstPage and LastPage", n)
		}
	}
	s.action(d.Get("Next"), where+" /Next", depth+1)
}

// outlines checks the actions of the outline items.
func (s *pdfaScan) outlines() {
	r := s.c.r
	root, ok := r.Dict(s.c.catalog.Get("Outlines"))
	if !ok {
		return
	}
	var walk func(o pdf.Object, depth int)
	walk = func(o pdf.Object, depth int) {
		for o != nil && depth <= maxScanDepth && s.visit(o, roleOutline) {
			d, ok := r.Dict(o)
			if !ok {
				return
			}
			s.action(d.Get("A"), fmt.Sprintf("outline item %q /A", s.text(d, "Title")), 0)
			walk(d.Get("First"), depth+1)
			o = d.Get("Next")
		}
	}
	walk(root.Get("First"), 0)
}

// fields checks the additional actions of the form fields. Widget
// annotations, merged with their field or not, are checked with the
// annotations of the pages.
func (s *pdfaScan) fields() {
	r := s.c.r
	form, ok := r.Dict(s.c.catalog.Get("AcroForm"))
	if !ok {
		return
	}
	var walk func(o pdf.Object, depth int)
	walk = func(o pdf.Object, depth int) {
		if depth > maxScanDepth || !s.visit(o, roleField) {
			return
		}
		d, ok := r.Dict(o)
		if !ok {
			return
		}
		if s.name(d, "Subtype") != "Widget" && s.present(d, "AA") {
			s.report("PDF-34", fmt.Sprintf("form field %q /AA", s.text(d, "T")), "the form field has an additional-actions dictionary (/AA)")
		}
		if kids, ok := r.Resolve(d.Get("Kids")).(pdf.Array); ok {
			for _, kid := range kids {
				walk(kid, depth+1)
			}
		}
	}
	if fields, ok := r.Resolve(form.Get("Fields")).(pdf.Array); ok {
		for _, f := range fields {
			walk(f, 0)
		}
	}
}

// pages walks the page tree. Resources are inherited from the ancestors of
// a page.
func (s *pdfaScan) pages() {
	r := s.c.r
	n := 0
	var walk func(o, res pdf.Object, depth int)
	walk = func(o, res pdf.Object, depth int) {
		if depth > maxScanDepth || !s.visit(o, roleNode) {
			return
		}
		d, ok := r.Dict(o)
		if !ok {
			return
		}
		if x := d.Get("Resources"); x != nil {
			res = x
		}
		if kids, ok := r.Resolve(d.Get("Kids")).(pdf.Array); ok && s.name(d, "Type") != "Page" {
			for _, kid := range kids {
				walk(kid, res, depth+1)
			}
			return
		}
		n++
		where := fmt.Sprintf("page %d", n)
		if s.present(d, "AA") {
			s.report("PDF-34", where+" /AA", "the page has an additional-actions dictionary (/AA)")
		}
		switch contents := d.Get("Contents"); v := r.Resolve(contents).(type) {
		case *pdf.Stream:
			s.stream(contents, where+" /Contents")
		case pdf.Array:
			for i, x := range v {
				s.stream(x, fmt.Sprintf("%s /Contents [%d]", where, i))
			}
		}
		s.resources(res, where+" /Resources", 0)
		if annots, ok := r.Resolve(d.Get("Annots")).(pdf.Array); ok {
			for i, a := range annots {
				s.annotation(a, fmt.Sprintf("%s /Annots [%d]", where, i))
			}
		}
	}
	walk(s.c.catalog.Get("Pages"), nil, 0)
}

// annotation checks the actions and appearance streams of an annotation,
// and collects the embedded file of a file attachment annotation.
func (s *pdfaScan) annotation(o pdf.Object, where string) {
	if !s.visit(o, roleAnnot) {
		return
	}
	r := s.c.r
	d, ok := r.Dict(o)
	if !ok {
		return
	}
	if s.present(d, "AA") {
		s.report("PDF-34", where+" /AA", "the annotation has an additional-actions dictionary (/AA)")
	}
	s.action(d.Get("A"), where+" /A", 0)
	if ap, ok := r.Dict(d.Get("AP")); ok {
		for _, key := range []pdf.Name{"N", "R", "D"} {
			x := ap.Get(key)
			if _, ok := r.Resolve(x).(*pdf.Stream); ok {
				s.xobject(x, where+" /AP /"+string(key), 0)
			} else if states, ok := r.Resolve(x).(pdf.Dict); ok {
				for _, st := range states {
					s.xobject(st.Val, where+" /AP /"+string(key)+" /"+string(st.Key), 0)
				}
			}
		}
	}
	if s.name(d, "Subtype") == "FileAttachment" {
		fsObj := d.Get("FS")
		if fs, ok := r.Resolve(fsObj).(pdf.Dict); ok && fs.Has("EF") {
			ref, _ := fsObj.(pdf.Ref)
			s.files = append(s.files, &fileSpec{ref: ref, dict: fs, name: specName(r, fs)})
		}
	}
}

// resources checks the fonts, XObjects, patterns, shadings and soft masks of
// a resource dictionary.
func (s *pdfaScan) resources(o pdf.Object, where string, depth int) {
	if o == nil || depth > maxScanDepth || !s.visit(o, roleResources) {
		return
	}
	r := s.c.r
	d, ok := r.Dict(o)
	if !ok {
		return
	}
	if fonts, ok := r.Dict(d.Get("Font")); ok {
		for _, f := range fonts {
			s.font(f.Val, where+" /Font /"+string(f.Key), depth)
		}
	}
	if xobjects, ok := r.Dict(d.Get("XObject")); ok {
		for _, x := range xobjects {
			s.xobject(x.Val, where+" /XObject /"+string(x.Key), depth)
		}
	}
	if patterns, ok := r.Dict(d.Get("Pattern")); ok {
		for _, p := range patterns {
			path := where + " /Pattern /" + string(p.Key)
			switch v := r.Resolve(p.Val).(type) {
			case *pdf.Stream: // tiling pattern
				s.xobject(p.Val, path, depth)
			case pdf.Dict: // shading pattern
				s.stream(v.Get("Shading"), path+" /Shading")
			}
		}
	}
	if shadings, ok := r.Dict(d.Get("Shading")); ok {
		for _, sh := range shadings {
			s.stream(sh.Val, where+" /Shading /"+string(sh.Key))
		}
	}
	if states, ok := r.Dict(d.Get("ExtGState")); ok {
		for _, gs := range states {
			if g, ok := r.Dict(gs.Val); ok {
				if mask, ok := r.Resolve(g.Get("SMask")).(pdf.Dict); ok {
					s.xobject(mask.Get("G"), where+" /ExtGState /"+string(gs.Key)+" /SMask /G", depth)
				}
			}
		}
	}
}

// xobject checks a form or image XObject, a tiling pattern or an
// appearance stream: its filters and, for a content stream, its resources.
func (s *pdfaScan) xobject(o pdf.Object, where string, depth int) {
	st, ok := s.c.r.Resolve(o).(*pdf.Stream)
	if !ok {
		return
	}
	s.stream(o, where)
	if depth > maxScanDepth || !s.visit(o, roleXObject) {
		return
	}
	if s.name(st.Dict, "Subtype") == "Image" {
		s.stream(st.Dict.Get("SMask"), where+" /SMask")
		s.stream(st.Dict.Get("Mask"), where+" /Mask")
		return
	}
	s.resources(st.Dict.Get("Resources"), where+" /Resources", depth+1)
}

// font checks that a font embeds its font program. Type 3 fonts, whose
// glyphs are content streams, have none: their resources are checked
// instead.
func (s *pdfaScan) font(o pdf.Object, where string, depth int) {
	if !s.visit(o, roleFont) {
		return
	}
	r := s.c.r
	d, ok := r.Dict(o)
	if !ok {
		return
	}
	sub := s.name(d, "Subtype")
	desc := d
	switch sub {
	case "Type3":
		s.resources(d.Get("Resources"), where+" /Resources", depth+1)
		return
	case "Type0":
		desc = nil
		if list, ok := r.Resolve(d.Get("DescendantFonts")).(pdf.Array); ok && len(list) > 0 {
			desc, _ = r.Dict(list[0])
		}
	}
	fd, ok := r.Dict(desc.Get("FontDescriptor"))
	if ok {
		for _, key := range []pdf.Name{"FontFile", "FontFile2", "FontFile3"} {
			if _, ok := r.Resolve(fd.Get(key)).(*pdf.Stream); ok {
				s.stream(fd.Get(key), where+" /FontDescriptor /"+string(key))
				return
			}
		}
	}
	name := string(s.name(d, "BaseFont"))
	if s.fonts[name+" "+string(sub)] {
		return
	}
	s.fonts[name+" "+string(sub)] = true
	if !ok {
		s.report("PDF-31", where, "font %q (%s) is not embedded: it has no font descriptor", name, sub)
		return
	}
	s.report("PDF-31", where, "font %q (%s) is not embedded: its font descriptor has no /FontFile, /FontFile2 or /FontFile3", name, sub)
}

// stream checks the filters of a stream: PDF/A forbids LZWDecode.
func (s *pdfaScan) stream(o pdf.Object, where string) {
	r := s.c.r
	st, ok := r.Resolve(o).(*pdf.Stream)
	if !ok || !s.visit(o, roleStream) {
		return
	}
	var filters pdf.Array
	switch f := r.Resolve(st.Dict.Get("Filter")).(type) {
	case pdf.Name:
		filters = pdf.Array{f}
	case pdf.Array:
		filters = f
	}
	for _, f := range filters {
		if n, _ := r.Resolve(f).(pdf.Name); n == "LZWDecode" || n == "LZW" {
			s.report("PDF-38", where, "the stream uses the LZWDecode filter, which PDF/A forbids")
			return
		}
	}
}

// embeddedFiles checks the embedded files besides the invoice: an
// /AFRelationship and a MIME type, as PDF/A-3 requires for every embedded
// file. The filters of every embedded file stream are checked too.
func (s *pdfaScan) embeddedFiles(invoice *fileSpec) {
	c := s.c
	r := c.r
	c.collectPages()
	files := append(append([]*fileSpec(nil), c.specs...), s.files...)
	var seen []*fileSpec
	for _, fs := range files {
		dup := false
		for _, o := range seen {
			if o == fs || (fs.ref != pdf.Ref{} && o.ref == fs.ref) || (fs.ref == pdf.Ref{} && sameDict(o.dict, fs.dict)) {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		seen = append(seen, fs)
		path := specPath(fs)
		if ef, ok := r.Dict(fs.dict.Get("EF")); ok {
			for _, key := range []pdf.Name{"F", "UF"} {
				s.stream(ef.Get(key), path+" /EF /"+string(key))
			}
		}
		if invoice != nil && (fs == invoice || (fs.ref != pdf.Ref{} && fs.ref == invoice.ref)) {
			continue
		}
		name := strconv.Quote(fs.filename())
		if s.name(fs.dict, "AFRelationship") == "" {
			s.report("PDF-36", path+" /AFRelationship", "the file specification of the embedded file %s has no /AFRelationship", name)
		}
		if st, ok := c.embeddedStream(fs); ok && s.name(st.Dict, "Subtype") == "" {
			s.report("PDF-37", "embedded file "+name+" /Subtype", "the embedded file %s has no MIME type (/Subtype)", name)
		}
	}
}

// docInfo checks that the entries of the document information dictionary
// that have an XMP equivalent have the same value in the XMP metadata
// (ISO 19005-3, 6.6.2.3.1). An empty entry needs no equivalent.
func (k *containerCheck) docInfo(c *container, info *xmp.Info) {
	d, ok := c.r.Dict(c.r.Trailer().Get("Info"))
	if !ok {
		return
	}
	for _, e := range xmp.InfoEquivalents {
		s, ok := c.r.Resolve(d.Get(pdf.Name(e.Entry))).(pdf.String)
		if !ok {
			continue
		}
		v := pdf.Text(s)
		x, has := info.InfoProps[e.Entry]
		path := "trailer /Info /" + e.Entry
		switch {
		case !has:
			if v != "" {
				k.add("PDF-35", path, "/Info /%s is %q, but the XMP metadata has no %s", e.Entry, v, e.Property)
			}
		case e.Entry == "CreationDate" || e.Entry == "ModDate":
			if !sameDate(v, x) {
				k.add("PDF-35", path, "/Info /%s is %q, XMP %s is %q: the dates differ", e.Entry, v, e.Property, x)
			}
		case v != x:
			k.add("PDF-35", path, "/Info /%s is %q, XMP %s is %q", e.Entry, v, e.Property, x)
		}
	}
}

// sameDate reports whether a PDF date and an XMP date are the same instant,
// to the second.
func sameDate(pdfDate, xmpDate string) bool {
	a, ok := pdf.ParseDate(pdfDate)
	if !ok {
		return false
	}
	b, ok := xmp.ParseDate(xmpDate)
	return ok && a.Equal(b.Truncate(time.Second))
}
