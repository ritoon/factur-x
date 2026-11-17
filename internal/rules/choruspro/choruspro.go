// Package choruspro checks French B2G invoices, addressed to a public buyer
// through Chorus Pro, for the data Chorus Pro needs to accept and route them.
//
// The rules come from the external specifications of Chorus Pro: the EDI
// appendix V4.23 (management rules G1.05, G2.06, G2.07, G2.16, G2.17,
// G2.19, G2.29, G3.01, G3.03, G3.04, and S2.04, S2.13 for the CII and
// Factur-X formats), its error messages appendix V3.02, and the Chorus Pro
// file of the DGFiP external specifications v3.1 (invoice number limited to
// 20 characters in B2G after the reform). Each message gives the Chorus Pro
// error code of the rejection it anticipates.
//
// No official schematron covers these checks: the identifiers (CPRO-*) are
// specific to this package. Some requirements depend on the settings of the
// public structure in the Chorus Pro directory (service code or engagement
// number required); the caller gives them in Settings. What Chorus Pro
// checks against its directory (the SIRET and the service code exist, the
// structure is active) cannot be checked offline.
//
// # Buyer identifier
//
// The public buyer is identified by the SIRET of the structure. The 2026
// reform carries it in the buyer identifier BT-46 with scheme 0009, the
// legal registration identifier BT-47 holding the SIREN (scheme 0002). The
// historical Chorus Pro mapping of Factur-X carries the SIRET in BT-47 with
// scheme 0002, or without scheme (rules S2.13 and G2.06). Both are accepted:
// a BT-46 with scheme 0009, or a BT-47 of 14 characters with scheme 0009,
// 0002 or none.
package choruspro

