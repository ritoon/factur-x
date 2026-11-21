package nl

import (
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/invoice"
)

const ublBase = "testdata/nlcius.ubl.xml" // NLCIUS UBL invoice without finding

func cbc(name, value string) string { return "<cbc:" + name + ">" + value + "</cbc:" + name + ">" }

// Anchors of the UBL base invoice.
const (
	uSpec         = "<cbc:CustomizationID>" + CIUSID + "</cbc:CustomizationID>"
	uDueDate      = "<cbc:DueDate>2026-12-02</cbc:DueDate>"
	uTypeCode     = "<cbc:InvoiceTypeCode>380</cbc:InvoiceTypeCode>"
	uNote         = "<cbc:Note>Onderhoud en apparatuur, oktober 2026.</cbc:Note>"
	uBuyerRef     = "<cbc:BuyerReference>PO-4512</cbc:BuyerReference>"
	uSupplier     = "<cac:AccountingSupplierParty>"
	uSellerCity   = "<cbc:CityName>Amsterdam</cbc:CityName>"
	uSellerZone   = "<cbc:PostalZone>1012 AB</cbc:PostalZone>"
	uCountry      = "<cbc:IdentificationCode>NL</cbc:IdentificationCode>" // 1: supplier, 2: customer
	uCountryBlock = "<cac:Country>\n          " + uCountry + "\n        </cac:Country>"
	uTaxSchemeEnd = "</cac:PartyTaxScheme>"
	uSellerName   = "<cbc:RegistrationName>Voorbeeld Netwerken B.V.</cbc:RegistrationName>"
	uSellerLegal  = `<cbc:CompanyID schemeID="0106">12345678</cbc:CompanyID>`
	uSellerEntity = "<cac:PartyLegalEntity>\n        " + uSellerName + "\n        " + uSellerLegal + "\n      </cac:PartyLegalEntity>"
	uBuyerStreet  = "<cbc:StreetName>Stationsplein 2</cbc:StreetName>"
	uBuyerCity    = "<cbc:CityName>Utrecht</cbc:CityName>"
	uBuyerZone    = "<cbc:PostalZone>3511 AB</cbc:PostalZone>"
	uBuyerName    = "<cbc:RegistrationName>Proef Detailhandel B.V.</cbc:RegistrationName>"
	uBuyerLegal   = `<cbc:CompanyID schemeID="0106">87654321</cbc:CompanyID>`
	uBuyerEntity  = "<cac:PartyLegalEntity>\n        " + uBuyerName + "\n        " + uBuyerLegal + "\n      </cac:PartyLegalEntity>"
	uDelivery     = "<cac:Delivery>"
	uPMCode       = "<cbc:PaymentMeansCode>30</cbc:PaymentMeansCode>"
	uAccount      = "<cbc:ID>NL91ABNA0417164300</cbc:ID>"
	uPM           = "<cac:PaymentMeans>\n    " + uPMCode + "\n    <cbc:PaymentID>F-2026-0042</cbc:PaymentID>\n    <cac:PayeeFinancialAccount>\n      " + uAccount + "\n    </cac:PayeeFinancialAccount>\n  </cac:PaymentMeans>"
	uChargeReason = "<cbc:AllowanceChargeReason>Vracht</cbc:AllowanceChargeReason>"
	uLineReason   = "<cbc:AllowanceChargeReason>Korting</cbc:AllowanceChargeReason>"
	uPercent      = "<cbc:Percent>21</cbc:Percent>" // 2: VAT breakdown
	uTaxAmount    = `<cbc:TaxAmount currencyID="EUR">210.00</cbc:TaxAmount>`
	uTaxTotalEnd  = "</cac:TaxTotal>"
	uPayable      = `<cbc:PayableAmount currencyID="EUR">1210.00</cbc:PayableAmount>`
	uLine1Amount  = `<cbc:LineExtensionAmount currencyID="EUR">750.00</cbc:LineExtensionAmount>`
)

// Elements added by the UBL cases.
const (
	uOrderRef     = "<cac:OrderReference><cbc:ID>PO-1</cbc:ID></cac:OrderReference>"
	uLineOrderRef = "<cac:OrderLineReference><cbc:LineID>1</cbc:LineID></cac:OrderLineReference>"
	uBranch       = "<cac:FinancialInstitutionBranch><cbc:ID>ABNANL2A</cbc:ID></cac:FinancialInstitutionBranch>"
)

