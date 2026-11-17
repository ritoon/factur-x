package fr

import (
	"iter"
	"strings"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
)

// This file implements the schematron BR-FR-Flux2-Schematron-UBL version
// 1.4.0.04 on the UBL document itself (rules.Doc.UBL): the identifiers,
// contexts and messages are those of the UBL schematron, which differ from
// the CII ones (see UBLCatalog).

// checkUBL runs the assertions of the UBL schematron on a UBL Invoice or
// CreditNote.
func checkUBL(c *rules.Checker) {
	k := newUBLChecker(c)
	k.identifiers()     // BR-FR-01, BR-FR-02
	k.dates()           // BR-FR-03
	k.documentTypes()   // BR-FR-04
	k.noteCodes()       // BR-FR-05, BR-FR-06
	k.billingMode()     // BR-FR-08
	k.siretSiren()      // BR-FR-09
	k.sellerSiren()     // BR-FR-10
	k.buyerSiren()      // BR-FR-11
	k.endpointPresent() // BR-FR-12, BR-FR-13
	k.vatCategories()   // BR-FR-15
	k.vatRates()        // BR-FR-16
	k.attachments()     // BR-FR-17, BR-FR-18
	k.barTreatment()    // BR-FR-20
	k.b2bEndpoints()    // BR-FR-21, BR-FR-22
	k.endpointFormats() // BR-FR-23
	k.privateIDFormat() // BR-FR-24
	k.endpointLengths() // BR-FR-25
	k.privateIDLength() // BR-FR-26
	k.characteristics() // BR-FR-27, BR-FR-28
	k.objects()         // BR-FR-29, BR-FR-30
	k.barUnique()       // pattern BR-FR-31 (BR-FR-30_BT-21)
	k.legalIDs()        // BR-FR-32
	k.coherence()       // BR-FR-CO-*
	k.decimals()        // BR-FR-DEC-*
	k.multiVendor()     // BR-FR-MV-*, BR-FR-BD-*
}

// ublChecker wraps rules.Checker with the values shared by the UBL rules.
type ublChecker struct {
	*rules.Checker
	root     *xmltree.Node // ubl:Invoice or cn:CreditNote
	credit   bool          // the root is cn:CreditNote
	line     string        // InvoiceLine or CreditNoteLine, the lines of the root
	typeCode string        // InvoiceTypeCode or CreditNoteTypeCode, BT-3 of the root

	special bool // custom:isSpecialContract: BT-23 is S8, B8, M8, S9, B9 or M9
	mv      bool // custom:isSpecialContractMV: S8, B8 or M8
	bd      bool // custom:isSpecialContractBD: S9, B9 or M9

	allNotes string          // string-join(cbc:Note, '#')
	invIDs   []*xmltree.Node // $invoiceID: the cbc:ID of the root (BT-1)
}

func newUBLChecker(c *rules.Checker) *ublChecker {
	root := c.Doc.UBL
	k := &ublChecker{Checker: c, root: root, line: "InvoiceLine", typeCode: "InvoiceTypeCode"}
	if root.Name == "CreditNote" {
		k.credit, k.line, k.typeCode = true, "CreditNoteLine", "CreditNoteTypeCode"
	}
	if p := first(root, "ProfileID"); p != nil {
		switch nsv(p) {
		case "S8", "B8", "M8":
			k.special, k.mv = true, true
		case "S9", "B9", "M9":
			k.special, k.bd = true, true
		}
	}
	var notes []string
	for n := range all(root, "Note") {
		notes = append(notes, sv(n))
	}
	k.allNotes = strings.Join(notes, "#")
	k.invIDs = nodes(root, "ID")
	return k
}

// fail records an error: every BR-FR assertion is fatal. The finding is
// located at the context node of the rule, the root element when there is
// none.
func (k *ublChecker) fail(rule string, n *xmltree.Node, format string, args ...any) {
	if n == nil {
		n = k.root
	}
	k.Fail(rule, rules.Error, n, format, args...)
}

// lines yields the lines of the root (ubl:Invoice/cac:InvoiceLine |
// cn:CreditNote/cac:CreditNoteLine).
func (k *ublChecker) lines() iter.Seq[*xmltree.Node] { return all(k.root, k.line) }

// anyLines returns the children of the root named InvoiceLine or
// CreditNoteLine, whatever the root ((cac:InvoiceLine | cac:CreditNoteLine)).
func (k *ublChecker) anyLines() []*xmltree.Node {
	var out []*xmltree.Node
	for _, c := range k.root.Children {
		if c.Name == "InvoiceLine" || c.Name == "CreditNoteLine" {
			out = append(out, c)
		}
	}
	return out
}

