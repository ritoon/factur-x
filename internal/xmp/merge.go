package xmp

import (
	"bytes"
	"sort"
	"strings"
	"time"
)

// Params are the values Merge writes.
type Params struct {
	// NamespaceURI of the hybrid-invoice schema; NSFacturX when empty.
	NamespaceURI     string
	DocumentType     string // fx:DocumentType, "INVOICE"
	DocumentFileName string // fx:DocumentFileName, "factur-x.xml"
	Version          string // fx:Version, "1.0"
	ConformanceLevel string // fx:ConformanceLevel, "EN 16931"
	// PDFAPart is the PDF/A part to declare: "3" or "4". When it is empty
	// (or another value), a packet that declares part 4 keeps it and any
	// other packet gets part 3.
	PDFAPart string
	// PDFAConformance is the conformance level written when the packet has
	// none that is valid for the part declared (A, B or U for PDF/A-3, F or E
	// for PDF/A-4, the levels that allow embedded files). When it is empty or
	// not valid either: "B" for PDF/A-3, "F" for PDF/A-4. A valid existing
	// level is kept.
	PDFAConformance string
	// Date, when not zero, is written to xmp:ModifyDate and
	// xmp:MetadataDate.
	Date time.Time
	// DocInfo holds the entries of the document information dictionary.
	// Those that have an XMP equivalent missing from the packet are added,
	// as PDF/A requires both to agree.
	DocInfo DocInfo
	// PDFAOnly writes the PDF/A identification, the dates and the entries
	// of DocInfo, without the hybrid-invoice properties and their extension
	// schema: those of the packet, if any, are kept as they are. The other
	// hybrid-invoice fields of Params are ignored.
	PDFAOnly bool
}

// DocInfo holds the entries of a PDF document information dictionary.
type DocInfo struct {
	Title, Author, Subject, Keywords, Creator, Producer string
	CreationDate                                        time.Time
}

// FormatDate formats t as an XMP date ("2026-11-16T14:30:00+01:00").
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// edit replaces data[start:end] with text.
type edit struct {
	start, end int
	text       string
}

// Merge returns an XMP packet holding the properties of existing (which may
// be empty), with the PDF/A identification of a hybrid invoice (part 3, or
// part 4 with its revision, see Params.PDFAPart), the hybrid-invoice
// properties of p, their PDF/A extension schema description, and the dates
// of p.
//
// Properties of the hybrid-invoice namespaces, pdfaid:part,
// pdfaid:conformance, pdfaid:rev, xmp:ModifyDate and xmp:MetadataDate are
// replaced, as are the extension schema descriptions of the hybrid-invoice
// namespaces. An existing pdfaid:rev is kept for part 4; part 4 without one
// gets "2020", the edition of ISO 19005-4. With Params.PDFAOnly, the
// hybrid-invoice properties and their schema description are left alone.
// Everything else is kept byte for byte. A packet that cannot be parsed is
// replaced by a new one.
func Merge(existing []byte, p Params) []byte {
	if p.NamespaceURI == "" {
		p.NamespaceURI = NSFacturX
	}
	if len(bytes.TrimSpace(existing)) > 0 {
		data := toUTF8(existing)
		if root, err := parse(data); err == nil {
			if rdf := findRDF(root); rdf != nil {
				if out, ok := mergeInto(data, rdf, p); ok {
					return out
				}
			}
		}
	}
	return fresh(p)
}

// fresh builds a new packet.
func fresh(p Params) []byte {
	var b strings.Builder
	b.WriteString("<?xpacket begin=\"\uFEFF\" id=\"W5M0MpCehiHzreSzNTczkc9d\"?>\n")
	b.WriteString("<x:xmpmeta xmlns:x=\"adobe:ns:meta/\">\n")
	b.WriteString("  <rdf:RDF xmlns:rdf=\"" + nsRDF + "\">\n")
	present := map[string]bool{}
	writeDescriptions(&b, "rdf", "", identify(pdfaID{}, p), p, present, true)
	b.WriteString("  </rdf:RDF>\n</x:xmpmeta>\n<?xpacket end=\"w\"?>")
	return []byte(b.String())
}

// managed reports whether Merge replaces a property; the hybrid-invoice
// properties are replaced unless pdfaOnly is set.
func managed(space, local string, pdfaOnly bool) bool {
	switch space {
	case nsPDFAID:
		return local == "part" || local == "conformance" || local == "rev"
	case nsXMP:
		return local == "ModifyDate" || local == "MetadataDate"
	}
	return !pdfaOnly && isInvoiceNS(space)
}

