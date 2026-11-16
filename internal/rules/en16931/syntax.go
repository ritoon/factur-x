package en16931

import (
	"strings"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
)

// The CII syntax rules (pattern EN16931-CII-Syntax) restrict the CII schema
// to the EN 16931 data set. Most of them are generated (catalog_gen.go): an
// element or attribute that must not exist (syntaxNots), or a number of
// occurrences (syntaxCounts). The others are written below.
//
// Rules whose context is an absolute path are checked with a trie of element
// names, walked together with the document; rules whose context is an
// element name or a name suffix ("//ram:*[ends-with(name(), 'ID')]") are
// checked by kind. As in the official schematron, a node that is the context
// of an absolute rule is not checked by the name rules declared after it.

type syntaxNot struct {
	Rule    string
	Context string
	Path    string
}

type syntaxCount struct {
	Rule    string
	Context string
	Path    string
	Exact   bool // count(path) = N, otherwise count(path) <= N
	N       int
}

// tnode is a node of the trie of absolute element paths.
type tnode struct {
	name     string
	children []*tnode
	ctx      bool // the element is the context of absolute rules
	nots     []notHit
	attrs    []notHit
	counts   []countRule
	special  special // hand written rules of the context, if any
}

// notHit is a rule violated when the element (or its attribute) exists.
type notHit struct {
	rule     string
	attr     string
	ctxDepth int // depth of the context node; the root element is at depth 0
}

type countRule struct {
	rule  string
	path  []string
	exact bool
	n     int
}

