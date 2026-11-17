package de

import (
	"strings"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
)

// Rules of the pattern cii-pattern: the XRechnung CIUS.

// taxCodes are the VAT category codes of BR-DE-16.
var taxCodes = []string{"S", "Z", "E", "AE", "K", "G", "L", "M"}

// document checks the rules whose context is the root element.
func (k *checker) document() {
	r := k.root
	settlement := first(r, pSettlement)

	// BR-DE-30, BR-DE-31: BG-19 is all of BT-89, BT-90 and BT-91, or none.
	bt89, bt90, bt91 := exists(r, pMandate), exists(r, pCreditorRef), exists(r, pPaymentMean+"/"+pPayerIBAN)
	none := !bt89 && !bt90 && !bt91
	if !((bt89 || bt91) && bt90 || none) {
		k.fail("BR-DE-30", rules.Error, settlement, "[BR-DE-30] If DIRECT DEBIT (BG-19) is present, the Bank assigned creditor identifier (BT-90) must be transmitted.")
	}
	if !((bt89 || bt90) && bt91 || none) {
		k.fail("BR-DE-31", rules.Error, settlement, "[BR-DE-31] If DIRECT DEBIT (BG-19) is present, the Debited account identifier (BT-91) must be transmitted.")
	}

	if !exists(r, pPaymentMean) {
		k.fail("BR-DE-1", rules.Error, settlement, "[BR-DE-1] An invoice must contain information on PAYMENT INSTRUCTIONS (BG-16).")
	}
	if !nonBlank(r, pAgreement+"/BuyerReference") {
		k.fail("BR-DE-15", rules.Error, first(r, pAgreement), "[BR-DE-15] The element Buyer reference (BT-10) must be transmitted; value: %q.", sv(first(r, pAgreement+"/BuyerReference")))
	}

	// BR-DE-16
	const lineTax = pLines + "/SpecifiedLineTradeSettlement/ApplicableTradeTax"
	const chargeTax = pSettlement + "/SpecifiedTradeAllowanceCharge/CategoryTradeTax"
	used := anyEq(r, lineTax+"/TypeCode", "VAT") && anyIn(r, lineTax+"/CategoryCode", taxCodes...) ||
		anyEq(r, chargeTax, "VAT") && anyIn(r, chargeTax+"/CategoryCode", taxCodes...)
	if used && !k.sellerTaxID() && !exists(r, pAgreement+"/SellerTaxRepresentativeTradeParty") {
		k.fail("BR-DE-16", rules.Error, first(r, pSeller), "[BR-DE-16] If the VAT category codes S, Z, E, AE, K, G, L or M are used in an invoice, at least one of the elements Seller VAT identifier (BT-31), Seller tax registration identifier (BT-32) or SELLER TAX REPRESENTATIVE PARTY (BG-11) must be transmitted.")
	}

	typeNode := first(r, pTypeCode)
	typeCode := nsv(typeNode)
	if !in(typeCode, "326", "380", "384", "389", "381", "875", "876", "877") {
		k.fail("BR-DE-17", rules.Warning, typeNode, "[BR-DE-17] The Invoice type code (BT-3) should only be one of the UNTDID 1001 codes 326 (Partial invoice), 380 (Commercial invoice), 384 (Corrected invoice), 389 (Self-billed invoice), 381 (Credit note), 875 (Partial construction invoice), 876 (Partial final construction invoice) and 877 (Final construction invoice); value: %q.", typeCode)
	}

	// BR-DE-18: first BT-20 of each ram:SpecifiedTradePaymentTerms.
	var descs []*xmltree.Node
	for terms := range all(r, pSettlement+"/SpecifiedTradePaymentTerms") {
		if d := first(terms, "Description"); d != nil {
			descs = append(descs, d)
		}
	}
	k.skonto(descs)
	k.fileNames() // BR-DE-22

	if typeCode == "384" && !exists(r, pSettlement+"/InvoiceReferencedDocument") {
		k.fail("BR-DE-26", rules.Warning, typeNode, "[BR-DE-26] If the Invoice type code (BT-3) is 384 (Corrected invoice), PRECEDING INVOICE REFERENCE (BG-3) should be present at least once.")
	}
}

