// Package de implements the German rules of XRechnung 3.0, as published by
// KoSIT in the XRechnung schematrons: for the CII syntax, the patterns
// cii-pattern, cii-extension-pattern and cii-cvd-pattern of
// XRechnung-CII-validation.sch; for the UBL syntax, the patterns
// ubl-pattern, ubl-extension-pattern and ubl-cvd-pattern of
// XRechnung-UBL-validation.sch; both with the variables of common.sch.
//
// The package reimplements the 52 assertions of the CII schematron in Go
// (Catalog), with their exact identifiers and flags:
//
//   - 37 assertions of the CIUS (BR-DE-*, BR-DE-TMP-32 and BR-TMP-*), applied
//     to every invoice, base profile and extension alike;
//   - 7 assertions of the extension (BR-DEX-*), applied when the
//     specification identifier BT-24 is ExtensionID;
//   - 8 assertions of the commercial vehicle data (BR-DE-CVD-* and
//     BR-TMP-CVD-01), applied when BT-24 is CVDID.
//
// The flags map to the severities of package rules: fatal to Error, warning
// to Warning and information to Info. The rules of EN 16931 (BR-*, BR-CL-*,
// CII-SR-*) are not part of this package, nor are the Peppol rules that the
// XRechnung schematrons include (patterns peppol-cii-* and peppol-ubl-*),
// with one exception for UBL (see below).
//
// Check applies the CIUS rules whatever BT-24: the caller decides when the
// German rules apply (typically when BT-24 starts with CIUSID). CheckProfile,
// which is not part of the schematron, reports the ZUGFeRD profiles that the
// BMF does not accept as e-invoices (DE-PROFILE-01).
//
// Values are evaluated as XPath 2.0 evaluates them in the reference
// implementation: general comparisons on the raw string value of the
// elements and attributes, normalize-space on XML white space only, \d
// matching any Unicode decimal digit, and the code lists of common.sch
// searched with contains().
//
// # UBL
//
// For a UBL invoice (rules.Doc.UBL set), Check runs the UBL schematron on the
// UBL document, not on its CII view: 57 assertions (UBLCatalog), whose
// findings are located at the element at fault when there is one, at the
// node of the rule context otherwise. An identifier shared with the CII
// schematron keeps its severity and description; the contexts and tests
// differ:
//
//   - BR-DE-1 requires a cac:PaymentMeans; BR-DE-15, BR-DE-16, BR-DE-17,
//     BR-DE-18, BR-DE-21, BR-DE-22, BR-DE-26 and BR-DE-TMP-32 are tested on
//     the root element. BR-DE-16 looks at the VAT categories of the lines
//     (BT-151) and of the document allowances and charges with a charge
//     indicator 'true' or 'false', not at the VAT breakdown; any non blank
//     cac:PartyTaxScheme/cbc:CompanyID of the seller satisfies it, whatever
//     its tax scheme. BR-DE-5 accepts only cbc:Name.
//   - BR-DE-30 and BR-DE-31 apply when a cac:PaymentMandate is present:
//     BR-DE-30 then requires a party identifier of scheme SEPA of the seller
//     or of the payee, BR-DE-31 a cac:PayerFinancialAccount/cbc:ID.
//   - BR-DE-23-a, BR-DE-24-a and BR-DE-25-a require cac:PayeeFinancialAccount,
//     cac:CardAccount and cac:PaymentMandate; BR-DE-23-b, BR-DE-24-b and
//     BR-DE-25-b forbid the two others.
//   - BR-TMP-2 checks the cbc:URI of every external reference of an
//     additional supporting document, whatever its type code; a missing URI
//     fails.
//   - BR-TMP-6, of the UBL schematron only, requires the form YYYY-MM-DD for
//     every cbc:IssueDate, cbc:DueDate, cbc:StartDate, cbc:EndDate,
//     cbc:ActualDeliveryDate, cbc:TaxPointDate and cbc:PaymentDueDate.
//     BR-TMP-3, BR-TMP-4, BR-TMP-5, BR-TMP-7 and BR-DEX-15 are CII only.
//   - The extension adds BR-DEX-02 and BR-DEX-03 (sub invoice lines),
//     BR-DEX-09 (amount due with the third party payments) and BR-DEX-10 to
//     BR-DEX-14 (third party payments). BR-DEX-02, BR-DEX-03 and BR-DEX-10
//     to BR-DEX-14 apply to an Invoice, not to a CreditNote. BR-DEX-04
//     applies to cac:PartyIdentification/cbc:ID, where SEPA is also allowed
//     for the seller and the payee, and BR-DEX-05 to
//     cac:PartyLegalEntity/cbc:CompanyID.
//   - PEPPOL-EN16931-R043 (charge indicator 'true' or 'false') is the only
//     assertion of the Peppol patterns of the UBL schematron that the package
//     implements. As in the schematron, it is not tested on an allowance or
//     charge with a percentage and no base amount, or the reverse, which
//     PEPPOL-EN16931-R041 and R042 match first.
//
// # Faithfulness notes
//
//   - BR-DE-16 compares the whole string value of ram:CategoryTradeTax
//     with 'VAT', which only holds for an element whose sole content is the
//     text VAT: the document level allowance and charge part of the
//     condition is in practice always false. It is implemented as written.
//     In UBL, the allowance variable without condition on the tax scheme
//     ($BT-95-UBL-CN) is tested for invoices too, which makes the one with
//     the condition void, and the charge indicator is compared untrimmed.
//   - The extended code lists of common.sch are the concatenation of
//     ' XR01 XR02 XR03 ' and a list that starts with a space: the double space
//     makes an empty or blank schemeID (BR-DEX-04 to BR-DEX-08) or listID
//     (BR-TMP-CVD-01, also when the attribute is absent) pass. This is kept.
//   - BR-TMP-3 compares the base quantities as strings ('1' and '1.0'
//     differ), and its test accepts a unit code on the net price only,
//     although the message requires it on both prices. The test is kept.
//   - BR-DEX-13 counts the characters after the first '.' of the raw value:
//     a trailing space counts.
//   - Where the reference would stop on an XPath type error (several values
//     given to a single-valued function), the first value in document order
//     is used. Where it would stop on a failed cast to xs:decimal (BR-DEX-02,
//     BR-DEX-09), the assertion fails.
package de

