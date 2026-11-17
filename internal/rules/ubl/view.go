package ubl

import (
	"strings"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// Namespaces of the UBL 2.1 syntax.
const (
	NSInvoice    = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	NSCreditNote = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
	NSCac        = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	NSCbc        = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
	NSExt        = "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2"
)

// IsUBL reports whether root is a UBL Invoice or CreditNote.
func IsUBL(root *xmltree.Node) bool {
	return root != nil && (root.Name == "Invoice" && root.Space == NSInvoice ||
		root.Name == "CreditNote" && root.Space == NSCreditNote)
}

// View builds the CII view of a UBL Invoice or CreditNote and wraps it in a
// rules.Doc (see rules.NewUBLDoc). The profile is read from BT-24 unless
// profile is not ProfileUnknown.
//
// The view carries the business terms of EN 16931 as CEN/TS 16931-3-2 binds
// them to UBL, in the CII elements that CEN/TS 16931-3-3 binds them to, so
// that the business rules written for CII give the verdicts of the UBL
// artefacts. Values are copied as they are (the rules compare them as the
// schematrons do), with the conversions of the bindings:
//
//   - dates (xs:date) become CII dates of format 102 (YYYYMMDD) when they
//     are well-formed, and are copied as they are otherwise;
//   - a note "#AAI#text" gives a note subject code (BT-21) and a note text;
//   - the VAT point date code (BT-8) is translated from the UBL codes 3, 35
//     and 432 to the CII codes 5, 29 and 72, and copied into each VAT
//     breakdown with the VAT point date (BT-7), as in CII;
//   - the tax scheme (cac:TaxScheme/cbc:ID) gives the tax type code, upper
//     case and without spaces, and the VAT category codes are trimmed: the
//     UBL rules compare normalize-space(upper-case(cbc:ID)) = 'VAT' and
//     normalize-space(cbc:ID) = 'S';
//   - the indicators (cbc:ChargeIndicator) are written true or false, which
//     is how the UBL rules compare them (cbc:ChargeIndicator = false());
//   - a party tax scheme gives a VAT identifier (scheme VA) when its tax
//     scheme is VAT, a tax registration identifier (scheme FC) otherwise;
//   - a party identifier of scheme SEPA is the bank assigned creditor
//     identifier (BT-90).
//
// Elements outside the binding are not carried: the UBL-CR rules report
// them.
func View(root *xmltree.Node, profile invoice.Profile) *rules.Doc {
	v := &view{origin: make(map[*xmltree.Node]*xmltree.Node, 256), credit: root.Name == "CreditNote", src: root}
	cii := v.elem(nil, rules.NSRsm, "rsm", "CrossIndustryInvoice", root)
	v.context(cii)
	v.header(cii)
	tx := v.elem(cii, rules.NSRsm, "rsm", "SupplyChainTradeTransaction", root)
	for _, l := range root.Children {
		if l.Name == "InvoiceLine" || l.Name == "CreditNoteLine" {
			v.line(tx, l)
		}
	}
	v.agreement(v.ram(tx, "ApplicableHeaderTradeAgreement", root))
	v.delivery(v.ram(tx, "ApplicableHeaderTradeDelivery", root))
	v.settlement(v.ram(tx, "ApplicableHeaderTradeSettlement", root))
	return rules.NewUBLDoc(root, cii, v.origin, profile)
}

// view builds a CII view.
type view struct {
	origin map[*xmltree.Node]*xmltree.Node
	credit bool          // the document is a CreditNote
	src    *xmltree.Node // root of the UBL document
}

// elem appends a new element to parent (nil for the root), standing for the
// UBL node from.
func (v *view) elem(parent *xmltree.Node, space, prefix, name string, from *xmltree.Node) *xmltree.Node {
	n := &xmltree.Node{Name: name, Space: space, Prefix: prefix, Parent: parent}
	if from != nil {
		n.Line = from.Line
		v.origin[n] = from
	}
	if parent != nil {
		parent.Children = append(parent.Children, n)
	}
	return n
}

// ram appends a new ram element.
func (v *view) ram(parent *xmltree.Node, name string, from *xmltree.Node) *xmltree.Node {
	return v.elem(parent, rules.NSRam, "ram", name, from)
}

// leaf appends a ram element with the text and the attributes of from.
func (v *view) leaf(parent *xmltree.Node, name string, from *xmltree.Node) *xmltree.Node {
	n := v.ram(parent, name, from)
	n.Text = from.Text
	for _, a := range from.Attrs {
		if a.Prefix == "" {
			n.Attrs = append(n.Attrs, a)
		}
	}
	return n
}

// leaves appends a leaf named name for each child of src named ublName.
func (v *view) leaves(parent *xmltree.Node, name string, src *xmltree.Node, ublName string) {
	for _, c := range kids(src) {
		if c.Name == ublName {
			v.leaf(parent, name, c)
		}
	}
}

// wrapped appends, for each child of src named ublName, an aggregate named
// outer holding a leaf named inner: "BuyerOrderReferencedDocument" holding
// "IssuerAssignedID" for each cbc:ID of cac:OrderReference.
func (v *view) wrapped(parent *xmltree.Node, outer, inner string, src *xmltree.Node, ublName string) {
	for _, c := range kids(src) {
		if c.Name == ublName {
			v.leaf(v.ram(parent, outer, c), inner, c)
		}
	}
}

// date appends a date of format 102, for each child of src named ublName,
// in an element named name: ram:IssueDateTime/udt:DateTimeString.
func (v *view) date(parent *xmltree.Node, name string, src *xmltree.Node, ublName string) {
	for _, c := range kids(src) {
		if c.Name == ublName {
			v.dateOf(parent, name, rules.NSUdt, "udt", "DateTimeString", c)
		}
	}
}

func (v *view) dateOf(parent *xmltree.Node, name, space, prefix, inner string, from *xmltree.Node) {
	d := v.elem(v.ram(parent, name, from), space, prefix, inner, from)
	d.Text = date102(from.Text)
	d.Attrs = []xmltree.Attr{{Name: "format", Value: "102"}}
}

// text appends a ram element with the given text, standing for from.
func (v *view) text(parent *xmltree.Node, name, text string, from *xmltree.Node) *xmltree.Node {
	n := v.ram(parent, name, from)
	n.Text = text
	return n
}

// kids returns the child elements of n (nil safe).
func kids(n *xmltree.Node) []*xmltree.Node {
	if n == nil {
		return nil
	}
	return n.Children
}

// each returns the children of n named name.
func each(n *xmltree.Node, name string) []*xmltree.Node { return n.Elems(name) }

// date102 converts an xs:date (YYYY-MM-DD, with an optional time zone and
// surrounding white space) to a CII date of format 102 (YYYYMMDD). Other
// values are returned unchanged.
func date102(s string) string {
	t := strings.TrimSpace(s)
	if len(t) < 10 || t[4] != '-' || t[7] != '-' {
		return s
	}
	for _, i := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if t[i] < '0' || t[i] > '9' {
			return s
		}
	}
	switch z := t[10:]; {
	case z == "", z == "Z":
	case len(z) == 6 && (z[0] == '+' || z[0] == '-') && z[3] == ':':
	default:
		return s
	}
	return t[0:4] + t[5:7] + t[8:10]
}