// uBillingRef returns a reference to a preceding invoice; extra is added
// after its identifier.
func uBillingRef(extra string) string {
	return "<cac:BillingReference><cac:InvoiceDocumentReference>" + cbc("ID", "F-2026-0041") + extra + "</cac:InvoiceDocumentReference></cac:BillingReference>"
}

// uTaxRep returns a tax representative in country; extra is added to its
// address, after the postal zone.
func uTaxRep(country, extra string) string {
	return "<cac:TaxRepresentativeParty><cac:PartyName>" + cbc("Name", "Fiscaal B.V.") + "</cac:PartyName><cac:PostalAddress>" +
		cbc("StreetName", "Kade 3") + cbc("CityName", "Rotterdam") + cbc("PostalZone", "3011 AA") + extra +
		"<cac:Country>" + cbc("IdentificationCode", country) + "</cac:Country></cac:PostalAddress></cac:TaxRepresentativeParty>"
}

// uDeliveryAddress returns a delivery location; extra is added to its
// address, after the city.
func uDeliveryAddress(extra string) string {
	return "<cac:DeliveryLocation><cac:Address>" + cbc("StreetName", "Dok 4") + cbc("CityName", "Rotterdam") + extra +
		"<cac:Country>" + cbc("IdentificationCode", "NL") + "</cac:Country></cac:Address></cac:DeliveryLocation>"
}

func uAddressLine(v string) string {
	return "<cac:AddressLine>" + cbc("Line", v) + "</cac:AddressLine>"
}

