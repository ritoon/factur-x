package fr

import (
	"math"
	"strconv"
	"strings"

	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// In UBL the multi-vendor (BT-23 = S8, B8, M8) and two-way (S9, B9, M9)
// structure is carried by the line invoice references: a line points to the
// invoice itself (cac:BillingReference/cac:InvoiceDocumentReference/cbc:ID =
// BT-1) with its sub-type (cbc:DocumentStatusCode, EXT-FR-FE-163) and its
// parent line (cac:BillingReferenceLine/cbc:ID, EXT-FR-FE-162). The line
// seller is the manufacturer of the item (cac:ManufacturerParty), the
// sub-invoice number the line document reference of type 130 and scheme AFL
// (BT-128).

// selfRef reports whether the billing reference br points to the invoice
// itself (cac:InvoiceDocumentReference/cbc:ID = $invoiceID).
func (k *ublChecker) selfRef(br *xmltree.Node) bool {
	return eqNodes(br, "InvoiceDocumentReference/ID", k.invIDs)
}

// isGroupRef reports whether br is the reference of a GROUP line without
// parent: it points to the invoice itself, with the sub-type GROUP and no
// parent line.
func (k *ublChecker) isGroupRef(br *xmltree.Node) bool {
	return k.selfRef(br) && anyEq(br, "InvoiceDocumentReference/DocumentStatusCode", "GROUP") && !exists(br, "BillingReferenceLine/ID")
}

// groupRefs returns the references of the GROUP lines without parent among
// lines, in document order.
func (k *ublChecker) groupRefs(lines []*xmltree.Node) []*xmltree.Node {
	var out []*xmltree.Node
	for _, line := range lines {
		for br := range all(line, "BillingReference") {
			if k.isGroupRef(br) {
				out = append(out, br)
			}
		}
	}
	return out
}

// rootLines returns the lines of the root (ubl:Invoice/cac:InvoiceLine |
// cn:CreditNote/cac:CreditNoteLine) as a slice.
func (k *ublChecker) rootLines() []*xmltree.Node { return nodes(k.root, k.line) }

// lineNumber returns the string value of the cbc:ID of a line, for messages.
func lineNumber(line *xmltree.Node) string { return sv(first(line, "ID")) }

// subInvoiceIDs returns the identifiers of scheme AFL of the document
// references of type 130 of line (cac:DocumentReference[cbc:DocumentTypeCode
// = '130']/cbc:ID[@schemeID = 'AFL']).
func subInvoiceIDs(line *xmltree.Node, scheme string) []*xmltree.Node {
	var out []*xmltree.Node
	for doc := range all(line, "DocumentReference") {
		if anyEq(doc, "DocumentTypeCode", "130") {
			out = append(out, withAttr(doc, "ID", "schemeID", scheme)...)
		}
	}
	return out
}

// subInvoice returns the normalized first sub-invoice number (AFL) of line.
func subInvoice(line *xmltree.Node) string {
	if ids := subInvoiceIDs(line, "AFL"); len(ids) > 0 {
		return nsv(ids[0])
	}
	return ""
}

// lineSellerID returns the normalized first legal identifier of the line
// seller (cac:Item/cac:ManufacturerParty/cac:PartyLegalEntity/cbc:CompanyID).
func lineSellerID(line *xmltree.Node) string {
	return nsv(first(line, "Item/ManufacturerParty/PartyLegalEntity/CompanyID"))
}

// lineTax returns the first line VAT total (cac:TaxTotal/cbc:TaxAmount)
// whose currency is one of the string values of currencies.
func lineTax(line *xmltree.Node, currencies []*xmltree.Node) *xmltree.Node {
	for t := range all(line, "TaxTotal/TaxAmount") {
		if cur, ok := t.LookupAttr("currencyID"); ok && strIn(cur, currencies) {
			return t
		}
	}
	return nil
}

// multiVendor runs the BR-FR-MV-* and BR-FR-BD-* rules of the UBL schematron.
func (k *ublChecker) multiVendor() {
	k.mv01()
	k.mv02()
	k.bd02()
	k.mv03()
	k.mv05()
	k.mv06()
	k.mv07()
	k.mv08()
	k.mv09()
	k.mv10()
	k.mv11()
	k.mv12()
	k.mv13()
	k.bd13()
	k.precedingInvoice(k.mv, "BR-FR-MV-14", "S8, B8 ou M8", "384", "472", "381", "396", "503")
	k.precedingInvoice(k.bd, "BR-FR-BD-14", "S9, B9 ou M9", "261", "471", "473", "502")
}

// mv01 checks BR-FR-MV-01: in the frames S8, B8, M8, S9, B9, M9 every line
// (cac:InvoiceLine or cac:CreditNoteLine, at any depth) references the
// invoice itself with a sub-type.
func (k *ublChecker) mv01() {
	if !k.special {
		return
	}
	k.root.Walk(func(line *xmltree.Node) bool {
		if line.Name != "InvoiceLine" && line.Name != "CreditNoteLine" {
			return true
		}
		var status *xmltree.Node
		self := false
		for br := range all(line, "BillingReference") {
			if k.selfRef(br) {
				self = true
				if status == nil {
					status = first(br, "InvoiceDocumentReference/DocumentStatusCode")
				}
			}
		}
		if !self || nsv(status) == "" {
			k.fail("BR-FR-MV-01_EXT-FR-FE-163", line, "BR-FR-MV-01/EXT-FR-FE-163 : lorsque le cadre de facturation (BT-23) est S8, B8, M8, S9, B9 ou M9, chaque ligne (BG-25) doit référencer la facture elle-même (cac:BillingReference, BT-1 = %q) avec un sous-type de ligne (EXT-FR-FE-163, cbc:DocumentStatusCode) ; ligne %q, sous-type : %q.", sv(first(k.root, "ID")), lineNumber(line), sv(status))
		}
		return true
	})
}

// mv02 checks BR-FR-MV-02: a multi-vendor invoice has at least one GROUP line
// without parent.
func (k *ublChecker) mv02() {
	if k.mv && len(k.groupRefs(k.anyLines())) < 1 {
		k.fail("BR-FR-MV-02_EXT-FR-FE-163", nil, "BR-FR-MV-02/EXT-FR-FE-163 : lorsque le cadre de facturation (BT-23) est S8, B8 ou M8, la facture doit contenir au moins une ligne (BG-25) de sous-type GROUP (cbc:DocumentStatusCode) sans ligne parent (cac:BillingReferenceLine).")
	}
}

// bd02 checks BR-FR-BD-02: a two-way invoice has exactly two GROUP lines
// without parent, one for the seller and one for the buyer.
func (k *ublChecker) bd02() {
	if !k.bd {
		return
	}
	refs := k.groupRefs(k.anyLines())
	if len(refs) != 2 {
		k.fail("BR-FR-BD-02_EXT-FR-FE-163", nil, "BR-FR-BD-02/EXT-FR-FE-163 : lorsque le cadre de facturation (BT-23) est S9, B9 ou M9, la facture doit contenir exactement deux lignes (BG-25) de sous-type GROUP (cbc:DocumentStatusCode) sans ligne parent (cac:BillingReferenceLine) ; lignes trouvées : %d.", len(refs))
	}
	for _, p := range [...]struct{ party, id, label string }{
		{"AccountingSupplierParty", "BR-FR-BD-02_BT-30", "du VENDEUR (BT-30)"},
		{"AccountingCustomerParty", "BR-FR-BD-02_BT-47", "de l'ACHETEUR (BT-47)"},
	} {
		n := 0
		for _, br := range refs {
			// ../cac:Item/…/cbc:CompanyID = ../../cac:<party>/…/cbc:CompanyID
			if eqNodes(br.Parent, "Item/ManufacturerParty/PartyLegalEntity/CompanyID", nodes(br.Parent.Parent, p.party+"/Party/PartyLegalEntity/CompanyID")) {
				n++
			}
		}
		if n != 1 {
			k.fail(p.id, nil, "%s : lorsque le cadre de facturation (BT-23) est S9, B9 ou M9, exactement une des deux lignes GROUP sans parent doit avoir pour vendeur à la ligne (cac:ManufacturerParty) l'identifiant légal %s ; lignes trouvées : %d.", strings.Replace(p.id, "_", "/", 1), p.label, n)
		}
	}
}

// mv03 checks BR-FR-MV-03: the GROUP lines without parent carry the line
// seller name, legal identifier and country, the line VAT totals and the
// invoiced objects AFL and AVV.
func (k *ublChecker) mv03() {
	if !k.special {
		return
	}
	invCur := nodes(k.root, "DocumentCurrencyCode")
	var accCur []*xmltree.Node // ../..//cbc:TaxCurrencyCode
	k.root.Walk(func(n *xmltree.Node) bool {
		if n.Name == "TaxCurrencyCode" && n != k.root {
			accCur = append(accCur, n)
		}
		return true
	})
	for _, br := range k.groupRefs(k.rootLines()) {
		line := br.Parent
		id := lineNumber(line)
		if v := first(line, "Item/ManufacturerParty/PartyLegalEntity/RegistrationName"); nsv(v) == "" {
			k.fail("BR-FR-MV-03_EXT-FR-FE-164", br, "BR-FR-MV-03/EXT-FR-FE-164 : ligne %q : pour une ligne GROUP sans parent (cadre S8, B8, M8, S9, B9 ou M9), le nom du vendeur à la ligne (cac:ManufacturerParty/cac:PartyLegalEntity/cbc:RegistrationName) doit être renseigné ; valeur actuelle : %q.", id, sv(v))
		}
		if v := first(line, "Item/ManufacturerParty/PartyLegalEntity/CompanyID"); nsv(v) == "" {
			k.fail("BR-FR-MV-03_EXT-FR-FE-167", br, "BR-FR-MV-03/EXT-FR-FE-167 : ligne %q : pour une ligne GROUP sans parent (cadre S8, B8, M8, S9, B9 ou M9), l'identifiant du vendeur à la ligne (cac:ManufacturerParty/cac:PartyLegalEntity/cbc:CompanyID) doit être renseigné ; valeur actuelle : %q.", id, sv(v))
		}
		var country *xmltree.Node // ..//cac:Item/…/cbc:IdentificationCode
		line.Walk(func(n *xmltree.Node) bool {
			if country == nil && n.Name == "Item" && n != line {
				country = first(n, "ManufacturerParty/PostalAddress/Country/IdentificationCode")
			}
			return country == nil
		})
		if nsv(country) == "" {
			k.fail("BR-FR-MV-03_EXT-FR-FE-177", br, "BR-FR-MV-03/EXT-FR-FE-177 : ligne %q : pour une ligne GROUP sans parent (cadre S8, B8, M8, S9, B9 ou M9), le code pays du vendeur à la ligne (cac:ManufacturerParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode) doit être renseigné ; valeur actuelle : %q.", id, sv(country))
		}
		if v := lineTax(line, invCur); nsv(v) == "" {
			k.fail("BR-FR-MV-03_EXT-FR-FE-181", br, "BR-FR-MV-03/EXT-FR-FE-181 : ligne %q : pour une ligne GROUP sans parent, le montant total de TVA de la ligne (cac:TaxTotal/cbc:TaxAmount) doit être renseigné en devise de facture (%q) ; valeur actuelle : %q.", id, sv(first(k.root, "DocumentCurrencyCode")), sv(v))
		}
		if len(accCur) > 0 {
			nonEmpty := false
			for _, c := range accCur {
				if sv(c) != "" {
					nonEmpty = true
				}
			}
			if v := lineTax(line, accCur); !nonEmpty || nsv(v) == "" {
				k.fail("BR-FR-MV-03_EXT-FR-FE-182", br, "BR-FR-MV-03/EXT-FR-FE-182 : ligne %q : pour une ligne GROUP sans parent, le montant total de TVA de la ligne (cac:TaxTotal/cbc:TaxAmount) doit être renseigné en devise de comptabilisation (BT-6 = %q) ; valeur actuelle : %q.", id, sv(accCur[0]), sv(v))
			}
		}
		afl := subInvoiceIDs(line, "AFL")
		var aflNode *xmltree.Node
		if len(afl) > 0 {
			aflNode = afl[0]
		}
		if nsv(aflNode) == "" {
			k.fail("BR-FR-MV-03_BT-128_AFL", br, "BR-FR-MV-03/BT-128 : ligne %q : pour une ligne GROUP sans parent, un objet facturé (BT-128, cac:DocumentReference de type 130) de schéma AFL (numéro de sous-facture) doit être présent et renseigné ; valeur actuelle : %q.", id, sv(aflNode))
		}
		avv := subInvoiceIDs(line, "AVV")
		var avvNode *xmltree.Node
		if len(avv) > 0 {
			avvNode = avv[0]
		}
		frame := false
		for _, n := range avv {
			if in(sv(n), "M8", "S8", "B8") {
				frame = true
			}
		}
		if nsv(avvNode) == "" || frame {
			k.fail("BR-FR-MV-03_BT-128_AVV", br, "BR-FR-MV-03/BT-128 : ligne %q : pour une ligne GROUP sans parent, un objet facturé (BT-128, cac:DocumentReference de type 130) de schéma AVV doit être présent, renseigné et différent de M8, S8, B8 ; valeur actuelle : %q.", id, sv(avvNode))
		}
	}
}

// childRefs yields the lines among lines that have a billing reference
// accepted by keep.
func childLines(lines []*xmltree.Node, keep func(br *xmltree.Node) bool) []*xmltree.Node {
	var out []*xmltree.Node
	for _, line := range lines {
		for br := range all(line, "BillingReference") {
			if keep(br) {
				out = append(out, line)
				break
			}
		}
	}
	return out
}

// mv05 checks BR-FR-MV-05: the net total of a GROUP line without parent
// equals the sum of the net totals of its children other than INFORMATION,
// within 0.01 per child. As in the schematron, the sum is computed in double
// precision.
func (k *ublChecker) mv05() {
	if !k.special {
		return
	}
	for _, br := range k.groupRefs(k.rootLines()) {
		group := br.Parent
		gid := nsv(first(group, "ID"))
		children := childLines(k.anyLines(), func(c *xmltree.Node) bool {
			return k.selfRef(c) && anyNe(c, "InvoiceDocumentReference/DocumentStatusCode", "INFORMATION") && anyEq(c, "BillingReferenceLine/ID", gid)
		})
		sum, n, sumOK := 0.0, 0, true
		for _, child := range children {
			for amt := range all(child, "LineExtensionAmount") {
				f, ok := float(amt)
				if !ok {
					sumOK = false
				}
				sum += f
				n++
			}
		}
		totalNode := first(group, "LineExtensionAmount")
		total, ok := decimal(totalNode)
		if n == 0 {
			// sum(()) is the integer 0: the comparison stays decimal.
			ok = ok && total.IsZero()
		} else {
			ok = ok && sumOK && math.Abs(toFloat(total)-sum) <= tolerance(n)
		}
		if !ok {
			k.fail("BR-FR-MV-05_EXT-FR-FE-BG-12", br, "BR-FR-MV-05/EXT-FR-FE-BG-12 : ligne GROUP %q : le total HT de la ligne GROUP (BT-131, cbc:LineExtensionAmount) doit être égal, à 0,01 près par sous-ligne, à la somme des totaux HT de ses sous-lignes ; total : %q, somme des sous-lignes : %s, nombre de sous-lignes : %d.", gid, sv(totalNode), strconv.FormatFloat(sum, 'g', -1, 64), n)
		}
	}
}

// parentIDs returns the parent line identifiers of line
// (cac:BillingReference[…/cbc:ID = $invoiceID]/cac:BillingReferenceLine/cbc:ID).
func (k *ublChecker) parentIDs(line *xmltree.Node) []*xmltree.Node {
	var out []*xmltree.Node
	for br := range all(line, "BillingReference") {
		if k.selfRef(br) {
			out = append(out, nodes(br, "BillingReferenceLine/ID")...)
		}
	}
	return out
}

// parentValue returns value applied to the first parent line of line that
// gives a non-nil node: the lines of the root whose cbc:ID is one of the
// parent identifiers ((../cac:InvoiceLine | ../cac:CreditNoteLine)[cbc:ID =
// $parentlineID]/path).
func (k *ublChecker) parentValue(parents []*xmltree.Node, value func(*xmltree.Node) *xmltree.Node) string {
	for _, l := range k.anyLines() {
		if !eqNodes(l, "ID", parents) {
			continue
		}
		if v := value(l); v != nil {
			return nsv(v)
		}
	}
	return ""
}

// mv06 checks BR-FR-MV-06: every line has a line seller legal identifier,
// equal to the one of its parent line.
func (k *ublChecker) mv06() {
	if !k.special {
		return
	}
	for line := range k.lines() {
		legal := lineSellerID(line)
		parents := k.parentIDs(line)
		parentLegal := k.parentValue(parents, func(l *xmltree.Node) *xmltree.Node {
			return first(l, "Item/ManufacturerParty/PartyLegalEntity/CompanyID")
		})
		if legal == "" || len(parents) > 0 && legal != parentLegal {
			k.fail("BR-FR-MV-06_EXT-FR-FE-167", line, "BR-FR-MV-06/EXT-FR-FE-167 : ligne %q (ligne parent %q) : chaque ligne doit contenir l'identifiant légal du vendeur à la ligne (cac:ManufacturerParty/cac:PartyLegalEntity/cbc:CompanyID), identique à celui de la ligne parent ; identifiant : %q, identifiant de la ligne parent : %q.", lineNumber(line), firstValue(parents), legal, parentLegal)
		}
	}
}

// firstValue returns the string value of the first node of ns, or "".
func firstValue(ns []*xmltree.Node) string {
	if len(ns) == 0 {
		return ""
	}
	return sv(ns[0])
}

// mv07 checks BR-FR-MV-07: every line has a sub-invoice number (AFL, type
// 130), equal to the one of its parent line.
func (k *ublChecker) mv07() {
	if !k.special {
		return
	}
	for line := range k.lines() {
		num := subInvoice(line)
		parents := k.parentIDs(line)
		parentNum := k.parentValue(parents, func(l *xmltree.Node) *xmltree.Node {
			if ids := subInvoiceIDs(l, "AFL"); len(ids) > 0 {
				return ids[0]
			}
			return nil
		})
		if num == "" || len(parents) > 0 && num != parentNum {
			k.fail("BR-FR-MV-07_BT-128", line, "BR-FR-MV-07/BT-128 : ligne %q (ligne parent %q) : chaque ligne doit contenir un numéro de facture codifié AFL (BT-128, cac:DocumentReference de type 130), identique à celui de la ligne parent ; numéro : %q, numéro de la ligne parent : %q.", lineNumber(line), firstValue(parents), num, parentNum)
		}
	}
}

// mv08 checks BR-FR-MV-08: the VAT exemption reason of each DETAIL line
// starts with its sub-invoice number between #.
func (k *ublChecker) mv08() {
	if !k.special {
		return
	}
	for line := range k.lines() {
		detail := false
		for br := range all(line, "BillingReference") {
			if k.selfRef(br) && anyEq(br, "InvoiceDocumentReference/DocumentStatusCode", "DETAIL") {
				detail = true
			}
		}
		if !detail {
			continue
		}
		num := subInvoice(line)
		reason := first(line, "Item/ClassifiedTaxCategory/TaxExemptionReason")
		if nsv(reason) == "" || num == "" || !strings.HasPrefix(sv(reason), "#"+num+"#") {
			k.fail("BR-FR-MV-08_BT-128", line, "BR-FR-MV-08/BT-128 : ligne %q : la raison d'exemption de TVA (cbc:TaxExemptionReason) de chaque ligne DETAIL doit commencer par le numéro de sous-facture de la ligne entre # ; raison : %q, numéro de facture de la ligne : %q.", lineNumber(line), sv(reason), num)
		}
	}
}

// mv09 checks BR-FR-MV-09: the VAT total of a GROUP line without parent
// equals, within 0.01, the sum of the VAT breakdowns (BT-117) whose
// exemption reason starts with its sub-invoice number between #. The sum is
// computed in double precision.
func (k *ublChecker) mv09() {
	if !k.special {
		return
	}
	invCur := nodes(k.root, "DocumentCurrencyCode")
	for _, br := range k.groupRefs(k.rootLines()) {
		group := br.Parent
		num := subInvoice(group)
		prefix := "#" + num + "#"
		sum, sumOK, terms := 0.0, true, 0
		for sub := range all(k.root, "TaxTotal/TaxSubtotal") {
			if !anyEq(sub, "TaxCategory/TaxScheme/ID", "VAT") || !strings.HasPrefix(sv(first(sub, "TaxCategory/TaxExemptionReason")), prefix) {
				continue
			}
			for amt := range all(sub, "TaxAmount") {
				f, ok := float(amt)
				if !ok {
					sumOK = false
				}
				sum += f
				terms++
			}
		}
		totalNode := lineTax(group, invCur)
		total, ok := decimal(totalNode)
		if terms == 0 {
			// sum(()) is the integer 0: the comparison stays decimal.
			ok = ok && total.Abs().Cmp(invoice.NewDecimal(1, 2)) <= 0
		} else {
			ok = ok && sumOK && math.Abs(toFloat(total)-sum) <= 0.01
		}
		if !ok {
			k.fail("BR-FR-MV-09_EXT-FR-FE-181", br, "BR-FR-MV-09/EXT-FR-FE-181 : ligne GROUP %q, numéro de facture %q : le montant total de TVA de la ligne GROUP (EXT-FR-FE-181, cac:TaxTotal/cbc:TaxAmount) doit être égal à la somme des montants de TVA des ventilations (BT-117) dont la raison d'exemption commence par le numéro de facture entre # ; total TVA : %q, somme : %s.", lineNumber(group), num, sv(totalNode), strconv.FormatFloat(sum, 'g', -1, 64))
		}
	}
}

// mv10 checks BR-FR-MV-10: when a GROUP line without parent gives its gross
// total (EXT-FR-FE-184, cbc:TaxInclusiveLineExtensionAmount), it equals the
// net total plus the VAT total within 0.01 per DETAIL or GROUP child
// reference.
func (k *ublChecker) mv10() {
	if !k.special {
		return
	}
	invCur := nodes(k.root, "DocumentCurrencyCode")
	for _, br := range k.groupRefs(k.rootLines()) {
		group := br.Parent
		grandNode := first(group, "TaxInclusiveLineExtensionAmount")
		if grandNode == nil {
			continue
		}
		ids := nodes(group, "ID")
		n := 0
		for _, l := range k.anyLines() {
			for c := range all(l, "BillingReference") {
				if k.selfRef(c) && anyIn(c, "InvoiceDocumentReference/DocumentStatusCode", "DETAIL", "GROUP") && eqNodes(c, "BillingReferenceLine/ID", ids) {
					n++
				}
			}
		}
		netNode := first(group, "LineExtensionAmount")
		taxNode := lineTax(group, invCur)
		grand, ok1 := decimal(grandNode)
		net, ok2 := decimal(netNode)
		tax, ok3 := decimal(taxNode)
		ok := nsv(grandNode) != "" && ok1 && ok2 && ok3 &&
			grand.Sub(net).Sub(tax).Abs().Cmp(invoice.NewDecimal(int64(n), 2)) <= 0
		if !ok {
			k.fail("BR-FR-MV-10_EXT-FR-FE-184", br, "BR-FR-MV-10/EXT-FR-FE-184 : ligne GROUP %q : le montant total TTC de la ligne GROUP (cbc:TaxInclusiveLineExtensionAmount) doit être égal à la somme du total HT (cbc:LineExtensionAmount) et du total TVA (cac:TaxTotal/cbc:TaxAmount), à 0,01 près par sous-ligne DETAIL ou GROUP ; TTC : %q, HT : %q, TVA : %q, sous-lignes : %d.", lineNumber(group), sv(grandNode), sv(netNode), sv(taxNode), n)
		}
	}
}

// mv11 checks BR-FR-MV-11: the sub-invoice of the main seller (line seller
// legal identifier equal to BT-30) is numbered with the invoice number BT-1.
func (k *ublChecker) mv11() {
	sellerIDs := nodes(k.root, "AccountingSupplierParty/Party/PartyLegalEntity/CompanyID")
	sellerLine := func(line *xmltree.Node) bool { return strIn(lineSellerID(line), sellerIDs) }
	// $nbSubinvoiceSeller counts the lines of the main seller with a GROUP
	// reference without parent to the invoice and a sub-invoice number BT-1.
	nb := 0
	for line := range k.lines() {
		if !sellerLine(line) || len(k.groupRefs([]*xmltree.Node{line})) == 0 {
			continue
		}
		for _, id := range subInvoiceIDs(line, "AFL") {
			if strIn(sv(id), k.invIDs) {
				nb++
				break
			}
		}
	}
	if !k.special {
		return
	}
	for line := range k.lines() {
		if !sellerLine(line) {
			continue
		}
		num := subInvoice(line)
		numNotInv := false // $numFactLine != $invoiceID
		for _, id := range k.invIDs {
			if num != sv(id) {
				numNotInv = true
			}
		}
		for _, br := range k.groupRefs([]*xmltree.Node{line}) {
			if nb == 1 || nb == 0 && strIn(num, k.invIDs) || nb > 1 && numNotInv {
				continue
			}
			k.fail("BR-FR-MV-11_BT-128", br, "BR-FR-MV-11/BT-128 : ligne GROUP %q du vendeur principal (identifiant légal %q) : le vendeur principal doit avoir une et une seule ligne GROUP sans parent dont le numéro de facture à la ligne (BT-128, schéma AFL) est égal au numéro de facture (BT-1 = %q) ; lignes GROUP numérotées BT-1 : %d, numéro de la ligne : %q.", lineNumber(line), firstValue(sellerIDs), sv(first(k.root, "ID")), nb, num)
		}
	}
}

// mv12 checks BR-FR-MV-12: the sub-invoice numbers of the GROUP lines
// without parent are unique. As in the schematron, a number is compared with
// the AFL identifiers of every document reference of type 130 that precedes
// it in the document.
func (k *ublChecker) mv12() {
	if !k.special {
		return
	}
	targets := map[*xmltree.Node]bool{}
	for _, line := range k.anyLines() {
		if len(k.groupRefs([]*xmltree.Node{line})) > 0 {
			for _, id := range subInvoiceIDs(line, "AFL") {
				targets[id] = true
			}
		}
	}
	if len(targets) == 0 {
		return
	}
	// Walk the document in order; a document reference counts as preceding
	// once its subtree is done (preceding:: excludes the ancestors).
	var dup *xmltree.Node
	seen := map[string]bool{}
	var visit func(n *xmltree.Node)
	visit = func(n *xmltree.Node) {
		if dup != nil {
			return
		}
		if targets[n] && seen[sv(n)] {
			dup = n
			return
		}
		for _, c := range n.Children {
			visit(c)
		}
		if n.Name == "DocumentReference" && anyEq(n, "DocumentTypeCode", "130") {
			for _, id := range withAttr(n, "ID", "schemeID", "AFL") {
				seen[sv(id)] = true
			}
		}
	}
	visit(k.root)
	if dup != nil {
		k.fail("BR-FR-MV-12_BT-128", nil, "BR-FR-MV-12/BT-128 : lorsque le cadre de facturation (BT-23) est S8, B8, M8, S9, B9 ou M9, les numéros de facture à la ligne (BT-128, cac:DocumentReference de type 130 et de schéma AFL) des lignes GROUP sans parent doivent être uniques ; numéro répété : %q.", sv(dup))
	}
}

// selfBilled are the self-billed document types of BR-FR-MV-13 and
// BR-FR-BD-13.
var selfBilled = []string{"389", "261", "501", "500", "502", "471", "473"}

// mv13 checks BR-FR-MV-13: a multi-vendor invoice is not self-billed.
func (k *ublChecker) mv13() {
	if !k.mv {
		return
	}
	for n := range all(k.root, k.typeCode) {
		if s := nsv(n); s == "" || in(s, selfBilled...) {
			k.fail("BR-FR-MV-13_BT-3", n, "BR-FR-MV-13/BT-3 : lorsque le cadre de facturation (BT-23) est S8, B8 ou M8, le code type de facture (BT-3, cbc:%s) ne doit pas être un type auto-facturé (389, 261, 501, 500, 502, 471, 473) ; valeur actuelle : %q.", k.typeCode, sv(n))
		}
	}
}

// bd13 checks the pattern BR-FR-BD-13, whose assertion is identified as
// BR-FR-MV-BD_BT-3 in the UBL schematron: a two-way invoice is self-billed.
func (k *ublChecker) bd13() {
	if !k.bd {
		return
	}
	for n := range all(k.root, k.typeCode) {
		if s := nsv(n); s == "" || !in(s, selfBilled...) {
			k.fail("BR-FR-MV-BD_BT-3", n, "BR-FR-BD-13/BT-3 : lorsque le cadre de facturation (BT-23) est S9, B9 ou M9, le code type de facture (BT-3, cbc:%s) doit être un type auto-facturé (389, 261, 501, 500, 502, 471, 473) ; valeur actuelle : %q.", k.typeCode, sv(n))
		}
	}
}

// precedingInvoice checks BR-FR-MV-14 (frames S8, B8, M8) and BR-FR-BD-14
// (S9, B9, M9): for the given document types, the GROUP lines without
// parent also reference a preceding invoice (EXT-FR-FE-136) with its date
// (EXT-FR-FE-138).
func (k *ublChecker) precedingInvoice(frame bool, base, frames string, types ...string) {
	if !frame || !k.typeCodeIn(types...) {
		return
	}
	for _, br := range k.groupRefs(k.anyLines()) {
		line := br.Parent
		id, date := false, false
		for other := range all(line, "BillingReference") {
			if !neNodes(other, "InvoiceDocumentReference/ID", k.invIDs) {
				continue
			}
			id = id || exists(other, "InvoiceDocumentReference/ID")
			date = date || exists(other, "InvoiceDocumentReference/IssueDate")
		}
		if !id {
			k.fail(base+"-EXT-FR-FE-136", br, "%s-EXT-FR-FE-136 : numéro de facture antérieure manquant ligne %q : lorsque le cadre de facturation (BT-23) est %s, pour les factures rectificatives et les avoirs, chaque ligne GROUP sans parent doit référencer une facture antérieure (EXT-FR-FE-136, cac:BillingReference/cac:InvoiceDocumentReference/cbc:ID) avec sa date (EXT-FR-FE-138).", base, lineNumber(line), frames)
		}
		if !date {
			k.fail(base+"-EXT-FR-FE-138", br, "%s-EXT-FR-FE-138 : date de facture antérieure manquante ligne %q : lorsque le cadre de facturation (BT-23) est %s, pour les factures rectificatives et les avoirs, chaque ligne GROUP sans parent doit référencer une facture antérieure (EXT-FR-FE-136) avec sa date (EXT-FR-FE-138, cac:BillingReference/cac:InvoiceDocumentReference/cbc:IssueDate).", base, lineNumber(line), frames)
		}
	}
}
