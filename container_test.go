package facturx

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ritoon/factur-x/internal/pdf"
	"github.com/ritoon/factur-x/internal/pdf/pdftest"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/internal/xmp"
	"github.com/ritoon/factur-x/invoice"
)

var testDate = time.Date(2026, 11, 16, 12, 0, 0, 0, time.FixedZone("", 3600))

// ciiXML returns a minimal CII invoice with the given BT-24.
func ciiXML(urn string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">
  <rsm:ExchangedDocumentContext>
    <ram:GuidelineSpecifiedDocumentContextParameter>
      <ram:ID>` + urn + `</ram:ID>
    </ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
  <rsm:ExchangedDocument><ram:ID>F-1</ram:ID></rsm:ExchangedDocument>
</rsm:CrossIndustryInvoice>
`)
}

// zf1XML returns a minimal ZUGFeRD 1.0 invoice.
func zf1XML(level string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryDocument xmlns:rsm="urn:ferd:CrossIndustryDocument:invoice:1p0" xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:12">
  <rsm:SpecifiedExchangedDocumentContext>
    <ram:GuidelineSpecifiedDocumentContextParameter><ram:ID>urn:ferd:CrossIndustryDocument:invoice:1p0:` + level + `</ram:ID></ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:SpecifiedExchangedDocumentContext>
</rsm:CrossIndustryDocument>
`)
}

// file is an embedded file of a fixture.
type file struct {
	name      string // /F, /UF and name tree key
	noF, noUF bool
	rel       string // /AFRelationship, none when empty
	subtype   string // /Subtype of the stream, as written ("text#2Fxml"), none when empty
	modDate   string // /Params /ModDate, none when empty
	data      []byte
	hex       bool // ASCIIHexDecode instead of FlateDecode
	lzw       bool // declared LZWDecode, data left as is
	notInAF   bool
	notInTree bool
}

// fixture describes a PDF built by hand for the tests.
type fixture struct {
	version        string
	opts           pdftest.Options
	files          []file
	kids           bool // name tree with intermediate nodes
	pageAF         bool // list the files in the /AF array of the page instead of the catalog
	xmp            string
	noOutputIntent bool
	noDest         bool
	noID           bool
	encrypted      bool
	info           string // document information dictionary; testInfo when empty
	names          string // entries added to the /Names dictionary of the catalog
	pagesExtra     string // entries added to the root of the page tree
	// extra, when set, adds objects and returns entries appended to the
	// catalog and page dictionaries.
	extra func(b *pdftest.Builder) (catalog, page string)
	// specs holds the object numbers of the file specifications of files,
	// set by build before it calls extra.
	specs []int
}

// testInfo is the document information dictionary of the fixtures, and
// testDocInfo its XMP equivalent (its ModDate is testDate).
const testInfo = "<< /Producer (pdftest) /Title (Facture) /CreationDate (D:20260101000000Z) /ModDate (D:20261116120000+01'00') >>"