func mergeInto(data []byte, rdf *node, p Params) ([]byte, bool) {
	rdfPrefix := string(rdf.prefix)
	if rdfPrefix == "" {
		return nil, false
	}
	var (
		edits   []edit
		about   string
		gotAbt  bool
		found   pdfaID // PDF/A identification of the packet
		bag     *node  // pdfaExtension:schemas container where the fx schema goes
		present = map[string]bool{}
	)
	for _, d := range rdf.children {
		if !d.is(nsRDF, "Description") {
			continue
		}
		if a, ok := d.attr(nsRDF, "about"); ok && !gotAbt {
			about, gotAbt = a, true
		}
		var dEdits []edit
		kept := 0
		for _, a := range d.attrs {
			if a.space == nsRDF || a.space == nsXML || a.space == "" {
				continue
			}
			local := string(a.local)
			if a.space == nsPDFAID {
				found.set(local, unescape(a.value))
			}
			if managed(a.space, local, p.PDFAOnly) {
				dEdits = append(dEdits, edit{start: trimLeft(data, a.start), end: a.end})
				continue
			}
			present[a.space+" "+local] = true
			kept++
		}
		for _, c := range d.children {
			local := string(c.local)
			if c.space == nsPDFAID {
				found.set(local, c.text(data))
			}
			if managed(c.space, local, p.PDFAOnly) {
				dEdits = append(dEdits, edit{start: trimLeft(data, c.start), end: c.end})
				continue
			}
			present[c.space+" "+local] = true
			kept++
			if c.is(nsPDFAExtension, "schemas") {
				for _, b := range c.children {
					if !b.is(nsRDF, "Bag") && !b.is(nsRDF, "Seq") {
						continue
					}
					for _, li := range b.children {
						if li.is(nsRDF, "li") && !p.PDFAOnly && isInvoiceNS(schemaNS(data, li)) {
							dEdits = append(dEdits, edit{start: trimLeft(data, li.start), end: li.end})
						}
					}
					if bag == nil {
						bag = b
					}
				}
			}
		}
		if kept == 0 {
			// Nothing left: drop the whole description.
			edits = append(edits, edit{start: trimLeft(data, d.start), end: d.end})
		} else {
			edits = append(edits, dEdits...)
		}
	}
	var b strings.Builder
	b.WriteString("\n")
	writeDescriptions(&b, rdfPrefix, about, identify(found, p), p, present, bag == nil)
	ins := strings.TrimSuffix(b.String(), "\n") + indentBefore(data, rdf.endStart)
	edits = append(edits, edit{start: trimLeft(data, rdf.endStart), end: rdf.endStart, text: ins})
	if bag != nil && !p.PDFAOnly {
		var li strings.Builder
		writeSchemaLI(&li, rdfPrefix, p.NamespaceURI, true)
		if bag.endStart == bag.end {
			// Self-closing <rdf:Bag/>.
			name := qname(bag)
			edits = append(edits, edit{start: bag.start, end: bag.end, text: "<" + name + ">" + li.String() + "</" + name + ">"})
		} else {
			edits = append(edits, edit{start: trimLeft(data, bag.endStart), end: bag.endStart, text: li.String() + indentBefore(data, bag.endStart)})
		}
	}
	return apply(data, edits), true
}

// pdfaID is a PDF/A identification: pdfaid:part, pdfaid:conformance and
// pdfaid:rev.
type pdfaID struct {
	part, conf, rev string
}

// set records a pdfaid property read from a packet.
func (id *pdfaID) set(local, value string) {
	value = strings.TrimSpace(value)
	switch local {
	case "part":
		id.part = value
	case "conformance":
		id.conf = value
	case "rev":
		id.rev = value
	}
}

// allowsEmbedding reports whether conformance level conf of PDF/A part allows
// embedded files of any type, such as the invoice XML: all levels of
// PDF/A-3, levels F and E of PDF/A-4 (plain PDF/A-4 only allows embedded
// PDF/A files).
func allowsEmbedding(part, conf string) bool {
	if part == "4" {
		return conf == "F" || conf == "E"
	}
	return conf == "A" || conf == "B" || conf == "U"
}

