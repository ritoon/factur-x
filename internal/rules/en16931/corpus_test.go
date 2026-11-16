package en16931

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// check parses an invoice and runs Check on it.
func check(t testing.TB, data []byte, profile invoice.Profile) *rules.Report {
	t.Helper()
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := &rules.Report{}
	Check(&rules.Checker{Doc: rules.NewDoc(root, profile), Report: r, Source: rules.SourceEN16931})
	return r
}

// errorRules returns the sorted distinct rules of the errors of r.
func errorRules(r *rules.Report) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range r.Errors() {
		if !seen[f.Rule] {
			seen[f.Rule] = true
			out = append(out, f.Rule)
		}
	}
	sort.Strings(out)
	return out
}

// TestCorpusParity checks the 26 valid CII samples of the XML-Rechnung corpus
// (ZUGFeRD/FeRD and KoSIT): the only errors are the ones reported by the
// reference validator.
func TestCorpusParity(t *testing.T) {
	dir := os.Getenv("FX_CORPUS")
	if dir == "" {
		t.Skip("FX_CORPUS not set")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "XML-Rechnung", "CII", "*.cii.xml"))
	if len(files) == 0 {
		t.Skip("no CII sample in FX_CORPUS")
	}
	// Errors also reported by the reference validators (papierleicht, i.e.
	// the KoSIT configuration for XRECHNUNG and the FeRD XSLT otherwise):
	// the electronic address scheme 9958 (Leitweg-ID) is not in the EAS
	// code list of 1.3.17, and the KoSIT configuration makes CII-SR-465
	// (only one buyer reference BT-41: PersonName and DepartmentName of
	// the seller contact) an error.
	want := map[string][]string{
		"EN16931_ElektronischeAdresse.cii.xml":       {"BR-CL-25"},
		"EN16931_RechnungsUebertragung.cii.xml":      {"BR-CL-25"},
		"XRECHNUNG_Betriebskostenabrechnung.cii.xml": {"CII-SR-465"},
		"XRECHNUNG_Einfach.cii.xml":                  {"CII-SR-465"},
		"XRECHNUNG_Elektron.cii.xml":                 {"CII-SR-465"},
		"XRECHNUNG_Reisekostenabrechnung.cii.xml":    {"CII-SR-465"},
	}
	n := 0
	for _, f := range files {
		name := filepath.Base(f)
		if strings.HasPrefix(name, "not_validating") {
			continue
		}
		n++
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		r := check(t, data, invoice.ProfileUnknown)
		got := strings.Join(errorRules(r), " ")
		if exp := strings.Join(want[name], " "); got != exp {
			t.Errorf("%s: errors %q, want %q", name, got, exp)
			for _, e := range r.Errors() {
				t.Logf("  %s", e)
			}
		}
		if testing.Verbose() {
			for _, w := range r.Findings {
				if w.Severity != rules.Error {
					t.Logf("%s: %s", name, w)
				}
			}
		}
	}
	if n != 26 {
		t.Errorf("%d samples, want 26", n)
	}
}

// corpusXMLErrors lists the errors expected on the XML invoices extracted
// from the PDF corpus, by file name. All of them are also reported by the
// official CEN XSLT 1.3.17.
var corpusXMLErrors = map[string]string{
	// Electronic address scheme 9958 (Leitweg-ID), not in the EAS list.
	"EN16931_ElektronischeAdresse.xml":              "BR-CL-25",
	"EN16931_RechnungsUebertragung.xml":             "BR-CL-25",
	"zugferd_2p0_EN16931_ElektronischeAdresse.xml":  "BR-CL-25",
	"zugferd_2p0_EN16931_RechnungsUebertragung.xml": "BR-CL-25",
	"zugferd_2p1_EN16931_ElektronischeAdresse.xml":  "BR-CL-25",
	"zugferd_2p1_EN16931_RechnungsUebertragung.xml": "BR-CL-25",
	// XRechnung invoices with PersonName and DepartmentName in a contact:
	// an error in the KoSIT configuration.
	"XRECHNUNG_Betriebskostenabrechnung.xml":                              "CII-SR-465",
	"XRECHNUNG_Einfach.xml":                                               "CII-SR-465",
	"XRECHNUNG_Elektron.xml":                                              "CII-SR-465",
	"XRECHNUNG_Reisekostenabrechnung.xml":                                 "CII-SR-465",
	"zugferd_2p1_EN16931_Betriebskostenabrechnung_XRechnung_embedded.xml": "CII-SR-465",
	"zugferd_2p1_EN16931_Elektron_XRechnung.xml":                          "CII-SR-465",
	"zugferd_2p1_EN16931_Reisekostenabrechnung_XRechnung_embedded.xml":    "CII-SR-465",
	// EAS code 9930 used as an ICD scheme for the seller identifier.
	"MustangGnuaccountingBeispielRE-20190610_507.xml": "BR-CL-11",
	// Unknown specification identifier, hence the full CEN rules, and
	// currencyID attributes on amounts other than BT-110 and BT-111.
	"Avoir_FR_type381_BASIC.xml": "CII-DT-031",
}

// TestCorpusXML checks the XML invoices extracted from the PDF corpus: the
// only errors are the ones of corpusXMLErrors. ZUGFeRD 1 invoices, whose
// root is not rsm:CrossIndustryInvoice, are skipped.
func TestCorpusXML(t *testing.T) {
	dir := os.Getenv("FX_CORPUS_XML")
	if dir == "" {
		t.Skip("FX_CORPUS_XML not set")
	}
	n := 0
	err := filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(p, ".xml") {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		root, err := xmltree.Parse(data)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			return nil
		}
		if root.Name != "CrossIndustryInvoice" {
			return nil
		}
		n++
		r := check(t, data, invoice.ProfileUnknown)
		name := filepath.Base(p)
		if got, want := strings.Join(errorRules(r), " "), corpusXMLErrors[name]; got != want {
			t.Errorf("%s: errors %q, want %q", p, got, want)
			for _, e := range r.Errors() {
				t.Logf("  %s", e)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Skip("no CII invoice in FX_CORPUS_XML")
	}
	t.Logf("%d CII invoices", n)
}
