package nl

import (
	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// Rules of the pattern nlcius of NLCIUS-CII-validation.sch, in the order of
// the schematron.

var ciiNS = map[string]string{
	"rsm": rules.NSRsm,
	"ram": rules.NSRam,
	"udt": rules.NSUdt,
	"qdt": rules.NSQdt,
}

// cii compiles CII contexts.
func cii(paths ...string) []pattern {
	out := make([]pattern, len(paths))
	for i, p := range paths {
		out[i] = compile(p, ciiNS)
	}
	return out
}

// Messages of the CII assertions.
const (
	msgCII1  = "[BR-NL-1] For suppliers in the Netherlands the supplier MUST provide either a KVK or OIN number for its legal entity identifier (ram:SellerTradeParty/ram:SpecifiedLegalOrganization/ram:ID/@schemeID with schemeID 0106 or 0190)"
	msgCII2  = "[BR-NL-2] For suppliers in the Netherlands, the invoice MUST contain either the buyer reference (ram:BuyerReference) or the order reference (ram:BuyerOrderReferencedDocument/ram:IssuerAssignedID)"
	msgCII3  = "[BR-NL-3] For suppliers in the Netherlands the supplier's address (ram:SellerTradeParty/PostalTradeAddres) MUST contain street name (ram:LineOne), city (ram:CityName) and postal zone (ram:PostcodeCode)"
	msgCII4  = "[BR-NL-4] For suppliers in the Netherlands, if the customer is in the Netherlands, the customer address (ram:BuyerTradeParty/PostalTradeAddres) MUST contain street name (ram:LineOne), city (ram:CityName) and postal zone (ram:PostcodeCode)"
	msgCII5  = "[BR-NL-5] For suppliers in the Netherlands, if the fiscal representative is in the Netherlands, the representative's address (ram:SellerTaxRepresentativeTradeParty) MUST contain street name (ram:LineOne), city (ram:CityName) and postal zone"
	msgCII7  = "[BR-NL-7] The invoice type code (ram:TypeCode) MUST have one of the following values: 380, 381, 384, 389"
	msgCII10 = "[BR-NL-10] For suppliers in the Netherlands, if the customer is in the Netherlands, the customer's legal entity identifier (/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement/ram:BuyerTradeParty/ram:SpecifiedLegalOrganization/ram:ID) MUST be either a KVK (schemeID=0106) or OIN number (schemeID=0190)"
	msgCII11 = "[BR-NL-11] For suppliers in the Netherlands, the supplier MUST provide a means of payment (rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeSettlement/ram:SpecifiedTradeSettlementPaymentMeans) if the payment is from customer to supplier"
	msgCII12 = "[BR-NL-12] For suppliers in the Netherlands, the payment means code (ram:ApplicableHeaderTradeSettlement/ram:SpecifiedTradeSettlementPaymentMeans/ram:TypeCode) MUST be one of 30, 48, 49, 57, 58 or 59"
	msgCII13 = "[BR-NL-13] If an order line reference (BT-132) is used, there must be an order reference on the document level (BT-13)"
	msgCII19 = "[BR-NL-19] The use of a tax currency code (/*/ram:ApplicableHeaderTradeSettlement/ram:TaxCurrencyCode) is not recommended"
	msgCII20 = "[BR-NL-20] The use of a tax point date (/*/ram:ApplicableHeaderTradeSettlement/ram:ApplicableTradeTax/ram:TaxPointDate/udt:DateString) is not recommended, and its value will be ignored"
	msgCII21 = "[BR-NL-21] The use of a tax point date code (/*/ram:ApplicableHeaderTradeSettlement/ram:ApplicableTradeTax/ram:DueDateTypeCode) is not recommended, and its value will be ignored"
	msgCII22 = "[BR-NL-22] The use of an invoice note subject code (/rsm:ExchangedDocument/ram:IncludedNote/ram:SubjectCode) is not recommended. If deemed necessary the code shall be agreed upon with the receiving party."
	msgCII23 = "[BR-NL-23] The use of an invoice note subject code (/*/rsm:ExchangedDocumentContext/ram:BusinessProcessSpecifiedDocumentContextParameter/ram:ID) is not recommended, unless desired by a particular network (such as PEPPOL)"
	msgCII24 = "[BR-NL-24] The use of a preceding invoice issue date (/*/ram:ApplicableHeaderTradeSettlement/ram:InvoiceReferencedDocument/ram:FormattedIssueDateTime/qdt:DateTimeString) is not recommended"
	msgCII25 = "[BR-NL-25] The use of a seller tax registration identifier (/*/*/ram:SellerTradeParty/ram:SpecifiedTaxRegistration/ram:ID) is not recommended when the tax scheme is not VAT (VA), since this is not applicable to suppliers in the Netherlands"
	msgCII26 = "[BR-NL-26] The use of the seller additional legal information field (/*/*/am:SellerTradeParty/ram:Description) is not recommended, since this is not applicable for suppliers in the Netherlands"
	msgCII29 = "[BR-NL-29] The use of a payment means text (/*/ram:ApplicableHeaderTradeSettlement/ram:SpecifiedTradeSettlementPaymentMeans/ram:Information) is not recommended"
	msgCII30 = "[BR-NL-30] The use of a payment account name (/*/*/ram:SpecifiedTradeSettlementPaymentMeans/ram:PayeePartyCreditorFinancialAccount/ram:AccountName) is not recommended"
	msgCII32 = "[BR-NL-32] / [BR-NL-34] The use of an allowance reason code or charge reason code (ram:SpecifiedTradeAllowanceCharge/ram:ReasonCode) are not recommended, both on document level and on line level."
	msgCII33 = "[BR-NL-33] The use of a tax total in accounting currency (ram:SpecifiedTradeSettlementHeaderMonetarySummation/ram:TaxTotalAmount@currencyID different than InvoiceCurrencyCode) is not recommended"
	msgCII35 = "[BR-NL-35] The use of a tax exemption reason code (/*/ram:ApplicableHeaderTradeSettlement/ram:ApplicableTradeTax/ram:ExemptionReasonCode) is not recommended"
)

// lineThree and subdivision give the messages of BR-NL-27 and BR-NL-28 for
// a party.
func lineThree(party, role string) string {
	return "[BR-NL-27] The use of the " + role + " address line 3 (/*/*/ram:" + party + "/ram:PostalTradeAddress/ram:LineThree) is not recommended, but allowed"
}

func subdivision(party string) string {
	return "[BR-NL-28] The use of a country subdivision (/*/*/ram:" + party + "/ram:PostalTradeAddress/ram:CountrySubDivisionName) is not recommended, but allowed"
}

// ciiRules are the rules of the pattern. BR-NL-9 and BR-NL-31, whose
// contexts repeat those of BR-NL-7 and BR-NL-12, never apply (see the
// package documentation).
var ciiRules = []rule{
	{cii("ram:SellerTradeParty/ram:SpecifiedLegalOrganization"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		ids := list(all(n, "ID"))
		k.assert(schemes(ids, "0106", "0190") && anyNonBlank(ids), "BR-NL-1", rules.Error, n, msgCII1)
	}},
	{cii("ram:ApplicableHeaderTradeAgreement"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(exists(n, "BuyerReference") || exists(n, "BuyerOrderReferencedDocument/IssuerAssignedID"), "BR-NL-2", rules.Error, n, msgCII2)
	}},
	{cii("ram:SellerTradeParty/ram:PostalTradeAddress"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(ciiAddress(n), "BR-NL-3", rules.Error, n, msgCII3)
	}},
	{cii("ram:BuyerTradeParty/ram:PostalTradeAddress"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(anyNe(list(all(n, "CountryID")), "NL") || ciiAddress(n), "BR-NL-4", rules.Error, n, msgCII4)
	}},
	{cii("ram:SellerTaxRepresentativeTradeParty/ram:PostalTradeAddress"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(anyNe(list(all(n, "CountryID")), "NL") || ciiAddress(n), "BR-NL-5", rules.Error, n, msgCII5)
	}},
	{cii("/*/rsm:ExchangedDocument/ram:TypeCode"), nlcius, func(k *checker, n *xmltree.Node) {
		eq, ok := numEq([]*xmltree.Node{n}, 380, 381, 384, 389)
		k.assert(eq && ok, "BR-NL-7", rules.Error, n, msgCII7)
	}},
	{cii("ram:BuyerTradeParty"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		ids := list(all(n, "SpecifiedLegalOrganization/ID"))
		dutch := anyEq(list(all(n, "PostalTradeAddress/CountryID")), "NL")
		k.assert((!dutch || schemes(ids, "0106", "0190")) && anyNonBlank(ids), "BR-NL-10", rules.Error, n, msgCII10)
	}},
	{cii("ram:SpecifiedTradeSettlementHeaderMonetarySummation"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(ciiPaymentMeansOK(k, n), "BR-NL-11", rules.Error, n, msgCII11)
	}},
	{cii("ram:SpecifiedTradeSettlementPaymentMeans"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		eq, ok := numEq(list(all(n, "TypeCode")), 30, 48, 49, 57, 58, 59)
		k.assert(eq && ok, "BR-NL-12", rules.Error, n, msgCII12)
	}},
	{cii("ram:BuyerOrderReferencedDocument/ram:LineID"), nlcius, func(k *checker, n *xmltree.Node) {
		ok := first(descendants(k.root, "ApplicableHeaderTradeAgreement/BuyerOrderReferencedDocument/IssuerAssignedID")) != nil
		k.assert(ok, "BR-NL-13", rules.Error, n, msgCII13)
	}},
	{cii("ram:TaxCurrencyCode"), dutchSupplier, discouraged("BR-NL-19", msgCII19)},
	{cii("ram:TaxPointDate/udt:DateString"), dutchSupplier, discouraged("BR-NL-20", msgCII20)},
	{cii("ram:DueDateTypeCode"), dutchSupplier, discouraged("BR-NL-21", msgCII21)},
	{cii("ram:SubjectCode"), dutchSupplier, discouraged("BR-NL-22", msgCII22)},
	{cii("ram:BusinessProcessSpecifiedDocumentContextParameter/ram:ID"), dutchSupplier, discouraged("BR-NL-23", msgCII23)},
	{cii("ram:InvoiceReferencedDocument/ram:FormattedIssueDateTime/qdt:DateTimeString"), dutchSupplier, discouraged("BR-NL-24", msgCII24)},
	{cii("ram:SellerTradeParty/ram:SpecifiedTaxRegistration"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		ok := !exists(n, "ID")
		for id := range all(n, "ID") {
			if v, has := id.LookupAttr("schemeID"); has && v == "VA" {
				ok = true
			}
		}
		k.assert(ok, "BR-NL-25", rules.Warning, n, msgCII25)
	}},
	{cii("ram:SellerTradeParty/ram:Description"), dutchSupplier, discouraged("BR-NL-26", msgCII26)},
	{cii("ram:SellerTradeParty/ram:PostalTradeAddress/ram:LineThree"), dutchSupplier, discouraged("BR-NL-27-1", lineThree("SellerTradeParty", "seller"))},
	{cii("ram:BuyerTradeParty/ram:PostalTradeAddress/ram:LineThree"), dutchSupplier, discouraged("BR-NL-27-2", lineThree("BuyerTradeParty", "customer"))},
	{cii("ram:SellerTaxRepresentativeTradeParty/ram:PostalTradeAddress/ram:LineThree"), dutchSupplier, discouraged("BR-NL-27-3", lineThree("SellerTaxRepresentativeTradeParty", "tax representative"))},
	{cii("ram:ShipToTradeParty/ram:PostalTradeAddress/ram:LineThree"), dutchSupplier, discouraged("BR-NL-27-4", lineThree("ShipToTradeParty", "delivery"))},
	{cii("ram:SellerTradeParty/ram:PostalTradeAddress/ram:CountrySubDivisionName"), dutchSupplier, discouraged("BR-NL-28-1", subdivision("SellerTradeParty"))},
	{cii("ram:BuyerTradeParty/ram:PostalTradeAddress/ram:CountrySubDivisionName"), dutchSupplier, discouraged("BR-NL-28-2", subdivision("BuyerTradeParty"))},
	{cii("ram:SellerTaxRepresentativeTradeParty/ram:PostalTradeAddress/ram:CountrySubDivisionName"), dutchSupplier, discouraged("BR-NL-28-3", subdivision("SellerTaxRepresentativeTradeParty"))},
	{cii("ram:ShipToTradeParty/ram:PostalTradeAddress/ram:CountrySubDivisionName"), dutchSupplier, discouraged("BR-NL-28-4", subdivision("ShipToTradeParty"))},
	{cii("ram:SpecifiedTradeSettlementPaymentMeans/ram:Information"), dutchSupplier, discouraged("BR-NL-29", msgCII29)},
	{cii("ram:PayeePartyCreditorFinancialAccount/ram:AccountName"), dutchSupplier, discouraged("BR-NL-30", msgCII30)},
	{cii("ram:SpecifiedTradeAllowanceCharge/ram:ReasonCode"), dutchSupplier, discouraged("BR-NL-32-and-34", msgCII32)},
	{cii("ram:SpecifiedTradeSettlementHeaderMonetarySummation/ram:TaxTotalAmount"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		cur, has := n.LookupAttr("currencyID")
		ok := has && n.Parent != nil && anyEq(list(all(n.Parent.Parent, "InvoiceCurrencyCode")), cur)
		k.assert(ok, "BR-NL-33", rules.Warning, n, msgCII33)
	}},
	{cii("ram:ApplicableTradeTax/ram:ExemptionReasonCode"), dutchSupplier, discouraged("BR-NL-35", msgCII35)},
}

// ciiAddress reports whether a postal address has a street (ram:LineOne), a
// city and a post code.
func ciiAddress(n *xmltree.Node) bool {
	return exists(n, "LineOne") && exists(n, "CityName") && exists(n, "PostcodeCode")
}

// ciiPaymentMeansOK evaluates the test of BR-NL-11 on the monetary summation
// n: xs:decimal(./ram:DuePayableAmount) <= 0.0 or
// (/*/rsm:ExchangedDocument/ram:TypeCode[$si] = 381) or
// (../ram:SpecifiedTradeSettlementPaymentMeans/ram:TypeCode). The operands
// are evaluated in order; a cast error makes the test false.
func ciiPaymentMeansOK(k *checker, n *xmltree.Node) bool {
	switch due := list(all(n, "DuePayableAmount")); len(due) {
	case 0: // the empty sequence compares false
	case 1:
		sign, ok := decimalSign(sv(due[0]))
		if !ok {
			return false
		}
		if sign <= 0 {
			return true
		}
	default:
		return false
	}
	if k.si {
		eq, ok := numEq(list(all(k.root, "ExchangedDocument/TypeCode")), 381)
		if !ok {
			return false
		}
		if eq {
			return true
		}
	}
	return exists(n.Parent, "SpecifiedTradeSettlementPaymentMeans/TypeCode")
}