var testDocInfo = xmp.DocInfo{Title: "Facture", Producer: "pdftest", CreationDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

// validXMP returns the XMP packet of a Factur-X EN 16931 file.
func validXMP() string {
	return string(xmp.Merge(nil, xmp.Params{
		DocumentType: "INVOICE", DocumentFileName: "factur-x.xml", Version: "1.0",
		ConformanceLevel: "EN 16931", Date: testDate, DocInfo: testDocInfo,
	}))
}

func invoiceFile(data []byte) file {
	return file{name: "factur-x.xml", rel: "Alternative", subtype: "text#2Fxml", modDate: "D:20261116120000+01'00'", data: data}
}

// valid returns a fixture that passes CheckContainer.
func valid(data []byte) *fixture {
	return &fixture{files: []file{invoiceFile(data)}, xmp: validXMP()}
}

func lit(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return "(" + r.Replace(s) + ")"
}

func (f *fixture) build() []byte {
	b := &pdftest.Builder{Version: f.version}
	cat, pages, page := b.Reserve(), b.Reserve(), b.Reserve()
	var specs []int
	for _, fl := range f.files {
		dict := "<< /Type /EmbeddedFile"
		if fl.subtype != "" {
			dict += " /Subtype /" + fl.subtype
		}
		if fl.modDate != "" {
			dict += " /Params << /ModDate " + lit(fl.modDate) + fmt.Sprintf(" /Size %d >>", len(fl.data))
		}
		data := pdftest.Flate(fl.data)
		switch {
		case fl.lzw:
			dict += " /Filter /LZWDecode"
			data = fl.data
		case fl.hex:
			dict += " /Filter /ASCIIHexDecode"
			data = []byte(fmt.Sprintf("%X>", fl.data))
		default:
			dict += " /Filter /FlateDecode"
		}
		ef := b.AddStream(dict+" >>", data)
		fs := "<< /Type /Filespec"
		if !fl.noF {
			fs += " /F " + lit(fl.name)
		}
		if !fl.noUF {
			fs += " /UF " + lit(fl.name)
		}
		if fl.rel != "" {
			fs += " /AFRelationship /" + fl.rel
		}
		fs += fmt.Sprintf(" /EF << /F %d 0 R /UF %d 0 R >> >>", ef, ef)
		specs = append(specs, b.Add(fs))
	}
	f.specs = specs

	var tree, af []string
	var leaves []int
	for i, fl := range f.files {
		if !fl.notInTree {
			entry := fmt.Sprintf("%s %d 0 R", lit(fl.name), specs[i])
			if f.kids {
				leaves = append(leaves, b.Add(fmt.Sprintf("<< /Limits [%s %s] /Names [%s] >>", lit(fl.name), lit(fl.name), entry)))
			} else {
				tree = append(tree, entry)
			}
		}
		if !fl.notInAF {
			af = append(af, fmt.Sprintf("%d 0 R", specs[i]))
		}
	}

	catalog := fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R", pages)
	pageDict := fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 595 842]", pages)
	names := f.names
	switch {
	case len(leaves) > 0:
		var kids []string
		for _, l := range leaves {
			kids = append(kids, fmt.Sprintf("%d 0 R", l))
		}
		names = fmt.Sprintf(" /EmbeddedFiles << /Kids [%s] >>", strings.Join(kids, " ")) + names
	case len(tree) > 0:
		names = " /EmbeddedFiles << /Names [" + strings.Join(tree, " ") + "] >>" + names
	}
	if names != "" {
		catalog += " /Names <<" + names + " >>"
	}
	if f.extra != nil {
		c, p := f.extra(b)
		catalog += " " + c
		pageDict += " " + p
	}
	if len(af) > 0 {
		if f.pageAF {
			pageDict += " /AF [" + strings.Join(af, " ") + "]"
		} else {
			catalog += " /AF [" + strings.Join(af, " ") + "]"
		}
	}
	if f.xmp != "" {
		catalog += fmt.Sprintf(" /Metadata %d 0 R", b.AddStream("<< /Type /Metadata /Subtype /XML >>", []byte(f.xmp)))
	}
	if !f.noOutputIntent {
		dest := ""
		if !f.noDest {
			dest = fmt.Sprintf(" /DestOutputProfile %d 0 R", b.AddStream("<< /N 3 >>", []byte("not really an ICC profile")))
		}
		catalog += " /OutputIntents [<< /Type /OutputIntent /S /GTS_PDFA1 /OutputConditionIdentifier (sRGB)" + dest + " >>]"
	}
	b.Set(cat, catalog+" >>")
	b.Set(pages, fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R] /Count 1%s >>", page, f.pagesExtra))
	b.Set(page, pageDict+" >>")
	info := f.info
	if info == "" {
		info = testInfo
	}
	b.Trailer = fmt.Sprintf("/Root %d 0 R /Info %d 0 R", cat, b.Add(info))
	if !f.noID {
		b.Trailer += " /ID [<00112233445566778899AABBCCDDEEFF> <00112233445566778899AABBCCDDEEFF>]"
	}
	if f.encrypted {
		b.Trailer += fmt.Sprintf(" /Encrypt %d 0 R", b.Add("<< /Filter /Standard /V 2 /R 3 /O <00> /U <00> /P -4 >>"))
	}
	return b.Bytes(f.opts)
}

func readTestdata(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExtract(t *testing.T) {
	x := ciiXML(invoice.URNEN16931)
	for _, tc := range []struct {
		name string
		f    *fixture
	}{
		{"classic", valid(x)},
		{"xref stream and object stream", &fixture{files: []file{invoiceFile(x)}, xmp: validXMP(), opts: pdftest.Options{Layout: pdftest.XrefStream, ObjStm: true, Predictor: true}}},
		{"hybrid", &fixture{files: []file{invoiceFile(x)}, xmp: validXMP(), opts: pdftest.Options{Layout: pdftest.Hybrid, ObjStm: true}}},
		{"no xref", &fixture{files: []file{invoiceFile(x)}, xmp: validXMP(), opts: pdftest.Options{Layout: pdftest.NoXref}}},
		{"shifted offsets", &fixture{files: []file{invoiceFile(x)}, xmp: validXMP(), opts: pdftest.Options{Shift: 5}}},
		{"name tree with kids", &fixture{files: []file{{name: "notes.txt", data: []byte("hello")}, invoiceFile(x)}, kids: true, xmp: validXMP()}},
		{"AF only", &fixture{files: []file{func() file { f := invoiceFile(x); f.notInTree = true; return f }()}, xmp: validXMP()}},
		{"page AF only", &fixture{files: []file{func() file { f := invoiceFile(x); f.notInTree = true; return f }()}, pageAF: true, xmp: validXMP()}},
		{"hex filter", &fixture{files: []file{func() file { f := invoiceFile(x); f.hex = true; return f }()}, xmp: validXMP()}},
		{"PDF 1.3 with AFRelationship", &fixture{version: "1.3", files: []file{invoiceFile(x)}, xmp: validXMP()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Extract(tc.f.build())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a.XML, x) {
				t.Fatalf("XML differs: %q", a.XML)
			}
			if a.Filename != "factur-x.xml" || a.Relationship != "Alternative" || a.MimeType != "text/xml" {
				t.Errorf("attachment %+v", a)
			}
			if !a.ModDate.Equal(testDate) {
				t.Errorf("ModDate %v", a.ModDate)
			}
			wantAF := !tc.f.files[len(tc.f.files)-1].notInAF && !tc.f.pageAF
			if a.InAF != wantAF {
				t.Errorf("InAF %v", a.InAF)
			}
			m := a.Metadata
			if m == nil || m.NamespaceURI != NamespaceFacturX || m.DocumentType != "INVOICE" || m.DocumentFileName != "factur-x.xml" ||
				m.Version != "1.0" || m.ConformanceLevel != "EN 16931" || m.PDFAPart != 3 || m.PDFAConformance != "B" {
				t.Errorf("metadata %+v", m)
			}
		})
	}
}