// normalize returns normalize-space(s).
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// isVATScheme reports whether a tax scheme is VAT, as the UBL rules test it:
// cac:TaxScheme/normalize-space(upper-case(cbc:ID)) = 'VAT'.
func isVATScheme(taxScheme *xmltree.Node) bool {
	for _, id := range each(taxScheme, "ID") {
		if strings.ToUpper(normalize(id.Text)) == "VAT" {
			return true
		}
	}
	return false
}

func (v *view) context(cii *xmltree.Node) {
	ctx := v.elem(cii, rules.NSRsm, "rsm", "ExchangedDocumentContext", v.src)
	v.wrapped(ctx, "BusinessProcessSpecifiedDocumentContextParameter", "ID", v.src, "ProfileID")
	v.wrapped(ctx, "GuidelineSpecifiedDocumentContextParameter", "ID", v.src, "CustomizationID")
}

func (v *view) header(cii *xmltree.Node) {
	root := v.src
	h := v.elem(cii, rules.NSRsm, "rsm", "ExchangedDocument", root)
	v.leaves(h, "ID", root, "ID")
	for _, c := range kids(root) {
		if c.Name == "InvoiceTypeCode" || c.Name == "CreditNoteTypeCode" {
			v.leaf(h, "TypeCode", c)
		}
	}
	v.date(h, "IssueDateTime", root, "IssueDate")
	for _, c := range each(root, "Note") {
		n := v.ram(h, "IncludedNote", c)
		code, content := splitNote(c.Text)
		v.text(n, "Content", content, c)
		if code != "" {
			v.text(n, "SubjectCode", code, c)
		}
	}
}

