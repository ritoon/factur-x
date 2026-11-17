package validate

import (
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/invoice"
)

const zf1Invoice = "../testdata/ZUGFeRD_1p0_COMFORT_SEPA_Prenotification.xml" // ZUGFeRD 1.0 COMFORT, seller in Germany

// ruleIDs returns the rule identifiers of the findings of r, in order.
func ruleIDs(r *Report) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.Rule)
	}
	return out
}

// TestZUGFeRD1 checks a valid ZUGFeRD 1.0 invoice: whatever the rule sets,
// only the warning ZF1-04 is reported, and the profile is the one matched to
// the ZUGFeRD 1.0 identifier.
func TestZUGFeRD1(t *testing.T) {
	doc := []byte(readXML(t, zf1Invoice))
	for _, opts := range []*Options{
		nil,
		{Rules: Schema},
		{Rules: Default | France | Germany | GermanyProfile | ChorusPro | Peppol},
		{Rules: EN16931 | FacturX},
		{Auto: true, RejectBasicWL: true},
	} {
		r := XML(doc, opts)
		if got := ruleIDs(r); !slices.Equal(got, []string{ruleZF1Scope}) {
			t.Errorf("%+v: findings = %v", opts, r.Findings)
			continue
		}
		f := r.Findings[0]
		if f.Severity != Warning || f.Source != SourceSyntax || f.Path != "/rsm:CrossIndustryDocument" || f.Line == 0 ||
			!strings.Contains(f.Message, "EN 16931") {
			t.Errorf("%+v: ZF1-04 = %+v", opts, f)
		}
		if !r.Valid() || r.Profile != invoice.ProfileEN16931 {
			t.Errorf("%+v: valid = %v, profile = %v", opts, r.Valid(), r.Profile)
		}
	}
	if r := XML(doc, &Options{Profile: invoice.ProfileExtended}); r.Profile != invoice.ProfileExtended {
		t.Errorf("Options.Profile: profile = %v", r.Profile)
	}
}