func TestExtractSelection(t *testing.T) {
	inv := ciiXML(invoice.URNBasic)
	order := []byte(`<?xml version="1.0"?><rsm:SCRDMCCBDACIOMessageStructure xmlns:rsm="urn:un:unece:uncefact:data:SCRDMCCBDACIOMessageStructure:100"/>`)
	for _, tc := range []struct {
		name  string
		files []file
		want  string // file name, "" for ErrNoInvoice
	}{
		{"factur-x.xml preferred", []file{{name: "zugferd-invoice.xml", data: ciiXML(invoice.URNMinimum)}, {name: "factur-x.xml", data: inv}}, "factur-x.xml"},
		{"legacy name", []file{{name: "ZUGFeRD-invoice.xml", data: inv}}, "ZUGFeRD-invoice.xml"},
		{"xrechnung.xml", []file{{name: "xrechnung.xml", data: inv}}, "xrechnung.xml"},
		{"order-x ignored", []file{{name: "order-x.xml", data: order}, {name: "invoice.xml", data: inv}}, "invoice.xml"},
		{"order-x only", []file{{name: "order-x.xml", data: order}}, ""},
		{"by root element", []file{{name: "readme.txt", data: []byte("hello")}, {name: "data.xml", data: inv}}, "data.xml"},
		{"not an invoice", []file{{name: "data.xml", data: []byte("<root/>")}}, ""},
		{"no attachment", nil, ""},
		{"UF only", []file{{name: "factur-x.xml", noF: true, data: inv}}, "factur-x.xml"},
		{"F only", []file{{name: "factur-x.xml", noUF: true, data: inv}}, "factur-x.xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Extract((&fixture{files: tc.files}).build())
			if tc.want == "" {
				if !errors.Is(err, ErrNoInvoice) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if a.Filename != tc.want {
				t.Fatalf("got %q", a.Filename)
			}
			if a.Metadata != nil {
				t.Errorf("metadata without XMP: %+v", a.Metadata)
			}
		})
	}
}

func TestExtractAll(t *testing.T) {
	f := valid(ciiXML(invoice.URNEN16931))
	f.files = append(f.files, file{name: "logo.txt", data: []byte("logo"), rel: "Supplement"})
	all, err := ExtractAll(f.build())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Filename != "factur-x.xml" || all[1].Filename != "logo.txt" || string(all[1].XML) != "logo" {
		t.Fatalf("got %d attachments", len(all))
	}
}

func TestExtractErrors(t *testing.T) {
	f := valid(ciiXML(invoice.URNEN16931))
	f.encrypted = true
	if _, err := Extract(f.build()); !errors.Is(err, ErrEncrypted) {
		t.Errorf("encrypted: %v", err)
	}
	if _, err := Extract([]byte("not a pdf")); err == nil || errors.Is(err, ErrNoInvoice) {
		t.Errorf("not a PDF: %v", err)
	}
	// A malformed ModDate does not prevent extraction.
	f = valid(ciiXML(invoice.URNEN16931))
	f.files[0].modDate = "yesterday"
	a, err := Extract(f.build())
	if err != nil || !a.ModDate.IsZero() {
		t.Errorf("malformed ModDate: %v, %v", a, err)
	}
}

func TestExtractTestdata(t *testing.T) {
	a, err := Extract(readTestdata(t, "EN16931_Einfach.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(a.XML, []byte("<?xml")) || xmlRoot(a.XML) != "CrossIndustryInvoice" {
		t.Fatalf("XML %.60q", a.XML)
	}
	if _, id := specificationID(a.XML); invoice.ProfileFromURN(id) != invoice.ProfileEN16931 {
		t.Errorf("BT-24 %q", id)
	}
	if a.Metadata == nil || a.Metadata.PDFAPart != 3 {
		t.Errorf("metadata %+v", a.Metadata)
	}
}

func TestSpecificationIDMalformed(t *testing.T) {
	// More end tags than start tags used to drive the scan depth below zero
	// and panic (index out of range).
	for _, doc := range []string{
		"</a></b><c>",
		"</a><b>",
		"</a>",
		"<a></a></b><c><d><e><f>",
		"<a/></b><c>",
		// "<?" and "?>" overlap.
		"<?>",
	} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%q: specificationID panics: %v", doc, r)
				}
			}()
			specificationID([]byte(doc))
		}()
	}
	if root, id := specificationID([]byte("</a></b><c>")); id != "" {
		t.Errorf("root %q, BT-24 %q", root, id)
	}

	// The same XML through the public API: an error, not a panic.
	// CheckContainer leaves the unreadable BT-24 to the XML validation.
	xml := []byte("</a></b><rsm:CrossIndustryInvoice>")
	if _, err := Embed(base(pdftest.Options{}), xml, nil); err == nil {
		t.Error("Embed: no error")
	}
	if a, _, err := CheckContainer(valid(xml).build()); err != nil || a == nil {
		t.Errorf("CheckContainer: %+v, %v", a, err)
	}
}