// splitNote splits a note "#AAI#text" into its subject code and its text. A
// note that does not start with a code of one to ten letters or digits
// between two "#" is returned whole.
func splitNote(s string) (code, content string) {
	if len(s) < 3 || s[0] != '#' {
		return "", s
	}
	end := strings.IndexByte(s[1:], '#')
	if end <= 0 || end > 10 {
		return "", s
	}
	code = s[1 : 1+end]
	for i := 0; i < len(code); i++ {
		c := code[i]
		if !('A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9') {
			return "", s
		}
	}
	return code, s[2+end:]
}

func (v *view) line(tx, l *xmltree.Node) {
	li := v.ram(tx, "IncludedSupplyChainTradeLineItem", l)
	doc := v.ram(li, "AssociatedDocumentLineDocument", l)
	v.leaves(doc, "LineID", l, "ID")
	for _, c := range each(l, "Note") {
		v.leaf(v.ram(doc, "IncludedNote", c), "Content", c)
	}

	for _, item := range each(l, "Item") {
		p := v.ram(li, "SpecifiedTradeProduct", item)
		for _, s := range each(item, "StandardItemIdentification") {
			v.leaves(p, "GlobalID", s, "ID")
		}
		for _, s := range each(item, "SellersItemIdentification") {
			v.leaves(p, "SellerAssignedID", s, "ID")
		}
		for _, s := range each(item, "BuyersItemIdentification") {
			v.leaves(p, "BuyerAssignedID", s, "ID")
		}
		v.leaves(p, "Name", item, "Name")
		v.leaves(p, "Description", item, "Description")
		for _, a := range each(item, "AdditionalItemProperty") {
			c := v.ram(p, "ApplicableProductCharacteristic", a)
			v.leaves(c, "Description", a, "Name")
			v.leaves(c, "Value", a, "Value")
		}
		for _, cc := range each(item, "CommodityClassification") {
			for _, code := range each(cc, "ItemClassificationCode") {
				v.leaf(v.ram(p, "DesignatedProductClassification", cc), "ClassCode", code)
			}
		}
		for _, o := range each(item, "OriginCountry") {
			v.wrapped(p, "OriginTradeCountry", "ID", o, "IdentificationCode")
		}
	}

	agr := v.ram(li, "SpecifiedLineTradeAgreement", l)
	for _, r := range each(l, "OrderLineReference") {
		v.wrapped(agr, "BuyerOrderReferencedDocument", "LineID", r, "LineID")
	}
	for _, price := range each(l, "Price") {
		for _, ac := range each(price, "AllowanceCharge") {
			g := v.ram(agr, "GrossPriceProductTradePrice", ac)
			v.leaves(g, "ChargeAmount", ac, "BaseAmount")
			a := v.ram(g, "AppliedTradeAllowanceCharge", ac)
			v.indicator(a, ac)
			v.leaves(a, "ActualAmount", ac, "Amount")
		}
		n := v.ram(agr, "NetPriceProductTradePrice", price)
		v.leaves(n, "ChargeAmount", price, "PriceAmount")
		v.leaves(n, "BasisQuantity", price, "BaseQuantity")
	}

	del := v.ram(li, "SpecifiedLineTradeDelivery", l)
	v.leaves(del, "BilledQuantity", l, "InvoicedQuantity")
	v.leaves(del, "BilledQuantity", l, "CreditedQuantity")

	set := v.ram(li, "SpecifiedLineTradeSettlement", l)
	for _, item := range each(l, "Item") {
		for _, t := range each(item, "ClassifiedTaxCategory") {
			v.taxCategory(v.ram(set, "ApplicableTradeTax", t), t)
		}
	}
	for _, p := range each(l, "InvoicePeriod") {
		v.period(set, p)
	}
	for _, ac := range each(l, "AllowanceCharge") {
		v.allowanceCharge(set, ac, false)
	}
	for _, a := range each(l, "LineExtensionAmount") {
		v.leaf(v.ram(set, "SpecifiedTradeSettlementLineMonetarySummation", a), "LineTotalAmount", a)
	}
	for _, r := range each(l, "DocumentReference") {
		d := v.ram(set, "AdditionalReferencedDocument", r)
		v.leaves(d, "IssuerAssignedID", r, "ID")
		v.leaves(d, "TypeCode", r, "DocumentTypeCode")
		for _, id := range each(r, "ID") {
			if s, ok := id.LookupAttr("schemeID"); ok {
				v.text(d, "ReferenceTypeCode", s, id)
			}
		}
	}
	v.wrapped(set, "ReceivableSpecifiedTradeAccountingAccount", "ID", l, "AccountingCost")
}

