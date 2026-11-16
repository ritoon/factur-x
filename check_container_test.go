package facturx

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmp"
	"github.com/ritoon/factur-x/invoice"
)

// xmpWith returns a valid Factur-X XMP packet modified by fn.
func xmpWith(fn func(p *xmp.Params)) string {
	p := xmp.Params{DocumentType: "INVOICE", DocumentFileName: "factur-x.xml", Version: "1.0", ConformanceLevel: "EN 16931", Date: testDate}
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
		{"not in AF", modFile(func(fl *file) { fl.notInAF = true }), "PDF-15"},
		{"no MIME type", modFile(func(fl *file) { fl.subtype = "" }), "PDF-16"},
		{"PDF MIME type", modFile(func(fl *file) { fl.subtype = "application#2Fpdf" }), "PDF-16"},
		{"application/xml", modFile(func(fl *file) { fl.subtype = "application#2Fxml" }), "PDF-17"},
		{"no ModDate", modFile(func(fl *file) { fl.modDate = "" }), "PDF-18"},
		{"bad ModDate", modFile(func(fl *file) { fl.modDate = "2026-11-16" }), "PDF-19"},
		{"no UF", modFile(func(fl *file) { fl.noUF = true }), "PDF-20"},
		{"no F", modFile(func(fl *file) { fl.noF = true }), "PDF-20"},
		{"no fx schema", mod(func(f *fixture) { f.xmp = pdfaOnly }), "PDF-21"},
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
			f.files = append(f.files, file{name: "xrechnung.xml", data: ciiXML(invoice.URNXRechnung), rel: "Alternative"})
		}), "PDF-28"},
		{"many problems", mod(func(f *fixture) {
			f.noID, f.noOutputIntent = true, true
			f.files[0].rel, f.files[0].notInAF, f.files[0].subtype = "", true, ""
		}), "PDF-06,PDF-08,PDF-12,PDF-15,PDF-16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, findings, err := CheckContainer(tc.f.build())
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(rulesOf(findings), ","); got != tc.rules {
				t.Errorf("rules %q, want %q", got, tc.rules)
				for _, f := range findings {
					t.Log(f)
				}
			}
			noAttachment := tc.rules == "PDF-01" || tc.rules == "PDF-09"
			if (a == nil) != noAttachment {
				t.Errorf("attachment %v", a)
			}
			for _, f := range findings {
				if f.Source != rules.SourcePDF || f.Message == "" || f.Severity != containerSeverity(f.Rule) {
					t.Errorf("finding %+v", f)
				}
			}
		})
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
// be readable and have an invoice attachment. The findings are logged by
// rule.
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
