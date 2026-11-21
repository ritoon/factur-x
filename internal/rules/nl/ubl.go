package nl

import (
	"strings"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// Rules of the SI-UBL 2.0 artefacts: the pattern SI-UBL-VERSION of
// si-ubl-2.0.sch and the pattern nlcius of si-ubl-2.0-nlcius.sch, in the
// order of the schematrons.

// Namespace URIs of UBL.
const (
	nsInvoice    = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	nsCreditNote = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
	nsCAC        = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	nsCBC        = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
)

var ublNS = map[string]string{
	"ubl": nsInvoice,
	"cn":  nsCreditNote,
	"cac": nsCAC,
	"cbc": nsCBC,
}

// ubl compiles UBL contexts.
func ubl(paths ...string) []pattern {
	out := make([]pattern, len(paths))
	for i, p := range paths {
		out[i] = compile(p, ublNS)
	}
	return out
}

// checkUBL runs the rules of the SI-UBL 2.0 artefacts on the UBL document of
// c.
func checkUBL(c *rules.Checker) {
	root := c.Doc.UBL
	k := &checker{Checker: c, root: root}
	k.si = isSIUBL(nsv(first(all(root, "CustomizationID"))))
	country := "XX"
	if code := first(all(root, "AccountingSupplierParty/Party/PostalAddress/Country/IdentificationCode")); code != nil {
		country = strings.ToUpper(nsv(code))
	}
	k.s = k.si && country == "NL"
	k.run(versionRules, "")
	k.run(ublRules, "SI-UBL-2")
}

// isSIUBL reports whether a customization identifier (BT-24) declares an
// SI-UBL 2.0 invoice ($si of si-ubl-2.0-nlcius.sch).
func isSIUBL(id string) bool {
	return strings.Contains(id, "#compliant#urn:fdc:nen.nl:nlcius:v1.0") ||
		strings.Contains(id, "#conformant#urn:fdc:nen.nl:gaccount:v1.0")
}

// Messages of the UBL assertions.
const (
	msgInvoiceID    = "[BII2-T10-R001] An invoice MUST have a customization identifier"
	msgCreditNoteID = "[BII2-T10-R001] A credit note MUST have a customization identifier"
	msgVersion      = "[SI-V20-INV-R000]-This XML instance is NOT tagged as an SI-UBL 2.0 invoice or credit note; please check the CustomizationID value"

	msgUBL1   = "[BR-NL-1] For suppliers in the Netherlands the supplier MUST provide either a KVK or OIN number for its legal entity identifier (cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID with schemeID 0106 or 0190)"
	msgUBL2   = "[BR-NL-2] For suppliers in the Netherlands, the invoice MUST contain either the buyer reference (cbc:BuyerReference) or the order reference (cac:OrderReference/cbc:ID)"
	msgUBL3   = "[BR-NL-3] For suppliers in the Netherlands the supplier's address (cac:AccountingSupplierParty/cac:Party/cac:PostalAddress) MUST contain street name (cbc:StreetName), city (cbc:CityName) and postal zone (cbc:PostalZone)"
	msgUBL4   = "[BR-NL-4] For suppliers in the Netherlands, if the customer is in the Netherlands, the customer address (cac:AccountingCustomerParty/cac:Party/cac:PostalAddress) MUST contain the street name (cbc:StreetName), the city (cbc:CityName) and the postal zone (cbc:PostalZone)"
	msgUBL5   = "[BR-NL-5] For suppliers in the Netherlands, if the fiscal representative is in the Netherlands, the representative's address (cac:TaxRepresentativeParty/cac:PostalAddress) MUST contain street name (cbc:StreetName), city (cbc:CityName) and postal zone (cbc:PostalZone)"
	msgUBL7   = "[BR-NL-7] The invoice or credit note type code (cbc:InvoiceTypeCode/cbc:CreditNoteTypeCode) MUST have one of the following values: 380, 381, 384, 389"
	msgUBL8a  = "[BR-NL-8] If the invoice type code (cbc:InvoiceTypeCode) is 381, the document MUST use the CreditNote scheme"
	msgUBL8b  = "[BR-NL-8] If the credit note type code (cbc:CreditNoteTypeCode) is 380, 384 or 389, the document MUST use the Invoice scheme"
	msgUBL9   = "[BR-NL-9] For suppliers in the Netherlands, if the document is a corrective invoice (cbc:InvoiceTypeCode = 384), the document MUST contain an invoice reference (cac:BillingReference/cac:InvoiceDocumentReference/cbc:ID)"
	msgUBL10  = "[BR-NL-10] For suppliers in the Netherlands, if the customer is in the Netherlands, the customer's legal entity identifier (cac:AccountingCustomerParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID) MUST be either a KVK (schemeID=0106) or OIN number (schemeID=0190)"
	msgUBL11  = "[BR-NL-11] For suppliers in the Netherlands, the supplier MUST provide a means of payment (cac:PaymentMeans) if the payment is from customer to supplier"
	msgUBL12  = "[BR-NL-12] For suppliers in the Netherlands, the payment means code (cac:PaymentMeans/cbc:PaymentMeansCode) MUST be one of 30, 48, 49, 57, 58 or 59"
	msgUBL31  = "[BR-NL-31] The use of a payment service provider identifier (cac:PaymentMeans/cac:PayeeFinancialAccount/cac:FinancialInstitutionBranch/cbc:ID) is not recommended for SEPA payments (cac:PaymentMeans/cbc:PaymentMeansCode = 58 or 59)"
	msgUBL13  = "[BR-NL-13] If an order line reference (BT-132) is used, there must be an order reference on the document level (BT-13)"
	msgUBL19  = "[BR-NL-19] The use of a tax currency code (cbc:TaxCurrencyCode) is not recommended"
	msgUBL20  = "[BR-NL-20] The use of a tax point date (cbc:TaxPointDate) is not recommended, and its value will be ignored"
	msgUBL21  = "[BR-NL-21] The use of a tax point date code (cac:InvoicePeriod/cbc:DescriptionCode) is not recommended, and its value will be ignored"
	msgUBL24  = "[BR-NL-24] The use of a preceding invoice issue date (cac:BillingReference/cac:InvoiceDocumentReference/cbc:IssueDate) is not recommended"
	msgUBL25  = "[BR-NL-25] The use of a seller tax registration identifier (cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID) is not recommended when the tax scheme is not VAT, since this is not applicable to suppliers in the Netherlands"
	msgUBL26  = "[BR-NL-26] The use of the seller additional legal information field (cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyLegalForm) is not recommended, since this is not applicable for suppliers in the Netherlands"
	msgUBL271 = "[BR-NL-27] The use of the seller address line 3 (cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:AddressLine/cbc:Line) is not recommended"
	msgUBL272 = "[BR-NL-27] The use of the customer address line 3 (cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:AddressLine/cbc:Line) is not recommended"
	msgUBL273 = "[BR-NL-27] The use of the tax representative address line 3 (cac:TaxRepresentativePart/cac:PostalAddress/cac:AddressLine/cbc:Line) is not recommended"
	msgUBL274 = "[BR-NL-27] The use of the delivery address line 3 (cac:Delivery/cac:DeliveryLocation/cac:Address/cac:AddressLine/cbc:Line) is not recommended"
	msgUBL281 = "[BR-NL-28] The use of a country subdivision (cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cbc:CountrySubentity) is not recommended"
	msgUBL282 = "[BR-NL-28] The use of a country subdivision (cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cbc:CountrySubentity) is not recommended"
	msgUBL283 = "[BR-NL-28] The use of a country subdivision (cac:TaxRepresentativePart/cac:PostalAddress/cbc:CountrySubentity) is not recommended"
	msgUBL284 = "[BR-NL-28] The use of a country subdivision (cac:Delivery/cac:DeliveryLocation/cac:Address/cbc:CountrySubentity) is not recommended"
	msgUBL29  = "[BR-NL-29] The use of a payment means text (cac:PaymentMeans/cbc:PaymentMeansCode/@name) is not recommended"
	msgUBL30  = "[BR-NL-30] The use of a payment account name (cac:PaymentMeans/cac:PayeeFinancialAccount/cbc:Name) is not recommended"
	msgUBL32  = "[BR-NL-32] The use of an allowance reason code (cac:AllowanceCharge/cbc:AllowanceChargeReasonCode) is not recommended"
	msgUBL33  = "[BR-NL-33] The use of a tax total in accounting currency (cac:TaxTotal/cbc:TaxAmount/@currencyID different than DocumentCurrencyCode) is not recommended"
	msgUBL35  = "[BR-NL-35] The use of a tax exemption reason code (cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory/cbc:TaxExemptionReasonCode) is not recommended"
)

// versionRules are the rules of the pattern SI-UBL-VERSION, without
// SI-INV-R000 (see the package documentation). Its assertions have no
// identifier: the one in brackets at the start of their message is used.
var versionRules = []rule{
	{ubl("ubl:Invoice"), always, func(k *checker, n *xmltree.Node) {
		k.assert(exists(n, "CustomizationID"), "BII2-T10-R001", rules.Error, n, msgInvoiceID)
	}},
	{ubl("cn:CreditNote"), always, func(k *checker, n *xmltree.Node) {
		k.assert(exists(n, "CustomizationID"), "BII2-T10-R001", rules.Error, n, msgCreditNoteID)
	}},
	{ubl("cbc:CustomizationID"), always, func(k *checker, n *xmltree.Node) {
		k.assert(strings.HasPrefix(nsv(n), CIUSID), "SI-V20-INV-R000", rules.Error, n, msgVersion)
	}},
}

// ublRules are the rules of the pattern nlcius. The rules of BR-NL-32-2,
// BR-NL-32-3 and BR-NL-34, whose contexts are matched by an earlier rule,
// never apply (see the package documentation).
var ublRules = []rule{
	{ubl("cac:AccountingSupplierParty/cac:Party"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		ids := list(all(n, "PartyLegalEntity/CompanyID"))
		k.assert(schemes(ids, "0106", "0190") && anyNonBlank(ids), "BR-NL-1", rules.Error, n, msgUBL1)
	}},
	{ubl("/*"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(exists(n, "BuyerReference") || exists(n, "OrderReference/ID"), "BR-NL-2", rules.Error, n, msgUBL2)
	}},
	{ubl("cac:AccountingSupplierParty/cac:Party/cac:PostalAddress"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(ublAddress(n), "BR-NL-3", rules.Error, n, msgUBL3)
	}},
	{ubl("cac:AccountingCustomerParty/cac:Party/cac:PostalAddress"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(anyNe(list(all(n, "Country/IdentificationCode")), "NL") || ublAddress(n), "BR-NL-4", rules.Error, n, msgUBL4)
	}},
	{ubl("cac:TaxRepresentativeParty/cac:PostalAddress"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(anyNe(list(all(n, "Country/IdentificationCode")), "NL") || ublAddress(n), "BR-NL-5", rules.Error, n, msgUBL5)
	}},
	{ubl("cbc:InvoiceTypeCode", "cbc:CreditNoteTypeCode"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		v := sv(n)
		k.assert(v == "380" || v == "381" || v == "384" || v == "389", "BR-NL-7", rules.Error, n, msgUBL7)
		k.assert(v != "381" || k.rootIs(nsCreditNote, "CreditNote"), "BR-NL-8", rules.Error, n, msgUBL8a)
		k.assert(v == "381" || k.rootIs(nsInvoice, "Invoice"), "BR-NL-8", rules.Error, n, msgUBL8b)
		k.assert(v != "384" || exists(k.root, "BillingReference/InvoiceDocumentReference/ID"), "BR-NL-9", rules.Error, n, msgUBL9)
	}},
	{ubl("cac:AccountingCustomerParty/cac:Party/cac:PartyLegalEntity"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		dutch := anyEq(list(descendants(k.root, "AccountingCustomerParty/Party/PostalAddress/Country/IdentificationCode")), "NL")
		allIDs := list(descendants(k.root, "AccountingCustomerParty/Party/PartyLegalEntity/CompanyID"))
		ids := list(all(n, "CompanyID"))
		k.assert((!dutch || schemes(allIDs, "0106", "0190")) && (len(ids) == 0 || anyNonBlank(ids)), "BR-NL-10", rules.Error, n, msgUBL10)
	}},
	{ubl("/ubl:Invoice/cac:LegalMonetaryTotal"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(ublPaymentMeansOK(k, n), "BR-NL-11", rules.Error, n, msgUBL11)
	}},
	{ubl("cac:PaymentMeans"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		codes := list(all(n, "PaymentMeansCode"))
		single := len(codes) <= 1 // normalize-space of several values is an error
		code := ""
		if len(codes) == 1 {
			code = nsv(codes[0])
		}
		k.assert(single && (code == "30" || code == "48" || code == "49" || code == "57" || code == "58" || code == "59"), "BR-NL-12", rules.Error, n, msgUBL12)
		k.assert(single && (code != "58" && code != "59" || !exists(n, "PayeeFinancialAccount/FinancialInstitutionBranch/ID")), "BR-NL-31", rules.Warning, n, msgUBL31)
	}},
	{ubl("cac:OrderLineReference/cbc:LineID"), nlcius, func(k *checker, n *xmltree.Node) {
		k.assert(exists(k.root, "OrderReference/ID"), "BR-NL-13", rules.Error, n, msgUBL13)
	}},
	{ubl("cbc:TaxCurrencyCode"), dutchSupplier, discouraged("BR-NL-19", msgUBL19)},
	{ubl("cbc:TaxPointDate"), dutchSupplier, discouraged("BR-NL-20", msgUBL20)},
	{ubl("cac:InvoicePeriod/cbc:DescriptionCode"), dutchSupplier, discouraged("BR-NL-21", msgUBL21)},
	{ubl("cac:BillingReference/cac:InvoiceDocumentReference/cbc:IssueDate"), dutchSupplier, discouraged("BR-NL-24", msgUBL24)},
	{ubl("cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		k.assert(!exists(n, "CompanyID") || anyEq(list(all(n, "TaxScheme/ID")), "VAT"), "BR-NL-25", rules.Warning, n, msgUBL25)
	}},
	{ubl("cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyLegalForm"), dutchSupplier, discouraged("BR-NL-26", msgUBL26)},
	{ubl("cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:AddressLine/cbc:Line"), dutchSupplier, discouraged("BR-NL-27-1", msgUBL271)},
	{ubl("cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:AddressLine/cbc:Line"), dutchSupplier, discouraged("BR-NL-27-2", msgUBL272)},
	{ubl("cac:TaxRepresentativeParty/cac:PostalAddress/cac:AddressLine/cbc:Line"), dutchSupplier, discouraged("BR-NL-27-3", msgUBL273)},
	{ubl("cac:Delivery/cac:DeliveryLocation/cac:Address/cac:AddressLine/cbc:Line"), dutchSupplier, discouraged("BR-NL-27-4", msgUBL274)},
	{ubl("cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cbc:CountrySubentity"), dutchSupplier, discouraged("BR-NL-28-1", msgUBL281)},
	{ubl("cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cbc:CountrySubentity"), dutchSupplier, discouraged("BR-NL-28-2", msgUBL282)},
	{ubl("cac:TaxRepresentativeParty/cac:PostalAddress/cbc:CountrySubentity"), dutchSupplier, discouraged("BR-NL-28-3", msgUBL283)},
	{ubl("cac:Delivery/cac:DeliveryLocation/cac:Address/cbc:CountrySubentity"), dutchSupplier, discouraged("BR-NL-28-4", msgUBL284)},
	{ubl("cac:PaymentMeans/cbc:PaymentMeansCode"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		_, named := n.LookupAttr("name")
		k.assert(!named, "BR-NL-29", rules.Warning, n, msgUBL29)
	}},
	{ubl("cac:PaymentMeans/cac:PayeeFinancialAccount/cbc:Name"), dutchSupplier, discouraged("BR-NL-30", msgUBL30)},
	{ubl("cac:AllowanceCharge/cbc:AllowanceChargeReasonCode"), dutchSupplier, discouraged("BR-NL-32-1", msgUBL32)},
	{ubl("cac:TaxTotal/cbc:TaxAmount"), dutchSupplier, func(k *checker, n *xmltree.Node) {
		cur, has := n.LookupAttr("currencyID")
		k.assert(has && anyEq(list(descendants(k.root, "DocumentCurrencyCode")), cur), "BR-NL-33", rules.Warning, n, msgUBL33)
	}},
	{ubl("cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory/cbc:TaxExemptionReasonCode"), dutchSupplier, discouraged("BR-NL-35", msgUBL35)},
}

// rootIs reports whether the root element has the namespace space and the
// local name name (/ubl:Invoice, /cn:CreditNote).
func (k *checker) rootIs(space, name string) bool {
	return k.root.Space == space && k.root.Name == name
}

// ublAddress reports whether a postal address has a street name, a city and
// a postal zone.
func ublAddress(n *xmltree.Node) bool {
	return exists(n, "StreetName") && exists(n, "CityName") && exists(n, "PostalZone")
}

// ublPaymentMeansOK evaluates the test of BR-NL-11 on cac:LegalMonetaryTotal:
// xs:decimal(cbc:PayableAmount) <= 0.0 or (//cac:PaymentMeans). A cast error
// makes the test false.
func ublPaymentMeansOK(k *checker, n *xmltree.Node) bool {
	switch amounts := list(all(n, "PayableAmount")); len(amounts) {
	case 0: // the empty sequence compares false
	case 1:
		sign, ok := decimalSign(sv(amounts[0]))
		if !ok {
			return false
		}
		if sign <= 0 {
			return true
		}
	default:
		return false
	}
	return first(descendants(k.root, "PaymentMeans")) != nil
}
