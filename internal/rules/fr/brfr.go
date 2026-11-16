package fr

import (
	"strings"

	"github.com/ritoon/factur-x/internal/xmltree"
)

// Code lists of the schematron functions.
var (
	// custom:is-valid-document-type-code (BR-FR-04).
	documentTypes = []string{"380", "389", "393", "501", "386", "500", "384", "471", "472", "473", "261", "262", "381", "396", "502", "503"}
	// custom:is-valid-billing-mode (BR-FR-08), v1.4 with frame 9.
	billingModes = []string{"B1", "S1", "M1", "B2", "S2", "M2", "S3", "B4", "S4", "M4", "S5", "S6", "B7", "S7", "B8", "S8", "M8", "B9", "S9", "M9"}
	// custom:is-valid-bar-treatment (BR-FR-20).
	barTreatments = []string{"B2B", "B2BINT", "B2C", "B2CINT", "OUTOFSCOPE", "ARCHIVEONLY"}
	// custom:is-valid-vat-category-code (BR-FR-15).
	vatCategories = []string{"S", "E", "AE", "K", "G", "O", "Z"}
	// custom:is-valid-vat-rate (BR-FR-16): lexical forms, compared as strings.
	vatRates = []string{
		"0", "0.0", "0.00", "10", "10.0", "10.00", "13", "13.0", "13.00", "20", "20.0", "20.00",
		"8.5", "8.50", "19.6", "19.60", "2.1", "2.10", "5.5", "5.50", "7", "7.0", "7.00",
		"20.6", "20.60", "1.05", "0.9", "0.90", "1.75", "9.2", "9.20", "9.6", "9.60",
	}
	// custom:is-valid-attachment-code (BR-FR-17), CYS 1.3.1.
	attachmentCodes = []string{
		"RIB", "LISIBLE", "FEUILLE_DE_STYLE", "PJA", "BORDEREAU_SUIVI",
		"DOCUMENT_ANNEXE", "BON_LIVRAISON", "BON_COMMANDE",
		"BORDEREAU_SUIVI_VALIDATION", "ETAT_ACOMPTE", "FACTURE_PAIEMENT_DIRECT", "RECAPITULATIF_COTRAITANCE",
	}
	// Self-billed document types excluded from BR-FR-21 and required by BR-FR-22.
	selfBilledTypes = []string{"389", "501", "500", "471", "473", "261", "502"}
)

// validID implements custom:is-valid-id-format (BR-FR-02): the normalized
// value is made of [A-Za-z0-9+\-_/] only and the raw value has no space.
func validID(s string) bool {
	n := norm(s)
	if n == "" || strings.IndexByte(s, ' ') >= 0 {
		return false
	}
	for i := 0; i < len(n); i++ {
		c := n[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '_' || c == '/') {
			return false
		}
	}
	return true
}

// validDate implements custom:is-valid-date-format (BR-FR-03): the first
// eight characters are a real date YYYYMMDD of the years 2000 to 2099.
func validDate(s string) bool {
	d := substr(s, 8)
	if len(d) != 8 {
		return false
	}
	for i := 0; i < 8; i++ {
		if d[i] < '0' || d[i] > '9' {
			return false
		}
	}
	if d[0] != '2' || d[1] != '0' {
		return false
	}
	year := int(d[2]-'0')*10 + int(d[3]-'0') + 2000
	month := int(d[4]-'0')*10 + int(d[5]-'0')
	day := int(d[6]-'0')*10 + int(d[7]-'0')
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return false
	}
	maxDay := 31
	switch month {
	case 4, 6, 9, 11:
		maxDay = 30
	case 2:
		maxDay = 28
		if year%4 == 0 && year%100 != 0 || year%400 == 0 {
			maxDay = 29
		}
	}
	return day <= maxDay
}

