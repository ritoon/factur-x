package fr

import (
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// coherence runs the BR-FR-CO-* rules of the UBL schematron.
func (k *ublChecker) coherence() {
	k.co03()
	k.co04()
	k.co05()
	k.co07()
	k.co08()
	k.co09()
	k.co10()
	k.co12()
	k.co14()
	k.co15()
}

// co03 checks BR-FR-CO-03 on credit notes: a global allowance credit note
// (262) carries the contract reference (BT-12) and the invoicing period
// (BG-14).
func (k *ublChecker) co03() {
	if !k.credit || !anyEq(k.root, "CreditNoteTypeCode", "262") {
		return
	}
	contract := sv(first(k.root, "ContractDocumentReference/ID"))
	start := sv(first(k.root, "InvoicePeriod/StartDate"))
	end := sv(first(k.root, "InvoicePeriod/EndDate"))
	if contract == "" || start == "" || end == "" {
		k.fail("BR-FR-CO-03_BT-3", nil, "BR-FR-CO-03/BT-3 : pour un avoir remise globale (BT-3 = 262), le numéro de contrat (BT-12, cac:ContractDocumentReference/cbc:ID) et la période de facturation (BG-14, cbc:StartDate et cbc:EndDate) doivent être renseignés ; BT-12 : %q, période : %q à %q.", contract, start, end)
	}
}

// co04 checks BR-FR-CO-04: a corrective invoice (cbc:InvoiceTypeCode 384,
// 471, 472, 473) references exactly one preceding invoice
// (cac:BillingReference).
func (k *ublChecker) co04() {
	if !anyIn(k.root, "InvoiceTypeCode", "384", "471", "472", "473") {
		return
	}
	if n := count(k.root, "BillingReference"); n != 1 {
		k.fail("BR-FR-CO-04_BT-3", nil, "BR-FR-CO-04/BT-3 : une facture rectificative (BT-3 = 384, 471, 472 ou 473) doit comporter une et une seule référence à une facture antérieure (cac:BillingReference, BT-25 et BT-26) ; références trouvées : %d.", n)
	}
}

// datedReference reports whether ref (a cac:InvoiceDocumentReference) has
// an identifier and an issue date.
func datedReference(ref *xmltree.Node) bool {
	return exists(ref, "ID") && exists(ref, "IssueDate")
}

// co05 checks BR-FR-CO-05 on credit notes: a credit note (261, 381, 396,
// 502, 503) references a preceding invoice in the header, or on every line
// that does not reference the credit note itself (BT-1) or does with the
// sub-type DETAIL.
func (k *ublChecker) co05() {
	if !k.credit || !anyIn(k.root, "CreditNoteTypeCode", "261", "381", "396", "502", "503") {
		return
	}
	headerRefs := 0
	for ref := range all(k.root, "BillingReference/InvoiceDocumentReference") {
		if datedReference(ref) {
			headerRefs++
		}
	}
	lineCount, lineRefs := 0, 0
	for line := range all(k.root, "CreditNoteLine") {
		detail := false
		for br := range all(line, "BillingReference") {
			if eqNodes(br, "InvoiceDocumentReference/ID", k.invIDs) && anyEq(br, "InvoiceDocumentReference/DocumentStatusCode", "DETAIL") {
				detail = true
			}
		}
		if eqNodes(line, "BillingReference/InvoiceDocumentReference/ID", k.invIDs) && !detail {
			continue
		}
		lineCount++
		for br := range all(line, "BillingReference") {
			if !neNodes(br, "InvoiceDocumentReference/ID", k.invIDs) {
				continue
			}
			dated := false
			for ref := range all(br, "InvoiceDocumentReference") {
				if datedReference(ref) {
					dated = true
				}
			}
			if dated {
				lineRefs++
				break
			}
		}
	}
	if headerRefs < 1 && lineRefs != lineCount {
		k.fail("BR-FR-CO-05_BT-3", nil, "BR-FR-CO-05/BT-3 : un avoir (BT-3 = 261, 381, 396, 502 ou 503) doit comporter soit au moins une référence à une facture antérieure (BT-25) avec sa date (BT-26) en entête, soit sur chaque ligne une référence à une facture antérieure (EXT-FR-FE-136) avec sa date (EXT-FR-FE-138) ; références en entête : %d, lignes référencées : %d sur %d.", headerRefs, lineRefs, lineCount)
	}
}

// dueDates returns the due dates of the document: cbc:DueDate and
// cac:PaymentMeans/cbc:PaymentDueDate, in document order.
func (k *ublChecker) dueDates() []*xmltree.Node {
	var out []*xmltree.Node
	for _, c := range k.root.Children {
		switch c.Name {
		case "DueDate":
			out = append(out, c)
		case "PaymentMeans":
			out = append(out, c.Elems("PaymentDueDate")...)
		}
	}
	return out
}

// typeCodeIn reports whether BT-3 (cbc:InvoiceTypeCode or
// cbc:CreditNoteTypeCode) has one of the values vs.
func (k *ublChecker) typeCodeIn(vs ...string) bool {
	return anyIn(k.root, "InvoiceTypeCode", vs...) || anyIn(k.root, "CreditNoteTypeCode", vs...)
}

// co07 checks BR-FR-CO-07: the due dates (BT-9) are not before the issue
// date (BT-2), except for advance invoices and paid frames. Dates are
// compared as strings, as the schematron does.
func (k *ublChecker) co07() {
	due := k.dueDates()
	if len(due) == 0 || k.typeCodeIn("386", "500", "503") || anyIn(k.root, "ProfileID", "B2", "S2", "M2") {
		return
	}
	for _, d := range due {
		for issue := range all(k.root, "IssueDate") {
			if sv(d) < sv(issue) {
				k.fail("BR-FR-CO-07_BT-9", nil, "BR-FR-CO-07/BT-9 : la date d'échéance (BT-9) doit être postérieure ou égale à la date de facture (BT-2), sauf pour les factures et avoirs d'acompte (386, 500, 503) ou les cadres de facturation B2, S2, M2 ; date de facture : %q, date d'échéance : %q, cadre : %q.", sv(issue), sv(d), sv(first(k.root, "ProfileID")))
				return
			}
		}
	}
}

// co08 checks BR-FR-CO-08: final invoices after advance (B4, S4, M4) are not
// advance invoices or credit notes (386, 500, 503).
func (k *ublChecker) co08() {
	if anyIn(k.root, "ProfileID", "B4", "S4", "M4") && k.typeCodeIn("386", "500", "503") {
		k.fail("BR-FR-CO-08_BT-23", nil, "BR-FR-CO-08/BT-23 : avec le cadre de facturation B4, S4 ou M4 (facture définitive après acompte), le type de facture (BT-3) ne peut pas être une facture ou un avoir d'acompte (386, 500, 503) ; BT-23 : %q, BT-3 : %q.", sv(first(k.root, "ProfileID")), sv(first(k.root, k.typeCode)))
	}
}

// co09 checks BR-FR-CO-09: invoices of the paid frames (B2, S2, M2) are fully
// prepaid, with nothing due and a due date.
func (k *ublChecker) co09() {
	if !anyIn(k.root, "ProfileID", "B2", "S2", "M2") {
		return
	}
	paidNode := first(k.root, "LegalMonetaryTotal/PrepaidAmount")
	totalNode := first(k.root, "LegalMonetaryTotal/TaxInclusiveAmount")
	dueNode := first(k.root, "LegalMonetaryTotal/PayableAmount")
	paid, okPaid := decimal(paidNode)
	total, okTotal := decimal(totalNode)
	if !okPaid || !okTotal || paid.Cmp(total) != 0 {
		k.fail("BR-FR-CO-09_BT-23-1", nil, "BR-FR-CO-09/BT-23 : avec le cadre de facturation B2, S2 ou M2 (facture déjà payée), le montant déjà payé (BT-113, cbc:PrepaidAmount) doit être égal au montant total TTC (BT-112, cbc:TaxInclusiveAmount) ; montant payé : %q, montant total : %q.", sv(paidNode), sv(totalNode))
	}
	if due, ok := decimal(dueNode); !ok || !due.IsZero() {
		k.fail("BR-FR-CO-09_BT-23-2", nil, "BR-FR-CO-09/BT-23 : avec le cadre de facturation B2, S2 ou M2, le net à payer (BT-115, cbc:PayableAmount) doit être égal à 0 ; net à payer : %q.", sv(dueNode))
	}
	var dueDate *xmltree.Node
	if d := k.dueDates(); len(d) > 0 {
		dueDate = d[0]
	}
	if sv(dueDate) == "" {
		k.fail("BR-FR-CO-09_BT-23-3", nil, "BR-FR-CO-09/BT-23 : avec le cadre de facturation B2, S2 ou M2, la date d'échéance (BT-9) doit être renseignée et correspondre à la date de paiement ; date d'échéance : %q.", sv(dueDate))
	}
}

// co10 checks BR-FR-CO-10: every identifier of a party has a scheme, and each
// scheme appears once per party. The party contexts apply at any depth,
// except the delivery locations.
func (k *ublChecker) co10() {
	type party struct {
		parties  func(yield func(*xmltree.Node) bool)
		ids      string
		bt       string
		label    string
		required string // business term of the scheme, for the message
	}
	const pid = "PartyIdentification/ID"
	for _, p := range []party{
		{anywhere(k.root, "AccountingSupplierParty/Party"), pid, "BT-29", "du vendeur", "BT-29-1"},
		{anywhere(k.root, "AccountingCustomerParty/Party"), pid, "BT-46", "de l'acheteur", "BT-46-1"},
		{anywhere(k.root, "PayeeParty"), pid, "BT-60", "du bénéficiaire", "BT-60-1"},
		{anywhere(k.root, "AccountingCustomerParty/Party/AgentParty"), pid, "EXT-FR-FE-06", "de l'agent de l'acheteur", "EXT-FR-FE-07"},
		{anywhere(k.root, "PaymentMeans/PaymentMandate/PayerParty"), pid, "EXT-FR-FE-46", "du payeur", "EXT-FR-FE-47"},
		{anywhere(k.root, "AccountingSupplierParty/Party/AgentParty"), pid, "EXT-FR-FE-69", "de l'agent du vendeur", "EXT-FR-FE-70"},
		{anywhere(k.root, "AccountingCustomerParty/Party/ServiceProviderParty/Party"), pid, "EXT-FR-FE-92", "du facturé à", "EXT-FR-FE-92-1"},
		{anywhere(k.root, "AccountingSupplierParty/Party/ServiceProviderParty/Party"), pid, "EXT-FR-FE-115", "du facturant", "EXT-FR-FE-116"},
		{all(k.root, "Delivery/DeliveryLocation"), "ID", "BT-71", "du livré à", "BT-71-1"},
		{all(k.root, k.line+"/Delivery/DeliveryLocation"), "ID", "EXT-FR-FE-146", "du livré à de la ligne", "EXT-FR-FE-148"},
	} {
		for n := range p.parties {
			var schemes []string
			missing := false
			for id := range all(n, p.ids) {
				if s, ok := id.LookupAttr("schemeID"); ok {
					schemes = append(schemes, s)
				} else {
					missing = true
				}
			}
			if missing {
				k.fail("BR-FR-CO-10_"+p.bt+"-1", n, "BR-FR-CO-10/%s : l'identifiant %s (%s, cbc:ID) est renseigné sans son schéma d'identification (%s, attribut schemeID).", p.bt, p.label, p.bt, p.required)
			}
			if dup, ok := duplicate(schemes); ok {
				k.fail("BR-FR-CO-10_"+p.bt+"-2", n, "BR-FR-CO-10/%s : chaque schemeID ne peut apparaître qu'une seule fois dans les identifiants %s (%s) ; schemeID répété : %q.", p.bt, p.label, p.bt, dup)
			}
		}
	}
}

// co12 checks BR-FR-CO-12: an invoice not in EUR gives the accounting
// currency EUR (BT-6) and the VAT total in EUR (BT-111).
func (k *ublChecker) co12() {
	if !anyNe(k.root, "DocumentCurrencyCode", "EUR") {
		return
	}
	taxEUR := firstWith(k.root, "TaxTotal/TaxAmount", "currencyID", "EUR")
	if !anyEq(k.root, "TaxCurrencyCode", "EUR") || sv(taxEUR) == "" {
		k.fail("BR-FR-CO-12_BT-5", nil, "BR-FR-CO-12/BT-5 : si la devise de facture (BT-5, cbc:DocumentCurrencyCode) est différente de EUR, la devise de comptabilité (BT-6, cbc:TaxCurrencyCode) doit être présente et égale à EUR, et le montant de TVA en devise de comptabilité (BT-111, cbc:TaxAmount de devise EUR) doit être renseigné ; BT-5 : %q, BT-6 : %q, BT-111 : %q.", sv(first(k.root, "DocumentCurrencyCode")), sv(first(k.root, "TaxCurrencyCode")), sv(taxEUR))
	}
}

// sellerIsAU reports whether the seller has an identifier of scheme 0231
// (member of a single taxable person).
func (k *ublChecker) sellerIsAU() bool {
	return firstWith(k.root, "AccountingSupplierParty/Party/PartyIdentification/ID", "schemeID", "0231") != nil
}

// co14 checks BR-FR-CO-14: a seller member of a single taxable person (0231)
// carries the note TXD MEMBRE_ASSUJETTI_UNIQUE.
func (k *ublChecker) co14() {
	if !k.sellerIsAU() {
		return
	}
	if v := noteValue(k.allNotes, "TXD"); v != "MEMBRE_ASSUJETTI_UNIQUE" {
		k.fail("BR-FR-CO-14_BT-29-1", nil, "BR-FR-CO-14/BT-29-1 : le vendeur est identifié avec le schéma 0231 (membre d'un assujetti unique) ; une note (BG-3) de code sujet TXD (BT-21) et de texte MEMBRE_ASSUJETTI_UNIQUE (BT-22) doit alors être présente (cbc:Note #TXD#MEMBRE_ASSUJETTI_UNIQUE) ; valeur TXD : %q.", v)
	}
}

// co15 checks BR-FR-CO-15: a seller member of a single taxable person (0231)
// has a tax representative (BG-11) with the VAT identifier of the single
// taxable person (BT-63).
func (k *ublChecker) co15() {
	if !k.sellerIsAU() {
		return
	}
	rep := first(k.root, "TaxRepresentativeParty/PartyTaxScheme")
	vat := first(k.root, "TaxRepresentativeParty/PartyTaxScheme/CompanyID")
	if rep != nil && sv(vat) != "" {
		return
	}
	state := "absent"
	if rep != nil {
		state = "présent"
	}
	k.fail("BR-FR-CO-15_BT-29-1", nil, "BR-FR-CO-15/BT-29-1 : le vendeur est membre d'un assujetti unique (BT-29-1 = 0231) ; le représentant fiscal (BG-11, cac:TaxRepresentativeParty/cac:PartyTaxScheme) doit être présent avec le numéro de TVA de l'assujetti unique (BT-63, cbc:CompanyID) ; représentant fiscal %s, numéro de TVA : %q.", state, sv(vat))
}

// decimals runs the BR-FR-DEC-* rules on the formats of amounts,
// quantities and rates.
func (k *ublChecker) decimals() {
	type ctx struct {
		nodes func(yield func(*xmltree.Node) bool)
		id    string
		label string
	}
	lmt := func(name string) func(yield func(*xmltree.Node) bool) {
		return anywhere(k.root, "LegalMonetaryTotal/"+name)
	}
	for _, a := range []ctx{
		{all(k.root, "AllowanceCharge/BaseAmount"), "BR-FR-DEC-01_BT-93_BT-100", "BT-93/BT-100"},
		{all(k.root, "AllowanceCharge/Amount"), "BR-FR-DEC-01_BT-92_BT-99", "BT-92/BT-99"},
		{lmt("LineExtensionAmount"), "BR-FR-DEC-01_BT-106", "BT-106"},
		{lmt("AllowanceTotalAmount"), "BR-FR-DEC-01_BT-107", "BT-107"},
		{lmt("ChargeTotalAmount"), "BR-FR-DEC-01_BT-108", "BT-108"},
		{lmt("TaxExclusiveAmount"), "BR-FR-DEC-01_BT-109", "BT-109"},
		// BT-110 and BT-111 share this context: the first rule wins.
		{anywhere(k.root, "TaxTotal/TaxAmount"), "BR-FR-DEC-01_BT-110", "BT-110"},
		{lmt("TaxInclusiveAmount"), "BR-FR-DEC-01_BT-112", "BT-112"},
		{lmt("PrepaidAmount"), "BR-FR-DEC-01_BT-113", "BT-113"},
		{lmt("PayableRoundingAmount"), "BR-FR-DEC-01_BT-114", "BT-114"},
		{lmt("PayableAmount"), "BR-FR-DEC-01_BT-115", "BT-115"},
		{anywhere(k.root, "TaxTotal/TaxSubtotal/TaxableAmount"), "BR-FR-DEC-01_BT-116", "BT-116"},
		{anywhere(k.root, "TaxTotal/TaxSubtotal/TaxAmount"), "BR-FR-DEC-01_BT-117", "BT-117"},
		{all(k.root, k.line+"/LineExtensionAmount"), "BR-FR-DEC-01_BT-131", "BT-131"},
		{all(k.root, k.line+"/AllowanceCharge/Amount"), "BR-FR-DEC-01_BT-136_BT-141", "BT-136/BT-141"},
		{all(k.root, k.line+"/AllowanceCharge/BaseAmount"), "BR-FR-DEC-01_BT-137_BT-142", "BT-137/BT-142"},
	} {
		for n := range a.nodes {
			if s := nsv(n); !decimalFormat(s, true, 2) {
				k.fail(a.id, n, "BR-FR-DEC-01/%s : le montant %q est invalide ; il doit comporter au plus 2 décimales, avec un point comme séparateur, et ne pas dépasser 19 caractères (hors séparateur) ; le signe - est autorisé.", a.label, s)
			}
		}
	}
	quantity := "InvoicedQuantity"
	if k.credit {
		quantity = "CreditedQuantity"
	}
	for n := range all(k.root, k.line+"/"+quantity) {
		if s := nsv(n); !decimalFormat(s, true, 4) {
			k.fail("BR-FR-DEC-02_BT-129", n, "BR-FR-DEC-02/BT-129 : la quantité %q (cbc:%s) est invalide ; elle doit être un nombre avec au plus 4 décimales (séparateur .), d'au plus 19 caractères (hors séparateur), éventuellement précédé du signe -.", s, quantity)
		}
	}
	for line := range k.lines() {
		if s := nsv(first(line, "Price/BaseQuantity")); s != "" && !decimalFormat(s, true, 4) {
			k.fail("BR-FR-DEC-02_BT-149", line, "BR-FR-DEC-02/BT-149 : la quantité de base du prix %q (cbc:BaseQuantity) est invalide ; elle doit être un nombre avec au plus 4 décimales (séparateur .), d'au plus 19 caractères (hors séparateur), éventuellement précédé du signe -.", s)
		}
	}
	price := func(id, label string, n *xmltree.Node, s string) {
		if s != "" && !decimalFormat(s, k.bd, 6) {
			k.fail(id, n, "BR-FR-DEC-03/%s : le montant %q est invalide ; il doit être positif (sans signe -, sauf si le cadre de facturation BT-23 est S9, B9 ou M9), comporter au plus 6 décimales (séparateur .) et ne pas dépasser 19 caractères (hors séparateur).", label, s)
		}
	}
	for line := range k.lines() {
		price("BR-FR-DEC-03_BT-146", "BT-146", line, nsv(first(line, "Price/PriceAmount")))
	}
	for ac := range all(k.root, k.line+"/Price/AllowanceCharge") {
		price("BR-FR-DEC-03_BT-147", "BT-147", ac, nsv(first(ac, "Amount")))
	}
	for n := range all(k.root, k.line+"/Price/AllowanceCharge/BaseAmount") {
		price("BR-FR-DEC-03_BT-148", "BT-148", n, nsv(n))
	}
	for _, r := range []ctx{
		{anywhere(k.root, "AllowanceCharge/TaxCategory/Percent"), "BR-FR-DEC-04_BT-96_BT-103", "BT-96 ou BT-103"},
		{anywhere(k.root, "TaxTotal/TaxSubtotal/TaxCategory/Percent"), "BR-FR-DEC-04_BT-119", "BT-119"},
		{anywhere(k.root, "Item/ClassifiedTaxCategory/Percent"), "BR-FR-DEC-04_BT-152", "BT-152"},
	} {
		for n := range r.nodes {
			if s := nsv(n); !percentFormat(s) {
				k.fail(r.id, n, "BR-FR-DEC-04/%s : le taux de TVA %q est invalide ; il doit être positif (sans signe -), comporter au plus 2 décimales (séparateur .) et ne pas dépasser 4 caractères (hors séparateur).", r.label, s)
			}
		}
	}
}