var (
	uForeignSupplier = one(repN(1, uCountry, cbc("IdentificationCode", "BE")))
	uForeignCustomer = one(repN(2, uCountry, cbc("IdentificationCode", "BE")))
	uNoPayment       = one(remove(uPM))
	// creditNote turns the invoice into a credit note.
	creditNote = one(
		rep(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"`, `<CreditNote xmlns="urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"`),
		rep("</Invoice>", "</CreditNote>"),
		rep(uTypeCode, cbc("CreditNoteTypeCode", "381")),
		repAll("cac:InvoiceLine>", "cac:CreditNoteLine>"),
		repAll("cbc:InvoicedQuantity", "cbc:CreditedQuantity"),
	)
)

func ublCases() []mutationCase {
	return []mutationCase{
		{"base", nil, nil},
		{"credit note", creditNote, nil},

		// Conditions and pattern SI-UBL-VERSION.
		// The G-account rules, see gaccountCases, apply to an invoice that
		// declares the extension: they fail on the base invoice.
		{"G-account extension", one(rep(uSpec, cbc("CustomizationID", GAccountID))), ids("BR-GA-1", "BR-GA-2", "BR-GA-3", "BR-GA-5", "BR-GA-7")},
		{"G-account extension of a credit note", edits(creditNote, one(rep(uSpec, cbc("CustomizationID", GAccountID)))), nil},
		{"NLCIUS followed by other text", one(rep(uSpec, cbc("CustomizationID", CIUSID+"#conformant#urn:example")), remove(uBuyerRef)), ids("BR-NL-2")},
		{"identifier normalized", one(rep(uSpec, cbc("CustomizationID", "\n  "+CIUSID+" ")), remove(uBuyerRef)), ids("BR-NL-2")},
		{"BII2-T10-R001 invoice", one(remove(uSpec), remove(uBuyerRef)), ids("BII2-T10-R001")},
		{"BII2-T10-R001 credit note", edits(creditNote, one(remove(uSpec))), ids("BII2-T10-R001")},
		{"SI-V20-INV-R000 Peppol", one(rep(uSpec, cbc("CustomizationID", invoice.URNPeppol)), remove(uBuyerRef), rep(uTypeCode, cbc("InvoiceTypeCode", "326"))), ids("SI-V20-INV-R000")},
		{"SI-V20-INV-R000 empty: both patterns", one(rep(uSpec, "<cbc:CustomizationID/>")), ids("SI-V20-INV-R000", "SI-UBL-2")},
		{"supplier country normalized and upper-cased", one(repN(1, uCountry, cbc("IdentificationCode", " nl ")), remove(uBuyerRef)), ids("BR-NL-2")},
		{"no supplier country", one(repN(1, uCountryBlock, ""), remove(uBuyerRef)), nil},
		{"foreign supplier: only $si rules", edits(uForeignSupplier, one(remove(uBuyerRef), rep(uTypeCode, cbc("InvoiceTypeCode", "326")), remove(uSellerCity), after(uLine1Amount, uLineOrderRef))), ids("BR-NL-13")},
		{"foreign supplier: empty elements", edits(uForeignSupplier, one(rep(uNote, "<cbc:Note/>"))), ids("SI-UBL-2")},

		{"BR-NL-1 scheme", one(rep(uSellerLegal, `<cbc:CompanyID schemeID="0088">12345678</cbc:CompanyID>`)), ids("BR-NL-1")},
		{"BR-NL-1 OIN", one(rep(uSellerLegal, `<cbc:CompanyID schemeID="0190">00000001234567890000</cbc:CompanyID>`)), nil},
		{"BR-NL-1 no legal entity", one(remove(uSellerEntity)), ids("BR-NL-1")},
		{"BR-NL-1 blank identifier", one(rep(uSellerLegal, `<cbc:CompanyID schemeID="0106"> </cbc:CompanyID>`)), ids("BR-NL-1", "SI-UBL-2")},
		{"BR-NL-2 no reference", one(remove(uBuyerRef)), ids("BR-NL-2")},
		{"BR-NL-2 order reference", one(remove(uBuyerRef), before(uSupplier, uOrderRef)), nil},
		{"BR-NL-2 empty buyer reference", one(rep(uBuyerRef, "<cbc:BuyerReference/>")), ids("SI-UBL-2")},
		{"BR-NL-3 no city", one(remove(uSellerCity)), ids("BR-NL-3")},
		{"BR-NL-3 no postal zone", one(remove(uSellerZone)), ids("BR-NL-3")},
		{"BR-NL-4 no street", one(remove(uBuyerStreet)), ids("BR-NL-4")},
		{"BR-NL-4 foreign customer", edits(uForeignCustomer, one(remove(uBuyerCity))), nil},
		{"BR-NL-4 customer without country", one(repN(2, uCountryBlock, ""), remove(uBuyerCity)), ids("BR-NL-4")},
		{"BR-NL-5 no street", one(before(uDelivery, strings.Replace(uTaxRep("NL", ""), cbc("StreetName", "Kade 3"), "", 1))), ids("BR-NL-5")},
		{"BR-NL-5 complete", one(before(uDelivery, uTaxRep("NL", ""))), nil},
		{"BR-NL-5 foreign representative", one(before(uDelivery, strings.Replace(uTaxRep("DE", ""), cbc("StreetName", "Kade 3"), "", 1))), nil},
		{"BR-NL-7 type code", one(rep(uTypeCode, cbc("InvoiceTypeCode", "326"))), ids("BR-NL-7")},
		{"BR-NL-7 compared as a string", one(rep(uTypeCode, cbc("InvoiceTypeCode", "0380"))), ids("BR-NL-7")},
		{"BR-NL-7 self-billed invoice", one(rep(uTypeCode, cbc("InvoiceTypeCode", "389"))), nil},
		{"BR-NL-8 invoice 381", one(rep(uTypeCode, cbc("InvoiceTypeCode", "381"))), ids("BR-NL-8")},
		{"BR-NL-8 credit note 380", edits(creditNote, one(rep(cbc("CreditNoteTypeCode", "381"), cbc("CreditNoteTypeCode", "380")))), ids("BR-NL-8")},
		{"BR-NL-9 corrective invoice", one(rep(uTypeCode, cbc("InvoiceTypeCode", "384"))), ids("BR-NL-9")},
		{"BR-NL-9 invoice reference", one(rep(uTypeCode, cbc("InvoiceTypeCode", "384")), before(uSupplier, uBillingRef(""))), nil},
		{"BR-NL-9 credit note 384", edits(creditNote, one(rep(cbc("CreditNoteTypeCode", "381"), cbc("CreditNoteTypeCode", "384")))), ids("BR-NL-8", "BR-NL-9")},
		{"BR-NL-10 scheme", one(rep(uBuyerLegal, `<cbc:CompanyID schemeID="0088">87654321</cbc:CompanyID>`)), ids("BR-NL-10")},
		{"BR-NL-10 OIN", one(rep(uBuyerLegal, `<cbc:CompanyID schemeID="0190">00000001234567890000</cbc:CompanyID>`)), nil},
		{"BR-NL-10 no identifier", one(remove(uBuyerLegal)), ids("BR-NL-10")},
		{"BR-NL-10 only with a legal entity", one(remove(uBuyerEntity)), nil},
		{"BR-NL-10 foreign customer, other scheme", edits(uForeignCustomer, one(rep(uBuyerLegal, `<cbc:CompanyID schemeID="0208">0123456749</cbc:CompanyID>`))), nil},
		{"BR-NL-10 foreign customer, no identifier", edits(uForeignCustomer, one(remove(uBuyerLegal))), nil},
		{"BR-NL-10 foreign customer, blank identifier", edits(uForeignCustomer, one(rep(uBuyerLegal, `<cbc:CompanyID schemeID="0208"> </cbc:CompanyID>`))), ids("BR-NL-10", "SI-UBL-2")},
		{"BR-NL-11 no payment means", uNoPayment, ids("BR-NL-11")},
		{"BR-NL-11 nothing payable", edits(uNoPayment, one(rep(uPayable, `<cbc:PayableAmount currencyID="EUR">0.00</cbc:PayableAmount>`))), nil},
		{"BR-NL-11 payable amount not a decimal", one(rep(uPayable, `<cbc:PayableAmount currencyID="EUR">1e3</cbc:PayableAmount>`)), ids("BR-NL-11")},
		{"BR-NL-11 credit note", edits(creditNote, uNoPayment), nil},
		{"BR-NL-11 two payable amounts", one(after(uPayable, uPayable)), ids("BR-NL-11")},
		{"BR-NL-12 code", one(rep(uPMCode, cbc("PaymentMeansCode", "31"))), ids("BR-NL-12")},
		{"BR-NL-12 normalized", one(rep(uPMCode, cbc("PaymentMeansCode", " 58 "))), nil},
		{"BR-NL-12 no code", one(remove(uPMCode)), ids("BR-NL-12")},
		{"BR-NL-12 two codes", one(after(uPMCode, uPMCode)), ids("BR-NL-12", "BR-NL-31")},
		{"BR-NL-31 SEPA credit transfer with a branch", one(rep(uPMCode, cbc("PaymentMeansCode", "58")), after(uAccount, uBranch)), ids("BR-NL-31")},
		{"BR-NL-31 credit transfer with a branch", one(after(uAccount, uBranch)), nil},
		{"BR-NL-13 no order reference", one(after(uLine1Amount, uLineOrderRef)), ids("BR-NL-13")},
		{"BR-NL-13 order reference", one(after(uLine1Amount, uLineOrderRef), before(uSupplier, uOrderRef)), nil},

		// Recommendations.
		{"BR-NL-19 tax currency", one(before(uBuyerRef, cbc("TaxCurrencyCode", "EUR"))), ids("BR-NL-19")},
		{"BR-NL-20 tax point date", one(after(uDueDate, cbc("TaxPointDate", "2026-10-31"))), ids("BR-NL-20")},
		{"BR-NL-20 empty: first rule only", one(after(uDueDate, "<cbc:TaxPointDate/>")), ids("BR-NL-20")},
		{"BR-NL-21 tax point date code", one(before(uSupplier, "<cac:InvoicePeriod>"+cbc("DescriptionCode", "35")+"</cac:InvoicePeriod>")), ids("BR-NL-21")},
		{"BR-NL-24 preceding invoice date", one(before(uSupplier, uBillingRef(cbc("IssueDate", "2026-10-01")))), ids("BR-NL-24")},
		{"BR-NL-25 other tax scheme", one(after(uTaxSchemeEnd, "<cac:PartyTaxScheme>"+cbc("CompanyID", "12345")+"<cac:TaxScheme>"+cbc("ID", "FC")+"</cac:TaxScheme></cac:PartyTaxScheme>")), ids("BR-NL-25")},
		{"BR-NL-25 other tax scheme without identifier", one(after(uTaxSchemeEnd, "<cac:PartyTaxScheme><cac:TaxScheme>"+cbc("ID", "FC")+"</cac:TaxScheme></cac:PartyTaxScheme>")), nil},
		{"BR-NL-26 legal information", one(after(uSellerName, cbc("CompanyLegalForm", "B.V."))), ids("BR-NL-26")},
		{"BR-NL-27-1 supplier line 3", one(after(uSellerZone, uAddressLine("Gebouw B"))), ids("BR-NL-27-1")},
		{"BR-NL-27-2 customer line 3", one(after(uBuyerZone, uAddressLine("Gebouw C"))), ids("BR-NL-27-2")},
		{"BR-NL-27-3 representative line 3", one(before(uDelivery, uTaxRep("NL", uAddressLine("Kantoor 2")))), ids("BR-NL-27-3")},
		{"BR-NL-27-4 delivery line 3", one(after(uDelivery, uDeliveryAddress(uAddressLine("Dock 4")))), ids("BR-NL-27-4")},
		{"BR-NL-28-1 supplier subdivision", one(after(uSellerZone, cbc("CountrySubentity", "Noord-Holland"))), ids("BR-NL-28-1")},
		{"BR-NL-28-2 customer subdivision", one(after(uBuyerZone, cbc("CountrySubentity", "Utrecht"))), ids("BR-NL-28-2")},
		{"BR-NL-28-3 representative subdivision", one(before(uDelivery, uTaxRep("NL", cbc("CountrySubentity", "Zuid-Holland")))), ids("BR-NL-28-3")},
		{"BR-NL-28-4 delivery subdivision", one(after(uDelivery, uDeliveryAddress(cbc("CountrySubentity", "Zuid-Holland")))), ids("BR-NL-28-4")},
		{"BR-NL-29 payment means text", one(rep(uPMCode, `<cbc:PaymentMeansCode name="Overboeking">30</cbc:PaymentMeansCode>`)), ids("BR-NL-29")},
		{"BR-NL-30 account name", one(after(uAccount, cbc("Name", "Voorbeeld Netwerken"))), ids("BR-NL-30")},
		{"BR-NL-32-1 charge reason code", one(before(uChargeReason, cbc("AllowanceChargeReasonCode", "FC"))), ids("BR-NL-32-1")},
		{"BR-NL-32-1 line allowance reason code", one(before(uLineReason, cbc("AllowanceChargeReasonCode", "95"))), ids("BR-NL-32-1")},
		{"BR-NL-33 accounting currency", one(after(uTaxTotalEnd, `<cac:TaxTotal><cbc:TaxAmount currencyID="USD">230.00</cbc:TaxAmount></cac:TaxTotal>`)), ids("BR-NL-33")},
		{"BR-NL-33 no currency", one(repN(1, uTaxAmount, cbc("TaxAmount", "210.00"))), ids("BR-NL-33")},
		{"BR-NL-35 exemption reason code", one(repN(2, uPercent, uPercent+cbc("TaxExemptionReasonCode", "VATEX-EU-79-C"))), ids("BR-NL-35")},
		{"SI-UBL-2", one(rep(uNote, "<cbc:Note>  </cbc:Note>")), ids("SI-UBL-2")},
		{"SI-UBL-2 attributes only", one(before(uSupplier, `<cbc:AccountingCost schemeID="x"/>`)), ids("SI-UBL-2")},
	}
}

func TestUBLMutations(t *testing.T) {
	doc := load(t, ublBase)
	for _, c := range ublCases() {
		t.Run(c.name, func(t *testing.T) {
			r := run(t, []byte(apply(t, doc, c.edits)))
			want := slices.Clone(c.want)
			slices.Sort(want)
			if got := r.Rules(); !slices.Equal(got, want) {
				t.Errorf("rules = %q, want %q\n%s", got, want, r)
			}
			checkUBLFindings(t, r)
		})
	}
}

// checkUBLFindings verifies that every finding is in UBLCatalog, with its
// severity, located in the UBL document and without CII names.
func checkUBLFindings(t *testing.T, r *rules.Report) {
	t.Helper()
	checkFindings(t, r, UBLCatalog)
	for _, f := range r.Findings {
		if !strings.HasPrefix(f.Path, "/Invoice") && !strings.HasPrefix(f.Path, "/CreditNote") {
			t.Errorf("%s: path %q is not in the UBL document", f.Rule, f.Path)
		}
		if strings.Contains(f.Message, "ram:") {
			t.Errorf("%s: CII name in %q", f.Rule, f.Message)
		}
	}
}

// TestUBLCoverage checks that every assertion of UBLCatalog fails in at
// least one UBL or G-account mutation case.
func TestUBLCoverage(t *testing.T) {
	failing := map[string]bool{}
	for _, c := range slices.Concat(ublCases(), gaccountCases()) {
		for _, id := range c.want {
			failing[id] = true
		}
	}
	for _, r := range UBLCatalog {
		if !failing[r.ID] {
			t.Errorf("no UBL mutation case fails %s", r.ID)
		}
		if strings.Contains(r.Description, "ram:") {
			t.Errorf("CII name in the description of %s", r.ID)
		}
	}
}

func TestUBLEmpty(t *testing.T) {
	for _, doc := range []string{
		`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`,
		`<CreditNote xmlns="urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"/>`,
	} {
		r := run(t, []byte(doc))
		checkUBLFindings(t, r)
		if want := ids("BII2-T10-R001", "SI-UBL-2"); !slices.Equal(r.Rules(), want) {
			t.Errorf("%s: rules = %q, want %q", doc, r.Rules(), want)
		}
	}
}
