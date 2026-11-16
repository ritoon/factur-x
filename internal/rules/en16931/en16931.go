// Package en16931 implements the validation rules of EN 16931-1 for the CII
// syntax, as published by CEN TC 434 in its validation artefacts (release
// Release): the business rules BR-* and BR-CO-*, the decimal rules BR-DEC-*,
// the VAT category rules (BR-S, BR-Z, BR-E, BR-AE, BR-IC, BR-G, BR-O, BR-IP
// for IGIC, BR-IG for IPSI, BR-B), the code list rules BR-CL-* and the CII
// syntax rules CII-SR-* and CII-DT-*.
//
// The rules reproduce the XPath of the official schematron rather than the
// prose of the standard, so that the verdicts are the same as those of the
// official artefacts:
//
//   - amounts are exact decimals (xs:decimal), never floating point numbers;
//     round() rounds halves towards positive infinity;
//   - string comparisons are exact (no trimming) unless the rule uses
//     normalize-space(); code lists accept a value when, once trimmed, it
//     contains no space and is in the list;
//   - within a pattern, a node is checked by the first rule whose context
//     matches it only, so some rules never run on some nodes (for instance
//     the VAT breakdown rules BR-45 to BR-48 do not apply to the breakdowns
//     of category O, L and M, which have their own rules);
//   - assertions that can never fail in the official artefacts are not
//     implemented: BR-CO-05 to BR-CO-08, BR-IP-08, BR-IP-09, BR-IG-08,
//     BR-IG-09, CII-SR-039 to CII-SR-044 and CII-DT-010 to CII-DT-012.
//
// Severities are the official ones (fatal: Error, warning: Warning), with two
// adjustments that follow the reference validators of the profiles:
//
//   - XRECHNUNG: the severities set by the KoSIT validator configuration
//     XRechnung 3.0.2 (CII-SR-452, 453, 454, 465 and 466 are errors, BR-CL-21
//     and BR-CL-23 warnings, CII-SR-475 and 476 information);
//   - EXTENDED and EXTENDED-CTC-FR: Factur-X EXTENDED is only conformant
//     with EN 16931 and its schematron keeps a subset of the CEN rules.
//     The rules of that subset keep their severity; the other business rules
//     and code lists are reported as warnings, and the other CII syntax rules
//     (which restrict CII to the EN 16931 data set) are not reported.
//
// MINIMUM and BASIC WL invoices are not EN 16931 invoices: Check does nothing
// for them.
package en16931

