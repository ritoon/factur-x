package cii

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/schema"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

const zf1Sample = "ZUGFeRD_1p0_COMFORT_SEPA_Prenotification.xml"

// zf1Head opens a ZUGFeRD 1.0 document; the test adds the elements under the
// root and closes it with zf1Tail.
const zf1Head = `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryDocument xmlns:rsm="urn:ferd:CrossIndustryDocument:invoice:1p0" xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:12" xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:15">
`

const zf1Tail = `</rsm:CrossIndustryDocument>`

// zf1Extended is a ZUGFeRD 1.0 EXTENDED invoice that uses the elements that
// the CII D16B view renames, moves or rewrites: referenced documents whose
// ID follows the date, dates as text with a time, in format 610 or 616,
// customer orders, a line accounting account, the SEPA direct debit data of
// the payment means, and an element that CII D16B does not know.
const zf1Extended = zf1Head + `<rsm:SpecifiedExchangedDocumentContext>
<ram:TestIndicator><udt:Indicator>false</udt:Indicator></ram:TestIndicator>
<ram:GuidelineSpecifiedDocumentContextParameter><ram:ID>urn:ferd:CrossIndustryDocument:invoice:1p0:extended</ram:ID></ram:GuidelineSpecifiedDocumentContextParameter>
</rsm:SpecifiedExchangedDocumentContext>
<rsm:HeaderExchangedDocument>
<ram:ID>ZF1-2014-7</ram:ID>
<ram:Name>RECHNUNG</ram:Name>
<ram:TypeCode>380</ram:TypeCode>
<ram:IssueDateTime><udt:DateTimeString format="102">20140305</udt:DateTimeString></ram:IssueDateTime>
<ram:IncludedNote><ram:Content>Vielen Dank.</ram:Content></ram:IncludedNote>
</rsm:HeaderExchangedDocument>
<rsm:SpecifiedSupplyChainTradeTransaction>
<ram:ApplicableSupplyChainTradeAgreement>
<ram:BuyerReference>LW-1</ram:BuyerReference>
<ram:SellerTradeParty><ram:Name>Lieferant GmbH</ram:Name>
<ram:PostalTradeAddress><ram:PostcodeCode>80333</ram:PostcodeCode><ram:CityName>München</ram:CityName><ram:CountryID>DE</ram:CountryID></ram:PostalTradeAddress>
<ram:SpecifiedTaxRegistration><ram:ID schemeID="VA">DE123456789</ram:ID></ram:SpecifiedTaxRegistration>
</ram:SellerTradeParty>
<ram:BuyerTradeParty><ram:Name>Kunden AG</ram:Name></ram:BuyerTradeParty>
<ram:BuyerOrderReferencedDocument><ram:IssueDateTime>2014-02-20</ram:IssueDateTime><ram:ID>PO-42</ram:ID></ram:BuyerOrderReferencedDocument>
<ram:CustomerOrderReferencedDocument><ram:IssueDateTime>2014-02-28T10:30:00</ram:IssueDateTime><ram:ID>CO-7</ram:ID></ram:CustomerOrderReferencedDocument>
</ram:ApplicableSupplyChainTradeAgreement>
<ram:ApplicableSupplyChainTradeDelivery>
<ram:ActualDeliverySupplyChainEvent><ram:OccurrenceDateTime><udt:DateTimeString format="102">20140303</udt:DateTimeString></ram:OccurrenceDateTime></ram:ActualDeliverySupplyChainEvent>
</ram:ApplicableSupplyChainTradeDelivery>
<ram:ApplicableSupplyChainTradeSettlement>
<ram:PaymentReference>ZF1-2014-7</ram:PaymentReference>
<ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
<ram:SpecifiedTradeSettlementPaymentMeans>
<ram:TypeCode>59</ram:TypeCode>
<ram:ID schemeAgencyID="DE98ZZZ09999999999">MANDAT-9</ram:ID>
<ram:PayerPartyDebtorFinancialAccount><ram:IBANID>DE21860000000086001055</ram:IBANID></ram:PayerPartyDebtorFinancialAccount>
<ram:PayeeSpecifiedCreditorFinancialInstitution><ram:BICID>COBADEFFXXX</ram:BICID><ram:GermanBankleitzahlID>37040044</ram:GermanBankleitzahlID></ram:PayeeSpecifiedCreditorFinancialInstitution>
</ram:SpecifiedTradeSettlementPaymentMeans>
<ram:ApplicableTradeTax><ram:CalculatedAmount currencyID="EUR">19.00</ram:CalculatedAmount><ram:TypeCode>VAT</ram:TypeCode><ram:BasisAmount currencyID="EUR">100.00</ram:BasisAmount><ram:CategoryCode>S</ram:CategoryCode><ram:ApplicablePercent>19.00</ram:ApplicablePercent></ram:ApplicableTradeTax>
<ram:BillingSpecifiedPeriod>
<ram:StartDateTime><udt:DateTimeString format="616">201410</udt:DateTimeString></ram:StartDateTime>
<ram:EndDateTime><udt:DateTimeString format="610">201403</udt:DateTimeString></ram:EndDateTime>
</ram:BillingSpecifiedPeriod>
<ram:SpecifiedTradeSettlementMonetarySummation>
<ram:LineTotalAmount currencyID="EUR">100.00</ram:LineTotalAmount>
<ram:ChargeTotalAmount currencyID="EUR">0.00</ram:ChargeTotalAmount>
<ram:AllowanceTotalAmount currencyID="EUR">0.00</ram:AllowanceTotalAmount>
<ram:TaxBasisTotalAmount currencyID="EUR">100.00</ram:TaxBasisTotalAmount>
<ram:TaxTotalAmount currencyID="EUR">19.00</ram:TaxTotalAmount>
<ram:GrandTotalAmount currencyID="EUR">119.00</ram:GrandTotalAmount>
<ram:DuePayableAmount currencyID="EUR">119.00</ram:DuePayableAmount>
</ram:SpecifiedTradeSettlementMonetarySummation>
</ram:ApplicableSupplyChainTradeSettlement>
<ram:IncludedSupplyChainTradeLineItem>
<ram:AssociatedDocumentLineDocument><ram:LineID>1</ram:LineID></ram:AssociatedDocumentLineDocument>
<ram:SpecifiedSupplyChainTradeAgreement>
<ram:BuyerOrderReferencedDocument><ram:LineID>3</ram:LineID><ram:ID>PO-42</ram:ID></ram:BuyerOrderReferencedDocument>
<ram:NetPriceProductTradePrice><ram:ChargeAmount currencyID="EUR">10.00</ram:ChargeAmount></ram:NetPriceProductTradePrice>
<ram:CustomerOrderReferencedDocument><ram:LineID>2</ram:LineID><ram:ID>CO-7</ram:ID></ram:CustomerOrderReferencedDocument>
</ram:SpecifiedSupplyChainTradeAgreement>
<ram:SpecifiedSupplyChainTradeDelivery><ram:BilledQuantity unitCode="C62">10.0000</ram:BilledQuantity></ram:SpecifiedSupplyChainTradeDelivery>
<ram:SpecifiedSupplyChainTradeSettlement>
<ram:ApplicableTradeTax><ram:TypeCode>VAT</ram:TypeCode><ram:CategoryCode>S</ram:CategoryCode><ram:ApplicablePercent>19.00</ram:ApplicablePercent></ram:ApplicableTradeTax>
<ram:SpecifiedTradeAccountingAccount><ram:ID>4711</ram:ID></ram:SpecifiedTradeAccountingAccount>
<ram:SpecifiedTradeSettlementMonetarySummation><ram:LineTotalAmount currencyID="EUR">100.00</ram:LineTotalAmount></ram:SpecifiedTradeSettlementMonetarySummation>
</ram:SpecifiedSupplyChainTradeSettlement>
<ram:SpecifiedTradeProduct><ram:Name>Schrauben</ram:Name></ram:SpecifiedTradeProduct>
</ram:IncludedSupplyChainTradeLineItem>
</rsm:SpecifiedSupplyChainTradeTransaction>
` + zf1Tail

