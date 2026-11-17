package cii

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// ErrNotCII is returned (wrapped) by Parse when the document is well-formed
// XML but neither a rsm:CrossIndustryInvoice nor a ZUGFeRD 1.0
// rsm:CrossIndustryDocument, for example a UBL invoice.
var ErrNotCII = errors.New("cii: not a CrossIndustryInvoice document")

// Issue is a value that could not be read: a malformed amount, quantity,
// date, indicator or base64 attachment. The corresponding field of the
// invoice is left empty.
type Issue struct {
	Path    string // location of the element, as given by xmltree.Node.Path
	Line    int    // line of the element in the document, 1-based
	Message string
}

// String formats the issue as "line <Line>: <Path>: <Message>".
func (i Issue) String() string {
	return "line " + strconv.Itoa(i.Line) + ": " + i.Path + ": " + i.Message
}

// IssuesError is the error returned by Parse, together with a non-nil
// invoice, when some values could not be read. Everything else was read: a
// caller that tolerates incomplete data can use the invoice and inspect
// Issues with errors.As.
type IssuesError struct {
	Issues []Issue
}

// Error gives the number of issues and the first one.
func (e *IssuesError) Error() string {
	if len(e.Issues) == 1 {
		return "cii: unreadable value at " + e.Issues[0].String()
	}
	return "cii: " + strconv.Itoa(len(e.Issues)) + " unreadable values, first at " + e.Issues[0].String()
}

// Parse reads a CII invoice.
//
// It fails, with a nil invoice, only when data is not well-formed XML or its
// root is not a rsm:CrossIndustryInvoice nor a ZUGFeRD 1.0 invoice (the
// error then wraps ErrNotCII). When some values cannot be read, Parse
// returns the invoice with the corresponding fields empty and an
// *IssuesError listing them.
//
// Reading is tolerant: namespace prefixes are ignored (the root namespace is
// checked), values are trimmed, elements are recognized in any order, and the
// ZUGFeRD 2.0/2.1 variants are accepted. The profile is derived from the
// specification identifier BT-24, which is kept in SpecificationID.
//
// ZUGFeRD 1.0 invoices (rsm:CrossIndustryDocument, and rsm:Invoice of the
// 2014 release candidate, see IsZUGFeRD1) are read too, from a CII D16B view
// of the document: the elements that ZUGFeRD 2 renamed are read under their
// new name and the extensions get their CII D16B path, so that Marshal
// writes the invoice as Factur-X. Only the syntax is translated: the values,
// codes included, are read as they are. The profile is the one matched to
// the ZUGFeRD 1.0 identifier (see invoice.ProfileFromZUGFeRD1URN), BT-24
// keeps that identifier and the issues are located in the ZUGFeRD 1.0
// document. ZUGFeRD 1.0 is read only: Marshal writes CII D16B.
//
// Nothing is dropped: the elements and attributes that the model does not
// carry (EXTENDED elements without a field of their own, a listID, an
// element that no schema knows…) are kept as extensions, in the Extensions
// field of the value that encloses them; Invoice.Unmodeled lists them.
// Marshal writes them back at their place when the target profile allows
// them, and reports them as lost otherwise. A currencyID equal to the invoice
// or VAT accounting currency is implied by BT-5 and BT-6, and attributes in
// other namespaces (xsi:schemaLocation) are not kept.
func Parse(data []byte) (*invoice.Invoice, error) {
	root, err := xmltree.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("cii: %w", err)
	}
	if err := checkRoot(root); err != nil {
		return nil, err
	}
	inv, issues := ParseTree(root)
	if len(issues) > 0 {
		return inv, &IssuesError{Issues: issues}
	}
	return inv, nil
}

// ParseTree reads a CII invoice from an already parsed document, so that a
// caller that also validates the document parses the XML only once. It
// returns the invoice and the values that could not be read (see Parse). If
// root is neither a rsm:CrossIndustryInvoice element nor the root of a
// ZUGFeRD 1.0 invoice, the invoice is nil and the single issue says why.
// The tree is not modified.
func ParseTree(root *xmltree.Node) (*invoice.Invoice, []Issue) {
	if err := checkRoot(root); err != nil {
		is := Issue{Message: err.Error()}
		if root != nil {
			is.Path, is.Line = root.Path(), root.Line
		}
		return nil, []Issue{is}
	}
	r := reader{inv: new(invoice.Invoice)}
	if IsZUGFeRD1(root) {
		root, r.origin = zugferd1View(root)
	}
	r.document(root)
	return r.inv, r.issues
}

func checkRoot(root *xmltree.Node) error {
	switch {
	case root == nil:
		return ErrNotCII
	case IsZUGFeRD1(root):
		return nil
	case root.Name == "CrossIndustryDocument":
		return fmt.Errorf("%w: CrossIndustryDocument in namespace %q, not the ZUGFeRD 1.0 namespace %s", ErrNotCII, root.Space, NamespaceZUGFeRD1)
	case root.Name != "CrossIndustryInvoice":
		return fmt.Errorf("%w: root element is <%s>", ErrNotCII, root.Name)
	case root.Space != NamespaceRSM:
		return fmt.Errorf("%w: root element in namespace %q", ErrNotCII, root.Space)
	}
	return nil
}

// reader maps a CII tree to the model.
//
// Every element and attribute of the document is either read into a field of
// the model or kept as an extension: each container function handles the
// children it knows and passes the others to unread. Extensions go to the
// value being read whose Extensions field is ext, with a path relative to the
// element of that value (path holds its names).
type reader struct {
	inv    *invoice.Invoice
	issues []Issue

	ext  *[]invoice.Extension
	path []string

	// cur and taxCur are the invoice and VAT accounting currencies: a
	// currencyID equal to one of them is implied by BT-5 and BT-6.
	cur, taxCur string

	// origin, for a ZUGFeRD 1.0 invoice, maps each node of the CII D16B view
	// being read to the node of the document it comes from, where issues
	// are located.
	origin map[*xmltree.Node]*xmltree.Node

	// unreadPaths, when not nil, receives the location of each extension
	// (tests use it to list the elements outside the model).
	unreadPaths *[]string
	buf         []byte
}

func (r *reader) issue(n *xmltree.Node, msg string) {
	if o := r.origin[n]; o != nil {
		n = o
	}
	r.issues = append(r.issues, Issue{Path: n.Path(), Line: n.Line, Message: msg})
}

