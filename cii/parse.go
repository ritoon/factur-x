package cii

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// ErrNotCII is returned (wrapped) by Parse when the document is well-formed
// XML but not a rsm:CrossIndustryInvoice, for example a ZUGFeRD 1.0
// CrossIndustryDocument or an UBL invoice.
var ErrNotCII = errors.New("cii: not a CrossIndustryInvoice document")

// Issue is a value that could not be read: a malformed amount, quantity,
// date, indicator or base64 attachment. The corresponding field of the
// invoice is left empty.
type Issue struct {
	Path    string // location of the element, as given by xmltree.Node.Path
	Line    int    // line of the element in the document, 1-based
	Message string
}

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

func (e *IssuesError) Error() string {
	if len(e.Issues) == 1 {
		return "cii: unreadable value at " + e.Issues[0].String()
	}
	return "cii: " + strconv.Itoa(len(e.Issues)) + " unreadable values, first at " + e.Issues[0].String()
}

// Parse reads a CII invoice.
//
// It fails, with a nil invoice, only when data is not well-formed XML or its
// root is not a rsm:CrossIndustryInvoice (the error then wraps ErrNotCII).
// When some values cannot be read, Parse returns the invoice with the
// corresponding fields empty and an *IssuesError listing them.
//
// Reading is tolerant: namespace prefixes are ignored (the root namespace is
// checked), values are trimmed, elements are recognized in any order, and the
// ZUGFeRD 2.0/2.1 variants are accepted. The profile is derived from the
// specification identifier BT-24, which is kept in SpecificationID.
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
// root is not a rsm:CrossIndustryInvoice element, the invoice is nil and the
// single issue says why.
func ParseTree(root *xmltree.Node) (*invoice.Invoice, []Issue) {
	if err := checkRoot(root); err != nil {
		is := Issue{Message: err.Error()}
		if root != nil {
			is.Path, is.Line = root.Path(), root.Line
		}
		return nil, []Issue{is}
	}
	r := reader{inv: new(invoice.Invoice)}
	r.document(root)
	return r.inv, r.issues
}

func checkRoot(root *xmltree.Node) error {
	switch {
	case root == nil:
		return ErrNotCII
	case root.Name == "CrossIndustryDocument":
		return fmt.Errorf("%w: ZUGFeRD 1.0 CrossIndustryDocument is not supported", ErrNotCII)
	case root.Name != "CrossIndustryInvoice":
		return fmt.Errorf("%w: root element is <%s>", ErrNotCII, root.Name)
	case root.Space != NamespaceRSM:
		return fmt.Errorf("%w: root element in namespace %q", ErrNotCII, root.Space)
	}
	return nil
}

type reader struct {
	inv    *invoice.Invoice
	issues []Issue
}

func (r *reader) issue(n *xmltree.Node, msg string) {
	r.issues = append(r.issues, Issue{Path: n.Path(), Line: n.Line, Message: msg})
}

// text returns the trimmed text of n ("" for nil).
func text(n *xmltree.Node) string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Text)
}

// child returns the first child element of n with the given local name.
func child(n *xmltree.Node, name string) *xmltree.Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// childText returns the trimmed text of the first child named name.
func childText(n *xmltree.Node, name string) string { return text(child(n, name)) }

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
func ident(n *xmltree.Node) invoice.Identifier {
	return invoice.Identifier{ID: text(n), Scheme: attr(n, "schemeID")}
}

func (r *reader) decimal(n *xmltree.Node) invoice.Decimal {
	s := text(n)
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
	for _, c := range n.Children {
		if c.Name == "DateTimeString" || c.Name == "DateString" {
			v = c
			break
		}
	}
	s := text(v)
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
	if c := child(n, "Indicator"); c != nil {
		v = c
	}
	switch s := text(v); s {
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
	s := strings.Map(func(c rune) rune {
		switch c {
		case ' ', '\t', '\n', '\r':
			return -1
		}
		return c
	}, n.Text)
	if s == "" {
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		var err2 error
		if b, err2 = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "=")); err2 != nil {
			r.issue(n, "invalid base64 content: "+err.Error())
			return nil
		}
	}
	return b
}

