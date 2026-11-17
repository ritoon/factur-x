package facturx

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/pdf/pdftest"
	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmp"
	"github.com/ritoon/factur-x/invoice"
	"github.com/ritoon/factur-x/validate"
)

// xmpWith returns a valid Factur-X XMP packet modified by fn.
func xmpWith(fn func(p *xmp.Params)) string {
	p := xmp.Params{DocumentType: "INVOICE", DocumentFileName: "factur-x.xml", Version: "1.0", ConformanceLevel: "EN 16931", Date: testDate, DocInfo: testDocInfo}
	fn(&p)
	return string(xmp.Merge(nil, p))
}

// replaced replaces old by new in s, and panics when old is absent.
func replaced(s, old, new string) string {
	if !strings.Contains(s, old) {
		panic("replaced: " + old + " not found")
	}
	return strings.Replace(s, old, new, 1)
}

// withoutExtension removes the PDF/A extension schema description.
func withoutExtension(s string) string {
	i := strings.Index(s, "<rdf:Description")
	j := strings.Index(s[i+1:], "<rdf:Description") + i + 1
	k := strings.LastIndex(s, "</rdf:Description>") + len("</rdf:Description>")
	return s[:j] + s[k:]
}

func rulesOf(findings []rules.Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Rule)
	}
	sort.Strings(out)
	return out
}