// specificationIDCases are documents on which the quick scan of BT-24 used
// to read something else than xmltree. specificationID must now give the
// root and BT-24 of the full parse.
var specificationIDCases = []struct {
	name     string
	doc      string
	root, id string
}{
	// Markup and references inside BT-24: xmltree keeps the character data
	// only, and expands the references.
	{"element", string(ciiXML("a<x/>b")), "CrossIndustryInvoice", "ab"},
	{"element with text", string(ciiXML("a<x>y</x>b")), "CrossIndustryInvoice", "ab"},
	{"comment", string(ciiXML("a<!--c-->b")), "CrossIndustryInvoice", "ab"},
	{"processing instruction", string(ciiXML("a<?p?>b")), "CrossIndustryInvoice", "ab"},
	{"CDATA", string(ciiXML("a<![CDATA[<b>]]>c")), "CrossIndustryInvoice", "a<b>c"},
	{"character reference", string(ciiXML("&#65;")), "CrossIndustryInvoice", "A"},
	{"escaped URN", string(ciiXML("urn&#58;cen.eu:en16931:2017")), "CrossIndustryInvoice", invoice.URNEN16931},
	{"carriage return", string(ciiXML("a\rb\r\nc")), "CrossIndustryInvoice", "a\nb\nc"},
	// Attribute values may hold '>' and "/>".
	{"attribute with >", strings.Replace(string(ciiXML(invoice.URNEN16931)), "<ram:ID>", `<ram:ID schemeID="a>b">`, 1),
		"CrossIndustryInvoice", invoice.URNEN16931},
	{"attribute with />", strings.Replace(string(ciiXML(invoice.URNEN16931)), "<rsm:ExchangedDocumentContext>", `<rsm:ExchangedDocumentContext a="/>">`, 1),
		"CrossIndustryInvoice", invoice.URNEN16931},
	// A colon that starts a name belongs to the local name.
	{"leading colon", strings.ReplaceAll(string(ciiXML(invoice.URNEN16931)), "rsm:CrossIndustryInvoice", ":CrossIndustryInvoice"),
		":CrossIndustryInvoice", invoice.URNEN16931},
	// xmltree converts ISO-8859-1 to UTF-8.
	{"ISO-8859-1", strings.Replace(string(ciiXML("urn:\xe9")), `encoding="UTF-8"`, `encoding="ISO-8859-1"`, 1),
		"CrossIndustryInvoice", "urn:é"},
	// A DOCTYPE may hide '>' and '<' that are not markup.
	{"DOCTYPE", `<!DOCTYPE x [<!ENTITY e ">"><y>]><ExchangedDocumentContext><GuidelineSpecifiedDocumentContextParameter>` +
		`<ID>urn</ID></GuidelineSpecifiedDocumentContextParameter></ExchangedDocumentContext>`, "ExchangedDocumentContext", ""},
	// The "-->" of a comment comes after its "<!--".
	{"comment <!-->", `<r><!--><ExchangedDocumentContext>--><GuidelineSpecifiedDocumentContextParameter>` +
		`<ID>u</ID></GuidelineSpecifiedDocumentContextParameter></r>`, "r", ""},
	// The first ID decides, even empty; the ZUGFeRD 1.0 path comes next.
	{"empty ID first", strings.Replace(string(ciiXML(invoice.URNEN16931)), "<ram:ID>", "<ram:ID/><ram:ID>", 1),
		"CrossIndustryInvoice", ""},
	{"empty ID, ZUGFeRD 1.0 context", strings.Replace(string(ciiXML("")), "</rsm:CrossIndustryInvoice>",
		"<rsm:SpecifiedExchangedDocumentContext><ram:GuidelineSpecifiedDocumentContextParameter><ram:ID>zf1</ram:ID>"+
			"</ram:GuidelineSpecifiedDocumentContextParameter></rsm:SpecifiedExchangedDocumentContext></rsm:CrossIndustryInvoice>", 1),
		"CrossIndustryInvoice", "zf1"},
}

func TestSpecificationIDAsXMLTree(t *testing.T) {
	for _, tc := range specificationIDCases {
		if _, err := xmltree.Parse([]byte(tc.doc)); err != nil {
			t.Errorf("%s: not well-formed: %v", tc.name, err)
		}
		if root, id := specificationID([]byte(tc.doc)); root != tc.root || id != tc.id {
			t.Errorf("%s: root %q, BT-24 %q; want %q, %q", tc.name, root, id, tc.root, tc.id)
		}
	}

	// Ordinary invoices keep the quick scan, with CRLF line ends too.
	docs := [][]byte{zf1XML("BASIC")}
	for _, urn := range []string{invoice.URNMinimum, invoice.URNEN16931, invoice.URNExtended} {
		docs = append(docs, ciiXML(urn), bytes.ReplaceAll(ciiXML(urn), []byte("\n"), []byte("\r\n")))
	}
	for _, name := range []string{"EN16931_Einfach.cii.xml", "XRECHNUNG_Einfach.cii.xml"} {
		docs = append(docs, readTestdata(t, name))
	}
	for _, doc := range docs {
		if root, id, ok := scanSpecificationID(doc); !ok || id == "" {
			t.Errorf("quick scan: root %q, BT-24 %q, ok %v\n%.300s", root, id, ok, doc)
		}
	}

	// An escaped URN is a known profile for Embed and CheckContainer.
	escaped := func(urn string) []byte { return ciiXML(strings.Replace(urn, ":", "&#58;", 1)) }
	out, err := Embed(base(pdftest.Options{}), escaped(invoice.URNEN16931), &EmbedOptions{ModDate: testDate})
	if err != nil {
		t.Fatal(err)
	}
	if a, findings, err := CheckContainer(out); err != nil || len(findings) > 0 || a.Relationship != "Alternative" {
		t.Errorf("Embed: %+v, %v, %v", a, findings, err)
	}
	// The XMP says EN 16931, BT-24 BASIC.
	checkContainerRules(t, valid(escaped(invoice.URNBasic)), "PDF-24")
}

