package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/cii"
	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
	ublsyntax "github.com/ritoon/zerechnung/ubl"
)

const (
	nlciusInvoice    = "../internal/rules/nl/testdata/nlcius.xml"     // NLCIUS, seller in the Netherlands
	nlciusUBLInvoice = "../internal/rules/nl/testdata/nlcius.ubl.xml" // nlciusInvoice in UBL 2.1 (SI-UBL 2.0)
)

// TestNLCIUS checks the NLCIUS invoices of internal/rules/nl with every
// rule set that applies, and when the Dutch rules apply: on an NLCIUS
// invoice (BT-24) whatever the rule sets, or on request, never by default
// nor with Auto.
func TestNLCIUS(t *testing.T) {
	for file, buyerRef := range map[string]string{
		nlciusInvoice:    "<ram:BuyerReference>PO-4512</ram:BuyerReference>",
		nlciusUBLInvoice: "<cbc:BuyerReference>PO-4512</cbc:BuyerReference>",
	} {
		doc := readXML(t, file)
		r := XML([]byte(doc), &Options{Auto: true})
		if r.Profile != invoice.ProfileNLCIUS || len(r.Findings) > 0 {
			t.Errorf("%s: profile %s\n%s", file, r.Profile, r)
		}
		// Without buyer reference, BR-NL-2 is reported whatever the rule
		// sets, at the context of its rule: the header trade agreement in
		// CII, the root element in UBL.
		if !strings.Contains(doc, buyerRef) {
			t.Fatalf("%s: BT-10 not found", file)
		}
		noBuyerRef := []byte(strings.Replace(doc, buyerRef, "", 1))
		for _, opts := range []*Options{
			nil,
			{Auto: true},
			{Rules: Default | France},
			{Rules: Schema | EN16931},
		} {
			r := XML(noBuyerRef, opts)
			f := findings(r, "BR-NL-")
			if len(f) != 1 || f[0].Rule != "BR-NL-2" || f[0].Source != SourceNL || f[0].Severity != Error ||
				f[0].Path != "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement" && f[0].Path != "/Invoice" {
				t.Errorf("%s without BT-10 (%+v):\n%s", file, opts, r)
			}
		}
	}

	// NLCIUS is detected in BT-24, also with the G-account extension.
	gaccount := strings.Replace(readXML(t, nlciusUBLInvoice), invoice.URNNLCIUS, invoice.URNNLCIUS+"#conformant#urn:fdc:nen.nl:gaccount:v1.0", 1)
	if r := XML([]byte(gaccount), nil); r.Profile != invoice.ProfileNLCIUS || len(r.Findings) > 0 {
		t.Errorf("G-account: profile %s\n%s", r.Profile, r)
	}

	// The Dutch rules apply to other invoices on request only. They then
	// report nothing on a CII invoice that does not declare NLCIUS, and the
	// SI-UBL 2.0 version check on a UBL one.
	french := []byte(readXML(t, frenchInvoice))
	peppolUBL := []byte(readXML(t, peppolUBLInvoice))
	for _, opts := range []*Options{nil, {Auto: true}} {
		if r := XML(french, opts); findings(r, "BR-NL") != nil {
			t.Errorf("Dutch rules applied to a Factur-X invoice (%+v):\n%s", opts, r)
		}
		if r := XML(peppolUBL, opts); findings(r, "SI-") != nil {
			t.Errorf("Dutch rules applied to a Peppol invoice (%+v):\n%s", opts, r)
		}
	}
	if r := XML(french, &Options{Rules: Default | Netherlands}); len(r.Findings) > 0 {
		t.Errorf("Rules: Default | Netherlands on a Factur-X invoice:\n%s", r)
	}
	r := XML(peppolUBL, &Options{Rules: Default | Netherlands})
	if f := findings(r, "SI-V20-INV-R000"); len(f) != 1 || f[0].Source != SourceNL || len(r.Findings) != 1 {
		t.Errorf("Rules: Default | Netherlands on a Peppol UBL invoice:\n%s", r)
	}
}