func (r *reader) document(root *xmltree.Node) {
	for _, c := range root.Children {
		switch c.Name {
		case "ExchangedDocumentContext":
			r.context(c)
		case "ExchangedDocument":
			r.exchangedDocument(c)
		case "SupplyChainTradeTransaction":
			r.transaction(c)
		}
	}
	r.inv.Profile = detectProfile(r.inv.SpecificationID)
}

func (r *reader) context(n *xmltree.Node) {
	inv := r.inv
	for _, c := range n.Children {
		switch c.Name {
		case "TestIndicator":
			inv.TestIndicator = r.indicator(c)
		case "BusinessProcessSpecifiedDocumentContextParameter":
			if inv.BusinessProcess == "" {
				inv.BusinessProcess = childText(c, "ID")
			}
		case "GuidelineSpecifiedDocumentContextParameter":
			if inv.SpecificationID == "" {
				inv.SpecificationID = childText(c, "ID")
			}
		}
	}
}

func (r *reader) exchangedDocument(n *xmltree.Node) {
	inv := r.inv
	for _, c := range n.Children {
		switch c.Name {
		case "ID":
			inv.Number = text(c)
		case "TypeCode":
			inv.TypeCode = text(c)
		case "IssueDateTime":
			inv.IssueDate = r.date(c)
		case "LanguageID":
			inv.Language = text(c)
		case "IncludedNote":
			inv.Notes = append(inv.Notes, readNote(c))
		}
	}
}

func readNote(n *xmltree.Node) invoice.Note {
	var note invoice.Note
	for _, c := range n.Children {
		switch c.Name {
		case "ContentCode":
			note.ContentCode = text(c)
		case "Content":
			note.Content = joinText(note.Content, text(c))
		case "SubjectCode":
			note.SubjectCode = text(c)
		}
	}
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
		}
	}
}

func (r *reader) agreement(n *xmltree.Node) {
	inv := r.inv
	for _, c := range n.Children {
		switch c.Name {
		case "BuyerReference":
			inv.BuyerReference = text(c)
		case "SellerTradeParty":
			r.party(c, &inv.Seller)
		case "BuyerTradeParty":
			r.party(c, &inv.Buyer)
		case "SalesAgentTradeParty":
			inv.SalesAgent = r.newParty(c)
		case "SellerTaxRepresentativeTradeParty":
			inv.SellerTaxRepresentative = r.newParty(c)
		case "BuyerAgentTradeParty":
			inv.BuyerAgent = r.newParty(c)
		case "SellerOrderReferencedDocument":
			inv.SalesOrderReference = childText(c, "IssuerAssignedID")
		case "BuyerOrderReferencedDocument":
			inv.PurchaseOrderReference = childText(c, "IssuerAssignedID")
		case "ContractReferencedDocument":
			inv.ContractReference = childText(c, "IssuerAssignedID")
		case "AdditionalReferencedDocument":
			inv.AdditionalDocuments = append(inv.AdditionalDocuments, r.additionalDocument(c))
		case "SpecifiedProcuringProject":
			inv.ProjectReference = childText(c, "ID")
			inv.ProjectName = childText(c, "Name")
		}
	}
}

func (r *reader) additionalDocument(n *xmltree.Node) invoice.AdditionalDocument {
	var d invoice.AdditionalDocument
	for _, c := range n.Children {
		switch c.Name {
		case "IssuerAssignedID":
			d.ID = text(c)
		case "URIID":
			d.URI = text(c)
		case "TypeCode":
			d.TypeCode = text(c)
		case "Name":
			d.Description = text(c)
		case "AttachmentBinaryObject":
			d.AttachmentMimeCode = attr(c, "mimeCode")
			d.AttachmentFilename = attr(c, "filename")
			d.Attachment = r.binary(c)
		case "ReferenceTypeCode":
			d.ReferenceTypeCode = text(c)
		case "FormattedIssueDateTime":
			d.IssueDate = r.date(c)
		}
	}
	return d
}

