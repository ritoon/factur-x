// Package de implements the German rules of XRechnung 3.0 for the CII
// syntax, as published by KoSIT in the XRechnung schematron (patterns
// cii-pattern, cii-extension-pattern and cii-cvd-pattern of
// XRechnung-CII-validation.sch, with the variables of common.sch).
//
// The package reimplements the 52 assertions of the schematron in Go, with
// their exact identifiers and flags:
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
// CII-SR-*) are not part of this package.
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
// # Faithfulness notes
//
//   - BR-DE-16 compares the whole string value of ram:CategoryTradeTax
//     with 'VAT', which only holds for an element whose sole content is the
//     text VAT: the document level allowance and charge part of the
//     condition is in practice always false. It is implemented as written.
//   - The extended code lists of common.sch are the concatenation of
//     ' XR01 XR02 XR03 ' and a list that starts with a space: the double space
//     makes an empty or blank schemeID (BR-DEX-04 to BR-DEX-08) or listID
//     (BR-TMP-CVD-01, also when the attribute is absent) pass. This is kept.
//   - BR-TMP-3 compares the base quantities as strings ('1' and '1.0'
//     differ), and its test accepts a unit code on the net price only,
//     although the message requires it on both prices. The test is kept.
//   - Where the reference would stop on an XPath type error (several values
//     given to a single-valued function), the first value in document order
//     is used.
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
	if c == nil || c.Doc == nil || c.Doc.Root == nil {
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