func (t *tnode) child(name string) *tnode {
	for _, c := range t.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

func (t *tnode) add(name string) *tnode {
	if c := t.child(name); c != nil {
		return c
	}
	c := &tnode{name: name}
	t.children = append(t.children, c)
	return c
}

// kind identifies the syntax rules whose context is a name or a name suffix.
type kind uint8

const (
	kindNone kind = iota
	kindPaymentMeans
	kindAllowanceCharge
	kindGrossAllowanceCharge
	kindDocContext
	kindID
	kindTypeCode
	kindRefDoc
	kindAmount
	kindQuantity
	kindTradeTax
	kindPeriod
	kindAddress
	nKinds
)

var kindContexts = map[string]kind{
	"//ram:SpecifiedTradeSettlementPaymentMeans":                              kindPaymentMeans,
	"//ram:SpecifiedTradeAllowanceCharge":                                     kindAllowanceCharge,
	"//ram:GrossPriceProductTradePrice/ram:AppliedTradeAllowanceCharge":       kindGrossAllowanceCharge,
	"//*[ends-with(name(), 'DocumentContextParameter')]":                      kindDocContext,
	"//ram:*[ends-with(name(), 'ID')]":                                        kindID,
	"//ram:TypeCode":                                                          kindTypeCode,
	"//ram:*[ends-with(name(), 'ReferencedDocument')]":                        kindRefDoc,
	"//ram:*[ends-with(name(), 'Amount') and not (self::ram:TaxTotalAmount)]": kindAmount,
	"//ram:*[ends-with(name(), 'Quantity')]":                                  kindQuantity,
	"//ram:*[ends-with(name(), 'TradeTax')]":                                  kindTradeTax,
	"//ram:BillingSpecifiedPeriod":                                            kindPeriod,
	"//ram:PostalTradeAddress":                                                kindAddress,
}

// kindNot is a rule violated when path (relative to the node) exists, or
// when the element at path carries attribute attr.
type kindNot struct {
	rule string
	path []string
	attr string
}

type kindSet struct {
	nots   []kindNot
	counts []countRule
}

// Absolute contexts of the rules written by hand.
const (
	ctxRoot            = "/rsm:CrossIndustryInvoice"
	ctxAgreement       = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement"
	ctxDelivery        = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeDelivery"
	ctxSettlement      = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeSettlement"
	ctxProduct         = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:IncludedSupplyChainTradeLineItem/ram:SpecifiedTradeProduct"
	ctxLineAgreement   = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:IncludedSupplyChainTradeLineItem/ram:SpecifiedLineTradeAgreement"
	ctxLineSettlement  = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:IncludedSupplyChainTradeLineItem/ram:SpecifiedLineTradeSettlement"
	ctxAdditionalDoc   = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement/ram:AdditionalReferencedDocument"
	ctxAttachment      = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement/ram:AdditionalReferencedDocument/ram:AttachmentBinaryObject"
	ctxLineCategory    = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:IncludedSupplyChainTradeLineItem/ram:SpecifiedLineTradeSettlement/ram:ApplicableTradeTax/ram:CategoryCode"
	ctxIncludedNote    = "/rsm:CrossIndustryInvoice/rsm:ExchangedDocument/ram:IncludedNote"
	ctxDocumentContext = "/rsm:CrossIndustryInvoice/rsm:ExchangedDocumentContext"
)

// special identifies the hand written syntax rules of an absolute context.
// A switch rather than function values keeps the checker on the stack.
type special uint8

const (
	specialNone special = iota
	specialAgreement
	specialDelivery
	specialSettlement
	specialProduct
	specialLineAgreement
	specialLineSettlement
	specialAdditionalDoc
	specialAttachment
)

var specials = map[string]special{
	ctxAgreement:      specialAgreement,
	ctxDelivery:       specialDelivery,
	ctxSettlement:     specialSettlement,
	ctxProduct:        specialProduct,
	ctxLineAgreement:  specialLineAgreement,
	ctxLineSettlement: specialLineSettlement,
	ctxAdditionalDoc:  specialAdditionalDoc,
	ctxAttachment:     specialAttachment,
	ctxRoot:           specialNone, // rules needing the whole document run in finish
}

// runSpecial runs the hand written syntax rules of context node n.
func (k *checker) runSpecial(s special, n *xmltree.Node) {
	switch s {
	case specialAgreement:
		k.syntaxAgreement(n)
	case specialDelivery:
		k.syntaxDelivery(n)
	case specialSettlement:
		k.syntaxSettlement(n)
	case specialProduct:
		k.syntaxProduct(n)
	case specialLineAgreement:
		k.syntaxLineAgreement(n)
	case specialLineSettlement:
		k.syntaxLineSettlement(n)
	case specialAdditionalDoc:
		k.syntaxAdditionalDoc(n)
	case specialAttachment:
		k.syntaxAttachment(n)
	}
}

var (
	trie  *tnode
	kinds [nKinds]kindSet
)

func init() {
	root := &tnode{name: "CrossIndustryInvoice"}
	// contexts returns the trie nodes of the absolute contexts of ctx, or
	// nil and the kind for a name context.
	contexts := func(ctx string) ([]*tnode, []int) {
		var nodes []*tnode
		var depths []int
		for _, alt := range strings.Split(ctx, "|") {
			alt = strings.TrimSpace(alt)
			if !strings.HasPrefix(alt, "/rsm:CrossIndustryInvoice") {
				panic("en16931: unsupported syntax context " + ctx)
			}
			segs := steps(alt)
			t := root
			for _, s := range segs[1:] {
				t = t.add(s)
			}
			t.ctx = true
			nodes = append(nodes, t)
			depths = append(depths, len(segs)-1)
		}
		return nodes, depths
	}
	for _, r := range syntaxNots {
		path, attr := splitAttr(r.Path)
		if kd, ok := kindContexts[r.Context]; ok {
			kinds[kd].nots = append(kinds[kd].nots, kindNot{rule: r.Rule, path: path, attr: attr})
			continue
		}
		nodes, depths := contexts(r.Context)
		for i, t := range nodes {
			for _, s := range path {
				t = t.add(s)
			}
			h := notHit{rule: r.Rule, attr: attr, ctxDepth: depths[i]}
			if attr != "" {
				t.attrs = append(t.attrs, h)
			} else {
				t.nots = append(t.nots, h)
			}
		}
	}
	for _, r := range syntaxCounts {
		cr := countRule{rule: r.Rule, path: steps(r.Path), exact: r.Exact, n: r.N}
		if kd, ok := kindContexts[r.Context]; ok {
			kinds[kd].counts = append(kinds[kd].counts, cr)
			continue
		}
		nodes, _ := contexts(r.Context)
		for _, t := range nodes {
			t.counts = append(t.counts, cr)
		}
	}
	for ctx, sp := range specials {
		nodes, _ := contexts(ctx)
		for _, t := range nodes {
			t.special = sp
		}
	}
	trie = root
}

// steps returns the local names of the steps of a path ("ram:A/ram:B").
func steps(path string) []string {
	var out []string
	for _, s := range strings.Split(path, "/") {
		if s == "" {
			continue
		}
		if i := strings.IndexByte(s, ':'); i >= 0 {
			s = s[i+1:]
		}
		out = append(out, s)
	}
	return out
}

// splitAttr splits "ram:A/ram:B/@attr" into its element steps and attribute.
func splitAttr(path string) ([]string, string) {
	if i := strings.LastIndex(path, "@"); i >= 0 {
		return steps(path[:i]), path[i+1:]
	}
	return steps(path), ""
}

// nodeKind returns the kind of the name rules applying to n, which is not the
// context of an absolute rule.
func nodeKind(n *xmltree.Node) kind {
	name := n.Name
	switch name {
	case "SpecifiedTradeSettlementPaymentMeans":
		return kindPaymentMeans
	case "SpecifiedTradeAllowanceCharge":
		return kindAllowanceCharge
	case "AppliedTradeAllowanceCharge":
		if n.Parent != nil && n.Parent.Name == "GrossPriceProductTradePrice" {
			return kindGrossAllowanceCharge
		}
		return kindNone
	case "TypeCode":
		return kindTypeCode
	case "BillingSpecifiedPeriod":
		return kindPeriod
	case "PostalTradeAddress":
		return kindAddress
	case "TaxTotalAmount":
		return kindNone
	}
	switch {
	case strings.HasSuffix(name, "DocumentContextParameter"):
		return kindDocContext
	case strings.HasSuffix(name, "ID"):
		return kindID
	case strings.HasSuffix(name, "ReferencedDocument"):
		return kindRefDoc
	case strings.HasSuffix(name, "Amount"):
		return kindAmount
	case strings.HasSuffix(name, "Quantity"):
		return kindQuantity
	case strings.HasSuffix(name, "TradeTax"):
		return kindTradeTax
	}
	return kindNone
}

// trieChecks runs the absolute syntax rules attached to the trie node of n.
func (k *checker) trieChecks(n *xmltree.Node, t *tnode) {
	for _, h := range t.nots {
		k.failOnce(h.rule, k.stack[h.ctxDepth], n, "%s", description[h.rule])
	}
	for _, h := range t.attrs {
		if v, ok := n.LookupAttr(h.attr); ok {
			k.failOnce(h.rule, k.stack[h.ctxDepth], n, "%s (@%s=%q)", description[h.rule], h.attr, quote(v))
		}
	}
	if !t.ctx {
		return
	}
	for _, cr := range t.counts {
		k.checkCount(n, cr)
	}
	if t.special != specialNone {
		k.runSpecial(t.special, n)
	}
}

func (k *checker) checkCount(n *xmltree.Node, cr countRule) {
	c := count(n, cr.path)
	if (cr.exact && c != cr.n) || (!cr.exact && c > cr.n) {
		k.fail(cr.rule, n, "%s (%d found)", description[cr.rule], c)
	}
}

// kindChecks runs the name syntax rules applying to n.
func (k *checker) kindChecks(n *xmltree.Node) {
	kd := nodeKind(n)
	if kd == kindNone {
		if n.Name == "DateTimeString" && n.Space == rules.NSUdt {
			if f, ok := n.LookupAttr("format"); ok && f == "102" && !date102(n.Text) {
				k.fail("CII-DT-097", n, "date %q is not in format 102 (YYYYMMDD)", quote(n.Text))
			}
		}
		return
	}
	ks := &kinds[kd]
	for i := range ks.nots {
		r := &ks.nots[i]
		m := n
		if len(r.path) > 0 {
			m = find(n, r.path...)
			if m == nil {
				continue
			}
		}
		if r.attr == "" {
			k.fail(r.rule, m, "%s", description[r.rule])
			continue
		}
		if r.path == nil {
			if v, ok := m.LookupAttr(r.attr); ok {
				k.fail(r.rule, m, "%s (@%s=%q)", description[r.rule], r.attr, quote(v))
			}
			continue
		}
		// The attribute may be on any element at path.
		k.attrOnPath(r.rule, n, r.path, r.attr)
	}
	for _, cr := range ks.counts {
		k.checkCount(n, cr)
	}
	switch kd {
	case kindAllowanceCharge:
		if n.Elem("ChargeIndicator") == nil {
			k.failDesc("CII-SR-463", n)
		}
	case kindRefDoc:
		k.syntaxRefDoc(n)
	case kindQuantity:
		if _, ok := n.LookupAttr("unitCode"); ok && !k.billedUnit {
			k.failDesc("CII-DT-033", n)
		}
	case kindTradeTax:
		k.syntaxTradeTax(n)
	}
}

// attrOnPath reports rule when an element at path below n carries attr.
func (k *checker) attrOnPath(rule string, n *xmltree.Node, path []string, attr string) {
	if len(path) == 0 {
		if v, ok := n.LookupAttr(attr); ok {
			k.fail(rule, n, "%s (@%s=%q)", description[rule], attr, quote(v))
		}
		return
	}
	for _, c := range n.Children {
		if c.Name == path[0] {
			k.attrOnPath(rule, c, path[1:], attr)
		}
	}
}

// date102 reports whether s matches
// ^\s*(\d{4})(1[0-2]|0[1-9]){1}(3[01]|[12][0-9]|0[1-9]){1}\s*$ (CII-DT-097).
func date102(s string) bool {
	s = trim(s)
	if len(s) != 8 {
		return false
	}
	for i := 0; i < 8; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	m := int(s[4]-'0')*10 + int(s[5]-'0')
	d := int(s[6]-'0')*10 + int(s[7]-'0')
	return m >= 1 && m <= 12 && d >= 1 && d <= 31
}

func (k *checker) syntaxRefDoc(n *xmltree.Node) {
	ard := n.Name == "AdditionalReferencedDocument"
	tc916 := ard && childText(n, "TypeCode", "916")
	if m := n.Elem("URIID"); m != nil && !tc916 {
		k.failDesc("CII-DT-015", m)
	}
	if m := n.Elem("TypeCode"); m != nil && !(ard && (childText(n, "TypeCode", "50") || childText(n, "TypeCode", "130") || tc916)) {
		k.fail("CII-DT-018", m, "%s (TypeCode %q)", description["CII-DT-018"], quote(m.Text))
	}
	if m := n.Elem("Name"); m != nil && !tc916 {
		k.failDesc("CII-DT-021", m)
	}
	if m := n.Elem("AttachmentBinaryObject"); m != nil && !tc916 {
		k.failDesc("CII-DT-022", m)
	}
	if m := n.Elem("ReferenceTypeCode"); m != nil && !(ard && childText(n, "TypeCode", "130")) {
		k.failDesc("CII-DT-024", m)
	}
	if m := n.Elem("FormattedIssueDateTime"); m != nil && n.Name != "InvoiceReferencedDocument" {
		k.failDesc("CII-DT-027", m)
	}
}

func (k *checker) syntaxTradeTax(n *xmltree.Node) {
	if tc := n.Elem("TypeCode"); tc != nil && !childText(n, "TypeCode", "VAT") {
		k.fail("CII-DT-037", tc, "%s (TypeCode %q)", description["CII-DT-037"], quote(tc.Text))
	}
	header := k.hasAncestor("ApplicableHeaderTradeSettlement")
	applicable := n.Name == "ApplicableTradeTax"
	if m := n.Elem("BasisAmount"); m != nil && !header {
		k.failDesc("CII-DT-041", m)
	}
	if m := n.Elem("ExemptionReasonCode"); m != nil && !applicable {
		k.failDesc("CII-DT-052", m)
	}
	if m := n.Elem("ExemptionReason"); m != nil && !applicable {
		k.failDesc("CII-DT-098", m)
	}
	if m := n.Elem("TaxPointDate"); m != nil && !header {
		k.failDesc("CII-DT-054", m)
	}
	if m := n.Elem("DueDateTypeCode"); m != nil && !header {
		k.failDesc("CII-DT-058", m)
	}
}

// both reports whether party (a child of n) has both an ID and a GlobalID.
func both(n *xmltree.Node, party string) *xmltree.Node {
	if exists(n, party, "ID") && exists(n, party, "GlobalID") {
		return n.Elem(party)
	}
	return nil
}

func (k *checker) syntaxAgreement(n *xmltree.Node) {
	if p := both(n, "BuyerTradeParty"); p != nil {
		k.failDesc("CII-SR-450", p)
	}
	n50, n130 := 0, 0
	for it, d := elems(n, "AdditionalReferencedDocument"); d != nil; d = it.next() {
		if childText(d, "TypeCode", "50") {
			n50++
		}
		if childText(d, "TypeCode", "130") {
			n130++
		}
	}
	if n50 > 1 {
		k.fail("CII-SR-457", n, "%s (%d found)", description["CII-SR-457"], n50)
	}
	if n130 > 1 {
		k.fail("CII-SR-458", n, "%s (%d found)", description["CII-SR-458"], n130)
	}
	if exists(n, "SellerTradeParty", "DefinedTradeContact", "PersonName") && exists(n, "SellerTradeParty", "DefinedTradeContact", "DepartmentName") {
		k.fail("CII-SR-465", find(n, "SellerTradeParty", "DefinedTradeContact"), "%s (the seller contact has both a PersonName and a DepartmentName)", description["CII-SR-465"])
	}
	if exists(n, "BuyerTradeParty", "DefinedTradeContact", "PersonName") && exists(n, "BuyerTradeParty", "DefinedTradeContact", "DepartmentName") {
		k.fail("CII-SR-466", find(n, "BuyerTradeParty", "DefinedTradeContact"), "%s (the buyer contact has both a PersonName and a DepartmentName)", description["CII-SR-466"])
	}
}

func (k *checker) syntaxDelivery(n *xmltree.Node) {
	if p := both(n, "ShipToTradeParty"); p != nil {
		k.failDesc("CII-SR-449", p)
	}
}

func (k *checker) syntaxSettlement(n *xmltree.Node) {
	if p := both(n, "PayeeTradeParty"); p != nil {
		k.failDesc("CII-SR-451", p)
	}
	for it, pm := elems(n, "SpecifiedTradeSettlementPaymentMeans"); pm != nil; pm = it.next() {
		tc := trim(text(pm.Elem("TypeCode")))
		if (tc == "30" || tc == "58") && !exists(pm, "PayeePartyCreditorFinancialAccount", "IBANID") && !exists(pm, "PayeePartyCreditorFinancialAccount", "ProprietaryID") {
			k.fail("CII-SR-470", pm, "%s (payment means %s)", description["CII-SR-470"], tc)
		}
	}
}

func (k *checker) syntaxProduct(n *xmltree.Node) {
	if gid := n.Elem("GlobalID"); gid != nil {
		ok := false
		for it, g := elems(n, "GlobalID"); g != nil; g = it.next() {
			if _, has := g.LookupAttr("schemeID"); has {
				ok = true
			}
		}
		if !ok {
			k.failDesc("CII-SR-046", gid)
		}
	}
	if o := n.Elem("OriginTradeCountry"); o != nil {
		if c := count(n, []string{"OriginTradeCountry", "ID"}); c != 1 {
			k.fail("CII-SR-090", o, "%s (%d found)", description["CII-SR-090"], c)
		}
	}
}

func (k *checker) syntaxLineAgreement(n *xmltree.Node) {
	hasCI, hasCIFalse, hasAmount := false, false, false
	var at *xmltree.Node
	for it, g := elems(n, "GrossPriceProductTradePrice"); g != nil; g = it.next() {
		for it, ac := elems(g, "AppliedTradeAllowanceCharge"); ac != nil; ac = it.next() {
			at = ac
			if ac.Elem("ActualAmount") != nil {
				hasAmount = true
			}
			for it, ci := elems(ac, "ChargeIndicator"); ci != nil; ci = it.next() {
				hasCI = true
				for it, ind := elems(ci, "Indicator"); ind != nil; ind = it.next() {
					if v, ok := boolean(ind.Text); ok && !v {
						hasCIFalse = true
					}
				}
			}
		}
	}
	if !((hasCIFalse && hasAmount) || (!hasCI && !hasAmount)) {
		k.failDesc("CII-SR-119", at)
	}
}

func (k *checker) syntaxLineSettlement(n *xmltree.Node) {
	c := 0
	for it, d := elems(n, "AdditionalReferencedDocument"); d != nil; d = it.next() {
		if trim(text(d.Elem("TypeCode"))) == "130" {
			c++
		}
	}
	if c > 1 {
		k.fail("CII-SR-474", n, "%s (%d found)", description["CII-SR-474"], c)
	}
}

func (k *checker) syntaxAdditionalDoc(n *xmltree.Node) {
	if trim(text(n.Elem("TypeCode"))) != "916" {
		return
	}
	if c := countElems(n, "Name"); c > 1 {
		k.fail("CII-SR-475", n, "%s (%d found)", description["CII-SR-475"], c)
	}
	if c := countElems(n, "AttachmentBinaryObject"); c > 1 {
		k.fail("CII-SR-476", n, "%s (%d found)", description["CII-SR-476"], c)
	}
}

func (k *checker) syntaxAttachment(n *xmltree.Node) {
	if _, ok := n.LookupAttr("mimeCode"); !ok {
		k.failDesc("CII-DT-105", n)
	}
	if _, ok := n.LookupAttr("filename"); !ok {
		k.failDesc("CII-DT-106", n)
	}
}

// syntaxRoot runs the syntax rules of context /rsm:CrossIndustryInvoice that
// need the whole document, and CII-SR-462 (context ApplicableHeaderTradeSettlement).
func (k *checker) syntaxRoot() {
	root := k.d.Root
	if k.pmTypeDiffer {
		k.failDesc("CII-SR-467", k.pmType)
	}
	if k.pmInfoDiffer {
		k.failDesc("CII-SR-468", k.pmInfo)
	}
	if k.paymentRefs > 1 {
		k.fail("CII-SR-469", root, "%s (%d found)", description["CII-SR-469"], k.paymentRefs)
	}
	if k.financialCards > 1 {
		k.fail("CII-SR-495", root, "%s (%d found)", description["CII-SR-495"], k.financialCards)
	}
	if k.debtorAccounts > 1 {
		k.fail("CII-SR-496", root, "%s (%d found)", description["CII-SR-496"], k.debtorAccounts)
	}
	if k.dueCodes > 1 {
		for i := 0; i < k.settl.n; i++ {
			if s := k.settl.at(i); s.Parent == k.d.Transaction && s.Parent.Parent == root {
				k.failDesc("CII-SR-462", s)
			}
		}
	}
}