import (
	"strings"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

//go:generate go run ./gen -o catalog_gen.go

// Check runs the rules on c.Doc and records the findings in c.Report.
func Check(c *rules.Checker) {
	d := c.Doc
	if d == nil || d.Root == nil {
		return
	}
	if p := d.Profile; p != invoice.ProfileUnknown && !p.EN16931() {
		return
	}
	var k checker
	k.init(c)
	k.walk(d.Root, trie, 0)
	k.finish()
}

// mode selects the severity adjustments of a profile.
type mode uint8

const (
	modeCEN mode = iota
	modeXRechnung
	modeXRechnungExtension
	modeXRechnungCVD
	modeExtended
)

// Category indexes of the VAT category codes of the EN 16931 subset.
const (
	catS = iota
	catZ
	catE
	catAE
	catK
	catG
	catO
	catL
	catM
	catB
	nCat
)

var catCodes = [nCat]string{"S", "Z", "E", "AE", "K", "G", "O", "L", "M", "B"}

// catIndex returns the index of a VAT category code compared exactly, or -1.
func catIndex(s string) int {
	switch s {
	case "S":
		return catS
	case "Z":
		return catZ
	case "E":
		return catE
	case "AE":
		return catAE
	case "K":
		return catK
	case "G":
		return catG
	case "O":
		return catO
	case "L":
		return catL
	case "M":
		return catM
	case "B":
		return catB
	}
	return -1
}

// nodeList is a list of nodes kept inline up to two elements, so that the
// checker does not allocate for the usual documents.
type nodeList struct {
	n    int
	head [2]*xmltree.Node
	more []*xmltree.Node
}

func (l *nodeList) add(n *xmltree.Node) {
	if l.n < len(l.head) {
		l.head[l.n] = n
	} else {
		l.more = append(l.more, n)
	}
	l.n++
}

func (l *nodeList) at(i int) *xmltree.Node {
	if i < len(l.head) {
		return l.head[i]
	}
	return l.more[i-len(l.head)]
}

type seenKey struct {
	rule string
	ctx  *xmltree.Node
}

// checker holds the state of one run.
type checker struct {
	c    *rules.Checker
	d    *rules.Doc
	mode mode

	stack [64]*xmltree.Node // ancestors of the visited node, by depth
	depth int
	seen  []seenKey // syntax findings already reported, per context node
	summ  nodeList  // ram:SpecifiedTradeSettlementHeaderMonetarySummation
	settl nodeList  // ram:ApplicableHeaderTradeSettlement

	// Facts known before the walk.
	sellerVAFC   bool // seller VAT identifier (schemeID VA) or tax registration (FC)
	sellerVA     bool
	taxRepVA     bool
	buyerVA      bool
	buyerLegalID bool
	billedUnit   bool // a line BilledQuantity has a unitCode (CII-DT-033)
	headerTax    bool // a VAT breakdown exists (BR-CO-18)

	// Facts collected during the walk.
	catHdr, catLine, catTax [nCat]int // ApplicableTradeTax / CategoryTradeTax elements per category
	anyCode                 [nCat]bool
	countryNotIT            bool // a ram:CountryID is not IT (BR-B-01)
	taxPointDate            bool // a ram:TaxPointDate exists (BR-CO-03)
	dueDateTypeCode         bool // a ram:DueDateTypeCode exists (BR-CO-03)
	applicableNotO          bool // an ApplicableTradeTax has a category other than O
	categoryNotO            bool // a CategoryTradeTax has a category other than O
	dueCode                 string
	dueCodes                int // distinct DueDateTypeCode values of ApplicableTradeTax (0, 1, 2 = several)
	pmType, pmInfo          *xmltree.Node
	pmTypeDiffer            bool
	pmInfoDiffer            bool
	paymentRefs             int
	financialCards          int
	debtorAccounts          int
}

func (k *checker) init(c *rules.Checker) {
	k.c = c
	k.d = c.Doc
	d := k.d
	spec := d.SpecificationID()
	switch d.Profile {
	case invoice.ProfileXRechnung:
		switch {
		case strings.Contains(spec, "#conformant#urn:xeinkauf.de:kosit:extension:xrechnung"):
			k.mode = modeXRechnungExtension
		case strings.Contains(spec, "xrechnung:cvd"):
			k.mode = modeXRechnungCVD
		default:
			k.mode = modeXRechnung
		}
	case invoice.ProfileExtended, invoice.ProfileExtendedCTCFR:
		k.mode = modeExtended
	}

	seller := d.Agreement.Elem("SellerTradeParty")
	for it, r := elems(seller, "SpecifiedTaxRegistration"); r != nil; r = it.next() {
		for it, id := elems(r, "ID"); id != nil; id = it.next() {
			switch s, _ := id.LookupAttr("schemeID"); s {
			case "VA":
				k.sellerVA = true
				k.sellerVAFC = true
			case "FC":
				k.sellerVAFC = true
			}
		}
	}
	k.taxRepVA = hasTaxRegistration(d.Agreement.Elem("SellerTaxRepresentativeTradeParty"), "VA")
	buyer := d.Agreement.Elem("BuyerTradeParty")
	k.buyerVA = hasTaxRegistration(buyer, "VA")
	for it, o := elems(buyer, "SpecifiedLegalOrganization"); o != nil; o = it.next() {
		if o.Elem("ID") != nil {
			k.buyerLegalID = true
		}
	}
	for _, l := range d.Lines {
		for it, del := elems(l, "SpecifiedLineTradeDelivery"); del != nil; del = it.next() {
			for it, q := elems(del, "BilledQuantity"); q != nil; q = it.next() {
				if _, ok := q.LookupAttr("unitCode"); ok {
					k.billedUnit = true
				}
			}
		}
	}
	for it, s := elems(d.Transaction, "ApplicableHeaderTradeSettlement"); s != nil; s = it.next() {
		if s.Elem("ApplicableTradeTax") != nil {
			k.headerTax = true
		}
	}
}

// hasTaxRegistration reports whether party has a SpecifiedTaxRegistration/ID
// with the given schemeID.
func hasTaxRegistration(party *xmltree.Node, scheme string) bool {
	for it, r := elems(party, "SpecifiedTaxRegistration"); r != nil; r = it.next() {
		for it, id := elems(r, "ID"); id != nil; id = it.next() {
			if s, ok := id.LookupAttr("schemeID"); ok && s == scheme {
				return true
			}
		}
	}
	return false
}

// severity maps the rule identifiers of Catalog to their severity.
var severity = func() map[string]rules.Severity {
	m := make(map[string]rules.Severity, len(Catalog))
	for _, r := range Catalog {
		m[r.ID] = r.Severity
	}
	return m
}()

// description maps the rule identifiers of Catalog to their description.
var description = func() map[string]string {
	m := make(map[string]string, len(Catalog))
	for _, r := range Catalog {
		m[r.ID] = r.Description
	}
	return m
}()

// fail records a finding for rule id located at n, with the severity of the
// rule adjusted for the profile.
func (k *checker) fail(id string, n *xmltree.Node, format string, args ...any) {
	sev, ok := severity[id]
	if !ok {
		sev = rules.Error
	}
	switch k.mode {
	case modeExtended:
		if !extendedRules[id] {
			if strings.HasPrefix(id, "CII-") {
				return
			}
			sev = rules.Warning
			format += " (not enforced for Factur-X EXTENDED)"
		}
	case modeXRechnung, modeXRechnungExtension, modeXRechnungCVD:
		if s, ok := xrechnungLevels[k.mode][id]; ok {
			sev = s
		}
	}
	if n == nil {
		n = k.d.Root
	}
	k.c.Fail(id, sev, n, format, args...)
}

// failOnce records a finding for rule id unless one was already recorded for
// the same context node.
func (k *checker) failOnce(id string, ctx, n *xmltree.Node, format string, args ...any) {
	for _, s := range k.seen {
		if s.rule == id && s.ctx == ctx {
			return
		}
	}
	k.seen = append(k.seen, seenKey{id, ctx})
	k.fail(id, n, format, args...)
}

// failDesc records a finding whose message is the official description of the
// rule.
func (k *checker) failDesc(id string, n *xmltree.Node) {
	k.fail(id, n, "%s", description[id])
}

// parentIs reports whether the parent of the visited node is named names[0],
// its grandparent names[1], and so on.
func (k *checker) parentIs(names ...string) bool {
	for i, name := range names {
		j := k.depth - 1 - i
		if j < 0 || k.stack[j].Name != name {
			return false
		}
	}
	return true
}

// hasAncestor reports whether an ancestor of the visited node has the given
// local name.
func (k *checker) hasAncestor(name string) bool {
	for i := k.depth - 1; i >= 0; i-- {
		if k.stack[i].Name == name {
			return true
		}
	}
	return false
}