// indicator appends the ram:ChargeIndicator of an allowance or charge.
func (v *view) indicator(parent, ac *xmltree.Node) {
	for _, c := range each(ac, "ChargeIndicator") {
		i := v.elem(v.ram(parent, "ChargeIndicator", c), rules.NSUdt, "udt", "Indicator", c)
		switch strings.TrimSpace(c.Text) {
		case "true", "1":
			i.Text = "true"
		case "false", "0":
			i.Text = "false"
		default:
			i.Text = c.Text
		}
	}
}

// taxCategory fills a ram:ApplicableTradeTax or ram:CategoryTradeTax from a
// cac:ClassifiedTaxCategory or cac:TaxCategory: tax type, category code and
// rate.
func (v *view) taxCategory(t, c *xmltree.Node) {
	v.taxType(t, c)
	v.categoryCode(t, c)
	v.leaves(t, "RateApplicablePercent", c, "Percent")
}

func (v *view) taxType(t, c *xmltree.Node) {
	for _, s := range each(c, "TaxScheme") {
		for _, id := range each(s, "ID") {
			v.text(t, "TypeCode", strings.ToUpper(normalize(id.Text)), id)
		}
	}
}

func (v *view) categoryCode(t, c *xmltree.Node) {
	for _, id := range each(c, "ID") {
		v.text(t, "CategoryCode", normalize(id.Text), id)
	}
}

func (v *view) period(parent, p *xmltree.Node) {
	if p.Elem("StartDate") == nil && p.Elem("EndDate") == nil {
		return
	}
	n := v.ram(parent, "BillingSpecifiedPeriod", p)
	v.date(n, "StartDateTime", p, "StartDate")
	v.date(n, "EndDateTime", p, "EndDate")
}

// allowanceCharge appends a ram:SpecifiedTradeAllowanceCharge; header tells
// whether it is a document level allowance or charge, with a VAT category.
func (v *view) allowanceCharge(parent, ac *xmltree.Node, header bool) {
	n := v.ram(parent, "SpecifiedTradeAllowanceCharge", ac)
	v.indicator(n, ac)
	v.leaves(n, "CalculationPercent", ac, "MultiplierFactorNumeric")
	v.leaves(n, "BasisAmount", ac, "BaseAmount")
	v.leaves(n, "ActualAmount", ac, "Amount")
	v.leaves(n, "ReasonCode", ac, "AllowanceChargeReasonCode")
	v.leaves(n, "Reason", ac, "AllowanceChargeReason")
	if header {
		for _, t := range each(ac, "TaxCategory") {
			v.taxCategory(v.ram(n, "CategoryTradeTax", t), t)
		}
	}
}

// Party roles.
const (
	roleSeller = iota
	roleBuyer
	rolePayee
	roleTaxRep
)

