package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ritoon/zerechnung/cii"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// ZUGFeRD 1.0 invoices get a minimal structural check. ZUGFeRD 1.0 predates
// EN 16931 and CII D16B: the rules of EN 16931, of Factur-X and the national
// rules, written for CII D16B invoices that follow EN 16931, would give
// false findings on a valid ZUGFeRD 1.0 invoice, and its own XML schema is
// not part of this package. The check makes sure that the document is a
// ZUGFeRD 1.0 invoice that can be read: a known profile, readable values and
// the data that identifies an invoice.

// Rules of the ZUGFeRD 1.0 check.
const (
	ruleZF1Profile   = "ZF1-01"
	ruleZF1Value     = "ZF1-02"
	ruleZF1Mandatory = "ZF1-03"
	ruleZF1Scope     = "ZF1-04"
)

// zugferd1Catalog lists the rules of the ZUGFeRD 1.0 check, part of the
// Schema rule set, except ZF1-04, reported whatever the rule sets.
var zugferd1Catalog = []RuleInfo{
	{ID: ruleZF1Profile, Severity: Error, Description: "The specification identifier of a ZUGFeRD 1.0 invoice is urn:ferd:CrossIndustryDocument:invoice:1p0:basic, comfort or extended (or a release candidate identifier, urn:ferd:invoice:rc:…)."},
	{ID: ruleZF1Value, Severity: Error, Description: "Every amount, quantity, percentage, date and indicator of a ZUGFeRD 1.0 invoice can be read."},
	{ID: ruleZF1Mandatory, Severity: Error, Description: "A ZUGFeRD 1.0 invoice has an invoice number, a type code, an issue date, a seller name, a buyer name, a currency code and a grand total amount."},
	{ID: ruleZF1Scope, Severity: Warning, Description: "ZUGFeRD 1.0 predates EN 16931: its invoices only get the structural check ZF1-01 to ZF1-03, not the EN 16931, Factur-X or national rules, and are not EN 16931 invoices."},
}

// zf1Mandatory lists the data checked by ZF1-03, by its path from the root
// of a ZUGFeRD 1.0 invoice.
var zf1Mandatory = []struct{ path, name string }{
	{"HeaderExchangedDocument/ID", "invoice number"},
	{"HeaderExchangedDocument/TypeCode", "type code"},
	{"HeaderExchangedDocument/IssueDateTime", "issue date"},
	{"SpecifiedSupplyChainTradeTransaction/ApplicableSupplyChainTradeAgreement/SellerTradeParty/Name", "seller name"},
	{"SpecifiedSupplyChainTradeTransaction/ApplicableSupplyChainTradeAgreement/BuyerTradeParty/Name", "buyer name"},
	{"SpecifiedSupplyChainTradeTransaction/ApplicableSupplyChainTradeSettlement/InvoiceCurrencyCode", "currency code"},
	{"SpecifiedSupplyChainTradeTransaction/ApplicableSupplyChainTradeSettlement/SpecifiedTradeSettlementMonetarySummation/GrandTotalAmount", "grand total amount"},
}

// checkZUGFeRD1 checks root, the root element of a ZUGFeRD 1.0 invoice (see
// cii.IsZUGFeRD1). The profile of the report is the one matched to its
// specification identifier, unless opts.Profile is set.
func checkZUGFeRD1(root *xmltree.Node, opts *Options, r *Report) {
	fail := func(rule string, sev Severity, n *xmltree.Node, msg string) {
		r.Add(Finding{Rule: rule, Severity: sev, Source: SourceSyntax, Path: n.Path(), Line: n.Line, Message: msg})
	}
	idPath := "SpecifiedExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/ID"
	id := strings.TrimSpace(root.Value(idPath))
	r.Profile = opts.Profile
	if r.Profile == invoice.ProfileUnknown {
		r.Profile = invoice.ProfileFromZUGFeRD1URN(id)
	}
	set := opts.Rules
	if set == 0 {
		set = Default
	}
	if set&Schema != 0 {
		switch {
		case id == "":
			fail(ruleZF1Profile, Error, deepest(root, idPath), "the specification identifier is missing: the ZUGFeRD 1.0 profile cannot be determined")
		case invoice.ProfileFromZUGFeRD1URN(id) == invoice.ProfileUnknown:
			fail(ruleZF1Profile, Error, root.Find(idPath), fmt.Sprintf("the specification identifier %q is not a ZUGFeRD 1.0 invoice identifier (%s, %s or %s)",
				id, invoice.URNZUGFeRD1Basic, invoice.URNZUGFeRD1Comfort, invoice.URNZUGFeRD1Extended))
		}
		_, issues := cii.ParseTree(root)
		for _, is := range issues {
			r.Add(Finding{Rule: ruleZF1Value, Severity: Error, Source: SourceSyntax, Path: is.Path, Line: is.Line, Message: is.Message})
		}
		for _, m := range zf1Mandatory {
			if n := root.Find(m.path); n == nil || !hasValue(n) {
				fail(ruleZF1Mandatory, Error, deepest(root, m.path), "the "+m.name+" is missing")
			}
		}
	}
	fail(ruleZF1Scope, Warning, root, "ZUGFeRD 1.0 predates EN 16931: only the structure of the invoice is checked (ZF1-01 to ZF1-03), "+
		"not the EN 16931, Factur-X or national rules. It is not an EN 16931 invoice: the BMF accepts ZUGFeRD from version 2.0.1 as an e-invoice, "+
		"and the French reform requires a format that follows EN 16931.")
}

// deepest returns the deepest element of root on path: the element itself,
// or the nearest ancestor present when it is missing.
func deepest(root *xmltree.Node, path string) *xmltree.Node {
	n := root
	for seg := range strings.SplitSeq(path, "/") {
		c := n.Elem(seg)
		if c == nil {
			break
		}
		n = c
	}
	return n
}

// hasValue reports whether n or one of its descendants has text.
func hasValue(n *xmltree.Node) bool {
	if strings.TrimSpace(n.Text) != "" {
		return true
	}
	return slices.ContainsFunc(n.Children, hasValue)
}
