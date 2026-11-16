package choruspro

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

const base = "testdata/b2g.xml" // B2G invoice without finding, whatever the settings

func run(t *testing.T, data string, s Settings) *rules.Report {
	t.Helper()
	root, err := xmltree.Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	r := &rules.Report{}
	Check(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileUnknown), Report: r, Source: rules.SourceChorusPro}, s)
	return r
}

const (
	buyerSIRET  = `<ram:GlobalID schemeID="0009">99999999800019</ram:GlobalID>`
	buyerSIREN  = `<ram:ID schemeID="0002">999999998</ram:ID>`
	serviceRef  = `<ram:BuyerReference>SERVICE-FINANCES</ram:BuyerReference>`
	orderRef    = `<ram:IssuerAssignedID>EJ-2026-0042</ram:IssuerAssignedID>`
	contractRef = `<ram:IssuerAssignedID>CONTRAT-2026-07</ram:IssuerAssignedID>`
	typeCode    = `<ram:TypeCode>380</ram:TypeCode>`
	number      = `<ram:ID>FA-2026-00042</ram:ID>`
	precedingNr = `<ram:IssuerAssignedID>FA-2026-00017</ram:IssuerAssignedID>`
)

var all = Settings{ServiceCodeRequired: true, EngagementRequired: true}

type edit struct{ old, new string }

type testCase struct {
	name     string
	edits    []edit
	settings Settings
	want     []string
}

// cases returns edits of the base invoice and the rules they make fail.
func cases() []testCase {
	long := func(n int) string { return strings.Repeat("A", n) }
	return []testCase{
		{"valid", nil, all, nil},
		{"valid without settings", nil, Settings{}, nil},

		// CPRO-01, CPRO-02: buyer SIRET.
		{"no buyer SIRET", []edit{{buyerSIRET, ""}}, all, []string{RuleBuyerSIRET}},
		{"SIRET without scheme", []edit{{buyerSIRET, `<ram:GlobalID>99999999800019</ram:GlobalID>`}}, all, []string{RuleBuyerSIRET}},
		{"SIRET with another scheme", []edit{{buyerSIRET, `<ram:GlobalID schemeID="0088">99999999800019</ram:GlobalID>`}}, all, []string{RuleBuyerSIRET}},
		{"bad SIRET key", []edit{{buyerSIRET, `<ram:GlobalID schemeID="0009">99999999800018</ram:GlobalID>`}}, all, []string{RuleSIRETKey}},
		{"SIRET too short", []edit{{buyerSIRET, `<ram:GlobalID schemeID="0009">9999999980001</ram:GlobalID>`}}, all, []string{RuleSIRETKey}},
		{"SIRET with letters", []edit{{buyerSIRET, `<ram:GlobalID schemeID="0009">9999999980001A</ram:GlobalID>`}}, all, []string{RuleSIRETKey}},
		{"SIRET with spaces around", []edit{{buyerSIRET, `<ram:GlobalID schemeID="0009"> 99999999800019 </ram:GlobalID>`}}, all, nil},
		{"historical mapping: BT-47 scheme 0002", []edit{{buyerSIRET, ""}, {buyerSIREN, `<ram:ID schemeID="0002">99999999800019</ram:ID>`}}, all, nil},
		{"historical mapping: BT-47 without scheme", []edit{{buyerSIRET, ""}, {buyerSIREN, `<ram:ID>99999999800019</ram:ID>`}}, all, nil},
		{"BT-47 scheme 0009", []edit{{buyerSIRET, ""}, {buyerSIREN, `<ram:ID schemeID="0009">99999999800019</ram:ID>`}}, all, nil},
		{"BT-47 with a bad key", []edit{{buyerSIRET, ""}, {buyerSIREN, `<ram:ID schemeID="0002">99999999800018</ram:ID>`}}, all, []string{RuleSIRETKey}},
		{"BT-47 SIREN only", []edit{{buyerSIRET, ""}}, all, []string{RuleBuyerSIRET}},
		{"BT-47 with another scheme", []edit{{buyerSIRET, ""}, {buyerSIREN, `<ram:ID schemeID="0088">99999999800019</ram:ID>`}}, all, []string{RuleBuyerSIRET}},
		{"both locations, one bad", []edit{{buyerSIREN, `<ram:ID schemeID="0009">99999999800018</ram:ID>`}}, all, []string{RuleSIRETKey}},

		// CPRO-03, CPRO-04: service code.
		{"no service code, required", []edit{{serviceRef, ""}}, all, []string{RuleServiceCode}},
		{"blank service code, required", []edit{{serviceRef, `<ram:BuyerReference> </ram:BuyerReference>`}}, all, []string{RuleServiceCode}},
		{"no service code, not required", []edit{{serviceRef, ""}}, Settings{EngagementRequired: true}, nil},
		{"service code with accents and spaces", []edit{{serviceRef, `<ram:BuyerReference>Régie des eaux_Nord-2</ram:BuyerReference>`}}, all, nil},
		{"service code of 100 characters", []edit{{serviceRef, "<ram:BuyerReference>" + long(100) + "</ram:BuyerReference>"}}, all, nil},
		{"service code of 101 characters", []edit{{serviceRef, "<ram:BuyerReference>" + long(101) + "</ram:BuyerReference>"}}, all, []string{RuleServiceCodeFormat}},
		{"service code with a slash", []edit{{serviceRef, `<ram:BuyerReference>SERVICE/FINANCES</ram:BuyerReference>`}}, Settings{}, []string{RuleServiceCodeFormat}},

		// CPRO-05, CPRO-06: engagement number and contract reference.
		{"no engagement, required", []edit{{orderRef, `<ram:IssuerAssignedID></ram:IssuerAssignedID>`}}, all, nil}, // the contract reference stands in
		{"no engagement nor contract, required", []edit{{orderRef, `<ram:IssuerAssignedID></ram:IssuerAssignedID>`}, {contractRef, `<ram:IssuerAssignedID></ram:IssuerAssignedID>`}}, all, []string{RuleEngagement}},
		{"no engagement nor contract, not required", []edit{{orderRef, `<ram:IssuerAssignedID></ram:IssuerAssignedID>`}, {contractRef, `<ram:IssuerAssignedID></ram:IssuerAssignedID>`}}, Settings{ServiceCodeRequired: true}, nil},
		{"engagement of 50 characters", []edit{{orderRef, "<ram:IssuerAssignedID>" + long(50) + "</ram:IssuerAssignedID>"}}, all, nil},
		{"engagement of 51 characters", []edit{{orderRef, "<ram:IssuerAssignedID>" + long(51) + "</ram:IssuerAssignedID>"}}, Settings{}, []string{RuleReferenceLength}},
		{"contract reference of 51 characters", []edit{{contractRef, "<ram:IssuerAssignedID>" + long(51) + "</ram:IssuerAssignedID>"}}, Settings{}, []string{RuleReferenceLength}},

		// CPRO-07: document type.
		{"type 751", []edit{{typeCode, `<ram:TypeCode>751</ram:TypeCode>`}}, Settings{}, []string{RuleTypeCode}},
		{"credit note", []edit{{typeCode, `<ram:TypeCode>381</ram:TypeCode>`}}, all, nil},

		// CPRO-08: invoice numbers.
		{"invoice number of 20 characters", []edit{{number, "<ram:ID>FA-2026-000000000042</ram:ID>"}}, all, nil},
		{"invoice number of 21 characters", []edit{{number, "<ram:ID>FA-2026-0000000000042</ram:ID>"}}, all, []string{RuleInvoiceNumber}},
		{"invoice number with a space", []edit{{number, "<ram:ID>FA 2026+42/A_1</ram:ID>"}}, all, nil},
		{"invoice number with a dot", []edit{{number, "<ram:ID>FA.2026.42</ram:ID>"}}, all, []string{RuleInvoiceNumber}},
		{"invoice number with an accent", []edit{{number, "<ram:ID>FÉ-2026-42</ram:ID>"}}, all, []string{RuleInvoiceNumber}},
		{"preceding invoice number of 21 characters", []edit{{precedingNr, "<ram:IssuerAssignedID>FA-2026-0000000000017</ram:IssuerAssignedID>"}}, all, []string{RuleInvoiceNumber}},
	}
}