// sellerTaxID reports whether the seller has a non-blank tax registration of
// scheme VA or FC (BT-31, BT-32).
func (k *checker) sellerTaxID() bool {
	for id := range all(k.root, pSeller+"/SpecifiedTaxRegistration/ID") {
		if s := norm(attr(id, "schemeID")); (s == "VA" || s == "FC") && nsv(id) != "" {
			return true
		}
	}
	return false
}

// skonto checks BR-DE-18 on descs, the first payment terms (BT-20) of each
// payment terms group: their lines that start with # are cash discount lines
// #SKONTO#TAGE=n#PROZENT=n.nn[#BASISBETRAG=n.nn]#, and the last one is
// followed by a line break.
func (k *checker) skonto(descs []*xmltree.Node) {
	var lastTokens []string // tokenize(Description[1], '#.+#')[last()]
	for _, d := range descs {
		if s := sv(d); s != "" {
			lastTokens = append(lastTokens, lastToken(s))
		}
	}
	// The line break test does not depend on the line: it is evaluated once.
	breakOK := len(lastTokens) > 0 && leadingLineBreak(lastTokens[0])
	for _, d := range descs {
		for line := range lines(sv(d)) {
			l := norm(line)
			if !strings.HasPrefix(l, "#") {
				continue
			}
			if !validSkonto(l) {
				k.fail("BR-DE-18", rules.Error, d, "[BR-DE-18] Cash discount lines in Payment terms (BT-20) must match #SKONTO#TAGE=n#PROZENT=n.nn# with an optional #BASISBETRAG=n.nn segment before the last #, in capital letters, without additional white space, each followed by an XML line break; line: %q.", l)
				return
			}
			if !breakOK {
				k.fail("BR-DE-18", rules.Error, d, "[BR-DE-18] A complete cash discount entry in Payment terms (BT-20) must be followed by an XML line break; text after the last entry: %q.", lastTokens[0])
				return
			}
		}
	}
}

// lines yields tokenize(s, '(\r?\n)'): the lines of s without their line
// break. The empty string has no line.
func lines(s string) func(func(string) bool) {
	return func(yield func(string) bool) {
		if s == "" {
			return
		}
		for {
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				yield(s)
				return
			}
			if !yield(strings.TrimSuffix(s[:i], "\r")) {
				return
			}
			s = s[i+1:]
		}
	}
}

// lastToken returns tokenize(s, '#.+#')[last()]: the text after the last
// match of '#.+#', where . matches any character but a line break and .+ is
// greedy.
func lastToken(s string) string {
	end := 0
	for i := 0; i < len(s); {
		j := strings.IndexByte(s[i:], '#')
		if j < 0 {
			break
		}
		j += i
		e := j + 1
		for e < len(s) && s[e] != '\n' && s[e] != '\r' {
			e++
		}
		if c := strings.LastIndexByte(s[j+1:e], '#'); c >= 1 {
			end = j + 1 + c + 1
			i = end
			continue
		}
		i = j + 1
	}
	return s[end:]
}

// leadingLineBreak implements matches(s, '^\s*\n').
func leadingLineBreak(s string) bool {
	for i := 0; i < len(s) && isSpace(s[i]); i++ {
		if s[i] == '\n' {
			return true
		}
	}
	return false
}

// validSkonto implements matches(s, $XR-SKONTO-REGEX) on a normalized line:
// ^#SKONTO#TAGE=[0-9]+#PROZENT=[0-9]+\.[0-9]{2}(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$.
func validSkonto(s string) bool {
	p := scanner(s)
	if !p.lit("#SKONTO#TAGE=") || !p.digits() || !p.lit("#PROZENT=") || !p.amount() {
		return false
	}
	if p.lit("#BASISBETRAG=") {
		p.lit("-")
		if !p.amount() {
			return false
		}
	}
	return p.lit("#") && p == ""
}

// scanner consumes a string from the left.
type scanner string

func (p *scanner) lit(s string) bool {
	r, ok := strings.CutPrefix(string(*p), s)
	*p = scanner(r)
	return ok
}

// digits consumes [0-9]+.
func (p *scanner) digits() bool {
	i := 0
	for i < len(*p) && (*p)[i] >= '0' && (*p)[i] <= '9' {
		i++
	}
	*p = (*p)[i:]
	return i > 0
}

// amount consumes [0-9]+\.[0-9]{2}.
func (p *scanner) amount() bool {
	if !p.digits() || !p.lit(".") || len(*p) < 2 {
		return false
	}
	for i := 0; i < 2; i++ {
		if (*p)[i] < '0' || (*p)[i] > '9' {
			return false
		}
	}
	*p = (*p)[2:]
	return true
}