// holder is the state saved by hold.
type holder struct {
	ext  *[]invoice.Extension
	path []string
}

// hold makes the value whose Extensions field is ext the receiver of the
// extensions found until release, with paths relative to n, its element.
// The attributes of n itself are kept with an empty path.
func (r *reader) hold(n *xmltree.Node, ext *[]invoice.Extension) holder {
	h := holder{r.ext, r.path}
	r.ext, r.path = ext, r.path[len(r.path):]
	if len(n.Attrs) > 0 {
		r.keepAttrs(n, "", "", "", "")
	}
	return h
}

func (r *reader) release(h holder) { r.ext, r.path = h.ext, h.path }

// enter starts reading the children of the container n: its attributes are
// kept as extensions and its name extends the path.
func (r *reader) enter(n *xmltree.Node) {
	if len(n.Attrs) > 0 {
		r.keepAttrs(n, r.attrPath(n), "", "", "")
	}
	r.path = append(r.path, n.Name)
}

func (r *reader) leave() { r.path = r.path[:len(r.path)-1] }

func (r *reader) pathString() string { return strings.Join(r.path, "/") }

// unread keeps n, a child of the current container, as an extension.
func (r *reader) unread(n *xmltree.Node) {
	r.buf = appendElement(r.buf[:0], n, false, 0)
	e := invoice.Extension{Path: r.pathString(), XML: string(r.buf)}
	*r.ext = append(*r.ext, e)
	if r.unreadPaths != nil {
		*r.unreadPaths = append(*r.unreadPaths, n.Path())
	}
}

// keepAttrs keeps the attributes of n, whose location is path (see
// attrPath), except those named a, b and c, which the caller reads.
// Attributes in another namespace (xsi:schemaLocation…) are not part of the
// data.
func (r *reader) keepAttrs(n *xmltree.Node, path, a, b, c string) {
	for _, at := range n.Attrs {
		if at.Prefix != "" || at.Name == a || at.Name == b || at.Name == c {
			continue
		}
		*r.ext = append(*r.ext, invoice.Extension{Path: path, Attr: at.Name, Value: at.Value})
		if r.unreadPaths != nil {
			*r.unreadPaths = append(*r.unreadPaths, n.Path()+"/@"+at.Name)
		}
	}
}

// attrPath returns the location of n, a child of the current container:
// the current path followed by the name of n, with its occurrence number
// when it is not the first child of that name.
func (r *reader) attrPath(n *xmltree.Node) string {
	name := n.Name
	if p := n.Parent; p != nil {
		k := 0
		for _, s := range p.Children {
			if s == n {
				break
			}
			if s.Name == n.Name {
				k++
			}
		}
		if k > 0 {
			name += "[" + strconv.Itoa(k+1) + "]"
		}
	}
	if len(r.path) == 0 {
		return name
	}
	return r.pathString() + "/" + name
}

// leafChildren keeps the children of a leaf element (a document error) as
// extensions.
func (r *reader) leafChildren(n *xmltree.Node) {
	r.path = append(r.path, n.Name)
	for _, c := range n.Children {
		r.unread(c)
	}
	r.leave()
}

// text returns the trimmed text of the leaf n ("" for nil) and keeps its
// attributes.
func (r *reader) text(n *xmltree.Node) string {
	if n == nil {
		return ""
	}
	if len(n.Attrs) > 0 {
		r.keepAttrs(n, r.attrPath(n), "", "", "")
	}
	if len(n.Children) > 0 {
		r.leafChildren(n)
	}
	return strings.TrimSpace(n.Text)
}

// textAttrs is text for a leaf whose attributes a, b and c are read by the
// caller.
func (r *reader) textAttrs(n *xmltree.Node, a, b, c string) string {
	if len(n.Attrs) > 0 {
		r.keepAttrs(n, r.attrPath(n), a, b, c)
	}
	if len(n.Children) > 0 {
		r.leafChildren(n)
	}
	return strings.TrimSpace(n.Text)
}

// attr returns the trimmed value of an attribute of n.
func attr(n *xmltree.Node, name string) string {
	for i := range n.Attrs {
		if n.Attrs[i].Name == name {
			return strings.TrimSpace(n.Attrs[i].Value)
		}
	}
	return ""
}

// ident reads an identifier element with its schemeID.
func (r *reader) ident(n *xmltree.Node) invoice.Identifier {
	return invoice.Identifier{ID: r.textAttrs(n, "schemeID", "", ""), Scheme: attr(n, "schemeID")}
}

// decimal reads an amount, a rate or a percentage. A currencyID equal to
// the invoice or VAT accounting currency is implied; another one is kept.
func (r *reader) decimal(n *xmltree.Node) invoice.Decimal {
	var s string
	if c := attr(n, "currencyID"); c != "" && (c == r.cur || c == r.taxCur) {
		s = r.textAttrs(n, "currencyID", "", "")
	} else {
		s = r.text(n)
	}
	return r.parseDecimal(n, s)
}

// quantity reads a quantity and its unit code.
func (r *reader) quantity(n *xmltree.Node) (invoice.Decimal, string) {
	return r.parseDecimal(n, r.textAttrs(n, "unitCode", "", "")), attr(n, "unitCode")
}

func (r *reader) parseDecimal(n *xmltree.Node, s string) invoice.Decimal {
	if s == "" {
		return invoice.Decimal{}
	}
	d, err := invoice.ParseDecimal(s)
	if err != nil {
		r.issue(n, err.Error())
		return invoice.Decimal{}
	}
	return d
}

// date reads a udt:DateTimeType, udt:DateType or qdt:FormattedDateTimeType
// element; the text of n itself is used when it has no DateTimeString child.
func (r *reader) date(n *xmltree.Node) invoice.Date {
	v := n
	r.enter(n)
	for _, c := range n.Children {
		if (c.Name == "DateTimeString" || c.Name == "DateString") && v == n {
			v = c
			continue
		}
		r.unread(c)
	}
	var s string
	if v == n {
		s = strings.TrimSpace(n.Text)
	} else {
		s = r.textAttrs(v, "format", "", "")
	}
	r.leave()
	if s == "" {
		return invoice.Date{}
	}
	d, err := invoice.ParseDate(s, attr(v, "format"))
	if err != nil {
		r.issue(v, err.Error())
		return invoice.Date{}
	}
	return d
}

