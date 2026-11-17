package cii

import (
	"slices"
	"strings"

	"github.com/ritoon/zerechnung/internal/schema"
	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// This file reads ZUGFeRD 1.0 invoices. Their syntax, rsm:CrossIndustryDocument,
// is the CII of 2013, before D16B: most elements kept their name, a few
// containers and leaves were renamed and the namespaces changed. A ZUGFeRD
// 1.0 document is read through a CII D16B view of it, built by renaming its
// elements, which the CII reader then reads like any CrossIndustryInvoice.
// Only the syntax is translated: the values, codes included, are kept as
// they are.

// Namespaces of the root element of ZUGFeRD 1.0 invoices.
const (
	// NamespaceZUGFeRD1 is the namespace of rsm:CrossIndustryDocument, the
	// root element of a ZUGFeRD 1.0 invoice.
	NamespaceZUGFeRD1 = "urn:ferd:CrossIndustryDocument:invoice:1p0"
	// NamespaceZUGFeRD1RC is the namespace of rsm:Invoice, the root element
	// of the invoices made with the ZUGFeRD 1.0 release candidate (2014),
	// whose other elements have no namespace.
	NamespaceZUGFeRD1RC = "urn:un:unece:uncefact:data:standard:CBFBUY:5"

	nsZF1RAM = "urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:12"
	nsZF1UDT = "urn:un:unece:uncefact:data:standard:UnqualifiedDataType:15"
)

// IsZUGFeRD1 reports whether root is the root element of a ZUGFeRD 1.0
// invoice: rsm:CrossIndustryDocument in namespace NamespaceZUGFeRD1, or
// rsm:Invoice in namespace NamespaceZUGFeRD1RC (release candidate).
func IsZUGFeRD1(root *xmltree.Node) bool {
	return root != nil && (root.Name == "CrossIndustryDocument" && root.Space == NamespaceZUGFeRD1 ||
		root.Name == "Invoice" && root.Space == NamespaceZUGFeRD1RC)
}

// zf1Names gives the CII D16B name of a ZUGFeRD 1.0 element, keyed by the
// D16B name of its parent and its own name. The elements of a referenced
// document are renamed by zf1Name.
var zf1Names = map[[2]string]string{
	{"CrossIndustryInvoice", "SpecifiedExchangedDocumentContext"}:                    "ExchangedDocumentContext",
	{"CrossIndustryInvoice", "HeaderExchangedDocument"}:                              "ExchangedDocument",
	{"CrossIndustryInvoice", "SpecifiedSupplyChainTradeTransaction"}:                 "SupplyChainTradeTransaction",
	{"SupplyChainTradeTransaction", "ApplicableSupplyChainTradeAgreement"}:           "ApplicableHeaderTradeAgreement",
	{"SupplyChainTradeTransaction", "ApplicableSupplyChainTradeDelivery"}:            "ApplicableHeaderTradeDelivery",
	{"SupplyChainTradeTransaction", "ApplicableSupplyChainTradeSettlement"}:          "ApplicableHeaderTradeSettlement",
	{"ApplicableHeaderTradeAgreement", "CustomerOrderReferencedDocument"}:            "UltimateCustomerOrderReferencedDocument",
	{"ApplicableHeaderTradeSettlement", "SpecifiedTradeSettlementMonetarySummation"}: "SpecifiedTradeSettlementHeaderMonetarySummation",
	{"IncludedSupplyChainTradeLineItem", "SpecifiedSupplyChainTradeAgreement"}:       "SpecifiedLineTradeAgreement",
	{"IncludedSupplyChainTradeLineItem", "SpecifiedSupplyChainTradeDelivery"}:        "SpecifiedLineTradeDelivery",
	{"IncludedSupplyChainTradeLineItem", "SpecifiedSupplyChainTradeSettlement"}:      "SpecifiedLineTradeSettlement",
	{"SpecifiedLineTradeAgreement", "CustomerOrderReferencedDocument"}:               "UltimateCustomerOrderReferencedDocument",
	{"SpecifiedLineTradeSettlement", "SpecifiedTradeSettlementMonetarySummation"}:    "SpecifiedTradeSettlementLineMonetarySummation",
	{"SpecifiedLineTradeSettlement", "SpecifiedTradeAccountingAccount"}:              "ReceivableSpecifiedTradeAccountingAccount",
	{"ApplicableTradeTax", "ApplicablePercent"}:                                      "RateApplicablePercent",
	{"CategoryTradeTax", "ApplicablePercent"}:                                        "RateApplicablePercent",
	{"AppliedTradeTax", "ApplicablePercent"}:                                         "RateApplicablePercent",
}

// zf1Name returns the CII D16B name of a child name of a ZUGFeRD 1.0 element
// whose D16B name is parent.
func zf1Name(parent, name string) string {
	if d, ok := zf1Names[[2]string{parent, name}]; ok {
		return d
	}
	if strings.HasSuffix(parent, "ReferencedDocument") {
		switch name {
		case "ID":
			return "IssuerAssignedID"
		case "IssueDateTime":
			return "FormattedIssueDateTime"
		}
	}
	return name
}

// zf1View builds the CII D16B view of a ZUGFeRD 1.0 document.
type zf1View struct {
	// origin maps each node of the view to the node of the ZUGFeRD 1.0
	// document it comes from.
	origin map[*xmltree.Node]*xmltree.Node
}

// zugferd1View returns the CII D16B view of root, the root element of a
// ZUGFeRD 1.0 invoice (see IsZUGFeRD1), and the map from each node of the
// view to the node of root it comes from. The document is not modified.
//
// The view renames the elements (zf1Names, and in a referenced document ID
// and IssueDateTime), puts them in the CII namespaces and in the order of
// the EXTENDED schema, wraps the values that ZUGFeRD 1.0 writes as text into
// udt:DateTimeString, qdt:DateTimeString or udt:Indicator, writes the dates
// in format 102 (see zf1Date), and moves the SEPA direct debit data of the first
// payment means that carries it, the mandate reference (ram:ID) and the
// creditor identifier (its schemeAgencyID), to BT-89 and BT-90. Elements
// that CII D16B does not know keep their name: the reader keeps them as
// extensions, which Marshal reports as lost.
func zugferd1View(root *xmltree.Node) (*xmltree.Node, map[*xmltree.Node]*xmltree.Node) {
	v := zf1View{origin: make(map[*xmltree.Node]*xmltree.Node)}
	return v.node(root, nil, "CrossIndustryInvoice", schema.Root(invoice.ProfileExtended)), v.origin
}

// node returns the view of o, named name, child of parent in the view, whose
// type in the EXTENDED schema is t (invalid when the schema does not know
// it).
func (v *zf1View) node(o, parent *xmltree.Node, name string, t schema.Type) *xmltree.Node {
	n := &xmltree.Node{Name: name, Space: zf1Space(o, t), Attrs: o.Attrs, Text: o.Text, Parent: parent, Line: o.Line}
	n.Prefix = zf1Prefix(n.Space, o.Prefix)
	v.origin[n] = o
	if len(o.Children) == 0 && strings.TrimSpace(o.Text) != "" {
		switch {
		case t.Child("DateTimeString").Valid():
			v.dateTime(n, o, t.Child("DateTimeString"))
		case t.Child("Indicator").Valid():
			v.wrap(n, o, t.Child("Indicator"), o.Text, nil)
		case name == "DateTimeString":
			n.Text, n.Attrs = zf1Date(o.Text, o.Attrs)
		}
		return n
	}
	if len(o.Children) > 0 {
		n.Children = make([]*xmltree.Node, 0, len(o.Children))
		for _, c := range o.Children {
			cn := zf1Name(name, c.Name)
			n.Children = append(n.Children, v.node(c, n, cn, t.Child(cn)))
		}
	}
	if name == "ApplicableHeaderTradeSettlement" {
		v.directDebit(n, t)
	}
	sortBySchema(n, t)
	return n
}

// zf1Space returns the namespace of the view of o, whose type in the
// EXTENDED schema is t: the namespace of t, or for an element that the
// schema does not know, the CII namespace that matches its ZUGFeRD 1.0
// namespace. Elements in other namespaces keep theirs.
func zf1Space(o *xmltree.Node, t schema.Type) string {
	switch o.Space {
	case NamespaceZUGFeRD1, NamespaceZUGFeRD1RC, nsZF1RAM, nsZF1UDT, "":
	default:
		return o.Space
	}
	switch {
	case t.Valid():
		return t.Space()
	case o.Space == nsZF1UDT:
		return NamespaceUDT
	case o.Space == NamespaceZUGFeRD1 || o.Space == NamespaceZUGFeRD1RC:
		return NamespaceRSM
	}
	return NamespaceRAM
}

// zf1Prefix returns the prefix of an element of the view: the conventional
// one of the CII namespaces, the prefix of the document otherwise.
func zf1Prefix(space, prefix string) string {
	if p := nsPrefix(space); p != "" {
		return p
	}
	return prefix
}

// wrap makes the view n of o, a leaf whose value ZUGFeRD 1.0 writes as text,
// hold that value in a child element of type t, with the attributes attrs.
func (v *zf1View) wrap(n, o *xmltree.Node, t schema.Type, text string, attrs []xmltree.Attr) {
	c := &xmltree.Node{Name: t.Name(), Space: t.Space(), Attrs: attrs, Text: text, Parent: n, Line: o.Line}
	c.Prefix = zf1Prefix(c.Space, "")
	v.origin[c] = o
	n.Text = ""
	n.Children = []*xmltree.Node{c}
}

// dateTime makes the view n of o, a date written as text (in a referenced
// document, or anywhere in the release candidate), hold it in a
// DateTimeString child of type t, with the format attribute of o.
func (v *zf1View) dateTime(n, o *xmltree.Node, t schema.Type) {
	var attrs []xmltree.Attr
	if f := attr(o, "format"); f != "" {
		attrs = []xmltree.Attr{{Name: "format", Value: f}}
	}
	text, attrs := zf1Date(o.Text, attrs)
	n.Attrs = withoutAttr(o.Attrs, "format")
	v.wrap(n, o, t, text, attrs)
}

// zf1Date returns the text and the attributes of a ZUGFeRD 1.0 date in
// format 102, the only one of CII D16B: a date that the reader can read in
// another format (610, 616, an ISO 8601 date, possibly followed by a time as
// in a referenced document) or without a format is written in format 102,
// as the reader would read it. A date that cannot be read is kept as it is,
// for the reader to report it.
func zf1Date(text string, attrs []xmltree.Attr) (string, []xmltree.Attr) {
	s, format := strings.TrimSpace(text), ""
	for _, a := range attrs {
		if a.Prefix == "" && a.Name == "format" {
			format = strings.TrimSpace(a.Value)
		}
	}
	if len(s) > 10 && s[10] == 'T' {
		s = s[:10] // the time of an xs:dateTime
	}
	d, err := invoice.ParseDate(s, format)
	if err != nil {
		return text, attrs
	}
	return d.Format102(), append(slices.Clip(withoutAttr(attrs, "format")), xmltree.Attr{Name: "format", Value: "102"})
}

// withoutAttr returns attrs without the attribute named name, sharing attrs
// when it has none.
func withoutAttr(attrs []xmltree.Attr, name string) []xmltree.Attr {
	i := slices.IndexFunc(attrs, func(a xmltree.Attr) bool { return a.Prefix == "" && a.Name == name })
	if i < 0 {
		return attrs
	}
	out := make([]xmltree.Attr, 0, len(attrs)-1)
	out = append(out, attrs[:i]...)
	return append(out, attrs[i+1:]...)
}

// directDebit moves the SEPA direct debit data of the first payment means
// that has a ram:ID, in st, the view of the header settlement whose type is
// t: ZUGFeRD 1.0 writes the mandate reference in that ID and the creditor
// identifier in its schemeAgencyID, where CII D16B has BT-89
// (SpecifiedTradePaymentTerms/DirectDebitMandateID) and BT-90
// (CreditorReferenceID). The ID stays in place when BT-89 is already there.
func (v *zf1View) directDebit(st *xmltree.Node, t schema.Type) {
	var pm, id *xmltree.Node
	for _, c := range st.Children {
		if c.Name == "SpecifiedTradeSettlementPaymentMeans" {
			if id = child(c, "ID"); id != nil {
				pm = c
				break
			}
		}
	}
	if pm == nil {
		return
	}
	terms := child(st, "SpecifiedTradePaymentTerms")
	if child(terms, "DirectDebitMandateID") != nil {
		return
	}
	tt := t.Child("SpecifiedTradePaymentTerms")
	if terms == nil {
		terms = &xmltree.Node{Name: tt.Name(), Space: tt.Space(), Prefix: zf1Prefix(tt.Space(), ""), Parent: st, Line: id.Line}
		v.origin[terms] = v.origin[id]
		st.Children = append(st.Children, terms)
	}
	pm.Children = slices.DeleteFunc(pm.Children, func(c *xmltree.Node) bool { return c == id })
	if a := attr(id, "schemeAgencyID"); a != "" && child(st, "CreditorReferenceID") == nil {
		ct := t.Child("CreditorReferenceID")
		cr := &xmltree.Node{Name: ct.Name(), Space: ct.Space(), Prefix: zf1Prefix(ct.Space(), ""), Text: a, Parent: st, Line: id.Line}
		v.origin[cr] = v.origin[id]
		st.Children = append(st.Children, cr)
	}
	id.Name, id.Parent = "DirectDebitMandateID", terms
	id.Attrs = withoutAttr(id.Attrs, "schemeAgencyID")
	terms.Children = append(terms.Children, id)
	sortBySchema(terms, tt)
}

// sortBySchema puts the children of n, whose type is t, in the order of the
// schema. An element that the schema does not know stays after the element
// it follows; elements of the same name keep their order.
func sortBySchema(n *xmltree.Node, t schema.Type) {
	if len(n.Children) < 2 || !t.Valid() {
		return
	}
	type keyed struct {
		pos int
		n   *xmltree.Node
	}
	ks := make([]keyed, len(n.Children))
	pos := -1
	for i, c := range n.Children {
		if ct := t.Child(c.Name); ct.Valid() {
			pos = ct.Pos()
		}
		ks[i] = keyed{pos, c}
	}
	slices.SortStableFunc(ks, func(a, b keyed) int { return a.pos - b.pos })
	for i := range ks {
		n.Children[i] = ks[i].n
	}
}