// TestZUGFeRD1Findings checks the rules ZF1-01 to ZF1-03 on altered copies
// of a valid ZUGFeRD 1.0 invoice.
func TestZUGFeRD1Findings(t *testing.T) {
	const (
		root      = "/rsm:CrossIndustryDocument"
		context   = root + "/rsm:SpecifiedExchangedDocumentContext"
		header    = root + "/rsm:HeaderExchangedDocument"
		trade     = root + "/rsm:SpecifiedSupplyChainTradeTransaction"
		buyer     = trade + "/ram:ApplicableSupplyChainTradeAgreement/ram:BuyerTradeParty"
		totals    = trade + "/ram:ApplicableSupplyChainTradeSettlement/ram:SpecifiedTradeSettlementMonetarySummation"
		zf1ID     = "<ram:ID>urn:ferd:CrossIndustryDocument:invoice:1p0:comfort</ram:ID>"
		guideline = "<ram:GuidelineSpecifiedDocumentContextParameter>\n\t\t\t" + zf1ID + "\n\t\t</ram:GuidelineSpecifiedDocumentContextParameter>"
	)
	valid := strings.ReplaceAll(readXML(t, zf1Invoice), "\r\n", "\n") // the sample has CRLF line ends
	cases := []struct {
		name     string
		old, new string
		profile  invoice.Profile
		want     []Finding // rule and path of the findings before ZF1-04
	}{
		{"unknown identifier", zf1ID, "<ram:ID>urn:ferd:CrossIndustryDocument:invoice:1p0:premium</ram:ID>", invoice.ProfileUnknown,
			[]Finding{{Rule: ruleZF1Profile, Path: context + "/ram:GuidelineSpecifiedDocumentContextParameter/ram:ID"}}},
		{"Factur-X identifier", zf1ID, "<ram:ID>" + invoice.URNEN16931 + "</ram:ID>", invoice.ProfileUnknown,
			[]Finding{{Rule: ruleZF1Profile, Path: context + "/ram:GuidelineSpecifiedDocumentContextParameter/ram:ID"}}},
		{"missing identifier", guideline, "", invoice.ProfileUnknown,
			[]Finding{{Rule: ruleZF1Profile, Path: context}}},
		{"empty identifier", zf1ID, "<ram:ID></ram:ID>", invoice.ProfileUnknown,
			[]Finding{{Rule: ruleZF1Profile, Path: context + "/ram:GuidelineSpecifiedDocumentContextParameter/ram:ID"}}},
		{"release candidate identifier", zf1ID, "<ram:ID>urn:ferd:invoice:rc:extended</ram:ID>", invoice.ProfileExtended, nil},
		{"unreadable amount", ">529.87</ram:GrandTotalAmount>", ">529,87</ram:GrandTotalAmount>", invoice.ProfileEN16931,
			[]Finding{{Rule: ruleZF1Value, Path: totals + "/ram:GrandTotalAmount"}}},
		{"unreadable date", `<udt:DateTimeString format="102">20140305</udt:DateTimeString>`, `<udt:DateTimeString format="102">20140230</udt:DateTimeString>`, invoice.ProfileEN16931,
			[]Finding{{Rule: ruleZF1Value, Path: header + "/ram:IssueDateTime/udt:DateTimeString"}}},
		{"missing buyer name", "<ram:Name>Kunden AG Mitte</ram:Name>", "", invoice.ProfileEN16931,
			[]Finding{{Rule: ruleZF1Mandatory, Path: buyer}}},
		{"empty invoice number", "<ram:ID>471102</ram:ID>", "<ram:ID> </ram:ID>", invoice.ProfileEN16931,
			[]Finding{{Rule: ruleZF1Mandatory, Path: header + "/ram:ID"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(valid, c.old) {
				t.Fatalf("%q not in the sample", c.old)
			}
			r := XML([]byte(strings.Replace(valid, c.old, c.new, 1)), nil)
			if r.Profile != c.profile {
				t.Errorf("profile = %v, want %v", r.Profile, c.profile)
			}
			got := r.Findings
			if n := len(got); n == 0 || got[n-1].Rule != ruleZF1Scope {
				t.Fatalf("findings = %v, want ZF1-04 last", got)
			}
			got = got[:len(got)-1]
			if len(got) != len(c.want) {
				t.Fatalf("findings = %v, want %v", got, c.want)
			}
			for i, f := range got {
				if f.Rule != c.want[i].Rule || f.Path != c.want[i].Path || f.Severity != Error || f.Source != SourceSyntax ||
					f.Line == 0 || f.Message == "" {
					t.Errorf("finding %d = %+v, want %s at %s", i, f, c.want[i].Rule, c.want[i].Path)
				}
			}
			if r.Valid() != (len(c.want) == 0) {
				t.Errorf("Valid() = %v", r.Valid())
			}
			// Without the Schema rule set, only ZF1-04 is reported.
			if r := XML([]byte(strings.Replace(valid, c.old, c.new, 1)), &Options{Rules: EN16931 | France}); len(r.Findings) != 1 {
				t.Errorf("without Schema: findings = %v", r.Findings)
			}
		})
	}
}

func TestCatalogZUGFeRD1(t *testing.T) {
	ids := func(c []RuleInfo) []string {
		var out []string
		for _, r := range c {
			if strings.HasPrefix(r.ID, "ZF1-") {
				out = append(out, r.ID)
			}
		}
		return out
	}
	if got := ids(Catalog(Schema)); !slices.Equal(got, []string{ruleZF1Profile, ruleZF1Value, ruleZF1Mandatory, ruleZF1Scope}) {
		t.Errorf("Catalog(Schema) ZF1 rules = %v", got)
	}
	if got := ids(Catalog(EN16931 | FacturX | France | Germany | Peppol)); got != nil {
		t.Errorf("Catalog without Schema lists %v", got)
	}
}
