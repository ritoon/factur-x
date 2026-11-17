// Package fr implements the French rules BR-FR of the 2026 e-invoicing
// reform for the CII syntax, as published by the FNFE-MPE in the schematron
// "BR-FR-Flux2-Schematron-CII" version 1.4.0.04 (fatal on reception by
// 1 October 2026 at the latest, and on emission from that date).
//
// The package reimplements every assertion of the schematron in Go, with its
// exact identifier: 59 base identifiers (29 BR-FR, 10 BR-FR-CO, 4 BR-FR-DEC,
// 13 BR-FR-MV and 3 BR-FR-BD) declined in 171 assertions, all fatal. The
// custom XSLT functions of the schematron (identifier format, SIRET/SIREN
// coherence, schemeID 0225/0224 formats, billing frames BT-23, VAT rates,
// attachment codes, BAR treatments, decimal formats, multi-vendor and
// two-way frames) are reimplemented as well.
//
// Values are evaluated as XPath 2.0 evaluates them in the reference
// implementation: general comparisons ("=") on the raw string value of the
// elements, normalize-space on XML white space only, string-length in
// characters, \d matching any Unicode decimal digit, string comparison of
// the dates in BR-FR-CO-07, and sums of untyped amounts in double precision
// (sum() in BR-FR-MV-05 and BR-FR-MV-09).
//
// # Profiles
//
// The schematron targets the EN 16931 and EXTENDED-CTC-FR profiles. Check
// applies it whatever the profile of the invoice: the caller decides when the
// French rules apply. CheckProfile, which is not part of the schematron,
// reports the profiles that the reform does not admit (FR-PROFILE-*). Assertions on elements that only exist in the EXTENDED
// data set never fire on an EN 16931 invoice: they are the EXT-FR-FE-*
// variants of BR-FR-01 to BR-FR-04 (line invoice references, line delivery
// dates), BR-FR-09, BR-FR-23, BR-FR-25 and BR-FR-CO-10 for the agent,
// invoicer, invoicee, payer and line ship-to parties, BR-FR-27 for the
// attribute code (EXT-FR-FE-159), BR-FR-15_BT-95_BT-151 for GROUP and
// INFORMATION lines, and all BR-FR-MV-* and BR-FR-BD-* assertions, which
// apply to the multi-vendor (BT-23 = S8, B8, M8) and two-way (S9, B9, M9)
// billing frames and rely on the line sub-types (EXT-FR-FE-162/163), the line
// seller (EXT-FR-FE-164 to 177) and the line totals (EXT-FR-FE-181 to 184).
//
// # UBL
//
// For a UBL invoice (rules.Doc.UBL not nil), Check runs instead the
// schematron "BR-FR-Flux2-Schematron-UBL" of the same version on the UBL
// document itself, with the identifiers, contexts and messages of that
// schematron (UBLCatalog): 175 assertions in 59 patterns, 173 of which can
// fire. The UBL schematron splits, renames or adds identifiers:
// BR-FR-01_BT-1-1 (length) and BR-FR-01_BT-1-2 (format) for BR-FR-01_BT-1,
// BR-FR-05_BT-22-1 to -3 and BR-FR-06_BT-21-1 to -4 for the subject-code
// variants, BR-FR-15_* per context (with BR-FR-08_BT-151-2 for the items),
// BR-FR-30_BT-21 for BR-FR-31_BT-21, BR-FR-32-ID for BR-FR-32-GLOBALID,
// BR-FR-CO-04_BT-3 for BR-FR-CO-04_BT-4, BR-FR-BD-02_BT-30 for
// BR-FR-BD-02_BT-31, BR-FR-MV-BD_BT-3 for BR-FR-BD-13_BT-3, and
// BR-FR-03_BT-134/135 for the line invoicing period. In UBL the subject code
// of a note is written between # at its start (cbc:Note "#PMT#…") and the
// notes are joined with # before the search; the line sub-type and parent
// line of the multi-vendor and two-way frames are carried by the line
// reference to the invoice itself (cac:BillingReference with
// cbc:DocumentStatusCode and cac:BillingReferenceLine); the line seller is
// cac:Item/cac:ManufacturerParty and the sub-invoice number the line document
// reference of type 130 and scheme AFL. A finding is located at the context
// node of its rule, the root element when the rule has none.
//
// Several contexts of the UBL schematron have no leading root ("a/b/c") and
// match at any depth: BR-FR-10, BR-FR-15, BR-FR-16, BR-FR-17, BR-FR-32,
// BR-FR-CO-10 and BR-FR-DEC-01 (monetary totals and VAT amounts) apply to
// such elements wherever they are.
//
// # Faithfulness notes
//
//   - BR-FR-DEC-01_BT-111 has the same context as BR-FR-DEC-01_BT-110
//     (ram:TaxTotalAmount) in the same pattern. In Schematron only the first
//     matching rule of a pattern fires for a node, so the reference reports
//     every tax total as BR-FR-DEC-01_BT-110 and never BT-111. Check does the
//     same; BR-FR-DEC-01_BT-111 is not in Catalog.
//   - BR-FR-30 targets ram:AdditionalReferencedDocument directly under
//     ram:IncludedSupplyChainTradeLineItem, a path the CII schema does not
//     define (BT-128 lives under ram:SpecifiedLineTradeSettlement). It is
//     implemented as written and only fires on schema-invalid documents.
//   - Where the reference would stop on an XPath type error (several values
//     given to a single-valued function, such as two SIREN 0002 for one
//     party), the first value in document order is used. Where it would stop
//     on a failed xs:decimal cast, the assertion fails.
//   - UBL: for the same reason as BR-FR-DEC-01_BT-111, BR-FR-16_BT-103
//     (second rule on cac:AllowanceCharge/cac:TaxCategory in its pattern)
//     never fires: every allowance or charge rate is reported as
//     BR-FR-16_BT-96. Neither is in UBLCatalog.
//   - UBL: BR-FR-27 targets cac:Item directly under the root,
//     BR-FR-09_EXT-FR-FE-146 looks for the SIRET below the line delivery
//     location itself (cac:DeliveryLocation/cac:DeliveryLocation/cbc:ID), and
//     BR-FR-29_BT-18 and BR-FR-30_BT-128 count several cbc:ID in one document
//     reference: these paths do not exist in the UBL schema, so the
//     assertions only fire on schema-invalid documents. BR-FR-16_BT-152 only
//     applies to cac:InvoiceLine, not to cac:CreditNoteLine.
//   - UBL: a date (BR-FR-03) must be written with ASCII digits; any other
//     digit makes the cast of the year fail in the reference.
package fr