// anywhere yields, in document order, the elements matched by a rule
// context without leading root ("a/b/c"): elements named c whose parent is
// named b and grandparent a, at any depth.
func anywhere(root *xmltree.Node, pattern string) iter.Seq[*xmltree.Node] {
	segs := strings.Split(pattern, "/")
	return func(yield func(*xmltree.Node) bool) {
		stop := false
		root.Walk(func(n *xmltree.Node) bool {
			if stop {
				return false
			}
			if matchUp(n, segs) && !yield(n) {
				stop = true
			}
			return !stop
		})
	}
}

// matchUp reports whether n and its ancestors are named, from the bottom,
// like the last segments of segs.
func matchUp(n *xmltree.Node, segs []string) bool {
	for i := len(segs) - 1; i >= 0; i-- {
		if n == nil || n.Name != segs[i] {
			return false
		}
		n = n.Parent
	}
	return true
}

// withAttr returns the elements matching path below n whose attribute attr
// equals v (path[@attr = 'v']).
func withAttr(n *xmltree.Node, path, attr, v string) []*xmltree.Node {
	var out []*xmltree.Node
	for c := range all(n, path) {
		if attrIs(c, attr, v) {
			out = append(out, c)
		}
	}
	return out
}

// eqNodes reports whether an element matching path below n has the string
// value of one of vs (the general comparison path = $vs).
func eqNodes(n *xmltree.Node, path string, vs []*xmltree.Node) bool {
	for c := range all(n, path) {
		s := sv(c)
		for _, v := range vs {
			if s == sv(v) {
				return true
			}
		}
	}
	return false
}

// neNodes reports whether an element matching path below n has a string
// value other than one of vs (the general comparison path != $vs).
func neNodes(n *xmltree.Node, path string, vs []*xmltree.Node) bool {
	for c := range all(n, path) {
		s := sv(c)
		for _, v := range vs {
			if s != sv(v) {
				return true
			}
		}
	}
	return false
}

// strIn reports whether s equals the string value of one of vs (s = $vs).
func strIn(s string, vs []*xmltree.Node) bool {
	for _, v := range vs {
		if s == sv(v) {
			return true
		}
	}
	return false
}

// noteValue returns the value that follows #code# in the joined notes, up to
// the next # (substring-after, then substring-before when a # follows).
func noteValue(notes, code string) string {
	_, after, ok := strings.Cut(notes, "#"+code+"#")
	if !ok {
		return ""
	}
	v, _, _ := strings.Cut(after, "#")
	return v
}