func TestRulesForNLCIUS(t *testing.T) {
	nlcius := readXML(t, nlciusInvoice)
	german := readXML(t, germanInvoice)
	for _, c := range []struct {
		name, doc string
		opts      Options
		want      RuleSet
	}{
		{"NLCIUS", nlcius, Options{}, Default | Netherlands},
		{"NLCIUS with Auto", nlcius, Options{Auto: true}, Default | Netherlands},
		{"NLCIUS explicit rules", nlcius, Options{Rules: Schema | France}, Schema | France | Netherlands},
		{"NLCIUS profile option", german, Options{Profile: invoice.ProfileNLCIUS}, Default | Netherlands},
		{"Dutch seller without NLCIUS", strings.Replace(german, "<ram:CountryID>DE</ram:CountryID>", "<ram:CountryID>NL</ram:CountryID>", 1), Options{Auto: true}, Default},
	} {
		root, err := xmltree.Parse([]byte(c.doc))
		if err != nil {
			t.Fatal(err)
		}
		if got := RulesFor(rules.NewDoc(root, c.opts.Profile), &c.opts); got != c.want {
			t.Errorf("%s: RulesFor = %b, want %b", c.name, got, c.want)
		}
	}
}

func TestCatalogNL(t *testing.T) {
	ids := map[string]int{}
	for _, r := range Catalog(Netherlands) {
		ids[r.ID]++
	}
	// BR-NL-32-and-34 is a CII rule, BR-NL-8 and SI-V20-INV-R000 UBL rules;
	// the rules of both syntaxes are listed once.
	for _, id := range []string{"BR-NL-1", "BR-NL-32-and-34", "empty-element-check", "BR-NL-8", "SI-V20-INV-R000", "SI-UBL-2"} {
		if ids[id] != 1 {
			t.Errorf("Catalog(Netherlands) lists %s %d times", id, ids[id])
		}
	}
	if len(ids) != 39 {
		t.Errorf("Catalog(Netherlands): %d rules, want 39", len(ids))
	}
	for _, r := range Catalog(Default | Peppol) {
		if strings.HasPrefix(r.ID, "BR-NL-") {
			t.Errorf("%s outside Catalog(Netherlands)", r.ID)
		}
	}
}

// TestNLCIUSRoundTrip reads the NLCIUS invoices and writes them back in
// both syntaxes: the results are still valid NLCIUS invoices.
func TestNLCIUSRoundTrip(t *testing.T) {
	for _, file := range []string{nlciusInvoice, nlciusUBLInvoice} {
		data := []byte(readXML(t, file))
		parse := cii.Parse
		if strings.HasSuffix(file, ".ubl.xml") {
			parse = ublsyntax.Parse
		}
		inv, err := parse(data)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if inv.Profile != invoice.ProfileNLCIUS {
			t.Fatalf("%s: profile %s", file, inv.Profile)
		}
		ciiData, err := cii.Marshal(inv, &cii.MarshalOptions{Strict: true})
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		ublData, err := ublsyntax.Marshal(inv, nil)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, out := range [][]byte{ciiData, ublData} {
			if r := XML(out, nil); r.Profile != invoice.ProfileNLCIUS || len(r.Findings) > 0 {
				t.Errorf("%s: profile %s\n%s\n%s", file, r.Profile, r, out)
			}
		}
	}
}

// TestNLTestFiles validates the test files of SimplerInvoicing that break no
// rule (FX_NL_TESTS, see conformance/fetch-nl.sh; their name has _ok): they
// are NLCIUS invoices without error, whatever the rule set.
func TestNLTestFiles(t *testing.T) {
	dir := os.Getenv("FX_NL_TESTS")
	if dir == "" {
		t.Skip("FX_NL_TESTS is not set")
	}
	n := 0
	for _, sub := range []string{"SI-UBL-2.0.3.2", "NLCIUS-CII-1.0.3"} {
		files, err := filepath.Glob(filepath.Join(dir, sub, "*_ok*.xml"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			n++
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if r := XML(b, &Options{Auto: true}); r.Profile != invoice.ProfileNLCIUS || !r.Valid() {
				t.Errorf("%s: profile %s\n%s", filepath.Base(file), r.Profile, r)
			}
		}
	}
	if n < 30 {
		t.Errorf("%d files, want at least 30", n)
	}
	t.Logf("%d files", n)
}