// FuzzSpecificationID checks that reading BT-24 never panics, whatever the
// attachment holds, and that the quick scan reads the root element and BT-24
// as xmltree does whenever both accept the document:
//
//	go test -run '^$' -fuzz FuzzSpecificationID .
func FuzzSpecificationID(f *testing.F) {
	for _, urn := range []string{invoice.URNMinimum, invoice.URNEN16931, invoice.URNExtended} {
		f.Add(ciiXML(urn))
	}
	f.Add(zf1XML("BASIC"))
	f.Add([]byte("</a></b><c>"))
	for _, tc := range specificationIDCases {
		f.Add([]byte(tc.doc))
	}
	for _, name := range []string{"EN16931_Einfach.cii.xml", "XRECHNUNG_Einfach.cii.xml"} {
		if data, err := os.ReadFile(filepath.Join("testdata", name)); err == nil {
			f.Add(data)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		specificationID(data)
		root, id, ok := scanSpecificationID(data)
		if !ok {
			return
		}
		tree, err := xmltree.Parse(data)
		if err != nil {
			return // not well-formed: the quick scan reads what it can
		}
		want, zf1 := tree.Value(bt24Path), tree.Value(bt24PathZF1)
		if want == "" {
			want = zf1
		}
		// The quick scan reads the first document context, the full parse
		// prefers ExchangedDocumentContext: a document holding both may give
		// the ZUGFeRD 1.0 identifier.
		if root != tree.Name || id != want && id != zf1 {
			t.Errorf("quick scan: root %q, BT-24 %q; xmltree: root %q, BT-24 %q\n%q", root, id, tree.Name, want, data)
		}
	})
}

// base returns a PDF/A-ready document without attachment nor XMP.
func base(opts pdftest.Options) []byte {
	return (&fixture{opts: opts}).build()
}

func TestEmbedRoundTrip(t *testing.T) {
	x := readTestdata(t, "EN16931_Einfach.cii.xml")
	for _, opts := range []pdftest.Options{
		{},
		{Layout: pdftest.XrefStream, ObjStm: true},
		{Layout: pdftest.Hybrid, ObjStm: true},
		{Shift: 9},
	} {
		in := base(opts)
		out, err := Embed(in, x, &EmbedOptions{ModDate: testDate})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(out, in) {
			t.Fatal("not an incremental update")
		}
		a, findings, err := CheckContainer(out)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range findings {
			t.Errorf("layout %+v: %s", opts, f)
		}
		if !bytes.Equal(a.XML, x) || a.Filename != "factur-x.xml" || a.Relationship != "Alternative" ||
			a.MimeType != "text/xml" || !a.InAF || !a.ModDate.Equal(testDate) || a.Description != "Invoice EN 16931 (factur-x.xml)" {
			t.Errorf("attachment %+v", a)
		}

		// Stream parameters and file identifier.
		c, err := openContainer(out)
		if err != nil {
			t.Fatal(err)
		}
		_, _, st, _ := c.invoice()
		params, _ := c.r.Dict(st.Dict.Get("Params"))
		sum := md5.Sum(x)
		if n, _ := params.Int("Size"); n != int64(len(x)) {
			t.Errorf("Size %d", n)
		}
		if cs, _ := params.Get("CheckSum").(pdf.String); !bytes.Equal(cs, sum[:]) {
			t.Errorf("CheckSum %x", cs)
		}
		id, _ := c.r.Resolve(c.r.Trailer().Get("ID")).(pdf.Array)
		if len(id) != 2 || !bytes.Equal(id[0].(pdf.String), []byte("\x00\x11\x22\x33\x44\x55\x66\x77\x88\x99\xaa\xbb\xcc\xdd\xee\xff")) ||
			bytes.Equal(id[1].(pdf.String), id[0].(pdf.String)) {
			t.Errorf("ID %x", id)
		}
		// /Info ModDate follows the XMP dates.
		info, _ := c.r.Dict(c.r.Trailer().Get("Info"))
		if s, _ := info.Get("ModDate").(pdf.String); string(s) != "D:20261116120000+01'00'" {
			t.Errorf("Info ModDate %q", s)
		}
		xi, _, _ := c.xmpInfo()
		if xi.ModifyDate != "2026-11-16T12:00:00+01:00" || xi.CreateDate != "2026-01-01T00:00:00Z" {
			t.Errorf("XMP dates %q %q", xi.ModifyDate, xi.CreateDate)
		}
	}
}

func TestEmbedDeterministic(t *testing.T) {
	in := base(pdftest.Options{})
	x := ciiXML(invoice.URNBasic)
	a, err1 := Embed(in, x, &EmbedOptions{ModDate: testDate})
	b, err2 := Embed(in, x, &EmbedOptions{ModDate: testDate})
	if err1 != nil || err2 != nil || !bytes.Equal(a, b) {
		t.Fatal("outputs differ")
	}
}

