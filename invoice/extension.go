package invoice

import (
	"strconv"
	"strings"
)

// Extension is a piece of the source document that no other field of the
// model carries: an element of the EXTENDED data set that the model does not
// describe (a fax number, a product brand name, a payment discount term…), an
// attribute such as a listID or a schemeID, or an element that no Factur-X
// schema knows. Readers keep it so that nothing is dropped silently, and
// writers put it back at its place when the target profile allows it (the
// CII writer reports it as lost otherwise).
//
// An extension belongs to the value that represents its nearest enclosing
// element in the model (the invoice, a line, a party, a payment means…) and is
// stored in that value's Extensions field. It is tied to the syntax it was read
// from: CII local element names, and the conventional rsm, ram, udt and qdt
// prefixes. An extension that depends on a value of the model (a listID on a
// code, for instance) is not updated when that value changes.
type Extension struct {
	// Path locates the element that contains the extension, relative to the
	// element of the value that holds it: "" for that element itself, or
	// local names separated by "/", such as "SpecifiedLineTradeDelivery" or
	// "SpecifiedTradeProduct/ApplicableProductCharacteristic". For an
	// attribute, Path locates the element that carries it; its last name ends
	// with an occurrence number such as "[2]" when that element is not the
	// first child of that name.
	Path string
	// XML is the element, with its content, serialized with the conventional
	// prefixes ("<ram:BrandName>Acme</ram:BrandName>"). It is empty for an
	// attribute.
	XML string
	// Attr and Value are the local name and the value of an attribute.
	Attr, Value string
}

// IsAttr reports whether the extension is an attribute.
func (e Extension) IsAttr() bool { return e.XML == "" && e.Attr != "" }