// indicator reads a udt:IndicatorType element.
func (r *reader) indicator(n *xmltree.Node) bool {
	v := n
	r.enter(n)
	for _, c := range n.Children {
		if c.Name == "Indicator" && v == n {
			v = c
			continue
		}
		r.unread(c)
	}
	s := strings.TrimSpace(n.Text)
	if v != n {
		s = r.text(v)
	}
	r.leave()
	switch s {
	case "true", "1":
		return true
	case "false", "0", "":
		return false
	default:
		r.issue(v, "invalid indicator "+strconv.Quote(s))
	}
	return false
}

// binary decodes a base64 binary object, ignoring white space.
func (r *reader) binary(n *xmltree.Node) []byte {
	r.textAttrs(n, "mimeCode", "filename", "")
	s := unspace(n.Text)
	if len(s) == 0 {
		return nil
	}
	b := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
	k, err := base64.StdEncoding.Decode(b, s)
	if err != nil {
		s = bytes.TrimRight(s, "=")
		b = make([]byte, base64.RawStdEncoding.DecodedLen(len(s)))
		var err2 error
		if k, err2 = base64.RawStdEncoding.Decode(b, s); err2 != nil {
			r.issue(n, "invalid base64 content: "+err.Error())
			return nil
		}
	}
	return b[:k]
}

// unspace returns s without XML white space.
func unspace(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		j := i
		for j < len(s) && (s[j] > ' ' || !isXMLSpace(s[j])) {
			j++
		}
		out = append(out, s[i:j]...)
		for i = j; i < len(s) && isXMLSpace(s[i]); i++ {
		}
	}
	return out
}

func isXMLSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// child returns the first child element of n with the given local name
// (nil when n is nil).
func child(n *xmltree.Node, name string) *xmltree.Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// textOf returns the trimmed text of n without keeping anything ("" for
// nil); it is used to look ahead.
func textOf(n *xmltree.Node) string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Text)
}

