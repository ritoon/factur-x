package fr

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/rules/ubl"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// Test invoices of testdata/, all valid against every BR-FR assertion.
const (
	baseEN16931 = "en16931.xml"     // EN 16931 invoice, billing frame S1
	baseMV      = "extended-mv.xml" // EXTENDED-CTC-FR multi-vendor invoice, frame S8
	baseBD      = "extended-bd.xml" // EXTENDED-CTC-FR two-way invoice, frame S9
	testdataDir = "testdata"
)

func load(t testing.TB, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testdataDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// run parses data and runs Check on it.
func run(t testing.TB, data []byte) *rules.Report {
	t.Helper()
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	r := &rules.Report{}
	Check(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileUnknown), Report: r, Source: rules.SourceFR})
	return r
}

// edit replaces one occurrence (nth, from 1) or every occurrence (nth = 0)
// of old by new. The edit fails the test when old is not found.
type edit struct {
	old, new string
	nth      int
}

func rep(old, new string) edit               { return edit{old, new, 1} }
func repN(n int, old, new string) edit       { return edit{old, new, n} }
func repAll(old, new string) edit            { return edit{old, new, 0} }
func before(anchor, xml string) edit         { return edit{anchor, xml + anchor, 1} }
func after(anchor, xml string) edit          { return edit{anchor, anchor + xml, 1} }
func afterN(n int, anchor, xml string) edit  { return edit{anchor, anchor + xml, n} }
func beforeN(n int, anchor, xml string) edit { return edit{anchor, xml + anchor, n} }
func edits(e ...edit) []edit                 { return e }
func ids(id ...string) []string              { return id }
func remove(old string) edit                 { return edit{old, "", 1} }
func typeCode(code string) edit {
	return rep("<ram:TypeCode>380</ram:TypeCode>", "<ram:TypeCode>"+code+"</ram:TypeCode>")
}
func frame(from, to string) edit     { return rep("<ram:ID>"+from+"</ram:ID>", "<ram:ID>"+to+"</ram:ID>") }
func elem(name, value string) string { return "<ram:" + name + ">" + value + "</ram:" + name + ">" }
func party(tag, inner string) string { return "<ram:" + tag + ">" + inner + "</ram:" + tag + ">" }
func note(code, content string) string {
	return party("IncludedNote", elem("Content", content)+elem("SubjectCode", code))
}
func uri(v string) string {
	return party("URIUniversalCommunication", `<ram:URIID schemeID="0225">`+v+`</ram:URIID>`)
}
func legal(scheme, v string) string {
	return party("SpecifiedLegalOrganization", `<ram:ID schemeID="`+scheme+`">`+v+`</ram:ID>`)
}
func globalID(scheme, v string) string {
	return `<ram:GlobalID schemeID="` + scheme + `">` + v + `</ram:GlobalID>`
}
func ard(id, typeCode, refType string) string {
	return party("AdditionalReferencedDocument", elem("IssuerAssignedID", id)+elem("TypeCode", typeCode)+elem("ReferenceTypeCode", refType))
}

// invRef returns an InvoiceReferencedDocument, with a date when date is not
// empty.
func invRef(id, date string) string {
	s := elem("IssuerAssignedID", id)
	if date != "" {
		s += `<ram:FormattedIssueDateTime><qdt:DateTimeString format="102">` + date + `</qdt:DateTimeString></ram:FormattedIssueDateTime>`
	}
	return party("InvoiceReferencedDocument", s)
}

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

// Anchors of the test invoices.
const (
	aNotesEnd      = "</rsm:ExchangedDocument>"
	aAgreement     = "<ram:BuyerOrderReferencedDocument>"         // EN 16931 base: insert header agreement parties before
	aSettlement    = "<ram:SpecifiedTradeSettlementPaymentMeans>" // insert header settlement parties before
	aLineSumEnd    = "</ram:SpecifiedTradeSettlementLineMonetarySummation>"
	aLineDelivery  = `<ram:BilledQuantity unitCode="H87">4</ram:BilledQuantity>`
	aHeaderSumEnd  = "</ram:SpecifiedTradeSettlementHeaderMonetarySummation>"
	aContractEnd   = "</ram:ContractReferencedDocument>"
	aSellerSIRET   = `<ram:GlobalID schemeID="0009">12345678900017</ram:GlobalID>`
	aBuyerSIRET    = `<ram:GlobalID schemeID="0009">98765432100015</ram:GlobalID>`
	aPayeeSIRET    = `<ram:GlobalID schemeID="0009">11122233300018</ram:GlobalID>`
	aShipToSIRET   = `<ram:GlobalID schemeID="0009">98765432100023</ram:GlobalID>`
	aPayeeName     = "<ram:Name>Exemple Affacturage</ram:Name>"
	aSellerSIREN   = `<ram:ID schemeID="0002">123456789</ram:ID>`
	aBuyerSIREN    = `<ram:ID schemeID="0002">987654321</ram:ID>`
	aSellerURI     = `<ram:URIID schemeID="0225">123456789_FACTURES</ram:URIID>`
	aBuyerURI      = `<ram:URIID schemeID="0225">987654321_ACHATS</ram:URIID>`
	aBAR           = "<ram:Content>B2B</ram:Content>"
	aCategoryS     = "<ram:CategoryCode>S</ram:CategoryCode>"
	aRate20        = "<ram:RateApplicablePercent>20.00</ram:RateApplicablePercent>"
	aCharName      = "<ram:Description>Couleur</ram:Description>"
	aCharValue     = "<ram:Value>Vert</ram:Value>"
	aCurrency      = "<ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>"
	aPrepaid       = "<ram:TotalPrepaidAmount>0.00</ram:TotalPrepaidAmount>"
	aDuePayable    = "<ram:DuePayableAmount>292.75</ram:DuePayableAmount>"
	aDueDate       = `<udt:DateTimeString format="102">20261114</udt:DateTimeString>`
	aHeaderRefDate = `<qdt:DateTimeString format="102">20260915</qdt:DateTimeString>`
	aLineDetail    = "<ram:LineStatusReasonCode>DETAIL</ram:LineStatusReasonCode>"
	aLineGroup     = "<ram:LineStatusReasonCode>GROUP</ram:LineStatusReasonCode>"
	aMVGroupGross  = "<ram:GrandTotalAmount>120.00</ram:GrandTotalAmount>"
	aMVGroupNet    = "<ram:LineTotalAmount>100.00</ram:LineTotalAmount>"
	aMVAVV         = "<ram:IssuerAssignedID>COMMANDE-A-17</ram:IssuerAssignedID>"
)

// extraParties are the EXTENDED parties absent from the EN 16931 base: they
// are inserted to reach the EXT-FR-FE-* assertions.
var extraParties = []struct {
	tag, anchor string
	party, uri  string // identifier suffixes for BR-FR-09/BR-FR-CO-10 and BR-FR-23/BR-FR-25
}{
	{"BuyerAgentTradeParty", aAgreement, "EXT-FR-FE-06", "EXT-FR-FE-12"},
	{"PayerTradeParty", aSettlement, "EXT-FR-FE-46", "EXT-FR-FE-52"},
	{"SalesAgentTradeParty", aAgreement, "EXT-FR-FE-69", "EXT-FR-FE-75"},
	{"InvoiceeTradeParty", aSettlement, "EXT-FR-FE-92", "EXT-FR-FE-98"},
	{"InvoicerTradeParty", aSettlement, "EXT-FR-FE-115", "EXT-FR-FE-121"},
}