func TestCheck(t *testing.T) {
	b, err := os.ReadFile(base)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, c := range cases() {
		t.Run(c.name, func(t *testing.T) {
			data := doc
			for _, e := range c.edits {
				if !strings.Contains(data, e.old) {
					t.Fatalf("%q not found", e.old)
				}
				data = strings.Replace(data, e.old, e.new, 1)
			}
			r := run(t, data, c.settings)
			want := slices.Clone(c.want)
			slices.Sort(want)
			if got := r.Rules(); !slices.Equal(got, want) {
				t.Errorf("rules = %q, want %q\n%s", got, want, r)
			}
			checkFindings(t, r)
		})
	}
}

// TestCoverage checks that every rule of Catalog fails alone in a test case.
func TestCoverage(t *testing.T) {
	alone := map[string]bool{}
	for _, c := range cases() {
		if len(c.want) == 1 {
			alone[c.want[0]] = true
		}
	}
	for _, r := range Catalog {
		if !alone[r.ID] {
			t.Errorf("no test case fails %s alone", r.ID)
		}
	}
}

func checkFindings(t *testing.T, r *rules.Report) {
	t.Helper()
	for _, f := range r.Findings {
		var info *rules.RuleInfo
		for i := range Catalog {
			if Catalog[i].ID == f.Rule {
				info = &Catalog[i]
			}
		}
		if info == nil || f.Severity != info.Severity || f.Source != rules.SourceChorusPro ||
			!strings.HasPrefix(f.Message, "["+f.Rule+"] ") || f.Path == "" {
			t.Errorf("bad finding %+v", f)
		}
	}
}

func TestEmpty(t *testing.T) {
	Check(nil, all)
	Check(&rules.Checker{}, all)
	r := run(t, `<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"/>`, all)
	if got := r.Rules(); !slices.Equal(got, []string{RuleBuyerSIRET, RuleServiceCode, RuleEngagement}) {
		t.Errorf("empty invoice: %q", got)
	}
	checkFindings(t, r)
}

func TestCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Catalog {
		if seen[r.ID] || !strings.HasPrefix(r.ID, "CPRO-") || r.Description == "" || r.Severity == 0 {
			t.Errorf("bad entry %+v", r)
		}
		seen[r.ID] = true
	}
}
