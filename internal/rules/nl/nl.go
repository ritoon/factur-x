// Package nl implements the Dutch rules of NLCIUS, the CIUS of EN 16931
// for the Netherlands, as published by the Dutch Peppol Authority (NPa) in
// its validation artefacts (github.com/peppolautoriteit-nl/validation,
// release of 2026-05-21): for the CII syntax, the pattern nlcius of
// NLCIUS-CII-validation.sch (NLCIUS-CII 1.0.3.13); for the UBL syntax
// (SI-UBL 2.0), the pattern nlcius of si-ubl-2.0-nlcius.sch and the pattern
// SI-UBL-VERSION of si-ubl-2.0.sch (SI-UBL 2.0.3.13); for a UBL invoice that
// uses the G-account extension, the pattern g-account-extension of
// si-ubl-2.0-ext-gaccount.sch (version 1.0.2.13) instead of SI-UBL-VERSION.
// The rules of EN 16931 that the validators include are not part of this
// package (see package ubl for the syntax rules that the G-account artefacts
// leave out).
//
// The package reimplements the assertions of these patterns in Go, with
// their identifiers and flags: Catalog lists the 32 assertions it applies to
// a CII invoice, UBLCatalog the 43 it applies to a UBL invoice. The flags map
// to the severities of package rules: fatal to Error, warning to Warning.
//
// # Conditions
//
// The schematrons decide themselves which invoices their rules apply to,
// through two variables, and so does Check:
//
//   - $si: the specification identifier BT-24 is the NLCIUS one, alone or
//     followed by the G-account extension. In CII, normalize-space of the
//     first ram:GuidelineSpecifiedDocumentContextParameter/ram:ID must equal
//     one of the two identifiers; in UBL, normalize-space of
//     cbc:CustomizationID must contain '#compliant#urn:fdc:nen.nl:nlcius:v1.0'
//     or '#conformant#urn:fdc:nen.nl:gaccount:v1.0'.
//   - $s: $si, and the supplier is in the Netherlands (BT-40, upper-cased
//     after normalize-space, is NL).
//
// Most rules apply with $s; BR-NL-13, and BR-NL-7 in CII, apply with $si.
// The checks of the empty elements (empty-element-check, SI-UBL-2) and of
// the UBL pattern SI-UBL-VERSION (BII2-T10-R001, SI-V20-INV-R000) have no
// condition: they apply to any invoice Check runs on.
//
// # G-account extension
//
// A validator picks the artefacts of the G-account extension by the
// specification identifier; so does Check, for a UBL invoice whose first
// cbc:CustomizationID contains '#conformant#urn:fdc:nen.nl:gaccount:v1.0'
// (rules.Doc.GAccount). The pattern g-account-extension then replaces
// SI-UBL-VERSION, which these artefacts do not include, and its rules apply
// with no condition: BR-GA-0 and its own SI-V20-INV-R000 to every
// cbc:CustomizationID (BR-GA-0 fails for another one, such as one inside the
// UBL extensions), BR-GA-1 to BR-GA-7 to an Invoice and its payment means
// and terms; a CreditNote only gets the first two. BII2-T10-R001 cannot fail
// on such an invoice. The pattern nlcius applies as for any SI-UBL 2.0
// invoice. The NPa publishes no artefacts of the extension for NLCIUS-CII: a
// CII invoice that declares it gets the NLCIUS rules only.
//
// # Rule order
//
// In a schematron pattern, an element is checked by the first rule whose
// context it matches, and by no other. Check visits the elements in
// document order and applies the same choice, which makes some assertions
// of the artefacts unreachable; they are not implemented, as no reference
// validator reports them:
//
//   - CII: BR-NL-9 has the context of BR-NL-7 (/*/rsm:ExchangedDocument/
//     ram:TypeCode) and BR-NL-31 the context of BR-NL-12
//     (ram:SpecifiedTradeSettlementPaymentMeans).
//   - UBL: the context of BR-NL-32-1, cac:AllowanceCharge/
//     cbc:AllowanceChargeReasonCode, also matches the reason codes of the
//     line allowances and charges: BR-NL-32-2 and BR-NL-32-3 never apply,
//     nor do the three assertions with the message of BR-NL-34, whose rules
//     repeat the contexts of BR-NL-32-1 to BR-NL-32-3.
//   - An empty element matched by an earlier rule is not reported as empty:
//     an empty cbc:TaxPointDate of a Dutch supplier gives BR-NL-20 only.
//
// # Faithfulness notes
//
//   - In CII, BR-NL-1 applies only when the seller has a
//     ram:SpecifiedLegalOrganization, and the test of BR-NL-10 requires a
//     non blank legal registration identifier of the buyer whatever its
//     country: the condition on the Dutch buyer only applies to the scheme.
//     In UBL, BR-NL-1 applies to every supplier party and BR-NL-10 to a
//     customer cac:PartyLegalEntity.
//   - In UBL, BR-NL-11 applies to an Invoice only; in CII, to every invoice
//     whose type code is not 381.
//   - The string comparisons are those of XPath on the raw values (no
//     trimming), except where the schematron normalizes. In CII, the type
//     codes of BR-NL-7, BR-NL-11 and BR-NL-12 are compared as numbers: '0380'
//     equals 380.
//   - Where the reference would stop on a dynamic error (a value that does
//     not cast to xs:decimal or xs:double, several values given to
//     normalize-space or to a cast), the assertion fails; where it would
//     read the first of several values of a variable, the first value in
//     document order is used.
//   - The rule SI-INV-R000 of SI-UBL-VERSION (the root is an Invoice or a
//     CreditNote) always holds for the documents Check runs on.
//   - BR-GA-3 compares the amounts as decimals, without rounding. BR-GA-6
//     and BR-GA-7 read the text nodes of a payment means identifier
//     (text()) as one, the character data of the element.
//
// Findings are located at the element of the rule context.
package nl