func (v *view) agreement(agr *xmltree.Node) {
	root := v.src
	v.leaves(agr, "BuyerReference", root, "BuyerReference")
	for _, p := range each(root, "AccountingSupplierParty") {
		v.party(agr, "SellerTradeParty", p, p.Elem("Party"), roleSeller)
	}
	for _, p := range each(root, "AccountingCustomerParty") {
		v.party(agr, "BuyerTradeParty", p, p.Elem("Party"), roleBuyer)
	}
	for _, p := range each(root, "TaxRepresentativeParty") {
		v.party(agr, "SellerTaxRepresentativeTradeParty", p, p, roleTaxRep)
	}
	for _, o := range each(root, "OrderReference") {
		v.wrapped(agr, "SellerOrderReferencedDocument", "IssuerAssignedID", o, "SalesOrderID")
	}
	for _, o := range each(root, "OrderReference") {
		v.wrapped(agr, "BuyerOrderReferencedDocument", "IssuerAssignedID", o, "ID")
	}
	for _, c := range each(root, "ContractDocumentReference") {
		v.wrapped(agr, "ContractReferencedDocument", "IssuerAssignedID", c, "ID")
	}
	var projects []*xmltree.Node
	for _, r := range each(root, "AdditionalDocumentReference") {
		code := strings.TrimSpace(r.Value("DocumentTypeCode"))
		if v.credit && code == "50" {
			projects = append(projects, r)
			continue
		}
		d := v.ram(agr, "AdditionalReferencedDocument", r)
		v.leaves(d, "IssuerAssignedID", r, "ID")
		for _, a := range each(r, "Attachment") {
			for _, e := range each(a, "ExternalReference") {
				v.leaves(d, "URIID", e, "URI")
			}
		}
		switch tc := r.Elem("DocumentTypeCode"); {
		case code == "130":
			v.text(d, "TypeCode", "130", tc)
		case tc != nil:
			v.text(d, "TypeCode", "916", tc)
		default:
			v.text(d, "TypeCode", "916", r)
		}
		v.leaves(d, "Name", r, "DocumentDescription")
		for _, a := range each(r, "Attachment") {
			v.leaves(d, "AttachmentBinaryObject", a, "EmbeddedDocumentBinaryObject")
		}
		if code == "130" {
			for _, id := range each(r, "ID") {
				if s, ok := id.LookupAttr("schemeID"); ok {
					v.text(d, "ReferenceTypeCode", s, id)
				}
			}
		}
	}
	for _, r := range each(root, "OriginatorDocumentReference") {
		d := v.ram(agr, "AdditionalReferencedDocument", r)
		v.leaves(d, "IssuerAssignedID", r, "ID")
		v.text(d, "TypeCode", "50", r)
	}
	for _, r := range each(root, "ProjectReference") {
		v.wrapped(agr, "SpecifiedProcuringProject", "ID", r, "ID")
	}
	for _, r := range projects {
		v.wrapped(agr, "SpecifiedProcuringProject", "ID", r, "ID")
	}
}

// party appends a CII party for the UBL party p (the cac:Party of outer, or
// the party itself for the tax representative).
func (v *view) party(parent *xmltree.Node, name string, outer, p *xmltree.Node, role int) {
	from := p
	if from == nil {
		from = outer
	}
	n := v.ram(parent, name, from)
	if p == nil {
		return
	}
	if role != roleTaxRep {
		for _, pi := range each(p, "PartyIdentification") {
			for _, id := range each(pi, "ID") {
				switch s, ok := id.LookupAttr("schemeID"); {
				case s == "SEPA":
				case ok:
					v.leaf(n, "GlobalID", id)
				default:
					v.leaf(n, "ID", id)
				}
			}
		}
	}
	switch role {
	case roleSeller, roleBuyer:
		for _, le := range each(p, "PartyLegalEntity") {
			v.leaves(n, "Name", le, "RegistrationName")
		}
	default:
		for _, pn := range each(p, "PartyName") {
			v.leaves(n, "Name", pn, "Name")
		}
	}
	if role == roleSeller {
		for _, le := range each(p, "PartyLegalEntity") {
			v.leaves(n, "Description", le, "CompanyLegalForm")
		}
	}
	if role != roleTaxRep {
		v.legalOrganization(n, p, role)
	}
	if role == roleSeller || role == roleBuyer {
		for _, c := range each(p, "Contact") {
			dc := v.ram(n, "DefinedTradeContact", c)
			v.leaves(dc, "PersonName", c, "Name")
			v.wrapped(dc, "TelephoneUniversalCommunication", "CompleteNumber", c, "Telephone")
			v.wrapped(dc, "EmailURIUniversalCommunication", "URIID", c, "ElectronicMail")
		}
	}
	if role != rolePayee {
		for _, a := range each(p, "PostalAddress") {
			v.address(n, a)
		}
	}
	if role == roleSeller || role == roleBuyer {
		v.wrapped(n, "URIUniversalCommunication", "URIID", p, "EndpointID")
	}
	if role != rolePayee {
		for _, s := range each(p, "PartyTaxScheme") {
			scheme := "FC"
			for _, ts := range each(s, "TaxScheme") {
				if isVATScheme(ts) {
					scheme = "VA"
				}
			}
			for _, id := range each(s, "CompanyID") {
				r := v.ram(n, "SpecifiedTaxRegistration", s)
				i := v.text(r, "ID", id.Text, id)
				i.Attrs = []xmltree.Attr{{Name: "schemeID", Value: scheme}}
			}
		}
	}
}

