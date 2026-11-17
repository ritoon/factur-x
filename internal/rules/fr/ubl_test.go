package fr

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/rules/ubl"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// UBL test invoices of testdata/, all valid against every assertion of the
// UBL schematron.
const (
	ublEN16931 = "en16931.ubl.xml"     // EN 16931 invoice, billing frame S1
	ublMV      = "extended-mv.ubl.xml" // multi-vendor invoice, frame S8
	ublBD      = "extended-bd.ubl.xml" // two-way self-billed invoice, frame S9
)

// runUBL parses a UBL invoice and runs Check on it.
func runUBL(t testing.TB, data []byte) *rules.Report {
	t.Helper()
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !ubl.IsUBL(root) {
		t.Fatalf("not a UBL invoice: %s", root.Name)
	}
	r := &rules.Report{}
	Check(&rules.Checker{Doc: ubl.View(root, invoice.ProfileUnknown), Report: r, Source: rules.SourceFR})
	return r
}

// Builders of UBL fragments.
func cbc(name, v string) string { return "<cbc:" + name + ">" + v + "</cbc:" + name + ">" }
func cac(name string, inner ...string) string {
	return "<cac:" + name + ">" + strings.Join(inner, "") + "</cac:" + name + ">"
}
func cbcAttr(name, attr, av, v string) string {
	return "<cbc:" + name + " " + attr + `="` + av + `">` + v + "</cbc:" + name + ">"
}
func uID(scheme, v string) string       { return cbcAttr("ID", "schemeID", scheme, v) }
func uEndpoint(scheme, v string) string { return cbcAttr("EndpointID", "schemeID", scheme, v) }
func uLegal(scheme, v string) string {
	return cac("PartyLegalEntity", cbcAttr("CompanyID", "schemeID", scheme, v))
}
func uNote(code, text string) string { return cbc("Note", "#"+code+"#"+text) }
func amt(name, v string) string      { return cbcAttr(name, "currencyID", "EUR", v) }

// uPartyID returns a cac:PartyIdentification, without scheme when scheme is
// empty.
func uPartyID(scheme, v string) string {
	if scheme == "" {
		return cac("PartyIdentification", cbc("ID", v))
	}
	return cac("PartyIdentification", uID(scheme, v))
}

// morePartyIDs, inserted after the cbc:ID of a cac:PartyIdentification,
// closes it and opens one more cac:PartyIdentification per identifier.
func morePartyIDs(ids ...string) string {
	var b strings.Builder
	for _, id := range ids {
		b.WriteString("</cac:PartyIdentification><cac:PartyIdentification>" + id)
	}
	return b.String()
}

// uInvRef returns a cac:BillingReference to the invoice id, with a date when
// date is not empty, followed by more.
func uInvRef(id, date string, more ...string) string {
	s := cbc("ID", id)
	if date != "" {
		s += cbc("IssueDate", date)
	}
	return cac("BillingReference", cac("InvoiceDocumentReference", s+strings.Join(more, "")))
}

// Anchors of the UBL test invoices (EN 16931 base unless stated).
const (
	uCurrency      = "<cbc:DocumentCurrencyCode>EUR</cbc:DocumentCurrencyCode>" // insert header notes before
	uBAR           = "<cbc:Note>#BAR#B2B</cbc:Note>"
	uID1           = "<cbc:ID>FA-2026-00042</cbc:ID>" // BT-1; in the MV base also the line self references
	uRefID         = "<cbc:ID>FA-2026-00017</cbc:ID>"
	uRefDate       = "<cbc:IssueDate>2026-09-15</cbc:IssueDate>"
	uDueDate       = "<cbc:DueDate>2026-11-14</cbc:DueDate>"
	uProfile       = "<cbc:ProfileID>S1</cbc:ProfileID>"
	uTypeCode      = "<cbc:InvoiceTypeCode>380</cbc:InvoiceTypeCode>"
	uCreditType    = "<cbc:CreditNoteTypeCode>381</cbc:CreditNoteTypeCode>" // after creditNote
	uSellerSIRET   = `<cbc:ID schemeID="0009">12345678900017</cbc:ID>`
	uBuyerSIRET    = `<cbc:ID schemeID="0009">98765432100015</cbc:ID>`
	uPayeeSIRET    = `<cbc:ID schemeID="0009">11122233300018</cbc:ID>`
	uShipToSIRET   = `<cbc:ID schemeID="0009">98765432100023</cbc:ID>`
	uSellerSIREN   = `<cbc:CompanyID schemeID="0002">123456789</cbc:CompanyID>` // MV base: 1 seller, 2 line 1, 3 line 2
	uBuyerSIREN    = `<cbc:CompanyID schemeID="0002">987654321</cbc:CompanyID>` // BD base: 1 buyer, 2 line 3, 3 line 4
	uSellerRegName = "<cbc:RegistrationName>Exemple Informatique SAS</cbc:RegistrationName>"
	uSellerEP      = `<cbc:EndpointID schemeID="0225">123456789_FACTURES</cbc:EndpointID>`
	uBuyerEP       = `<cbc:EndpointID schemeID="0225">987654321_ACHATS</cbc:EndpointID>`
	uPayee         = "<cac:PayeeParty>"
	uPaymentCode   = "<cbc:PaymentMeansCode>58</cbc:PaymentMeansCode>"
	uLine1         = `<cbc:LineExtensionAmount currencyID="EUR">190.00</cbc:LineExtensionAmount>` // insert line 1 elements after
	uLine2         = `<cbc:LineExtensionAmount currencyID="EUR">50.00</cbc:LineExtensionAmount>`  // insert line 2 elements after
	uFirstLine     = "<cac:InvoiceLine>"
	uCatS          = "<cbc:ID>S</cbc:ID>"               // 1 charge, 2 and 3 breakdown, 4 and 5 lines
	uRate20        = "<cbc:Percent>20.00</cbc:Percent>" // 1 charge, 2 breakdown, 3 line 1
	uPropName      = "<cbc:Name>Couleur</cbc:Name>"
	uPropValue     = "<cbc:Value>Vert</cbc:Value>"
	uFR            = "<cbc:IdentificationCode>FR</cbc:IdentificationCode>" // MV base: 1 seller, 2 buyer, 3 line 1
	uGroup         = "<cbc:DocumentStatusCode>GROUP</cbc:DocumentStatusCode>"
	uDetail        = "<cbc:DocumentStatusCode>DETAIL</cbc:DocumentStatusCode>"
	uAFL1          = `<cbc:ID schemeID="AFL">FA-2026-00042</cbc:ID>`                                             // MV base: lines 1 and 2
	uReason1       = "<cbc:TaxExemptionReason>#FA-2026-00042# Exemple Informatique SAS</cbc:TaxExemptionReason>" // MV base: 1 breakdown, 2 line 2
	uSelfBilled    = "<cbc:InvoiceTypeCode>389</cbc:InvoiceTypeCode>"                                            // BD base
)

