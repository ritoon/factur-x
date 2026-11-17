package de

import (
	"strings"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// Rules of CheckIdentifiers. The KoSIT schematron does not check the format
// of these identifiers, so the identifiers are specific to this package.
const (
	RuleLeitwegFormat = "DE-LEITWEG-01" // format of a Leitweg-ID
	RuleLeitwegKey    = "DE-LEITWEG-02" // check digits of a Leitweg-ID
	RuleGEBAFormat    = "DE-GEBA-01"    // format of a GEBA
	RuleGEBAKey       = "DE-GEBA-02"    // check digit of the W-IdNr of a GEBA
)

// IdentifierCatalog lists the checks of CheckIdentifiers.
var IdentifierCatalog = []rules.RuleInfo{
	{ID: RuleLeitwegFormat, Severity: rules.Warning, Description: "The Buyer reference (BT-10) and the electronic addresses (BT-34, BT-49) of scheme 0204 should have the format of a Leitweg-ID (Formatspezifikation Leitweg-ID 2.0.2): a coarse addressing of 2, 3, 5, 8, 9 or 12 digits starting with the code of a Land (01 to 16) or of the Federation (99), an optional fine addressing of up to 30 letters and digits, and two check digits, separated by '-'."},
	{ID: RuleLeitwegKey, Severity: rules.Warning, Description: "The check digits of a Leitweg-ID in the Buyer reference (BT-10) or in an electronic address (BT-34, BT-49) of scheme 0204 should be valid (ISO/IEC 7064 MOD 97-10)."},
	{ID: RuleGEBAFormat, Severity: rules.Warning, Description: "The electronic addresses (BT-34, BT-49) of scheme 0246 should have the format of a GEBA (GEBA Formatspezifikation 1.0.1): DE and the 9 digits of the W-IdNr, optionally '-' and a distinguishing feature of 5 digits, optionally '.' and a sub-address of 1 to 8 letters or digits."},
	{ID: RuleGEBAKey, Severity: rules.Warning, Description: "The check digit of the W-IdNr of a GEBA in an electronic address (BT-34, BT-49) of scheme 0246 should be valid (ISO 7064 MOD 11,10, as for the USt-IdNr)."},
}

// CheckIdentifiers checks the German routing identifiers of c.Doc, whose
// format the KoSIT schematron does not check, and records the findings in
// c.Report:
//
//   - the Leitweg-ID (Formatspezifikation Leitweg-ID 2.0.2), in the Buyer
//     reference (BT-10), which carries the Leitweg-ID of a public buyer in an
//     XRechnung invoice, and in the electronic addresses (BT-34, BT-49) of
//     scheme 0204: DE-LEITWEG-01 for the format, DE-LEITWEG-02 for the check
//     digits;
//   - the GEBA (GEBA Formatspezifikation 1.0.1), in the electronic addresses
//     of scheme 0246: DE-GEBA-01 for the format, DE-GEBA-02 for the check
//     digit of the W-IdNr.
//
// The findings are warnings. The check digits are verified only on a value
// in the right format. Values are trimmed; a blank value is not checked
// (BR-DE-15 requires BT-10, EN 16931 a scheme for an electronic address).
// For a UBL invoice, the check runs on the CII view of the document and the
// findings are located in the UBL document.
func CheckIdentifiers(c *rules.Checker) {
	if c == nil || c.Doc == nil || c.Doc.Root == nil {
		return
	}
	agr := c.Doc.Agreement
	for _, n := range agr.Elems("BuyerReference") {
		if v := n.Trim(); v != "" {
			checkLeitweg(c, n, v, "Buyer reference (BT-10)")
		}
	}
	for _, a := range []struct{ path, name string }{
		{"SellerTradeParty/URIUniversalCommunication/URIID", "Seller electronic address (BT-34)"},
		{"BuyerTradeParty/URIUniversalCommunication/URIID", "Buyer electronic address (BT-49)"},
	} {
		for _, n := range agr.FindAll(a.path) {
			v := n.Trim()
			if v == "" {
				continue
			}
			switch scheme := strings.TrimSpace(n.Attr("schemeID")); scheme {
			case invoice.SchemeLeitwegID:
				checkLeitweg(c, n, v, a.name+" of scheme "+scheme)
			case invoice.SchemeGEBA:
				checkGEBA(c, n, v, a.name+" of scheme "+scheme)
			}
		}
	}
}

// checkLeitweg checks that the value v of n, named name in the messages, is
// a Leitweg-ID.
func checkLeitweg(c *rules.Checker, n *xmltree.Node, v, name string) {
	if why := leitwegFormat(v); why != "" {
		c.Fail(RuleLeitwegFormat, rules.Warning, n, "[%s] The %s should be a Leitweg-ID (Formatspezifikation Leitweg-ID 2.0.2): %s; value: %q.", RuleLeitwegFormat, name, why, v)
		return
	}
	addr, key := leitwegParts(v)
	if want := leitwegCheckDigits(addr); key != want {
		c.Fail(RuleLeitwegKey, rules.Warning, n, "[%s] The check digits of the Leitweg-ID in the %s should be %s, not %s (ISO/IEC 7064 MOD 97-10); value: %q.", RuleLeitwegKey, name, want, key, v)
	}
}

// checkGEBA checks that the value v of n, named name in the messages, is a
// GEBA.
func checkGEBA(c *rules.Checker, n *xmltree.Node, v, name string) {
	if why := gebaFormat(v); why != "" {
		c.Fail(RuleGEBAFormat, rules.Warning, n, "[%s] The %s should be a GEBA (GEBA Formatspezifikation 1.0.1): %s; value: %q.", RuleGEBAFormat, name, why, v)
		return
	}
	if want := widnrCheckDigit(v[2:10]); v[10] != want {
		c.Fail(RuleGEBAKey, rules.Warning, n, "[%s] The check digit of the W-IdNr %s in the %s should be %c, not %c (ISO 7064 MOD 11,10, as for the USt-IdNr); value: %q.", RuleGEBAKey, v[:11], name, want, v[10], v)
	}
}