// legalOrganization appends the ram:SpecifiedLegalOrganization of a party:
// its legal registration identifier and, for the seller and the buyer, its
// trading name.
func (v *view) legalOrganization(n, p *xmltree.Node, role int) {
	var ids, names []*xmltree.Node
	for _, le := range each(p, "PartyLegalEntity") {
		ids = append(ids, each(le, "CompanyID")...)
	}
	if role == roleSeller || role == roleBuyer {
		for _, pn := range each(p, "PartyName") {
			names = append(names, each(pn, "Name")...)
		}
	}
	if len(ids) == 0 && len(names) == 0 {
		return
	}
	from := p.Elem("PartyLegalEntity")
	if from == nil {
		from = p.Elem("PartyName")
	}
	o := v.ram(n, "SpecifiedLegalOrganization", from)
	for _, id := range ids {
		v.leaf(o, "ID", id)
	}
	for _, name := range names {
		v.leaf(o, "TradingBusinessName", name)
	}
}

func (v *view) address(parent, a *xmltree.Node) {
	n := v.ram(parent, "PostalTradeAddress", a)
	v.leaves(n, "PostcodeCode", a, "PostalZone")
	v.leaves(n, "LineOne", a, "StreetName")
	v.leaves(n, "LineTwo", a, "AdditionalStreetName")
	for _, l := range each(a, "AddressLine") {
		v.leaves(n, "LineThree", l, "Line")
	}
	v.leaves(n, "CityName", a, "CityName")
	for _, c := range each(a, "Country") {
		v.leaves(n, "CountryID", c, "IdentificationCode")
	}
	v.leaves(n, "CountrySubDivisionName", a, "CountrySubentity")
}

func (v *view) delivery(del *xmltree.Node) {
	root := v.src
	for _, d := range each(root, "Delivery") {
		locs, parties := each(d, "DeliveryLocation"), each(d, "DeliveryParty")
		if len(locs) > 0 || len(parties) > 0 {
			st := v.ram(del, "ShipToTradeParty", d)
			for _, l := range locs {
				for _, id := range each(l, "ID") {
					if _, ok := id.LookupAttr("schemeID"); ok {
						v.leaf(st, "GlobalID", id)
					} else {
						v.leaf(st, "ID", id)
					}
				}
			}
			for _, p := range parties {
				for _, pn := range each(p, "PartyName") {
					v.leaves(st, "Name", pn, "Name")
				}
			}
			for _, l := range locs {
				for _, a := range each(l, "Address") {
					v.address(st, a)
				}
			}
		}
		for _, ad := range each(d, "ActualDeliveryDate") {
			e := v.ram(del, "ActualDeliverySupplyChainEvent", ad)
			v.dateOf(e, "OccurrenceDateTime", rules.NSUdt, "udt", "DateTimeString", ad)
		}
	}
	for _, r := range each(root, "DespatchDocumentReference") {
		v.wrapped(del, "DespatchAdviceReferencedDocument", "IssuerAssignedID", r, "ID")
	}
	for _, r := range each(root, "ReceiptDocumentReference") {
		v.wrapped(del, "ReceivingAdviceReferencedDocument", "IssuerAssignedID", r, "ID")
	}
}