// fileNames checks BR-DE-22: no ram:AdditionalReferencedDocument has an
// attachment file name equal to the one of a preceding sibling document.
func (k *checker) fileNames() {
	dup := ""
	k.root.Walk(func(n *xmltree.Node) bool {
		if dup != "" {
			return false
		}
		var seen map[string]bool
		for _, doc := range n.Children {
			if doc.Name != "AdditionalReferencedDocument" {
				continue
			}
			for obj := range all(doc, "AttachmentBinaryObject") {
				if f, ok := obj.LookupAttr("filename"); ok && seen[f] {
					dup = f
					return false
				}
			}
			for obj := range all(doc, "AttachmentBinaryObject") {
				if f, ok := obj.LookupAttr("filename"); ok {
					if seen == nil {
						seen = map[string]bool{}
					}
					seen[f] = true
				}
			}
		}
		return true
	})
	if dup != "" {
		k.fail("BR-DE-22", rules.Error, nil, "[BR-DE-22] Not all filename attributes of the attached documents (BT-125) are unique; repeated file name: %q.", dup)
	}
}

// guideline checks BR-DE-21: BT-24 is an XRechnung 3.0 identifier.
func (k *checker) guideline() {
	for ctx := range all(k.root, pContext) {
		const path = "GuidelineSpecifiedDocumentContextParameter/ID"
		if !anyIn(ctx, path, CIUSID, ExtensionID, CVDID) {
			k.fail("BR-DE-21", rules.Warning, orNode(first(ctx, path), ctx), "[BR-DE-21] The Specification identifier (BT-24) should syntactically match the identifier of the XRechnung standard (%s, or the identifier of the extension or of the CVD); value: %q.", CIUSID, sv(first(ctx, path)))
		}
	}
}

// orNode returns n, or alt when n is nil.
func orNode(n, alt *xmltree.Node) *xmltree.Node {
	if n != nil {
		return n
	}
	return alt
}

// seller checks BR-DE-2 to BR-DE-7, BR-DE-27 and BR-DE-28 on the seller,
// its address and its contacts.
func (k *checker) seller() {
	for seller := range all(k.root, pSeller) {
		if !exists(seller, "DefinedTradeContact") {
			k.fail("BR-DE-2", rules.Error, seller, "[BR-DE-2] The group SELLER CONTACT (BG-6) must be transmitted.")
		}
	}
	for addr := range all(k.root, pSeller+"/PostalTradeAddress") {
		if !nonBlank(addr, "CityName") {
			k.fail("BR-DE-3", rules.Error, addr, "[BR-DE-3] The element Seller city (BT-37) must be transmitted; value: %q.", sv(first(addr, "CityName")))
		}
		if !nonBlank(addr, "PostcodeCode") {
			k.fail("BR-DE-4", rules.Error, addr, "[BR-DE-4] The element Seller post code (BT-38) must be transmitted; value: %q.", sv(first(addr, "PostcodeCode")))
		}
	}
	for contact := range all(k.root, pSeller+"/DefinedTradeContact") {
		if !nonBlank(contact, "PersonName") && !nonBlank(contact, "DepartmentName") {
			k.fail("BR-DE-5", rules.Error, contact, "[BR-DE-5] The element Seller contact point (BT-41) must be transmitted.")
		}
		const phonePath = "TelephoneUniversalCommunication/CompleteNumber"
		const emailPath = "EmailURIUniversalCommunication/URIID"
		if !nonBlank(contact, phonePath) {
			k.fail("BR-DE-6", rules.Error, contact, "[BR-DE-6] The element Seller contact telephone number (BT-42) must be transmitted.")
		}
		if !nonBlank(contact, emailPath) {
			k.fail("BR-DE-7", rules.Error, contact, "[BR-DE-7] The element Seller contact email address (BT-43) must be transmitted.")
		}
		phoneNode := first(contact, phonePath)
		if phone := nsv(phoneNode); !validPhone(phone) {
			k.fail("BR-DE-27", rules.Warning, orNode(phoneNode, contact), "[BR-DE-27] The Seller contact telephone number (BT-42) should contain at least three digits; value: %q.", phone)
		}
		emailNode := first(contact, emailPath)
		if email := nsv(emailNode); !validEmail(email) {
			k.fail("BR-DE-28", rules.Warning, orNode(emailNode, contact), "[BR-DE-28] The Seller contact email address (BT-43) should contain exactly one @ sign, with at least one character before it and a domain of at least two dot-separated labels after it, without white space; value: %q.", email)
		}
	}
}

