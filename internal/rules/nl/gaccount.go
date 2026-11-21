package nl

import (
	"math/big"
	"strings"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// Rules of the G-account extension of SI-UBL 2.0: the pattern
// g-account-extension of si-ubl-2.0-ext-gaccount.sch (version 1.0.2.13), in
// the order of the schematron. They apply to a UBL invoice that declares the
// extension (see rules.Doc.GAccount), with no other condition.

// Messages of the G-account assertions.
const (
	msgGA0 = "[BR-GA-0] When using the G-account extension, the CustomizationID must specify this."
	msgGA1 = "[BR-GA-1] The number of Payment Terms (NL-GA-01) in each invoice MUST be two."
	msgGA2 = "[BR-GA-2] The number of Payment Instructions (BG-16) in each invoice MUST be two."
	msgGA3 = "[BR-GA-3] Sum of Amount due for payment (BT-115) = ∑ Payment Amount (NL-GA-03)"
	msgGA7 = "[BR-GA-7] There MUST be a Payment Means identifier (NL-GA-04) with a value equal to 'GACCOUNT'"
	msgGA4 = "[BR-GA-4] Each Payment Terms (NL-GA-01) MUST include a Payment Means reference (NL-GA-02)"
	msgGA5 = "[BR-GA-5] Each Payment Instructions (BG-16) MUST include a Payment Means identifier (NL-GA-04)"
	msgGA6 = "[BR-GA-6] The value of each Payment Means reference (NL-GA-02) MUST correspond with one and only one Payment means identifier (NL-GA-04)"
)

// gaccountRules are the rules of the pattern g-account-extension. The first
// one repeats the version check of the pattern SI-UBL-VERSION, which the
// artefacts of the extension do not include.
var gaccountRules = []rule{
	{ubl("cbc:CustomizationID"), always, func(k *checker, n *xmltree.Node) {
		k.assert(strings.HasPrefix(nsv(n), CIUSID), "SI-V20-INV-R000", rules.Error, n, msgVersion)
		k.assert(strings.Contains(sv(n), rules.GAccountExtension), "BR-GA-0", rules.Error, n, msgGA0)
	}},
	{ubl("/ubl:Invoice"), always, func(k *checker, n *xmltree.Node) {
		k.assert(len(list(all(n, "PaymentTerms"))) == 2, "BR-GA-1", rules.Error, n, msgGA1)
		k.assert(len(list(all(n, "PaymentMeans"))) == 2, "BR-GA-2", rules.Error, n, msgGA2)
		k.assert(gaccountSumOK(n), "BR-GA-3", rules.Error, n, msgGA3)
		gaccount := 0
		for id := range all(n, "PaymentMeans/ID") {
			if id.Text == "GACCOUNT" {
				gaccount++
			}
		}
		k.assert(gaccount == 1, "BR-GA-7", rules.Error, n, msgGA7)
	}},
	{ubl("/ubl:Invoice/cac:PaymentTerms"), always, func(k *checker, n *xmltree.Node) {
		k.assert(len(list(all(n, "PaymentMeansID"))) == 1, "BR-GA-4", rules.Error, n, msgGA4)
	}},
	{ubl("/ubl:Invoice/cac:PaymentMeans"), always, func(k *checker, n *xmltree.Node) {
		k.assert(len(list(all(n, "ID"))) == 1, "BR-GA-5", rules.Error, n, msgGA5)
	}},
	{ubl("/ubl:Invoice/cac:PaymentTerms/cbc:PaymentMeansID"), always, func(k *checker, n *xmltree.Node) {
		// . = $payment-means-ids, the text nodes of
		// /ubl:Invoice/cac:PaymentMeans/cbc:ID: an empty identifier has none.
		ref, found := sv(n), false
		for id := range all(k.root, "PaymentMeans/ID") {
			found = found || id.Text != "" && id.Text == ref
		}
		k.assert(found, "BR-GA-6", rules.Error, n, msgGA6)
	}},
}

// gaccountSumOK evaluates the test of BR-GA-3 on /ubl:Invoice:
// cac:LegalMonetaryTotal/xs:decimal(cbc:PayableAmount) =
// sum(cac:PaymentTerms/xs:decimal(cbc:Amount)). The amounts are compared as
// decimals, without rounding. A value that does not cast to xs:decimal, or
// several values given to a cast, make the test false.
func gaccountSumOK(n *xmltree.Node) bool {
	sum := new(big.Rat)
	for terms := range all(n, "PaymentTerms") {
		v, ok := decimalOf(list(all(terms, "Amount")))
		if !ok {
			return false
		}
		if v != nil {
			sum.Add(sum, v)
		}
	}
	eq := false
	for total := range all(n, "LegalMonetaryTotal") {
		v, ok := decimalOf(list(all(total, "PayableAmount")))
		if !ok {
			return false
		}
		eq = eq || v != nil && v.Cmp(sum) == 0
	}
	return eq
}

// decimalOf casts the value of nodes to xs:decimal: nil for no node. ok is
// false when there are several nodes or when the value does not cast.
func decimalOf(nodes []*xmltree.Node) (v *big.Rat, ok bool) {
	switch len(nodes) {
	case 0:
		return nil, true
	case 1:
		s := sv(nodes[0])
		if _, ok := decimalSign(s); !ok {
			return nil, false
		}
		return new(big.Rat).SetString(trimXML(s))
	}
	return nil, false
}