// identify returns the PDF/A identification to write, from the one found
// in the packet and the parameters.
func identify(found pdfaID, p Params) pdfaID {
	id := found
	switch {
	case p.PDFAPart == "3" || p.PDFAPart == "4":
		id.part = p.PDFAPart
	case id.part != "4":
		id.part = "3"
	}
	switch {
	case allowsEmbedding(id.part, id.conf):
	case allowsEmbedding(id.part, p.PDFAConformance):
		id.conf = p.PDFAConformance
	case id.part == "4":
		id.conf = "F"
	default:
		id.conf = "B"
	}
	switch {
	case id.part != "4":
		id.rev = ""
	case id.rev == "":
		id.rev = "2020"
	}
	return id
}

// schemaNS returns the pdfaSchema:namespaceURI of an extension schema li.
func schemaNS(data []byte, li *node) string {
	var ns string
	fields(data, li, nsPDFASchema, func(local, value string, _ *node) {
		if local == "namespaceURI" {
			ns = strings.TrimSpace(value)
		}
	})
	return ns
}

// trimLeft extends an edit start over the white space before it.
func trimLeft(data []byte, i int) int {
	for i > 0 && isSpace(data[i-1]) {
		i--
	}
	return i
}

// indentBefore returns the white space that precedes offset i, so that a
// replaced region keeps the line break and indentation of the original.
func indentBefore(data []byte, i int) string {
	return string(data[trimLeft(data, i):i])
}

// apply applies non-overlapping edits. Edits nested in a removed range are
// dropped.
func apply(data []byte, edits []edit) []byte {
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start < edits[j].start
		}
		return edits[i].end > edits[j].end
	})
	size := len(data)
	for _, e := range edits {
		size += len(e.text)
	}
	out := make([]byte, 0, size)
	pos := 0
	for _, e := range edits {
		if e.start < pos {
			continue // inside a previous edit
		}
		out = append(out, data[pos:e.start]...)
		out = append(out, e.text...)
		pos = e.end
	}
	return append(out, data[pos:]...)
}