// pdfaPacket returns an XMP packet holding only a PDF/A identification.
func pdfaPacket(props string) string {
	return `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">` + props + `</rdf:Description>
</rdf:RDF></x:xmpmeta>`
}

func TestEmbedPDFAPart(t *testing.T) {
	const (
		a4f = "<pdfaid:part>4</pdfaid:part><pdfaid:rev>2020</pdfaid:rev><pdfaid:conformance>F</pdfaid:conformance>"
		a4e = "<pdfaid:part>4</pdfaid:part><pdfaid:rev>2020</pdfaid:rev><pdfaid:conformance>E</pdfaid:conformance>"
	)
	x := ciiXML(invoice.URNEN16931)
	for _, tc := range []struct {
		name            string
		version, packet string
		part            int
		conf, rev       string
		rules           string // findings of CheckContainer on the result
	}{
		{"PDF/A-4f", "2.0", pdfaPacket(a4f), 4, "F", "2020", ""},
		{"PDF/A-4e", "2.0", pdfaPacket(a4e), 4, "E", "2020", ""},
		{"PDF/A-4, no level nor rev", "2.0", pdfaPacket("<pdfaid:part>4</pdfaid:part>"), 4, "F", "2020", ""},
		{"PDF 2.0, no XMP", "2.0", "", 4, "F", "2020", ""},
		{"PDF 2.0 declared PDF/A-3b", "2.0", pdfaPacket("<pdfaid:part>3</pdfaid:part><pdfaid:conformance>B</pdfaid:conformance>"), 4, "F", "2020", ""},
		{"PDF/A-2u", "1.7", pdfaPacket("<pdfaid:part>2</pdfaid:part><pdfaid:conformance>U</pdfaid:conformance>"), 3, "U", "", ""},
		{"PDF 1.7, no XMP", "", "", 3, "B", "", ""},
		// The declaration of the host is kept; the version mismatch is reported.
		{"PDF 1.7 declared PDF/A-4f", "1.7", pdfaPacket(a4f), 4, "F", "2020", "PDF-30"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := (&fixture{version: tc.version, xmp: tc.packet}).build()
			out, err := Embed(in, x, &EmbedOptions{ModDate: testDate})
			if err != nil {
				t.Fatal(err)
			}
			a, findings, err := CheckContainer(out)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(rulesOf(findings), ","); got != tc.rules {
				t.Errorf("rules %q, want %q", got, tc.rules)
				for _, f := range findings {
					t.Log(f)
				}
			}
			m := a.Metadata
			if m == nil || m.PDFAPart != tc.part || m.PDFAConformance != tc.conf || m.PDFARev != tc.rev {
				t.Errorf("metadata %+v, want part %d, conformance %q, rev %q", m, tc.part, tc.conf, tc.rev)
			}
		})
	}
}

func TestEmbedReplace(t *testing.T) {
	f := &fixture{files: []file{{name: "notes.txt", data: []byte("notes"), rel: "Supplement"}}}
	in := f.build()
	first, err := Embed(in, ciiXML(invoice.URNMinimum), &EmbedOptions{ModDate: testDate})
	if err != nil {
		t.Fatal(err)
	}
	x := readTestdata(t, "EN16931_Rabatte.cii.xml")
	second, err := Embed(first, x, &EmbedOptions{ModDate: testDate.Add(time.Hour), Filename: "FACTUR-X.xml"})
	if err != nil {
		t.Fatal(err)
	}
	all, err := ExtractAll(second)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range all {
		names = append(names, a.Filename)
	}
	// The name tree is sorted by key.
	if strings.Join(names, ",") != "FACTUR-X.xml,notes.txt" {
		t.Fatalf("attachments %v", names)
	}
	c, _ := openContainer(second)
	// notes.txt and the new invoice; the first invoice is gone.
	if af, _ := c.r.Resolve(c.catalog.Get("AF")).(pdf.Array); len(af) != 2 {
		t.Errorf("AF has %d entries", len(af))
	}
	a, _ := Extract(second)
	if !bytes.Equal(a.XML, x) || a.Metadata.ConformanceLevel != "EN 16931" || a.Metadata.DocumentFileName != "FACTUR-X.xml" {
		t.Errorf("replaced attachment %+v %+v", a, a.Metadata)
	}
	if a.Relationship != "Alternative" {
		t.Errorf("relationship %q", a.Relationship)
	}
	xi, _, _ := c.xmpInfo()
	if len(xi.Schemas) != 1 {
		t.Errorf("%d extension schemas", len(xi.Schemas))
	}
}

