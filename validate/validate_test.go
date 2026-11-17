package validate

import (
	"os"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/rules/ubl"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

const (
	frenchInvoice = "../internal/rules/fr/testdata/en16931.xml" // seller in France, EN 16931
	germanInvoice = "../testdata/EN16931_Einfach.cii.xml"       // seller in Germany, EN 16931
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
		{"../testdata/EN16931_Einfach.ubl.xml", invoice.ProfileEN16931},
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
		if findings(r, "FX-") != nil || findings(r, "XSD-PROFILE") != nil {
			t.Errorf("%s: CII rules applied to UBL:\n%s", c.file, r)
		}
	}

	// The findings are located in the UBL document.
	doc := strings.Replace(readXML(t, "../testdata/EN16931_Einfach.ubl.xml"), ">529.87</cbc:PayableAmount>", ">1.234</cbc:PayableAmount>", 1)
	r := XML([]byte(doc), nil)
	if got, want := strings.Join(r.Rules(), " "), "BR-CO-16 BR-DEC-18 UBL-DT-01 XSD-UBL"; got != want {
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
	if !has(EN16931, "UBL-CR-001") || !has(EN16931, "UBL-SR-01") || !has(Schema, RuleUBLSchema) || has(Schema, "UBL-CR-001") {
		t.Errorf("UBL rules missing from the catalog")
	}
}