func (r *reader) delivery(n *xmltree.Node) {
	inv := r.inv
	for _, c := range n.Children {
		switch c.Name {
		case "ShipToTradeParty":
			inv.ShipTo = r.newParty(c)
		case "ActualDeliverySupplyChainEvent":
			if oc := child(c, "OccurrenceDateTime"); oc != nil {
				inv.DeliveryDate = r.date(oc)
			}
		case "DespatchAdviceReferencedDocument":
			inv.DespatchAdviceReference = childText(c, "IssuerAssignedID")
		case "ReceivingAdviceReferencedDocument":
			inv.ReceivingAdviceReference = childText(c, "IssuerAssignedID")
		}
	}
}

func (r *reader) settlement(n *xmltree.Node) {
	inv := r.inv
	var summation *xmltree.Node
	for _, c := range n.Children {
		switch c.Name {
		case "CreditorReferenceID":
			inv.CreditorReferenceID = text(c)
		case "PaymentReference":
			inv.RemittanceInformation = text(c)
		case "TaxCurrencyCode":
			inv.TaxCurrency = text(c)
		case "InvoiceCurrencyCode":
			inv.Currency = text(c)
		case "InvoicerTradeParty":
			inv.Invoicer = r.newParty(c)
		case "InvoiceeTradeParty":
			inv.Invoicee = r.newParty(c)
		case "PayeeTradeParty":
			inv.Payee = r.newParty(c)
		case "PayerTradeParty":
			inv.Payer = r.newParty(c)
		case "SpecifiedTradeSettlementPaymentMeans":
			inv.PaymentMeans = append(inv.PaymentMeans, r.paymentMeans(c))
		case "ApplicableTradeTax":
			inv.VATBreakdown = append(inv.VATBreakdown, r.vat(c))
		case "BillingSpecifiedPeriod":
			inv.InvoicingPeriod = r.period(c)
		case "SpecifiedTradeAllowanceCharge":
			inv.AllowancesCharges = append(inv.AllowancesCharges, r.allowanceCharge(c))
		case "SpecifiedTradePaymentTerms":
			if t := r.paymentTerms(c); t != (invoice.PaymentTerms{}) {
				inv.PaymentTerms = append(inv.PaymentTerms, t)
			}
		case "SpecifiedTradeSettlementHeaderMonetarySummation":
			summation = c
		case "InvoiceReferencedDocument":
			inv.PrecedingInvoices = append(inv.PrecedingInvoices, r.docRef(c))
		case "ReceivableSpecifiedTradeAccountingAccount":
			if inv.BuyerAccountingReference == "" {
				inv.BuyerAccountingReference = childText(c, "ID")
			}
		}
	}
	// The totals are read last: BT-111 is recognized by its currency.
	if summation != nil {
		r.totals(summation)
	}
}

func (r *reader) paymentMeans(n *xmltree.Node) invoice.PaymentMeans {
	var pm invoice.PaymentMeans
	for _, c := range n.Children {
		switch c.Name {
		case "TypeCode":
			pm.TypeCode = text(c)
		case "Information":
			pm.Information = text(c)
		case "ApplicableTradeSettlementFinancialCard":
			pm.CardID = childText(c, "ID")
			pm.CardholderName = childText(c, "CardholderName")
		case "PayerPartyDebtorFinancialAccount":
			pm.PayerIBAN = childText(c, "IBANID")
		case "PayeePartyCreditorFinancialAccount":
			for _, cc := range c.Children {
				switch cc.Name {
				case "IBANID":
					pm.PayeeIBAN = text(cc)
				case "AccountName":
					pm.PayeeAccountName = text(cc)
				case "ProprietaryID":
					pm.PayeeProprietaryID = text(cc)
				}
			}
		case "PayeeSpecifiedCreditorFinancialInstitution":
			pm.PayeeBIC = childText(c, "BICID")
		}
	}
	return pm
}