// validUBLDate implements custom:is-valid-date-format of the UBL schematron:
// matches(s, '^20\d{2}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$') and a day
// that exists in the month. Only ASCII digits are accepted: other digits
// would make the cast of the year fail in the reference.
func validUBLDate(s string) bool {
	if len(s) != 10 || s[0] != '2' || s[1] != '0' || s[4] != '-' || s[7] != '-' {
		return false
	}
	for _, i := range [...]int{2, 3, 5, 6, 8, 9} {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return validDate(s[:4] + s[5:7] + s[8:10])
}

// identifiers checks BR-FR-01 (length and format) and BR-FR-02 (format) on
// BT-1, BT-25 and EXT-FR-FE-136.
func (k *ublChecker) identifiers() {
	for _, p := range []struct{ path, bt, label string }{
		{"ID", "BT-1", "L'identifiant de facture (cbc:ID)"},
		{"BillingReference/InvoiceDocumentReference/ID", "BT-25", "L'identifiant de facture référencée (cbc:ID)"},
		{k.line + "/BillingReference/InvoiceDocumentReference/ID", "EXT-FR-FE-136", "L'identifiant de facture référencée en ligne (cbc:ID)"},
	} {
		for n := range all(k.root, p.path) {
			s := sv(n)
			if l := length(s); l > 35 {
				k.fail("BR-FR-01_"+p.bt+"-1", n, "BR-FR-01/%s : %s ne doit pas dépasser 35 caractères ; valeur actuelle : %q (%d caractères).", p.bt, p.label, s, l)
			}
			if !validID(s) {
				k.fail("BR-FR-01_"+p.bt+"-2", n, "BR-FR-01/%s : %s contient des caractères non autorisés ; seuls les caractères alphanumériques et les symboles + - _ / sont autorisés, sans espaces ; valeur actuelle : %q.", p.bt, p.label, s)
			}
		}
	}
	for _, p := range []struct{ path, bt, label string }{
		{"ID", "BT-1", "L'identifiant de facture (cbc:ID)"},
		{"BillingReference/InvoiceDocumentReference/ID", "BT-25", "L'identifiant de facture référencée (cbc:ID)"},
		{k.line + "/BillingReference/InvoiceDocumentReference/ID", "EXT-FR-FE-136", "L'identifiant de facture référencée en ligne (cbc:ID)"},
	} {
		for n := range all(k.root, p.path) {
			if s := sv(n); !validID(s) {
				k.fail("BR-FR-02_"+p.bt, n, "BR-FR-02/%s : %s doit être composé uniquement de caractères alphanumériques et des symboles + - _ /, sans espaces ; valeur actuelle : %q.", p.bt, p.label, s)
			}
		}
	}
}

// dates checks BR-FR-03: dates are valid dates YYYY-MM-DD of the years 2000
// to 2099.
func (k *ublChecker) dates() {
	ref := "BillingReference/InvoiceDocumentReference/IssueDate"
	for _, d := range []struct{ path, id, label string }{
		{"IssueDate", "BR-FR-03_BT-2", "La date d'émission (cbc:IssueDate)"},
		{"TaxPointDate", "BR-FR-03_BT-7", "La date de fait générateur de la taxe (cbc:TaxPointDate)"},
		{"DueDate", "BR-FR-03_BT-9", "La date d'échéance (cbc:DueDate)"},
		{ref, "BR-FR-03_BT-26", "La date d'émission de la facture référencée (cbc:IssueDate)"},
		{"Delivery/ActualDeliveryDate", "BR-FR-03_BT-72", "La date de livraison effective (cbc:ActualDeliveryDate)"},
		{"InvoicePeriod/StartDate", "BR-FR-03_BT-73", "La date de début de période de facturation (cbc:StartDate)"},
		{"InvoicePeriod/EndDate", "BR-FR-03_BT-74", "La date de fin de période de facturation (cbc:EndDate)"},
		{k.line + "/" + ref, "BR-FR-03_EXT-FR-FE-138", "La date d'émission de la facture référencée en ligne (cbc:IssueDate)"},
		{k.line + "/Delivery/ActualDeliveryDate", "BR-FR-03_EXT-FR-FE-158", "La date de livraison effective en ligne (cbc:ActualDeliveryDate)"},
		{k.line + "/InvoicePeriod/StartDate", "BR-FR-03_BT-134", "La date de début de période de facturation en ligne (cbc:StartDate)"},
		{k.line + "/InvoicePeriod/EndDate", "BR-FR-03_BT-135", "La date de fin de période de facturation en ligne (cbc:EndDate)"},
	} {
		for n := range all(k.root, d.path) {
			if s := sv(n); !validUBLDate(s) {
				k.fail(d.id, n, "BR-FR-03 : %s doit être une date valide au format AAAA-MM-JJ avec une année comprise entre 2000 et 2099 ; valeur actuelle : %q.", d.label, s)
			}
		}
	}
}

// documentTypes checks BR-FR-04: document type codes belong to the French
// subset of UNTDID 1001.
func (k *ublChecker) documentTypes() {
	ref := "BillingReference/InvoiceDocumentReference/DocumentTypeCode"
	for _, d := range []struct{ path, id, label string }{
		{k.typeCode, "BR-FR-04_BT-3", "Le code type de document (cbc:" + k.typeCode + ")"},
		{ref, "BR-FR-04_EXT-FR-FE-02", "Le code type de document référencé (cbc:DocumentTypeCode)"},
		{k.line + "/" + ref, "BR-FR-04_EXT-FR-FE-137", "Le code type de document référencé en ligne (cbc:DocumentTypeCode)"},
	} {
		for n := range all(k.root, d.path) {
			if s := sv(n); !in(s, documentTypes...) {
				k.fail(d.id, n, "BR-FR-04 : %s n'est pas autorisé ; valeurs acceptées : 380, 389, 393, 501, 386, 500, 384, 471, 472, 473, 261, 262, 381, 396, 502, 503 ; valeur actuelle : %q.", d.label, s)
			}
		}
	}
}

// noteCodes checks BR-FR-05 (notes PMT, PMD and AAB present) and BR-FR-06
// (subject codes PMT, PMD, AAB and TXD at most once). In UBL the subject
// code is written between # at the start of the note (cbc:Note "#PMT#…"),
// and the notes are joined with # before the search.
func (k *ublChecker) noteCodes() {
	for i, n := range [...]struct{ code, label string }{
		{"PMT", "aux frais de recouvrement"},
		{"PMD", "aux pénalités de retard"},
		{"AAB", "à l'escompte ou à son absence"},
	} {
		if !strings.Contains(k.allNotes, "#"+n.code+"#") {
			k.fail("BR-FR-05_BT-22-"+string(rune('1'+i)), nil, "BR-FR-05/BT-22 : la mention relative %s (code %s) est absente ; elle est obligatoire dans les notes (BG-3, cbc:Note commençant par #%s#).", n.label, n.code, n.code)
		}
	}
	for i, n := range [...]struct{ code, label string }{
		{"PMT", "indemnité forfaitaire pour frais de recouvrement"},
		{"PMD", "pénalités de retard"},
		{"AAB", "mention d'escompte ou d'absence d'escompte"},
		{"TXD", "mention de taxe"},
	} {
		if c := strings.Count(k.allNotes, "#"+n.code+"#"); c > 1 {
			k.fail("BR-FR-06_BT-21-"+string(rune('1'+i)), nil, "BR-FR-06/BT-21 : le code sujet %s (%s) ne doit apparaître qu'une seule fois dans les notes (BG-3) ; occurrences : %d.", n.code, n.label, c)
		}
	}
}

// billingMode checks BR-FR-08: BT-23 (cbc:ProfileID) is present and is a
// French billing frame.
func (k *ublChecker) billingMode() {
	p := first(k.root, "ProfileID")
	if p == nil || !in(sv(p), billingModes...) {
		k.fail("BR-FR-08_BT-23", nil, "BR-FR-08/BT-23 : le cadre de facturation (cbc:ProfileID) est absent ou n'est pas autorisé ; valeurs acceptées : B1, S1, M1, B2, S2, M2, S3, B4, S4, M4, S5, S6, B7, S7, B8, S8, M8, B9, S9, M9 ; valeur actuelle : %q.", sv(p))
	}
}

// siretSiren checks BR-FR-09: the first SIRET (identifier of scheme 0009) of
// a party has 14 digits and starts with its SIREN (legal identifier 0002).
// For the seller and the buyer the SIREN is mandatory when a SIRET is given;
// for the other parties the first 9 digits of the SIRET stand for a missing
// SIREN.
func (k *ublChecker) siretSiren() {
	const pid, legal = "PartyIdentification/ID", "PartyLegalEntity/CompanyID"
	for _, p := range []struct {
		path, siret, siren, id string
		strict                 bool
	}{
		{"AccountingSupplierParty/Party", pid, legal, "BR-FR-09_BT-29", true},
		{"AccountingCustomerParty/Party", pid, legal, "BR-FR-09_BT-46", true},
		{"PayeeParty", pid, legal, "BR-FR-09_BT-60", false},
		{"AccountingCustomerParty/Party/AgentParty", pid, legal, "BR-FR-09_EXT-FR-FE-06", false},
		{"PaymentMeans/PaymentMandate/PayerParty", pid, legal, "BR-FR-09_EXT-FR-FE-46", false},
		{"AccountingSupplierParty/Party/AgentParty", pid, legal, "BR-FR-09_EXT-FR-FE-69", false},
		{"AccountingCustomerParty/Party/ServiceProviderParty/Party", pid, legal, "BR-FR-09_EXT-FR-FE-92", false},
		{"AccountingSupplierParty/Party/ServiceProviderParty/Party", pid, legal, "BR-FR-09_EXT-FR-FE-115", false},
		{"Delivery", "DeliveryLocation/ID", "DeliveryParty/" + legal, "BR-FR-09_BT-71", false},
		// As written in the schematron, the SIRET is looked for below the
		// line delivery location itself (cac:DeliveryLocation/cbc:ID relative
		// to cac:DeliveryLocation): the assertion only fires on documents
		// that are not valid against the UBL schema.
		{k.line + "/Delivery/DeliveryLocation", "DeliveryLocation/ID", "DeliveryParty/" + legal, "BR-FR-09_EXT-FR-FE-146", false},
	} {
		for party := range all(k.root, p.path) {
			sirets := withAttr(party, p.siret, "schemeID", "0009")
			if len(sirets) == 0 {
				continue
			}
			siret := sv(sirets[0])
			var sirenNode *xmltree.Node
			if s := withAttr(party, p.siren, "schemeID", "0002"); len(s) > 0 {
				sirenNode = s[0]
			}
			siren, hasSiren := sv(sirenNode), sirenNode != nil
			if !p.strict && siren == "" {
				siren, hasSiren = substr(siret, 9), true
			}
			if !siretMatchesSiren(siret, siren, hasSiren) {
				k.fail(p.id, party, "BR-FR-09/%s : le SIRET (identifiant de schéma 0009) doit contenir 14 chiffres et commencer par le SIREN (identifiant légal de schéma 0002) ; SIRET : %q, SIREN : %q.", strings.TrimPrefix(p.id, "BR-FR-09_"), siret, siren)
			}
		}
	}
}

// sellerSiren checks BR-FR-10 on every legal entity of the seller: it has a
// SIREN of 9 digits. Without cac:PartyLegalEntity the rule does not fire.
func (k *ublChecker) sellerSiren() {
	for le := range anywhere(k.root, "AccountingSupplierParty/Party/PartyLegalEntity") {
		s := withAttr(le, "CompanyID", "schemeID", "0002")
		if len(s) == 0 || !digits(nsv(s[0]), 9) {
			var v *xmltree.Node
			if len(s) > 0 {
				v = s[0]
			}
			k.fail("BR-FR-10_BT-30", le, "BR-FR-10/BT-30 : le SIREN du vendeur (cbc:CompanyID de schéma 0002) est obligatoire et doit être composé exactement de 9 chiffres ; valeur actuelle : %q.", sv(v))
		}
	}
}

// buyerSiren checks BR-FR-11: with the treatment BAR B2B, the buyer has a
// SIREN of 9 digits.
func (k *ublChecker) buyerSiren() {
	bar := noteValue(k.allNotes, "BAR")
	if bar != "B2B" {
		return
	}
	s := withAttr(k.root, "AccountingCustomerParty/Party/PartyLegalEntity/CompanyID", "schemeID", "0002")
	if len(s) == 0 || !digits(nsv(s[0]), 9) {
		var v *xmltree.Node
		if len(s) > 0 {
			v = s[0]
		}
		k.fail("BR-FR-11_BT-47", nil, "BR-FR-11/BT-47 : avec une note de code sujet BAR de valeur B2B, le SIREN de l'acheteur (cbc:CompanyID de schéma 0002) est obligatoire et doit être composé exactement de 9 chiffres ; traitement BAR : %q, SIREN : %q.", bar, sv(v))
	}
}

// endpointPresent checks BR-FR-12 and BR-FR-13: the buyer and the seller
// have an electronic address (BT-49, BT-34).
func (k *ublChecker) endpointPresent() {
	if v := first(k.root, "AccountingCustomerParty/Party/EndpointID"); nsv(v) == "" {
		k.fail("BR-FR-12_BT-49", nil, "BR-FR-12/BT-49 : l'adresse électronique de l'acheteur (cbc:EndpointID, BT-49) est obligatoire ; valeur actuelle : %q.", sv(v))
	}
	if v := first(k.root, "AccountingSupplierParty/Party/EndpointID"); nsv(v) == "" {
		k.fail("BR-FR-13_BT-34", nil, "BR-FR-13/BT-34 : l'adresse électronique du vendeur (cbc:EndpointID, BT-34) est obligatoire ; valeur actuelle : %q.", sv(v))
	}
}

// vatCategories checks BR-FR-15 on the VAT categories of the document level
// allowances and charges (BT-95, BT-102), of the VAT breakdown (BT-118) and
// of the items (BT-151), at any depth: the category is one of S, E, AE, K,
// G, O, Z, and not L nor M. The schematron reports the item variant of the
// second test as BR-FR-08_BT-151-2.
func (k *ublChecker) vatCategories() {
	for _, c := range []struct{ ctx, id1, id2, bt string }{
		{"AllowanceCharge/TaxCategory", "BR-FR-15_BT-95_BT-102-1", "BR-FR-15_BT-95_BT-102-2", "BT-95/BT-102"},
		{"TaxTotal/TaxSubtotal/TaxCategory", "BR-FR-15_BT-118-1", "BR-FR-15_BT-118-2", "BT-118"},
		{"Item/ClassifiedTaxCategory", "BR-FR-15_BT-151-1", "BR-FR-08_BT-151-2", "BT-151"},
	} {
		for tc := range anywhere(k.root, c.ctx) {
			code := first(tc, "ID")
			if code != nil && !in(sv(code), vatCategories...) {
				k.fail(c.id1, tc, "BR-FR-15/%s : le code de catégorie de TVA (cbc:ID) est invalide ; codes autorisés : S, E, AE, K, G, O, Z ; valeur actuelle : %q.", c.bt, sv(code))
			}
			if anyIn(tc, "ID", "L", "M") {
				k.fail(c.id2, tc, "BR-FR-15/%s : les codes de catégorie de TVA L et M ne sont pas pertinents en France ; valeur actuelle : %q.", c.bt, sv(code))
			}
		}
	}
}

// vatRates checks BR-FR-16: the VAT rates (cbc:Percent) are French rates.
// The schematron has two rules on cac:AllowanceCharge/cac:TaxCategory in
// the same pattern, BT-96 then BT-103: only the first fires, so every
// allowance or charge rate is reported as BR-FR-16_BT-96. The item rule
// only applies to invoice lines.
func (k *ublChecker) vatRates() {
	for _, r := range []struct{ ctx, id, bt string }{
		{"AllowanceCharge/TaxCategory", "BR-FR-16_BT-96", "BT-96"},
		{"TaxTotal/TaxSubtotal/TaxCategory", "BR-FR-16_BT-119", "BT-119"},
		{"InvoiceLine/Item/ClassifiedTaxCategory", "BR-FR-16_BT-152", "BT-152"},
	} {
		for tc := range anywhere(k.root, r.ctx) {
			if rate := first(tc, "Percent"); rate != nil && !in(sv(rate), vatRates...) {
				k.fail(r.id, tc, "BR-FR-16/%s : le taux de TVA (cbc:Percent) doit être exprimé en pourcentage sans symbole %% et faire partie des valeurs autorisées ; taux fourni : %q.", r.bt, sv(rate))
			}
		}
	}
}

// attachments checks BR-FR-17 (the description of each additional document
// reference, at any depth, is a French attachment code) and BR-FR-18 (at
// most one LISIBLE attachment).
func (k *ublChecker) attachments() {
	for ref := range anywhere(k.root, "AdditionalDocumentReference") {
		if d := first(ref, "DocumentDescription"); d != nil && !in(sv(d), attachmentCodes...) {
			k.fail("BR-FR-17_BT-123", ref, "BR-FR-17/BT-123 : le code de qualification de la pièce jointe (cbc:DocumentDescription) %q est invalide ; il doit appartenir à la liste des codes autorisés.", sv(d))
		}
	}
	n := 0
	for ref := range all(k.root, "AdditionalDocumentReference") {
		if anyEq(ref, "DocumentDescription", "LISIBLE") {
			n++
		}
	}
	if n > 1 {
		k.fail("BR-FR-18_BT-123", nil, "BR-FR-18/BT-123 : il ne peut y avoir qu'un document additionnel (cac:AdditionalDocumentReference) de description (cbc:DocumentDescription) LISIBLE ; documents trouvés : %d.", n)
	}
}

// barTreatment checks BR-FR-20: the value of the note BAR is a known
// treatment.
func (k *ublChecker) barTreatment() {
	if bar := noteValue(k.allNotes, "BAR"); bar != "" && !in(bar, barTreatments...) {
		k.fail("BR-FR-20_BT-21", nil, "BR-FR-20/BT-21 : la valeur associée au code sujet BAR (cbc:Note #BAR#…) doit être l'une des suivantes : B2B, B2BINT, B2C, B2CINT, OUTOFSCOPE, ARCHIVEONLY ; valeur fournie : %q.", bar)
	}
}

// b2bEndpoints checks BR-FR-21 and BR-FR-22: with the treatment BAR B2B, the
// electronic address of the buyer (or of the seller for self-billed
// documents) starts with its SIREN and has the scheme 0225.
func (k *ublChecker) b2bEndpoints() {
	if noteValue(k.allNotes, "BAR") != "B2B" {
		return
	}
	selfBilled := anyIn(k.root, "InvoiceTypeCode", selfBilledTypes...) || anyIn(k.root, "CreditNoteTypeCode", selfBilledTypes...)
	ok := func(party string) (bool, string, string) {
		ep := first(k.root, party+"/EndpointID")
		siren := withAttr(k.root, party+"/PartyLegalEntity/CompanyID", "schemeID", "0002")
		prefix := ""
		if len(siren) > 0 {
			prefix = sv(siren[0])
		}
		scheme := false
		for e := range all(k.root, party+"/EndpointID") {
			if attrIs(e, "schemeID", "0225") {
				scheme = true
			}
		}
		return strings.HasPrefix(sv(ep), prefix) && scheme, sv(ep), prefix
	}
	if !selfBilled {
		if good, ep, siren := ok("AccountingCustomerParty/Party"); !good {
			k.fail("BR-FR-21_BT-49", nil, "BR-FR-21/BT-49 : avec le traitement BAR B2B et un type de document non auto-facturé, l'adresse électronique de l'acheteur (cbc:EndpointID, BT-49) doit commencer par son SIREN (cbc:CompanyID de schéma 0002) et avoir le schéma 0225 ; BT-49 : %q, SIREN : %q.", ep, siren)
		}
	} else if good, ep, siren := ok("AccountingSupplierParty/Party"); !good {
		k.fail("BR-FR-22_BT-34", nil, "BR-FR-22/BT-34 : avec le traitement BAR B2B et un type de document auto-facturé (389, 501, 500, 471, 473, 261, 502), l'adresse électronique du vendeur (cbc:EndpointID, BT-34) doit commencer par son SIREN (cbc:CompanyID de schéma 0002) et avoir le schéma 0225 ; BT-34 : %q, SIREN : %q.", ep, siren)
	}
}

// endpointParties are the parties whose electronic address BR-FR-23 and
// BR-FR-25 check, with the business term of the address.
var endpointParties = []struct{ path, bt string }{
	{"AccountingSupplierParty/Party", "BT-34"},
	{"AccountingCustomerParty/Party", "BT-49"},
	{"AccountingCustomerParty/Party/AgentParty", "EXT-FR-FE-12"},
	{"PayeeParty", "EXT-FR-FE-29"},
	{"PaymentMeans/PaymentMandate/PayerParty", "EXT-FR-FE-52"},
	{"AccountingSupplierParty/Party/AgentParty", "EXT-FR-FE-75"},
	{"AccountingCustomerParty/Party/ServiceProviderParty/Party", "EXT-FR-FE-98"},
	{"AccountingSupplierParty/Party/ServiceProviderParty/Party", "EXT-FR-FE-121"},
}

// endpointFormats checks BR-FR-23: the electronic addresses of scheme 0225
// are made of [A-Za-z0-9+\-_.].
func (k *ublChecker) endpointFormats() {
	for _, p := range endpointParties {
		for _, ep := range withAttr(k.root, p.path+"/EndpointID", "schemeID", "0225") {
			if s := sv(ep); !validSchemeID(s) {
				k.fail("BR-FR-23_"+p.bt, ep, "BR-FR-23/%s : l'adresse électronique (cbc:EndpointID) ne respecte pas le format autorisé ; seuls les caractères alphanumériques et les symboles + - _ . sont autorisés ; valeur actuelle : %q.", p.bt, s)
			}
		}
	}
}

// privateIDFormat checks BR-FR-24: the private identifiers (scheme 0224) of
// the seller and the buyer are made of [A-Za-z0-9+\-_.].
func (k *ublChecker) privateIDFormat() {
	for _, p := range [...]struct{ path, bt string }{
		{"AccountingSupplierParty/Party/PartyIdentification/ID", "BT-29"},
		{"AccountingCustomerParty/Party/PartyIdentification/ID", "BT-46"},
	} {
		for _, id := range withAttr(k.root, p.path, "schemeID", "0224") {
			if s := sv(id); !validSchemeID(s) {
				k.fail("BR-FR-24_"+p.bt, id, "BR-FR-24/%s : l'identifiant privé (cbc:ID de schéma 0224) ne respecte pas le format autorisé ; seuls les caractères alphanumériques et les symboles + - _ . sont autorisés ; valeur actuelle : %q.", p.bt, s)
			}
		}
	}
}

// endpointLengths checks BR-FR-25: the electronic addresses have at most 125
// characters, whatever their scheme.
func (k *ublChecker) endpointLengths() {
	for _, p := range endpointParties {
		for ep := range all(k.root, p.path+"/EndpointID") {
			if s := sv(ep); length(s) > 125 {
				k.fail("BR-FR-25_"+p.bt, ep, "BR-FR-25/%s : l'adresse électronique (cbc:EndpointID) dépasse 125 caractères ; valeur actuelle : %q (%d caractères).", p.bt, s, length(s))
			}
		}
	}
}

// privateIDLength checks BR-FR-26: the private identifiers (scheme 0224) of
// the seller and the buyer have at most 100 characters.
func (k *ublChecker) privateIDLength() {
	for _, p := range [...]struct{ path, bt string }{
		{"AccountingSupplierParty/Party/PartyIdentification/ID", "BT-29"},
		{"AccountingCustomerParty/Party/PartyIdentification/ID", "BT-46"},
	} {
		for _, id := range withAttr(k.root, p.path, "schemeID", "0224") {
			if s := sv(id); length(s) > 100 {
				k.fail("BR-FR-26_"+p.bt, id, "BR-FR-26/%s : l'identifiant privé (cbc:ID de schéma 0224) dépasse 100 caractères ; valeur actuelle : %q (%d caractères).", p.bt, s, length(s))
			}
		}
	}
}

// characteristics checks BR-FR-27 and BR-FR-28 on the item attributes
// (BG-32). As written in the schematron, BR-FR-27 targets cac:Item directly
// under the root, which the UBL schema does not define: it only fires on
// schema-invalid documents. BR-FR-28 checks the attributes of the line
// items: a value (cbc:Value) or a quantity with its unit
// (cbc:ValueQuantity/@unitCode), not both.
func (k *ublChecker) characteristics() {
	for prop := range all(k.root, "Item/AdditionalItemProperty") {
		if !exists(prop, "Name") && !exists(prop, "NameCode") {
			k.fail("BR-FR-27_BG-32", prop, "BR-FR-27/BG-32 : le groupe attribut d'article (BG-32) doit contenir un nom d'attribut (BT-160, cbc:Name) ou un code d'attribut (EXT-FR-FE-159, cbc:NameCode).")
		}
	}
	for n := range all(k.root, "Item/AdditionalItemProperty/Name") {
		if nsv(n) == "" {
			k.fail("BR-FR-27_BT-160", n, "BR-FR-27/BT-160 : le nom d'attribut d'article (cbc:Name) ne doit pas être vide ; valeur actuelle : %q.", sv(n))
		}
	}
	for n := range all(k.root, "Item/AdditionalItemProperty/NameCode") {
		if nsv(n) == "" {
			k.fail("BR-FR-27_EXT-FR-FE-159", n, "BR-FR-27/EXT-FR-FE-159 : le code d'attribut d'article (cbc:NameCode) ne doit pas être vide ; valeur actuelle : %q.", sv(n))
		}
	}
	for prop := range all(k.root, k.line+"/Item/AdditionalItemProperty") {
		value, qty := exists(prop, "Value"), exists(prop, "ValueQuantity")
		unit := false
		for q := range all(prop, "ValueQuantity") {
			if u, ok := q.LookupAttr("unitCode"); ok && u != "" {
				unit = true
			}
		}
		if !(value && !qty || !value && unit) {
			q := first(prop, "ValueQuantity")
			k.fail("BR-FR-28-Value", prop, "BR-FR-28 : l'attribut d'article doit avoir une valeur (cbc:Value) ou une quantité avec son unité de mesure (cbc:ValueQuantity/@unitCode), mais pas les deux ; valeur : %q, quantité : %q, unité : %q.", sv(first(prop, "Value")), sv(q), q.Attr("unitCode"))
		}
	}
}

// objects checks BR-FR-29 and BR-FR-30 on the invoiced objects of scheme
// AFL and AVV: at most one of each per document reference, and not empty.
// BR-FR-29 checks the additional document references (BT-18), BR-FR-30 the
// line document references (BT-128). A reference has a single cbc:ID in the
// UBL schema, so BR-FR-29_BT-18 and BR-FR-30_BT-128 only fire on
// schema-invalid documents.
func (k *ublChecker) objects() {
	for _, o := range [...]struct{ path, base, bt, label string }{
		{"AdditionalDocumentReference", "BR-FR-29", "BT-18", "d'objets facturés (BT-18)"},
		{k.line + "/DocumentReference", "BR-FR-30", "BT-128", "d'objets facturés à la ligne (BT-128)"},
	} {
		for ref := range all(k.root, o.path) {
			afl, avv := len(withAttr(ref, "ID", "schemeID", "AFL")), len(withAttr(ref, "ID", "schemeID", "AVV"))
			if afl > 1 || avv > 1 {
				k.fail(o.base+"_"+o.bt, ref, "%s/%s : parmi les identifiants %s, les schémas AFL et AVV ne doivent être présents qu'une seule fois chacun ; AFL : %d occurrence(s), AVV : %d occurrence(s).", o.base, o.bt, o.label, afl, avv)
			}
		}
		for _, scheme := range [...]string{"AFL", "AVV"} {
			for ref := range all(k.root, o.path) {
				for _, id := range withAttr(ref, "ID", "schemeID", scheme) {
					if nsv(id) == "" {
						k.fail(o.base+"_"+scheme, id, "%s/%s : l'identifiant de schéma %s (cbc:ID) ne doit pas être vide ; valeur actuelle : %q.", o.base, scheme, scheme, sv(id))
					}
				}
			}
		}
	}
}

// barUnique checks the pattern BR-FR-31, whose assertion is identified as
// BR-FR-30_BT-21 in the UBL schematron: among the notes that contain #BAR#,
// at most one treatment is given.
func (k *ublChecker) barUnique() {
	var b strings.Builder
	for n := range all(k.root, "Note") {
		if s := sv(n); strings.Contains(s, "#BAR#") {
			b.WriteString(s)
		}
	}
	b.WriteByte('#')
	joined := b.String()
	n := 0
	for _, t := range barTreatments {
		n += strings.Count(joined, "BAR#"+t+"#")
	}
	if n > 1 {
		k.fail("BR-FR-30_BT-21", nil, "BR-FR-31/BT-21 : lorsque plusieurs notes ont le code sujet BAR (BT-21), une seule valeur de traitement (B2B, B2BINT, B2C, B2CINT, OUTOFSCOPE, ARCHIVEONLY) peut être donnée ; valeurs trouvées : %d.", n)
	}
}

// legalIDs checks BR-FR-32: every SIREN of a party (legal identifier 0002,
// identifier 0002 or 0231) has 9 digits.
func (k *ublChecker) legalIDs() {
	for id := range anywhere(k.root, "PartyLegalEntity/CompanyID") {
		if attrIs(id, "schemeID", "0002") && !digits(nsv(id), 9) {
			k.fail("BR-FR-32-LEGALID", id, "BR-FR-32/LEGALID : tout identifiant légal d'une partie (cbc:CompanyID) de schéma 0002 doit être composé de 9 chiffres ; valeur actuelle : %q.", sv(id))
		}
	}
	for id := range anywhere(k.root, "PartyIdentification/ID") {
		if (attrIs(id, "schemeID", "0002") || attrIs(id, "schemeID", "0231")) && !digits(nsv(id), 9) {
			k.fail("BR-FR-32-ID", id, "BR-FR-32/ID : tout identifiant d'une partie (cbc:ID) de schéma 0002 ou 0231 doit être composé de 9 chiffres ; valeur actuelle : %q.", sv(id))
		}
	}
}