// validPhone implements matches(s, $XR-TELEPHONE-REGEX) on a normalized
// value: at least three ASCII digits.
func validPhone(s string) bool {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n++
		}
	}
	return n >= 3
}

// validEmail implements matches(s, $XR-EMAIL-REGEX):
// ^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$.
func validEmail(s string) bool {
	local, domain, ok := strings.Cut(s, "@")
	if !ok || local == "" || strings.IndexByte(domain, '@') >= 0 || strings.ContainsAny(s, " \t\n\r") {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if l == "" {
			return false
		}
	}
	return true
}

// addresses checks BR-DE-8 to BR-DE-11: city and post code of the buyer and
// deliver to addresses.
func (k *checker) addresses() {
	for _, a := range []struct {
		path             string
		city, postcode   string
		cityMsg, postMsg string
	}{
		{pBuyer + "/PostalTradeAddress", "BR-DE-8", "BR-DE-9",
			"[BR-DE-8] The element Buyer city (BT-52) must be transmitted; value: %q.",
			"[BR-DE-9] The element Buyer post code (BT-53) must be transmitted; value: %q."},
		{pShipTo + "/PostalTradeAddress", "BR-DE-10", "BR-DE-11",
			"[BR-DE-10] The element Deliver to city (BT-77) must be transmitted if the group DELIVER TO ADDRESS (BG-15) is transmitted; value: %q.",
			"[BR-DE-11] The element Deliver to post code (BT-78) must be transmitted if the group DELIVER TO ADDRESS (BG-15) is transmitted; value: %q."},
	} {
		for addr := range all(k.root, a.path) {
			if !nonBlank(addr, "CityName") {
				k.fail(a.city, rules.Error, addr, a.cityMsg, sv(first(addr, "CityName")))
			}
			if !nonBlank(addr, "PostcodeCode") {
				k.fail(a.postcode, rules.Error, addr, a.postMsg, sv(first(addr, "PostcodeCode")))
			}
		}
	}
}

// attachments checks BR-TMP-2, BR-TMP-4 and BR-TMP-5 on the supporting
// documents of type 916 (BG-24).
func (k *checker) attachments() {
	for doc := range all(k.root, pHeaderARD) {
		if !anyEq(doc, "TypeCode", "916") {
			continue
		}
		if u := first(doc, "URIID"); u != nil && !validURL(sv(u)) {
			k.fail("BR-TMP-2", rules.Error, u, "[BR-TMP-2] The External document location (BT-124) must contain an absolute URL with a valid scheme; value: %q.", sv(u))
		}
		if n := count(doc, "Name"); n > 1 {
			k.fail("BR-TMP-4", rules.Warning, doc, "[BR-TMP-4] The Supporting document description (BT-123, ram:Name) may appear at most once in Additional supporting documents (BG-24); occurrences: %d.", n)
		}
		if n := count(doc, "AttachmentBinaryObject"); n > 1 {
			k.fail("BR-TMP-5", rules.Warning, doc, "[BR-TMP-5] The Attached document (BT-125, ram:AttachmentBinaryObject) may appear at most once in Additional supporting documents (BG-24); occurrences: %d.", n)
		}
	}
}

// validURL implements matches(s, $XR-URL-REGEX): ^([a-zA-Z])([a-zA-Z0-9+.-])+:.*
func validURL(s string) bool {
	if s == "" || !(s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z') {
		return false
	}
	i := 1
	for i < len(s) {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '+' || c == '.' || c == '-') {
			break
		}
		i++
	}
	return i >= 2 && i < len(s) && s[i] == ':'
}

// dates checks BR-TMP-7: every udt:DateTimeString, qdt:DateTimeString and
// udt:DateString has the format 102 and a value of 8 digits.
func (k *checker) dates() {
	k.root.Walk(func(n *xmltree.Node) bool {
		if !(n.Name == "DateTimeString" && (n.Space == rules.NSUdt || n.Space == rules.NSQdt) ||
			n.Name == "DateString" && n.Space == rules.NSUdt) {
			return true
		}
		format := norm(attr(n, "format"))
		value := norm(n.Text)
		if format != "102" || !digits8(value) {
			k.fail("BR-TMP-7", rules.Warning, n, "[BR-TMP-7] Dates in UN/CEFACT CII must have the attribute format=\"102\" and the form YYYYMMDD; format: %q, value: %q.", format, value)
		}
		return true
	})
}