func (v *view) settlement(set *xmltree.Node) {
	root := v.src
	// BT-90: the party identifier of scheme SEPA of the seller or the payee.
	for _, outer := range append(each(root, "AccountingSupplierParty"), each(root, "PayeeParty")...) {
		p := outer
		if outer.Name == "AccountingSupplierParty" {
			p = outer.Elem("Party")
		}
		for _, pi := range each(p, "PartyIdentification") {
			for _, id := range each(pi, "ID") {
				if s, _ := id.LookupAttr("schemeID"); s == "SEPA" {
					n := v.leaf(set, "CreditorReferenceID", id)
					n.Attrs = nil
				}
			}
		}
	}
	means := each(root, "PaymentMeans")
	for _, pm := range means {
		v.leaves(set, "PaymentReference", pm, "PaymentID")
	}
	v.leaves(set, "TaxCurrencyCode", root, "TaxCurrencyCode")
	v.leaves(set, "InvoiceCurrencyCode", root, "DocumentCurrencyCode")
	for _, p := range each(root, "PayeeParty") {
		v.party(set, "PayeeTradeParty", p, p, rolePayee)
	}
	for _, pm := range means {
		v.paymentMeans(set, pm)
	}

	var taxPoints, dueCodes []*xmltree.Node
	taxPoints = each(root, "TaxPointDate")
	for _, p := range each(root, "InvoicePeriod") {
		dueCodes = append(dueCodes, each(p, "DescriptionCode")...)
	}
	for _, tt := range each(root, "TaxTotal") {
		for _, st := range each(tt, "TaxSubtotal") {
			t := v.ram(set, "ApplicableTradeTax", st)
			v.leaves(t, "CalculatedAmount", st, "TaxAmount")
			cats := each(st, "TaxCategory")
			for _, c := range cats {
				v.taxType(t, c)
			}
			for _, c := range cats {
				v.leaves(t, "ExemptionReason", c, "TaxExemptionReason")
			}
			v.leaves(t, "BasisAmount", st, "TaxableAmount")
			for _, c := range cats {
				v.categoryCode(t, c)
			}
			for _, c := range cats {
				v.leaves(t, "ExemptionReasonCode", c, "TaxExemptionReasonCode")
			}
			for _, d := range taxPoints {
				v.dateOf(t, "TaxPointDate", rules.NSUdt, "udt", "DateString", d)
			}
			for _, c := range dueCodes {
				v.text(t, "DueDateTypeCode", taxPointCode(c.Text), c)
			}
			for _, c := range cats {
				v.leaves(t, "RateApplicablePercent", c, "Percent")
			}
		}
	}
	for _, p := range each(root, "InvoicePeriod") {
		v.period(set, p)
	}
	for _, ac := range each(root, "AllowanceCharge") {
		v.allowanceCharge(set, ac, true)
	}
	v.paymentTerms(set, means)
	for _, m := range each(root, "LegalMonetaryTotal") {
		s := v.ram(set, "SpecifiedTradeSettlementHeaderMonetarySummation", m)
		v.leaves(s, "LineTotalAmount", m, "LineExtensionAmount")
		v.leaves(s, "ChargeTotalAmount", m, "ChargeTotalAmount")
		v.leaves(s, "AllowanceTotalAmount", m, "AllowanceTotalAmount")
		v.leaves(s, "TaxBasisTotalAmount", m, "TaxExclusiveAmount")
		for _, tt := range each(root, "TaxTotal") {
			v.leaves(s, "TaxTotalAmount", tt, "TaxAmount")
		}
		v.leaves(s, "RoundingAmount", m, "PayableRoundingAmount")
		v.leaves(s, "GrandTotalAmount", m, "TaxInclusiveAmount")
		v.leaves(s, "TotalPrepaidAmount", m, "PrepaidAmount")
		v.leaves(s, "DuePayableAmount", m, "PayableAmount")
	}
	for _, b := range each(root, "BillingReference") {
		for _, r := range each(b, "InvoiceDocumentReference") {
			d := v.ram(set, "InvoiceReferencedDocument", r)
			v.leaves(d, "IssuerAssignedID", r, "ID")
			for _, i := range each(r, "IssueDate") {
				v.dateOf(d, "FormattedIssueDateTime", rules.NSQdt, "qdt", "DateTimeString", i)
			}
		}
	}
	v.wrapped(set, "ReceivableSpecifiedTradeAccountingAccount", "ID", root, "AccountingCost")
}