func escape(s string) string {
	if !strings.ContainsAny(s, "<>&\"") {
		return s
	}
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

// writeDescriptions writes the managed properties: one rdf:Description with
// the simple properties and, when withSchema is set, one with the extension
// schema description.
func writeDescriptions(b *strings.Builder, r, about string, id pdfaID, p Params, present map[string]bool, withSchema bool) {
	has := func(space, local string) bool { return present[space+" "+local] }
	ab := escape(about)
	open := func(ns ...string) {
		b.WriteString("    <" + r + ":Description " + r + ":about=\"" + ab + "\"")
		for i := 0; i+1 < len(ns); i += 2 {
			b.WriteString("\n        xmlns:" + ns[i] + "=\"" + ns[i+1] + "\"")
		}
		b.WriteString(">\n")
	}
	prop := func(qn, v string) {
		b.WriteString("      <" + qn + ">" + escape(v) + "</" + qn + ">\n")
	}
	lang := func(qn, container, v string) {
		b.WriteString("      <" + qn + "><" + r + ":" + container + "><" + r + ":li")
		if container == "Alt" {
			b.WriteString(" xml:lang=\"x-default\"")
		}
		b.WriteString(">" + escape(v) + "</" + r + ":li></" + r + ":" + container + "></" + qn + ">\n")
	}

	di := p.DocInfo
	needDC := (di.Title != "" && !has(nsDC, "title")) || (di.Author != "" && !has(nsDC, "creator")) ||
		(di.Subject != "" && !has(nsDC, "description"))
	needPDF := (di.Keywords != "" && !has(nsPDF, "Keywords")) || (di.Producer != "" && !has(nsPDF, "Producer"))
	needXMP := !p.Date.IsZero() || (di.Creator != "" && !has(nsXMP, "CreatorTool")) ||
		(!di.CreationDate.IsZero() && !has(nsXMP, "CreateDate"))

	ns := []string{"pdfaid", nsPDFAID}
	if needXMP {
		ns = append(ns, "xmp", nsXMP)
	}
	if needDC {
		ns = append(ns, "dc", nsDC)
	}
	if needPDF {
		ns = append(ns, "pdf", nsPDF)
	}
	if !p.PDFAOnly {
		ns = append(ns, "fx", p.NamespaceURI)
	}
	open(ns...)
	prop("pdfaid:part", id.part)
	if id.rev != "" {
		prop("pdfaid:rev", id.rev)
	}
	prop("pdfaid:conformance", id.conf)
	if needXMP {
		if !p.Date.IsZero() {
			d := FormatDate(p.Date)
			prop("xmp:ModifyDate", d)
			prop("xmp:MetadataDate", d)
		}
		if di.Creator != "" && !has(nsXMP, "CreatorTool") {
			prop("xmp:CreatorTool", di.Creator)
		}
		if !di.CreationDate.IsZero() && !has(nsXMP, "CreateDate") {
			prop("xmp:CreateDate", FormatDate(di.CreationDate))
		}
	}
	if needDC {
		if di.Title != "" && !has(nsDC, "title") {
			lang("dc:title", "Alt", di.Title)
		}
		if di.Author != "" && !has(nsDC, "creator") {
			lang("dc:creator", "Seq", di.Author)
		}
		if di.Subject != "" && !has(nsDC, "description") {
			lang("dc:description", "Alt", di.Subject)
		}
	}
	if needPDF {
		if di.Keywords != "" && !has(nsPDF, "Keywords") {
			prop("pdf:Keywords", di.Keywords)
		}
		if di.Producer != "" && !has(nsPDF, "Producer") {
			prop("pdf:Producer", di.Producer)
		}
	}
	if !p.PDFAOnly {
		prop("fx:DocumentType", p.DocumentType)
		prop("fx:DocumentFileName", p.DocumentFileName)
		prop("fx:Version", p.Version)
		prop("fx:ConformanceLevel", p.ConformanceLevel)
	}
	b.WriteString("    </" + r + ":Description>\n")

	if withSchema && !p.PDFAOnly {
		open("pdfaExtension", nsPDFAExtension, "pdfaSchema", nsPDFASchema, "pdfaProperty", nsPDFAProperty)
		b.WriteString("      <pdfaExtension:schemas>\n        <" + r + ":Bag>")
		writeSchemaLI(b, r, p.NamespaceURI, false)
		b.WriteString("\n        </" + r + ":Bag>\n      </pdfaExtension:schemas>\n")
		b.WriteString("    </" + r + ":Description>\n")
	}
}

// fxProperties describes the properties of the hybrid-invoice schema.
var fxProperties = [...][2]string{
	{"DocumentFileName", "The name of the embedded XML invoice file"},
	{"DocumentType", "The type of the hybrid document in capital letters, e.g. INVOICE"},
	{"Version", "The version of the specification the embedded XML document conforms to"},
	{"ConformanceLevel", "The conformance level (profile) of the embedded XML document"},
}

// writeSchemaLI writes the rdf:li describing the hybrid-invoice schema. When
// declare is set, the li declares the pdfaSchema and pdfaProperty
// namespaces itself (it is inserted in an existing container).
func writeSchemaLI(b *strings.Builder, r, uri string, declare bool) {
	b.WriteString("\n          <" + r + ":li " + r + ":parseType=\"Resource\"")
	if declare {
		b.WriteString(" xmlns:pdfaSchema=\"" + nsPDFASchema + "\" xmlns:pdfaProperty=\"" + nsPDFAProperty + "\"")
	}
	b.WriteString(">\n")
	b.WriteString("            <pdfaSchema:schema>Factur-X PDFA Extension Schema</pdfaSchema:schema>\n")
	b.WriteString("            <pdfaSchema:namespaceURI>" + escape(uri) + "</pdfaSchema:namespaceURI>\n")
	b.WriteString("            <pdfaSchema:prefix>fx</pdfaSchema:prefix>\n")
	b.WriteString("            <pdfaSchema:property>\n              <" + r + ":Seq>\n")
	for _, fp := range fxProperties {
		b.WriteString("                <" + r + ":li " + r + ":parseType=\"Resource\">\n")
		b.WriteString("                  <pdfaProperty:name>" + fp[0] + "</pdfaProperty:name>\n")
		b.WriteString("                  <pdfaProperty:valueType>Text</pdfaProperty:valueType>\n")
		b.WriteString("                  <pdfaProperty:category>external</pdfaProperty:category>\n")
		b.WriteString("                  <pdfaProperty:description>" + fp[1] + "</pdfaProperty:description>\n")
		b.WriteString("                </" + r + ":li>\n")
	}
	b.WriteString("              </" + r + ":Seq>\n            </pdfaSchema:property>\n          </" + r + ":li>")
}