// snapshot returns the content of a tree, one line per element and
// attribute, to check that it is not modified.
func snapshot(root *xmltree.Node) []string {
	var out []string
	root.Walk(func(n *xmltree.Node) bool {
		out = append(out, fmt.Sprintf("%s {%s} %q %d", n.Path(), n.Space, n.Text, n.Line))
		for _, a := range n.Attrs {
			out = append(out, fmt.Sprintf("%s/@%s:%s=%q", n.Path(), a.Prefix, a.Name, a.Value))
		}
		return true
	})
	return out
}

// checkSchema checks out, written for profile p, against the XML schema of
// p (internal/schema, which needs no XSD file).
func checkSchema(t *testing.T, p invoice.Profile, out []byte) {
	t.Helper()
	root, err := xmltree.Parse(out)
	if err != nil {
		t.Fatalf("output is not well-formed: %v", err)
	}
	r := &rules.Report{}
	schema.Check(&rules.Checker{Doc: rules.NewDoc(root, p), Report: r, Source: rules.SourceSyntax})
	for _, f := range r.Findings {
		t.Errorf("%v schema: %s %s: %s", p, f.Rule, f.Path, f.Message)
	}
}

func TestIsZUGFeRD1(t *testing.T) {
	cases := []struct {
		doc  string
		want bool
	}{
		{`<rsm:CrossIndustryDocument xmlns:rsm="urn:ferd:CrossIndustryDocument:invoice:1p0"/>`, true},
		{`<CrossIndustryDocument xmlns="urn:ferd:CrossIndustryDocument:invoice:1p0"/>`, true},
		{`<rsm:Invoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CBFBUY:5"/>`, true},
		{`<rsm:CrossIndustryDocument xmlns:rsm="urn:ferd:CrossIndustryDocument:order:1p0"/>`, false},
		{`<rsm:HeaderExchangedDocument xmlns:rsm="urn:ferd:CrossIndustryDocument:invoice:1p0"/>`, false},
		{`<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"/>`, false},
		{`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`, false},
	}
	for _, c := range cases {
		root, err := xmltree.Parse([]byte(c.doc))
		if err != nil {
			t.Fatal(err)
		}
		if got := IsZUGFeRD1(root); got != c.want {
			t.Errorf("IsZUGFeRD1(%s) = %v", c.doc, got)
		}
	}
	if IsZUGFeRD1(nil) {
		t.Error("IsZUGFeRD1(nil) = true")
	}
}