import (
	"strings"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// Specification identifiers (BT-24).
const (
	// CIUSID identifies an NLCIUS invoice.
	CIUSID = invoice.URNNLCIUS
	// GAccountID identifies an NLCIUS invoice with the G-account extension.
	GAccountID = CIUSID + rules.GAccountExtension
)

// guard is the condition of a rule context.
type guard uint8

const (
	always        guard = iota // no condition
	nlcius                     // $si: BT-24 is NLCIUS
	dutchSupplier              // $s: $si and a supplier in the Netherlands
)

// rule is a schematron rule: its contexts, its condition and the check of
// its assertions on a matching element.
type rule struct {
	contexts []pattern
	guard    guard
	check    func(k *checker, n *xmltree.Node)
}

// checker wraps rules.Checker with the variables of the schematron.
type checker struct {
	*rules.Checker
	root *xmltree.Node
	si   bool // $si
	s    bool // $s
}

// Check runs the NLCIUS rules on c.Doc and records the findings in c.Report:
// the rules of the UBL artefacts on a UBL invoice, those of the CII
// artefacts otherwise.
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
	spec := nsv(first(descendants(k.root, "GuidelineSpecifiedDocumentContextParameter/ID")))
	k.si = spec == CIUSID || spec == GAccountID
	country := "XX"
	if first(descendants(k.root, "ApplicableHeaderTradeAgreement/SellerTradeParty/PostalTradeAddress/CountryID")) != nil {
		country = strings.ToUpper(nsv(first(descendants(k.root, "SellerTradeParty/PostalTradeAddress/CountryID"))))
	}
	k.s = k.si && country == "NL"
	k.run(ciiRules, "empty-element-check")
}

// holds reports whether the condition g is true.
func (k *checker) holds(g guard) bool {
	switch g {
	case nlcius:
		return k.si
	case dutchSupplier:
		return k.s
	}
	return true
}

// run applies a pattern: each element, in document order, is checked by the
// first rule whose context it matches and whose condition holds; an element
// that no rule matches is reported under emptyID when it is empty.
func (k *checker) run(rs []rule, emptyID string) {
	k.root.Walk(func(n *xmltree.Node) bool {
		for i := range rs {
			r := &rs[i]
			if k.holds(r.guard) && r.matches(n) {
				r.check(k, n)
				return true
			}
		}
		if emptyID != "" && emptyLeaf(n) {
			k.Fail(emptyID, rules.Warning, n, "Document should not contain empty elements.")
		}
		return true
	})
}

// matches reports whether n matches a context of r.
func (r *rule) matches(n *xmltree.Node) bool {
	for _, p := range r.contexts {
		if p.match(n) {
			return true
		}
	}
	return false
}

// assert records a finding at n when ok is false.
func (k *checker) assert(ok bool, id string, sev rules.Severity, n *xmltree.Node, msg string) {
	if !ok {
		k.Fail(id, sev, n, "%s", msg)
	}
}

// discouraged returns the check of a rule whose assertion is false(): the
// element of the context is reported whenever it is present.
func discouraged(id, msg string) func(*checker, *xmltree.Node) {
	return func(k *checker, n *xmltree.Node) { k.Fail(id, rules.Warning, n, "%s", msg) }
}