// Name returns the local name of the element, or "@" followed by the name of
// the attribute.
func (e Extension) Name() string {
	if e.IsAttr() {
		return "@" + e.Attr
	}
	s := strings.TrimPrefix(e.XML, "<")
	if i := strings.IndexAny(s, " \t\r\n/>"); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// String returns the location of the extension relative to the value that
// holds it: "SpecifiedTradeProduct/BrandName", "ExchangedDocument/ID/@schemeID".
func (e Extension) String() string {
	if e.Path == "" {
		return e.Name()
	}
	return e.Path + "/" + e.Name()
}

// HasExtensions reports whether the invoice or one of its values holds
// extensions.
func (inv *Invoice) HasExtensions() bool {
	found := false
	inv.walkExtensions(func(extOwner, []Extension) { found = true })
	return found
}

// Unmodeled lists the extensions of the invoice: the elements and attributes
// of the source document that the fields of the model do not carry. Each
// entry gives the value that holds the extension, as a Go expression relative
// to the invoice, then the location of the extension relative to the element
// of that value (see Extension.String):
//
//	SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement/QuotationReferencedDocument
//	Seller: DefinedTradeContact
//	Lines[0]: SpecifiedTradeProduct/BrandName
//	Lines[0].Item.Classifications[1]: ClassCode/@listURI
//
// The entries of the invoice itself have no prefix.
func (inv *Invoice) Unmodeled() []string {
	var out []string
	inv.walkExtensions(func(o extOwner, exts []Extension) {
		prefix := o.String()
		if prefix != "" {
			prefix += ": "
		}
		for i := range exts {
			out = append(out, prefix+exts[i].String())
		}
	})
	return out
}

// extOwner names a value that holds extensions: field[i].sub[j], i and j
// being -1 when absent.
type extOwner struct {
	field string
	i     int
	sub   string
	j     int
}

func (o extOwner) String() string {
	s := o.field
	if o.i >= 0 {
		s += "[" + strconv.Itoa(o.i) + "]"
	}
	if o.sub != "" {
		s += "." + o.sub
		if o.j >= 0 {
			s += "[" + strconv.Itoa(o.j) + "]"
		}
	}
	return s
}

// walkExtensions calls f for each value of the invoice that holds
// extensions, in document order.
func (inv *Invoice) walkExtensions(f func(extOwner, []Extension)) {
	visit := func(field string, i int, sub string, j int, exts []Extension) {
		if len(exts) > 0 {
			f(extOwner{field, i, sub, j}, exts)
		}
	}
	party := func(field string, i int, sub string, p *Party) {
		if p != nil {
			visit(field, i, sub, -1, p.Extensions)
		}
	}
	ref := func(field string, i int, sub string, r *DocumentReference) {
		if r != nil {
			visit(field, i, sub, -1, r.Extensions)
		}
	}
	visit("", -1, "", -1, inv.Extensions)
	for i := range inv.Notes {
		visit("Notes", i, "", -1, inv.Notes[i].Extensions)
	}
	for i := range inv.Lines {
		l := &inv.Lines[i]
		visit("Lines", i, "", -1, l.Extensions)
		for j := range l.Item.Attributes {
			visit("Lines", i, "Item.Attributes", j, l.Item.Attributes[j].Extensions)
		}
		for j := range l.Item.Classifications {
			visit("Lines", i, "Item.Classifications", j, l.Item.Classifications[j].Extensions)
		}
		for j := range l.Item.ReferencedProducts {
			visit("Lines", i, "Item.ReferencedProducts", j, l.Item.ReferencedProducts[j].Extensions)
		}
		for j := range l.PriceAllowancesCharges {
			visit("Lines", i, "PriceAllowancesCharges", j, l.PriceAllowancesCharges[j].Extensions)
		}
		for j := range l.UltimateCustomerOrders {
			visit("Lines", i, "UltimateCustomerOrders", j, l.UltimateCustomerOrders[j].Extensions)
		}
		party("Lines", i, "ShipTo", l.ShipTo)
		party("Lines", i, "UltimateShipTo", l.UltimateShipTo)
		ref("Lines", i, "DeliveryNote", l.DeliveryNote)
		for j := range l.AllowancesCharges {
			visit("Lines", i, "AllowancesCharges", j, l.AllowancesCharges[j].Extensions)
		}
		ref("Lines", i, "InvoiceReference", l.InvoiceReference)
	}
	party("Seller", -1, "", &inv.Seller)
	party("Buyer", -1, "", &inv.Buyer)
	party("SalesAgent", -1, "", inv.SalesAgent)
	party("SellerTaxRepresentative", -1, "", inv.SellerTaxRepresentative)
	party("ProductEndUser", -1, "", inv.ProductEndUser)
	for i := range inv.AdditionalDocuments {
		visit("AdditionalDocuments", i, "", -1, inv.AdditionalDocuments[i].Extensions)
	}
	party("BuyerAgent", -1, "", inv.BuyerAgent)
	for i := range inv.UltimateCustomerOrders {
		visit("UltimateCustomerOrders", i, "", -1, inv.UltimateCustomerOrders[i].Extensions)
	}
	party("ShipTo", -1, "", inv.ShipTo)
	party("UltimateShipTo", -1, "", inv.UltimateShipTo)
	party("ShipFrom", -1, "", inv.ShipFrom)
	ref("DeliveryNote", -1, "", inv.DeliveryNote)
	party("Invoicer", -1, "", inv.Invoicer)
	party("Invoicee", -1, "", inv.Invoicee)
	party("Payee", -1, "", inv.Payee)
	party("Payer", -1, "", inv.Payer)
	for i := range inv.PaymentMeans {
		visit("PaymentMeans", i, "", -1, inv.PaymentMeans[i].Extensions)
	}
	for i := range inv.VATBreakdown {
		visit("VATBreakdown", i, "", -1, inv.VATBreakdown[i].Extensions)
	}
	for i := range inv.AllowancesCharges {
		visit("AllowancesCharges", i, "", -1, inv.AllowancesCharges[i].Extensions)
	}
	for i := range inv.LogisticsCharges {
		visit("LogisticsCharges", i, "", -1, inv.LogisticsCharges[i].Extensions)
	}
	for i := range inv.PaymentTerms {
		visit("PaymentTerms", i, "", -1, inv.PaymentTerms[i].Extensions)
	}
	for i := range inv.ThirdPartyCharges {
		visit("ThirdPartyCharges", i, "", -1, inv.ThirdPartyCharges[i].Extensions)
	}
	for i := range inv.PrecedingInvoices {
		visit("PrecedingInvoices", i, "", -1, inv.PrecedingInvoices[i].Extensions)
	}
}