func TestEmbedOptions(t *testing.T) {
	in := base(pdftest.Options{})
	out, err := Embed(in, readTestdata(t, "XRECHNUNG_Einfach.cii.xml"), &EmbedOptions{ModDate: testDate})
	if err != nil {
		t.Fatal(err)
	}
	a, findings, _ := CheckContainer(out)
	if a.Filename != "xrechnung.xml" || a.Metadata.ConformanceLevel != "XRECHNUNG" || len(findings) != 0 {
		t.Errorf("XRechnung: %+v %+v %v", a, a.Metadata, findings)
	}

	out, err = Embed(in, ciiXML(invoice.URNMinimum), &EmbedOptions{
		ModDate: testDate, Relationship: "Source", Description: "Facture n° 1", Version: "1.1", DocumentType: "INVOICE",
	})
	if err != nil {
		t.Fatal(err)
	}
	a, _ = Extract(out)
	if a.Relationship != "Source" || a.Description != "Facture n° 1" || a.Metadata.Version != "1.1" || a.Metadata.ConformanceLevel != "MINIMUM" {
		t.Errorf("options: %+v %+v", a, a.Metadata)
	}

	// An explicit profile wins over BT-24.
	out, _ = Embed(in, ciiXML("urn:example:custom"), &EmbedOptions{Profile: invoice.ProfileExtended})
	if a, _ := Extract(out); a == nil || a.Metadata.ConformanceLevel != "EXTENDED" {
		t.Errorf("explicit profile: %+v", a)
	}

	for name, tc := range map[string]struct {
		xml  []byte
		opts *EmbedOptions
	}{
		"no BT-24":         {[]byte("<rsm:CrossIndustryInvoice/>"), nil},
		"unknown BT-24":    {ciiXML("urn:example:custom"), nil},
		"bad relationship": {ciiXML(invoice.URNBasic), &EmbedOptions{Relationship: "Supplement"}},
		"empty XML":        {nil, nil},
	} {
		if _, err := Embed(in, tc.xml, tc.opts); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// Another invoice attachment is replaced too.
	out, err = Embed(valid(ciiXML(invoice.URNEN16931)).build(), readTestdata(t, "XRECHNUNG_Einfach.cii.xml"), &EmbedOptions{ModDate: testDate})
	if err != nil {
		t.Fatal(err)
	}
	if all, _ := ExtractAll(out); len(all) != 1 || all[0].Filename != "xrechnung.xml" {
		t.Errorf("factur-x.xml not replaced by xrechnung.xml: %d attachments", len(all))
	}
	if _, findings, _ := CheckContainer(out); len(findings) != 0 {
		t.Errorf("findings %v", findings)
	}

	enc := valid(ciiXML(invoice.URNBasic))
	enc.encrypted = true
	if _, err := Embed(enc.build(), ciiXML(invoice.URNBasic), nil); !errors.Is(err, ErrEncrypted) {
		t.Errorf("encrypted: %v", err)
	}
}

func TestEmbedAttachments(t *testing.T) {
	// The host has an attachment that a supplementary file replaces.
	f := &fixture{files: []file{
		{name: "Timesheet.csv", data: []byte("old"), rel: "Supplement", subtype: "text#2Fcsv"},
		{name: "logo.png", data: []byte("png"), rel: "Unspecified", subtype: "image#2Fpng"},
	}}
	in := f.build()
	order := []byte("%PDF-1.7 purchase order")
	later := testDate.Add(-24 * time.Hour)
	opts := &EmbedOptions{ModDate: testDate, Attachments: []File{
		{Name: "commande.pdf", Data: order, Description: "Bon de commande n° 12"},
		{Name: "timesheet.csv", Data: []byte("day;hours\n1;7\n")},
		{Name: "notes", MimeType: "text/markdown", Data: nil, Relationship: "Data", ModDate: later},
	}}
	out, err := Embed(in, ciiXML(invoice.URNEN16931), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, findings, err := CheckContainer(out); err != nil || len(findings) != 0 {
		t.Errorf("CheckContainer: %v %v", findings, err)
	}
	again, _ := Embed(in, ciiXML(invoice.URNEN16931), opts)
	if !bytes.Equal(out, again) {
		t.Error("outputs differ")
	}
	plain, _ := Embed(in, ciiXML(invoice.URNEN16931), &EmbedOptions{ModDate: testDate})
	if a, b := fileID(t, out), fileID(t, plain); bytes.Equal(a, b) {
		t.Error("the attachments do not change the file identifier")
	}

	all, err := ExtractAll(out)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*Attachment{}
	var names []string
	for _, a := range all {
		got[a.Filename] = a
		names = append(names, a.Filename)
	}
	// The name tree is sorted by key; Timesheet.csv of the host is replaced.
	if strings.Join(names, ",") != "commande.pdf,factur-x.xml,logo.png,notes,timesheet.csv" {
		t.Fatalf("attachments %v", names)
	}
	for _, want := range []Attachment{
		{Filename: "commande.pdf", XML: order, Relationship: "Supplement", MimeType: "application/pdf", Description: "Bon de commande n° 12", ModDate: testDate},
		{Filename: "timesheet.csv", XML: []byte("day;hours\n1;7\n"), Relationship: "Supplement", MimeType: "text/csv", ModDate: testDate},
		{Filename: "notes", XML: []byte{}, Relationship: "Data", MimeType: "text/markdown", ModDate: later},
	} {
		a := got[want.Filename]
		if a == nil || !bytes.Equal(a.XML, want.XML) || a.Relationship != want.Relationship || a.MimeType != want.MimeType ||
			a.Description != want.Description || !a.ModDate.Equal(want.ModDate) || !a.InAF {
			t.Errorf("%s: %+v", want.Filename, a)
		}
	}
	// The invoice comes first in /AF, then the supplementary files in their
	// order; logo.png, kept, stays before them.
	c, _ := openContainer(out)
	af, _ := c.r.Resolve(c.catalog.Get("AF")).(pdf.Array)
	var afNames []string
	for _, x := range af {
		d, _ := c.r.Dict(x)
		afNames = append(afNames, specName(c.r, d))
	}
	if strings.Join(afNames, ",") != "logo.png,factur-x.xml,commande.pdf,timesheet.csv,notes" {
		t.Errorf("AF %v", afNames)
	}
	if a, _ := Extract(out); a.Filename != "factur-x.xml" {
		t.Errorf("invoice %q", a.Filename)
	}

	x := ciiXML(invoice.URNBasic)
	for name, files := range map[string][]File{
		"no name":                 {{Data: order}},
		"factur-x.xml":            {{Name: "factur-x.xml", Data: x}},
		"XRECHNUNG.XML":           {{Name: "XRECHNUNG.XML", Data: x}},
		"zugferd-invoice.xml":     {{Name: "zugferd-invoice.xml", Data: x}},
		"invoice filename":        {{Name: "Invoice.xml", Data: x}},
		"duplicate name":          {{Name: "a.pdf"}, {Name: "A.PDF"}},
		"bad relationship":        {{Name: "a.pdf", Relationship: "Attachment"}},
		"MIME type without slash": {{Name: "a.pdf", MimeType: "pdf"}},
		"MIME type with a space":  {{Name: "a.pdf", MimeType: "application/ pdf"}},
		"MIME type parameters":    {{Name: "a.txt", MimeType: "text/plain; charset=utf-8"}},
	} {
		if _, err := Embed(in, x, &EmbedOptions{Filename: "invoice.xml", Attachments: files}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// fileID returns the second part of the file identifier of a PDF.
func fileID(t *testing.T, data []byte) []byte {
	t.Helper()
	c, err := openContainer(data)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := c.r.Resolve(c.r.Trailer().Get("ID")).(pdf.Array)
	if len(id) != 2 {
		t.Fatalf("ID %v", id)
	}
	s, _ := id[1].(pdf.String)
	return s
}

func TestMimeType(t *testing.T) {
	for name, want := range map[string]string{
		"a.pdf":          "application/pdf",
		"A.PDF":          "application/pdf",
		"x.y.XLSX":       "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"photo.jpeg":     "image/jpeg",
		"releve.xml":     "text/xml",
		"archive.tar":    "application/octet-stream",
		"README":         "application/octet-stream",
		"dir.pdf/README": "application/octet-stream",
	} {
		if got := mimeType(name); got != want {
			t.Errorf("mimeType(%q) = %q, want %q", name, got, want)
		}
	}
	for s, want := range map[string]bool{
		"application/pdf": true, "image/svg+xml": true, "application/vnd.ms-excel": true,
		"": false, "pdf": false, "/pdf": false, "application/": false, "a/b/c": false, "text/plain; charset=utf-8": false, "téxt/plain": false,
	} {
		if got := validMIME(s); got != want {
			t.Errorf("validMIME(%q) = %v", s, got)
		}
	}
}

func TestEmbedTestdata(t *testing.T) {
	// Re-embedding into a real Factur-X file replaces its attachment and
	// keeps its XMP metadata consistent.
	in := readTestdata(t, "EN16931_Einfach.pdf")
	x := readTestdata(t, "EN16931_Rabatte.cii.xml")
	before, findingsBefore, err := CheckContainer(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Embed(in, x, &EmbedOptions{ModDate: testDate})
	if err != nil {
		t.Fatal(err)
	}
	a, findings, err := CheckContainer(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.XML, x) || a.Metadata.PDFAConformance != before.Metadata.PDFAConformance {
		t.Errorf("attachment %+v", a.Metadata)
	}
	if len(findings) > len(findingsBefore) {
		t.Errorf("findings after embedding: %v (before: %v)", findings, findingsBefore)
	}
	all, _ := ExtractAll(out)
	if len(all) != 1 {
		t.Errorf("%d attachments", len(all))
	}
}

// TestCorpus extracts the invoice of every PDF of FX_CORPUS and compares it
// with the reference XML of FX_CORPUS_XML (same relative path, .xml), when
// present. Some references lack the final end of line of the attachment.
func TestCorpus(t *testing.T) {
	root := os.Getenv("FX_CORPUS")
	if root == "" {
		t.Skip("FX_CORPUS not set")
	}
	refs := os.Getenv("FX_CORPUS_XML")
	var total, compared int
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".pdf") {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		total++
		c, err := openContainer(data)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			return nil
		}
		_, x, st, err := c.invoice()
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			return nil
		}
		if params, ok := c.r.Dict(st.Dict.Get("Params")); ok {
			if n, ok := pdf.AsInt(c.r.Resolve(params.Get("Size"))); ok && n != int64(len(x)) {
				t.Logf("%s: /Size %d, decoded %d bytes", rel, n, len(x))
			}
			if cs, ok := c.r.Resolve(params.Get("CheckSum")).(pdf.String); ok && len(cs) == 16 {
				if sum := md5.Sum(x); !bytes.Equal(cs, sum[:]) {
					t.Logf("%s: /CheckSum mismatch", rel)
				}
			}
		}
		if refs == "" {
			return nil
		}
		ref, err := os.ReadFile(filepath.Join(refs, strings.TrimSuffix(rel, filepath.Ext(rel))+".xml"))
		if err != nil {
			return nil
		}
		compared++
		trim := func(b []byte) []byte { return bytes.TrimRight(b, "\r\n") }
		if !bytes.Equal(trim(ref), trim(x)) {
			t.Errorf("%s: XML differs from the reference (%d vs %d bytes)", rel, len(x), len(ref))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d PDFs, %d compared with a reference", total, compared)
}
