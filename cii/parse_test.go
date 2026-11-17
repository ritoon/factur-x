package cii

import (
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

func readTestdata(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseEinfach(t *testing.T) {
	inv := mustParse(t, readTestdata(t, "EN16931_Einfach.cii.xml"))
	checks := []struct {
		name      string
		got, want any
	}{
		{"profile", inv.Profile, invoice.ProfileEN16931},
		{"BT-24", inv.SpecificationID, invoice.URNEN16931},
		{"BT-1", inv.Number, "471102"},
		{"BT-3", inv.TypeCode, "380"},
		{"BT-2", inv.IssueDate, dt("2018-03-05")},
		{"notes", len(inv.Notes), 2},
		{"BT-21", inv.Notes[1].SubjectCode, "REG"},
		{"BT-5", inv.Currency, "EUR"},
		{"seller", inv.Seller.Name, "Lieferant GmbH"},
		{"seller VAT", inv.Seller.VATID, "DE123456789"},
		{"seller FC", inv.Seller.TaxRegistrationID, "201/113/40209"},
		{"buyer", inv.Buyer.Name, "Kunden AG Mitte"},
		{"lines", len(inv.Lines), 2},
		{"BT-157", inv.Lines[0].Item.GlobalID, invoice.Identifier{ID: "4012345001235", Scheme: "0160"}},
		{"BT-129", inv.Lines[0].Quantity.String(), "20.0000"},
		{"BT-130", inv.Lines[0].UnitCode, "H87"},
		{"BT-146", inv.Lines[0].NetPrice.String(), "9.9000"},
		{"BT-131", inv.Lines[0].NetAmount.String(), "198.00"},
		{"BG-23", len(inv.VATBreakdown), 2},
		{"BT-112", inv.Totals.GrandTotal.String(), "529.87"},
		{"BT-110", inv.Totals.TaxTotal.String(), "56.87"},
		{"BT-115", inv.Totals.DuePayable.String(), "529.87"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if !strings.HasPrefix(inv.Notes[1].Content, "Lieferant GmbH") || strings.HasSuffix(inv.Notes[1].Content, " ") {
		t.Errorf("note not trimmed: %q", inv.Notes[1].Content)
	}
}

func TestParseXRechnung(t *testing.T) {
	inv := mustParse(t, readTestdata(t, "XRECHNUNG_Einfach.cii.xml"))
	if inv.Profile != invoice.ProfileXRechnung {
		t.Errorf("profile = %v", inv.Profile)
	}
	if inv.BuyerReference == "" {
		t.Error("BT-10 not read")
	}
	out := mustMarshal(t, inv, nil)
	back := mustParse(t, out)
	if back.SpecificationID != inv.SpecificationID {
		t.Errorf("XRechnung BT-24 not kept: %q", back.SpecificationID)
	}
}

const ciiHead = `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100" xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100" xmlns:qdt="urn:un:unece:uncefact:data:standard:QualifiedDataType:100">`

func TestParseIssues(t *testing.T) {
	doc := ciiHead + `
<rsm:ExchangedDocumentContext><ram:GuidelineSpecifiedDocumentContextParameter><ram:ID>urn:cen.eu:en16931:2017</ram:ID></ram:GuidelineSpecifiedDocumentContextParameter></rsm:ExchangedDocumentContext>
<rsm:ExchangedDocument><ram:ID>1</ram:ID><ram:TypeCode>380</ram:TypeCode>
<ram:IssueDateTime><udt:DateTimeString format="102">20261331</udt:DateTimeString></ram:IssueDateTime></rsm:ExchangedDocument>
<rsm:SupplyChainTradeTransaction>
<ram:ApplicableHeaderTradeSettlement>
<ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
<ram:SpecifiedTradeAllowanceCharge><ram:ChargeIndicator><udt:Indicator>maybe</udt:Indicator></ram:ChargeIndicator><ram:ActualAmount>1.0</ram:ActualAmount></ram:SpecifiedTradeAllowanceCharge>
<ram:SpecifiedTradeSettlementHeaderMonetarySummation>
<ram:TaxBasisTotalAmount>12,50</ram:TaxBasisTotalAmount>
<ram:GrandTotalAmount> 13.75 </ram:GrandTotalAmount>
</ram:SpecifiedTradeSettlementHeaderMonetarySummation>
</ram:ApplicableHeaderTradeSettlement>
</rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`
	inv, err := Parse([]byte(doc))
	var ie *IssuesError
	if !errors.As(err, &ie) {
		t.Fatalf("error = %v, want *IssuesError", err)
	}
	if inv == nil {
		t.Fatal("nil invoice with IssuesError")
	}
	if len(ie.Issues) != 3 {
		t.Fatalf("issues = %v", ie.Issues)
	}
	wantPaths := []string{
		"/rsm:CrossIndustryInvoice/rsm:ExchangedDocument/ram:IssueDateTime/udt:DateTimeString",
		"/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeSettlement/ram:SpecifiedTradeAllowanceCharge/ram:ChargeIndicator/udt:Indicator",
		"/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeSettlement/ram:SpecifiedTradeSettlementHeaderMonetarySummation/ram:TaxBasisTotalAmount",
	}
	for i, is := range ie.Issues {
		if is.Path != wantPaths[i] || is.Line == 0 || is.Message == "" {
			t.Errorf("issue %d = %+v", i, is)
		}
	}
	if !inv.IssueDate.IsZero() || inv.Totals.TaxBasisTotal.IsSet() {
		t.Error("unreadable values not left empty")
	}
	if inv.Number != "1" || inv.Totals.GrandTotal.String() != "13.75" || inv.AllowancesCharges[0].Amount.String() != "1.0" {
		t.Error("reading did not continue after the issues")
	}
	if !strings.Contains(err.Error(), "3 unreadable values") {
		t.Errorf("error text = %q", err)
	}

	// ParseTree reports the same issues.
	root, err := xmltree.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if _, issues := ParseTree(root); len(issues) != 3 {
		t.Errorf("ParseTree issues = %v", issues)
	}
}

func TestParseRejects(t *testing.T) {
	cases := []struct {
		name   string
		doc    string
		notCII bool
	}{
		{"empty", "", false},
		{"not xml", "%PDF-1.7 garbage", false},
		{"truncated", ciiHead + "<rsm:ExchangedDocument>", false},
		{"zugferd1", `<rsm:CrossIndustryDocument xmlns:rsm="urn:ferd:CrossIndustryDocument:invoice:1p0"/>`, true},
		{"ubl", `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`, true},
		{"wrong namespace", `<CrossIndustryInvoice xmlns="urn:example"/>`, true},
		{"xmp", `<x:xmpmeta xmlns:x="adobe:ns:meta/"/>`, true},
	}
	for _, c := range cases {
		inv, err := Parse([]byte(c.doc))
		if err == nil || inv != nil {
			t.Errorf("%s: Parse = %v, %v; want an error", c.name, inv, err)
			continue
		}
		var ie *IssuesError
		if errors.As(err, &ie) {
			t.Errorf("%s: got IssuesError", c.name)
		}
		if got := errors.Is(err, ErrNotCII); got != c.notCII {
			t.Errorf("%s: errors.Is(err, ErrNotCII) = %v (%v)", c.name, got, err)
		}
	}
	root, _ := xmltree.Parse([]byte(`<rsm:CrossIndustryDocument xmlns:rsm="urn:ferd:CrossIndustryDocument:invoice:1p0"/>`))
	if inv, issues := ParseTree(root); inv != nil || len(issues) != 1 {
		t.Errorf("ParseTree(ZUGFeRD 1) = %v, %v", inv, issues)
	}
	if inv, issues := ParseTree(nil); inv != nil || len(issues) != 1 {
		t.Errorf("ParseTree(nil) = %v, %v", inv, issues)
	}
}

// TestParseTolerance reads a document with unusual prefixes, a default
// namespace, white space around values, elements out of order and a
// ZUGFeRD 2.0 identifier.
func TestParseTolerance(t *testing.T) {
	doc := `<?xml version="1.0" encoding="ISO-8859-1"?>
<CrossIndustryInvoice xmlns="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" xmlns:a="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100" xmlns:b="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
  <ExchangedDocument>
    <a:TypeCode>
      381
    </a:TypeCode>
    <a:ID>  CN-1  </a:ID>
    <a:IssueDateTime><b:DateTimeString format="102"> 20200131 </b:DateTimeString></a:IssueDateTime>
  </ExchangedDocument>
  <ExchangedDocumentContext>
    <a:GuidelineSpecifiedDocumentContextParameter><a:ID> urn:cen.eu:en16931:2017#conformant#urn:zugferd.de:2p0:extended </a:ID></a:GuidelineSpecifiedDocumentContextParameter>
  </ExchangedDocumentContext>
  <SupplyChainTradeTransaction>
    <a:ApplicableHeaderTradeSettlement>
      <a:SpecifiedTradeSettlementHeaderMonetarySummation>
        <a:TaxTotalAmount currencyID="CHF">10.00</a:TaxTotalAmount>
        <a:TaxTotalAmount currencyID="EUR">9.50</a:TaxTotalAmount>
      </a:SpecifiedTradeSettlementHeaderMonetarySummation>
      <a:InvoiceCurrencyCode>EUR</a:InvoiceCurrencyCode>
      <a:TaxCurrencyCode>CHF</a:TaxCurrencyCode>
    </a:ApplicableHeaderTradeSettlement>
    <a:ApplicableHeaderTradeAgreement>
      <a:SellerTradeParty><a:Name>Caf` + "\xe9" + `</a:Name></a:SellerTradeParty>
    </a:ApplicableHeaderTradeAgreement>
    <a:IncludedSupplyChainTradeLineItem>
      <a:SpecifiedLineTradeAgreement>
        <a:GrossPriceProductTradePrice><a:ChargeAmount>12</a:ChargeAmount><a:BasisQuantity unitCode="KGM">10</a:BasisQuantity></a:GrossPriceProductTradePrice>
        <a:NetPriceProductTradePrice><a:ChargeAmount>11</a:ChargeAmount></a:NetPriceProductTradePrice>
      </a:SpecifiedLineTradeAgreement>
      <a:AssociatedDocumentLineDocument>
        <a:LineID>1</a:LineID>
        <a:IncludedNote><a:Content>first</a:Content></a:IncludedNote>
        <a:IncludedNote><a:Content>second</a:Content></a:IncludedNote>
      </a:AssociatedDocumentLineDocument>
    </a:IncludedSupplyChainTradeLineItem>
  </SupplyChainTradeTransaction>
</CrossIndustryInvoice>`
	inv := mustParse(t, []byte(doc))
	if inv.Profile != invoice.ProfileExtended {
		t.Errorf("profile = %v", inv.Profile)
	}
	if inv.Number != "CN-1" || inv.TypeCode != "381" || inv.IssueDate != dt("2020-01-31") {
		t.Errorf("document = %q %q %v", inv.Number, inv.TypeCode, inv.IssueDate)
	}
	if inv.Seller.Name != "Café" {
		t.Errorf("seller = %q", inv.Seller.Name)
	}
	// BT-111 is the amount in the VAT accounting currency, whatever its
	// position.
	if inv.Totals.TaxTotal.String() != "9.50" || inv.Totals.TaxTotalInTaxCurrency.String() != "10.00" {
		t.Errorf("BT-110 = %v, BT-111 = %v", inv.Totals.TaxTotal, inv.Totals.TaxTotalInTaxCurrency)
	}
	l := inv.Lines[0]
	if l.PriceBaseQuantity.String() != "10" || l.PriceBaseQuantityUnit != "KGM" {
		t.Errorf("BT-149/150 from the gross price = %v %q", l.PriceBaseQuantity, l.PriceBaseQuantityUnit)
	}
	// BT-127 is the first note; EXTENDED allows several, the others are
	// kept as extensions.
	if l.Note != "first" {
		t.Errorf("line note = %q", l.Note)
	}
	want := []invoice.Extension{{Path: "AssociatedDocumentLineDocument", XML: "<ram:IncludedNote><ram:Content>second</ram:Content></ram:IncludedNote>"}}
	if !reflect.DeepEqual(l.Extensions, want) {
		t.Errorf("line extensions = %+v", l.Extensions)
	}
	// The currencyID of the totals are implied by BT-5 and BT-6.
	if len(inv.Extensions) > 0 {
		t.Errorf("invoice extensions = %+v", inv.Extensions)
	}
}

func TestParseOddSpecificationID(t *testing.T) {
	for urn, want := range map[string]invoice.Profile{
		"urn:cen.eu:en16931:2017:compliant:factur-x.eu:1p0:basic":                    invoice.ProfileBasic,
		"urn:cen.eu:en16931:2017#compliant#urn:factur-x.eu:1p0:basicwl":              invoice.ProfileBasicWL,
		"urn:cen.eu:en16931:2017#compliant#urn:xoev-de:kosit:standard:xrechnung_1.2": invoice.ProfileXRechnung,
		"urn:cen.eu:en16931:2017#conformant#urn:example:extended-ctc-fr":             invoice.ProfileExtendedCTCFR,
		"urn:cen.eu:en16931:2017#compliant#urn:example:other":                        invoice.ProfileEN16931,
		"urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0": invoice.ProfilePeppol,
		"urn:example:other": invoice.ProfileUnknown,
		"":                  invoice.ProfileUnknown,
	} {
		if got := detectProfile(urn); got != want {
			t.Errorf("detectProfile(%q) = %v, want %v", urn, got, want)
		}
	}
	// A non-standard identifier is kept when writing the same profile.
	inv := &invoice.Invoice{Profile: invoice.ProfileBasic, SpecificationID: "urn:cen.eu:en16931:2017:compliant:factur-x.eu:1p0:basic"}
	back := mustParse(t, mustMarshal(t, inv, nil))
	if back.SpecificationID != inv.SpecificationID || back.Profile != invoice.ProfileBasic {
		t.Errorf("BT-24 = %q (%v)", back.SpecificationID, back.Profile)
	}
}

// TestParsePriceAllowances sums the EXTENDED price allowances into BT-147
// and details the allowances and charges of the price, which EXTENDED writes
// back and EN 16931 reports as lost.
func TestParsePriceAllowances(t *testing.T) {
	doc := ciiHead + `<rsm:SupplyChainTradeTransaction><ram:IncludedSupplyChainTradeLineItem><ram:SpecifiedLineTradeAgreement>
<ram:GrossPriceProductTradePrice><ram:ChargeAmount>1.5000</ram:ChargeAmount>
<ram:AppliedTradeAllowanceCharge><ram:ChargeIndicator><udt:Indicator>false</udt:Indicator></ram:ChargeIndicator><ram:ActualAmount>0.0300</ram:ActualAmount></ram:AppliedTradeAllowanceCharge>
<ram:AppliedTradeAllowanceCharge><ram:ChargeIndicator><udt:Indicator>true</udt:Indicator></ram:ChargeIndicator><ram:ActualAmount>9</ram:ActualAmount></ram:AppliedTradeAllowanceCharge>
<ram:AppliedTradeAllowanceCharge><ram:ChargeIndicator><udt:Indicator>false</udt:Indicator></ram:ChargeIndicator><ram:ActualAmount>0.02</ram:ActualAmount></ram:AppliedTradeAllowanceCharge>
</ram:GrossPriceProductTradePrice>
</ram:SpecifiedLineTradeAgreement></ram:IncludedSupplyChainTradeLineItem></rsm:SupplyChainTradeTransaction></rsm:CrossIndustryInvoice>`
	inv := mustParse(t, []byte(doc))
	l := &inv.Lines[0]
	if got := l.PriceDiscount.String(); got != "0.0500" {
		t.Errorf("BT-147 = %s, want 0.0500", got)
	}
	if len(l.PriceAllowancesCharges) != 3 || !l.PriceAllowancesCharges[1].Charge || l.PriceAllowancesCharges[1].Amount.String() != "9" {
		t.Fatalf("price allowances and charges = %+v", l.PriceAllowancesCharges)
	}
	out := mustMarshal(t, inv, &MarshalOptions{Profile: invoice.ProfileExtended, Strict: true})
	if n := strings.Count(string(out), "<ram:AppliedTradeAllowanceCharge>"); n != 3 {
		t.Errorf("EXTENDED output has %d price allowances and charges:\n%s", n, out)
	}
	back := mustParse(t, out)
	if !reflect.DeepEqual(back.Lines[0].PriceAllowancesCharges, l.PriceAllowancesCharges) || back.Lines[0].PriceDiscount != l.PriceDiscount {
		t.Errorf("EXTENDED round trip = %+v", back.Lines[0])
	}
	_, err := Marshal(inv, &MarshalOptions{Profile: invoice.ProfileEN16931, Strict: true})
	var le *LossError
	if !errors.As(err, &le) || !slices.Contains(le.Terms, "EXT Line.PriceAllowancesCharges") {
		t.Errorf("EN 16931: %v", err)
	}
	en := mustParse(t, mustMarshal(t, inv, &MarshalOptions{Profile: invoice.ProfileEN16931}))
	if en.Lines[0].PriceDiscount.String() != "0.0500" || en.Lines[0].PriceAllowancesCharges != nil {
		t.Errorf("EN 16931 line = %+v", en.Lines[0])
	}
}

// TestParseUltimateShipTo is the case of ticket #5: an EXTENDED document
// with an ultimate ship-to party, which used to be dropped silently.
func TestParseUltimateShipTo(t *testing.T) {
	doc := strings.Replace(string(readTestdata(t, "EN16931_Einfach.cii.xml")), "<ram:ID>urn:cen.eu:en16931:2017</ram:ID>", "<ram:ID>"+invoice.URNExtended+"</ram:ID>", 1)
	doc = strings.Replace(doc, "<ram:ApplicableHeaderTradeDelivery>", `<ram:ApplicableHeaderTradeDelivery>
      <ram:UltimateShipToTradeParty>
        <ram:Name>Entrepôt final</ram:Name>
        <ram:PostalTradeAddress><ram:CityName>Lyon</ram:CityName><ram:CountryID>FR</ram:CountryID></ram:PostalTradeAddress>
      </ram:UltimateShipToTradeParty>`, 1)
	inv := mustParse(t, []byte(doc))
	if inv.Profile != invoice.ProfileExtended {
		t.Fatalf("profile = %v", inv.Profile)
	}
	if p := inv.UltimateShipTo; p == nil || p.Name != "Entrepôt final" || p.Address.City != "Lyon" {
		t.Fatalf("ultimate ship-to = %+v", p)
	}
	if u := inv.Unmodeled(); len(u) > 0 {
		t.Errorf("unmodeled = %v", u)
	}
	out := mustMarshal(t, inv, &MarshalOptions{Strict: true})
	checkXSD(t, invoice.ProfileExtended, out)
	if !strings.Contains(string(out), "<ram:UltimateShipToTradeParty><ram:Name>Entrepôt final</ram:Name>") {
		t.Errorf("the ultimate ship-to party is not written:\n%s", out)
	}
	orig, _ := xmltree.Parse([]byte(doc))
	back, _ := xmltree.Parse(out)
	if d := diffTrees(orig, back, treeNorm{cur: inv.Currency, taxCur: inv.TaxCurrency, spec: true}); len(d.missing) > 0 || d.order != "" {
		t.Errorf("the output does not hold the tree of the original:\n%v", d)
	}
	// EN 16931 cannot carry it: Strict reports the loss.
	_, err := Marshal(inv, &MarshalOptions{Profile: invoice.ProfileEN16931, Strict: true})
	var le *LossError
	if !errors.As(err, &le) || !slices.Equal(le.Terms, []string{"EXT UltimateShipTo"}) {
		t.Errorf("EN 16931: %v", err)
	}
}