// digits8 implements matches(s, '^\d{8}$').
func digits8(s string) bool {
	n := 0
	for _, r := range s {
		if !isDigit(r) {
			return false
		}
		n++
	}
	return n == 8
}

// paymentMeans checks BR-DE-19, BR-DE-20 and BR-DE-23 to BR-DE-25 on each
// payment means, according to its type code (BT-81).
func (k *checker) paymentMeans() {
	r := k.root
	directDebit := exists(r, pMandate) || exists(r, pCreditorRef)
	for pm := range all(r, pPaymentMean) {
		code := nsv(first(pm, "TypeCode"))
		payerIBAN := exists(pm, pPayerIBAN)
		switch code {
		case "30", "58":
			if code == "58" {
				if iban := sv(first(pm, pPayeeAcct+"/IBANID")); !checkIBAN(iban) {
					k.fail("BR-DE-19", rules.Warning, orNode(first(pm, pPayeeAcct+"/IBANID"), pm), "[BR-DE-19] The Payment account identifier (BT-84) should contain a valid IBAN when the Payment means type code (BT-81) is 58 (SEPA credit transfer); value: %q.", iban)
				}
			}
			if !exists(pm, pPayeeAcct) {
				k.fail("BR-DE-23-a", rules.Error, pm, "[BR-DE-23-a] If the Payment means type code (BT-81) is a credit transfer code (30, 58), CREDIT TRANSFER (BG-17) must be transmitted; type code: %q.", code)
			}
			if exists(pm, pCard) || directDebit || payerIBAN {
				k.fail("BR-DE-23-b", rules.Error, pm, "[BR-DE-23-b] If the Payment means type code (BT-81) is a credit transfer code (30, 58), PAYMENT CARD INFORMATION (BG-18) and DIRECT DEBIT (BG-19) must not be transmitted; type code: %q.", code)
			}
		case "48", "54", "55":
			if !exists(pm, pCard) {
				k.fail("BR-DE-24-a", rules.Error, pm, "[BR-DE-24-a] If the Payment means type code (BT-81) is a payment card code (48, 54, 55), PAYMENT CARD INFORMATION (BG-18) must be transmitted; type code: %q.", code)
			}
			if exists(pm, pPayeeAcct) || directDebit || payerIBAN {
				k.fail("BR-DE-24-b", rules.Error, pm, "[BR-DE-24-b] If the Payment means type code (BT-81) is a payment card code (48, 54, 55), CREDIT TRANSFER (BG-17) and DIRECT DEBIT (BG-19) must not be transmitted; type code: %q.", code)
			}
		case "59":
			if iban := sv(first(pm, pPayerIBAN)); !checkIBAN(iban) {
				k.fail("BR-DE-20", rules.Warning, orNode(first(pm, pPayerIBAN), pm), "[BR-DE-20] The Debited account identifier (BT-91) should contain a valid IBAN when the Payment means type code (BT-81) is 59 (SEPA direct debit); value: %q.", iban)
			}
			if !directDebit && !payerIBAN {
				k.fail("BR-DE-25-a", rules.Error, pm, "[BR-DE-25-a] If the Payment means type code (BT-81) is a direct debit code (59), DIRECT DEBIT (BG-19) must be transmitted.")
			}
			if exists(pm, pPayeeAcct) || exists(pm, "PayeeSpecifiedCreditorFinancialInstitution") || exists(pm, "PayerSpecifiedDebtorFinancialInstitution") || exists(pm, pCard) {
				k.fail("BR-DE-25-b", rules.Error, pm, "[BR-DE-25-b] If the Payment means type code (BT-81) is a direct debit code (59), CREDIT TRANSFER (BG-17) and PAYMENT CARD INFORMATION (BG-18) must not be transmitted.")
			}
		}
	}
}