import (
	"unicode"
	"unicode/utf8"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// Settings are the settings of the public structure in the Chorus Pro
// directory that change the checks.
type Settings struct {
	ServiceCodeRequired bool // the structure requires a service code (BT-10)
	EngagementRequired  bool // the structure requires an engagement number (BT-13, or the contract reference BT-12)
}

// Rule identifiers.
const (
	RuleBuyerSIRET        = "CPRO-01" // the public buyer is identified by its SIRET
	RuleSIRETKey          = "CPRO-02" // the buyer SIRET has 14 digits and a valid key
	RuleServiceCode       = "CPRO-03" // the service code (BT-10) is given when required
	RuleServiceCodeFormat = "CPRO-04" // the service code has at most 100 allowed characters
	RuleEngagement        = "CPRO-05" // the engagement number (BT-13) is given when required
	RuleReferenceLength   = "CPRO-06" // BT-13 and BT-12 have at most 50 characters
	RuleTypeCode          = "CPRO-07" // the document type (BT-3) is not 751
	RuleInvoiceNumber     = "CPRO-08" // BT-1 and BT-25 have at most 20 allowed characters
)

// Catalog lists the rules of Check.
var Catalog = []rules.RuleInfo{
	{ID: RuleBuyerSIRET, Severity: rules.Error, Description: "La structure publique destinataire est identifiée par son SIRET : BT-46 avec le schéma 0009, ou BT-47 de 14 caractères (format historique de Chorus Pro)."},
	{ID: RuleSIRETKey, Severity: rules.Error, Description: "Le SIRET de la structure publique a 14 chiffres et une clé de Luhn valide (pour La Poste, une somme des chiffres multiple de 5)."},
	{ID: RuleServiceCode, Severity: rules.Error, Description: "Le code service (BT-10) est présent quand la structure publique l'exige."},
	{ID: RuleServiceCodeFormat, Severity: rules.Error, Description: "Le code service (BT-10) a au plus 100 caractères : lettres, chiffres, espace, « _ » et « - »."},
	{ID: RuleEngagement, Severity: rules.Error, Description: "Le numéro d'engagement (BT-13), ou à défaut la référence du marché (BT-12), est présent quand la structure publique l'exige."},
	{ID: RuleReferenceLength, Severity: rules.Error, Description: "Le numéro d'engagement (BT-13) et la référence du marché (BT-12) ont au plus 50 caractères."},
	{ID: RuleTypeCode, Severity: rules.Error, Description: "Le type de document (BT-3) n'est pas 751, que Chorus Pro n'admet pas."},
	{ID: RuleInvoiceNumber, Severity: rules.Error, Description: "Le numéro de facture (BT-1) et celui de la facture d'origine (BT-25) ont au plus 20 caractères : lettres et chiffres ASCII, espace, « - », « + », « _ » et « / »."},
}

// Limits of Chorus Pro.
const (
	maxServiceCode   = 100 // G2.19
	maxReference     = 50  // G3.03, G3.04
	maxInvoiceNumber = 20  // G1.05
)

// Paths from ram:ApplicableHeaderTradeAgreement.
const (
	pBuyer      = "BuyerTradeParty"
	pBuyerRef   = "BuyerReference"
	pEngagement = "BuyerOrderReferencedDocument/IssuerAssignedID"
	pContract   = "ContractReferencedDocument/IssuerAssignedID"
)

// Check runs the Chorus Pro rules on c.Doc and records the findings in
// c.Report.
func Check(c *rules.Checker, s Settings) {
	if c == nil || c.Doc == nil || c.Doc.Root == nil {
		return
	}
	d := c.Doc
	or := func(n, alt *xmltree.Node) *xmltree.Node {
		switch {
		case n != nil:
			return n
		case alt != nil:
			return alt
		}
		return d.Root
	}

	// CPRO-08: invoice numbers.
	numbers := []*xmltree.Node{d.Header.Elem("ID")}
	numbers = append(numbers, d.Settlement.FindAll("InvoiceReferencedDocument/IssuerAssignedID")...)
	for i, n := range numbers {
		if n == nil {
			continue
		}
		if v := n.Trim(); !validInvoiceNumber(v) {
			bt := "BT-1"
			if i > 0 {
				bt = "BT-25"
			}
			c.Fail(RuleInvoiceNumber, rules.Error, n, "[%s] Chorus Pro limite le numéro de facture (%s) à 20 caractères : lettres et chiffres ASCII, espace, « - », « + », « _ » et « / » (rejet FOR-049) ; valeur : %q.", RuleInvoiceNumber, bt, v)
		}
	}

	// CPRO-07: document type.
	if tc := d.Header.Elem("TypeCode"); tc != nil && tc.Trim() == "751" {
		c.Fail(RuleTypeCode, rules.Error, tc, "[%s] Chorus Pro n'admet pas le type de document 751 (information de facture à des fins comptables), qui n'est pas une facture : utiliser 380, 384, 386 ou 389 pour une facture, 381 ou 261 pour un avoir (rejet COD-001).", RuleTypeCode)
	}

	// CPRO-01, CPRO-02: buyer SIRET.
	buyer := d.Agreement.Elem(pBuyer)
	sirets := buyerSIRETs(buyer)
	if len(sirets) == 0 {
		c.Fail(RuleBuyerSIRET, rules.Error, or(buyer, d.Agreement), "[%s] La structure publique destinataire doit être identifiée par son SIRET (rejet COD-021) : identifiant de l'acheteur (BT-46, ram:GlobalID) avec le schemeID 0009, ou, dans le format historique de Chorus Pro, immatriculation légale (BT-47, ram:SpecifiedLegalOrganization/ram:ID) de 14 caractères.", RuleBuyerSIRET)
	}
	for _, n := range sirets {
		if v := n.Trim(); !ValidSIRET(v) {
			c.Fail(RuleSIRETKey, rules.Error, n, "[%s] Le SIRET de la structure publique doit compter 14 chiffres et avoir une clé de Luhn valide ; valeur : %q.", RuleSIRETKey, v)
		}
	}

	// CPRO-03, CPRO-04: service code.
	ref := d.Agreement.Elem(pBuyerRef)
	switch v := ref.Trim(); {
	case v == "":
		if s.ServiceCodeRequired {
			c.Fail(RuleServiceCode, rules.Error, or(ref, d.Agreement), "[%s] La structure publique exige un code service : la référence acheteur (BT-10, ram:BuyerReference) doit le porter (rejet COD-022).", RuleServiceCode)
		}
	case !validServiceCode(v):
		c.Fail(RuleServiceCodeFormat, rules.Error, ref, "[%s] Le code service (BT-10) doit avoir au plus 100 caractères, lettres, chiffres, espace, « _ » ou « - » (rejets FOR-009 et FOR-047) ; valeur : %q.", RuleServiceCodeFormat, v)
	}

	// CPRO-05, CPRO-06: engagement number and contract reference.
	engagement, contract := d.Agreement.Find(pEngagement), d.Agreement.Find(pContract)
	if s.EngagementRequired && engagement.Trim() == "" && contract.Trim() == "" {
		c.Fail(RuleEngagement, rules.Error, or(engagement, d.Agreement), "[%s] La structure publique exige un numéro d'engagement : la référence de commande (BT-13, ram:BuyerOrderReferencedDocument/ram:IssuerAssignedID), ou à défaut la référence du marché (BT-12), doit le porter (rejet COH-053).", RuleEngagement)
	}
	for _, r := range []struct {
		n      *xmltree.Node
		bt, fr string
	}{{engagement, "BT-13", "numéro d'engagement"}, {contract, "BT-12", "référence du marché"}} {
		if v := r.n.Trim(); utf8.RuneCountInString(v) > maxReference {
			c.Fail(RuleReferenceLength, rules.Error, r.n, "[%s] Chorus Pro limite le %s (%s) à 50 caractères (rejet FOR-013) ; longueur : %d.", RuleReferenceLength, r.fr, r.bt, utf8.RuneCountInString(v))
		}
	}
}

// buyerSIRETs returns the identifiers of the buyer that carry its SIRET:
// the global identifiers (BT-46) with scheme 0009, and the legal
// registration identifier (BT-47) when it has scheme 0009, or 14 characters
// with scheme 0002 or none (historical Chorus Pro mapping).
func buyerSIRETs(buyer *xmltree.Node) []*xmltree.Node {
	var out []*xmltree.Node
	for _, n := range buyer.Elems("GlobalID") {
		if n.Attr("schemeID") == "0009" {
			out = append(out, n)
		}
	}
	if n := buyer.Find("SpecifiedLegalOrganization/ID"); n != nil {
		switch n.Attr("schemeID") {
		case "0009":
			out = append(out, n)
		case "0002", "":
			if utf8.RuneCountInString(n.Trim()) == 14 {
				out = append(out, n)
			}
		}
	}
	return out
}

// validServiceCode implements G2.19 and G2.29: at most 100 characters,
// letters (accented letters included), digits, underscore, hyphen and space.
func validServiceCode(s string) bool {
	if utf8.RuneCountInString(s) > maxServiceCode {
		return false
	}
	for _, r := range s {
		if !(unicode.Is(unicode.Latin, r) || r >= '0' && r <= '9' || r == '_' || r == '-' || r == ' ') {
			return false
		}
	}
	return true
}

// validInvoiceNumber implements G1.05: at most 20 characters, ASCII letters
// and digits, space, hyphen, plus sign, underscore and slash.
func validInvoiceNumber(s string) bool {
	if len(s) > maxInvoiceNumber {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch b := s[i]; {
		case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		case b == ' ', b == '-', b == '+', b == '_', b == '/':
		default:
			return false
		}
	}
	return true
}