import (
	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
)

// Specification identifiers (BT-24) of XRechnung 3.0.
const (
	// CIUSID identifies an invoice of the XRechnung CIUS.
	CIUSID = "urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0"
	// ExtensionID identifies an invoice of the XRechnung extension: the
	// BR-DEX-* rules apply.
	ExtensionID = CIUSID + "#conformant#urn:xeinkauf.de:kosit:extension:xrechnung_3.0"
	// CVDID identifies an invoice with commercial vehicle data (Clean
	// Vehicles Directive): the BR-DE-CVD-* rules apply.
	CVDID = CIUSID + "#compliant#urn:xeinkauf.de:kosit:xrechnung:cvd_0.9"
)

// Check runs the BR-DE rules on c.Doc and records the findings in c.Report.
func Check(c *rules.Checker) {
	if c == nil || c.Doc == nil {
		return
	}
	if c.Doc.UBL != nil {
		checkUBL(c)
		return
	}
	if c.Doc.Root == nil {
		return
	}
	k := &checker{Checker: c, root: c.Doc.Root}
	k.ext = k.specification(ExtensionID)
	k.cvd = k.specification(CVDID)

	// cii-pattern
	k.document()       // BR-DE-30, 31, 1, 15, 16, 17, 18, 22, 26
	k.guideline()      // BR-DE-21
	k.seller()         // BR-DE-2 to BR-DE-7, BR-DE-27, BR-DE-28
	k.addresses()      // BR-DE-8 to BR-DE-11
	k.attachments()    // BR-TMP-2, BR-TMP-4, BR-TMP-5
	k.dates()          // BR-TMP-7
	k.paymentMeans()   // BR-DE-19, 20, 23, 24, 25
	k.vatRates()       // BR-DE-14
	k.deliveryDates()  // BR-DE-TMP-32
	k.baseQuantities() // BR-TMP-3
	k.extension()      // cii-extension-pattern: BR-DEX-*
	k.vehicles()       // cii-cvd-pattern: BR-DE-CVD-*, BR-TMP-CVD-01
}

// checker wraps rules.Checker with the values shared by several rules.
type checker struct {
	*rules.Checker
	root *xmltree.Node

	ext bool // BT-24 is ExtensionID ($isExtension)
	cvd bool // BT-24 is CVDID ($isCVD)
}

// specification reports whether a text of BT-24 equals id
// (ram:GuidelineSpecifiedDocumentContextParameter/ram:ID/text() = id).
func (k *checker) specification(id string) bool {
	for n := range all(k.root, pGuideline) {
		if n.Text == id {
			return true
		}
	}
	return false
}

// fail records a finding located at n, or at the root element when n is nil.
func (k *checker) fail(rule string, sev rules.Severity, n *xmltree.Node, format string, args ...any) {
	if n == nil {
		n = k.root
	}
	k.Fail(rule, sev, n, format, args...)
}

// Paths from the root element (rsm:CrossIndustryInvoice), in local names.
const (
	pContext     = "ExchangedDocumentContext"
	pGuideline   = "ExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/ID"
	pTypeCode    = "ExchangedDocument/TypeCode"
	pTransaction = "SupplyChainTradeTransaction"
	pLines       = "SupplyChainTradeTransaction/IncludedSupplyChainTradeLineItem"
	pAgreement   = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement"
	pSeller      = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/SellerTradeParty"
	pBuyer       = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/BuyerTradeParty"
	pHeaderARD   = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/AdditionalReferencedDocument"
	pShipTo      = "SupplyChainTradeTransaction/ApplicableHeaderTradeDelivery/ShipToTradeParty"
	pSettlement  = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement"
	pPaymentMean = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/SpecifiedTradeSettlementPaymentMeans"
	pHeaderTax   = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/ApplicableTradeTax"
	pMandate     = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/SpecifiedTradePaymentTerms/DirectDebitMandateID"
	pCreditorRef = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/CreditorReferenceID"
	pPayerIBAN   = "PayerPartyDebtorFinancialAccount/IBANID"
	pPayeeAcct   = "PayeePartyCreditorFinancialAccount"
	pCard        = "ApplicableTradeSettlementFinancialCard"
)