// vat reads a header ram:ApplicableTradeTax (BG-23) and the BT-7/BT-8 it
// carries.
func (r *reader) vat(n *xmltree.Node) invoice.VATBreakdown {
	var v invoice.VATBreakdown
	inv := r.inv
	for _, c := range n.Children {
		switch c.Name {
		case "CalculatedAmount":
			v.TaxAmount = r.decimal(c)
		case "BasisAmount":
			v.TaxableAmount = r.decimal(c)
		case "CategoryCode":
			v.CategoryCode = text(c)
		case "RateApplicablePercent":
			v.Rate = r.decimal(c)
		case "ExemptionReason":
			v.ExemptionReason = text(c)
		case "ExemptionReasonCode":
			v.ExemptionReasonCode = text(c)
		case "TaxPointDate":
			if inv.TaxPointDate.IsZero() {
				inv.TaxPointDate = r.date(c)
			}
		case "DueDateTypeCode":
			if inv.TaxPointDateCode == "" {
				inv.TaxPointDateCode = text(c)
			}
		}
	}
	return v
}

func (r *reader) period(n *xmltree.Node) invoice.Period {
	var p invoice.Period
	for _, c := range n.Children {
		switch c.Name {
		case "Description":
			p.Description = text(c)
		case "StartDateTime":
			p.Start = r.date(c)
		case "EndDateTime":
			p.End = r.date(c)
		}
	}
	return p
}

func (r *reader) allowanceCharge(n *xmltree.Node) invoice.AllowanceCharge {
	var ac invoice.AllowanceCharge
	for _, c := range n.Children {
		switch c.Name {
		case "ChargeIndicator":
			ac.Charge = r.indicator(c)
		case "CalculationPercent":
			ac.Percentage = r.decimal(c)
		case "BasisAmount":
			ac.BaseAmount = r.decimal(c)
		case "ActualAmount":
			ac.Amount = r.decimal(c)
		case "ReasonCode":
			ac.ReasonCode = text(c)
		case "Reason":
			ac.Reason = text(c)
		case "CategoryTradeTax":
			for _, cc := range c.Children {
				switch cc.Name {
				case "CategoryCode":
					ac.VATCategoryCode = text(cc)
				case "RateApplicablePercent":
					ac.VATRate = r.decimal(cc)
				}
			}
		}
	}
	return ac
}

func (r *reader) paymentTerms(n *xmltree.Node) invoice.PaymentTerms {
	var t invoice.PaymentTerms
	for _, c := range n.Children {
		switch c.Name {
		case "Description":
			t.Description = joinText(t.Description, text(c))
		case "DueDateDateTime":
			t.DueDate = r.date(c)
		case "DirectDebitMandateID":
			t.DirectDebitMandateID = text(c)
		case "PartialPaymentAmount":
			t.PartialAmount = r.decimal(c)
		}
	}
	return t
}

func (r *reader) totals(n *xmltree.Node) {
	inv := r.inv
	t := &inv.Totals
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
			d := r.decimal(c)
			cur := attr(c, "currencyID")
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
		}
	}
}

// docRef reads a ram:ReferencedDocumentType as a document reference.
func (r *reader) docRef(n *xmltree.Node) invoice.DocumentReference {
	var d invoice.DocumentReference
	for _, c := range n.Children {
		switch c.Name {
		case "IssuerAssignedID":
			d.ID = text(c)
		case "LineID":
			d.LineID = text(c)
		case "TypeCode":
			d.TypeCode = text(c)
		case "FormattedIssueDateTime":
			d.IssueDate = r.date(c)
		}
	}
	return d
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
	for _, c := range n.Children {
		switch c.Name {
		case "ID", "GlobalID":
			p.IDs = append(p.IDs, ident(c))
		case "Name":
			p.Name = text(c)
		case "Description":
			p.Description = text(c)
		case "SpecifiedLegalOrganization":
			for _, cc := range c.Children {
				switch cc.Name {
				case "ID":
					p.LegalRegistration = ident(cc)
				case "TradingBusinessName":
					p.TradingName = text(cc)
				}
			}
		case "DefinedTradeContact":
			if p.Contact == nil {
				p.Contact = readContact(c)
			}
		case "PostalTradeAddress":
			p.Address = readAddress(c)
		case "URIUniversalCommunication":
			if u := child(c, "URIID"); u != nil {
				p.ElectronicAddress = ident(u)
			}
		case "SpecifiedTaxRegistration":
			id := child(c, "ID")
			if id == nil {
				continue
			}
			switch s := attr(id, "schemeID"); {
			case strings.EqualFold(s, "VA"):
				p.VATID = text(id)
			case strings.EqualFold(s, "FC"):
				p.TaxRegistrationID = text(id)
			}
		}
	}
}