// TestZUGFeRD1Sample reads a ZUGFeRD 1.0 COMFORT invoice of the ZUGFeRD
// corpus, with a SEPA direct debit, and converts it to Factur-X.
func TestZUGFeRD1Sample(t *testing.T) {
	data := readTestdata(t, zf1Sample)
	inv := mustParse(t, data)
	checks := []struct {
		name      string
		got, want any
	}{
		{"profile", inv.Profile, invoice.ProfileEN16931},
		{"BT-24", inv.SpecificationID, invoice.URNZUGFeRD1Comfort},
		{"test", inv.TestIndicator, true},
		{"BT-1", inv.Number, "471102"},
		{"BT-3", inv.TypeCode, "380"},
		{"BT-2", inv.IssueDate, dt("2014-03-05")},
		{"BT-5", inv.Currency, "EUR"},
		{"seller", inv.Seller.Name, "Lieferant GmbH"},
		{"BT-90", inv.CreditorReferenceID, "DE98ZZZ09999999999"},
		{"payment means", len(inv.PaymentMeans), 1},
		{"BT-81", inv.PaymentMeans[0].TypeCode, "49"},
		{"BT-91", inv.PaymentMeans[0].PayerIBAN, "DE21860000000086001055"},
		{"BT-215", inv.PaymentMeans[0].PayerBIC, "MARKDEF1860"},
		{"payment terms", len(inv.PaymentTerms), 1},
		{"BT-89", inv.PaymentTerms[0].DirectDebitMandateID, "REF A-123"},
		{"BT-9", inv.PaymentTerms[0].DueDate, dt("2014-03-20")},
		{"BG-23", len(inv.VATBreakdown), 2},
		{"BT-119", inv.VATBreakdown[0].Rate.String(), "7.00"},
		{"BT-119 (2)", inv.VATBreakdown[1].Rate.String(), "19.00"},
		{"BT-112", inv.Totals.GrandTotal.String(), "529.87"},
		{"BT-115", inv.Totals.DuePayable.String(), "529.87"},
		{"lines", len(inv.Lines), 2},
		{"BT-152", inv.Lines[0].VATRate.String(), "19.00"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	// The data that CII D16B cannot carry, kept with their D16B location.
	wantUnmodeled := []string{
		"ExchangedDocument/Name",
		"PaymentMeans[0]: PayerSpecifiedDebtorFinancialInstitution/Name",
	}
	if got := inv.Unmodeled(); !slices.Equal(got, wantUnmodeled) {
		t.Errorf("Unmodeled = %q, want %q", got, wantUnmodeled)
	}

	// ParseTree does not modify the document.
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	before := snapshot(root)
	if _, issues := ParseTree(root); len(issues) != 0 {
		t.Fatalf("ParseTree issues = %v", issues)
	}
	if after := snapshot(root); !slices.Equal(before, after) {
		t.Error("ParseTree modified the ZUGFeRD 1.0 document")
	}

	// Converted to Factur-X: the identifier of the target profile, a valid
	// document, and only the data that the profile cannot carry lost.
	lost := "EXT SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/SpecifiedTradeSettlementPaymentMeans/PayerSpecifiedDebtorFinancialInstitution/Name"
	for _, c := range []struct {
		p    invoice.Profile
		lost []string
	}{
		{invoice.ProfileExtended, []string{lost}},
		{invoice.ProfileEN16931, []string{"EXT TestIndicator", "EXT ExchangedDocument/Name", "BT-215", lost}},
	} {
		t.Run(c.p.String(), func(t *testing.T) {
			_, err := Marshal(inv, &MarshalOptions{Profile: c.p, Strict: true})
			var le *LossError
			if !errors.As(err, &le) || !slices.Equal(le.Terms, c.lost) {
				t.Errorf("strict Marshal: %v, want the loss of %q", err, c.lost)
			}
			out := mustMarshal(t, inv, &MarshalOptions{Profile: c.p})
			checkSchema(t, c.p, out)
			back := mustParse(t, out)
			if back.Profile != c.p || back.SpecificationID != c.p.URN() {
				t.Errorf("BT-24 = %q (%v), want %q", back.SpecificationID, back.Profile, c.p.URN())
			}
			if c.p != invoice.ProfileExtended {
				return
			}
			// EXTENDED keeps everything but the lost extension.
			want := mustParse(t, data)
			want.Profile, want.SpecificationID = c.p, c.p.URN()
			want.PaymentMeans[0].Extensions = nil
			if d := diffInvoices(want, back); d != "" {
				t.Errorf("EXTENDED round trip:\n%s", d)
			}
		})
	}
}

// TestZUGFeRD1View reads a ZUGFeRD 1.0 EXTENDED invoice through the
// elements that the CII D16B view renames, moves or rewrites.
func TestZUGFeRD1View(t *testing.T) {
	inv := mustParse(t, []byte(zf1Extended))
	checks := []struct {
		name      string
		got, want any
	}{
		{"profile", inv.Profile, invoice.ProfileExtended},
		{"BT-24", inv.SpecificationID, invoice.URNZUGFeRD1Extended},
		{"BT-1", inv.Number, "ZF1-2014-7"},
		{"BT-22", inv.Notes[0].Content, "Vielen Dank."},
		{"BT-10", inv.BuyerReference, "LW-1"},
		{"BT-31", inv.Seller.VATID, "DE123456789"},
		{"BT-13", inv.PurchaseOrderReference, "PO-42"},
		{"customer orders", len(inv.UltimateCustomerOrders), 1},
		{"customer order", inv.UltimateCustomerOrders[0].ID, "CO-7"},
		{"customer order date", inv.UltimateCustomerOrders[0].IssueDate, dt("2014-02-28")},
		{"BT-72", inv.DeliveryDate, dt("2014-03-03")},
		{"BT-73 (616)", inv.InvoicingPeriod.Start, dt("2014-03-03")},
		{"BT-74 (610)", inv.InvoicingPeriod.End, dt("2014-03-01")},
		{"BT-83", inv.RemittanceInformation, "ZF1-2014-7"},
		{"BT-90", inv.CreditorReferenceID, "DE98ZZZ09999999999"},
		{"BT-81", inv.PaymentMeans[0].TypeCode, "59"},
		{"BT-86", inv.PaymentMeans[0].PayeeBIC, "COBADEFFXXX"},
		{"BT-91", inv.PaymentMeans[0].PayerIBAN, "DE21860000000086001055"},
		{"payment terms", len(inv.PaymentTerms), 1},
		{"BT-89", inv.PaymentTerms[0].DirectDebitMandateID, "MANDAT-9"},
		{"BT-119", inv.VATBreakdown[0].Rate.String(), "19.00"},
		{"BT-112", inv.Totals.GrandTotal.String(), "119.00"},
		{"BT-132", inv.Lines[0].OrderLineReference, "3"},
		{"line customer orders", len(inv.Lines[0].UltimateCustomerOrders), 1},
		{"line customer order", inv.Lines[0].UltimateCustomerOrders[0].ID, "CO-7"},
		{"line customer order line", inv.Lines[0].UltimateCustomerOrders[0].LineID, "2"},
		{"BT-133", inv.Lines[0].AccountingReference, "4711"},
		{"BT-146", inv.Lines[0].NetPrice.String(), "10.00"},
		{"BT-129", inv.Lines[0].Quantity.String(), "10.0000"},
		{"BT-152", inv.Lines[0].VATRate.String(), "19.00"},
		{"BT-131", inv.Lines[0].NetAmount.String(), "100.00"},
		{"BT-153", inv.Lines[0].Item.Name, "Schrauben"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if inv.TestIndicator {
		t.Error("TestIndicator false read as true")
	}
	// The order date and the German bank code, which CII D16B does not
	// carry, are kept as extensions located in the D16B document.
	unmodeled := strings.Join(inv.Unmodeled(), "\n")
	for _, w := range []string{"BuyerOrderReferencedDocument/FormattedIssueDateTime", "PaymentMeans[0]: PayeeSpecifiedCreditorFinancialInstitution/GermanBankleitzahlID"} {
		if !strings.Contains(unmodeled, w) {
			t.Errorf("Unmodeled lacks %q:\n%s", w, unmodeled)
		}
	}
	_, err := Marshal(inv, &MarshalOptions{Profile: invoice.ProfileExtended, Strict: true})
	var le *LossError
	if !errors.As(err, &le) || !slices.Contains(le.Terms,
		"EXT SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/SpecifiedTradeSettlementPaymentMeans/PayeeSpecifiedCreditorFinancialInstitution/GermanBankleitzahlID") {
		t.Errorf("strict Marshal: %v, want the loss of the German bank code", err)
	}
	out := mustMarshal(t, inv, &MarshalOptions{Profile: invoice.ProfileExtended})
	checkSchema(t, invoice.ProfileExtended, out)
	back := mustParse(t, out)
	if back.PaymentTerms[0].DirectDebitMandateID != "MANDAT-9" || back.CreditorReferenceID != "DE98ZZZ09999999999" ||
		back.InvoicingPeriod.Start != dt("2014-03-03") || back.Lines[0].AccountingReference != "4711" {
		t.Errorf("EXTENDED output lost data:\n%s", out)
	}
}

// TestZUGFeRD1DirectDebit checks where the SEPA direct debit data of the
// payment means go.
func TestZUGFeRD1DirectDebit(t *testing.T) {
	settlement := func(content string) string {
		return zf1Head + `<rsm:SpecifiedExchangedDocumentContext><ram:GuidelineSpecifiedDocumentContextParameter><ram:ID>urn:ferd:CrossIndustryDocument:invoice:1p0:extended</ram:ID></ram:GuidelineSpecifiedDocumentContextParameter></rsm:SpecifiedExchangedDocumentContext>
<rsm:HeaderExchangedDocument><ram:ID>1</ram:ID></rsm:HeaderExchangedDocument>
<rsm:SpecifiedSupplyChainTradeTransaction><ram:ApplicableSupplyChainTradeSettlement>` + content + `</ram:ApplicableSupplyChainTradeSettlement></rsm:SpecifiedSupplyChainTradeTransaction>
` + zf1Tail
	}
	cases := []struct {
		name               string
		content            string
		mandate, creditor  string
		terms              int
		paymentMeansExtras int
	}{
		{"no ID", `<ram:SpecifiedTradeSettlementPaymentMeans><ram:TypeCode>58</ram:TypeCode></ram:SpecifiedTradeSettlementPaymentMeans>`,
			"", "", 0, 0},
		{"no creditor identifier", `<ram:SpecifiedTradeSettlementPaymentMeans><ram:TypeCode>59</ram:TypeCode><ram:ID>M-1</ram:ID></ram:SpecifiedTradeSettlementPaymentMeans>`,
			"M-1", "", 1, 0},
		{"first payment means with an ID", `<ram:SpecifiedTradeSettlementPaymentMeans><ram:TypeCode>58</ram:TypeCode></ram:SpecifiedTradeSettlementPaymentMeans>
<ram:SpecifiedTradeSettlementPaymentMeans><ram:TypeCode>59</ram:TypeCode><ram:ID schemeAgencyID="DE98ZZZ09999999999">M-2</ram:ID></ram:SpecifiedTradeSettlementPaymentMeans>
<ram:SpecifiedTradeSettlementPaymentMeans><ram:TypeCode>59</ram:TypeCode><ram:ID>M-3</ram:ID></ram:SpecifiedTradeSettlementPaymentMeans>`,
			"M-2", "DE98ZZZ09999999999", 1, 1},
		{"terms with a due date", `<ram:SpecifiedTradeSettlementPaymentMeans><ram:TypeCode>59</ram:TypeCode><ram:ID schemeAgencyID="DE98ZZZ09999999999">M-4</ram:ID></ram:SpecifiedTradeSettlementPaymentMeans>
<ram:SpecifiedTradePaymentTerms><ram:DueDateDateTime><udt:DateTimeString format="102">20140320</udt:DateTimeString></ram:DueDateDateTime></ram:SpecifiedTradePaymentTerms>`,
			"M-4", "DE98ZZZ09999999999", 1, 0},
		{"mandate already there", `<ram:SpecifiedTradeSettlementPaymentMeans><ram:TypeCode>59</ram:TypeCode><ram:ID>M-5</ram:ID></ram:SpecifiedTradeSettlementPaymentMeans>
<ram:SpecifiedTradePaymentTerms><ram:DirectDebitMandateID>M-6</ram:DirectDebitMandateID></ram:SpecifiedTradePaymentTerms>`,
			"M-6", "", 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			inv := mustParse(t, []byte(settlement(c.content)))
			mandate := ""
			if len(inv.PaymentTerms) > 0 {
				mandate = inv.PaymentTerms[0].DirectDebitMandateID
			}
			if mandate != c.mandate || inv.CreditorReferenceID != c.creditor || len(inv.PaymentTerms) != c.terms {
				t.Errorf("BT-89 = %q, BT-90 = %q, %d payment terms; want %q, %q, %d",
					mandate, inv.CreditorReferenceID, len(inv.PaymentTerms), c.mandate, c.creditor, c.terms)
			}
			extras := 0
			for _, pm := range inv.PaymentMeans {
				extras += len(pm.Extensions)
			}
			if extras != c.paymentMeansExtras {
				t.Errorf("%d IDs left in the payment means, want %d: %q", extras, c.paymentMeansExtras, inv.Unmodeled())
			}
			if strings.Contains(c.content, "DueDateDateTime") && inv.PaymentTerms[0].DueDate != dt("2014-03-20") {
				t.Errorf("BT-9 = %v", inv.PaymentTerms[0].DueDate)
			}
		})
	}
}

// TestZUGFeRD1ReleaseCandidate reads an invoice of the ZUGFeRD 1.0 release
// candidate: rsm:Invoice, elements without namespace, values as text.
func TestZUGFeRD1ReleaseCandidate(t *testing.T) {
	doc := `<rsm:Invoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CBFBUY:5">
<SpecifiedExchangedDocumentContext>
<TestIndicator>true</TestIndicator>
<GuidelineSpecifiedDocumentContextParameter><ID>urn:ferd:invoice:rc:comfort</ID></GuidelineSpecifiedDocumentContextParameter>
</SpecifiedExchangedDocumentContext>
<HeaderExchangedDocument><ID>RC-1</ID><TypeCode>380</TypeCode><IssueDateTime>2014-03-05T00:00:00</IssueDateTime></HeaderExchangedDocument>
<SpecifiedSupplyChainTradeTransaction>
<ApplicableSupplyChainTradeSettlement>
<InvoiceCurrencyCode>EUR</InvoiceCurrencyCode>
<SpecifiedTradeSettlementMonetarySummation><GrandTotalAmount currencyID="EUR">119.00</GrandTotalAmount></SpecifiedTradeSettlementMonetarySummation>
</ApplicableSupplyChainTradeSettlement>
</SpecifiedSupplyChainTradeTransaction>
</rsm:Invoice>`
	inv := mustParse(t, []byte(doc))
	if inv.Profile != invoice.ProfileEN16931 || inv.SpecificationID != "urn:ferd:invoice:rc:comfort" {
		t.Errorf("profile = %v (%q)", inv.Profile, inv.SpecificationID)
	}
	if inv.Number != "RC-1" || inv.IssueDate != dt("2014-03-05") || !inv.TestIndicator ||
		inv.Currency != "EUR" || inv.Totals.GrandTotal.String() != "119.00" {
		t.Errorf("invoice = %+v", inv)
	}
}

// TestZUGFeRD1Issues checks that the values that cannot be read are located
// in the ZUGFeRD 1.0 document.
func TestZUGFeRD1Issues(t *testing.T) {
	doc := zf1Head + `<rsm:SpecifiedExchangedDocumentContext><ram:GuidelineSpecifiedDocumentContextParameter><ram:ID>urn:ferd:CrossIndustryDocument:invoice:1p0:basic</ram:ID></ram:GuidelineSpecifiedDocumentContextParameter></rsm:SpecifiedExchangedDocumentContext>
<rsm:HeaderExchangedDocument><ram:ID>1</ram:ID><ram:TypeCode>380</ram:TypeCode>
<ram:IssueDateTime><udt:DateTimeString format="102">20139102</udt:DateTimeString></ram:IssueDateTime></rsm:HeaderExchangedDocument>
<rsm:SpecifiedSupplyChainTradeTransaction>
<ram:ApplicableSupplyChainTradeAgreement><ram:CustomerOrderReferencedDocument><ram:IssueDateTime>2014-02-30</ram:IssueDateTime><ram:ID>CO</ram:ID></ram:CustomerOrderReferencedDocument></ram:ApplicableSupplyChainTradeAgreement>
<ram:ApplicableSupplyChainTradeSettlement>
<ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
<ram:SpecifiedTradeSettlementMonetarySummation><ram:GrandTotalAmount currencyID="EUR">12,50</ram:GrandTotalAmount></ram:SpecifiedTradeSettlementMonetarySummation>
</ram:ApplicableSupplyChainTradeSettlement>
</rsm:SpecifiedSupplyChainTradeTransaction>
` + zf1Tail
	inv, err := Parse([]byte(doc))
	var ie *IssuesError
	if !errors.As(err, &ie) {
		t.Fatalf("error = %v, want *IssuesError", err)
	}
	if inv == nil || inv.Profile != invoice.ProfileBasic || inv.Number != "1" {
		t.Fatalf("invoice = %+v", inv)
	}
	want := []struct {
		path string
		line int
	}{
		{"/rsm:CrossIndustryDocument/rsm:HeaderExchangedDocument/ram:IssueDateTime/udt:DateTimeString", 5},
		{"/rsm:CrossIndustryDocument/rsm:SpecifiedSupplyChainTradeTransaction/ram:ApplicableSupplyChainTradeAgreement/ram:CustomerOrderReferencedDocument/ram:IssueDateTime", 7},
		{"/rsm:CrossIndustryDocument/rsm:SpecifiedSupplyChainTradeTransaction/ram:ApplicableSupplyChainTradeSettlement/ram:SpecifiedTradeSettlementMonetarySummation/ram:GrandTotalAmount", 10},
	}
	if len(ie.Issues) != len(want) {
		t.Fatalf("issues = %v", ie.Issues)
	}
	for i, is := range ie.Issues {
		if is.Path != want[i].path || is.Line != want[i].line || is.Message == "" {
			t.Errorf("issue %d = %+v, want %s line %d", i, is, want[i].path, want[i].line)
		}
	}
}
