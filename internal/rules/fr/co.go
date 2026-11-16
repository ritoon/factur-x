package fr

import (
	"github.com/ritoon/factur-x/internal/xmltree"
)

// coherence runs the BR-FR-CO-* rules.
func (k *checker) coherence() {
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

// co03 checks BR-FR-CO-03: a global allowance credit note (262) carries the
// contract reference (BT-12) and the invoicing period (BG-14).
func (k *checker) co03() {
	if !anyEq(k.root, pTypeCode, "262") {
		return
	}
	contract := sv(first(k.root, pAgreement+"/ContractReferencedDocument/IssuerAssignedID"))
	start := sv(first(k.root, pSettlement+"/BillingSpecifiedPeriod/StartDateTime/DateTimeString"))
	end := sv(first(k.root, pSettlement+"/BillingSpecifiedPeriod/EndDateTime/DateTimeString"))
	if contract == "" || start == "" || end == "" {
		k.fail("BR-FR-CO-03_BT-3", first(k.root, pTypeCode), "BR-FR-CO-03/BT-3 : pour un avoir remise globale (BT-3 = 262), le numéro de contrat (BT-12) et la période de facturation (BG-14, dates de début et de fin) doivent être renseignés ; BT-12 : %q, période : %q à %q.", contract, start, end)
	}
}

// co04 checks BR-FR-CO-04: a corrective invoice (384, 471, 472, 473)
// references exactly one preceding invoice (BG-3).
func (k *checker) co04() {
	if !anyIn(k.root, pTypeCode, "384", "471", "472", "473") {
		return
	}
	if n := count(k.root, pSettlement+"/InvoiceReferencedDocument"); n != 1 {
		k.fail("BR-FR-CO-04_BT-4", first(k.root, pTypeCode), "BR-FR-CO-04/BT-3 : une facture rectificative (BT-3 = 384, 471, 472 ou 473) doit comporter une et une seule référence à une facture antérieure (BT-25) avec sa date (BT-26) ; références trouvées : %d.", n)
	}
}

// hasDatedReference counts the InvoiceReferencedDocument of parent with an
// IssuerAssignedID and a FormattedIssueDateTime/DateTimeString.
func hasDatedReference(parent *xmltree.Node) int {
	n := 0
	for ref := range all(parent, "InvoiceReferencedDocument") {
		if exists(ref, "IssuerAssignedID") && exists(ref, "FormattedIssueDateTime/DateTimeString") {
			n++
		}
	}
	return n
}

// co05 checks BR-FR-CO-05: a credit note (261, 381, 396, 502, 503)
// references a preceding invoice in the header, or on every line without
// sub-type or with sub-type DETAIL.
func (k *checker) co05() {
	if !anyIn(k.root, pTypeCode, "261", "381", "396", "502", "503") {
		return
	}
	headerRefs := 0
	for s := range all(k.root, pSettlement) {
		headerRefs += hasDatedReference(s)
	}
	if headerRefs >= 1 {
		return
	}
	linesOK := true
	for line := range all(k.root, pLines) {
		if exists(line, pLineStatus) && !anyEq(line, pLineStatus, "DETAIL") {
			continue
		}
		if !lineHasDatedReference(line) {
			linesOK = false
			break
		}
	}
	if !linesOK {
		k.fail("BR-FR-CO-05_BT-3", first(k.root, pTypeCode), "BR-FR-CO-05/BT-3 : un avoir (BT-3 = 261, 381, 396, 502 ou 503) doit comporter soit au moins une référence à une facture antérieure (BT-25) avec sa date (BT-26) en entête, soit sur chaque ligne sans sous-type ou de sous-type DETAIL une référence à une facture antérieure (EXT-FR-FE-136) avec sa date (EXT-FR-FE-138) ; références en entête : %d.", headerRefs)
	}
}

// lineHasDatedReference reports whether one of the SpecifiedLineTradeSettlement
// of line has a dated invoice reference.
func lineHasDatedReference(line *xmltree.Node) bool {
	for s := range all(line, "SpecifiedLineTradeSettlement") {
		if hasDatedReference(s) > 0 {
			return true
		}
	}
	return false
}

// co07 checks BR-FR-CO-07: the due dates (BT-9) are not before the issue
// date (BT-2), except for advance invoices and paid frames. Dates are
// compared as strings, as the schematron does.
func (k *checker) co07() {
	var due []*xmltree.Node
	nonBlank := false
	for d := range all(k.root, pDueDates) {
		due = append(due, d)
		if nsv(d) != "" {
			nonBlank = true
		}
	}
	if !nonBlank {
		return
	}
	if anyIn(k.root, pTypeCode, "386", "500", "503") || anyIn(k.root, pBillingMode, "B2", "S2", "M2") {
		return
	}
	issueNode := first(k.root, pIssueDate)
	issue := sv(issueNode)
	for _, d := range due {
		if issueNode == nil || sv(d) < issue {
			k.fail("BR-FR-CO-07_BT-9", d, "BR-FR-CO-07/BT-9 : la date d'échéance (BT-9) doit être postérieure ou égale à la date de facture (BT-2), sauf pour les factures et avoirs d'acompte (386, 500, 503) ou les cadres de facturation B2, S2, M2 ; date de facture : %q, date d'échéance : %q, type : %q, cadre : %q.", issue, sv(d), sv(first(k.root, pTypeCode)), sv(first(k.root, pBillingMode)))
			return
		}
	}
}

// co08 checks BR-FR-CO-08: final invoices after advance (B4, S4, M4) are not
// advance invoices or credit notes (386, 500, 503).
func (k *checker) co08() {
	if anyIn(k.root, pBillingMode, "B4", "S4", "M4") && anyIn(k.root, pTypeCode, "386", "500", "503") {
		k.fail("BR-FR-CO-08_BT-23", first(k.root, pBillingMode), "BR-FR-CO-08/BT-23 : avec le cadre de facturation B4, S4 ou M4 (facture définitive après acompte), le type de facture (BT-3) ne peut pas être une facture ou un avoir d'acompte (386, 500, 503) ; BT-23 : %q, BT-3 : %q.", sv(first(k.root, pBillingMode)), sv(first(k.root, pTypeCode)))
	}
}

// co09 checks BR-FR-CO-09: invoices of the paid frames (B2, S2, M2) are fully
// prepaid, with nothing due and a due date.
func (k *checker) co09() {
	if !anyIn(k.root, pBillingMode, "B2", "S2", "M2") {
		return
	}
	at := first(k.root, pBillingMode)
	paidNode := first(k.root, pSummation+"/TotalPrepaidAmount")
	totalNode := first(k.root, pSummation+"/GrandTotalAmount")
	dueNode := first(k.root, pSummation+"/DuePayableAmount")
	paid, okPaid := decimal(paidNode)
	total, okTotal := decimal(totalNode)
	if !okPaid || !okTotal || paid.Cmp(total) != 0 {
		k.fail("BR-FR-CO-09_BT-23-1", orNode(paidNode, at), "BR-FR-CO-09/BT-23 : avec le cadre de facturation B2, S2 ou M2 (facture déjà payée), le montant déjà payé (BT-113) doit être égal au montant total TTC (BT-112) ; montant payé : %q, montant total : %q.", sv(paidNode), sv(totalNode))
	}
	if due, ok := decimal(dueNode); !ok || !due.IsZero() {
		k.fail("BR-FR-CO-09_BT-23-2", orNode(dueNode, at), "BR-FR-CO-09/BT-23 : avec le cadre de facturation B2, S2 ou M2, le net à payer (BT-115) doit être égal à 0 ; net à payer : %q.", sv(dueNode))
	}
	dueDate := false
	for d := range all(k.root, pDueDates) {
		if nsv(d) != "" {
			dueDate = true
			break
		}
	}
	if !dueDate {
		k.fail("BR-FR-CO-09_BT-23-3", at, "BR-FR-CO-09/BT-23 : avec le cadre de facturation B2, S2 ou M2, la date d'échéance (BT-9) doit être renseignée et correspondre à la date de paiement ; date d'échéance : %q.", sv(first(k.root, pDueDates)))
	}
}

// co10 checks BR-FR-CO-10: every global identifier of a party has a scheme,
// and each scheme appears once per party.
func (k *checker) co10() {
	for _, p := range []struct{ path, bt, label string }{
		{pSeller, "BT-29", "du vendeur"},
		{pBuyer, "BT-46", "de l'acheteur"},
		{pPayee, "BT-60", "du bénéficiaire"},
		{pBuyerAgent, "EXT-FR-FE-06", "de l'agent de l'acheteur"},
		{pPayer, "EXT-FR-FE-46", "du payeur"},
		{pSalesAgent, "EXT-FR-FE-69", "de l'agent du vendeur"},
		{pInvoicee, "EXT-FR-FE-92", "du facturé à"},
		{pInvoicer, "EXT-FR-FE-115", "du facturant"},
		{pShipTo, "BT-71", "du livré à"},
		{pLines + "/SpecifiedLineTradeDelivery/ShipToTradeParty", "EXT-FR-FE-146", "du livré à de la ligne"},
	} {
		for party := range all(k.root, p.path) {
			var schemes []string
			missing := false
			for id := range all(party, "GlobalID") {
				if s, ok := id.LookupAttr("schemeID"); ok {
					schemes = append(schemes, s)
				} else {
					missing = true
				}
			}
			if missing {
				k.fail("BR-FR-CO-10_"+p.bt+"-1", party, "BR-FR-CO-10/%s : l'identifiant global %s (%s) est renseigné sans son schéma d'identification (schemeID).", p.bt, p.label, p.bt)
			}
			if dup, ok := duplicate(schemes); ok {
				k.fail("BR-FR-CO-10_"+p.bt+"-2", party, "BR-FR-CO-10/%s : chaque schemeID ne peut apparaître qu'une seule fois dans les identifiants %s (%s) ; schemeID répété : %q.", p.bt, p.label, p.bt, dup)
			}
		}
	}
}

// duplicate returns the first value of vs that appears twice; ok is false
// when all values are distinct (count(distinct-values(vs)) = count(vs)).
func duplicate(vs []string) (dup string, ok bool) {
	for i := 1; i < len(vs); i++ {
		for j := 0; j < i; j++ {
			if vs[i] == vs[j] {
				return vs[i], true
			}
		}
	}
	return "", false
}

// co12 checks BR-FR-CO-12: an invoice not in EUR gives the accounting
// currency EUR (BT-6) and the VAT total in EUR (BT-111).
func (k *checker) co12() {
	if !anyNe(k.root, pCurrency, "EUR") {
		return
	}
	taxEUR := firstWith(k.root, pSummation+"/TaxTotalAmount", "currencyID", "EUR")
	if !anyEq(k.root, pTaxCurrency, "EUR") || sv(taxEUR) == "" {
		k.fail("BR-FR-CO-12_BT-5", first(k.root, pCurrency), "BR-FR-CO-12/BT-5 : si la devise de facture (BT-5) est différente de EUR, la devise de comptabilité (BT-6) doit être présente et égale à EUR, et le montant de TVA en devise de comptabilité (BT-111) doit être renseigné avec la devise EUR ; BT-5 : %q, BT-6 : %q, BT-111 : %q.", sv(first(k.root, pCurrency)), sv(first(k.root, pTaxCurrency)), sv(taxEUR))
	}
}

// sellerIsAU reports whether the seller has a global identifier of scheme
// 0231 (member of a single taxable person).
func (k *checker) sellerIsAU() bool {
	return firstWith(k.root, pSeller+"/GlobalID", "schemeID", "0231") != nil
}

// co14 checks BR-FR-CO-14: a seller member of a single taxable person (0231)
// carries the note TXD MEMBRE_ASSUJETTI_UNIQUE.
func (k *checker) co14() {
	if !k.sellerIsAU() {
		return
	}
	for note := range all(k.root, pNotes) {
		if anyEq(note, "SubjectCode", "TXD") && anyEq(note, "Content", "MEMBRE_ASSUJETTI_UNIQUE") {
			return
		}
	}
	k.fail("BR-FR-CO-14_BT-29-1", firstWith(k.root, pSeller+"/GlobalID", "schemeID", "0231"), "BR-FR-CO-14/BT-29-1 : le vendeur est identifié avec le schéma 0231 (membre d'un assujetti unique) ; une note (BG-1) de code sujet TXD (BT-21) et de texte MEMBRE_ASSUJETTI_UNIQUE (BT-22) doit alors être présente.")
}

// co15 checks BR-FR-CO-15: a seller member of a single taxable person (0231)
// has a tax representative (BG-11) with the VAT identifier of the single
// taxable person (BT-63).
func (k *checker) co15() {
	if !k.sellerIsAU() {
		return
	}
	rep := first(k.root, pTaxRep)
	vat := firstWith(k.root, pTaxRep+"/SpecifiedTaxRegistration/ID", "schemeID", "VA")
	if rep != nil && sv(vat) != "" {
		return
	}
	state := "absent"
	if rep != nil {
		state = "présent"
	}
	k.fail("BR-FR-CO-15_BT-29-1", firstWith(k.root, pSeller+"/GlobalID", "schemeID", "0231"), "BR-FR-CO-15/BT-29-1 : le vendeur est membre d'un assujetti unique (BT-29-1 = 0231) ; le représentant fiscal (BG-11) doit être présent avec le numéro de TVA de l'assujetti unique (BT-63) ; représentant fiscal %s, numéro de TVA : %q.", state, sv(vat))
}

// decimals runs the BR-FR-DEC-* rules on the formats of amounts,
// quantities, prices and rates.
func (k *checker) decimals() {
	amounts := []struct{ path, id, label string }{
		{pHeaderAC + "/ActualAmount", "BR-FR-DEC-01_BT-92_BT-99", "BT-92/BT-99"},
		{pHeaderAC + "/BasisAmount", "BR-FR-DEC-01_BT-93_BT-100", "BT-93/BT-100"},
		{pSummation + "/LineTotalAmount", "BR-FR-DEC-01_BT-106", "BT-106"},
		{pSummation + "/AllowanceTotalAmount", "BR-FR-DEC-01_BT-107", "BT-107"},
		{pSummation + "/ChargeTotalAmount", "BR-FR-DEC-01_BT-108", "BT-108"},
		{pSummation + "/TaxBasisTotalAmount", "BR-FR-DEC-01_BT-109", "BT-109"},
		// BT-110 and BT-111 share this context: the first rule wins.
		{pSummation + "/TaxTotalAmount", "BR-FR-DEC-01_BT-110", "BT-110"},
		{pSummation + "/GrandTotalAmount", "BR-FR-DEC-01_BT-112", "BT-112"},
		{pSummation + "/TotalPrepaidAmount", "BR-FR-DEC-01_BT-113", "BT-113"},
		{pSummation + "/RoundingAmount", "BR-FR-DEC-01_BT-114", "BT-114"},
		{pSummation + "/DuePayableAmount", "BR-FR-DEC-01_BT-115", "BT-115"},
		{pHeaderTax + "/BasisAmount", "BR-FR-DEC-01_BT-116", "BT-116"},
		{pHeaderTax + "/CalculatedAmount", "BR-FR-DEC-01_BT-117", "BT-117"},
		{pLines + "/" + pLineSum + "/LineTotalAmount", "BR-FR-DEC-01_BT-131", "BT-131"},
		{pLines + "/SpecifiedLineTradeSettlement/SpecifiedTradeAllowanceCharge/ActualAmount", "BR-FR-DEC-01_BT-136_BT-141", "BT-136/BT-141"},
		{pLines + "/SpecifiedLineTradeSettlement/SpecifiedTradeAllowanceCharge/BasisAmount", "BR-FR-DEC-01_BT-137_BT-142", "BT-137/BT-142"},
	}
	for _, a := range amounts {
		for n := range all(k.root, a.path) {
			if s := nsv(n); !decimalFormat(s, true, 2) {
				k.fail(a.id, n, "BR-FR-DEC-01/%s : le montant %q est invalide ; il doit comporter au plus 2 décimales, avec un point comme séparateur, et ne pas dépasser 19 caractères (hors séparateur) ; le signe - est autorisé.", a.label, s)
			}
		}
	}
	for _, q := range []struct{ path, id, label string }{
		{pLines + "/SpecifiedLineTradeDelivery/BilledQuantity", "BR-FR-DEC-02_BT-129", "BT-129"},
		{pLines + "/SpecifiedLineTradeAgreement/NetPriceProductTradePrice/BasisQuantity", "BR-FR-DEC-02_BT-149", "BT-149"},
	} {
		for n := range all(k.root, q.path) {
			if s := nsv(n); !decimalFormat(s, true, 4) {
				k.fail(q.id, n, "BR-FR-DEC-02/%s : la quantité %q est invalide ; elle doit être un nombre avec au plus 4 décimales (séparateur .), d'au plus 19 caractères (hors séparateur), éventuellement précédé du signe -.", q.label, s)
			}
		}
	}
	for _, p := range []struct{ path, id, label string }{
		{pLines + "/SpecifiedLineTradeAgreement/NetPriceProductTradePrice/ChargeAmount", "BR-FR-DEC-03_BT-146", "BT-146"},
		{pLines + "/SpecifiedLineTradeAgreement/GrossPriceProductTradePrice/AppliedTradeAllowanceCharge/ActualAmount", "BR-FR-DEC-03_BT-147", "BT-147"},
		{pLines + "/SpecifiedLineTradeAgreement/GrossPriceProductTradePrice/ChargeAmount", "BR-FR-DEC-03_BT-148", "BT-148"},
	} {
		for n := range all(k.root, p.path) {
			if s := nsv(n); !decimalFormat(s, k.bd, 6) {
				k.fail(p.id, n, "BR-FR-DEC-03/%s : le montant %q est invalide ; il doit être positif (sans signe -, sauf si le cadre de facturation BT-23 est S9, B9 ou M9), comporter au plus 6 décimales (séparateur .) et ne pas dépasser 19 caractères (hors séparateur).", p.label, s)
			}
		}
	}
	for _, r := range []struct{ path, id, label string }{
		{pHeaderAC + "/CategoryTradeTax/RateApplicablePercent", "BR-FR-DEC-04_BT-96_BT-103", "BT-96 ou BT-103"},
		{pHeaderTax + "/RateApplicablePercent", "BR-FR-DEC-04_BT-119", "BT-119"},
		{pLines + "/SpecifiedLineTradeSettlement/ApplicableTradeTax/RateApplicablePercent", "BR-FR-DEC-04_BT-152", "BT-152"},
	} {
		for n := range all(k.root, r.path) {
			if s := nsv(n); !percentFormat(s) {
				k.fail(r.id, n, "BR-FR-DEC-04/%s : le taux de TVA %q est invalide ; il doit être positif (sans signe -), comporter au plus 2 décimales (séparateur .) et ne pas dépasser 4 caractères (hors séparateur).", r.label, s)
			}
		}
	}
}

// decimalFormat implements custom:is-valid-decimal-19-N:
// matches(s, '^[-]?\d{1,19}(\.\d{1,N})?$') and at most 19 characters once
// the decimal point is removed. signed allows the minus sign.
func decimalFormat(s string, signed bool, maxFrac int) bool {
	return numberFormat(s, signed, 19, maxFrac, 19)
}

// percentFormat implements custom:is-valid-percent-4-2-positive:
// matches(s, '^\d{1,4}(\.\d{1,2})?$') and at most 4 characters once the
// decimal point is removed.
func percentFormat(s string) bool { return numberFormat(s, false, 4, 2, 4) }

func numberFormat(s string, signed bool, maxInt, maxFrac, maxLen int) bool {
	n := 0 // characters other than '.'
	intDigits, fracDigits := 0, 0
	dot := false
	for i, r := range s {
		switch {
		case r == '-' && i == 0 && signed:
		case r == '.' && !dot && intDigits > 0:
			dot = true
			n--
		case isDigit(r):
			if dot {
				fracDigits++
			} else {
				intDigits++
			}
		default:
			return false
		}
		n++
	}
	if intDigits < 1 || intDigits > maxInt || dot && (fracDigits < 1 || fracDigits > maxFrac) {
		return false
	}
	return n <= maxLen
}