type mutationCase struct {
	name  string
	base  string
	edits []edit
	want  []string // exact set of rule identifiers reported; empty for a valid invoice
}

// mutationCases returns the table of mutations of the test invoices. Every
// assertion of Catalog fails in at least one case (see TestCoverage).
func mutationCases() []mutationCase {
	long := func(prefix string, n int) string { return prefix + strings.Repeat("A", n-len(prefix)) }
	cs := []mutationCase{
		// Valid invoices.
		{"valid EN 16931", baseEN16931, nil, nil},
		{"valid multi-vendor S8", baseMV, nil, nil},
		{"valid two-way S9", baseBD, nil, nil},

		// BR-FR-01, BR-FR-02: identifiers.
		{"BR-FR-01 BT-1 36 characters", baseEN16931, edits(rep("<ram:ID>FA-2026-00042</ram:ID>", "<ram:ID>"+long("FA-2026-", 36)+"</ram:ID>")), ids("BR-FR-01_BT-1")},
		{"BR-FR-01 BT-1 35 characters", baseEN16931, edits(rep("<ram:ID>FA-2026-00042</ram:ID>", "<ram:ID>"+long("FA-2026-", 35)+"</ram:ID>")), nil},
		{"BR-FR-02 BT-1 space", baseEN16931, edits(rep("<ram:ID>FA-2026-00042</ram:ID>", "<ram:ID>FA 2026-00042</ram:ID>")), ids("BR-FR-02_BT-1")},
		{"BR-FR-02 BT-1 every allowed character", baseEN16931, edits(rep("<ram:ID>FA-2026-00042</ram:ID>", "<ram:ID>Fa+2026/00_42-x</ram:ID>")), nil},
		{"BR-FR-01 BT-25", baseEN16931, edits(rep("<ram:IssuerAssignedID>FA-2026-00017</ram:IssuerAssignedID>", elem("IssuerAssignedID", long("FA", 36)))), ids("BR-FR-01_BT-25")},
		{"BR-FR-02 BT-25 dot", baseEN16931, edits(rep("<ram:IssuerAssignedID>FA-2026-00017</ram:IssuerAssignedID>", elem("IssuerAssignedID", "FA.2026.00017"))), ids("BR-FR-02_BT-25")},
		{"BR-FR-01 EXT-FR-FE-136", baseEN16931, edits(after(aLineSumEnd, invRef(long("FA", 36), ""))), ids("BR-FR-01_EXT-FR-FE-136")},
		{"BR-FR-02 EXT-FR-FE-136", baseEN16931, edits(after(aLineSumEnd, invRef("FA#17", ""))), ids("BR-FR-02_EXT-FR-FE-136")},

		// BR-FR-03: dates.
		{"BR-FR-03 BT-2 30 February", baseEN16931, edits(rep("20261015", "20260230")), ids("BR-FR-03_BT-2")},
		{"BR-FR-03 BT-2 leap day", baseEN16931, edits(rep("20261015", "20240229")), nil},
		{"BR-FR-03 BT-2 leap day 2000", baseEN16931, edits(rep("20261015", "20000229")), nil},
		{"BR-FR-03 BT-7", baseEN16931, edits(after("<ram:CalculatedAmount>40.00</ram:CalculatedAmount>", `<ram:TaxPointDate><udt:DateString format="102">20261301</udt:DateString></ram:TaxPointDate>`)), ids("BR-FR-03_BT-7")},
		{"BR-FR-03 BT-9", baseEN16931, edits(rep("20261114", "20261131")), ids("BR-FR-03_BT-9")},
		{"BR-FR-03 BT-26", baseEN16931, edits(rep("20260915", "20260931")), ids("BR-FR-03_BT-26")},
		{"BR-FR-03 BT-72 7 digits", baseEN16931, edits(rep("20261012", "2026101")), ids("BR-FR-03_BT-72")},
		{"BR-FR-03 BT-73 year 1999", baseEN16931, edits(repN(2, "20261001", "19991001")), ids("BR-FR-03_BT-73")},
		{"BR-FR-03 BT-74 month 99", baseEN16931, edits(repN(2, "20261031", "20269931")), ids("BR-FR-03_BT-74")},
		{"BR-FR-03 EXT-FR-FE-134", baseEN16931, edits(rep("20261001", "20261000")), ids("BR-FR-03_EXT-FR-FE-134")},
		{"BR-FR-03 EXT-FR-FE-135", baseEN16931, edits(rep("20261031", "20261032")), ids("BR-FR-03_EXT-FR-FE-135")},
		{"BR-FR-03 EXT-FR-FE-138", baseEN16931, edits(after(aLineSumEnd, invRef("FA-2026-00017", "20260229"))), ids("BR-FR-03_EXT-FR-FE-138")},
		{"BR-FR-03 EXT-FR-FE-158", baseEN16931, edits(after(aLineDelivery, `<ram:ActualDeliverySupplyChainEvent><ram:OccurrenceDateTime><udt:DateTimeString format="102">21000101</udt:DateTimeString></ram:OccurrenceDateTime></ram:ActualDeliverySupplyChainEvent>`)), ids("BR-FR-03_EXT-FR-FE-158")},

		// BR-FR-04: document types.
		{"BR-FR-04 BT-3", baseEN16931, edits(typeCode("325")), ids("BR-FR-04_BT-3")},
		{"BR-FR-04 EXT-FR-FE-02", baseEN16931, edits(after("<ram:IssuerAssignedID>FA-2026-00017</ram:IssuerAssignedID>", elem("TypeCode", "751"))), ids("BR-FR-04_EXT-FR-FE-02")},
		{"BR-FR-04 EXT-FR-FE-137", baseEN16931, edits(after(aLineSumEnd, party("InvoiceReferencedDocument", elem("IssuerAssignedID", "FA-2026-00017")+elem("TypeCode", "751")))), ids("BR-FR-04_EXT-FR-FE-137")},

		// BR-FR-05, BR-FR-06: notes.
		{"BR-FR-05 PMT", baseEN16931, edits(rep(elem("SubjectCode", "PMT"), elem("SubjectCode", "REG"))), ids("BR-FR-05_BT-22_PMT")},
		{"BR-FR-05 PMD", baseEN16931, edits(rep(elem("SubjectCode", "PMD"), elem("SubjectCode", "REG"))), ids("BR-FR-05_BT-22_PMD")},
		{"BR-FR-05 AAB", baseEN16931, edits(rep(elem("SubjectCode", "AAB"), elem("SubjectCode", "REG"))), ids("BR-FR-05_BT-22_AAB")},
		{"BR-FR-06 PMT", baseEN16931, edits(before(aNotesEnd, note("PMT", "40 euros"))), ids("BR-FR-06_BT-21_PMT")},
		{"BR-FR-06 PMD", baseEN16931, edits(before(aNotesEnd, note("PMD", "Pénalités"))), ids("BR-FR-06_BT-21_PMD")},
		{"BR-FR-06 AAB", baseEN16931, edits(before(aNotesEnd, note("AAB", "Escompte"))), ids("BR-FR-06_BT-21_AAB")},
		{"BR-FR-06 TXD", baseEN16931, edits(before(aNotesEnd, note("TXD", "A")+note("TXD", "B"))), ids("BR-FR-06_BT-21_TXD")},
		{"BR-FR-06 one TXD", baseEN16931, edits(before(aNotesEnd, note("TXD", "A"))), nil},

		// BR-FR-08: billing frame.
		{"BR-FR-08 unknown frame", baseEN16931, edits(frame("S1", "S10")), ids("BR-FR-08_BT-23")},
		{"BR-FR-08 raw value with a space", baseEN16931, edits(frame("S1", "S1 ")), ids("BR-FR-08_BT-23")},
		{"BR-FR-08 missing", baseEN16931, edits(
			repAll("BusinessProcessSpecifiedDocumentContextParameter>", "BusinessProcessSpecifiedDocumentContextParameterX>"),
		), ids("BR-FR-08_BT-23")},

		// BR-FR-09: SIRET and SIREN.
		{"BR-FR-09 BT-29 SIRET of another SIREN", baseEN16931, edits(rep(aSellerSIRET, globalID("0009", "12345678800017"))), ids("BR-FR-09_BT-29")},
		{"BR-FR-09 BT-29 seller without SIREN", baseEN16931, edits(rep(aSellerSIREN, `<ram:ID schemeID="0003">123456789</ram:ID>`)), ids("BR-FR-09_BT-29", "BR-FR-10_BT-30")},
		{"BR-FR-09 BT-46 13 digits", baseEN16931, edits(rep(aBuyerSIRET, globalID("0009", "9876543210001"))), ids("BR-FR-09_BT-46")},
		{"BR-FR-09 BT-60 letter", baseEN16931, edits(rep(aPayeeSIRET, globalID("0009", "1112223330001A"))), ids("BR-FR-09_BT-60")},
		{"BR-FR-09 BT-60 SIREN mismatch", baseEN16931, edits(after(aPayeeName, legal("0002", "999999999"))), ids("BR-FR-09_BT-60")},
		{"BR-FR-09 BT-71 15 digits", baseEN16931, edits(rep(aShipToSIRET, globalID("0009", "987654321000231"))), ids("BR-FR-09_BT-71")},
		{"BR-FR-09 EXT-FR-FE-146", baseEN16931, edits(after(aLineDelivery, party("ShipToTradeParty", globalID("0009", "1234")))), ids("BR-FR-09_EXT-FR-FE-146")},
		{"BR-FR-09 line ship-to without SIREN", baseEN16931, edits(after(aLineDelivery, party("ShipToTradeParty", globalID("0009", "98765432100031")))), nil},

		// BR-FR-10, BR-FR-11: seller and buyer SIREN.
		{"BR-FR-10 missing", baseEN16931, edits(rep(aSellerSIREN, `<ram:ID schemeID="0003">123456789</ram:ID>`), remove(aSellerSIRET)), ids("BR-FR-10_BT-30")},
		{"BR-FR-10 letter", baseEN16931, edits(rep(aSellerSIREN, `<ram:ID schemeID="0002">12345678A</ram:ID>`)), ids("BR-FR-09_BT-29", "BR-FR-10_BT-30", "BR-FR-32-LEGALID")},
		{"BR-FR-11 missing with B2B", baseEN16931, edits(rep(aBuyerSIREN, `<ram:ID schemeID="0003">987654321</ram:ID>`), remove(aBuyerSIRET)), ids("BR-FR-11_BT-47")},
		{"BR-FR-11 missing with B2C", baseEN16931, edits(rep(aBuyerSIREN, `<ram:ID schemeID="0003">987654321</ram:ID>`), remove(aBuyerSIRET), rep(aBAR, elem("Content", "B2C"))), nil},

		// BR-FR-12, BR-FR-13: electronic addresses.
		{"BR-FR-12 missing", baseEN16931, edits(remove(aBuyerURI)), ids("BR-FR-12_BT-49", "BR-FR-21_BT-49")},
		{"BR-FR-12 missing with B2C", baseEN16931, edits(remove(aBuyerURI), rep(aBAR, elem("Content", "B2C"))), ids("BR-FR-12_BT-49")},
		{"BR-FR-13 missing", baseEN16931, edits(remove(aSellerURI)), ids("BR-FR-13_BT-34")},

		// BR-FR-15: VAT categories.
		{"BR-FR-15 BT-102", baseEN16931, edits(repN(5, aCategoryS, elem("CategoryCode", "X"))), ids("BR-FR-15_BT-95_BT-102")},
		{"BR-FR-15 BT-102 L", baseEN16931, edits(repN(5, aCategoryS, elem("CategoryCode", "L"))), ids("BR-FR-15_BT-95_BT-102", "BR-FR-15_BT-95_BT-102_LM")},
		{"BR-FR-15 BT-118", baseEN16931, edits(repN(3, aCategoryS, elem("CategoryCode", "Y"))), ids("BR-FR-15_BT-95_BT-118")},
		{"BR-FR-15 BT-118 M", baseEN16931, edits(repN(4, aCategoryS, elem("CategoryCode", "M"))), ids("BR-FR-15_BT-95_BT-118", "BR-FR-15_BT-95_BT-118_LM")},
		{"BR-FR-15 BT-151 missing", baseEN16931, edits(repN(1, aCategoryS, "")), ids("BR-FR-15_BT-95_BT-151")},
		{"BR-FR-15 BT-151 L", baseEN16931, edits(repN(2, aCategoryS, elem("CategoryCode", "L"))), ids("BR-FR-15_BT-95_BT-151", "BR-FR-15_BT-95_BT-151_LM")},

		// BR-FR-16, BR-FR-DEC-04: VAT rates.
		{"BR-FR-16 BT-96/BT-103", baseEN16931, edits(repN(3, aRate20, elem("RateApplicablePercent", "21"))), ids("BR-FR-16_BT-96_BT-103")},
		{"BR-FR-16 BT-119", baseEN16931, edits(repN(2, aRate20, elem("RateApplicablePercent", "19"))), ids("BR-FR-16_BT-119")},
		{"BR-FR-16 BT-152", baseEN16931, edits(repN(1, aRate20, elem("RateApplicablePercent", "19.5"))), ids("BR-FR-16_BT-152")},
		{"BR-FR-16 BT-152 raw value with a space", baseEN16931, edits(repN(1, aRate20, elem("RateApplicablePercent", " 20.00"))), ids("BR-FR-16_BT-152")},
		{"BR-FR-16 BT-152 19.6", baseEN16931, edits(repN(1, aRate20, elem("RateApplicablePercent", "19.6"))), nil},
		{"BR-FR-DEC-04 BT-96/BT-103", baseEN16931, edits(repN(3, aRate20, elem("RateApplicablePercent", "20.000"))), ids("BR-FR-16_BT-96_BT-103", "BR-FR-DEC-04_BT-96_BT-103")},
		{"BR-FR-DEC-04 BT-119", baseEN16931, edits(repN(2, aRate20, elem("RateApplicablePercent", "20.000"))), ids("BR-FR-16_BT-119", "BR-FR-DEC-04_BT-119")},
		{"BR-FR-DEC-04 BT-152 negative", baseEN16931, edits(repN(1, aRate20, elem("RateApplicablePercent", "-20"))), ids("BR-FR-16_BT-152", "BR-FR-DEC-04_BT-152")},

		// BR-FR-17, BR-FR-18: attachments.
		{"BR-FR-17 unknown code", baseEN16931, edits(rep(elem("Name", "BON_LIVRAISON"), elem("Name", "BL"))), ids("BR-FR-17_BT-123")},
		{"BR-FR-18 two LISIBLE", baseEN16931, edits(rep(elem("Name", "BON_LIVRAISON"), elem("Name", "LISIBLE")), after(elem("ReferenceTypeCode", "AVV"), elem("Name", "LISIBLE"))), ids("BR-FR-18_BT-123")},

		// BR-FR-20, BR-FR-21, BR-FR-22, BR-FR-31: BAR treatment.
		{"BR-FR-20 unknown treatment", baseEN16931, edits(rep(aBAR, elem("Content", "B2G"))), ids("BR-FR-20_BT-21")},
		{"BR-FR-21 address without SIREN prefix", baseEN16931, edits(rep(aBuyerURI, `<ram:URIID schemeID="0225">ACHATS_987654321</ram:URIID>`)), ids("BR-FR-21_BT-49")},
		{"BR-FR-21 scheme 0088", baseEN16931, edits(rep(aBuyerURI, `<ram:URIID schemeID="0088">987654321_ACHATS</ram:URIID>`)), ids("BR-FR-21_BT-49")},
		{"BR-FR-22 self-billed", baseEN16931, edits(typeCode("389")), nil},
		{"BR-FR-22 self-billed address without SIREN", baseEN16931, edits(typeCode("389"), rep(aSellerURI, `<ram:URIID schemeID="0225">FACTURES</ram:URIID>`)), ids("BR-FR-22_BT-34")},
		{"BR-FR-31 two treatments", baseEN16931, edits(before(aNotesEnd, note("BAR", "B2C"))), ids("BR-FR-31_BT-21")},

		// BR-FR-23 to BR-FR-26: formats and lengths of addresses and private identifiers.
		{"BR-FR-23 BT-34", baseEN16931, edits(rep(aSellerURI, `<ram:URIID schemeID="0225">123456789@FACTURES</ram:URIID>`)), ids("BR-FR-23_BT-34")},
		{"BR-FR-23 BT-49", baseEN16931, edits(rep(aBuyerURI, `<ram:URIID schemeID="0225">987654321 ACHATS</ram:URIID>`)), ids("BR-FR-23_BT-49")},
		{"BR-FR-23 EXT-FR-FE-29", baseEN16931, edits(after(aPayeeName, uri("affacturage@exemple.fr"))), ids("BR-FR-23_EXT-FR-FE-29")},
		{"BR-FR-25 BT-34", baseEN16931, edits(rep(aSellerURI, `<ram:URIID schemeID="0225">`+long("123456789_", 126)+`</ram:URIID>`)), ids("BR-FR-25_BT-34")},
		{"BR-FR-25 BT-34 125 characters", baseEN16931, edits(rep(aSellerURI, `<ram:URIID schemeID="0225">`+long("123456789_", 125)+`</ram:URIID>`)), nil},
		{"BR-FR-25 BT-49", baseEN16931, edits(rep(aBuyerURI, `<ram:URIID schemeID="0225">`+long("987654321_", 126)+`</ram:URIID>`)), ids("BR-FR-25_BT-49")},
		{"BR-FR-25 EXT-FR-FE-29", baseEN16931, edits(after(aPayeeName, uri(long("", 126)))), ids("BR-FR-25_EXT-FR-FE-29")},
		{"BR-FR-24 BT-29", baseEN16931, edits(after(aSellerSIRET, globalID("0224", "ABC#1"))), ids("BR-FR-24_BT-29")},
		{"BR-FR-24 BT-46", baseEN16931, edits(after(aBuyerSIRET, globalID("0224", "ABC 1"))), ids("BR-FR-24_BT-46")},
		{"BR-FR-26 BT-29", baseEN16931, edits(after(aSellerSIRET, globalID("0224", long("", 101)))), ids("BR-FR-26_BT-29")},
		{"BR-FR-26 BT-29 100 characters", baseEN16931, edits(after(aSellerSIRET, globalID("0224", long("", 100)))), nil},
		{"BR-FR-26 BT-46", baseEN16931, edits(after(aBuyerSIRET, globalID("0224", long("", 101)))), ids("BR-FR-26_BT-46")},

		// BR-FR-27, BR-FR-28: item attributes.
		{"BR-FR-27 BG-32 no name nor code", baseEN16931, edits(remove(aCharName)), ids("BR-FR-27_BG-32")},
		{"BR-FR-27 BT-160 blank", baseEN16931, edits(rep(aCharName, elem("Description", " "))), ids("BR-FR-27_BT-160")},
		{"BR-FR-27 EXT-FR-FE-159 blank", baseEN16931, edits(after(aCharName, elem("TypeCode", "  "))), ids("BR-FR-27_EXT-FR-FE-159")},
		{"BR-FR-27 code instead of name", baseEN16931, edits(rep(aCharName, elem("TypeCode", "COL"))), nil},
		{"BR-FR-28 value and measure", baseEN16931, edits(after(aCharValue, `<ram:ValueMeasure unitCode="KGM">3</ram:ValueMeasure>`)), ids("BR-FR-28-Value")},
		{"BR-FR-28 no value", baseEN16931, edits(remove(aCharValue)), ids("BR-FR-28-Value")},
		{"BR-FR-28 measure without unit", baseEN16931, edits(rep(aCharValue, `<ram:ValueMeasure>3</ram:ValueMeasure>`)), ids("BR-FR-28-Value")},
		{"BR-FR-28 measure with unit", baseEN16931, edits(rep(aCharValue, `<ram:ValueMeasure unitCode="KGM">3</ram:ValueMeasure>`)), nil},

		// BR-FR-29, BR-FR-30: invoiced objects.
		{"BR-FR-29 BT-18 two AVV", baseEN16931, edits(after(aContractEnd, ard("COMPTEUR-7782", "130", "AVV"))), ids("BR-FR-29_BT-18")},
		{"BR-FR-29 AFL blank", baseEN16931, edits(after(aContractEnd, ard(" ", "130", "AFL"))), ids("BR-FR-29_AFL")},
		{"BR-FR-29 AVV empty", baseEN16931, edits(rep(elem("IssuerAssignedID", "COMPTEUR-7781"), elem("IssuerAssignedID", ""))), ids("BR-FR-29_AVV")},
		{"BR-FR-30 BT-128 two AFL", baseEN16931, edits(after("</ram:SpecifiedLineTradeSettlement>", ard("A1", "130", "AFL")+ard("A2", "130", "AFL"))), ids("BR-FR-30_BT-128")},
		{"BR-FR-30 AFL empty", baseEN16931, edits(after("</ram:SpecifiedLineTradeSettlement>", ard("", "130", "AFL"))), ids("BR-FR-30_AFL")},
		{"BR-FR-30 AVV empty", baseEN16931, edits(after("</ram:SpecifiedLineTradeSettlement>", ard("", "130", "AVV"))), ids("BR-FR-30_AVV")},

		// BR-FR-32: legal and global identifiers of 9 digits.
		{"BR-FR-32 LEGALID", baseEN16931, edits(before(aAgreement, party("SalesAgentTradeParty", elem("Name", "Agent")+legal("0002", "12345")))), ids("BR-FR-32-LEGALID")},
		{"BR-FR-32 GLOBALID 0231", baseEN16931, edits(after(aShipToSIRET, globalID("0231", "12345678"))), ids("BR-FR-32-GLOBALID")},
		{"BR-FR-32 GLOBALID 0002", baseEN16931, edits(after(aPayeeSIRET, globalID("0002", "1112223334"))), ids("BR-FR-32-GLOBALID")},

		// BR-FR-CO-03 to BR-FR-CO-09: document type and billing frame coherence.
		{"BR-FR-CO-03 262 complete", baseEN16931, edits(typeCode("262")), nil},
		{"BR-FR-CO-03 262 without contract", baseEN16931, edits(typeCode("262"), rep(elem("IssuerAssignedID", "CONTRAT-2026-07"), elem("IssuerAssignedID", ""))), ids("BR-FR-CO-03_BT-3")},
		{"BR-FR-CO-03 262 without period end", baseEN16931, edits(typeCode("262"), repN(2, `<udt:DateTimeString format="102">20261031</udt:DateTimeString>`, "")), ids("BR-FR-CO-03_BT-3")},
		{"BR-FR-CO-04 384 one reference", baseEN16931, edits(typeCode("384")), nil},
		{"BR-FR-CO-04 384 two references", baseEN16931, edits(typeCode("384"), after("</ram:InvoiceReferencedDocument>", invRef("FA-2026-00018", ""))), ids("BR-FR-CO-04_BT-4")},
		{"BR-FR-CO-05 381 header reference", baseEN16931, edits(typeCode("381")), nil},
		{"BR-FR-CO-05 381 undated reference", baseEN16931, edits(typeCode("381"), remove(aHeaderRefDate)), ids("BR-FR-CO-05_BT-3")},
		{"BR-FR-CO-05 381 line references", baseEN16931, edits(typeCode("381"), remove(aHeaderRefDate),
			afterN(1, aLineSumEnd, invRef("FA-2026-00017", "20260915")), afterN(2, aLineSumEnd, invRef("FA-2026-00016", "20260815"))), nil},
		{"BR-FR-CO-05 381 one line reference", baseEN16931, edits(typeCode("381"), remove(aHeaderRefDate),
			afterN(1, aLineSumEnd, invRef("FA-2026-00017", "20260915"))), ids("BR-FR-CO-05_BT-3")},
		{"BR-FR-CO-07 due date before issue date", baseEN16931, edits(rep("20261114", "20261014")), ids("BR-FR-CO-07_BT-9")},
		{"BR-FR-CO-07 advance invoice", baseEN16931, edits(rep("20261114", "20261014"), typeCode("386")), nil},
		{"BR-FR-CO-07 paid frame", baseEN16931, edits(rep("20261114", "20261014"), frame("S1", "S2"),
			rep(aPrepaid, elem("TotalPrepaidAmount", "292.75")), rep(aDuePayable, elem("DuePayableAmount", "0.00"))), nil},
		{"BR-FR-CO-08 final invoice after advance", baseEN16931, edits(frame("S1", "S4"), typeCode("386")), ids("BR-FR-CO-08_BT-23")},
		{"BR-FR-CO-08 S4", baseEN16931, edits(frame("S1", "S4")), nil},
		{"BR-FR-CO-09 S2 not paid", baseEN16931, edits(frame("S1", "S2")), ids("BR-FR-CO-09_BT-23-1", "BR-FR-CO-09_BT-23-2")},
		{"BR-FR-CO-09 S2 paid", baseEN16931, edits(frame("S1", "S2"), rep(aPrepaid, elem("TotalPrepaidAmount", "292.75")), rep(aDuePayable, elem("DuePayableAmount", "0.00"))), nil},
		{"BR-FR-CO-09 S2 without due date", baseEN16931, edits(frame("S1", "S2"), rep(aPrepaid, elem("TotalPrepaidAmount", "292.75")),
			rep(aDuePayable, elem("DuePayableAmount", "0.00")), remove(aDueDate)), ids("BR-FR-CO-09_BT-23-3")},

		// BR-FR-CO-12: accounting currency.
		{"BR-FR-CO-12 USD without accounting currency", baseEN16931, edits(rep(aCurrency, elem("InvoiceCurrencyCode", "USD"))), ids("BR-FR-CO-12_BT-5")},
		{"BR-FR-CO-12 USD with EUR", baseEN16931, edits(rep(aCurrency, elem("InvoiceCurrencyCode", "USD")+elem("TaxCurrencyCode", "EUR"))), nil},
		{"BR-FR-CO-12 USD without VAT total in EUR", baseEN16931, edits(rep(aCurrency, elem("InvoiceCurrencyCode", "USD")+elem("TaxCurrencyCode", "EUR")),
			rep(`<ram:TaxTotalAmount currencyID="EUR">`, `<ram:TaxTotalAmount currencyID="USD">`)), ids("BR-FR-CO-12_BT-5")},

		// BR-FR-CO-14, BR-FR-CO-15: member of a single taxable person.
		{"BR-FR-CO-14/15 single taxable person", baseEN16931, edits(after(aSellerSIRET, globalID("0231", "123456789"))), ids("BR-FR-CO-14_BT-29-1", "BR-FR-CO-15_BT-29-1")},
		{"BR-FR-CO-15 with note", baseEN16931, edits(after(aSellerSIRET, globalID("0231", "123456789")), before(aNotesEnd, note("TXD", "MEMBRE_ASSUJETTI_UNIQUE"))), ids("BR-FR-CO-15_BT-29-1")},
		{"BR-FR-CO-14 with representative", baseEN16931, edits(after(aSellerSIRET, globalID("0231", "123456789")),
			before(aAgreement, party("SellerTaxRepresentativeTradeParty", elem("Name", "AU")+party("SpecifiedTaxRegistration", `<ram:ID schemeID="VA">FR11999999999</ram:ID>`)))), ids("BR-FR-CO-14_BT-29-1")},
		{"BR-FR-CO-15 representative without VAT identifier", baseEN16931, edits(after(aSellerSIRET, globalID("0231", "123456789")), before(aNotesEnd, note("TXD", "MEMBRE_ASSUJETTI_UNIQUE")),
			before(aAgreement, party("SellerTaxRepresentativeTradeParty", elem("Name", "AU")+party("SpecifiedTaxRegistration", `<ram:ID schemeID="FC">99999</ram:ID>`)))), ids("BR-FR-CO-15_BT-29-1")},
		{"BR-FR-CO-14/15 complete", baseEN16931, edits(after(aSellerSIRET, globalID("0231", "123456789")), before(aNotesEnd, note("TXD", "MEMBRE_ASSUJETTI_UNIQUE")),
			before(aAgreement, party("SellerTaxRepresentativeTradeParty", elem("Name", "AU")+party("SpecifiedTaxRegistration", `<ram:ID schemeID="VA">FR11999999999</ram:ID>`)))), nil},

		// BR-FR-DEC-01 to BR-FR-DEC-03: amounts, quantities and prices.
		{"BR-FR-DEC-01 BT-92/BT-99", baseEN16931, edits(repN(2, elem("ActualAmount", "10.00"), elem("ActualAmount", "10.001"))), ids("BR-FR-DEC-01_BT-92_BT-99")},
		{"BR-FR-DEC-01 BT-93/BT-100", baseEN16931, edits(afterN(2, elem("ActualAmount", "10.00"), elem("BasisAmount", "1,00"))), ids("BR-FR-DEC-01_BT-93_BT-100")},
		{"BR-FR-DEC-01 BT-106", baseEN16931, edits(rep(elem("LineTotalAmount", "240.00"), elem("LineTotalAmount", "240,00"))), ids("BR-FR-DEC-01_BT-106")},
		{"BR-FR-DEC-01 BT-106 Unicode digits", baseEN16931, edits(rep(elem("LineTotalAmount", "240.00"), elem("LineTotalAmount", "２４０.００"))), nil},
		{"BR-FR-DEC-01 BT-107", baseEN16931, edits(rep(elem("AllowanceTotalAmount", "0.00"), elem("AllowanceTotalAmount", "+0.00"))), ids("BR-FR-DEC-01_BT-107")},
		{"BR-FR-DEC-01 BT-108", baseEN16931, edits(rep(elem("ChargeTotalAmount", "10.00"), elem("ChargeTotalAmount", "1e1"))), ids("BR-FR-DEC-01_BT-108")},
		{"BR-FR-DEC-01 BT-109", baseEN16931, edits(rep(elem("TaxBasisTotalAmount", "250.00"), elem("TaxBasisTotalAmount", "250."))), ids("BR-FR-DEC-01_BT-109")},
		{"BR-FR-DEC-01 BT-110", baseEN16931, edits(rep(">42.75<", ">42.755<")), ids("BR-FR-DEC-01_BT-110")},
		{"BR-FR-DEC-01 BT-112 20 digits", baseEN16931, edits(rep(elem("GrandTotalAmount", "292.75"), elem("GrandTotalAmount", "12345678901234567890"))), ids("BR-FR-DEC-01_BT-112")},
		{"BR-FR-DEC-01 BT-112 19 characters", baseEN16931, edits(rep(elem("GrandTotalAmount", "292.75"), elem("GrandTotalAmount", "-12345678901234567.8"))), nil},
		{"BR-FR-DEC-01 BT-113", baseEN16931, edits(rep(aPrepaid, elem("TotalPrepaidAmount", "-"))), ids("BR-FR-DEC-01_BT-113")},
		{"BR-FR-DEC-01 BT-114", baseEN16931, edits(after(aPrepaid, elem("RoundingAmount", "0.001"))), ids("BR-FR-DEC-01_BT-114")},
		{"BR-FR-DEC-01 BT-114 negative", baseEN16931, edits(after(aPrepaid, elem("RoundingAmount", "-0.5"))), nil},
		{"BR-FR-DEC-01 BT-115", baseEN16931, edits(rep(aDuePayable, elem("DuePayableAmount", "292.75 EUR"))), ids("BR-FR-DEC-01_BT-115")},
		{"BR-FR-DEC-01 BT-116", baseEN16931, edits(repN(2, elem("BasisAmount", "200.00"), elem("BasisAmount", "200.0.0"))), ids("BR-FR-DEC-01_BT-116")},
		{"BR-FR-DEC-01 BT-117", baseEN16931, edits(rep(elem("CalculatedAmount", "40.00"), elem("CalculatedAmount", "40.000"))), ids("BR-FR-DEC-01_BT-117")},
		{"BR-FR-DEC-01 BT-131", baseEN16931, edits(rep(elem("LineTotalAmount", "190.00"), elem("LineTotalAmount", "190.123"))), ids("BR-FR-DEC-01_BT-131")},
		{"BR-FR-DEC-01 BT-136/BT-141", baseEN16931, edits(repN(1, elem("ActualAmount", "10.00"), elem("ActualAmount", "1O.00"))), ids("BR-FR-DEC-01_BT-136_BT-141")},
		{"BR-FR-DEC-01 BT-137/BT-142", baseEN16931, edits(repN(1, elem("BasisAmount", "200.00"), elem("BasisAmount", "200.001"))), ids("BR-FR-DEC-01_BT-137_BT-142")},
		{"BR-FR-DEC-02 BT-129", baseEN16931, edits(rep(aLineDelivery, `<ram:BilledQuantity unitCode="H87">4.12345</ram:BilledQuantity>`)), ids("BR-FR-DEC-02_BT-129")},
		{"BR-FR-DEC-02 BT-149", baseEN16931, edits(rep(`<ram:BasisQuantity unitCode="H87">1</ram:BasisQuantity>`, `<ram:BasisQuantity unitCode="H87">1.00000</ram:BasisQuantity>`)), ids("BR-FR-DEC-02_BT-149")},
		{"BR-FR-DEC-03 BT-146 negative", baseEN16931, edits(rep(elem("ChargeAmount", "50.00"), elem("ChargeAmount", "-50.00"))), ids("BR-FR-DEC-03_BT-146")},
		{"BR-FR-DEC-03 BT-146 6 decimals", baseEN16931, edits(rep(elem("ChargeAmount", "50.00"), elem("ChargeAmount", "50.123456"))), nil},
		{"BR-FR-DEC-03 BT-147", baseEN16931, edits(rep(elem("ActualAmount", "5.00"), elem("ActualAmount", "5.1234567"))), ids("BR-FR-DEC-03_BT-147")},
		{"BR-FR-DEC-03 BT-148", baseEN16931, edits(rep(elem("ChargeAmount", "55.00"), elem("ChargeAmount", "55,00"))), ids("BR-FR-DEC-03_BT-148")},

		// BR-FR-MV-*, BR-FR-BD-*: multi-vendor and two-way invoices.
		{"S8 on an EN 16931 invoice", baseEN16931, edits(frame("S1", "S8")), ids("BR-FR-MV-02_EXT-FR-FE-163", "BR-FR-MV-06_EXT-FR-FE-167", "BR-FR-MV-07_BT-128")},
		{"BR-FR-MV-01 empty sub-type", baseMV, edits(rep(aLineDetail, elem("LineStatusReasonCode", ""))), ids("BR-FR-MV-01_EXT-FR-FE-163")},
		{"BR-FR-MV-02 no GROUP line", baseMV, edits(repAll(aLineGroup, elem("LineStatusReasonCode", "INFORMATION"))), ids("BR-FR-MV-02_EXT-FR-FE-163")},
		{"BR-FR-BD-02 one GROUP line", baseBD, edits(repN(2, aLineGroup, elem("LineStatusReasonCode", "INFORMATION"))), ids("BR-FR-BD-02_BT-47", "BR-FR-BD-02_EXT-FR-FE-163")},
		{"BR-FR-BD-02 no seller sub-invoice", baseBD, edits(rep(aSellerSIREN, `<ram:ID schemeID="0002">111222333</ram:ID>`), rep(aSellerSIREN, `<ram:ID schemeID="0002">111222333</ram:ID>`)), ids("BR-FR-BD-02_BT-31")},
		{"BR-FR-MV-03 EXT-FR-FE-164", baseMV, edits(rep(elem("Name", "Exemple Informatique SAS"), elem("Name", " "))), ids("BR-FR-MV-03_EXT-FR-FE-164")},
		{"BR-FR-MV-03 EXT-FR-FE-167", baseMV, edits(rep(aSellerSIREN, `<ram:ID schemeID="0002"></ram:ID>`)), ids("BR-FR-32-LEGALID", "BR-FR-MV-03_EXT-FR-FE-167", "BR-FR-MV-06_EXT-FR-FE-167")},
		{"BR-FR-MV-03 EXT-FR-FE-177", baseMV, edits(rep(elem("CountryID", "FR"), elem("CountryID", ""))), ids("BR-FR-MV-03_EXT-FR-FE-177")},
		{"BR-FR-MV-03 EXT-FR-FE-181", baseMV, edits(rep(`<ram:TaxTotalAmount currencyID="EUR">20.00</ram:TaxTotalAmount>`, `<ram:TaxTotalAmount currencyID="USD">20.00</ram:TaxTotalAmount>`)),
			ids("BR-FR-MV-03_EXT-FR-FE-181", "BR-FR-MV-09_EXT-FR-FE-181", "BR-FR-MV-10_EXT-FR-FE-184")},
		{"BR-FR-MV-03 EXT-FR-FE-182", baseMV, edits(after(aCurrency, elem("TaxCurrencyCode", "USD"))), ids("BR-FR-MV-03_EXT-FR-FE-182")},
		{"BR-FR-MV-03 EXT-FR-FE-182 accounting currency EUR", baseMV, edits(after(aCurrency, elem("TaxCurrencyCode", "EUR"))), nil},
		{"BR-FR-MV-03 BT-128 AFL type", baseMV, edits(beforeN(1, "<ram:AdditionalReferencedDocument>", ard("X-1", "916", "AFL"))), ids("BR-FR-MV-03_BT-128_AFL")},
		{"BR-FR-MV-03 BT-128 AVV S8", baseMV, edits(rep(aMVAVV, elem("IssuerAssignedID", "S8"))), ids("BR-FR-MV-03_BT-128_AVV")},
		{"BR-FR-MV-03 BT-128 AVV type", baseMV, edits(repN(2, elem("TypeCode", "130"), elem("TypeCode", "916"))), ids("BR-FR-MV-03_BT-128_AVV")},
		{"BR-FR-MV-05 net total", baseMV, edits(rep(aMVGroupNet, elem("LineTotalAmount", "100.02")), rep(aMVGroupGross, elem("GrandTotalAmount", "120.02"))), ids("BR-FR-MV-05_EXT-FR-FE-BG-12")},
		// 100.01 - 100 is 0.010000000000005116 in double precision, as in
		// the reference: the 0.01 boundary fails.
		{"BR-FR-MV-05 double precision boundary", baseMV, edits(rep(aMVGroupNet, elem("LineTotalAmount", "100.01")), rep(aMVGroupGross, elem("GrandTotalAmount", "120.01"))), ids("BR-FR-MV-05_EXT-FR-FE-BG-12")},
		{"BR-FR-MV-05 INFORMATION child", baseMV, edits(rep(aLineDetail, elem("LineStatusReasonCode", "INFORMATION"))), ids("BR-FR-MV-05_EXT-FR-FE-BG-12")},
		{"BR-FR-MV-06 legal identifier of the parent", baseMV, edits(repN(2, aSellerSIREN, `<ram:ID schemeID="0002">123456780</ram:ID>`)), ids("BR-FR-MV-06_EXT-FR-FE-167")},
		{"BR-FR-MV-07 number of the parent", baseMV, edits(repN(2, elem("IssuerAssignedID", "FA-2026-00042"), elem("IssuerAssignedID", "FA-2026-00043")),
			rep("<ram:ExemptionReason>#FA-2026-00042#", "<ram:ExemptionReason>#FA-2026-00043#")), ids("BR-FR-MV-07_BT-128")},
		{"BR-FR-MV-08 exemption reason", baseMV, edits(rep("<ram:ExemptionReason>#FA-2026-00042#", "<ram:ExemptionReason>FA-2026-00042")), ids("BR-FR-MV-08_BT-128")},
		{"BR-FR-MV-09 VAT total", baseMV, edits(rep(elem("CalculatedAmount", "20.00"), elem("CalculatedAmount", "20.02"))), ids("BR-FR-MV-09_EXT-FR-FE-181")},
		{"BR-FR-MV-10 gross total", baseMV, edits(rep(aMVGroupGross, elem("GrandTotalAmount", "120.02"))), ids("BR-FR-MV-10_EXT-FR-FE-184")},
		{"BR-FR-MV-10 gross total within tolerance", baseMV, edits(rep(aMVGroupGross, elem("GrandTotalAmount", "120.01"))), nil},
		{"BR-FR-MV-11 no sub-invoice numbered BT-1", baseMV, edits(rep("<ram:ID>FA-2026-00042</ram:ID>", "<ram:ID>FA-2026-00099</ram:ID>")), ids("BR-FR-MV-11_BT-128")},
		{"BR-FR-MV-11 two sub-invoices numbered BT-1", baseMV, edits(repAll("555666777", "123456789"), repAll("LIB-2026-77", "FA-2026-00042")),
			ids("BR-FR-MV-09_EXT-FR-FE-181", "BR-FR-MV-11_BT-128", "BR-FR-MV-12_BT-128")},
		// The reference compares a sub-invoice number with every preceding
		// ram:IssuerAssignedID, here the AVV object of the first group.
		{"BR-FR-MV-12 number already used", baseMV, edits(rep(aMVAVV, elem("IssuerAssignedID", "LIB-2026-77"))), ids("BR-FR-MV-12_BT-128")},
		{"BR-FR-MV-13 self-billed", baseMV, edits(typeCode("389")), ids("BR-FR-MV-13_BT-3")},
		{"BR-FR-BD-13 not self-billed", baseBD, edits(rep("<ram:TypeCode>389</ram:TypeCode>", "<ram:TypeCode>380</ram:TypeCode>")), ids("BR-FR-BD-13_BT-3")},
		{"BR-FR-MV-14 credit note", baseMV, edits(typeCode("381"), after(aHeaderSumEnd, invRef("FA-2026-00017", "20260915"))), ids("BR-FR-MV-14-EXT-FR-FE-136", "BR-FR-MV-14-EXT-FR-FE-138")},
		{"BR-FR-MV-14 credit note undated line references", baseMV, edits(typeCode("381"), after(aHeaderSumEnd, invRef("FA-2026-00017", "20260915")),
			afterN(1, aLineSumEnd, invRef("FA-2026-00017", "")), afterN(3, aLineSumEnd, invRef("LIB-2026-12", ""))), ids("BR-FR-MV-14-EXT-FR-FE-138")},
		{"BR-FR-MV-14 credit note complete", baseMV, edits(typeCode("381"), after(aHeaderSumEnd, invRef("FA-2026-00017", "20260915")),
			afterN(1, aLineSumEnd, invRef("FA-2026-00017", "20260915")), afterN(3, aLineSumEnd, invRef("LIB-2026-12", "20260901"))), nil},
		{"BR-FR-BD-14 credit note", baseBD, edits(rep("<ram:TypeCode>389</ram:TypeCode>", "<ram:TypeCode>261</ram:TypeCode>"), after(aHeaderSumEnd, invRef("AF-2026-0001", "20260915"))),
			ids("BR-FR-BD-14-EXT-FR-FE-136", "BR-FR-BD-14-EXT-FR-FE-138")},
	}

	// BR-FR-09, BR-FR-23, BR-FR-25 and BR-FR-CO-10 on the EXTENDED parties.
	for _, p := range extraParties {
		cs = append(cs,
			mutationCase{"BR-FR-09 " + p.party, baseEN16931, edits(before(p.anchor, party(p.tag, globalID("0009", "1234567890001")+elem("Name", "Tiers")))), ids("BR-FR-09_" + p.party)},
			mutationCase{"BR-FR-09 " + p.party + " valid", baseEN16931, edits(before(p.anchor, party(p.tag, globalID("0009", "55566677700012")+elem("Name", "Tiers")))), nil},
			mutationCase{"BR-FR-23 " + p.uri, baseEN16931, edits(before(p.anchor, party(p.tag, elem("Name", "Tiers")+uri("tiers@exemple.fr")))), ids("BR-FR-23_" + p.uri)},
			mutationCase{"BR-FR-25 " + p.uri, baseEN16931, edits(before(p.anchor, party(p.tag, elem("Name", "Tiers")+uri(long("", 126))))), ids("BR-FR-25_" + p.uri)},
			mutationCase{"BR-FR-CO-10 " + p.party + " -1", baseEN16931, edits(before(p.anchor, party(p.tag, "<ram:GlobalID>X1</ram:GlobalID>"+elem("Name", "Tiers")))), ids("BR-FR-CO-10_" + p.party + "-1")},
			mutationCase{"BR-FR-CO-10 " + p.party + " -2", baseEN16931, edits(before(p.anchor, party(p.tag, globalID("0088", "3012345678901")+globalID("0088", "3012345678902")+elem("Name", "Tiers")))), ids("BR-FR-CO-10_" + p.party + "-2")},
		)
	}
	// BR-FR-CO-10 on the parties of the EN 16931 base.
	for _, p := range []struct{ bt, anchor string }{
		{"BT-29", aSellerSIRET}, {"BT-46", aBuyerSIRET}, {"BT-60", aPayeeSIRET}, {"BT-71", aShipToSIRET},
	} {
		cs = append(cs,
			mutationCase{"BR-FR-CO-10 " + p.bt + " -1", baseEN16931, edits(after(p.anchor, "<ram:GlobalID>X1</ram:GlobalID>")), ids("BR-FR-CO-10_" + p.bt + "-1")},
			mutationCase{"BR-FR-CO-10 " + p.bt + " -2", baseEN16931, edits(after(p.anchor, globalID("0088", "3012345678901")+globalID("0088", "3012345678902"))), ids("BR-FR-CO-10_" + p.bt + "-2")},
		)
	}
	cs = append(cs,
		mutationCase{"BR-FR-CO-10 EXT-FR-FE-146 -1", baseEN16931, edits(after(aLineDelivery, party("ShipToTradeParty", "<ram:GlobalID>X1</ram:GlobalID>"))), ids("BR-FR-CO-10_EXT-FR-FE-146-1")},
		mutationCase{"BR-FR-CO-10 EXT-FR-FE-146 -2", baseEN16931, edits(after(aLineDelivery, party("ShipToTradeParty", globalID("0088", "1")+globalID("0088", "2")))), ids("BR-FR-CO-10_EXT-FR-FE-146-2")},
	)
	return cs
}