// readContact reads a ram:DefinedTradeContact; it returns nil when empty.
func readContact(n *xmltree.Node) *invoice.Contact {
	var c invoice.Contact
	for _, e := range n.Children {
		switch e.Name {
		case "PersonName":
			c.PersonName = text(e)
		case "DepartmentName":
			c.DepartmentName = text(e)
		case "TelephoneUniversalCommunication":
			c.Phone = childText(e, "CompleteNumber")
		case "EmailURIUniversalCommunication":
			c.Email = childText(e, "URIID")
		}
	}
	if c == (invoice.Contact{}) {
		return nil
	}
	return &c
}

func readAddress(n *xmltree.Node) invoice.Address {
	var a invoice.Address
	for _, c := range n.Children {
		switch c.Name {
		case "PostcodeCode":
			a.PostCode = text(c)
		case "LineOne":
			a.Line1 = text(c)
		case "LineTwo":
			a.Line2 = text(c)
		case "LineThree":
			a.Line3 = text(c)
		case "CityName":
			a.City = text(c)
		case "CountryID":
			a.CountryCode = text(c)
		case "CountrySubDivisionName":
			a.CountrySubdivision = text(c)
		}
	}
	return a
}

func (r *reader) line(n *xmltree.Node, l *invoice.Line) {
	for _, c := range n.Children {
		switch c.Name {
		case "AssociatedDocumentLineDocument":
			for _, cc := range c.Children {
				switch cc.Name {
				case "LineID":
					l.ID = text(cc)
				case "ParentLineID":
					l.ParentID = text(cc)
				case "LineStatusCode":
					l.StatusCode = text(cc)
				case "LineStatusReasonCode":
					l.StatusReasonCode = text(cc)
				case "IncludedNote":
					// EXTENDED allows several line notes: they are joined.
					for _, nc := range cc.Children {
						if nc.Name == "Content" {
							l.Note = joinText(l.Note, text(nc))
						}
					}
				}
			}
		case "SpecifiedTradeProduct":
			r.item(c, &l.Item)
		case "SpecifiedLineTradeAgreement":
			r.lineAgreement(c, l)
		case "SpecifiedLineTradeDelivery":
			r.lineDelivery(c, l)
		case "SpecifiedLineTradeSettlement":
			r.lineSettlement(c, l)
		}
	}
}

func (r *reader) item(n *xmltree.Node, it *invoice.Item) {
	for _, c := range n.Children {
		switch c.Name {
		case "GlobalID":
			if it.GlobalID.IsZero() {
				it.GlobalID = ident(c)
			}
		case "SellerAssignedID":
			it.SellerID = text(c)
		case "BuyerAssignedID":
			it.BuyerID = text(c)
		case "Name":
			it.Name = text(c)
		case "Description":
			it.Description = text(c)
		case "ApplicableProductCharacteristic":
			var a invoice.ItemAttribute
			for _, cc := range c.Children {
				switch cc.Name {
				case "TypeCode":
					a.TypeCode = text(cc)
				case "Description":
					a.Name = text(cc)
				case "Value":
					a.Value = text(cc)
				}
			}
			it.Attributes = append(it.Attributes, a)
		case "DesignatedProductClassification":
			var cl invoice.Classification
			for _, cc := range c.Children {
				switch cc.Name {
				case "ClassCode":
					cl.Code = text(cc)
					cl.ListID = attr(cc, "listID")
					cl.ListVersionID = attr(cc, "listVersionID")
				case "ClassName":
					cl.Name = text(cc)
				}
			}
			if cl != (invoice.Classification{}) {
				it.Classifications = append(it.Classifications, cl)
			}
		case "OriginTradeCountry":
			it.OriginCountry = childText(c, "ID")
		}
	}
}