// validSchemeID implements custom:is-valid-schemeid-format (BR-FR-23,
// BR-FR-24): matches(value, '^[A-Za-z0-9+\-_.]+$') on the raw value.
func validSchemeID(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// siretMatchesSiren implements custom:check-siret-siren-coherence (BR-FR-09):
// the SIRET has 14 digits and starts with the SIREN. hasSiren is false when
// the SIREN is the empty sequence.
func siretMatchesSiren(siret, siren string, hasSiren bool) bool {
	return digits(siret, 14) && hasSiren && substr(siret, 9) == siren
}

// identifiers checks BR-FR-01 (length) and BR-FR-02 (format) on BT-1, BT-25
// and EXT-FR-FE-136.
func (k *checker) identifiers() {
	check := func(n *xmltree.Node, bt, label string) {
		s := sv(n)
		if l := length(s); l > 35 {
			k.fail("BR-FR-01_"+bt, n, "BR-FR-01/%s : %s ne doit pas dépasser 35 caractères ; valeur actuelle : %q (%d caractères).", bt, label, s, l)
		}
		if !validID(s) {
			k.fail("BR-FR-02_"+bt, n, "BR-FR-02/%s : %s contient des caractères non autorisés ; seuls les caractères alphanumériques et les symboles + - _ / sont autorisés, sans espaces ; valeur actuelle : %q.", bt, label, s)
		}
	}
	for n := range all(k.root, "ExchangedDocument/ID") {
		check(n, "BT-1", "L'identifiant de facture (ram:ID)")
	}
	for n := range all(k.root, pSettlement+"/InvoiceReferencedDocument/IssuerAssignedID") {
		check(n, "BT-25", "L'identifiant de facture référencée (ram:IssuerAssignedID)")
	}
	for n := range all(k.root, pLines+"/SpecifiedLineTradeSettlement/InvoiceReferencedDocument/IssuerAssignedID") {
		check(n, "EXT-FR-FE-136", "L'identifiant de facture référencée en ligne (ram:IssuerAssignedID)")
	}
}

// dates checks BR-FR-03: dates are valid YYYYMMDD dates of years 2000–2099.
func (k *checker) dates() {
	for _, d := range []struct{ path, id, label string }{
		{pIssueDate, "BR-FR-03_BT-2", "La date d'émission (BT-2)"},
		{pHeaderTax + "/TaxPointDate/DateString", "BR-FR-03_BT-7", "La date de fait générateur de la taxe (BT-7)"},
		{pDueDates, "BR-FR-03_BT-9", "La date d'échéance (BT-9)"},
		{pSettlement + "/InvoiceReferencedDocument/FormattedIssueDateTime/DateTimeString", "BR-FR-03_BT-26", "La date d'émission de la facture référencée (BT-26)"},
		{"SupplyChainTradeTransaction/ApplicableHeaderTradeDelivery/ActualDeliverySupplyChainEvent/OccurrenceDateTime/DateTimeString", "BR-FR-03_BT-72", "La date de livraison effective (BT-72)"},
		{pSettlement + "/BillingSpecifiedPeriod/StartDateTime/DateTimeString", "BR-FR-03_BT-73", "La date de début de période de facturation (BT-73)"},
		{pSettlement + "/BillingSpecifiedPeriod/EndDateTime/DateTimeString", "BR-FR-03_BT-74", "La date de fin de période de facturation (BT-74)"},
		{pLines + "/SpecifiedLineTradeSettlement/InvoiceReferencedDocument/FormattedIssueDateTime/DateTimeString", "BR-FR-03_EXT-FR-FE-138", "La date d'émission de la facture référencée en ligne (EXT-FR-FE-138)"},
		{pLines + "/SpecifiedLineTradeDelivery/ActualDeliverySupplyChainEvent/OccurrenceDateTime/DateTimeString", "BR-FR-03_EXT-FR-FE-158", "La date de livraison effective en ligne (EXT-FR-FE-158)"},
		{pLines + "/SpecifiedLineTradeSettlement/BillingSpecifiedPeriod/StartDateTime/DateTimeString", "BR-FR-03_EXT-FR-FE-134", "La date de début de période de facturation en ligne (BT-134)"},
		{pLines + "/SpecifiedLineTradeSettlement/BillingSpecifiedPeriod/EndDateTime/DateTimeString", "BR-FR-03_EXT-FR-FE-135", "La date de fin de période de facturation en ligne (BT-135)"},
	} {
		for n := range all(k.root, d.path) {
			if s := sv(n); !validDate(s) {
				k.fail(d.id, n, "BR-FR-03 : %s doit être une date valide au format AAAAMMJJ avec une année comprise entre 2000 et 2099 ; valeur actuelle : %q.", d.label, s)
			}
		}
	}
}

// documentTypes checks BR-FR-04: document type codes belong to the French
// subset of UNTDID 1001.
func (k *checker) documentTypes() {
	for _, d := range []struct{ path, id, label string }{
		{pTypeCode, "BR-FR-04_BT-3", "Le code type de document (BT-3)"},
		{pSettlement + "/InvoiceReferencedDocument/TypeCode", "BR-FR-04_EXT-FR-FE-02", "Le code type de document référencé (EXT-FR-FE-02)"},
		{pLines + "/SpecifiedLineTradeSettlement/InvoiceReferencedDocument/TypeCode", "BR-FR-04_EXT-FR-FE-137", "Le code type de document référencé en ligne (EXT-FR-FE-137)"},
	} {
		for n := range all(k.root, d.path) {
			if s := sv(n); !in(s, documentTypes...) {
				k.fail(d.id, n, "BR-FR-04 : %s n'est pas autorisé ; valeurs acceptées : 380, 389, 393, 501, 386, 500, 384, 471, 472, 473, 261, 262, 381, 396, 502, 503 ; valeur actuelle : %q.", d.label, s)
			}
		}
	}
}

// notesWith counts the notes of the document header whose subject code is
// code.
func notesWith(header *xmltree.Node, code string) int {
	n := 0
	for note := range all(header, "IncludedNote") {
		if anyEq(note, "SubjectCode", code) {
			n++
		}
	}
	return n
}

// mandatoryNotes checks BR-FR-05: notes PMT, PMD and AAB are present.
func (k *checker) mandatoryNotes() {
	for h := range all(k.root, pHeader) {
		if notesWith(h, "PMT") == 0 {
			k.fail("BR-FR-05_BT-22_PMT", h, "BR-FR-05/BT-22 : la mention relative à l'indemnité forfaitaire pour frais de recouvrement (note de code sujet PMT) est absente ; elle est obligatoire dans les notes (BG-1).")
		}
		if notesWith(h, "PMD") == 0 {
			k.fail("BR-FR-05_BT-22_PMD", h, "BR-FR-05/BT-22 : la mention relative aux pénalités de retard (note de code sujet PMD) est absente ; elle est obligatoire dans les notes (BG-1).")
		}
		if notesWith(h, "AAB") == 0 {
			k.fail("BR-FR-05_BT-22_AAB", h, "BR-FR-05/BT-22 : la mention relative à l'escompte ou à son absence (note de code sujet AAB) est absente ; elle est obligatoire dans les notes (BG-1).")
		}
	}
}

// uniqueNotes checks BR-FR-06: subject codes PMT, PMD, AAB and TXD appear at
// most once.
func (k *checker) uniqueNotes() {
	for h := range all(k.root, pHeader) {
		for _, code := range [...]struct{ code, label string }{
			{"PMT", "indemnité forfaitaire pour frais de recouvrement"},
			{"PMD", "pénalités de retard"},
			{"AAB", "mention d'escompte ou d'absence d'escompte"},
			{"TXD", "mention de taxe"},
		} {
			if n := notesWith(h, code.code); n > 1 {
				k.fail("BR-FR-06_BT-21_"+code.code, h, "BR-FR-06/BT-21 : le code sujet %s (%s) ne doit apparaître qu'une seule fois dans les notes (BG-1) ; occurrences : %d.", code.code, code.label, n)
			}
		}
	}
}

// billingMode checks BR-FR-08: BT-23 is present and is a French billing
// frame.
func (k *checker) billingMode() {
	for ctx := range all(k.root, pContext) {
		id := first(ctx, "BusinessProcessSpecifiedDocumentContextParameter/ID")
		if id == nil || !in(sv(id), billingModes...) {
			at := id
			if at == nil {
				at = ctx
			}
			k.fail("BR-FR-08_BT-23", at, "BR-FR-08/BT-23 : le cadre de facturation (ram:ID) est absent ou n'est pas autorisé ; valeurs acceptées : B1, S1, M1, B2, S2, M2, S3, B4, S4, M4, S5, S6, B7, S7, B8, S8, M8, B9, S9, M9 ; valeur actuelle : %q.", sv(id))
		}
	}
}

// siretSiren checks BR-FR-09: the first SIRET (GlobalID 0009) of a party has
// 14 digits and starts with its SIREN (legal identifier 0002). For the
// seller and the buyer the SIREN is mandatory when a SIRET is given; for the
// other parties the first 9 digits of the SIRET stand for a missing SIREN.
func (k *checker) siretSiren() {
	for _, p := range []struct {
		path, id string
		strict   bool
	}{
		{pSeller, "BR-FR-09_BT-29", true},
		{pBuyer, "BR-FR-09_BT-46", true},
		{pPayee, "BR-FR-09_BT-60", false},
		{pBuyerAgent, "BR-FR-09_EXT-FR-FE-06", false},
		{pPayer, "BR-FR-09_EXT-FR-FE-46", false},
		{pSalesAgent, "BR-FR-09_EXT-FR-FE-69", false},
		{pInvoicee, "BR-FR-09_EXT-FR-FE-92", false},
		{pInvoicer, "BR-FR-09_EXT-FR-FE-115", false},
		{pShipTo, "BR-FR-09_BT-71", false},
		{pLines + "/SpecifiedLineTradeDelivery/ShipToTradeParty", "BR-FR-09_EXT-FR-FE-146", false},
	} {
		for party := range all(k.root, p.path) {
			siretNode := firstWith(party, "GlobalID", "schemeID", "0009")
			if siretNode == nil {
				continue
			}
			siret := sv(siretNode)
			sirenNode := firstWith(party, pLegalID, "schemeID", "0002")
			siren, has := sv(sirenNode), sirenNode != nil
			if !p.strict && siren == "" {
				siren, has = substr(siret, 9), true
			}
			if !siretMatchesSiren(siret, siren, has) {
				k.fail(p.id, siretNode, "BR-FR-09/%s : le SIRET doit contenir 14 chiffres et commencer par le SIREN ; SIRET : %q, SIREN : %q.", p.id[len("BR-FR-09_"):], siret, sv(sirenNode))
			}
		}
	}
}

// sellerSiren checks BR-FR-10: the seller SIREN (BT-30, scheme 0002) is
// present and made of 9 digits.
func (k *checker) sellerSiren() {
	n := firstWith(k.root, pSeller+"/"+pLegalID, "schemeID", "0002")
	if n == nil || !digits(nsv(n), 9) {
		at := n
		if at == nil {
			at = first(k.root, pSeller)
		}
		k.fail("BR-FR-10_BT-30", at, "BR-FR-10/BT-30 : le SIREN du vendeur (ram:ID avec schemeID 0002) est obligatoire et doit être composé exactement de 9 chiffres ; valeur actuelle : %q.", sv(n))
	}
}

// barB2B reports whether a note with subject code BAR has the content B2B
// ($barTreatment = 'B2B').
func (k *checker) barB2B() bool {
	for note := range all(k.root, pNotes) {
		if anyEq(note, "SubjectCode", "BAR") && anyEq(note, "Content", "B2B") {
			return true
		}
	}
	return false
}

// buyerSiren checks BR-FR-11: with the BAR treatment B2B the buyer SIREN
// (BT-47, scheme 0002) is made of 9 digits.
func (k *checker) buyerSiren() {
	if !k.barB2B() {
		return
	}
	n := firstWith(k.root, pBuyer+"/"+pLegalID, "schemeID", "0002")
	if !digits(sv(n), 9) {
		at := n
		if at == nil {
			at = first(k.root, pBuyer)
		}
		k.fail("BR-FR-11_BT-47", at, "BR-FR-11/BT-47 : une note de code sujet BAR a la valeur B2B ; le SIREN de l'acheteur (BT-47, ram:ID avec schemeID 0002) est alors obligatoire et doit être composé exactement de 9 chiffres ; valeur actuelle : %q.", sv(n))
	}
}

// endpointPresent checks BR-FR-12 (BT-49) and BR-FR-13 (BT-34): the
// electronic addresses of the buyer and the seller are present.
func (k *checker) endpointPresent() {
	if n := first(k.root, pBuyer+"/"+pURIID); sv(n) == "" {
		k.fail("BR-FR-12_BT-49", orNode(n, first(k.root, pBuyer)), "BR-FR-12/BT-49 : l'adresse électronique de l'acheteur (BT-49) est obligatoire ; valeur actuelle : %q.", sv(n))
	}
	if n := first(k.root, pSeller+"/"+pURIID); sv(n) == "" {
		k.fail("BR-FR-13_BT-34", orNode(n, first(k.root, pSeller)), "BR-FR-13/BT-34 : l'adresse électronique du vendeur (BT-34) est obligatoire ; valeur actuelle : %q.", sv(n))
	}
}

// orNode returns n, or alt when n is nil.
func orNode(n, alt *xmltree.Node) *xmltree.Node {
	if n != nil {
		return n
	}
	return alt
}

// vatCategories checks BR-FR-15: VAT category codes are S, E, AE, K, G, O or
// Z, never L or M.
func (k *checker) vatCategories() {
	check := func(tax *xmltree.Node, id, label string, groupOK bool) {
		code := first(tax, "CategoryCode")
		s := sv(code)
		ok := code != nil && in(s, vatCategories...)
		if !ok && code == nil && groupOK {
			line := tax.Parent.Parent
			ok = anyIn(line, pLineStatus, "GROUP", "INFORMATION")
		}
		if !ok {
			k.fail(id, orNode(code, tax), "BR-FR-15/%s : code de catégorie de TVA invalide ; codes valides : S, E, AE, K, G, O, Z ; valeur actuelle : %q.", label, s)
		}
		if anyIn(tax, "CategoryCode", "L", "M") {
			k.fail(id+"_LM", orNode(code, tax), "BR-FR-15/%s : les codes L et M ne sont pas pertinents en France ; valeur actuelle : %q.", label, s)
		}
	}
	for tax := range all(k.root, pHeaderAC+"/CategoryTradeTax") {
		check(tax, "BR-FR-15_BT-95_BT-102", "BT-95 ou BT-102", false)
	}
	for tax := range all(k.root, pHeaderTax) {
		check(tax, "BR-FR-15_BT-95_BT-118", "BT-118", false)
	}
	for tax := range all(k.root, pLines+"/SpecifiedLineTradeSettlement/ApplicableTradeTax") {
		check(tax, "BR-FR-15_BT-95_BT-151", "BT-151", true)
	}
}

// vatRates checks BR-FR-16: VAT rates, as written, belong to the French
// rates.
func (k *checker) vatRates() {
	for _, p := range []struct{ path, id, label string }{
		{pHeaderAC + "/CategoryTradeTax", "BR-FR-16_BT-96_BT-103", "BT-96 ou BT-103"},
		{pHeaderTax, "BR-FR-16_BT-119", "BT-119"},
		{pLines + "/SpecifiedLineTradeSettlement/ApplicableTradeTax", "BR-FR-16_BT-152", "BT-152"},
	} {
		for tax := range all(k.root, p.path) {
			rate := first(tax, "RateApplicablePercent")
			if s := sv(rate); s != "" && !in(s, vatRates...) {
				k.fail(p.id, rate, "BR-FR-16/%s : le taux de TVA doit être exprimé en pourcentage sans symbole %% et faire partie des valeurs autorisées ; taux fourni : %q.", p.label, s)
			}
		}
	}
}

// attachments checks BR-FR-17 (attachment codes of documents 916) and
// BR-FR-18 (at most one LISIBLE document).
func (k *checker) attachments() {
	for doc := range all(k.root, pHeaderARD) {
		if !anyEq(doc, "TypeCode", "916") {
			continue
		}
		name := first(doc, "Name")
		if code := nsv(name); code != "" && !in(code, attachmentCodes...) {
			k.fail("BR-FR-17_BT-123", name, "BR-FR-17/BT-123 : le code de qualification de la pièce jointe %q est invalide ; il doit appartenir à la liste des codes autorisés (RIB, LISIBLE, FEUILLE_DE_STYLE, PJA, BORDEREAU_SUIVI, DOCUMENT_ANNEXE, BON_LIVRAISON, BON_COMMANDE, BORDEREAU_SUIVI_VALIDATION, ETAT_ACOMPTE, FACTURE_PAIEMENT_DIRECT, RECAPITULATIF_COTRAITANCE).", code)
		}
	}
	n := 0
	for doc := range all(k.root, pHeaderARD) {
		if anyEq(doc, "Name", "LISIBLE") {
			n++
		}
	}
	if n > 1 {
		k.fail("BR-FR-18_BT-123", first(k.root, pAgreement), "BR-FR-18/BT-123 : il ne peut y avoir qu'un seul document additionnel (BG-24) dont la description (BT-123) est LISIBLE ; documents trouvés : %d.", n)
	}
}

// barTreatment checks BR-FR-20: a note with subject code BAR holds one of
// the BAR treatments.
func (k *checker) barTreatment() {
	for note := range all(k.root, pNotes) {
		if !anyEq(note, "SubjectCode", "BAR") {
			continue
		}
		content := first(note, "Content")
		if s := sv(content); content == nil || !in(s, barTreatments...) {
			k.fail("BR-FR-20_BT-21", orNode(content, note), "BR-FR-20/BT-21 : une note de code sujet BAR doit avoir pour contenu (BT-22) l'une des valeurs B2B, B2BINT, B2C, B2CINT, OUTOFSCOPE, ARCHIVEONLY ; valeur fournie : %q.", s)
		}
	}
}

// b2bEndpoints checks BR-FR-21 and BR-FR-22: with the BAR treatment B2B, the
// electronic address of the buyer (or of the seller for self-billed
// documents) starts with its SIREN and uses the scheme 0225.
func (k *checker) b2bEndpoints() {
	if !k.barB2B() {
		return
	}
	selfBilled := anyIn(k.root, pTypeCode, selfBilledTypes...)
	check := func(party, id, bt, sirenBT string) {
		siren := sv(first(k.root, party+"/"+pLegalID))
		uri := first(k.root, party+"/"+pURIID)
		endpoint := sv(uri)
		scheme, _ := uri.LookupAttr("schemeID")
		scheme0225 := false // $endpointSchemeID = '0225', over every address
		for u := range all(k.root, party+"/"+pURIID) {
			if attrIs(u, "schemeID", "0225") {
				scheme0225 = true
			}
		}
		if !strings.HasPrefix(endpoint, siren) || !scheme0225 {
			k.fail(id, orNode(uri, first(k.root, party)), "%s/%s : avec le traitement BAR B2B, l'adresse électronique (%s) doit commencer par le SIREN (%s) et son schemeID doit être 0225 ; adresse : %q, schemeID : %q, SIREN : %q.", id[:len(id)-len(bt)-1], bt, bt, sirenBT, endpoint, scheme, siren)
		}
	}
	if !selfBilled {
		check(pBuyer, "BR-FR-21_BT-49", "BT-49", "BT-47")
	} else {
		check(pSeller, "BR-FR-22_BT-34", "BT-34", "BT-30")
	}
}

// electronicAddresses lists the parties whose electronic address is checked
// by BR-FR-23 and BR-FR-25, with the suffix of the assertion identifiers.
var electronicAddresses = []struct{ path, bt string }{
	{pSeller, "BT-34"},
	{pBuyer, "BT-49"},
	{pBuyerAgent, "EXT-FR-FE-12"},
	{pPayee, "EXT-FR-FE-29"},
	{pPayer, "EXT-FR-FE-52"},
	{pSalesAgent, "EXT-FR-FE-75"},
	{pInvoicee, "EXT-FR-FE-98"},
	{pInvoicer, "EXT-FR-FE-121"},
}

// endpointFormats checks BR-FR-23: electronic addresses of scheme 0225 are
// made of [A-Za-z0-9+\-_.].
func (k *checker) endpointFormats() {
	for _, p := range electronicAddresses {
		for uri := range all(k.root, p.path+"/"+pURIID) {
			if !attrIs(uri, "schemeID", "0225") {
				continue
			}
			if s := sv(uri); !validSchemeID(s) {
				k.fail("BR-FR-23_"+p.bt, uri, "BR-FR-23/%s : l'adresse électronique (ram:URIID, schemeID 0225) ne respecte pas le format autorisé ; seuls les caractères alphanumériques et les symboles + - _ . sont autorisés ; valeur actuelle : %q.", p.bt, s)
			}
		}
	}
}

// privateIDFormat checks BR-FR-24: private identifiers of scheme 0224 are
// made of [A-Za-z0-9+\-_.].
func (k *checker) privateIDFormat() {
	for _, p := range [...]struct{ path, bt string }{{pSeller, "BT-29"}, {pBuyer, "BT-46"}} {
		for id := range all(k.root, p.path+"/GlobalID") {
			if !attrIs(id, "schemeID", "0224") {
				continue
			}
			if s := sv(id); !validSchemeID(s) {
				k.fail("BR-FR-24_"+p.bt, id, "BR-FR-24/%s : l'identifiant privé (ram:GlobalID, schemeID 0224) ne respecte pas le format autorisé ; seuls les caractères alphanumériques et les symboles + - _ . sont autorisés ; valeur actuelle : %q.", p.bt, s)
			}
		}
	}
}

// endpointLengths checks BR-FR-25: electronic addresses have at most 125
// characters.
func (k *checker) endpointLengths() {
	for _, p := range electronicAddresses {
		for uri := range all(k.root, p.path+"/"+pURIID) {
			if s := sv(uri); length(s) > 125 {
				k.fail("BR-FR-25_"+p.bt, uri, "BR-FR-25/%s : l'adresse électronique (ram:URIID) dépasse 125 caractères (%d) ; valeur actuelle : %q.", p.bt, length(s), s)
			}
		}
	}
}

// privateIDLength checks BR-FR-26: private identifiers of scheme 0224 have
// at most 100 characters.
func (k *checker) privateIDLength() {
	for _, p := range [...]struct{ path, bt string }{{pSeller, "BT-29"}, {pBuyer, "BT-46"}} {
		for id := range all(k.root, p.path+"/GlobalID") {
			if !attrIs(id, "schemeID", "0224") {
				continue
			}
			if s := sv(id); length(s) > 100 {
				k.fail("BR-FR-26_"+p.bt, id, "BR-FR-26/%s : l'identifiant privé (ram:GlobalID, schemeID 0224) dépasse 100 caractères (%d) ; valeur actuelle : %q.", p.bt, length(s), s)
			}
		}
	}
}

// characteristics checks BR-FR-27 (item attribute name or code, not empty)
// and BR-FR-28 (attribute value or measure, not both).
func (k *checker) characteristics() {
	const path = pLines + "/SpecifiedTradeProduct/ApplicableProductCharacteristic"
	for ch := range all(k.root, path) {
		if !exists(ch, "Description") && !exists(ch, "TypeCode") {
			k.fail("BR-FR-27_BG-32", ch, "BR-FR-27/BG-32 : le groupe Attribut d'article (BG-32) doit contenir soit un nom d'attribut (BT-160, ram:Description), soit un code d'attribut (EXT-FR-FE-159, ram:TypeCode) ; aucun des deux n'a été trouvé.")
		}
	}
	for n := range all(k.root, path+"/Description") {
		if nsv(n) == "" {
			k.fail("BR-FR-27_BT-160", n, "BR-FR-27/BT-160 : le nom d'attribut d'article (ram:Description) ne doit pas être vide ; valeur actuelle : %q.", sv(n))
		}
	}
	for n := range all(k.root, path+"/TypeCode") {
		if nsv(n) == "" {
			k.fail("BR-FR-27_EXT-FR-FE-159", n, "BR-FR-27/EXT-FR-FE-159 : le code d'attribut d'article (ram:TypeCode) ne doit pas être vide ; valeur actuelle : %q.", sv(n))
		}
	}
	for ch := range all(k.root, path) {
		value := exists(ch, "Value")
		measure := exists(ch, "ValueMeasure")
		measureUnit := false
		for m := range all(ch, "ValueMeasure") {
			if u, ok := m.LookupAttr("unitCode"); ok && u != "" {
				measureUnit = true
				break
			}
		}
		if !(value && !measure || !value && measureUnit) {
			vm := first(ch, "ValueMeasure")
			unit, _ := vm.LookupAttr("unitCode")
			k.fail("BR-FR-28-Value", ch, "BR-FR-28 : la valeur d'attribut (ram:Value) ou la valeur mesurée (ram:ValueMeasure avec unité de mesure) doit être présente, mais pas les deux ; Value : %q, ValueMeasure : %q, unité : %q.", sv(first(ch, "Value")), sv(vm), unit)
		}
	}
}

// objects checks BR-FR-29 (invoiced objects BT-18) and BR-FR-30 (line
// invoiced objects BT-128): schemes AFL and AVV appear at most once, with a
// non-empty identifier.
func (k *checker) objects() {
	check := func(parent *xmltree.Node, base, bt string) {
		afl, avv := 0, 0
		for doc := range all(parent, "AdditionalReferencedDocument") {
			if anyEq(doc, "ReferenceTypeCode", "AFL") {
				afl++
			}
			if anyEq(doc, "ReferenceTypeCode", "AVV") {
				avv++
			}
		}
		if afl > 1 || avv > 1 {
			k.fail(base+"_"+bt, parent, "%s/%s : parmi les identifiants d'objets facturés (%s), les schémas AFL et AVV ne doivent être présents qu'une seule fois chacun ; AFL : %d occurrence(s), AVV : %d occurrence(s).", base, bt, bt, afl, avv)
		}
	}
	checkIDs := func(parent *xmltree.Node, base string) {
		for doc := range all(parent, "AdditionalReferencedDocument") {
			scheme := ""
			switch {
			case anyEq(doc, "ReferenceTypeCode", "AFL"):
				scheme = "AFL"
			case anyEq(doc, "ReferenceTypeCode", "AVV"):
				scheme = "AVV"
			default:
				continue
			}
			for id := range all(doc, "IssuerAssignedID") {
				if nsv(id) == "" {
					k.fail(base+"_"+scheme, id, "%s/%s : l'identifiant associé au schéma %s (ram:IssuerAssignedID) ne doit pas être vide ; valeur actuelle : %q.", base, scheme, scheme, sv(id))
				}
			}
		}
	}
	for agr := range all(k.root, pAgreement) {
		check(agr, "BR-FR-29", "BT-18")
	}
	for agr := range all(k.root, pAgreement) {
		checkIDs(agr, "BR-FR-29")
	}
	// BR-FR-30 looks for the documents directly under the line, as written
	// in the schematron.
	for line := range all(k.root, pLines) {
		check(line, "BR-FR-30", "BT-128")
	}
	for line := range all(k.root, pLines) {
		checkIDs(line, "BR-FR-30")
	}
}

// barUnique checks BR-FR-31: at most one BAR note holds a BAR treatment.
func (k *checker) barUnique() {
	for h := range all(k.root, pHeader) {
		n := 0
		var values []string
		for note := range all(h, "IncludedNote") {
			if !anyEq(note, "SubjectCode", "BAR") {
				continue
			}
			c := nsv(first(note, "Content"))
			values = append(values, c)
			if in(c, barTreatments...) {
				n++
			}
		}
		if n > 1 {
			k.fail("BR-FR-31_BT-21", h, "BR-FR-31/BT-21 : lorsque plusieurs notes ont le code sujet BAR (BT-21), une seule peut porter l'une des valeurs B2B, B2BINT, B2C, B2CINT, OUTOFSCOPE, ARCHIVEONLY ; valeurs fournies : %q, nombre de valeurs présentes : %d.", values, n)
		}
	}
}

// legalIDs checks BR-FR-32: every legal identifier of scheme 0002 and every
// global identifier of scheme 0002 or 0231 has 9 digits.
func (k *checker) legalIDs() {
	k.root.Walk(func(n *xmltree.Node) bool {
		switch n.Name {
		case "ID":
			if n.Parent != nil && n.Parent.Name == "SpecifiedLegalOrganization" && attrIs(n, "schemeID", "0002") && !digits(nsv(n), 9) {
				k.fail("BR-FR-32-LEGALID", n, "BR-FR-32/LEGALID : tout identifiant légal d'une partie avec schemeID 0002 doit être composé de 9 chiffres ; valeur actuelle : %q.", sv(n))
			}
		case "GlobalID":
			if (attrIs(n, "schemeID", "0002") || attrIs(n, "schemeID", "0231")) && !digits(nsv(n), 9) {
				k.fail("BR-FR-32-GLOBALID", n, "BR-FR-32/ID : tout identifiant d'une partie avec schemeID 0231 ou 0002 doit être composé de 9 chiffres ; valeur actuelle : %q.", sv(n))
			}
		}
		return true
	})
}