// creditNote turns a UBL base invoice of type 380 into a credit note of type
// 381.
func creditNote() []edit {
	return edits(
		rep(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"`, `<CreditNote xmlns="urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"`),
		rep("</Invoice>", "</CreditNote>"),
		rep(uTypeCode, uCreditType),
		repAll("cac:InvoiceLine>", "cac:CreditNoteLine>"),
		repAll("<cbc:InvoicedQuantity", "<cbc:CreditedQuantity"),
		repAll("</cbc:InvoicedQuantity>", "</cbc:CreditedQuantity>"),
	)
}

// hide renames the first element name of the document so that the rules do
// not see it.
func hide(name string) []edit {
	return edits(rep("<"+name+">", "<"+name+"X>"), rep("</"+name+">", "</"+name+"X>"))
}

// ublParties are parties that the EN 16931 base lacks, and its payee: wrap
// adds content to the party.
var ublParties = []struct {
	name string
	wrap func(inner string) edit
	id   string // business term of BR-FR-09 and BR-FR-CO-10
	ep   string // business term of BR-FR-23 and BR-FR-25
}{
	{"payee", func(s string) edit { return after(uPayee, s) }, "BT-60", "EXT-FR-FE-29"},
	{"buyer agent", func(s string) edit { return after(uBuyerEP, cac("AgentParty", s)) }, "EXT-FR-FE-06", "EXT-FR-FE-12"},
	{"payer", func(s string) edit { return after(uPaymentCode, cac("PaymentMandate", cac("PayerParty", s))) }, "EXT-FR-FE-46", "EXT-FR-FE-52"},
	{"seller agent", func(s string) edit { return after(uSellerEP, cac("AgentParty", s)) }, "EXT-FR-FE-69", "EXT-FR-FE-75"},
	{"invoicee", func(s string) edit { return after(uBuyerEP, cac("ServiceProviderParty", cac("Party", s))) }, "EXT-FR-FE-92", "EXT-FR-FE-98"},
	{"invoicer", func(s string) edit { return after(uSellerEP, cac("ServiceProviderParty", cac("Party", s))) }, "EXT-FR-FE-115", "EXT-FR-FE-121"},
}

// ublCases returns the table of mutations of the UBL test invoices. Every
// assertion of UBLCatalog fails in at least one case (see TestUBLCoverage).
func ublCases() []mutationCase {
	long := func(prefix string, n int) string { return prefix + strings.Repeat("A", n-len(prefix)) }
	line1 := func(xml string) edit { return after(uLine1, xml) }
	credit := func(e ...edit) []edit { return append(creditNote(), e...) }
	paid := func(e ...edit) []edit {
		return append(edits(
			rep(uProfile, cbc("ProfileID", "S2")),
			rep(amt("PrepaidAmount", "0.00"), amt("PrepaidAmount", "292.75")),
			rep(amt("PayableAmount", "292.75"), amt("PayableAmount", "0.00")),
		), e...)
	}
	au := func(e ...edit) []edit {
		return append(edits(after(uSellerSIRET, morePartyIDs(uID("0231", "123456789")))), e...)
	}
	taxRep := func(vat string) string {
		ts := cac("PartyTaxScheme", vat, cac("TaxScheme", cbc("ID", "VAT")))
		return cac("TaxRepresentativeParty", cac("PartyName", cbc("Name", "Représentant")), ts)
	}
	corrective := func(code string, e ...edit) []edit {
		return append(edits(
			rep(uTypeCode, cbc("InvoiceTypeCode", code)),
			before("<cac:AccountingSupplierParty>", uInvRef("FA-2026-00017", "2026-09-15")),
		), e...)
	}
	cs := []mutationCase{
		// Valid invoices.
		{"valid EN 16931", ublEN16931, nil, nil},
		{"valid multi-vendor S8", ublMV, nil, nil},
		{"valid two-way S9", ublBD, nil, nil},
		{"valid credit note", ublEN16931, creditNote(), nil},

		// BR-FR-01, BR-FR-02: identifiers.
		{"BR-FR-01 BT-1 36 characters", ublEN16931, edits(rep(uID1, cbc("ID", long("FA-2026-", 36)))), ids("BR-FR-01_BT-1-1")},
		{"BR-FR-01 BT-1 35 characters", ublEN16931, edits(rep(uID1, cbc("ID", long("FA-2026-", 35)))), nil},
		{"BR-FR-01 BR-FR-02 BT-1 space", ublEN16931, edits(rep(uID1, cbc("ID", "FA 2026-00042"))), ids("BR-FR-01_BT-1-2", "BR-FR-02_BT-1")},
		{"BR-FR-02 BT-1 every allowed character", ublEN16931, edits(rep(uID1, cbc("ID", "Fa+2026/00_42-x"))), nil},
		{"BR-FR-01 BT-25 36 characters", ublEN16931, edits(rep(uRefID, cbc("ID", long("FA", 36)))), ids("BR-FR-01_BT-25-1")},
		{"BR-FR-01 BR-FR-02 BT-25 dot", ublEN16931, edits(rep(uRefID, cbc("ID", "FA.2026.00017"))), ids("BR-FR-01_BT-25-2", "BR-FR-02_BT-25")},
		{"BR-FR-01 EXT-FR-FE-136 36 characters", ublEN16931, edits(line1(uInvRef(long("FA", 36), ""))), ids("BR-FR-01_EXT-FR-FE-136-1")},
		{"BR-FR-01 BR-FR-02 EXT-FR-FE-136 #", ublEN16931, edits(line1(uInvRef("FA#17", ""))), ids("BR-FR-01_EXT-FR-FE-136-2", "BR-FR-02_EXT-FR-FE-136")},
		{"BR-FR-01 EXT-FR-FE-136 credit note", ublEN16931, credit(line1(uInvRef(long("FA", 36), "2026-09-15"))), ids("BR-FR-01_EXT-FR-FE-136-1")},

		// BR-FR-03: dates.
		{"BR-FR-03 BT-2 30 February", ublEN16931, edits(rep("2026-10-15", "2026-02-30")), ids("BR-FR-03_BT-2")},
		{"BR-FR-03 BT-2 leap day", ublEN16931, edits(rep("2026-10-15", "2024-02-29")), nil},
		{"BR-FR-03 BT-2 leap day 2000", ublEN16931, edits(rep("2026-10-15", "2000-02-29")), nil},
		{"BR-FR-03 BT-2 year 1999", ublEN16931, edits(rep("2026-10-15", "1999-12-31")), ids("BR-FR-03_BT-2")},
		// The due date, compared as a string, is then before the issue date.
		{"BR-FR-03 BT-2 CII format", ublEN16931, edits(rep("2026-10-15", "20261015")), ids("BR-FR-03_BT-2", "BR-FR-CO-07_BT-9")},
		{"BR-FR-03 BT-7", ublEN16931, edits(after(uDueDate, cbc("TaxPointDate", "2026-13-01"))), ids("BR-FR-03_BT-7")},
		{"BR-FR-03 BT-9", ublEN16931, edits(rep("2026-11-14", "2026-11-31")), ids("BR-FR-03_BT-9")},
		{"BR-FR-03 BT-26", ublEN16931, edits(rep("2026-09-15", "2026-09-31")), ids("BR-FR-03_BT-26")},
		{"BR-FR-03 BT-72", ublEN16931, edits(rep("2026-10-12", "2026-10-1")), ids("BR-FR-03_BT-72")},
		{"BR-FR-03 BT-73", ublEN16931, edits(rep(cbc("StartDate", "2026-10-01"), cbc("StartDate", "2026-1-01"))), ids("BR-FR-03_BT-73")},
		{"BR-FR-03 BT-74", ublEN16931, edits(rep(cbc("EndDate", "2026-10-31"), cbc("EndDate", "2026-10-32"))), ids("BR-FR-03_BT-74")},
		{"BR-FR-03 BT-134", ublEN16931, edits(repN(2, cbc("StartDate", "2026-10-01"), cbc("StartDate", "2026/10/01"))), ids("BR-FR-03_BT-134")},
		{"BR-FR-03 BT-134 credit note", ublEN16931, credit(repN(2, cbc("StartDate", "2026-10-01"), cbc("StartDate", "2026/10/01"))), ids("BR-FR-03_BT-134")},
		{"BR-FR-03 BT-135", ublEN16931, edits(repN(2, cbc("EndDate", "2026-10-31"), cbc("EndDate", "2026-11-31"))), ids("BR-FR-03_BT-135")},
		{"BR-FR-03 EXT-FR-FE-138", ublEN16931, edits(line1(uInvRef("FA-2026-00017", "2026-02-29"))), ids("BR-FR-03_EXT-FR-FE-138")},
		{"BR-FR-03 EXT-FR-FE-158", ublEN16931, edits(line1(cac("Delivery", cbc("ActualDeliveryDate", "2026-10-00")))), ids("BR-FR-03_EXT-FR-FE-158")},

		// BR-FR-04: document types.
		{"BR-FR-04 BT-3", ublEN16931, edits(rep(uTypeCode, cbc("InvoiceTypeCode", "325"))), ids("BR-FR-04_BT-3")},
		{"BR-FR-04 BT-3 credit note", ublEN16931, credit(rep(uCreditType, cbc("CreditNoteTypeCode", "999"))), ids("BR-FR-04_BT-3")},
		{"BR-FR-04 EXT-FR-FE-02", ublEN16931, edits(after(uRefDate, cbc("DocumentTypeCode", "999"))), ids("BR-FR-04_EXT-FR-FE-02")},
		{"BR-FR-04 EXT-FR-FE-02 380", ublEN16931, edits(after(uRefDate, cbc("DocumentTypeCode", "380"))), nil},
		{"BR-FR-04 EXT-FR-FE-137", ublEN16931, edits(line1(uInvRef("FA-2026-00017", "2026-09-15", cbc("DocumentTypeCode", "999")))), ids("BR-FR-04_EXT-FR-FE-137")},

		// BR-FR-05, BR-FR-06: notes.
		{"BR-FR-05 BT-22 PMT", ublEN16931, edits(rep("#PMT#", "#ZZZ#")), ids("BR-FR-05_BT-22-1")},
		{"BR-FR-05 BT-22 PMD", ublEN16931, edits(rep("#PMD#", "#ZZZ#")), ids("BR-FR-05_BT-22-2")},
		{"BR-FR-05 BT-22 AAB", ublEN16931, edits(rep("#AAB#", "#ZZZ#")), ids("BR-FR-05_BT-22-3")},
		// The notes are joined with # before the search.
		{"BR-FR-05 BT-22 code without leading #", ublEN16931, edits(rep("#PMT#", "PMT#")), nil},
		{"BR-FR-06 BT-21 PMT twice", ublEN16931, edits(before(uCurrency, uNote("PMT", "Bis"))), ids("BR-FR-06_BT-21-1")},
		{"BR-FR-06 BT-21 PMD twice", ublEN16931, edits(before(uCurrency, uNote("PMD", "Bis"))), ids("BR-FR-06_BT-21-2")},
		{"BR-FR-06 BT-21 AAB twice", ublEN16931, edits(before(uCurrency, uNote("AAB", "Bis"))), ids("BR-FR-06_BT-21-3")},
		{"BR-FR-06 BT-21 TXD twice", ublEN16931, edits(before(uCurrency, uNote("TXD", "A")+uNote("TXD", "B"))), ids("BR-FR-06_BT-21-4")},

		// BR-FR-08: billing frame.
		{"BR-FR-08 BT-23 unknown", ublEN16931, edits(rep(uProfile, cbc("ProfileID", "S10"))), ids("BR-FR-08_BT-23")},
		{"BR-FR-08 BT-23 missing", ublEN16931, edits(remove(uProfile)), ids("BR-FR-08_BT-23")},

		// BR-FR-09: SIRET and SIREN.
		{"BR-FR-09 BT-29", ublEN16931, edits(rep(uSellerSIRET, uID("0009", "11111111100017"))), ids("BR-FR-09_BT-29")},
		{"BR-FR-09 BT-29 without SIREN", ublEN16931, edits(rep(uSellerSIREN, cbcAttr("CompanyID", "schemeID", "0009", "123456789"))), ids("BR-FR-09_BT-29", "BR-FR-10_BT-30")},
		{"BR-FR-09 BT-46", ublEN16931, edits(rep(uBuyerSIRET, uID("0009", "9876543210001"))), ids("BR-FR-09_BT-46")},
		{"BR-FR-09 BT-71", ublEN16931, edits(rep(uShipToSIRET, uID("0009", "9876543210002"))), ids("BR-FR-09_BT-71")},
		{"BR-FR-09 BT-71 SIREN of the delivery party", ublEN16931, edits(after("<cac:DeliveryParty>", uLegal("0002", "111222333"))), ids("BR-FR-09_BT-71")},
		// The SIRET is looked for below the line delivery location itself.
		{"BR-FR-09 EXT-FR-FE-146 schema-invalid", ublEN16931, edits(line1(cac("Delivery", cac("DeliveryLocation", cac("DeliveryLocation", uID("0009", "123")))))), ids("BR-FR-09_EXT-FR-FE-146")},
		{"BR-FR-09 EXT-FR-FE-146 line delivery location", ublEN16931, edits(line1(cac("Delivery", cac("DeliveryLocation", uID("0009", "123"))))), nil},

		// BR-FR-10, BR-FR-11: SIREN of the seller and the buyer.
		{"BR-FR-10 BT-30 8 digits", ublEN16931, edits(rep(uSellerSIREN, cbcAttr("CompanyID", "schemeID", "0002", "12345678"))), ids("BR-FR-10_BT-30", "BR-FR-09_BT-29", "BR-FR-32-LEGALID")},
		{"BR-FR-10 BT-30 legal entity without SIREN", ublEN16931, edits(before(uSellerRegName, "</cac:PartyLegalEntity><cac:PartyLegalEntity>")), ids("BR-FR-10_BT-30")},
		{"BR-FR-10 BT-30 no legal entity", ublEN16931, edits(rep("<cac:PartyLegalEntity>", "<cac:PartyLegalEntityX>"), rep("</cac:PartyLegalEntity>", "</cac:PartyLegalEntityX>")), ids("BR-FR-09_BT-29")},
		{"BR-FR-11 BT-47", ublEN16931, edits(remove(uBuyerSIREN)), ids("BR-FR-11_BT-47", "BR-FR-09_BT-46")},
		{"BR-FR-11 BT-47 B2C", ublEN16931, edits(rep(uBAR, uNote("BAR", "B2C")), remove(uBuyerSIREN)), ids("BR-FR-09_BT-46")},

		// BR-FR-12, BR-FR-13: electronic addresses.
		{"BR-FR-12 BT-49", ublEN16931, edits(remove(uBuyerEP)), ids("BR-FR-12_BT-49", "BR-FR-21_BT-49")},
		{"BR-FR-13 BT-34", ublEN16931, edits(remove(uSellerEP)), ids("BR-FR-13_BT-34")},

		// BR-FR-15, BR-FR-16: VAT categories and rates.
		{"BR-FR-15 BT-95 BT-102 unknown", ublEN16931, edits(rep(uCatS, cbc("ID", "X"))), ids("BR-FR-15_BT-95_BT-102-1")},
		{"BR-FR-15 BT-95 BT-102 L", ublEN16931, edits(rep(uCatS, cbc("ID", "L"))), ids("BR-FR-15_BT-95_BT-102-1", "BR-FR-15_BT-95_BT-102-2")},
		{"BR-FR-15 BT-118 unknown", ublEN16931, edits(repN(2, uCatS, cbc("ID", "X"))), ids("BR-FR-15_BT-118-1")},
		{"BR-FR-15 BT-118 M", ublEN16931, edits(repN(3, uCatS, cbc("ID", "M"))), ids("BR-FR-15_BT-118-1", "BR-FR-15_BT-118-2")},
		{"BR-FR-15 BT-151 unknown", ublEN16931, edits(repN(4, uCatS, cbc("ID", "X"))), ids("BR-FR-15_BT-151-1")},
		{"BR-FR-15 BT-151 L", ublEN16931, edits(repN(5, uCatS, cbc("ID", "L"))), ids("BR-FR-15_BT-151-1", "BR-FR-08_BT-151-2")},
		{"BR-FR-16 BT-96", ublEN16931, edits(rep(uRate20, cbc("Percent", "15.00"))), ids("BR-FR-16_BT-96")},
		{"BR-FR-16 BT-119", ublEN16931, edits(repN(2, uRate20, cbc("Percent", "15.00"))), ids("BR-FR-16_BT-119")},
		{"BR-FR-16 BT-152", ublEN16931, edits(repN(3, uRate20, cbc("Percent", "15.00"))), ids("BR-FR-16_BT-152")},
		{"BR-FR-16 BT-152 19.60", ublEN16931, edits(repN(3, uRate20, cbc("Percent", "19.60"))), nil},
		// The item rule only applies to invoice lines.
		{"BR-FR-16 BT-152 credit note", ublEN16931, credit(repN(3, uRate20, cbc("Percent", "15.00"))), nil},

		// BR-FR-17, BR-FR-18: attachments.
		{"BR-FR-17 BT-123", ublEN16931, edits(rep("BON_LIVRAISON", "BON_DE_LIVRAISON")), ids("BR-FR-17_BT-123")},
		{"BR-FR-18 BT-123 one LISIBLE", ublEN16931, edits(rep("BON_LIVRAISON", "LISIBLE")), nil},
		{"BR-FR-18 BT-123 two LISIBLE", ublEN16931, edits(rep("BON_LIVRAISON", "LISIBLE"), before("<cac:AccountingSupplierParty>", cac("AdditionalDocumentReference", cbc("ID", "PDF-2"), cbc("DocumentDescription", "LISIBLE")))), ids("BR-FR-18_BT-123")},

		// BR-FR-20 to BR-FR-22: treatment BAR.
		{"BR-FR-20 BT-21", ublEN16931, edits(rep(uBAR, uNote("BAR", "B2G"))), ids("BR-FR-20_BT-21")},
		{"BR-FR-21 BT-49 other SIREN", ublEN16931, edits(rep(uBuyerEP, uEndpoint("0225", "111111111_ACHATS"))), ids("BR-FR-21_BT-49")},
		{"BR-FR-21 BT-49 other scheme", ublEN16931, edits(rep(uBuyerEP, uEndpoint("0088", "987654321_ACHATS"))), ids("BR-FR-21_BT-49")},
		{"BR-FR-21 BT-49 B2C", ublEN16931, edits(rep(uBAR, uNote("BAR", "B2C")), rep(uBuyerEP, uEndpoint("0088", "987654321_ACHATS"))), nil},
		{"BR-FR-22 BT-34", ublEN16931, edits(rep(uTypeCode, cbc("InvoiceTypeCode", "389")), rep(uSellerEP, uEndpoint("0225", "111111111_FACTURES"))), ids("BR-FR-22_BT-34")},
		{"BR-FR-22 BT-34 buyer address not checked", ublEN16931, edits(rep(uTypeCode, cbc("InvoiceTypeCode", "389")), rep(uBuyerEP, uEndpoint("0088", "X"))), nil},

		// BR-FR-23 to BR-FR-26: formats and lengths of the seller and buyer
		// addresses and private identifiers (the other parties follow).
		{"BR-FR-23 BT-34", ublEN16931, edits(rep(uSellerEP, uEndpoint("0225", "123456789 FACTURES"))), ids("BR-FR-23_BT-34")},
		{"BR-FR-23 BT-49", ublEN16931, edits(rep(uBuyerEP, uEndpoint("0225", "987654321#ACHATS"))), ids("BR-FR-23_BT-49")},
		{"BR-FR-25 BT-34", ublEN16931, edits(rep(uSellerEP, uEndpoint("0225", long("123456789_", 126)))), ids("BR-FR-25_BT-34")},
		{"BR-FR-25 BT-49", ublEN16931, edits(rep(uBuyerEP, uEndpoint("0225", long("987654321_", 126)))), ids("BR-FR-25_BT-49")},
		{"BR-FR-25 BT-49 125 characters", ublEN16931, edits(rep(uBuyerEP, uEndpoint("0225", long("987654321_", 125)))), nil},
		{"BR-FR-24 BT-29", ublEN16931, edits(after(uSellerSIRET, morePartyIDs(uID("0224", "a b")))), ids("BR-FR-24_BT-29")},
		{"BR-FR-24 BT-46", ublEN16931, edits(after(uBuyerSIRET, morePartyIDs(uID("0224", "x/y")))), ids("BR-FR-24_BT-46")},
		{"BR-FR-26 BT-29", ublEN16931, edits(after(uSellerSIRET, morePartyIDs(uID("0224", long("P", 101))))), ids("BR-FR-26_BT-29")},
		{"BR-FR-26 BT-46", ublEN16931, edits(after(uBuyerSIRET, morePartyIDs(uID("0224", long("P", 101))))), ids("BR-FR-26_BT-46")},
		{"BR-FR-26 BT-46 100 characters", ublEN16931, edits(after(uBuyerSIRET, morePartyIDs(uID("0224", long("P", 100))))), nil},

		// BR-FR-27, BR-FR-28: item attributes. BR-FR-27 targets cac:Item
		// directly under the root.
		{"BR-FR-27 BG-32 schema-invalid", ublEN16931, edits(before(uFirstLine, cac("Item", cac("AdditionalItemProperty", cbc("Value", "x"))))), ids("BR-FR-27_BG-32")},
		{"BR-FR-27 BT-160 schema-invalid", ublEN16931, edits(before(uFirstLine, cac("Item", cac("AdditionalItemProperty", cbc("Name", " "), cbc("Value", "x"))))), ids("BR-FR-27_BT-160")},
		{"BR-FR-27 EXT-FR-FE-159 schema-invalid", ublEN16931, edits(before(uFirstLine, cac("Item", cac("AdditionalItemProperty", cbc("NameCode", ""), cbc("Value", "x"))))), ids("BR-FR-27_EXT-FR-FE-159")},
		{"BR-FR-27 BT-160 line item", ublEN16931, edits(rep(uPropName, cbc("Name", ""))), nil},
		{"BR-FR-28 value and quantity", ublEN16931, edits(after(uPropValue, `<cbc:ValueQuantity unitCode="KGM">2</cbc:ValueQuantity>`)), ids("BR-FR-28-Value")},
		{"BR-FR-28 no value", ublEN16931, edits(remove(uPropValue)), ids("BR-FR-28-Value")},
		{"BR-FR-28 quantity without unit", ublEN16931, edits(rep(uPropValue, cbc("ValueQuantity", "2"))), ids("BR-FR-28-Value")},
		{"BR-FR-28 quantity with unit", ublEN16931, edits(rep(uPropValue, `<cbc:ValueQuantity unitCode="KGM">2</cbc:ValueQuantity>`)), nil},

		// BR-FR-29, BR-FR-30, pattern BR-FR-31: invoiced objects and BAR.
		{"BR-FR-29 BT-18 schema-invalid", ublEN16931, edits(after(uID("AVV", "COMPTEUR-7781"), uID("AVV", "COMPTEUR-7782"))), ids("BR-FR-29_BT-18")},
		{"BR-FR-29 AFL", ublEN16931, edits(rep(cbc("ID", "BL-2026-311"), uID("AFL", ""))), ids("BR-FR-29_AFL")},
		{"BR-FR-29 AVV", ublEN16931, edits(rep("COMPTEUR-7781", " ")), ids("BR-FR-29_AVV")},
		{"BR-FR-30 BT-128 schema-invalid", ublEN16931, edits(line1(cac("DocumentReference", uID("AFL", "A"), uID("AFL", "B"), cbc("DocumentTypeCode", "130")))), ids("BR-FR-30_BT-128")},
		{"BR-FR-30 AFL", ublEN16931, edits(line1(cac("DocumentReference", uID("AFL", ""), cbc("DocumentTypeCode", "130")))), ids("BR-FR-30_AFL")},
		{"BR-FR-30 AVV", ublEN16931, edits(line1(cac("DocumentReference", uID("AVV", " "), cbc("DocumentTypeCode", "130")))), ids("BR-FR-30_AVV")},
		{"BR-FR-30 BT-21 two treatments", ublEN16931, edits(before(uCurrency, uNote("BAR", "B2C"))), ids("BR-FR-30_BT-21")},

		// BR-FR-32: SIREN of the parties.
		{"BR-FR-32 LEGALID", ublEN16931, edits(after(uBuyerEP, cac("AgentParty", uLegal("0002", "12345")))), ids("BR-FR-32-LEGALID")},
		{"BR-FR-32 ID 0002", ublEN16931, edits(after(uBuyerSIRET, morePartyIDs(uID("0002", "12345")))), ids("BR-FR-32-ID")},
		{"BR-FR-32 ID 0231", ublEN16931, edits(after(uBuyerSIRET, morePartyIDs(uID("0231", "1234567890")))), ids("BR-FR-32-ID")},

		// BR-FR-CO-03 to BR-FR-CO-05: references of credit notes and
		// corrective invoices.
		{"BR-FR-CO-03 BT-3", ublEN16931, credit(append(edits(rep(uCreditType, cbc("CreditNoteTypeCode", "262"))), hide("cac:ContractDocumentReference")...)...), ids("BR-FR-CO-03_BT-3")},
		{"BR-FR-CO-03 BT-3 no period", ublEN16931, credit(rep(uCreditType, cbc("CreditNoteTypeCode", "262")), remove(cbc("StartDate", "2026-10-01"))), ids("BR-FR-CO-03_BT-3")},
		{"BR-FR-CO-03 BT-3 valid", ublEN16931, credit(rep(uCreditType, cbc("CreditNoteTypeCode", "262"))), nil},
		{"BR-FR-CO-03 BT-3 invoice", ublEN16931, append(edits(rep(uTypeCode, cbc("InvoiceTypeCode", "262"))), hide("cac:ContractDocumentReference")...), nil},
		{"BR-FR-CO-04 BT-3 no reference", ublEN16931, append(edits(rep(uTypeCode, cbc("InvoiceTypeCode", "384"))), hide("cac:BillingReference")...), ids("BR-FR-CO-04_BT-3")},
		{"BR-FR-CO-04 BT-3 one reference", ublEN16931, edits(rep(uTypeCode, cbc("InvoiceTypeCode", "384"))), nil},
		{"BR-FR-CO-04 BT-3 two references", ublEN16931, edits(rep(uTypeCode, cbc("InvoiceTypeCode", "471")), before("<cac:ContractDocumentReference>", uInvRef("FA-2026-00018", "2026-09-16"))), ids("BR-FR-CO-04_BT-3")},
		{"BR-FR-CO-04 BT-3 credit note", ublEN16931, credit(append(edits(rep(uCreditType, cbc("CreditNoteTypeCode", "384"))), hide("cac:BillingReference")...)...), nil},
		{"BR-FR-CO-05 BT-3 no reference", ublEN16931, credit(hide("cac:BillingReference")...), ids("BR-FR-CO-05_BT-3")},
		{"BR-FR-CO-05 BT-3 undated reference", ublEN16931, credit(remove(uRefDate)), ids("BR-FR-CO-05_BT-3")},
		{"BR-FR-CO-05 BT-3 every line referenced", ublEN16931, credit(append(hide("cac:BillingReference"), after(uLine1, uInvRef("FA-2026-00017", "2026-09-15")), after(uLine2, uInvRef("FA-2026-00017", "2026-09-15")))...), nil},
		{"BR-FR-CO-05 BT-3 one line referenced", ublEN16931, credit(append(hide("cac:BillingReference"), after(uLine1, uInvRef("FA-2026-00017", "2026-09-15")))...), ids("BR-FR-CO-05_BT-3")},
		{"BR-FR-CO-05 BT-3 invoice", ublEN16931, append(edits(rep(uTypeCode, cbc("InvoiceTypeCode", "381"))), hide("cac:BillingReference")...), nil},

		// BR-FR-CO-07 to BR-FR-CO-09: due dates and frames.
		{"BR-FR-CO-07 BT-9", ublEN16931, edits(rep("2026-11-14", "2026-10-14")), ids("BR-FR-CO-07_BT-9")},
		{"BR-FR-CO-07 BT-9 same day", ublEN16931, edits(rep("2026-11-14", "2026-10-15")), nil},
		{"BR-FR-CO-07 BT-9 payment due date", ublEN16931, edits(after(uPaymentCode, cbc("PaymentDueDate", "2026-10-01"))), ids("BR-FR-CO-07_BT-9")},
		{"BR-FR-CO-07 BT-9 advance invoice", ublEN16931, edits(rep("2026-11-14", "2026-10-14"), rep(uTypeCode, cbc("InvoiceTypeCode", "386"))), nil},
		{"BR-FR-CO-08 BT-23", ublEN16931, edits(rep(uProfile, cbc("ProfileID", "S4")), rep(uTypeCode, cbc("InvoiceTypeCode", "386"))), ids("BR-FR-CO-08_BT-23")},
		{"BR-FR-CO-08 BT-23 final invoice", ublEN16931, edits(rep(uProfile, cbc("ProfileID", "S4"))), nil},
		{"BR-FR-CO-09 BT-23 not paid", ublEN16931, edits(rep(uProfile, cbc("ProfileID", "S2"))), ids("BR-FR-CO-09_BT-23-1", "BR-FR-CO-09_BT-23-2")},
		{"BR-FR-CO-09 BT-23 paid", ublEN16931, paid(), nil},
		{"BR-FR-CO-09 BT-23 no due date", ublEN16931, paid(remove(uDueDate)), ids("BR-FR-CO-09_BT-23-3")},
		{"BR-FR-CO-09 BT-23 payment due date", ublEN16931, paid(remove(uDueDate), after(uPaymentCode, cbc("PaymentDueDate", "2026-10-15"))), nil},

		// BR-FR-CO-12: currency (BR-FR-CO-10 follows).
		{"BR-FR-CO-12 BT-5", ublEN16931, edits(rep(uCurrency, cbc("DocumentCurrencyCode", "USD"))), ids("BR-FR-CO-12_BT-5")},
		{"BR-FR-CO-12 BT-5 accounting currency", ublEN16931, edits(rep(uCurrency, cbc("DocumentCurrencyCode", "USD")+cbc("TaxCurrencyCode", "EUR"))), nil},
		{"BR-FR-CO-12 BT-5 no VAT total in EUR", ublEN16931, edits(rep(uCurrency, cbc("DocumentCurrencyCode", "USD")+cbc("TaxCurrencyCode", "EUR")), rep(amt("TaxAmount", "42.75"), cbcAttr("TaxAmount", "currencyID", "USD", "42.75"))), ids("BR-FR-CO-12_BT-5")},

		// BR-FR-CO-14, BR-FR-CO-15: member of a single taxable person.
		{"BR-FR-CO-14 BR-FR-CO-15 BT-29-1", ublEN16931, au(), ids("BR-FR-CO-14_BT-29-1", "BR-FR-CO-15_BT-29-1")},
		{"BR-FR-CO-14 BT-29-1", ublEN16931, au(before("<cac:Delivery>", taxRep(cbc("CompanyID", "FR11111111111")))), ids("BR-FR-CO-14_BT-29-1")},
		{"BR-FR-CO-15 BT-29-1", ublEN16931, au(before(uCurrency, uNote("TXD", "MEMBRE_ASSUJETTI_UNIQUE"))), ids("BR-FR-CO-15_BT-29-1")},
		{"BR-FR-CO-15 BT-29-1 no VAT identifier", ublEN16931, au(before(uCurrency, uNote("TXD", "MEMBRE_ASSUJETTI_UNIQUE")), before("<cac:Delivery>", taxRep(""))), ids("BR-FR-CO-15_BT-29-1")},
		{"BR-FR-CO-14 BR-FR-CO-15 valid", ublEN16931, au(before(uCurrency, uNote("TXD", "MEMBRE_ASSUJETTI_UNIQUE")), before("<cac:Delivery>", taxRep(cbc("CompanyID", "FR11111111111")))), nil},

		// BR-FR-DEC-01 to BR-FR-DEC-04: formats of amounts, quantities and
		// rates (the monetary totals follow).
		{"BR-FR-DEC-01 BT-93 BT-100", ublEN16931, edits(after(amt("Amount", "10.00"), amt("BaseAmount", "10.001"))), ids("BR-FR-DEC-01_BT-93_BT-100")},
		{"BR-FR-DEC-01 BT-92 BT-99", ublEN16931, edits(rep(amt("Amount", "10.00"), amt("Amount", "10.000"))), ids("BR-FR-DEC-01_BT-92_BT-99")},
		{"BR-FR-DEC-01 BT-110", ublEN16931, edits(rep(amt("TaxAmount", "42.75"), amt("TaxAmount", "42.751"))), ids("BR-FR-DEC-01_BT-110")},
		{"BR-FR-DEC-01 BT-114", ublEN16931, edits(after(amt("PrepaidAmount", "0.00"), amt("PayableRoundingAmount", "0.001"))), ids("BR-FR-DEC-01_BT-114")},
		{"BR-FR-DEC-01 BT-116", ublEN16931, edits(rep(amt("TaxableAmount", "200.00"), amt("TaxableAmount", "200.001"))), ids("BR-FR-DEC-01_BT-116")},
		{"BR-FR-DEC-01 BT-117", ublEN16931, edits(rep(amt("TaxAmount", "40.00"), amt("TaxAmount", "40.001"))), ids("BR-FR-DEC-01_BT-117")},
		{"BR-FR-DEC-01 BT-131", ublEN16931, edits(rep(uLine1, amt("LineExtensionAmount", "190.001"))), ids("BR-FR-DEC-01_BT-131")},
		{"BR-FR-DEC-01 BT-136 BT-141", ublEN16931, edits(repN(2, amt("Amount", "10.00"), amt("Amount", "10.001"))), ids("BR-FR-DEC-01_BT-136_BT-141")},
		{"BR-FR-DEC-01 BT-137 BT-142", ublEN16931, edits(rep(amt("BaseAmount", "200.00"), amt("BaseAmount", "200.001"))), ids("BR-FR-DEC-01_BT-137_BT-142")},
		{"BR-FR-DEC-01 BT-106 negative", ublEN16931, edits(rep(amt("LineExtensionAmount", "240.00"), amt("LineExtensionAmount", "-240.00"))), nil},
		{"BR-FR-DEC-01 BT-106 20 digits", ublEN16931, edits(rep(amt("LineExtensionAmount", "240.00"), amt("LineExtensionAmount", "123456789012345678.00"))), ids("BR-FR-DEC-01_BT-106")},
		{"BR-FR-DEC-02 BT-129", ublEN16931, edits(rep(`<cbc:InvoicedQuantity unitCode="H87">4</cbc:InvoicedQuantity>`, `<cbc:InvoicedQuantity unitCode="H87">4.00001</cbc:InvoicedQuantity>`)), ids("BR-FR-DEC-02_BT-129")},
		{"BR-FR-DEC-02 BT-129 credit note", ublEN16931, credit(rep(`<cbc:CreditedQuantity unitCode="H87">4</cbc:CreditedQuantity>`, `<cbc:CreditedQuantity unitCode="H87">4.00001</cbc:CreditedQuantity>`)), ids("BR-FR-DEC-02_BT-129")},
		{"BR-FR-DEC-02 BT-149", ublEN16931, edits(rep(`<cbc:BaseQuantity unitCode="H87">1</cbc:BaseQuantity>`, `<cbc:BaseQuantity unitCode="H87">1.00001</cbc:BaseQuantity>`)), ids("BR-FR-DEC-02_BT-149")},
		{"BR-FR-DEC-03 BT-146 negative", ublEN16931, edits(rep(amt("PriceAmount", "50.00"), amt("PriceAmount", "-50.00"))), ids("BR-FR-DEC-03_BT-146")},
		{"BR-FR-DEC-03 BT-146 7 decimals", ublEN16931, edits(rep(amt("PriceAmount", "25.00"), amt("PriceAmount", "25.0000001"))), ids("BR-FR-DEC-03_BT-146")},
		{"BR-FR-DEC-03 BT-146 6 decimals", ublEN16931, edits(rep(amt("PriceAmount", "25.00"), amt("PriceAmount", "25.000001"))), nil},
		{"BR-FR-DEC-03 BT-146 S9 7 decimals", ublBD, edits(rep(amt("PriceAmount", "-15.00"), amt("PriceAmount", "-15.0000001"))), ids("BR-FR-DEC-03_BT-146")},
		{"BR-FR-DEC-03 BT-147", ublEN16931, edits(rep(amt("Amount", "5.00"), amt("Amount", "-5.00"))), ids("BR-FR-DEC-03_BT-147")},
		{"BR-FR-DEC-03 BT-148", ublEN16931, edits(rep(amt("BaseAmount", "55.00"), amt("BaseAmount", "55.0000001"))), ids("BR-FR-DEC-03_BT-148")},
		{"BR-FR-DEC-04 BT-96 BT-103", ublEN16931, edits(rep(uRate20, cbc("Percent", "-20.00"))), ids("BR-FR-DEC-04_BT-96_BT-103", "BR-FR-16_BT-96")},
		{"BR-FR-DEC-04 BT-119", ublEN16931, edits(repN(2, uRate20, cbc("Percent", "20.001"))), ids("BR-FR-DEC-04_BT-119", "BR-FR-16_BT-119")},
		{"BR-FR-DEC-04 BT-152 credit note", ublEN16931, credit(repN(3, uRate20, cbc("Percent", "20.000"))), ids("BR-FR-DEC-04_BT-152")},

		// BR-FR-MV-*, BR-FR-BD-*: multi-vendor and two-way invoices.
		{"BR-FR-MV-01 EXT-FR-FE-163 empty sub-type", ublMV, edits(rep(uDetail, cbc("DocumentStatusCode", " "))), ids("BR-FR-MV-01_EXT-FR-FE-163")},
		{"BR-FR-MV-01 EXT-FR-FE-163 no self reference", ublMV, edits(repN(3, uID1, cbc("ID", "FA-2026-00041"))), ids("BR-FR-MV-01_EXT-FR-FE-163", "BR-FR-MV-05_EXT-FR-FE-BG-12")},
		{"BR-FR-MV-02 EXT-FR-FE-163", ublMV, edits(repAll(uGroup, cbc("DocumentStatusCode", "GROUPE"))), ids("BR-FR-MV-02_EXT-FR-FE-163")},
		{"BR-FR-BD-02 EXT-FR-FE-163", ublBD, edits(repN(2, uGroup, cbc("DocumentStatusCode", "GROUPE"))), ids("BR-FR-BD-02_EXT-FR-FE-163", "BR-FR-BD-02_BT-47")},
		{"BR-FR-BD-02 BT-30", ublBD, edits(repN(2, uSellerSIREN, cbcAttr("CompanyID", "schemeID", "0002", "555666777")), repN(2, uSellerSIREN, cbcAttr("CompanyID", "schemeID", "0002", "555666777"))), ids("BR-FR-BD-02_BT-30")},
		{"BR-FR-BD-02 BT-47", ublBD, edits(repN(2, uBuyerSIREN, cbcAttr("CompanyID", "schemeID", "0002", "555666777")), repN(2, uBuyerSIREN, cbcAttr("CompanyID", "schemeID", "0002", "555666777"))), ids("BR-FR-BD-02_BT-47")},
		{"BR-FR-MV-03 EXT-FR-FE-164", ublMV, edits(repN(2, uSellerRegName, cbc("RegistrationName", ""))), ids("BR-FR-MV-03_EXT-FR-FE-164")},
		{"BR-FR-MV-03 EXT-FR-FE-167", ublMV, edits(repN(2, uSellerSIREN, cbc("CompanyID", ""))), ids("BR-FR-MV-03_EXT-FR-FE-167", "BR-FR-MV-06_EXT-FR-FE-167")},
		{"BR-FR-MV-03 EXT-FR-FE-177", ublMV, edits(repN(3, uFR, cbc("IdentificationCode", ""))), ids("BR-FR-MV-03_EXT-FR-FE-177")},
		{"BR-FR-MV-03 EXT-FR-FE-181", ublMV, edits(repN(2, amt("TaxAmount", "20.00"), cbcAttr("TaxAmount", "currencyID", "USD", "20.00"))), ids("BR-FR-MV-03_EXT-FR-FE-181", "BR-FR-MV-09_EXT-FR-FE-181")},
		{"BR-FR-MV-03 EXT-FR-FE-182", ublMV, edits(after(uCurrency, cbc("TaxCurrencyCode", "USD"))), ids("BR-FR-MV-03_EXT-FR-FE-182")},
		{"BR-FR-MV-03 EXT-FR-FE-182 EUR", ublMV, edits(after(uCurrency, cbc("TaxCurrencyCode", "EUR"))), nil},
		{"BR-FR-MV-03 BT-128 AFL", ublMV, edits(rep(uAFL1, uID("AFX", "FA-2026-00042"))), ids("BR-FR-MV-03_BT-128_AFL", "BR-FR-MV-07_BT-128", "BR-FR-MV-09_EXT-FR-FE-181", "BR-FR-MV-11_BT-128")},
		{"BR-FR-MV-03 BT-128 AVV frame", ublMV, edits(rep("COMMANDE-A-17", "S8")), ids("BR-FR-MV-03_BT-128_AVV")},
		{"BR-FR-MV-03 BT-128 AVV missing", ublMV, edits(rep(uID("AVV", "COMMANDE-B-18"), uID("AVX", "COMMANDE-B-18"))), ids("BR-FR-MV-03_BT-128_AVV")},
		{"BR-FR-MV-05 EXT-FR-FE-BG-12", ublMV, edits(repN(2, amt("LineExtensionAmount", "100.00"), amt("LineExtensionAmount", "100.02"))), ids("BR-FR-MV-05_EXT-FR-FE-BG-12")},
		{"BR-FR-MV-06 EXT-FR-FE-167", ublMV, edits(repN(3, uSellerSIREN, cbcAttr("CompanyID", "schemeID", "0002", "999999999"))), ids("BR-FR-MV-06_EXT-FR-FE-167")},
		{"BR-FR-MV-07 BT-128", ublMV, edits(repN(2, uAFL1, uID("AFL", "FA-2026-00043"))), ids("BR-FR-MV-07_BT-128", "BR-FR-MV-08_BT-128")},
		{"BR-FR-MV-08 BT-128", ublMV, edits(repN(2, uReason1, cbc("TaxExemptionReason", "FA-2026-00042 Exemple Informatique SAS"))), ids("BR-FR-MV-08_BT-128")},
		{"BR-FR-MV-09 EXT-FR-FE-181", ublMV, edits(rep(amt("TaxAmount", "20.00"), amt("TaxAmount", "20.05"))), ids("BR-FR-MV-09_EXT-FR-FE-181")},
		// cbc:TaxInclusiveLineExtensionAmount is not in the UBL 2.1 schema:
		// the base invoice has none, which the rule accepts.
		{"BR-FR-MV-10 EXT-FR-FE-184", ublMV, edits(after(amt("LineExtensionAmount", "100.00"), amt("TaxInclusiveLineExtensionAmount", "120.05"))), ids("BR-FR-MV-10_EXT-FR-FE-184")},
		{"BR-FR-MV-10 EXT-FR-FE-184 gross total", ublMV, edits(after(amt("LineExtensionAmount", "100.00"), amt("TaxInclusiveLineExtensionAmount", "120.00"))), nil},
		{"BR-FR-MV-11 BT-128", ublMV, edits(repAll(`schemeID="AFL">FA-2026-00042<`, `schemeID="AFL">SF-01<`), repAll("#FA-2026-00042#", "#SF-01#")), ids("BR-FR-MV-11_BT-128")},
		{"BR-FR-MV-12 BT-128", ublMV, edits(repAll("LIB-2026-77", "FA-2026-00042")), ids("BR-FR-MV-12_BT-128", "BR-FR-MV-09_EXT-FR-FE-181")},
		{"BR-FR-MV-13 BT-3", ublMV, edits(rep(uTypeCode, cbc("InvoiceTypeCode", "389"))), ids("BR-FR-MV-13_BT-3")},
		{"BR-FR-MV-BD BT-3", ublBD, edits(rep(uSelfBilled, cbc("InvoiceTypeCode", "380"))), ids("BR-FR-MV-BD_BT-3")},
		{"BR-FR-MV-14", ublMV, corrective("384"), ids("BR-FR-MV-14-EXT-FR-FE-136", "BR-FR-MV-14-EXT-FR-FE-138")},
		{"BR-FR-MV-14 dated line references", ublMV, corrective("384", afterN(4, "</cac:BillingReference>", uInvRef("FA-2026-00017", "2026-09-15")), afterN(2, "</cac:BillingReference>", uInvRef("FA-2026-00017", "2026-09-15"))), nil},
		{"BR-FR-MV-14 EXT-FR-FE-138", ublMV, corrective("384", afterN(4, "</cac:BillingReference>", uInvRef("FA-2026-00017", "")), afterN(2, "</cac:BillingReference>", uInvRef("FA-2026-00017", ""))), ids("BR-FR-MV-14-EXT-FR-FE-138")},
		{"BR-FR-MV-14 credit note", ublMV, creditNote(), ids("BR-FR-MV-14-EXT-FR-FE-136", "BR-FR-MV-14-EXT-FR-FE-138", "BR-FR-CO-05_BT-3")},
		{"BR-FR-BD-14", ublBD, edits(rep(uSelfBilled, cbc("InvoiceTypeCode", "261"))), ids("BR-FR-BD-14-EXT-FR-FE-136", "BR-FR-BD-14-EXT-FR-FE-138")},
	}

	// BR-FR-DEC-01 on the monetary totals.
	for _, a := range []struct{ name, value, bt string }{
		{"LineExtensionAmount", "240.00", "BT-106"},
		{"AllowanceTotalAmount", "0.00", "BT-107"},
		{"ChargeTotalAmount", "10.00", "BT-108"},
		{"TaxExclusiveAmount", "250.00", "BT-109"},
		{"TaxInclusiveAmount", "292.75", "BT-112"},
		{"PrepaidAmount", "0.00", "BT-113"},
		{"PayableAmount", "292.75", "BT-115"},
	} {
		cs = append(cs, mutationCase{"BR-FR-DEC-01 " + a.bt, ublEN16931, edits(rep(amt(a.name, a.value), amt(a.name, a.value+"1"))), ids("BR-FR-DEC-01_" + a.bt)})
	}

	// BR-FR-CO-10 on the seller, the buyer and the delivery location.
	for _, p := range []struct{ bt, anchor string }{{"BT-29", uSellerSIRET}, {"BT-46", uBuyerSIRET}} {
		cs = append(cs,
			mutationCase{"BR-FR-CO-10 " + p.bt + " -1", ublEN16931, edits(after(p.anchor, morePartyIDs(cbc("ID", "X1")))), ids("BR-FR-CO-10_" + p.bt + "-1")},
			mutationCase{"BR-FR-CO-10 " + p.bt + " -2", ublEN16931, edits(after(p.anchor, morePartyIDs(uID("0088", "3012345678901"), uID("0088", "3012345678902")))), ids("BR-FR-CO-10_" + p.bt + "-2")},
		)
	}
	cs = append(cs,
		mutationCase{"BR-FR-CO-10 BT-71 -1", ublEN16931, edits(after(uShipToSIRET, cbc("ID", "X1"))), ids("BR-FR-CO-10_BT-71-1")},
		mutationCase{"BR-FR-CO-10 BT-71 -2", ublEN16931, edits(after(uShipToSIRET, uID("0009", "98765432100031"))), ids("BR-FR-CO-10_BT-71-2")},
		mutationCase{"BR-FR-CO-10 EXT-FR-FE-146 -1", ublEN16931, edits(line1(cac("Delivery", cac("DeliveryLocation", cbc("ID", "X1"))))), ids("BR-FR-CO-10_EXT-FR-FE-146-1")},
		mutationCase{"BR-FR-CO-10 EXT-FR-FE-146 -2", ublEN16931, edits(line1(cac("Delivery", cac("DeliveryLocation", uID("0088", "1"), uID("0088", "2"))))), ids("BR-FR-CO-10_EXT-FR-FE-146-2")},
	)

	// BR-FR-09, BR-FR-CO-10, BR-FR-23 and BR-FR-25 on the other parties.
	for _, p := range ublParties {
		siret := func(v string) edit { return p.wrap(uPartyID("0009", v)) }
		if p.id == "BT-60" { // the payee of the base already has a SIRET
			siret = func(v string) edit { return rep(uPayeeSIRET, uID("0009", v)) }
		}
		cs = append(cs,
			mutationCase{"BR-FR-09 " + p.id + " " + p.name, ublEN16931, edits(siret("123")), ids("BR-FR-09_" + p.id)},
			mutationCase{"BR-FR-09 " + p.id + " " + p.name + " valid", ublEN16931, edits(siret("12345678900017")), nil},
			mutationCase{"BR-FR-CO-10 " + p.id + " " + p.name + " -1", ublEN16931, edits(p.wrap(uPartyID("", "X1"))), ids("BR-FR-CO-10_" + p.id + "-1")},
			mutationCase{"BR-FR-CO-10 " + p.id + " " + p.name + " -2", ublEN16931, edits(p.wrap(uPartyID("0088", "1") + uPartyID("0088", "2"))), ids("BR-FR-CO-10_" + p.id + "-2")},
			mutationCase{"BR-FR-23 " + p.ep + " " + p.name, ublEN16931, edits(p.wrap(uEndpoint("0225", "a b"))), ids("BR-FR-23_" + p.ep)},
			mutationCase{"BR-FR-25 " + p.ep + " " + p.name, ublEN16931, edits(p.wrap(uEndpoint("0088", long("X", 126)))), ids("BR-FR-25_" + p.ep)},
			mutationCase{"BR-FR-25 " + p.ep + " " + p.name + " 125 characters", ublEN16931, edits(p.wrap(uEndpoint("0088", long("X", 125)))), nil},
		)
	}
	return cs
}

func TestUBLMutations(t *testing.T) {
	bases := map[string]string{}
	for _, c := range ublCases() {
		t.Run(c.name, func(t *testing.T) {
			doc, ok := bases[c.base]
			if !ok {
				doc = load(t, c.base)
				bases[c.base] = doc
			}
			r := runUBL(t, []byte(apply(t, doc, c.edits)))
			got := r.Rules()
			want := slices.Clone(c.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("rules = %q, want %q\n%s", got, want, r)
			}
			checkUBLFindings(t, r)
		})
	}
}

// checkUBLFindings verifies the fields of every finding on a UBL invoice:
// the rule is in UBLCatalog, the finding is located in the UBL document and
// its message does not cite CII elements.
func checkUBLFindings(t *testing.T, r *rules.Report) {
	t.Helper()
	for _, f := range r.Findings {
		if !inUBLCatalog(f.Rule) {
			t.Errorf("rule %s is not in UBLCatalog", f.Rule)
		}
		if f.Severity != rules.Error || f.Source != rules.SourceFR || f.Message == "" {
			t.Errorf("bad finding %+v", f)
		}
		if !strings.HasPrefix(f.Path, "/Invoice") && !strings.HasPrefix(f.Path, "/CreditNote") {
			t.Errorf("%s located at %q, outside the UBL document", f.Rule, f.Path)
		}
		if strings.Contains(f.Message, "ram:") || strings.Contains(f.Message, "rsm:") {
			t.Errorf("%s: message cites CII elements: %s", f.Rule, f.Message)
		}
	}
}

func inUBLCatalog(id string) bool {
	for _, r := range UBLCatalog {
		if r.ID == id {
			return true
		}
	}
	return false
}

// TestUBLCoverage checks that every assertion of UBLCatalog fails in at least
// one UBL mutation case.
func TestUBLCoverage(t *testing.T) {
	failing := map[string]bool{}
	for _, c := range ublCases() {
		for _, id := range c.want {
			failing[id] = true
		}
	}
	for _, r := range UBLCatalog {
		if !failing[r.ID] {
			t.Errorf("no UBL mutation case fails %s", r.ID)
		}
	}
}

var ublBaseID = regexp.MustCompile(`^BR-FR-(?:(?:CO|DEC|MV|BD)-)?(?:\d\d|BD_)`)

func TestUBLCatalog(t *testing.T) {
	cii := map[string]string{}
	for _, r := range Catalog {
		cii[r.ID] = r.Description
	}
	seen := map[string]bool{}
	for _, r := range UBLCatalog {
		if seen[r.ID] {
			t.Errorf("duplicate %s", r.ID)
		}
		seen[r.ID] = true
		if r.Severity != rules.Error || r.Description == "" || !ublBaseID.MatchString(r.ID) {
			t.Errorf("bad entry %+v", r)
		}
		if strings.Contains(r.Description, "ram:") {
			t.Errorf("%s: description cites CII elements: %s", r.ID, r.Description)
		}
		if d, ok := cii[r.ID]; ok && d != r.Description {
			t.Errorf("%s: description %q, %q in Catalog", r.ID, r.Description, d)
		}
	}
	// 175 assertions in the UBL schematron, BR-FR-16_BT-103 and
	// BR-FR-DEC-01_BT-111 being unreachable.
	if len(UBLCatalog) != 173 {
		t.Errorf("%d assertions, want 173", len(UBLCatalog))
	}
	for _, id := range []string{"BR-FR-16_BT-103", "BR-FR-DEC-01_BT-111"} {
		if seen[id] {
			t.Errorf("%s is unreachable and must not be in UBLCatalog", id)
		}
	}
	// Identifiers of the CII schematron only.
	for _, id := range []string{"BR-FR-01_BT-1", "BR-FR-05_BT-22_PMT", "BR-FR-31_BT-21", "BR-FR-32-GLOBALID", "BR-FR-CO-04_BT-4", "BR-FR-BD-02_BT-31", "BR-FR-BD-13_BT-3"} {
		if !slices.ContainsFunc(Catalog, func(r rules.RuleInfo) bool { return r.ID == id }) {
			t.Errorf("%s is not in Catalog", id)
		}
		if seen[id] {
			t.Errorf("%s is a CII identifier and must not be in UBLCatalog", id)
		}
	}
}

// TestUBLEmpty checks that Check does not fail on degenerate UBL documents.
func TestUBLEmpty(t *testing.T) {
	for _, doc := range []string{
		`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`,
		`<CreditNote xmlns="urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"/>`,
	} {
		r := runUBL(t, []byte(doc))
		checkUBLFindings(t, r)
		if len(r.Findings) == 0 {
			t.Errorf("no finding on %s", doc)
		}
	}
}

func BenchmarkCheckUBL(b *testing.B) {
	root, err := xmltree.Parse([]byte(load(b, ublEN16931)))
	if err != nil {
		b.Fatal(err)
	}
	doc := ubl.View(root, invoice.ProfileUnknown)
	b.ReportAllocs()
	for b.Loop() {
		r := &rules.Report{}
		Check(&rules.Checker{Doc: doc, Report: r, Source: rules.SourceFR})
		if len(r.Findings) != 0 {
			b.Fatal(r)
		}
	}
}
