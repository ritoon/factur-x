package validate

import (
	"os"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/cii"
	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/rules/ubl"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

const (
	frenchInvoice = "../internal/rules/fr/testdata/en16931.xml"    // seller in France, EN 16931
	germanInvoice = "../testdata/EN16931_Einfach.cii.xml"          // seller in Germany, EN 16931
	peppolInvoice = "../internal/rules/peppol/testdata/peppol.xml" // Peppol BIS Billing 3.0, seller in the United Kingdom
	bt24EN16931   = "<ram:ID>urn:cen.eu:en16931:2017</ram:ID>"
)

func readXML(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// withProfile replaces BT-24 of an EN 16931 invoice by the identifier of p.
func withProfile(t *testing.T, doc string, p invoice.Profile) []byte {
	t.Helper()
	if !strings.Contains(doc, bt24EN16931) {
		t.Fatal("BT-24 not found")
	}
	return []byte(strings.Replace(doc, bt24EN16931, "<ram:ID>"+p.URN()+"</ram:ID>", 1))
}

func TestRulesForAuto(t *testing.T) {
	german := readXML(t, germanInvoice)
	for _, c := range []struct {
		name, doc string
		opts      Options
		want      RuleSet
	}{
		{"French seller", readXML(t, frenchInvoice), Options{Auto: true}, Default | France},
		{"German seller", german, Options{Auto: true}, Default | GermanyProfile},
		{"Belgian seller", strings.Replace(german, "<ram:CountryID>DE</ram:CountryID>", "<ram:CountryID>BE</ram:CountryID>", 1), Options{Auto: true}, Default},
		{"German seller without Auto", german, Options{}, Default},
		{"explicit rules", german, Options{Auto: true, Rules: Schema}, Schema | GermanyProfile},
		{"XRECHNUNG", readXML(t, "../testdata/XRECHNUNG_Einfach.cii.xml"), Options{Auto: true}, Default | Germany | GermanyProfile},
		{"Peppol", readXML(t, peppolInvoice), Options{}, Default | Peppol},
		{"Peppol with Auto", readXML(t, peppolInvoice), Options{Auto: true}, Default | Peppol},
		{"Peppol explicit rules", readXML(t, peppolInvoice), Options{Rules: Schema}, Schema},
		{"Peppol profile option", german, Options{Profile: invoice.ProfilePeppol}, Default | Peppol},
		{"French seller with Peppol", readXML(t, frenchInvoice), Options{Auto: true, Rules: Default | Peppol}, Default | Peppol | France},
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

// findings returns the findings of r whose rule starts with prefix.
func findings(r *Report, prefix string) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Rule, prefix) {
			out = append(out, f)
		}
	}
	return out
}

func TestFrenchProfiles(t *testing.T) {
	doc := readXML(t, frenchInvoice)
	for _, c := range []struct {
		profile invoice.Profile
		opts    Options
		rule    string
		sev     Severity
	}{
		{invoice.ProfileMinimum, Options{Rules: Default | France}, "FR-PROFILE-01", Error},
		{invoice.ProfileMinimum, Options{Auto: true}, "FR-PROFILE-01", Error},
		{invoice.ProfileMinimum, Options{}, "", 0}, // French rules not selected
		{invoice.ProfileBasic, Options{Auto: true}, "FR-PROFILE-02", Warning},
		{invoice.ProfileBasicWL, Options{Auto: true}, "", 0},
		{invoice.ProfileBasicWL, Options{Auto: true, RejectBasicWL: true}, "FR-PROFILE-03", Error},
		{invoice.ProfileBasicWL, Options{RejectBasicWL: true}, "", 0}, // French rules not selected
		{invoice.ProfileEN16931, Options{Auto: true, RejectBasicWL: true}, "", 0},
		{invoice.ProfileExtendedCTCFR, Options{Auto: true}, "", 0},
		{invoice.ProfileExtended, Options{Auto: true}, "", 0},
	} {
		r := XML(withProfile(t, doc, c.profile), &c.opts)
		got := findings(r, "FR-PROFILE")
		if c.rule == "" {
			if len(got) != 0 {
				t.Errorf("%s %+v: unexpected %v", c.profile, c.opts, got)
			}
			continue
		}
		if len(got) != 1 || got[0].Rule != c.rule || got[0].Severity != c.sev || got[0].Source != SourceFR {
			t.Errorf("%s %+v: got %v, want %s (%s)", c.profile, c.opts, got, c.rule, c.sev)
		}
		if findings(r, "DE-PROFILE") != nil {
			t.Errorf("%s: German profile check applied to a French seller", c.profile)
		}
	}
	// The profile forced by Options.Profile is the one checked.
	r := XML([]byte(doc), &Options{Rules: Default | France, Profile: invoice.ProfileMinimum})
	if !r.Has("FR-PROFILE-01") {
		t.Errorf("forced MINIMUM not reported:\n%s", r)
	}
}

func TestGermanProfiles(t *testing.T) {
	doc := readXML(t, germanInvoice)
	for _, c := range []struct {
		profile invoice.Profile
		opts    Options
		want    bool
	}{
		{invoice.ProfileMinimum, Options{Auto: true}, true},
		{invoice.ProfileBasicWL, Options{Auto: true}, true},
		{invoice.ProfileBasicWL, Options{Rules: Default | GermanyProfile}, true},
		{invoice.ProfileBasicWL, Options{}, false},
		{invoice.ProfileBasicWL, Options{Rules: Default | Germany}, false},
		{invoice.ProfileBasic, Options{Auto: true}, false},
		{invoice.ProfileEN16931, Options{Auto: true}, false},
		{invoice.ProfileExtended, Options{Auto: true}, false},
	} {
		r := XML(withProfile(t, doc, c.profile), &c.opts)
		got := findings(r, "DE-PROFILE")
		if !c.want {
			if len(got) != 0 {
				t.Errorf("%s %+v: unexpected %v", c.profile, c.opts, got)
			}
			continue
		}
		if len(got) != 1 || got[0].Rule != "DE-PROFILE-01" || got[0].Severity != Error || got[0].Source != SourceDE {
			t.Errorf("%s %+v: got %v", c.profile, c.opts, got)
		}
		if c.opts.Auto && findings(r, "BR-DE") != nil {
			t.Errorf("%s: Auto applied the XRechnung rules to a ZUGFeRD invoice:\n%s", c.profile, r)
		}
		if findings(r, "FR-PROFILE") != nil || findings(r, "BR-FR") != nil {
			t.Errorf("%s: French rules applied to a German seller", c.profile)
		}
	}
}

func TestCatalogProfiles(t *testing.T) {
	has := func(set RuleSet, id string) bool {
		for _, r := range Catalog(set) {
			if r.ID == id {
				return true
			}
		}
		return false
	}
	for _, c := range []struct {
		set  RuleSet
		id   string
		want bool
	}{
		{France, "FR-PROFILE-01", true},
		{France, "FR-PROFILE-02", true},
		{France, "FR-PROFILE-03", true},
		{Default, "FR-PROFILE-01", false},
		{GermanyProfile, "DE-PROFILE-01", true},
		{Germany, "DE-PROFILE-01", false},
		{Germany, "BR-DE-15", true},
		{GermanyProfile, "BR-DE-15", false},
		{Peppol, "PEPPOL-EN16931-R001", true},
		{Peppol, "NO-R-001", true},
		{Default, "PEPPOL-EN16931-R001", false},
	} {
		if got := has(c.set, c.id); got != c.want {
			t.Errorf("Catalog(%b) has %s = %v, want %v", c.set, c.id, got, c.want)
		}
	}
}

func TestUBL(t *testing.T) {
	for _, c := range []struct {
		file    string
		profile invoice.Profile
	}{
		{"../testdata/EN16931_Einfach.ubl.xml", invoice.ProfilePeppol},
		{"../testdata/XRECHNUNG_Einfach.ubl.xml", invoice.ProfileXRechnung},
		{"../testdata/ubl-tc434-creditnote1.xml", invoice.ProfileEN16931},
	} {
		r := XML([]byte(readXML(t, c.file)), &Options{Auto: true})
		if !r.Valid() || r.Profile != c.profile {
			t.Errorf("%s: profile %s, valid %v:\n%s", c.file, r.Profile, r.Valid(), r)
		}
		if !r.Has(RuleUBLSchema) {
			t.Errorf("%s: no %s", c.file, RuleUBLSchema)
		}
		if findings(r, "FX-") != nil || findings(r, "XSD-PROFILE") != nil || findings(r, "PEPPOL-EN16931") != nil {
			t.Errorf("%s: CII rules applied to UBL:\n%s", c.file, r)
		}
		if peppol := c.profile == invoice.ProfilePeppol; r.Has(RuleUBLPeppol) != peppol {
			t.Errorf("%s: %s reported = %v, want %v", c.file, RuleUBLPeppol, !peppol, peppol)
		}
	}

	// The findings are located in the UBL document.
	doc := strings.Replace(readXML(t, "../testdata/EN16931_Einfach.ubl.xml"), ">529.87</cbc:PayableAmount>", ">1.234</cbc:PayableAmount>", 1)
	r := XML([]byte(doc), nil)
	if got, want := strings.Join(r.Rules(), " "), "BR-CO-16 BR-DEC-18 PEPPOL-UBL UBL-DT-01 XSD-UBL"; got != want {
		t.Errorf("rules = %s, want %s\n%s", got, want, r)
	}
	for _, f := range r.Errors() {
		if f.Path != "/Invoice/cac:LegalMonetaryTotal/cbc:PayableAmount" || f.Line != 109 {
			t.Errorf("finding not located in the UBL document: %s", f)
		}
	}

	// Rule sets: the schema only reports that it is not checked; the German
	// rules apply to an XRechnung UBL invoice.
	r = XML([]byte(doc), &Options{Rules: Schema})
	if got := strings.Join(r.Rules(), " "); got != RuleUBLSchema || !r.Valid() {
		t.Errorf("Schema only: %s", r)
	}
	r = XML([]byte(doc), &Options{Rules: Peppol})
	if got := strings.Join(r.Rules(), " "); got != RuleUBLPeppol || !r.Valid() {
		t.Errorf("Peppol only: %s", r)
	}
	xr := readXML(t, "../testdata/XRECHNUNG_Einfach.ubl.xml")
	root, err := xmltree.Parse([]byte(xr))
	if err != nil {
		t.Fatal(err)
	}
	if got := RulesFor(ubl.View(root, invoice.ProfileUnknown), &Options{Auto: true}); got != Default|Germany|GermanyProfile {
		t.Errorf("RulesFor(XRechnung UBL) = %b", got)
	}
	r = XML([]byte(strings.Replace(xr, "<cbc:BuyerReference>04011000-12345-34</cbc:BuyerReference>", "", 1)), nil)
	if de := findings(r, "BR-DE-15"); len(de) != 1 || de[0].Source != SourceDE {
		t.Errorf("XRechnung UBL without buyer reference:\n%s", r)
	}

	// Neither CII nor UBL.
	r = XML([]byte(`<Invoice xmlns="urn:example:invoice"/>`), nil)
	if r.Valid() || !r.Has("CII") {
		t.Errorf("unknown root: %s", r)
	}
}

func TestCatalogUBL(t *testing.T) {
	has := func(set RuleSet, id string) bool {
		for _, r := range Catalog(set) {
			if r.ID == id {
				return true
			}
		}
		return false
	}
	if !has(EN16931, "UBL-CR-001") || !has(EN16931, "UBL-SR-01") || !has(Schema, RuleUBLSchema) || has(Schema, "UBL-CR-001") || !has(Peppol, RuleUBLPeppol) {
		t.Errorf("UBL rules missing from the catalog")
	}
}

// TestPeppol checks when the Peppol rules apply: on a Peppol invoice
// (BT-24), or on request, never by default nor with Auto.
func TestPeppol(t *testing.T) {
	r := XML([]byte(readXML(t, peppolInvoice)), nil)
	if r.Profile != invoice.ProfilePeppol || len(r.Findings) > 0 {
		t.Errorf("Peppol invoice: profile %s\n%s", r.Profile, r)
	}
	// A Peppol invoice without electronic addresses: the Peppol rules
	// apply by default.
	doc := strings.ReplaceAll(readXML(t, peppolInvoice), "URIUniversalCommunication>", "X>")
	r = XML([]byte(doc), nil)
	if len(findings(r, "PEPPOL-EN16931-R010")) != 1 || len(findings(r, "PEPPOL-EN16931-R020")) != 1 {
		t.Errorf("Peppol invoice without electronic address:\n%s", r)
	}
	for _, f := range findings(r, "PEPPOL") {
		if f.Source != SourcePeppol {
			t.Errorf("source of %s: %q", f.Rule, f.Source)
		}
	}

	french := []byte(readXML(t, frenchInvoice))
	for _, opts := range []*Options{nil, {Auto: true}} {
		if r := XML(french, opts); findings(r, "PEPPOL") != nil {
			t.Errorf("Peppol rules applied to a Factur-X invoice (%+v):\n%s", opts, r)
		}
	}
	r = XML(french, &Options{Rules: Default | Peppol})
	if findings(r, "PEPPOL-EN16931-R004") == nil {
		t.Errorf("Rules: Default | Peppol did not apply the Peppol rules:\n%s", r)
	}
}

// TestPeppolRoundTrip reads the Peppol invoice and writes it back: the
// result is still a valid Peppol invoice.
func TestPeppolRoundTrip(t *testing.T) {
	inv, err := cii.Parse([]byte(readXML(t, peppolInvoice)))
	if err != nil {
		t.Fatal(err)
	}
	if inv.Profile != invoice.ProfilePeppol {
		t.Fatalf("profile %s", inv.Profile)
	}
	data, err := cii.Marshal(inv, &cii.MarshalOptions{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if r := XML(data, nil); r.Profile != invoice.ProfilePeppol || len(r.Findings) > 0 {
		t.Errorf("profile %s\n%s\n%s", r.Profile, r, data)
	}
}