import (
	"iter"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// Check runs the BR-FR rules on c.Doc and records the findings in c.Report.
func Check(c *rules.Checker) {
	if c == nil || c.Doc == nil || c.Doc.Root == nil {
		return
	}
	if c.Doc.UBL != nil {
		checkUBL(c)
		return
	}
	k := newChecker(c)
	k.identifiers()     // BR-FR-01, BR-FR-02
	k.dates()           // BR-FR-03
	k.documentTypes()   // BR-FR-04
	k.mandatoryNotes()  // BR-FR-05
	k.uniqueNotes()     // BR-FR-06
	k.billingMode()     // BR-FR-08
	k.siretSiren()      // BR-FR-09
	k.sellerSiren()     // BR-FR-10
	k.buyerSiren()      // BR-FR-11
	k.endpointPresent() // BR-FR-12, BR-FR-13
	k.vatCategories()   // BR-FR-15
	k.vatRates()        // BR-FR-16
	k.attachments()     // BR-FR-17, BR-FR-18
	k.barTreatment()    // BR-FR-20
	k.b2bEndpoints()    // BR-FR-21, BR-FR-22
	k.endpointFormats() // BR-FR-23
	k.privateIDFormat() // BR-FR-24
	k.endpointLengths() // BR-FR-25
	k.privateIDLength() // BR-FR-26
	k.characteristics() // BR-FR-27, BR-FR-28
	k.objects()         // BR-FR-29, BR-FR-30
	k.barUnique()       // BR-FR-31
	k.legalIDs()        // BR-FR-32
	k.coherence()       // BR-FR-CO-*
	k.decimals()        // BR-FR-DEC-*
	k.multiVendor()     // BR-FR-MV-*, BR-FR-BD-*
}

// checker wraps rules.Checker with the values shared by several rules.
type checker struct {
	*rules.Checker
	root *xmltree.Node

	special bool // BT-23 is S8, B8, M8, S9, B9 or M9 (custom:isSpecialContract)
	mv      bool // BT-23 is S8, B8 or M8 (custom:isSpecialContractMV)
	bd      bool // BT-23 is S9, B9 or M9 (custom:isSpecialContractBD)
}

func newChecker(c *rules.Checker) *checker {
	k := &checker{Checker: c, root: c.Doc.Root}
	if id := first(k.root, pBillingMode); id != nil {
		switch norm(sv(id)) {
		case "S8", "B8", "M8":
			k.special, k.mv = true, true
		case "S9", "B9", "M9":
			k.special, k.bd = true, true
		}
	}
	return k
}

// fail records an error: every BR-FR assertion is fatal. A finding without
// element (a missing party) is located at the root element.
func (k *checker) fail(rule string, n *xmltree.Node, format string, args ...any) {
	if n == nil {
		n = k.root
	}
	k.Fail(rule, rules.Error, n, format, args...)
}

// Paths from the root element (rsm:CrossIndustryInvoice), in local names.
const (
	pContext     = "ExchangedDocumentContext"
	pBillingMode = "ExchangedDocumentContext/BusinessProcessSpecifiedDocumentContextParameter/ID"
	pHeader      = "ExchangedDocument"
	pNotes       = "ExchangedDocument/IncludedNote"
	pTypeCode    = "ExchangedDocument/TypeCode"
	pTransaction = "SupplyChainTradeTransaction"
	pLines       = "SupplyChainTradeTransaction/IncludedSupplyChainTradeLineItem"
	pAgreement   = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement"
	pSeller      = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/SellerTradeParty"
	pBuyer       = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/BuyerTradeParty"
	pBuyerAgent  = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/BuyerAgentTradeParty"
	pSalesAgent  = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/SalesAgentTradeParty"
	pTaxRep      = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/SellerTaxRepresentativeTradeParty"
	pHeaderARD   = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/AdditionalReferencedDocument"
	pShipTo      = "SupplyChainTradeTransaction/ApplicableHeaderTradeDelivery/ShipToTradeParty"
	pSettlement  = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement"
	pPayee       = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/PayeeTradeParty"
	pPayer       = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/PayerTradeParty"
	pInvoicee    = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/InvoiceeTradeParty"
	pInvoicer    = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/InvoicerTradeParty"
	pHeaderTax   = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/ApplicableTradeTax"
	pHeaderAC    = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/SpecifiedTradeAllowanceCharge"
	pSummation   = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/SpecifiedTradeSettlementHeaderMonetarySummation"
	pDueDates    = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/SpecifiedTradePaymentTerms/DueDateDateTime/DateTimeString"
	pCurrency    = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/InvoiceCurrencyCode"
	pTaxCurrency = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/TaxCurrencyCode"
	pIssueDate   = "ExchangedDocument/IssueDateTime/DateTimeString"
	pLegalID     = "SpecifiedLegalOrganization/ID"
	pURIID       = "URIUniversalCommunication/URIID"
	pLineStatus  = "AssociatedDocumentLineDocument/LineStatusReasonCode"
	pParentLine  = "AssociatedDocumentLineDocument/ParentLineID"
	pLineID      = "AssociatedDocumentLineDocument/LineID"
	pItemSeller  = "SpecifiedLineTradeAgreement/ItemSellerTradeParty"
	pLineSum     = "SpecifiedLineTradeSettlement/SpecifiedTradeSettlementLineMonetarySummation"
	pLineARD     = "SpecifiedLineTradeSettlement/AdditionalReferencedDocument"
)

// all yields the elements matching a path of local names below n, in
// document order, exploring every branch (like an XPath location path).
func all(n *xmltree.Node, path string) iter.Seq[*xmltree.Node] {
	return func(yield func(*xmltree.Node) bool) {
		walk(n, path, yield)
	}
}

func walk(n *xmltree.Node, path string, yield func(*xmltree.Node) bool) bool {
	if n == nil {
		return true
	}
	seg, rest, _ := strings.Cut(path, "/")
	for _, c := range n.Children {
		if c.Name != seg {
			continue
		}
		if rest == "" {
			if !yield(c) {
				return false
			}
		} else if !walk(c, rest, yield) {
			return false
		}
	}
	return true
}

// first returns the first element matching path below n, or nil.
func first(n *xmltree.Node, path string) *xmltree.Node {
	for c := range all(n, path) {
		return c
	}
	return nil
}

// firstWith returns the first element matching path whose attribute attr
// equals v, or nil (path[@attr = 'v'][1]).
func firstWith(n *xmltree.Node, path, attr, v string) *xmltree.Node {
	for c := range all(n, path) {
		if a, ok := c.LookupAttr(attr); ok && a == v {
			return c
		}
	}
	return nil
}

// count returns the number of elements matching path.
func count(n *xmltree.Node, path string) int {
	i := 0
	for range all(n, path) {
		i++
	}
	return i
}

// exists reports whether an element matches path.
func exists(n *xmltree.Node, path string) bool { return first(n, path) != nil }

// anyEq reports whether an element matching path has the string value v
// (the XPath general comparison path = 'v').
func anyEq(n *xmltree.Node, path, v string) bool {
	for c := range all(n, path) {
		if sv(c) == v {
			return true
		}
	}
	return false
}

// anyIn reports whether an element matching path has one of the string
// values vs (path = ('a', 'b', …)).
func anyIn(n *xmltree.Node, path string, vs ...string) bool {
	for c := range all(n, path) {
		s := sv(c)
		for _, v := range vs {
			if s == v {
				return true
			}
		}
	}
	return false
}

// anyNe reports whether an element matching path has a string value other
// than v (path != 'v'); false when nothing matches.
func anyNe(n *xmltree.Node, path, v string) bool {
	for c := range all(n, path) {
		if sv(c) != v {
			return true
		}
	}
	return false
}

// attrIs reports whether n has the attribute attr with value v (@attr = 'v').
func attrIs(n *xmltree.Node, attr, v string) bool {
	a, ok := n.LookupAttr(attr)
	return ok && a == v
}

// sv returns the XPath string value of n: the concatenated character data of
// the element and its descendants, untrimmed ("" for nil).
func sv(n *xmltree.Node) string {
	if n == nil {
		return ""
	}
	if len(n.Children) == 0 {
		return n.Text
	}
	var b strings.Builder
	appendSV(&b, n)
	return b.String()
}

func appendSV(b *strings.Builder, n *xmltree.Node) {
	b.WriteString(n.Text)
	for _, c := range n.Children {
		appendSV(b, c)
	}
}

// isSpace reports whether c is XML white space, the only characters
// normalize-space() strips.
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// norm implements XPath normalize-space(): XML white space is trimmed and
// every run of it is replaced by a single space.
func norm(s string) string {
	clean := true
	for i := 0; i < len(s); i++ {
		if isSpace(s[i]) && (s[i] != ' ' || i == 0 || i == len(s)-1 || s[i+1] == ' ') {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	b := make([]byte, 0, len(s))
	sp := false
	for i := 0; i < len(s); i++ {
		if isSpace(s[i]) {
			sp = len(b) > 0
			continue
		}
		if sp {
			b = append(b, ' ')
			sp = false
		}
		b = append(b, s[i])
	}
	return string(b)
}

// nsv returns normalize-space(n).
func nsv(n *xmltree.Node) string { return norm(sv(n)) }

// length returns string-length(): the number of characters.
func length(s string) int { return utf8.RuneCountInString(s) }

// substr returns substring(s, 1, n): the first n characters of s.
func substr(s string, n int) string {
	i := 0
	for j := range s {
		if i == n {
			return s[:j]
		}
		i++
	}
	return s
}

// isDigit matches the XPath regular expression \d: any Unicode decimal digit.
func isDigit(r rune) bool { return r >= '0' && r <= '9' || r > utf8.RuneSelf && unicode.IsDigit(r) }

// digits reports whether s is made of exactly n characters matching \d
// (matches(s, '^\d{n}$')).
func digits(s string, n int) bool {
	i := 0
	for _, r := range s {
		if !isDigit(r) {
			return false
		}
		i++
	}
	return i == n
}

// in reports whether s is one of vs (s = ('a', 'b', …)).
func in(s string, vs ...string) bool {
	for _, v := range vs {
		if s == v {
			return true
		}
	}
	return false
}

// decimal casts the string value of n to xs:decimal. ok is false when n is
// nil (the empty sequence) or the cast fails.
func decimal(n *xmltree.Node) (invoice.Decimal, bool) {
	if n == nil {
		return invoice.Decimal{}, false
	}
	d, err := invoice.ParseDecimal(norm(sv(n)))
	return d, err == nil
}

// float casts the string value of n to xs:double, as sum() does with untyped
// values. ok is false when the cast fails.
func float(n *xmltree.Node) (float64, bool) {
	s := norm(sv(n))
	switch s {
	case "INF", "+INF":
		s = "+Inf"
	case "-INF":
		s = "-Inf"
	case "NaN":
	default:
		// xs:double lexical space: (+|-)?(digits(.digits?)?|.digits)([Ee](+|-)?digits)?
		mant, exp := s, ""
		if i := strings.IndexAny(s, "eE"); i >= 0 {
			mant, exp = s[:i], s[i+1:]
			if exp != "" && (exp[0] == '+' || exp[0] == '-') {
				exp = exp[1:]
			}
			if exp == "" || strings.Trim(exp, "0123456789") != "" {
				return 0, false
			}
		}
		if mant != "" && (mant[0] == '+' || mant[0] == '-') {
			mant = mant[1:]
		}
		ip, fp, dot := strings.Cut(mant, ".")
		if ip == "" && fp == "" || strings.Trim(ip, "0123456789") != "" || strings.Trim(fp, "0123456789") != "" || dot && ip == "" && fp == "" {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return f, true
		}
		return 0, false
	}
	return f, true
}

// toFloat converts an xs:decimal to xs:double (correctly rounded).
func toFloat(d invoice.Decimal) float64 {
	f, _ := strconv.ParseFloat(d.String(), 64)
	return f
}