func TestMutations(t *testing.T) {
	bases := map[string]string{}
	for _, c := range mutationCases() {
		t.Run(c.name, func(t *testing.T) {
			doc, ok := bases[c.base]
			if !ok {
				doc = load(t, c.base)
				bases[c.base] = doc
			}
			r := run(t, []byte(apply(t, doc, c.edits)))
			got := r.Rules()
			want := slices.Clone(c.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("rules = %q, want %q\n%s", got, want, r)
			}
			checkFindings(t, r)
		})
	}
}

// checkFindings verifies the fields of every finding.
func checkFindings(t *testing.T, r *rules.Report) {
	t.Helper()
	for _, f := range r.Findings {
		if !inCatalog(f.Rule) {
			t.Errorf("rule %s is not in Catalog", f.Rule)
		}
		if f.Severity != rules.Error || f.Source != rules.SourceFR || f.Message == "" || f.Path == "" {
			t.Errorf("bad finding %+v", f)
		}
	}
}

func inCatalog(id string) bool {
	for _, r := range Catalog {
		if r.ID == id {
			return true
		}
	}
	return false
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

var baseID = regexp.MustCompile(`^BR-FR-(?:(?:CO|DEC|MV|BD)-)?\d\d`)

func TestCatalog(t *testing.T) {
	seen := map[string]bool{}
	bases := map[string]bool{}
	for _, r := range Catalog {
		if seen[r.ID] {
			t.Errorf("duplicate %s", r.ID)
		}
		seen[r.ID] = true
		if r.Severity != rules.Error || r.Description == "" {
			t.Errorf("bad entry %+v", r)
		}
		b := baseID.FindString(r.ID)
		if b == "" {
			t.Errorf("bad identifier %s", r.ID)
		}
		bases[b] = true
	}
	// 171 assertions in the schematron, BR-FR-DEC-01_BT-111 being
	// unreachable.
	if len(Catalog) != 170 {
		t.Errorf("%d assertions, want 170", len(Catalog))
	}
	if len(bases) != 59 {
		t.Errorf("%d base identifiers, want 59", len(bases))
	}
	if seen["BR-FR-DEC-01_BT-111"] {
		t.Error("BR-FR-DEC-01_BT-111 is unreachable and must not be in Catalog")
	}
}

// TestEmpty checks that Check does not fail on degenerate documents.
func TestEmpty(t *testing.T) {
	Check(nil)
	Check(&rules.Checker{})
	for _, doc := range []string{
		`<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"/>`,
		`<x/>`,
	} {
		r := run(t, []byte(doc))
		checkFindings(t, r)
	}
}

// TestCorpus runs Check on the CII and UBL files of the corpus (FX_CORPUS_XML
// and FX_CORPUS): no panic, and every reported rule is in Catalog (CII) or
// UBLCatalog (UBL).
func TestCorpus(t *testing.T) {
	var dirs []string
	for _, env := range []string{"FX_CORPUS_XML", "FX_CORPUS"} {
		if d := os.Getenv(env); d != "" {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		t.Skip("FX_CORPUS_XML and FX_CORPUS are not set")
	}
	files, ublFiles, valid := 0, 0, 0
	counts := map[string]int{}
	for _, dir := range dirs {
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
			var doc *rules.Doc
			switch {
			case root.Name == "CrossIndustryInvoice":
				doc = rules.NewDoc(root, invoice.ProfileUnknown)
				files++
			case ubl.IsUBL(root):
				doc = ubl.View(root, invoice.ProfileUnknown)
				ublFiles++
			default:
				return nil
			}
			r := &rules.Report{}
			Check(&rules.Checker{Doc: doc, Report: r, Source: rules.SourceFR})
			if doc.UBL != nil {
				checkUBLFindings(t, r)
			} else {
				checkFindings(t, r)
			}
			if len(r.Findings) == 0 {
				valid++
			}
			for _, id := range r.Rules() {
				counts[id]++
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d CII files, %d UBL files, %d without BR-FR finding, rules reported: %v", files, ublFiles, valid, counts)
}

func BenchmarkCheck(b *testing.B) {
	root, err := xmltree.Parse([]byte(load(b, baseEN16931)))
	if err != nil {
		b.Fatal(err)
	}
	doc := rules.NewDoc(root, invoice.ProfileUnknown)
	b.ReportAllocs()
	for b.Loop() {
		r := &rules.Report{}
		Check(&rules.Checker{Doc: doc, Report: r, Source: rules.SourceFR})
		if len(r.Findings) != 0 {
			b.Fatal(r)
		}
	}
}

func BenchmarkCheckMultiVendor(b *testing.B) {
	root, err := xmltree.Parse([]byte(load(b, baseMV)))
	if err != nil {
		b.Fatal(err)
	}
	doc := rules.NewDoc(root, invoice.ProfileUnknown)
	b.ReportAllocs()
	for b.Loop() {
		r := &rules.Report{}
		Check(&rules.Checker{Doc: doc, Report: r, Source: rules.SourceFR})
		if len(r.Findings) != 0 {
			b.Fatal(r)
		}
	}
}