func TestCheckContainer(t *testing.T) {
	en := ciiXML(invoice.URNEN16931)
	mod := func(fn func(f *fixture)) *fixture {
		f := valid(en)
		fn(f)
		return f
	}
	modFile := func(fn func(fl *file)) *fixture {
		return mod(func(f *fixture) { fn(&f.files[0]) })
	}
	// pdfa4 returns a valid PDF/A-4f fixture (PDF 2.0) modified by fn.
	pdfa4 := func(fn func(f *fixture)) *fixture {
		return mod(func(f *fixture) {
			f.version = "2.0"
			f.xmp = xmpWith(func(p *xmp.Params) { p.PDFAPart = "4" })
			fn(f)
		})
	}
	// xrechnung returns a valid XRECHNUNG fixture with the relationship rel.
	xrechnung := func(rel string) *fixture {
		return mod(func(f *fixture) {
			f.files[0].name, f.files[0].rel = "xrechnung.xml", rel
			f.files[0].data = ciiXML(invoice.URNXRechnung)
			f.xmp = xmpWith(func(p *xmp.Params) { p.DocumentFileName, p.ConformanceLevel = "xrechnung.xml", "XRECHNUNG" })
		})
	}
	pdfaOnly := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/"><pdfaid:part>3</pdfaid:part><pdfaid:conformance>B</pdfaid:conformance></rdf:Description>
</rdf:RDF></x:xmpmeta>`

	for _, tc := range []struct {
		name  string
		f     *fixture
		rules string // expected rules, comma separated
	}{
		{"valid", valid(en), ""},
		{"valid xref stream", mod(func(f *fixture) { f.opts.Layout, f.opts.ObjStm = 1, true }), ""},
		{"encrypted", mod(func(f *fixture) { f.encrypted = true }), "PDF-01"},
		{"no XMP", mod(func(f *fixture) { f.xmp = "" }), "PDF-02"},
		{"unreadable XMP", mod(func(f *fixture) { f.xmp = "<x:xmpmeta><rdf:RDF>" }), "PDF-03"},
		{"PDF/A-2", mod(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:part>3<", "<pdfaid:part>2<") }), "PDF-04"},
		{"no conformance", mod(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:conformance>B</pdfaid:conformance>", "") }), "PDF-05"},
		{"bad conformance", mod(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:conformance>B<", "<pdfaid:conformance>X<") }), "PDF-05"},
		{"PDF/A-3 level F", mod(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:conformance>B<", "<pdfaid:conformance>F<") }), "PDF-05"},
		{"PDF/A-4f", pdfa4(func(f *fixture) {}), ""},
		{"PDF/A-4e", pdfa4(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:conformance>F<", "<pdfaid:conformance>E<") }), ""},
		{"PDF/A-4 xref stream", pdfa4(func(f *fixture) { f.opts.Layout, f.opts.ObjStm = 1, true }), ""},
		{"plain PDF/A-4", pdfa4(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:conformance>F</pdfaid:conformance>", "") }), "PDF-05"},
		{"PDF/A-4 level B", pdfa4(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:conformance>F<", "<pdfaid:conformance>B<") }), "PDF-05"},
		{"PDF/A-4 without rev", pdfa4(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:rev>2020</pdfaid:rev>", "") }), "PDF-29"},
		{"PDF/A-4 bad rev", pdfa4(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:rev>2020<", "<pdfaid:rev>20<") }), "PDF-29"},
		{"PDF/A-4 in PDF 1.7", pdfa4(func(f *fixture) { f.version = "1.7" }), "PDF-30"},
		{"PDF/A-3 in PDF 2.0", mod(func(f *fixture) { f.version = "2.0" }), "PDF-30"},
		{"PDF/A-1", mod(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:part>3<", "<pdfaid:part>1<") }), "PDF-04"},
		{"no part", mod(func(f *fixture) { f.xmp = replaced(f.xmp, "<pdfaid:part>3</pdfaid:part>", "") }), "PDF-04"},
		{"no output intent", mod(func(f *fixture) { f.noOutputIntent = true }), "PDF-06"},
		{"no ICC profile", mod(func(f *fixture) { f.noDest = true }), "PDF-07"},
		{"no ID", mod(func(f *fixture) { f.noID = true }), "PDF-08"},
		{"no attachment", mod(func(f *fixture) { f.files = nil }), "PDF-09"},
		{"other name", mod(func(f *fixture) {
			f.files[0].name = "invoice.xml"
			f.xmp = xmpWith(func(p *xmp.Params) { p.DocumentFileName = "invoice.xml" })
		}), "PDF-10"},
		{"legacy name", mod(func(f *fixture) {
			f.files[0].name = "zugferd-invoice.xml"
			f.xmp = xmpWith(func(p *xmp.Params) { p.DocumentFileName = "zugferd-invoice.xml" })
		}), "PDF-11"},
		{"XRechnung named factur-x.xml", mod(func(f *fixture) { f.files[0].data = ciiXML(invoice.URNXRechnung) }), "PDF-11"},
		{"no AFRelationship", modFile(func(fl *file) { fl.rel = "" }), "PDF-12"},
		{"bad AFRelationship", modFile(func(fl *file) { fl.rel = "Supplement" }), "PDF-13"},
		{"Data for EN 16931", modFile(func(fl *file) { fl.rel = "Data" }), "PDF-14"},
		{"Source for EN 16931", modFile(func(fl *file) { fl.rel = "Source" }), ""},
		{"Alternative for MINIMUM", mod(func(f *fixture) {
			f.files[0].data = ciiXML(invoice.URNMinimum)
			f.xmp = xmpWith(func(p *xmp.Params) { p.ConformanceLevel = "MINIMUM" })
		}), "PDF-14"},
		{"Alternative for XRECHNUNG", xrechnung("Alternative"), ""},
		{"Source for XRECHNUNG", xrechnung("Source"), "PDF-40"},
		{"Data for XRECHNUNG", xrechnung("Data"), "PDF-40"},
		{"not in AF", modFile(func(fl *file) { fl.notInAF = true }), "PDF-15"},
		{"no MIME type", modFile(func(fl *file) { fl.subtype = "" }), "PDF-16"},
		{"PDF MIME type", modFile(func(fl *file) { fl.subtype = "application#2Fpdf" }), "PDF-16"},
		{"application/xml", modFile(func(fl *file) { fl.subtype = "application#2Fxml" }), "PDF-17"},
		{"no ModDate", modFile(func(fl *file) { fl.modDate = "" }), "PDF-18"},
		{"bad ModDate", modFile(func(fl *file) { fl.modDate = "2026-11-16" }), "PDF-19"},
		{"no UF", modFile(func(fl *file) { fl.noUF = true }), "PDF-20"},
		{"no F", modFile(func(fl *file) { fl.noF = true }), "PDF-20"},
		{"no fx schema", mod(func(f *fixture) { f.xmp, f.info = pdfaOnly, "<< >>" }), "PDF-21"},
		{"ORDER", mod(func(f *fixture) { f.xmp = replaced(f.xmp, ">INVOICE<", ">ORDER<") }), "PDF-22"},
		{"DocumentFileName", mod(func(f *fixture) { f.xmp = replaced(f.xmp, ">factur-x.xml<", ">invoice.xml<") }), "PDF-23"},
		{"ConformanceLevel mismatch", mod(func(f *fixture) { f.xmp = replaced(f.xmp, ">EN 16931<", ">BASIC<") }), "PDF-24"},
		{"unknown ConformanceLevel", mod(func(f *fixture) { f.xmp = replaced(f.xmp, ">EN 16931<", ">GOLD<") }), "PDF-24"},
		{"EXTENDED for EXTENDED-CTC-FR", mod(func(f *fixture) {
			f.files[0].data = ciiXML(invoice.URNExtendedCTCFR)
			f.xmp = xmpWith(func(p *xmp.Params) { p.ConformanceLevel = "EXTENDED" })
		}), ""},
		{"no extension schema", mod(func(f *fixture) { f.xmp = withoutExtension(f.xmp) }), "PDF-25"},
		{"property not described", mod(func(f *fixture) {
			f.xmp = replaced(f.xmp, "<pdfaProperty:name>Version</pdfaProperty:name>", "<pdfaProperty:name>Other</pdfaProperty:name>")
		}), "PDF-25"},
		{"no Version", mod(func(f *fixture) { f.xmp = replaced(f.xmp, "<fx:Version>1.0</fx:Version>", "") }), "PDF-26"},
		{"ZUGFeRD 1 namespace for CII", mod(func(f *fixture) {
			f.xmp = xmpWith(func(p *xmp.Params) { p.NamespaceURI = xmp.NSZUGFeRD1 })
		}), "PDF-27"},
		{"ZUGFeRD 1", mod(func(f *fixture) {
			f.files[0].name = "ZUGFeRD-invoice.xml"
			f.files[0].data = zf1XML("comfort")
			f.xmp = xmpWith(func(p *xmp.Params) {
				p.NamespaceURI, p.DocumentFileName, p.ConformanceLevel = xmp.NSZUGFeRD1, "ZUGFeRD-invoice.xml", "COMFORT"
			})
		}), "PDF-11"},
		{"ZUGFeRD 1 level mismatch", mod(func(f *fixture) {
			f.files[0].name = "ZUGFeRD-invoice.xml"
			f.files[0].data = zf1XML("comfort")
			f.xmp = xmpWith(func(p *xmp.Params) {
				p.NamespaceURI, p.DocumentFileName, p.ConformanceLevel = xmp.NSZUGFeRD1, "ZUGFeRD-invoice.xml", "BASIC"
			})
		}), "PDF-11,PDF-24"},
		{"two invoices", mod(func(f *fixture) {
			f.files = append(f.files, file{name: "xrechnung.xml", data: ciiXML(invoice.URNXRechnung), rel: "Alternative", subtype: "text#2Fxml"})
		}), "PDF-28"},
		{"many problems", mod(func(f *fixture) {
			f.noID, f.noOutputIntent = true, true
			f.files[0].rel, f.files[0].notInAF, f.files[0].subtype = "", true, ""
		}), "PDF-06,PDF-08,PDF-12,PDF-15,PDF-16"},
	} {
		t.Run(tc.name, func(t *testing.T) { checkContainerRules(t, tc.f, tc.rules) })
	}
}

// checkContainerRules checks that CheckContainer reports the rules want
// (sorted, comma separated) for the fixture f.
func checkContainerRules(t *testing.T, f *fixture, want string) {
	t.Helper()
	a, findings, err := CheckContainer(f.build())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(rulesOf(findings), ","); got != want {
		t.Errorf("rules %q, want %q", got, want)
		for _, f := range findings {
			t.Log(f)
		}
	}
	noAttachment := strings.HasPrefix(want, "PDF-01") || strings.Contains(want, "PDF-09")
	if (a == nil) != noAttachment {
		t.Errorf("attachment %v", a)
	}
	for _, f := range findings {
		if f.Source != rules.SourcePDF || f.Message == "" || f.Severity != containerSeverity(f.Rule) {
			t.Errorf("finding %+v", f)
		}
	}
}

// TestCheckRelationship covers the national usage of the /AFRelationship in
// Check: the rule sets of validate.Options select the French and German
// usages (Factur-X 1.09, 6.2.2). Only the container findings are compared:
// the XML of the fixtures is minimal.
func TestCheckRelationship(t *testing.T) {
	auto := &validate.Options{Auto: true}
	for _, tc := range []struct {
		name   string
		urn    string
		seller string
		rel    string
		opts   *validate.Options
		rules  string
	}{
		{"Source, seller in Germany, Auto", invoice.URNEN16931, "DE", "Source", auto, "PDF-40"},
		{"Data, seller in Germany, Auto", invoice.URNBasic, "DE", "Data", auto, "PDF-40"},
		{"Alternative, seller in Germany, Auto", invoice.URNExtended, "DE", "Alternative", auto, ""},
		{"Source, seller in Germany, no option", invoice.URNEN16931, "DE", "Source", nil, ""},
		{"Data, seller in Germany, no option", invoice.URNEN16931, "DE", "Data", nil, "PDF-14"},
		{"Data for MINIMUM, seller in Germany, Auto", invoice.URNMinimum, "DE", "Data", auto, ""},
		{"Alternative for BASIC WL, seller in Germany, Auto", invoice.URNBasicWL, "DE", "Alternative", auto, "PDF-14"},
		{"Source, German rules", invoice.URNEN16931, "FR", "Source", &validate.Options{Rules: validate.Default | validate.Germany}, "PDF-40"},
		{"Source, BMF profiles", invoice.URNEN16931, "FR", "Source", &validate.Options{Rules: validate.Default | validate.GermanyProfile}, "PDF-40"},
		{"Data, seller in France, Auto", invoice.URNEN16931, "FR", "Data", auto, ""},
		{"Data, French rules", invoice.URNExtendedCTCFR, "BE", "Data", &validate.Options{Rules: validate.Default | validate.France}, ""},
		{"Data, seller in France, no option", invoice.URNEN16931, "FR", "Data", nil, "PDF-14"},
		{"Source, seller in France, Auto", invoice.URNEN16931, "FR", "Source", auto, ""},
		{"Data, French and German rules", invoice.URNEN16931, "FR", "Data", &validate.Options{Rules: validate.Default | validate.France | validate.Germany}, "PDF-40"},
		{"Source for XRECHNUNG, no option", invoice.URNXRechnung, "DE", "Source", nil, "PDF-40"},
		{"Source for XRECHNUNG, rules without Germany", invoice.URNXRechnung, "DE", "Source", &validate.Options{Rules: validate.Schema}, "PDF-40"},
		{"Source, profile forced to XRECHNUNG", invoice.URNEN16931, "FR", "Source", &validate.Options{Profile: invoice.ProfileXRechnung}, "PDF-40"},
		{"Source, profile forced to XRECHNUNG, French rules", invoice.URNEN16931, "FR", "Source", &validate.Options{Profile: invoice.ProfileXRechnung, Rules: validate.Default | validate.France}, "PDF-40"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := invoice.ProfileFromURN(tc.urn)
			f := valid(sellerXML(tc.urn, tc.seller))
			f.files[0].name, f.files[0].rel = p.Filename(), tc.rel
			f.xmp = xmpWith(func(x *xmp.Params) { x.DocumentFileName, x.ConformanceLevel = p.Filename(), p.ConformanceLevel() })
			r, a, err := Check(f.build(), tc.opts)
			if err != nil || a == nil {
				t.Fatal(a, err)
			}
			var container []rules.Finding
			for _, f := range r.Findings {
				if f.Source == rules.SourcePDF {
					container = append(container, f)
				}
			}
			if got := strings.Join(rulesOf(container), ","); got != tc.rules {
				t.Errorf("rules %q, want %q", got, tc.rules)
				for _, f := range container {
					t.Log(f)
				}
			}
		})
	}
}

// TestCheckXML checks that Check validates the XML it parsed for the
// national rules as validate.XML does, including the XML that cannot be
// parsed or is not a CII invoice.
func TestCheckXML(t *testing.T) {
	en := readTestdata(t, "EN16931_Einfach.cii.xml")
	opts := &validate.Options{Auto: true}
	for name, x := range map[string][]byte{
		"EN 16931":   en,
		"malformed":  []byte("<rsm:CrossIndustryInvoice><ram:ID>"),
		"ZUGFeRD 1":  zf1XML("comfort"),
		"empty root": []byte("<a/>"),
	} {
		r, a, err := Check(valid(x).build(), opts)
		if err != nil || a == nil {
			t.Fatalf("%s: %v %v", name, a, err)
		}
		var got []rules.Finding
		for _, f := range r.Findings {
			if f.Source != rules.SourcePDF {
				got = append(got, f)
			}
		}
		want := validate.XML(x, opts)
		if fmt.Sprint(got) != fmt.Sprint(want.Findings) || r.Profile != want.Profile {
			t.Errorf("%s: Check gives %s %v, validate.XML %s %v", name, r.Profile, got, want.Profile, want.Findings)
		}
		if name != "EN 16931" && len(got) == 0 {
			t.Errorf("%s: no XML finding", name)
		}
	}
}

// TestCheckContainerPDFA covers the PDF/A requirements checked without
// rendering: fonts, JavaScript and actions, /Info and XMP, other embedded
// files and filters. The catalog is object 1, the page tree object 2 and
// the page object 3.
func TestCheckContainerPDFA(t *testing.T) {
	en := ciiXML(invoice.URNEN16931)
	mod := func(fn func(f *fixture)) *fixture {
		f := valid(en)
		fn(f)
		return f
	}
	// extra returns a valid fixture whose catalog and page get the entries
	// returned by fn.
	extra := func(fn func(b *pdftest.Builder) (catalog, page string)) *fixture {
		return mod(func(f *fixture) { f.extra = fn })
	}
	catalog := func(entries string) *fixture {
		return extra(func(*pdftest.Builder) (string, string) { return entries, "" })
	}
	page := func(entries string) *fixture {
		return extra(func(*pdftest.Builder) (string, string) { return "", entries })
	}
	// pageFont returns a valid fixture whose page uses the font written by
	// fn as /F1.
	pageFont := func(fn func(b *pdftest.Builder) string) *fixture {
		return extra(func(b *pdftest.Builder) (string, string) {
			return "", fmt.Sprintf("/Resources << /Font << /F1 %d 0 R >> >>", b.Add(fn(b)))
		})
	}
	font := func(body string) func(*pdftest.Builder) string {
		return func(*pdftest.Builder) string { return body }
	}
	// descriptor adds a font descriptor whose font program, when key is
	// set, is in the entry key.
	descriptor := func(b *pdftest.Builder, key string) int {
		d := "<< /Type /FontDescriptor /FontName /Font /Flags 32"
		if key != "" {
			d += fmt.Sprintf(" /%s %d 0 R", key, b.AddStream("<< >>", []byte("font program")))
		}
		return b.Add(d + " >>")
	}
	// form adds a form XObject whose resources use a non-embedded font.
	form := func(b *pdftest.Builder) int {
		f := b.Add("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
		return b.AddStream(fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /Resources << /Font << /F1 %d 0 R >> >> >>", f), []byte("BT /F1 1 Tf (a) Tj ET"))
	}
	info := func(entries string) *fixture {
		return mod(func(f *fixture) { f.info = strings.Replace(testInfo, ">>", entries+" >>", 1) })
	}
	attachment := func(fl file) *fixture {
		return mod(func(f *fixture) { f.files = append(f.files, fl) })
	}
	notes := func(rel, subtype string) file {
		return file{name: "notes.txt", data: []byte("notes"), rel: rel, subtype: subtype, modDate: "D:20261116120000+01'00'"}
	}
	// listed adds notes.txt, left out of the catalog /AF, and the objects
	// written by fn, which gets the object number of its file specification.
	listed := func(fn func(b *pdftest.Builder, spec int) (catalog, page string)) *fixture {
		return mod(func(f *fixture) {
			fl := notes("Supplement", "text#2Fplain")
			fl.notInAF = true
			f.files = append(f.files, fl)
			f.extra = func(b *pdftest.Builder) (string, string) { return fn(b, f.specs[1]) }
		})
	}
	js := "<< /S /JavaScript /JS (app.alert\\(1\\)) >>"

	for _, tc := range []struct {
		name  string
		f     *fixture
		rules string // expected rules, comma separated
	}{
		// PDF-31: embedded fonts.
		{"standard font not embedded", pageFont(font("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")), "PDF-31"},
		{"embedded TrueType font", pageFont(func(b *pdftest.Builder) string {
			return fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /ABCDEF+Arial /FontDescriptor %d 0 R >>", descriptor(b, "FontFile2"))
		}), ""},
		{"embedded CFF font", pageFont(func(b *pdftest.Builder) string {
			return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /ABCDEF+Font /FontDescriptor %d 0 R >>", descriptor(b, "FontFile3"))
		}), ""},
		{"descriptor without font file", pageFont(func(b *pdftest.Builder) string {
			return fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /Arial /FontDescriptor %d 0 R >>", descriptor(b, ""))
		}), "PDF-31"},
		{"embedded composite font", pageFont(func(b *pdftest.Builder) string {
			cid := b.Add(fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /ABCDEF+Arial /FontDescriptor %d 0 R >>", descriptor(b, "FontFile2")))
			return fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /ABCDEF+Arial /Encoding /Identity-H /DescendantFonts [%d 0 R] >>", cid)
		}), ""},
		{"composite font not embedded", pageFont(func(b *pdftest.Builder) string {
			cid := b.Add(fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /Arial /FontDescriptor %d 0 R >>", descriptor(b, "")))
			return fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /Arial /Encoding /Identity-H /DescendantFonts [%d 0 R] >>", cid)
		}), "PDF-31"},
		{"Type 3 font", pageFont(font("<< /Type /Font /Subtype /Type3 /FontBBox [0 0 1 1] /FontMatrix [1 0 0 1 0 0] /CharProcs << >> /Encoding << >> /FirstChar 0 /LastChar 0 /Widths [0] >>")), ""},
		{"font of a form XObject", extra(func(b *pdftest.Builder) (string, string) {
			return "", fmt.Sprintf("/Resources << /XObject << /X1 %d 0 R >> >>", form(b))
		}), "PDF-31"},
		{"font of an annotation appearance", extra(func(b *pdftest.Builder) (string, string) {
			return "", fmt.Sprintf("/Annots [<< /Type /Annot /Subtype /FreeText /Rect [0 0 1 1] /AP << /N %d 0 R >> >>]", form(b))
		}), "PDF-31"},
		{"inherited resources", mod(func(f *fixture) {
			f.pagesExtra = " /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >>"
		}), "PDF-31"},
		{"font used twice", extra(func(b *pdftest.Builder) (string, string) {
			f := b.Add("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
			return "", fmt.Sprintf("/Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> /XObject << /X1 %d 0 R >> >>", f, f, form(b))
		}), "PDF-31"},

		// PDF-32 to PDF-34: JavaScript and actions.
		{"document JavaScript", mod(func(f *fixture) { f.names = " /JavaScript << /Names [(init) " + js + "] >>" }), "PDF-32"},
		{"JavaScript open action", catalog("/OpenAction " + js), "PDF-33"},
		{"destination open action", catalog("/OpenAction [3 0 R /Fit]"), ""},
		{"allowed named action", catalog("/OpenAction << /S /Named /N /NextPage >>"), ""},
		{"forbidden named action", catalog("/OpenAction << /S /Named /N /Print >>"), "PDF-33"},
		{"chained ImportData action", catalog("/OpenAction << /S /GoTo /D [3 0 R /Fit] /Next << /S /ImportData /F (data.fdf) >> >>"), "PDF-33"},
		{"chain of actions", catalog("/OpenAction << /S /GoTo /D [3 0 R /Fit] /Next [<< /S /URI /URI (https://example.com) >> << /S /ResetForm >>] >>"), "PDF-33"},
		{"Launch link", page("/Annots [<< /Type /Annot /Subtype /Link /Rect [0 0 1 1] /A << /S /Launch /F (calc.exe) >> >>]"), "PDF-33"},
		{"URI link", page("/Annots [<< /Type /Annot /Subtype /Link /Rect [0 0 1 1] /A << /S /URI /URI (https://example.com) >> >>]"), ""},
		{"outline action", extra(func(b *pdftest.Builder) (string, string) {
			root, item := b.Reserve(), b.Reserve()
			b.Set(root, fmt.Sprintf("<< /Type /Outlines /First %d 0 R /Last %d 0 R /Count 1 >>", item, item))
			b.Set(item, fmt.Sprintf("<< /Title (Total) /Parent %d 0 R /A %s >>", root, js))
			return fmt.Sprintf("/Outlines %d 0 R", root), ""
		}), "PDF-33"},
		{"catalog additional actions", catalog("/AA << /WC " + js + " >>"), "PDF-34"},
		{"page additional actions", page("/AA << /O << /S /GoTo /D [3 0 R /Fit] >> >>"), "PDF-34"},
		{"widget additional actions", extra(func(b *pdftest.Builder) (string, string) {
			// A field merged with its widget is reported once.
			w := b.Add("<< /Type /Annot /Subtype /Widget /FT /Tx /T (amount) /Rect [0 0 1 1] /P 3 0 R /AA << /K " + js + " >> >>")
			return fmt.Sprintf("/AcroForm << /Fields [%d 0 R] >>", w), fmt.Sprintf("/Annots [%d 0 R]", w)
		}), "PDF-34"},
		{"field additional actions", extra(func(b *pdftest.Builder) (string, string) {
			parent, w := b.Reserve(), b.Reserve()
			b.Set(parent, fmt.Sprintf("<< /FT /Tx /T (amount) /Kids [%d 0 R] /AA << /F %s >> >>", w, js))
			b.Set(w, fmt.Sprintf("<< /Type /Annot /Subtype /Widget /Parent %d 0 R /Rect [0 0 1 1] /P 3 0 R >>", parent))
			return fmt.Sprintf("/AcroForm << /Fields [%d 0 R] >>", parent), fmt.Sprintf("/Annots [%d 0 R]", w)
		}), "PDF-34"},
		{"no invoice, JavaScript", mod(func(f *fixture) {
			f.files = nil
			f.names = " /JavaScript << /Names [(init) " + js + "] >>"
		}), "PDF-09,PDF-32"},

		// PDF-35: /Info and XMP.
		{"Info title differs", mod(func(f *fixture) { f.info = strings.Replace(testInfo, "(Facture)", "(Avoir)", 1) }), "PDF-35"},
		{"Info author not in XMP", info("/Author (Jean Dupont)"), "PDF-35"},
		{"Info ModDate differs", mod(func(f *fixture) { f.info = strings.Replace(testInfo, "120000+01'00'", "130000+01'00'", 1) }), "PDF-35"},
		{"Info ModDate in another time zone", mod(func(f *fixture) { f.info = strings.Replace(testInfo, "120000+01'00'", "110000Z", 1) }), ""},
		{"Info empty subject", info("/Subject ()"), ""},
		{"Info title in UTF-16", mod(func(f *fixture) {
			f.info = strings.Replace(testInfo, "(Facture)", "<FEFF0046006100630074007500720065>", 1)
		}), ""},
		{"Info producer and creation date differ", mod(func(f *fixture) {
			f.info = strings.Replace(testInfo, "(pdftest)", "(other)", 1)
			f.info = strings.Replace(f.info, "(D:20260101000000Z)", "(D:20250101000000Z)", 1)
		}), "PDF-35,PDF-35"},
		{"no Info", mod(func(f *fixture) { f.info = "null" }), ""},
		{"PDF/A-4 Info not checked", mod(func(f *fixture) {
			f.version, f.info = "2.0", "<< /Title (Avoir) >>"
			f.xmp = xmpWith(func(p *xmp.Params) { p.PDFAPart = "4" })
		}), ""},

		// PDF-36, PDF-37, PDF-39: other embedded files.
		{"other attachment", attachment(notes("Supplement", "text#2Fplain")), ""},
		{"other attachment without AFRelationship", attachment(notes("", "text#2Fplain")), "PDF-36"},
		{"other attachment without MIME type", attachment(notes("Supplement", "")), "PDF-37"},
		{"other attachment not in AF", listed(func(*pdftest.Builder, int) (string, string) { return "", "" }), "PDF-39"},
		{"other attachment in the page AF", listed(func(b *pdftest.Builder, spec int) (string, string) {
			return "", fmt.Sprintf("/AF [%d 0 R]", spec)
		}), ""},
		{"other attachment in an annotation AF", listed(func(b *pdftest.Builder, spec int) (string, string) {
			return "", fmt.Sprintf("/Annots [<< /Type /Annot /Subtype /Text /Rect [0 0 1 1] /AF [%d 0 R] >>]", spec)
		}), ""},
		{"other attachment in a form XObject AF", listed(func(b *pdftest.Builder, spec int) (string, string) {
			x := b.AddStream(fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 1 1] /AF [%d 0 R] >>", spec), nil)
			return "", fmt.Sprintf("/Resources << /XObject << /X1 %d 0 R >> >>", x)
		}), ""},
		{"other attachment in a property list AF", listed(func(b *pdftest.Builder, spec int) (string, string) {
			return "", fmt.Sprintf("/Resources << /Properties << /MC0 << /AF [%d 0 R] >> >> >>", spec)
		}), ""},
		{"other attachment in a structure element AF", listed(func(b *pdftest.Builder, spec int) (string, string) {
			table := b.Add(fmt.Sprintf("<< /Type /StructElem /S /Table /K [0] /AF %d 0 R >>", b.Add(fmt.Sprintf("[%d 0 R]", spec))))
			sect := b.Add(fmt.Sprintf("<< /Type /StructElem /S /Sect /K [<< /Type /StructElem /S /P /K 1 >> %d 0 R] >>", table))
			return fmt.Sprintf("/StructTreeRoot << /Type /StructTreeRoot /K << /Type /StructElem /S /Document /K %d 0 R >> >>", sect), ""
		}), ""},
		{"other attachment in an unrelated AF", listed(func(b *pdftest.Builder, spec int) (string, string) {
			// An /AF array the scan does not look at (here in the page-piece
			// dictionary of the page) does not count.
			return "", fmt.Sprintf("/PieceInfo << /App << /AF [%d 0 R] >> >>", spec)
		}), "PDF-39"},
		{"direct file specification in the page AF", extra(func(b *pdftest.Builder) (string, string) {
			ef := b.AddStream("<< /Type /EmbeddedFile /Subtype /text#2Fplain /Params << /ModDate (D:20261116120000+01'00') >> >>", []byte("notes"))
			return "", fmt.Sprintf("/AF [<< /Type /Filespec /F (b.txt) /UF (b.txt) /AFRelationship /Supplement /EF << /F %d 0 R >> >>]", ef)
		}), ""},
		{"file attachment annotation", extra(func(b *pdftest.Builder) (string, string) {
			ef := b.AddStream("<< /Type /EmbeddedFile >>", []byte("notes"))
			return "", fmt.Sprintf("/Annots [<< /Type /Annot /Subtype /FileAttachment /Rect [0 0 1 1] /FS << /Type /Filespec /F (a.txt) /UF (a.txt) /EF << /F %d 0 R >> >> >>]", ef)
		}), "PDF-36,PDF-37,PDF-39"},
		{"file attachment annotation with AF", extra(func(b *pdftest.Builder) (string, string) {
			ef := b.AddStream("<< /Type /EmbeddedFile /Subtype /text#2Fplain >>", []byte("notes"))
			fs := b.Add(fmt.Sprintf("<< /Type /Filespec /F (a.txt) /UF (a.txt) /AFRelationship /Supplement /EF << /F %d 0 R >> >>", ef))
			return "", fmt.Sprintf("/Annots [<< /Type /Annot /Subtype /FileAttachment /Rect [0 0 1 1] /FS %d 0 R /AF [%d 0 R] >>]", fs, fs)
		}), ""},

		// PDF-38: LZWDecode.
		{"LZW content stream", extra(func(b *pdftest.Builder) (string, string) {
			return "", fmt.Sprintf("/Contents %d 0 R", b.AddStream("<< /Filter /LZWDecode >>", []byte("data")))
		}), "PDF-38"},
		{"LZW image in a filter chain", extra(func(b *pdftest.Builder) (string, string) {
			im := b.AddStream("<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter [/ASCII85Decode /LZWDecode] >>", []byte("data"))
			return "", fmt.Sprintf("/Contents [%d 0 R] /Resources << /XObject << /Im1 %d 0 R >> >>", b.AddStream("<< >>", []byte("q 1 0 0 1 0 0 cm /Im1 Do Q")), im)
		}), "PDF-38"},
		{"LZW other attachment", attachment(file{name: "notes.txt", data: []byte("notes"), rel: "Supplement", subtype: "text#2Fplain", lzw: true}), "PDF-38"},
	} {
		t.Run(tc.name, func(t *testing.T) { checkContainerRules(t, tc.f, tc.rules) })
	}
}

func TestCheckContainerPDFALimit(t *testing.T) {
	f := valid(ciiXML(invoice.URNEN16931))
	f.extra = func(b *pdftest.Builder) (string, string) {
		var fonts []string
		for i := 0; i < maxReported+5; i++ {
			fonts = append(fonts, fmt.Sprintf("/F%d %d 0 R", i, b.Add(fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /Font%d >>", i))))
		}
		return "", "/Resources << /Font << " + strings.Join(fonts, " ") + " >> >>"
	}
	_, findings, err := CheckContainer(f.build())
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != maxReported+1 || findings[maxReported].Message != "5 more occurrences not listed" {
		t.Errorf("%d findings", len(findings))
		for _, f := range findings {
			t.Log(f)
		}
	}
}

func TestCheckContainerNotPDF(t *testing.T) {
	if _, _, err := CheckContainer([]byte("hello")); err == nil {
		t.Fatal("no error")
	}
}

func TestContainerCatalog(t *testing.T) {
	for i, r := range ContainerCatalog {
		if want := fmt.Sprintf("PDF-%02d", i+1); r.ID != want {
			t.Errorf("rule %d: ID %s, want %s", i, r.ID, want)
		}
		if r.Severity != rules.Error && r.Severity != rules.Warning && r.Severity != rules.Info {
			t.Errorf("%s: severity %v", r.ID, r.Severity)
		}
		if r.Description == "" || !strings.HasSuffix(r.Description, ".") {
			t.Errorf("%s: description %q", r.ID, r.Description)
		}
	}
}

func TestStrictDate(t *testing.T) {
	for s, want := range map[string]bool{
		"D:20261116120000+01'00'":  true,
		"D:20261116120000+01'00":   true,
		"D:20261116120000+0100":    true,
		"D:20261116120000-05":      true,
		"D:20261116120000Z":        true,
		"D:20261116120000Z00'00'":  true,
		"D:2026":                   true,
		"D:202611":                 true,
		"20261116120000":           false,
		"D:2026-11-16":             false,
		"D:20261316":               false,
		"D:20261116250000":         false,
		"D:20261116120000+01'00'x": false,
		"D:20261116120000+1":       false,
		"":                         false,
	} {
		if got := strictDate(s); got != want {
			t.Errorf("strictDate(%q) = %v", s, got)
		}
	}
}

// TestCheckContainerCorpus checks every PDF of FX_CORPUS: all of them must
// be readable and have an invoice attachment, except those of the
// unstructured directory, which must be reported as having none (PDF-09).
// The findings are logged by rule.
func TestCheckContainerCorpus(t *testing.T) {
	root := os.Getenv("FX_CORPUS")
	if root == "" {
		t.Skip("FX_CORPUS not set")
	}
	count := map[string]int{}
	files := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".pdf") {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files++
		a, findings, err := CheckContainer(data)
		if unstructuredPDF(rel) {
			noInvoice := slices.ContainsFunc(findings, func(f rules.Finding) bool {
				return f.Rule == "PDF-09" && f.Path == "catalog /Names /EmbeddedFiles"
			})
			if err != nil || a != nil || !noInvoice {
				t.Errorf("%s: unstructured PDF, want no attachment and PDF-09, got %v %v", rel, err, findings)
			}
			return nil
		}
		if err != nil || a == nil {
			t.Errorf("%s: %v %v", rel, err, findings)
			return nil
		}
		for _, f := range findings {
			count[f.Rule]++
			if f.Severity == rules.Error {
				t.Logf("%s: %s", rel, f)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%s (%s): %d files", k, containerSeverity(k), count[k])
	}
	t.Logf("%d PDFs", files)
}