func (r *reader) lineAgreement(n *xmltree.Node, l *invoice.Line) {
	var grossBasis *xmltree.Node
	for _, c := range n.Children {
		switch c.Name {
		case "BuyerOrderReferencedDocument":
			l.OrderReference = childText(c, "IssuerAssignedID")
			l.OrderLineReference = childText(c, "LineID")
		case "GrossPriceProductTradePrice":
			for _, cc := range c.Children {
				switch cc.Name {
				case "ChargeAmount":
					l.GrossPrice = r.decimal(cc)
				case "BasisQuantity":
					grossBasis = cc
				case "AppliedTradeAllowanceCharge":
					// BT-147 is the total of the price allowances (EXTENDED
					// allows several); charges on the price are not modeled.
					if ci := child(cc, "ChargeIndicator"); ci != nil && r.indicator(ci) {
						continue
					}
					if a := child(cc, "ActualAmount"); a != nil {
						if v := r.decimal(a); v.IsSet() {
							if l.PriceDiscount.IsSet() {
								v = l.PriceDiscount.Add(v)
							}
							l.PriceDiscount = v
						}
					}
				}
			}
		case "NetPriceProductTradePrice":
			for _, cc := range c.Children {
				switch cc.Name {
				case "ChargeAmount":
					l.NetPrice = r.decimal(cc)
				case "BasisQuantity":
					l.PriceBaseQuantity = r.decimal(cc)
					l.PriceBaseQuantityUnit = attr(cc, "unitCode")
				}
			}
		}
	}
	if grossBasis != nil && !l.PriceBaseQuantity.IsSet() {
		l.PriceBaseQuantity = r.decimal(grossBasis)
		l.PriceBaseQuantityUnit = attr(grossBasis, "unitCode")
	}
}

func (r *reader) lineDelivery(n *xmltree.Node, l *invoice.Line) {
	for _, c := range n.Children {
		switch c.Name {
		case "BilledQuantity":
			l.Quantity = r.decimal(c)
			l.UnitCode = attr(c, "unitCode")
		case "ShipToTradeParty":
			l.ShipTo = r.newParty(c)
		case "ActualDeliverySupplyChainEvent":
			if oc := child(c, "OccurrenceDateTime"); oc != nil {
				l.DeliveryDate = r.date(oc)
			}
		}
	}
}

func (r *reader) lineSettlement(n *xmltree.Node, l *invoice.Line) {
	for _, c := range n.Children {
		switch c.Name {
		case "ApplicableTradeTax":
			if l.VATCategoryCode == "" && !l.VATRate.IsSet() {
				for _, cc := range c.Children {
					switch cc.Name {
					case "CategoryCode":
						l.VATCategoryCode = text(cc)
					case "RateApplicablePercent":
						l.VATRate = r.decimal(cc)
					}
				}
			}
		case "BillingSpecifiedPeriod":
			l.Period = r.period(c)
		case "SpecifiedTradeAllowanceCharge":
			l.AllowancesCharges = append(l.AllowancesCharges, r.allowanceCharge(c))
		case "SpecifiedTradeSettlementLineMonetarySummation":
			if a := child(c, "LineTotalAmount"); a != nil {
				l.NetAmount = r.decimal(a)
			}
		case "InvoiceReferencedDocument":
			if d := r.docRef(c); d != (invoice.DocumentReference{}) {
				l.InvoiceReference = &d
			}
		case "AdditionalReferencedDocument":
			if tc := childText(c, "TypeCode"); (tc == "130" || tc == "") && l.ObjectIdentifier.IsZero() {
				l.ObjectIdentifier = invoice.Identifier{
					ID:     childText(c, "IssuerAssignedID"),
					Scheme: childText(c, "ReferenceTypeCode"),
				}
			}
		case "ReceivableSpecifiedTradeAccountingAccount":
			if l.AccountingReference == "" {
				l.AccountingReference = childText(c, "ID")
			}
		}
	}
}