// taxPointCode translates a VAT point date code (BT-8) from UBL to CII.
func taxPointCode(s string) string {
	switch strings.TrimSpace(s) {
	case "3":
		return "5"
	case "35":
		return "29"
	case "432":
		return "72"
	}
	return s
}

func (v *view) paymentMeans(set, pm *xmltree.Node) {
	n := v.ram(set, "SpecifiedTradeSettlementPaymentMeans", pm)
	v.leaves(n, "TypeCode", pm, "PaymentMeansCode")
	for _, c := range each(pm, "PaymentMeansCode") {
		if name, ok := c.LookupAttr("name"); ok {
			v.text(n, "Information", name, c)
		}
	}
	for _, card := range each(pm, "CardAccount") {
		c := v.ram(n, "ApplicableTradeSettlementFinancialCard", card)
		v.leaves(c, "ID", card, "PrimaryAccountNumberID")
		v.leaves(c, "CardholderName", card, "HolderName")
	}
	for _, m := range each(pm, "PaymentMandate") {
		for _, a := range each(m, "PayerFinancialAccount") {
			v.wrapped(n, "PayerPartyDebtorFinancialAccount", "IBANID", a, "ID")
		}
	}
	for _, a := range each(pm, "PayeeFinancialAccount") {
		acc := v.ram(n, "PayeePartyCreditorFinancialAccount", a)
		for _, id := range each(a, "ID") {
			if looksLikeIBAN(id.Text) {
				v.leaf(acc, "IBANID", id)
			} else {
				v.leaf(acc, "ProprietaryID", id)
			}
		}
		v.leaves(acc, "AccountName", a, "Name")
	}
	for _, a := range each(pm, "PayeeFinancialAccount") {
		for _, b := range each(a, "FinancialInstitutionBranch") {
			v.wrapped(n, "PayeeSpecifiedCreditorFinancialInstitution", "BICID", b, "ID")
		}
	}
}

// looksLikeIBAN reports whether s has the shape of an IBAN: two letters, two
// digits, then 11 to 30 letters or digits.
func looksLikeIBAN(s string) bool {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		upper, digit := 'A' <= c && c <= 'Z', '0' <= c && c <= '9'
		switch {
		case i < 2 && !upper, i >= 2 && i < 4 && !digit, !upper && !digit:
			return false
		}
	}
	return true
}

// paymentTerms appends the ram:SpecifiedTradePaymentTerms: the payment terms
// (BT-20), the payment due date (BT-9: cbc:DueDate in an invoice, the
// payment due date of the payment means in a credit note) and the mandate
// reference (BT-89).
func (v *view) paymentTerms(set *xmltree.Node, means []*xmltree.Node) {
	root := v.src
	var notes, dues, mandates []*xmltree.Node
	terms := each(root, "PaymentTerms")
	for _, t := range terms {
		notes = append(notes, each(t, "Note")...)
	}
	if v.credit {
		for _, pm := range means {
			dues = append(dues, each(pm, "PaymentDueDate")...)
		}
	} else {
		dues = each(root, "DueDate")
	}
	for _, pm := range means {
		for _, m := range each(pm, "PaymentMandate") {
			mandates = append(mandates, each(m, "ID")...)
		}
	}
	if len(notes)+len(dues)+len(mandates) == 0 {
		return
	}
	from := root
	if len(terms) > 0 {
		from = terms[0]
	}
	n := v.ram(set, "SpecifiedTradePaymentTerms", from)
	for _, c := range notes {
		v.leaf(n, "Description", c)
	}
	for _, d := range dues {
		v.dateOf(n, "DueDateDateTime", rules.NSUdt, "udt", "DateTimeString", d)
	}
	for _, m := range mandates {
		v.leaf(n, "DirectDebitMandateID", m)
	}
}