// checkIBAN implements xr:checkIBAN: once white space is removed, the value
// matches ^[A-Z]{2}[0-9]{2}[a-zA-Z0-9]{0,30}$ and the ISO 13616 checksum
// (mod 97) is 1. As in the reference, a lower case letter after the check
// digits counts as its code point minus 55.
func checkIBAN(iban string) bool {
	b := make([]byte, 0, len(iban))
	for i := 0; i < len(iban); i++ {
		if !isSpace(iban[i]) {
			b = append(b, iban[i])
		}
	}
	if len(b) < 4 || len(b) > 34 {
		return false
	}
	for i, c := range b {
		upper, digit := c >= 'A' && c <= 'Z', c >= '0' && c <= '9'
		switch {
		case i < 2 && !upper, i >= 2 && i < 4 && !digit, i >= 4 && !upper && !digit && !(c >= 'a' && c <= 'z'):
			return false
		}
	}
	// The country code and check digits move to the end.
	rem := 0
	for i := range b {
		c := b[(i+4)%len(b)]
		if c > 64 {
			rem = (rem*100 + int(c) - 55) % 97
		} else {
			rem = (rem*10 + int(c) - 48) % 97
		}
	}
	return rem == 1
}

// vatRates checks BR-DE-14: every VAT breakdown (BG-23) has a rate (BT-119).
func (k *checker) vatRates() {
	for tax := range all(k.root, pHeaderTax) {
		if !nonBlank(tax, "RateApplicablePercent") {
			k.fail("BR-DE-14", rules.Error, tax, "[BR-DE-14] The element VAT category rate (BT-119) must be transmitted; value: %q.", sv(first(tax, "RateApplicablePercent")))
		}
	}
}

// deliveryDates checks BR-DE-TMP-32: the invoice gives an actual delivery
// date (BT-72), an invoicing period (BG-14) or a period on every line (BG-26).
func (k *checker) deliveryDates() {
	for tx := range all(k.root, pTransaction) {
		if exists(tx, "ApplicableHeaderTradeDelivery/ActualDeliverySupplyChainEvent/OccurrenceDateTime") ||
			exists(tx, "ApplicableHeaderTradeSettlement/BillingSpecifiedPeriod") {
			continue
		}
		every := true
		for line := range all(tx, "IncludedSupplyChainTradeLineItem") {
			if !exists(line, "SpecifiedLineTradeSettlement/BillingSpecifiedPeriod") {
				every = false
				break
			}
		}
		if !every {
			k.fail("BR-DE-TMP-32", rules.Info, tx, "[BR-DE-TMP-32] An invoice should give the delivery or service date with either the Actual delivery date (BT-72), the Invoicing period (BG-14) or an Invoice line period (BG-26) on every line.")
		}
	}
}

// baseQuantities checks BR-TMP-3: when both prices of a line give a base
// quantity (BT-149), the quantities are equal, and so are their unit codes
// (BT-150) when both prices give one.
func (k *checker) baseQuantities() {
	const gross = "SpecifiedLineTradeAgreement/GrossPriceProductTradePrice/BasisQuantity"
	const net = "SpecifiedLineTradeAgreement/NetPriceProductTradePrice/BasisQuantity"
	for line := range all(k.root, pLines) {
		g, n := first(line, gross), first(line, net)
		if g == nil || n == nil {
			continue
		}
		ok := false
		for gq := range all(line, gross) {
			for nq := range all(line, net) {
				if sv(gq) == sv(nq) {
					ok = true
				}
			}
		}
		if ok && hasAttr(line, net, "unitCode") && hasAttr(line, gross, "unitCode") {
			ok = false
			for gq := range all(line, gross) {
				for nq := range all(line, net) {
					gu, gok := gq.LookupAttr("unitCode")
					nu, nok := nq.LookupAttr("unitCode")
					if gok && nok && gu == nu {
						ok = true
					}
				}
			}
		}
		if !ok {
			k.fail("BR-TMP-3", rules.Error, line, "[BR-TMP-3] If the Item price base quantity (BT-149) is given in both GrossPriceProductTradePrice and NetPriceProductTradePrice, the values must be identical, and so must the unit of measure codes (BT-150) when both are given; gross: %q (unit %q), net: %q (unit %q).", sv(g), attr(g, "unitCode"), sv(n), attr(n, "unitCode"))
		}
	}
}

// hasAttr reports whether an element matching path has the attribute name.
func hasAttr(n *xmltree.Node, path, name string) bool {
	for c := range all(n, path) {
		if _, ok := c.LookupAttr(name); ok {
			return true
		}
	}
	return false
}