func (r *reader) document(root *xmltree.Node) {
	inv := r.inv
	st := child(child(root, "SupplyChainTradeTransaction"), "ApplicableHeaderTradeSettlement")
	r.cur, r.taxCur = textOf(child(st, "InvoiceCurrencyCode")), textOf(child(st, "TaxCurrencyCode"))
	h := r.hold(root, &inv.Extensions)
	for _, c := range root.Children {
		switch c.Name {
		case "ExchangedDocumentContext":
			r.context(c)
		case "ExchangedDocument":
			r.exchangedDocument(c)
		case "SupplyChainTradeTransaction":
			r.transaction(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	inv.Profile = detectProfile(inv.SpecificationID)
}

func (r *reader) context(n *xmltree.Node) {
	inv := r.inv
	r.enter(n)
	for _, c := range n.Children {
		switch {
		case c.Name == "TestIndicator":
			inv.TestIndicator = r.indicator(c)
		case c.Name == "BusinessProcessSpecifiedDocumentContextParameter" && inv.BusinessProcess == "":
			inv.BusinessProcess = r.idOf(c)
		case c.Name == "GuidelineSpecifiedDocumentContextParameter" && inv.SpecificationID == "":
			inv.SpecificationID = r.idOf(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
}

// idOf reads a container whose only child is an ID (document context
// parameter, procuring project, accounting account…) and returns the ID.
func (r *reader) idOf(n *xmltree.Node) string {
	return r.childText(n, "ID")
}

// childText reads a container holding a single known leaf, name, and
// returns its text. The other children are kept as extensions.
func (r *reader) childText(n *xmltree.Node, name string) string {
	var v string
	seen := false
	r.enter(n)
	for _, c := range n.Children {
		if c.Name == name && !seen {
			seen = true
			v = r.text(c)
			continue
		}
		r.unread(c)
	}
	r.leave()
	return v
}

func (r *reader) exchangedDocument(n *xmltree.Node) {
	inv := r.inv
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "ID":
			inv.Number = r.text(c)
		case "TypeCode":
			inv.TypeCode = r.text(c)
		case "IssueDateTime":
			inv.IssueDate = r.date(c)
		case "CopyIndicator":
			inv.CopyIndicator = r.indicator(c)
		case "LanguageID":
			inv.Language = r.text(c)
		case "IncludedNote":
			inv.Notes = append(inv.Notes, r.note(c))
		default:
			r.unread(c)
		}
	}
	r.leave()
}

func (r *reader) note(n *xmltree.Node) invoice.Note {
	var note invoice.Note
	h := r.hold(n, &note.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "ContentCode":
			note.ContentCode = r.text(c)
		case "Content":
			note.Content = joinText(note.Content, r.text(c))
		case "SubjectCode":
			note.SubjectCode = r.text(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return note
}

// joinText appends s to a text, separated by a newline.
func joinText(a, s string) string {
	switch {
	case a == "":
		return s
	case s == "":
		return a
	}
	return a + "\n" + s
}

func (r *reader) transaction(n *xmltree.Node) {
	inv := r.inv
	lines := 0
	for _, c := range n.Children {
		if c.Name == "IncludedSupplyChainTradeLineItem" {
			lines++
		}
	}
	if lines > 0 {
		inv.Lines = make([]invoice.Line, 0, lines)
	}
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "IncludedSupplyChainTradeLineItem":
			inv.Lines = append(inv.Lines, invoice.Line{})
			r.line(c, &inv.Lines[len(inv.Lines)-1])
		case "ApplicableHeaderTradeAgreement":
			r.agreement(c)
		case "ApplicableHeaderTradeDelivery":
			r.delivery(c)
		case "ApplicableHeaderTradeSettlement":
			r.settlement(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
}

func (r *reader) agreement(n *xmltree.Node) {
	inv := r.inv
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "BuyerReference":
			inv.BuyerReference = r.text(c)
		case "SellerTradeParty":
			r.party(c, &inv.Seller)
		case "BuyerTradeParty":
			r.party(c, &inv.Buyer)
		case "SalesAgentTradeParty":
			inv.SalesAgent = r.newParty(c)
		case "SellerTaxRepresentativeTradeParty":
			inv.SellerTaxRepresentative = r.newParty(c)
		case "ProductEndUserTradeParty":
			inv.ProductEndUser = r.newParty(c)
		case "ApplicableTradeDeliveryTerms":
			inv.DeliveryTerms = r.deliveryTerms(c)
		case "BuyerAgentTradeParty":
			inv.BuyerAgent = r.newParty(c)
		case "SellerOrderReferencedDocument":
			inv.SalesOrderReference = r.childText(c, "IssuerAssignedID")
		case "BuyerOrderReferencedDocument":
			inv.PurchaseOrderReference = r.childText(c, "IssuerAssignedID")
		case "ContractReferencedDocument":
			inv.ContractReference = r.childText(c, "IssuerAssignedID")
		case "AdditionalReferencedDocument":
			inv.AdditionalDocuments = append(inv.AdditionalDocuments, r.additionalDocument(c))
		case "SpecifiedProcuringProject":
			r.enter(c)
			for _, cc := range c.Children {
				switch cc.Name {
				case "ID":
					inv.ProjectReference = r.text(cc)
				case "Name":
					inv.ProjectName = r.text(cc)
				default:
					r.unread(cc)
				}
			}
			r.leave()
		case "UltimateCustomerOrderReferencedDocument":
			inv.UltimateCustomerOrders = append(inv.UltimateCustomerOrders, r.docRef(c))
		default:
			r.unread(c)
		}
	}
	r.leave()
}

func (r *reader) deliveryTerms(n *xmltree.Node) *invoice.DeliveryTerms {
	t := new(invoice.DeliveryTerms)
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "DeliveryTypeCode":
			t.Code = r.text(c)
		case "RelevantTradeLocation":
			r.enter(c)
			for _, cc := range c.Children {
				switch cc.Name {
				case "CountryID":
					t.LocationCountryCode = r.text(cc)
				case "Name":
					t.LocationName = r.text(cc)
				default:
					r.unread(cc)
				}
			}
			r.leave()
		default:
			r.unread(c)
		}
	}
	r.leave()
	if *t == (invoice.DeliveryTerms{}) {
		return nil
	}
	return t
}

func (r *reader) additionalDocument(n *xmltree.Node) invoice.AdditionalDocument {
	var d invoice.AdditionalDocument
	h := r.hold(n, &d.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "IssuerAssignedID":
			d.ID = r.text(c)
		case "URIID":
			d.URI = r.text(c)
		case "TypeCode":
			d.TypeCode = r.text(c)
		case "Name":
			d.Description = r.text(c)
		case "AttachmentBinaryObject":
			d.AttachmentMimeCode = attr(c, "mimeCode")
			d.AttachmentFilename = attr(c, "filename")
			d.Attachment = r.binary(c)
		case "ReferenceTypeCode":
			d.ReferenceTypeCode = r.text(c)
		case "FormattedIssueDateTime":
			d.IssueDate = r.date(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return d
}

func (r *reader) delivery(n *xmltree.Node) {
	inv := r.inv
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "ShipToTradeParty":
			inv.ShipTo = r.newParty(c)
		case "UltimateShipToTradeParty":
			inv.UltimateShipTo = r.newParty(c)
		case "ShipFromTradeParty":
			inv.ShipFrom = r.newParty(c)
		case "ActualDeliverySupplyChainEvent":
			inv.DeliveryDate = r.event(c)
		case "DespatchAdviceReferencedDocument":
			inv.DespatchAdviceReference = r.childText(c, "IssuerAssignedID")
		case "ReceivingAdviceReferencedDocument":
			inv.ReceivingAdviceReference = r.childText(c, "IssuerAssignedID")
		case "DeliveryNoteReferencedDocument":
			inv.DeliveryNote = r.newDocRef(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
}

// event reads a ram:SupplyChainEventType and returns its date.
func (r *reader) event(n *xmltree.Node) invoice.Date {
	var d invoice.Date
	seen := false
	r.enter(n)
	for _, c := range n.Children {
		if c.Name == "OccurrenceDateTime" && !seen {
			seen = true
			d = r.date(c)
			continue
		}
		r.unread(c)
	}
	r.leave()
	return d
}

func (r *reader) settlement(n *xmltree.Node) {
	inv := r.inv
	var summation *xmltree.Node
	accounting := false
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "CreditorReferenceID":
			inv.CreditorReferenceID = r.text(c)
		case "PaymentReference":
			inv.RemittanceInformation = r.text(c)
		case "TaxCurrencyCode":
			inv.TaxCurrency = r.text(c)
		case "InvoiceCurrencyCode":
			inv.Currency = r.text(c)
		case "InvoicerTradeParty":
			inv.Invoicer = r.newParty(c)
		case "InvoiceeTradeParty":
			inv.Invoicee = r.newParty(c)
		case "PayeeTradeParty":
			inv.Payee = r.newParty(c)
		case "PayerTradeParty":
			inv.Payer = r.newParty(c)
		case "TaxApplicableTradeCurrencyExchange":
			inv.CurrencyExchange = r.currencyExchange(c)
		case "SpecifiedTradeSettlementPaymentMeans":
			inv.PaymentMeans = append(inv.PaymentMeans, r.paymentMeans(c))
		case "ApplicableTradeTax":
			inv.VATBreakdown = append(inv.VATBreakdown, r.vat(c))
		case "BillingSpecifiedPeriod":
			inv.InvoicingPeriod = r.period(c)
		case "SpecifiedTradeAllowanceCharge":
			inv.AllowancesCharges = append(inv.AllowancesCharges, r.allowanceCharge(c, true))
		case "SpecifiedLogisticsServiceCharge":
			inv.LogisticsCharges = append(inv.LogisticsCharges, r.logisticsCharge(c))
		case "SpecifiedTradePaymentTerms":
			if t := r.paymentTerms(c); !t.IsZero() {
				inv.PaymentTerms = append(inv.PaymentTerms, t)
			}
		case "SpecifiedTradeSettlementHeaderMonetarySummation":
			if summation == nil {
				summation = c
			} else {
				r.unread(c)
			}
		case "SpecifiedFinancialAdjustment":
			inv.ThirdPartyCharges = append(inv.ThirdPartyCharges, r.thirdPartyCharge(c))
		case "InvoiceReferencedDocument":
			inv.PrecedingInvoices = append(inv.PrecedingInvoices, r.docRef(c))
		case "ReceivableSpecifiedTradeAccountingAccount":
			if accounting {
				r.unread(c)
				continue
			}
			accounting = true
			inv.BuyerAccountingReference = r.idOf(c)
		default:
			r.unread(c)
		}
	}
	// The totals are read last: BT-111 is recognized by its currency.
	if summation != nil {
		r.totals(summation)
	}
	r.leave()
}

func (r *reader) currencyExchange(n *xmltree.Node) *invoice.CurrencyExchange {
	x := new(invoice.CurrencyExchange)
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "SourceCurrencyCode":
			x.SourceCurrency = r.text(c)
		case "TargetCurrencyCode":
			x.TargetCurrency = r.text(c)
		case "ConversionRate":
			x.Rate = r.decimal(c)
		case "ConversionRateDateTime":
			x.Date = r.date(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
	if x.SourceCurrency == "" && x.TargetCurrency == "" && !x.Rate.IsSet() && x.Date.IsZero() {
		return nil
	}
	return x
}

func (r *reader) thirdPartyCharge(n *xmltree.Node) invoice.ThirdPartyCharge {
	var tc invoice.ThirdPartyCharge
	h := r.hold(n, &tc.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "Reason":
			tc.Reason = r.text(c)
		case "ActualAmount":
			tc.Amount = r.decimal(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return tc
}

func (r *reader) logisticsCharge(n *xmltree.Node) invoice.LogisticsCharge {
	var lc invoice.LogisticsCharge
	h := r.hold(n, &lc.Extensions)
	tax := false
	for _, c := range n.Children {
		switch {
		case c.Name == "Description":
			lc.Description = r.text(c)
		case c.Name == "AppliedAmount":
			lc.Amount = r.decimal(c)
		case c.Name == "AppliedTradeTax" && !tax:
			tax = true
			lc.VATCategoryCode, lc.VATRate = r.tradeTax(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return lc
}

// tradeTax reads a ram:TradeTaxType that carries a VAT category and rate
// (line VAT, VAT of an allowance or of a logistics charge). The type code is
// implied when it is VAT; the other children are kept as extensions.
func (r *reader) tradeTax(n *xmltree.Node) (category string, rate invoice.Decimal) {
	r.enter(n)
	for _, c := range n.Children {
		switch {
		case c.Name == "TypeCode" && textOf(c) == "VAT":
			r.text(c)
		case c.Name == "CategoryCode":
			category = r.text(c)
		case c.Name == "RateApplicablePercent":
			rate = r.decimal(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
	return category, rate
}

func (r *reader) paymentMeans(n *xmltree.Node) invoice.PaymentMeans {
	var pm invoice.PaymentMeans
	h := r.hold(n, &pm.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "TypeCode":
			pm.TypeCode = r.text(c)
		case "Information":
			pm.Information = r.text(c)
		case "ApplicableTradeSettlementFinancialCard":
			r.enter(c)
			for _, cc := range c.Children {
				switch cc.Name {
				case "ID":
					pm.CardID = r.text(cc)
				case "CardholderName":
					pm.CardholderName = r.text(cc)
				default:
					r.unread(cc)
				}
			}
			r.leave()
		case "PayerPartyDebtorFinancialAccount":
			r.enter(c)
			for _, cc := range c.Children {
				switch cc.Name {
				case "IBANID":
					pm.PayerIBAN = r.text(cc)
				case "AccountName":
					pm.PayerAccountName = r.text(cc)
				default:
					r.unread(cc)
				}
			}
			r.leave()
		case "PayerSpecifiedDebtorFinancialInstitution":
			pm.PayerBIC = r.childText(c, "BICID")
		case "PayeePartyCreditorFinancialAccount":
			r.enter(c)
			for _, cc := range c.Children {
				switch cc.Name {
				case "IBANID":
					pm.PayeeIBAN = r.text(cc)
				case "AccountName":
					pm.PayeeAccountName = r.text(cc)
				case "ProprietaryID":
					pm.PayeeProprietaryID = r.text(cc)
				default:
					r.unread(cc)
				}
			}
			r.leave()
		case "PayeeSpecifiedCreditorFinancialInstitution":
			pm.PayeeBIC = r.childText(c, "BICID")
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return pm
}

// vat reads a header ram:ApplicableTradeTax (BG-23) and the BT-7/BT-8 it
// carries. Writers repeat BT-7 and BT-8 in every entry: a value that differs
// from the first one is kept as an extension.
func (r *reader) vat(n *xmltree.Node) invoice.VATBreakdown {
	var v invoice.VATBreakdown
	inv := r.inv
	h := r.hold(n, &v.Extensions)
	for _, c := range n.Children {
		switch {
		case c.Name == "CalculatedAmount":
			v.TaxAmount = r.decimal(c)
		case c.Name == "TypeCode" && textOf(c) == "VAT":
			r.text(c)
		case c.Name == "BasisAmount":
			v.TaxableAmount = r.decimal(c)
		case c.Name == "CategoryCode":
			v.CategoryCode = r.text(c)
		case c.Name == "RateApplicablePercent":
			v.Rate = r.decimal(c)
		case c.Name == "ExemptionReason":
			v.ExemptionReason = r.text(c)
		case c.Name == "ExemptionReasonCode":
			v.ExemptionReasonCode = r.text(c)
		case c.Name == "TaxPointDate":
			if d := r.peekDate(c); inv.TaxPointDate.IsZero() || d == inv.TaxPointDate {
				inv.TaxPointDate = r.date(c)
			} else {
				r.unread(c)
			}
		case c.Name == "DueDateTypeCode":
			if code := textOf(c); inv.TaxPointDateCode == "" || code == inv.TaxPointDateCode {
				inv.TaxPointDateCode = r.text(c)
			} else {
				r.unread(c)
			}
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return v
}

// peekDate returns the date of a date element without reporting anything.
func (r *reader) peekDate(n *xmltree.Node) invoice.Date {
	v := child(n, "DateString")
	if v == nil {
		v = child(n, "DateTimeString")
	}
	if v == nil {
		v = n
	}
	d, err := invoice.ParseDate(textOf(v), attr(v, "format"))
	if err != nil {
		return invoice.Date{}
	}
	return d
}

func (r *reader) period(n *xmltree.Node) invoice.Period {
	var p invoice.Period
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "Description":
			p.Description = r.text(c)
		case "StartDateTime":
			p.Start = r.date(c)
		case "EndDateTime":
			p.End = r.date(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
	return p
}

// allowanceCharge reads a ram:TradeAllowanceChargeType; vat tells whether
// its CategoryTradeTax is read (it is not for the allowances and charges of
// a price).
func (r *reader) allowanceCharge(n *xmltree.Node, vat bool) invoice.AllowanceCharge {
	var ac invoice.AllowanceCharge
	h := r.hold(n, &ac.Extensions)
	for _, c := range n.Children {
		switch {
		case c.Name == "ChargeIndicator":
			ac.Charge = r.indicator(c)
		case c.Name == "CalculationPercent":
			ac.Percentage = r.decimal(c)
		case c.Name == "BasisAmount":
			ac.BaseAmount = r.decimal(c)
		case c.Name == "ActualAmount":
			ac.Amount = r.decimal(c)
		case c.Name == "ReasonCode":
			ac.ReasonCode = r.text(c)
		case c.Name == "Reason":
			ac.Reason = r.text(c)
		case c.Name == "CategoryTradeTax" && vat:
			ac.VATCategoryCode, ac.VATRate = r.tradeTax(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return ac
}

func (r *reader) paymentTerms(n *xmltree.Node) invoice.PaymentTerms {
	var t invoice.PaymentTerms
	h := r.hold(n, &t.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "Description":
			t.Description = joinText(t.Description, r.text(c))
		case "DueDateDateTime":
			t.DueDate = r.date(c)
		case "DirectDebitMandateID":
			t.DirectDebitMandateID = r.text(c)
		case "PartialPaymentAmount":
			t.PartialAmount = r.decimal(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return t
}

func (r *reader) totals(n *xmltree.Node) {
	inv := r.inv
	t := &inv.Totals
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "LineTotalAmount":
			t.LineTotal = r.decimal(c)
		case "ChargeTotalAmount":
			t.ChargeTotal = r.decimal(c)
		case "AllowanceTotalAmount":
			t.AllowanceTotal = r.decimal(c)
		case "TaxBasisTotalAmount":
			t.TaxBasisTotal = r.decimal(c)
		case "TaxTotalAmount":
			cur := attr(c, "currencyID")
			d := r.parseDecimal(c, r.textAttrs(c, "currencyID", "", ""))
			switch {
			case inv.TaxCurrency != "" && inv.TaxCurrency != inv.Currency && cur == inv.TaxCurrency:
				t.TaxTotalInTaxCurrency = d
			case !t.TaxTotal.IsSet():
				t.TaxTotal = d
			default:
				t.TaxTotalInTaxCurrency = d
			}
		case "RoundingAmount":
			t.RoundingAmount = r.decimal(c)
		case "GrandTotalAmount":
			t.GrandTotal = r.decimal(c)
		case "TotalPrepaidAmount":
			t.PaidAmount = r.decimal(c)
		case "DuePayableAmount":
			t.DuePayable = r.decimal(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
}

// docRef reads a ram:ReferencedDocumentType as a document reference.
func (r *reader) docRef(n *xmltree.Node) invoice.DocumentReference {
	var d invoice.DocumentReference
	h := r.hold(n, &d.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "IssuerAssignedID":
			d.ID = r.text(c)
		case "LineID":
			d.LineID = r.text(c)
		case "TypeCode":
			d.TypeCode = r.text(c)
		case "FormattedIssueDateTime":
			d.IssueDate = r.date(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return d
}

// newDocRef reads an optional document reference; it returns nil when the
// reference is empty.
func (r *reader) newDocRef(n *xmltree.Node) *invoice.DocumentReference {
	d := r.docRef(n)
	if d.IsZero() {
		return nil
	}
	return &d
}

// newParty reads an optional party; it returns nil when the party is empty.
func (r *reader) newParty(n *xmltree.Node) *invoice.Party {
	p := new(invoice.Party)
	r.party(n, p)
	if p.IsZero() {
		return nil
	}
	return p
}

func (r *reader) party(n *xmltree.Node, p *invoice.Party) {
	h := r.hold(n, &p.Extensions)
	contact := false
	for _, c := range n.Children {
		switch c.Name {
		case "ID", "GlobalID":
			p.IDs = append(p.IDs, r.ident(c))
		case "Name":
			p.Name = r.text(c)
		case "Description":
			p.Description = r.text(c)
		case "SpecifiedLegalOrganization":
			r.enter(c)
			for _, cc := range c.Children {
				switch cc.Name {
				case "ID":
					p.LegalRegistration = r.ident(cc)
				case "TradingBusinessName":
					p.TradingName = r.text(cc)
				default:
					r.unread(cc)
				}
			}
			r.leave()
		case "DefinedTradeContact":
			if contact {
				r.unread(c)
				continue
			}
			contact = true
			p.Contact = r.contact(c)
		case "PostalTradeAddress":
			p.Address = r.address(c)
		case "URIUniversalCommunication":
			r.enter(c)
			for _, cc := range c.Children {
				if cc.Name == "URIID" && p.ElectronicAddress.IsZero() {
					p.ElectronicAddress = r.ident(cc)
					continue
				}
				r.unread(cc)
			}
			r.leave()
		case "SpecifiedTaxRegistration":
			id := child(c, "ID")
			s := ""
			if id != nil {
				s = attr(id, "schemeID")
			}
			switch {
			case strings.EqualFold(s, "VA") && p.VATID == "":
				p.VATID = r.taxRegistration(c)
			case strings.EqualFold(s, "FC") && p.TaxRegistrationID == "":
				p.TaxRegistrationID = r.taxRegistration(c)
			default:
				r.unread(c)
			}
		default:
			r.unread(c)
		}
	}
	r.release(h)
}

// taxRegistration reads a ram:SpecifiedTaxRegistration whose scheme is
// known (VA or FC, written back by the writer).
func (r *reader) taxRegistration(n *xmltree.Node) string {
	var v string
	seen := false
	r.enter(n)
	for _, c := range n.Children {
		if c.Name == "ID" && !seen {
			seen = true
			v = r.textAttrs(c, "schemeID", "", "")
			continue
		}
		r.unread(c)
	}
	r.leave()
	return v
}

// contact reads a ram:DefinedTradeContact; it returns nil when empty.
func (r *reader) contact(n *xmltree.Node) *invoice.Contact {
	var c invoice.Contact
	r.enter(n)
	for _, e := range n.Children {
		switch e.Name {
		case "PersonName":
			c.PersonName = r.text(e)
		case "DepartmentName":
			c.DepartmentName = r.text(e)
		case "TelephoneUniversalCommunication":
			c.Phone = r.childText(e, "CompleteNumber")
		case "EmailURIUniversalCommunication":
			c.Email = r.childText(e, "URIID")
		default:
			r.unread(e)
		}
	}
	r.leave()
	if c == (invoice.Contact{}) {
		return nil
	}
	return &c
}

func (r *reader) address(n *xmltree.Node) invoice.Address {
	var a invoice.Address
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "PostcodeCode":
			a.PostCode = r.text(c)
		case "LineOne":
			a.Line1 = r.text(c)
		case "LineTwo":
			a.Line2 = r.text(c)
		case "LineThree":
			a.Line3 = r.text(c)
		case "CityName":
			a.City = r.text(c)
		case "CountryID":
			a.CountryCode = r.text(c)
		case "CountrySubDivisionName":
			a.CountrySubdivision = r.text(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
	return a
}

func (r *reader) line(n *xmltree.Node, l *invoice.Line) {
	h := r.hold(n, &l.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "AssociatedDocumentLineDocument":
			r.lineDocument(c, l)
		case "SpecifiedTradeProduct":
			r.item(c, &l.Item)
		case "SpecifiedLineTradeAgreement":
			r.lineAgreement(c, l)
		case "SpecifiedLineTradeDelivery":
			r.lineDelivery(c, l)
		case "SpecifiedLineTradeSettlement":
			r.lineSettlement(c, l)
		default:
			r.unread(c)
		}
	}
	r.release(h)
}

func (r *reader) lineDocument(n *xmltree.Node, l *invoice.Line) {
	note := false
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "LineID":
			l.ID = r.text(c)
		case "ParentLineID":
			l.ParentID = r.text(c)
		case "LineStatusCode":
			l.StatusCode = r.text(c)
		case "LineStatusReasonCode":
			l.StatusReasonCode = r.text(c)
		case "IncludedNote":
			// BT-127 is the content of the first note; the codes and the
			// other notes that EXTENDED allows are kept as extensions.
			if note {
				r.unread(c)
				continue
			}
			note = true
			l.Note = r.childText(c, "Content")
		default:
			r.unread(c)
		}
	}
	r.leave()
}

func (r *reader) item(n *xmltree.Node, it *invoice.Item) {
	r.enter(n)
	for _, c := range n.Children {
		switch {
		case c.Name == "GlobalID" && it.GlobalID.IsZero():
			it.GlobalID = r.ident(c)
		case c.Name == "SellerAssignedID":
			it.SellerID = r.text(c)
		case c.Name == "BuyerAssignedID":
			it.BuyerID = r.text(c)
		case c.Name == "Name":
			it.Name = r.text(c)
		case c.Name == "Description":
			it.Description = r.text(c)
		case c.Name == "ApplicableProductCharacteristic":
			it.Attributes = append(it.Attributes, r.itemAttribute(c))
		case c.Name == "DesignatedProductClassification":
			if cl := r.classification(c); !cl.IsZero() {
				it.Classifications = append(it.Classifications, cl)
			}
		case c.Name == "OriginTradeCountry":
			it.OriginCountry = r.idOf(c)
		case c.Name == "IncludedReferencedProduct":
			it.ReferencedProducts = append(it.ReferencedProducts, r.referencedProduct(c))
		default:
			r.unread(c)
		}
	}
	r.leave()
}

func (r *reader) itemAttribute(n *xmltree.Node) invoice.ItemAttribute {
	var a invoice.ItemAttribute
	h := r.hold(n, &a.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "TypeCode":
			a.TypeCode = r.text(c)
		case "Description":
			a.Name = r.text(c)
		case "Value":
			a.Value = r.text(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return a
}

func (r *reader) classification(n *xmltree.Node) invoice.Classification {
	var cl invoice.Classification
	h := r.hold(n, &cl.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "ClassCode":
			cl.Code = r.textAttrs(c, "listID", "listVersionID", "")
			cl.ListID = attr(c, "listID")
			cl.ListVersionID = attr(c, "listVersionID")
		case "ClassName":
			cl.Name = r.text(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return cl
}

func (r *reader) referencedProduct(n *xmltree.Node) invoice.ReferencedProduct {
	var p invoice.ReferencedProduct
	h := r.hold(n, &p.Extensions)
	for _, c := range n.Children {
		switch c.Name {
		case "ID":
			p.ID = r.text(c)
		case "GlobalID":
			p.GlobalIDs = append(p.GlobalIDs, r.ident(c))
		case "SellerAssignedID":
			p.SellerID = r.text(c)
		case "BuyerAssignedID":
			p.BuyerID = r.text(c)
		case "IndustryAssignedID":
			p.IndustryID = r.text(c)
		case "Name":
			p.Name = r.text(c)
		case "Description":
			p.Description = r.text(c)
		case "UnitQuantity":
			p.UnitQuantity, p.UnitQuantityUnit = r.quantity(c)
		default:
			r.unread(c)
		}
	}
	r.release(h)
	return p
}

func (r *reader) lineAgreement(n *xmltree.Node, l *invoice.Line) {
	var gross, net *xmltree.Node
	r.enter(n)
	for _, c := range n.Children {
		switch {
		case c.Name == "ApplicableTradeDeliveryTerms":
			l.DeliveryTerms = r.deliveryTerms(c)
		case c.Name == "BuyerOrderReferencedDocument":
			r.enter(c)
			for _, cc := range c.Children {
				switch cc.Name {
				case "IssuerAssignedID":
					l.OrderReference = r.text(cc)
				case "LineID":
					l.OrderLineReference = r.text(cc)
				default:
					r.unread(cc)
				}
			}
			r.leave()
		case c.Name == "GrossPriceProductTradePrice" && gross == nil:
			gross = c
		case c.Name == "NetPriceProductTradePrice" && net == nil:
			net = c
		case c.Name == "UltimateCustomerOrderReferencedDocument":
			l.UltimateCustomerOrders = append(l.UltimateCustomerOrders, r.docRef(c))
		default:
			r.unread(c)
		}
	}
	// The net price is read first: the base quantity of the gross price is
	// only needed when the net price has none.
	if net != nil {
		r.enter(net)
		for _, c := range net.Children {
			switch c.Name {
			case "ChargeAmount":
				l.NetPrice = r.decimal(c)
			case "BasisQuantity":
				l.PriceBaseQuantity, l.PriceBaseQuantityUnit = r.quantity(c)
			default:
				r.unread(c)
			}
		}
		r.leave()
	}
	if gross != nil {
		r.grossPrice(gross, l)
	}
	r.leave()
}

// grossPrice reads the gross price (BT-148) and its allowances and charges.
func (r *reader) grossPrice(n *xmltree.Node, l *invoice.Line) {
	var acs []*xmltree.Node
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "ChargeAmount":
			l.GrossPrice = r.decimal(c)
		case "BasisQuantity":
			// Writers give the gross price the base quantity of the net
			// price; a different one is kept as an extension.
			q, unit := attrQuantity(c)
			if !l.PriceBaseQuantity.IsSet() && l.PriceBaseQuantityUnit == "" {
				l.PriceBaseQuantity, l.PriceBaseQuantityUnit = r.quantity(c)
			} else if q.IsSet() && q.Equal(l.PriceBaseQuantity) && unit == l.PriceBaseQuantityUnit {
				r.quantity(c)
			} else {
				r.unread(c)
			}
		case "AppliedTradeAllowanceCharge":
			acs = append(acs, c)
		default:
			r.unread(c)
		}
	}
	// A single allowance amount is BT-147. Anything richer is detailed in
	// PriceAllowancesCharges, BT-147 being the sum of the allowances.
	if len(acs) == 1 && plainAllowance(acs[0]) {
		r.enter(acs[0])
		for _, c := range acs[0].Children {
			switch c.Name {
			case "ChargeIndicator":
				r.indicator(c)
			case "ActualAmount":
				l.PriceDiscount = r.decimal(c)
			}
		}
		r.leave()
	} else {
		for _, c := range acs {
			ac := r.allowanceCharge(c, false)
			l.PriceAllowancesCharges = append(l.PriceAllowancesCharges, ac)
			if !ac.Charge && ac.Amount.IsSet() {
				if l.PriceDiscount.IsSet() {
					l.PriceDiscount = l.PriceDiscount.Add(ac.Amount)
				} else {
					l.PriceDiscount = ac.Amount
				}
			}
		}
	}
	r.leave()
}

// attrQuantity returns the value and unit of a quantity without reporting
// anything.
func attrQuantity(n *xmltree.Node) (invoice.Decimal, string) {
	d, _ := invoice.ParseDecimal(textOf(n))
	return d, attr(n, "unitCode")
}

// plainAllowance reports whether an AppliedTradeAllowanceCharge is an
// allowance that carries only its amount.
func plainAllowance(n *xmltree.Node) bool {
	if len(n.Attrs) > 0 {
		return false
	}
	var indicator, amount bool
	for _, c := range n.Children {
		switch c.Name {
		case "ChargeIndicator":
			v := child(c, "Indicator")
			if indicator || len(c.Attrs) > 0 || v == nil || len(c.Children) != 1 || len(v.Attrs) > 0 {
				return false
			}
			if s := textOf(v); s != "false" && s != "0" {
				return false
			}
			indicator = true
		case "ActualAmount":
			if amount {
				return false
			}
			for _, a := range c.Attrs {
				if a.Name != "currencyID" {
					return false
				}
			}
			amount = true
		default:
			return false
		}
	}
	return indicator && amount
}

func (r *reader) lineDelivery(n *xmltree.Node, l *invoice.Line) {
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "BilledQuantity":
			l.Quantity, l.UnitCode = r.quantity(c)
		case "ShipToTradeParty":
			l.ShipTo = r.newParty(c)
		case "UltimateShipToTradeParty":
			l.UltimateShipTo = r.newParty(c)
		case "ActualDeliverySupplyChainEvent":
			l.DeliveryDate = r.event(c)
		case "DeliveryNoteReferencedDocument":
			l.DeliveryNote = r.newDocRef(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
}

func (r *reader) lineSettlement(n *xmltree.Node, l *invoice.Line) {
	var tax, object, accounting bool
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "ApplicableTradeTax":
			if tax {
				r.unread(c)
				continue
			}
			tax = true
			l.VATCategoryCode, l.VATRate = r.tradeTax(c)
		case "BillingSpecifiedPeriod":
			l.Period = r.period(c)
		case "SpecifiedTradeAllowanceCharge":
			l.AllowancesCharges = append(l.AllowancesCharges, r.allowanceCharge(c, true))
		case "SpecifiedTradeSettlementLineMonetarySummation":
			r.enter(c)
			for _, cc := range c.Children {
				if cc.Name == "LineTotalAmount" && !l.NetAmount.IsSet() {
					l.NetAmount = r.decimal(cc)
					continue
				}
				r.unread(cc)
			}
			r.leave()
		case "InvoiceReferencedDocument":
			if l.InvoiceReference != nil {
				r.unread(c)
			} else if d := r.docRef(c); !d.IsZero() {
				l.InvoiceReference = &d
			}
		case "AdditionalReferencedDocument":
			if tc := textOf(child(c, "TypeCode")); (tc == "130" || tc == "") && !object {
				object = true
				r.objectIdentifier(c, l)
				continue
			}
			r.unread(c)
		case "ReceivableSpecifiedTradeAccountingAccount":
			if accounting {
				r.unread(c)
				continue
			}
			accounting = true
			l.AccountingReference = r.idOf(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
}

// objectIdentifier reads the line object identifier (BT-128): an
// AdditionalReferencedDocument of type 130.
func (r *reader) objectIdentifier(n *xmltree.Node, l *invoice.Line) {
	r.enter(n)
	for _, c := range n.Children {
		switch c.Name {
		case "IssuerAssignedID":
			l.ObjectIdentifier.ID = r.text(c)
		case "TypeCode":
			r.text(c)
		case "ReferenceTypeCode":
			l.ObjectIdentifier.Scheme = r.text(c)
		default:
			r.unread(c)
		}
	}
	r.leave()
}
