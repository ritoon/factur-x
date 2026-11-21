package nl

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/rules"
	ublrules "github.com/ritoon/zerechnung/internal/rules/ubl"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

const base = "testdata/nlcius.xml" // NLCIUS CII invoice without finding

func load(t testing.TB, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// newDoc returns the document Check runs on for a parsed root: the CII
// document, the UBL document with its CII view, or nil for another root.
func newDoc(root *xmltree.Node) *rules.Doc {
	switch {
	case root.Name == "CrossIndustryInvoice":
		return rules.NewDoc(root, invoice.ProfileUnknown)
	case ublrules.IsUBL(root):
		return ublrules.View(root, invoice.ProfileUnknown)
	}
	return nil
}

// catalogOf returns the catalog of the syntax of doc.
func catalogOf(doc *rules.Doc) []rules.RuleInfo {
	if doc.UBL != nil {
		return UBLCatalog
	}
	return Catalog
}

func checkDoc(doc *rules.Doc) *rules.Report {
	r := &rules.Report{}
	Check(&rules.Checker{Doc: doc, Report: r, Source: rules.SourceNL})
	return r
}

// run parses a CII or UBL document and runs Check on it.
func run(t testing.TB, data []byte) *rules.Report {
	t.Helper()
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc := newDoc(root)
	if doc == nil {
		doc = rules.NewDoc(root, invoice.ProfileUnknown)
	}
	return checkDoc(doc)
}

// edit replaces occurrence nth (from 1) of old by new, or every occurrence
// when nth is 0. The edit fails the test when old is not found.
type edit struct {
	old, new string
	nth      int
}

func rep(old, new string) edit         { return edit{old, new, 1} }
func repN(n int, old, new string) edit { return edit{old, new, n} }
func repAll(old, new string) edit      { return edit{old, new, 0} }
func remove(old string) edit           { return edit{old, "", 1} }
func before(anchor, xml string) edit   { return edit{anchor, xml + anchor, 1} }
func after(anchor, xml string) edit    { return edit{anchor, anchor + xml, 1} }
func edits(e ...[]edit) []edit         { return slices.Concat(e...) }
func one(e ...edit) []edit             { return e }
func ids(id ...string) []string        { return id }
func elem(name, value string) string   { return "<ram:" + name + ">" + value + "</ram:" + name + ">" }

func apply(t testing.TB, doc string, es []edit) string {
	t.Helper()
	for _, e := range es {
		if e.nth == 0 {
			if !strings.Contains(doc, e.old) {
				t.Fatalf("edit: %q not found", e.old)
			}
			doc = strings.ReplaceAll(doc, e.old, e.new)
			continue
		}
		at, off := -1, 0
		for range e.nth {
			i := strings.Index(doc[off:], e.old)
			if i < 0 {
				t.Fatalf("edit: occurrence %d of %q not found", e.nth, e.old)
			}
			at, off = off+i, off+i+len(e.old)
		}
		doc = doc[:at] + e.new + doc[at+len(e.old):]
	}
	return doc
}

// Anchors of the CII base invoice.
const (
	aSpec         = "<ram:ID>" + CIUSID + "</ram:ID>"
	aContext      = "<rsm:ExchangedDocumentContext>"
	aTypeCode     = "<ram:TypeCode>380</ram:TypeCode>"
	aNote         = "<ram:Content>Onderhoud en apparatuur, oktober 2026.</ram:Content>"
	aLineAgree    = "<ram:SpecifiedLineTradeAgreement>"
	aLineReason   = "<ram:Reason>Korting</ram:Reason>"
	aBuyerRef     = "<ram:BuyerReference>PO-4512</ram:BuyerReference>"
	aSellerName   = "<ram:Name>Voorbeeld Netwerken B.V.</ram:Name>"
	aSellerLegal  = `<ram:ID schemeID="0106">12345678</ram:ID>`
	aSellerOrg    = "<ram:SpecifiedLegalOrganization>\n          " + aSellerLegal + "\n        </ram:SpecifiedLegalOrganization>"
	aSellerPost   = "<ram:PostcodeCode>1012 AB</ram:PostcodeCode>"
	aSellerCity   = "<ram:CityName>Amsterdam</ram:CityName>"
	aCountry      = "<ram:CountryID>NL</ram:CountryID>" // 1: seller, 2: buyer
	aSellerTaxEnd = "</ram:SpecifiedTaxRegistration>"
	aBuyerLegal   = `<ram:ID schemeID="0106">87654321</ram:ID>`
	aBuyerOrg     = "<ram:SpecifiedLegalOrganization>\n          " + aBuyerLegal + "\n        </ram:SpecifiedLegalOrganization>"
	aBuyerPost    = "<ram:PostcodeCode>3511 AB</ram:PostcodeCode>"
	aBuyerCity    = "<ram:CityName>Utrecht</ram:CityName>"
	aBuyerEnd     = "</ram:BuyerTradeParty>"
	aAgreementEnd = "</ram:ApplicableHeaderTradeAgreement>"
	aDelivery     = "<ram:ApplicableHeaderTradeDelivery>"
	aCurrency     = "<ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>"
	aPMCode       = "<ram:TypeCode>30</ram:TypeCode>"
	aIBAN         = "<ram:IBANID>NL91ABNA0417164300</ram:IBANID>"
	aPayeeEnd     = "</ram:PayeePartyCreditorFinancialAccount>"
	aPM           = "<ram:SpecifiedTradeSettlementPaymentMeans>\n        " + aPMCode + "\n        <ram:PayeePartyCreditorFinancialAccount>\n          " + aIBAN + "\n        " + aPayeeEnd + "\n      </ram:SpecifiedTradeSettlementPaymentMeans>"
	aRate         = "<ram:RateApplicablePercent>21</ram:RateApplicablePercent>" // 3: header VAT breakdown
	aChargeReason = "<ram:Reason>Vracht</ram:Reason>"
	aTaxTotal     = `<ram:TaxTotalAmount currencyID="EUR">210.00</ram:TaxTotalAmount>`
	aDue          = "<ram:DuePayableAmount>1210.00</ram:DuePayableAmount>"
	aSummationEnd = "</ram:SpecifiedTradeSettlementHeaderMonetarySummation>"
)

// Elements added by the cases.
const (
	headerOrder = "<ram:BuyerOrderReferencedDocument><ram:IssuerAssignedID>PO-1</ram:IssuerAssignedID></ram:BuyerOrderReferencedDocument>"
	lineOrder   = "<ram:BuyerOrderReferencedDocument><ram:LineID>1</ram:LineID></ram:BuyerOrderReferencedDocument>"
	precedent   = `<ram:InvoiceReferencedDocument><ram:IssuerAssignedID>F-2026-0041</ram:IssuerAssignedID><ram:FormattedIssueDateTime><qdt:DateTimeString format="102">20261001</qdt:DateTimeString></ram:FormattedIssueDateTime></ram:InvoiceReferencedDocument>`
	taxPoint    = `<ram:TaxPointDate><udt:DateString format="102">20261031</udt:DateString></ram:TaxPointDate>`
	process     = "<ram:BusinessProcessSpecifiedDocumentContextParameter><ram:ID>urn:fdc:peppol.eu:2017:poacc:billing:01:1.0</ram:ID></ram:BusinessProcessSpecifiedDocumentContextParameter>"
	bic         = "<ram:PayeeSpecifiedCreditorFinancialInstitution><ram:BICID>ABNANL2A</ram:BICID></ram:PayeeSpecifiedCreditorFinancialInstitution>"
)

// party returns a party of role ("SellerTaxRepresentativeTradeParty",
// "ShipToTradeParty") with an address in country; extra is added to the
// address, after the street.
func party(role, country, extra string) string {
	return "<ram:" + role + "><ram:Name>Fiscaal B.V.</ram:Name><ram:PostalTradeAddress><ram:PostcodeCode>3011 AA</ram:PostcodeCode><ram:LineOne>Kade 3</ram:LineOne>" +
		extra + "<ram:CityName>Rotterdam</ram:CityName>" + elem("CountryID", country) + "</ram:PostalTradeAddress></ram:" + role + ">"
}

var (
	foreignSeller = one(repN(1, aCountry, elem("CountryID", "BE")))
	foreignBuyer  = one(repN(2, aCountry, elem("CountryID", "BE")))
	noPayment     = one(remove(aPM))
)

type mutationCase struct {
	name  string
	edits []edit
	want  []string
}

func mutationCases() []mutationCase {
	return []mutationCase{
		{"base", nil, nil},

		// Conditions.
		{"not NLCIUS: no condition holds", one(rep(aSpec, elem("ID", invoice.URNPeppol)), remove(aBuyerRef), rep(aTypeCode, elem("TypeCode", "326"))), nil},
		{"NLCIUS followed by other text is not NLCIUS in CII", one(rep(aSpec, elem("ID", CIUSID+"#conformant#urn:example")), remove(aBuyerRef)), nil},
		{"G-account extension", one(rep(aSpec, elem("ID", GAccountID)), remove(aBuyerRef)), ids("BR-NL-2")},
		{"identifier normalized", one(rep(aSpec, elem("ID", "\n  "+CIUSID+" ")), remove(aBuyerRef)), ids("BR-NL-2")},
		{"seller country normalized and upper-cased", one(repN(1, aCountry, elem("CountryID", " nl ")), remove(aBuyerRef)), ids("BR-NL-2")},
		{"foreign supplier: only $si rules", edits(foreignSeller, one(remove(aBuyerRef), rep(aTypeCode, elem("TypeCode", "326")), remove(aSellerCity))), ids("BR-NL-7")},
		{"foreign supplier: empty elements", edits(foreignSeller, one(rep(aNote, "<ram:Content/>"))), ids("empty-element-check")},

		{"BR-NL-1 scheme", one(rep(aSellerLegal, `<ram:ID schemeID="0088">12345678</ram:ID>`)), ids("BR-NL-1")},
		{"BR-NL-1 no scheme", one(rep(aSellerLegal, `<ram:ID>12345678</ram:ID>`)), ids("BR-NL-1")},
		{"BR-NL-1 OIN", one(rep(aSellerLegal, `<ram:ID schemeID="0190">00000001234567890000</ram:ID>`)), nil},
		{"BR-NL-1 blank identifier", one(rep(aSellerLegal, `<ram:ID schemeID="0106"> </ram:ID>`)), ids("BR-NL-1", "empty-element-check")},
		{"BR-NL-1 only with a legal organization", one(remove(aSellerOrg)), nil},
		{"BR-NL-2 no reference", one(remove(aBuyerRef)), ids("BR-NL-2")},
		{"BR-NL-2 order reference", one(remove(aBuyerRef), before(aAgreementEnd, headerOrder)), nil},
		{"BR-NL-2 empty buyer reference", one(rep(aBuyerRef, "<ram:BuyerReference/>")), ids("empty-element-check")},
		{"BR-NL-3 no city", one(remove(aSellerCity)), ids("BR-NL-3")},
		{"BR-NL-3 no post code", one(remove(aSellerPost)), ids("BR-NL-3")},
		{"BR-NL-4 no city", one(remove(aBuyerCity)), ids("BR-NL-4")},
		{"BR-NL-4 foreign customer", edits(foreignBuyer, one(remove(aBuyerCity))), nil},
		{"BR-NL-5 no street", one(after(aBuyerEnd, strings.Replace(party("SellerTaxRepresentativeTradeParty", "NL", ""), "<ram:LineOne>Kade 3</ram:LineOne>", "", 1))), ids("BR-NL-5")},
		{"BR-NL-5 complete", one(after(aBuyerEnd, party("SellerTaxRepresentativeTradeParty", "NL", ""))), nil},
		{"BR-NL-5 foreign representative", one(after(aBuyerEnd, strings.Replace(party("SellerTaxRepresentativeTradeParty", "DE", ""), "<ram:LineOne>Kade 3</ram:LineOne>", "", 1))), nil},
		{"BR-NL-7 type code", one(rep(aTypeCode, elem("TypeCode", "326"))), ids("BR-NL-7")},
		{"BR-NL-7 compared as a number", one(rep(aTypeCode, elem("TypeCode", " 0389 "))), nil},
		{"BR-NL-7 not a number, which BR-NL-11 compares with 381", one(rep(aTypeCode, elem("TypeCode", "38O"))), ids("BR-NL-7", "BR-NL-11")},
		{"BR-NL-9 never applies in CII", one(rep(aTypeCode, elem("TypeCode", "384"))), nil},
		{"BR-NL-10 scheme", one(rep(aBuyerLegal, `<ram:ID schemeID="0088">87654321</ram:ID>`)), ids("BR-NL-10")},
		{"BR-NL-10 OIN", one(rep(aBuyerLegal, `<ram:ID schemeID="0190">00000001234567890000</ram:ID>`)), nil},
		{"BR-NL-10 no legal organization", one(remove(aBuyerOrg)), ids("BR-NL-10")},
		{"BR-NL-10 foreign customer needs an identifier", edits(foreignBuyer, one(remove(aBuyerOrg))), ids("BR-NL-10")},
		{"BR-NL-10 foreign customer, other scheme", edits(foreignBuyer, one(rep(aBuyerLegal, `<ram:ID schemeID="0208">0123456749</ram:ID>`))), nil},
		{"BR-NL-11 no payment means", noPayment, ids("BR-NL-11")},
		{"BR-NL-11 nothing due", edits(noPayment, one(rep(aDue, elem("DuePayableAmount", "0.00")))), nil},
		{"BR-NL-11 negative amount due", edits(noPayment, one(rep(aDue, elem("DuePayableAmount", "-5")))), nil},
		{"BR-NL-11 amount due not a decimal", one(rep(aDue, elem("DuePayableAmount", "1e3"))), ids("BR-NL-11")},
		{"BR-NL-11 credit note", edits(noPayment, one(rep(aTypeCode, elem("TypeCode", "381")))), nil},
		{"BR-NL-11 two amounts due", one(after(aDue, aDue)), ids("BR-NL-11")},
		{"BR-NL-12 payment means code", one(rep(aPMCode, elem("TypeCode", "31"))), ids("BR-NL-12")},
		{"BR-NL-12 compared as a number", one(rep(aPMCode, elem("TypeCode", "058"))), nil},
		{"BR-NL-31 never applies in CII", one(rep(aPMCode, elem("TypeCode", "58")), after(aPayeeEnd, bic)), nil},
		{"BR-NL-13 no order reference", one(after(aLineAgree, lineOrder)), ids("BR-NL-13")},
		{"BR-NL-13 order reference", one(after(aLineAgree, lineOrder), before(aAgreementEnd, headerOrder)), nil},
		{"BR-NL-13 foreign supplier", edits(foreignSeller, one(after(aLineAgree, lineOrder))), ids("BR-NL-13")},

		// Recommendations.
		{"BR-NL-19 tax currency", one(before(aCurrency, elem("TaxCurrencyCode", "EUR"))), ids("BR-NL-19")},
		{"BR-NL-20 tax point date", one(repN(3, aRate, taxPoint+aRate)), ids("BR-NL-20")},
		{"BR-NL-20 empty: first rule only", one(repN(3, aRate, `<ram:TaxPointDate><udt:DateString format="102"></udt:DateString></ram:TaxPointDate>`+aRate)), ids("BR-NL-20")},
		{"BR-NL-21 tax point date code", one(repN(3, aRate, elem("DueDateTypeCode", "5")+aRate)), ids("BR-NL-21")},
		{"BR-NL-22 note subject", one(after(aNote, elem("SubjectCode", "AAI"))), ids("BR-NL-22")},
		{"BR-NL-23 business process", one(after(aContext, process)), ids("BR-NL-23")},
		{"BR-NL-24 preceding invoice date", one(after(aSummationEnd, precedent)), ids("BR-NL-24")},
		{"BR-NL-25 other tax scheme", one(after(aSellerTaxEnd, `<ram:SpecifiedTaxRegistration><ram:ID schemeID="FC">12345</ram:ID></ram:SpecifiedTaxRegistration>`)), ids("BR-NL-25")},
		{"BR-NL-26 legal information", one(after(aSellerName, elem("Description", "B.V."))), ids("BR-NL-26")},
		{"BR-NL-27-1 seller line 3", one(after(aSellerPost+"\n          <ram:LineOne>Voorbeeldstraat 1</ram:LineOne>", elem("LineThree", "Gebouw B"))), ids("BR-NL-27-1")},
		{"BR-NL-27-2 customer line 3", one(after(aBuyerPost+"\n          <ram:LineOne>Stationsplein 2</ram:LineOne>", elem("LineThree", "Gebouw C"))), ids("BR-NL-27-2")},
		{"BR-NL-27-3 representative line 3", one(after(aBuyerEnd, party("SellerTaxRepresentativeTradeParty", "NL", elem("LineThree", "Kantoor 2")))), ids("BR-NL-27-3")},
		{"BR-NL-27-4 delivery line 3", one(after(aDelivery, party("ShipToTradeParty", "NL", elem("LineThree", "Dock 4")))), ids("BR-NL-27-4")},
		{"BR-NL-28-1 seller subdivision", one(repN(1, aCountry, aCountry+elem("CountrySubDivisionName", "Noord-Holland"))), ids("BR-NL-28-1")},
		{"BR-NL-28-2 customer subdivision", one(repN(2, aCountry, aCountry+elem("CountrySubDivisionName", "Utrecht"))), ids("BR-NL-28-2")},
		{"BR-NL-28-3 representative subdivision", one(after(aBuyerEnd, strings.Replace(party("SellerTaxRepresentativeTradeParty", "NL", ""), "</ram:PostalTradeAddress>", elem("CountrySubDivisionName", "Zuid-Holland")+"</ram:PostalTradeAddress>", 1))), ids("BR-NL-28-3")},
		{"BR-NL-28-4 delivery subdivision", one(after(aDelivery, strings.Replace(party("ShipToTradeParty", "NL", ""), "</ram:PostalTradeAddress>", elem("CountrySubDivisionName", "Zuid-Holland")+"</ram:PostalTradeAddress>", 1))), ids("BR-NL-28-4")},
		{"BR-NL-29 payment means text", one(after(aPMCode, elem("Information", "Overschrijving"))), ids("BR-NL-29")},
		{"BR-NL-30 account name", one(after(aIBAN, elem("AccountName", "Voorbeeld Netwerken"))), ids("BR-NL-30")},
		{"BR-NL-32-and-34 charge reason code", one(before(aChargeReason, elem("ReasonCode", "FC"))), ids("BR-NL-32-and-34")},
		{"BR-NL-32-and-34 line allowance reason code", one(before(aLineReason, elem("ReasonCode", "95"))), ids("BR-NL-32-and-34")},
		{"BR-NL-33 accounting currency", one(rep(aTaxTotal, `<ram:TaxTotalAmount currencyID="USD">230.00</ram:TaxTotalAmount>`)), ids("BR-NL-33")},
		{"BR-NL-33 no currency", one(rep(aTaxTotal, elem("TaxTotalAmount", "210.00"))), ids("BR-NL-33")},
		{"BR-NL-35 exemption reason code", one(repN(3, aRate, elem("ExemptionReasonCode", "VATEX-EU-79-C")+aRate)), ids("BR-NL-35")},
		{"empty-element-check", one(rep(aNote, "<ram:Content>  </ram:Content>")), ids("empty-element-check")},
		{"empty-element-check attributes only", one(after(aSellerName, `<ram:ID schemeID="0088"/>`)), ids("empty-element-check")},
	}
}

func TestMutations(t *testing.T) {
	doc := load(t, base)
	for _, c := range mutationCases() {
		t.Run(c.name, func(t *testing.T) {
			r := run(t, []byte(apply(t, doc, c.edits)))
			want := slices.Clone(c.want)
			slices.Sort(want)
			if got := r.Rules(); !slices.Equal(got, want) {
				t.Errorf("rules = %q, want %q\n%s", got, want, r)
			}
			checkFindings(t, r, Catalog)
		})
	}
}

// checkFindings verifies that every finding is in catalog, with its
// severity, and located.
func checkFindings(t *testing.T, r *rules.Report, catalog []rules.RuleInfo) {
	t.Helper()
	for _, f := range r.Findings {
		info, ok := catalogEntry(catalog, f.Rule)
		if !ok {
			t.Errorf("rule %s is not in the catalog", f.Rule)
			continue
		}
		if f.Severity != info.Severity || f.Source != rules.SourceNL || f.Message == "" || f.Path == "" || f.Line == 0 {
			t.Errorf("bad finding %+v", f)
		}
	}
}

func catalogEntry(catalog []rules.RuleInfo, id string) (rules.RuleInfo, bool) {
	for _, r := range catalog {
		if r.ID == id {
			return r, true
		}
	}
	return rules.RuleInfo{}, false
}

// TestCoverage checks that every assertion of Catalog fails in at least one
// mutation case.
func TestCoverage(t *testing.T) {
	failing := map[string]bool{}
	for _, c := range mutationCases() {
		for _, id := range c.want {
			failing[id] = true
		}
	}
	for _, r := range Catalog {
		if !failing[r.ID] {
			t.Errorf("no mutation case fails %s", r.ID)
		}
	}
}

func TestCatalog(t *testing.T) {
	for _, c := range []struct {
		name             string
		catalog          []rules.RuleInfo
		n, errs, warning int
	}{
		{"CII", Catalog, 32, 10, 22},
		{"UBL", UBLCatalog, 35, 14, 21},
	} {
		seen := map[string]bool{}
		sev := map[rules.Severity]int{}
		for _, r := range c.catalog {
			if seen[r.ID] {
				t.Errorf("%s: duplicate %s", c.name, r.ID)
			}
			seen[r.ID] = true
			sev[r.Severity]++
			if r.Description == "" || strings.HasPrefix(r.Description, "[") || strings.HasPrefix(r.Description, "/") {
				t.Errorf("%s: bad description for %s: %q", c.name, r.ID, r.Description)
			}
		}
		if len(c.catalog) != c.n || sev[rules.Error] != c.errs || sev[rules.Warning] != c.warning {
			t.Errorf("%s: %d assertions (%d errors, %d warnings), want %d (%d, %d)", c.name, len(c.catalog), sev[rules.Error], sev[rules.Warning], c.n, c.errs, c.warning)
		}
	}
}

func TestEmpty(t *testing.T) {
	Check(nil)
	Check(&rules.Checker{})
	Check(&rules.Checker{Doc: &rules.Doc{}, Report: &rules.Report{}})
	for _, doc := range []string{
		`<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"/>`,
		`<x/>`,
	} {
		r := run(t, []byte(doc))
		if got := r.Rules(); !slices.Equal(got, ids("empty-element-check")) {
			t.Errorf("%s: %v", doc, got)
		}
	}
}

func TestCasts(t *testing.T) {
	for s, want := range map[string]int{"0": 0, "-0.00": 0, "+.5": 1, "-3.": -1, " \n12.5\t": 1} {
		if got, ok := decimalSign(s); !ok || got != want {
			t.Errorf("decimalSign(%q) = %d, %v, want %d", s, got, ok, want)
		}
	}
	for _, s := range []string{"", ".", "+", "1e3", "1.2.3", "1 2", "١٢", "INF"} {
		if _, ok := decimalSign(s); ok {
			t.Errorf("decimalSign(%q) accepted", s)
		}
	}
	for s, want := range map[string]float64{"380": 380, " 0380.0 ": 380, "3.8E2": 380, "38e+1": 380, "1.": 1} {
		if got, ok := double(s); !ok || got != want {
			t.Errorf("double(%q) = %v, %v, want %v", s, got, ok, want)
		}
	}
	for _, s := range []string{"", "e5", "1e", "1e+", "0x10", "1_0", "inf", "Infinity", "1 e5", "--1"} {
		if _, ok := double(s); ok {
			t.Errorf("double(%q) accepted", s)
		}
	}
	if f, ok := double("-INF"); !ok || f > 0 {
		t.Errorf("double(-INF) = %v, %v", f, ok)
	}
	if f, ok := double("1e400"); !ok || f < 1e308 {
		t.Errorf("double(1e400) = %v, %v", f, ok)
	}
}

func TestXPath(t *testing.T) {
	if got := norm(" \t a \n\r b  c "); got != "a b c" {
		t.Errorf("norm = %q", got)
	}
	root, err := xmltree.Parse([]byte("<a> x <b>y</b><c><d> z</d></c></a>"))
	if err != nil {
		t.Fatal(err)
	}
	if got := nsv(root); got != "x y z" {
		t.Errorf("nsv = %q", got)
	}
	if exists(nil, "a") || exists(root, "c/e") || !exists(root, "c/d") {
		t.Error("exists")
	}
	defer func() {
		if recover() == nil {
			t.Error("compile accepted an unknown prefix")
		}
	}()
	compile("x:TypeCode", ciiNS)
}

// eachInvoice calls fn for every XML file below dir whose root is a CII or a
// UBL invoice.
func eachInvoice(t *testing.T, dir string, fn func(path string, doc *rules.Doc)) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".xml") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		root, err := xmltree.Parse(data)
		if err != nil {
			return nil
		}
		if doc := newDoc(root); doc != nil {
			fn(path, doc)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCorpus runs Check on the CII and UBL files of FX_CORPUS and
// FX_CORPUS_XML: every reported rule is in the catalog of the syntax, and no
// NLCIUS rule applies to an invoice that does not declare NLCIUS, except
// the checks of the empty elements and of the UBL customization identifier.
func TestCorpus(t *testing.T) {
	var dirs []string
	for _, env := range []string{"FX_CORPUS", "FX_CORPUS_XML"} {
		if d := os.Getenv(env); d != "" {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		t.Skip("FX_CORPUS and FX_CORPUS_XML are not set")
	}
	files := 0
	for _, dir := range dirs {
		eachInvoice(t, dir, func(path string, doc *rules.Doc) {
			files++
			r := checkDoc(doc)
			checkFindings(t, r, catalogOf(doc))
			for _, f := range r.Findings {
				if strings.HasPrefix(f.Rule, "BR-NL-") {
					t.Errorf("%s: %s on an invoice that does not declare NLCIUS", path, f.Rule)
				}
			}
		})
	}
	t.Logf("%d files", files)
}

func benchmark(b *testing.B, data []byte) {
	root, err := xmltree.Parse(data)
	if err != nil {
		b.Fatal(err)
	}
	doc := newDoc(root)
	b.ReportAllocs()
	for b.Loop() {
		r := &rules.Report{}
		Check(&rules.Checker{Doc: doc, Report: r, Source: rules.SourceNL})
		if len(r.Findings) > 0 {
			b.Fatal(r)
		}
	}
}

// BenchmarkCheck runs Check on the CII base invoice.
func BenchmarkCheck(b *testing.B) { benchmark(b, []byte(load(b, base))) }

// BenchmarkCheckUBL runs Check on the UBL base invoice.
func BenchmarkCheckUBL(b *testing.B) { benchmark(b, []byte(load(b, ublBase))) }
