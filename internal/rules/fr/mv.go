package fr

import (
	"iter"
	"math"
	"strconv"
	"strings"

	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

// The BR-FR-MV-* (multi-vendor, BT-23 = S8, B8, M8) and BR-FR-BD-* (two-way,
// BT-23 = S9, B9, M9) rules describe an invoice made of sub-invoices: each
// sub-invoice is a line of sub-type GROUP without parent line
// (EXT-FR-FE-163/162), whose children (sub-type DETAIL, GROUP or
// INFORMATION) point to it with ParentLineID. They only apply to the
// EXTENDED-CTC-FR profile.

// isGroupRoot reports whether line is a GROUP line without parent line.
func isGroupRoot(line *xmltree.Node) bool {
	return anyEq(line, pLineStatus, "GROUP") && !exists(line, pParentLine)
}

// lineID returns the string value of the first LineID of line, for messages.
func lineID(line *xmltree.Node) string { return sv(first(line, pLineID)) }

// subInvoiceNumber returns the normalized identifier of the first line
// document AFL of type 130 (BT-128): the sub-invoice number.
func subInvoiceNumber(line *xmltree.Node) string {
	for doc := range all(line, pLineARD) {
		if anyEq(doc, "ReferenceTypeCode", "AFL") && anyEq(doc, "TypeCode", "130") {
			if id := first(doc, "IssuerAssignedID"); id != nil {
				return nsv(id)
			}
		}
	}
	return ""
}

// subInvoiceNumberNodes returns the identifiers of every line document AFL
// of type 130 of line.
func subInvoiceNumberNodes(line *xmltree.Node) []*xmltree.Node {
	var out []*xmltree.Node
	for doc := range all(line, pLineARD) {
		if anyEq(doc, "ReferenceTypeCode", "AFL") && anyEq(doc, "TypeCode", "130") {
			for id := range all(doc, "IssuerAssignedID") {
				out = append(out, id)
			}
		}
	}
	return out
}

// lineTaxTotal returns the first line VAT total (EXT-FR-FE-181/182) whose
// currency is one of the string values of currencies.
func lineTaxTotal(line *xmltree.Node, currencies []*xmltree.Node) *xmltree.Node {
	for t := range all(line, pLineSum+"/TaxTotalAmount") {
		cur, ok := t.LookupAttr("currencyID")
		if !ok {
			continue
		}
		for _, c := range currencies {
			if sv(c) == cur {
				return t
			}
		}
	}
	return nil
}

// nodes collects the elements matching path.
func nodes(n *xmltree.Node, path string) []*xmltree.Node {
	var out []*xmltree.Node
	for c := range all(n, path) {
		out = append(out, c)
	}
	return out
}

// multiVendor runs the BR-FR-MV-* and BR-FR-BD-* rules.
func (k *checker) multiVendor() {
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
	k.mv14()
	k.bd14()
}

// mv01 checks BR-FR-MV-01: in the frames S8, B8, M8, S9, B9, M9 the line
// sub-types are not empty.
func (k *checker) mv01() {
	if !k.special {
		return
	}
	for n := range all(k.root, pLines+"/"+pLineStatus) {
		if nsv(n) == "" {
			k.fail("BR-FR-MV-01_EXT-FR-FE-163", n, "BR-FR-MV-01/EXT-FR-FE-163 : lorsque le cadre de facturation (BT-23) est S8, B8, M8, S9, B9 ou M9, chaque ligne (BG-25) doit contenir un sous-type de ligne (ram:LineStatusReasonCode) renseigné ; valeur actuelle : %q.", sv(n))
		}
	}
}

// countGroupRoots counts the GROUP lines without parent of a transaction
// for which keep returns true (keep may be nil).
func countGroupRoots(tx *xmltree.Node, keep func(*xmltree.Node) bool) int {
	n := 0
	for line := range all(tx, "IncludedSupplyChainTradeLineItem") {
		if isGroupRoot(line) && (keep == nil || keep(line)) {
			n++
		}
	}
	return n
}

// mv02 checks BR-FR-MV-02: a multi-vendor invoice has at least one GROUP line
// without parent.
func (k *checker) mv02() {
	if !k.mv {
		return
	}
	for tx := range all(k.root, pTransaction) {
		if countGroupRoots(tx, nil) < 1 {
			k.fail("BR-FR-MV-02_EXT-FR-FE-163", tx, "BR-FR-MV-02/EXT-FR-FE-163 : lorsque le cadre de facturation (BT-23) est S8, B8 ou M8, la facture doit contenir au moins une ligne (BG-25) de sous-type GROUP sans identifiant de ligne parent (ram:ParentLineID).")
		}
	}
}

// bd02 checks BR-FR-BD-02: a two-way invoice has exactly two GROUP lines
// without parent, one for the seller and one for the buyer.
func (k *checker) bd02() {
	if !k.bd {
		return
	}
	for tx := range all(k.root, pTransaction) {
		if n := countGroupRoots(tx, nil); n != 2 {
			k.fail("BR-FR-BD-02_EXT-FR-FE-163", tx, "BR-FR-BD-02/EXT-FR-FE-163 : lorsque le cadre de facturation (BT-23) est S9, B9 ou M9, la facture doit contenir exactement deux lignes (BG-25) de sous-type GROUP sans identifiant de ligne parent (ram:ParentLineID) ; lignes trouvées : %d.", n)
		}
		for _, p := range [...]struct{ party, id, label string }{
			{"ApplicableHeaderTradeAgreement/SellerTradeParty", "BR-FR-BD-02_BT-31", "du VENDEUR (BT-30)"},
			{"ApplicableHeaderTradeAgreement/BuyerTradeParty", "BR-FR-BD-02_BT-47", "de l'ACHETEUR (BT-47)"},
		} {
			ids := nodes(tx, p.party+"/"+pLegalID)
			n := countGroupRoots(tx, func(line *xmltree.Node) bool {
				for lid := range all(line, pItemSeller+"/"+pLegalID) {
					for _, id := range ids {
						if sv(lid) == sv(id) {
							return true
						}
					}
				}
				return false
			})
			if n != 1 {
				k.fail(p.id, tx, "%s : lorsque le cadre de facturation (BT-23) est S9, B9 ou M9, exactement une des deux lignes GROUP sans parent doit avoir pour identifiant légal du vendeur à la ligne (EXT-FR-FE-167) l'identifiant légal %s ; lignes trouvées : %d.", strings.Replace(p.id, "_", "/", 1), p.label, n)
			}
		}
	}
}

// mv03 checks BR-FR-MV-03: the GROUP lines without parent carry the line
// seller name, legal identifier and country, the line VAT totals and valid
// invoiced objects AFL and AVV.
func (k *checker) mv03() {
	if !k.special {
		return
	}
	for tx := range all(k.root, pTransaction) {
		invCur := nodes(tx, "ApplicableHeaderTradeSettlement/InvoiceCurrencyCode")
		accCur := nodes(tx, "ApplicableHeaderTradeSettlement/TaxCurrencyCode")
		for line := range all(tx, "IncludedSupplyChainTradeLineItem") {
			if !isGroupRoot(line) {
				continue
			}
			id := lineID(line)
			if v := first(line, pItemSeller+"/Name"); nsv(v) == "" {
				k.fail("BR-FR-MV-03_EXT-FR-FE-164", orNode(v, line), "BR-FR-MV-03/EXT-FR-FE-164 : ligne %q : pour une ligne GROUP sans parent (cadre S8, B8, M8, S9, B9 ou M9), le nom du vendeur à la ligne (ram:Name) doit être renseigné ; valeur actuelle : %q.", id, sv(v))
			}
			if v := first(line, pItemSeller+"/"+pLegalID); nsv(v) == "" {
				k.fail("BR-FR-MV-03_EXT-FR-FE-167", orNode(v, line), "BR-FR-MV-03/EXT-FR-FE-167 : ligne %q : pour une ligne GROUP sans parent (cadre S8, B8, M8, S9, B9 ou M9), l'identifiant légal du vendeur à la ligne (ram:ID) doit être renseigné ; valeur actuelle : %q.", id, sv(v))
			}
			if v := first(line, pItemSeller+"/PostalTradeAddress/CountryID"); nsv(v) == "" {
				k.fail("BR-FR-MV-03_EXT-FR-FE-177", orNode(v, line), "BR-FR-MV-03/EXT-FR-FE-177 : ligne %q : pour une ligne GROUP sans parent (cadre S8, B8, M8, S9, B9 ou M9), le code pays du vendeur à la ligne (ram:CountryID) doit être renseigné ; valeur actuelle : %q.", id, sv(v))
			}
			if v := lineTaxTotal(line, invCur); nsv(v) == "" {
				k.fail("BR-FR-MV-03_EXT-FR-FE-181", orNode(v, line), "BR-FR-MV-03/EXT-FR-FE-181 : ligne %q : pour une ligne GROUP sans parent, le montant total de TVA (ram:TaxTotalAmount) doit être renseigné en devise de facture (%q) ; valeur actuelle : %q.", id, sv(first(tx, "ApplicableHeaderTradeSettlement/InvoiceCurrencyCode")), sv(v))
			}
			if len(accCur) > 0 {
				nonEmpty := false
				for _, c := range accCur {
					if sv(c) != "" {
						nonEmpty = true
					}
				}
				if v := lineTaxTotal(line, accCur); !nonEmpty || nsv(v) == "" {
					k.fail("BR-FR-MV-03_EXT-FR-FE-182", orNode(v, line), "BR-FR-MV-03/EXT-FR-FE-182 : ligne %q : pour une ligne GROUP sans parent, le montant total de TVA (ram:TaxTotalAmount) doit être renseigné en devise de comptabilisation (BT-6 = %q) ; valeur actuelle : %q.", id, sv(accCur[0]), sv(v))
				}
			}
		}
		for line := range all(tx, "IncludedSupplyChainTradeLineItem") {
			if !isGroupRoot(line) {
				continue
			}
			for doc := range all(line, pLineARD) {
				switch {
				case anyEq(doc, "ReferenceTypeCode", "AFL"):
					if !anyEq(doc, "TypeCode", "130") || nsv(first(doc, "IssuerAssignedID")) == "" {
						k.fail("BR-FR-MV-03_BT-128_AFL", doc, "BR-FR-MV-03/BT-128 : ligne %q : pour une ligne GROUP sans parent, l'objet facturé de schéma AFL doit être de type 130 avec un identifiant (ram:IssuerAssignedID) renseigné ; type : %q, valeur actuelle : %q.", lineID(line), sv(first(doc, "TypeCode")), sv(first(doc, "IssuerAssignedID")))
					}
				case anyEq(doc, "ReferenceTypeCode", "AVV"):
					if !anyEq(doc, "TypeCode", "130") || nsv(first(doc, "IssuerAssignedID")) == "" || anyIn(doc, "IssuerAssignedID", "M8", "S8", "B8") {
						k.fail("BR-FR-MV-03_BT-128_AVV", doc, "BR-FR-MV-03/BT-128 : ligne %q : pour une ligne GROUP sans parent, l'objet facturé de schéma AVV doit être de type 130 avec un identifiant (ram:IssuerAssignedID) renseigné et différent de M8, S8, B8 ; type : %q, valeur actuelle : %q.", lineID(line), sv(first(doc, "TypeCode")), sv(first(doc, "IssuerAssignedID")))
					}
				}
			}
		}
	}
}

// tolerance returns 0.01 × n as an xs:decimal converted to xs:double.
func tolerance(n int) float64 {
	f, _ := strconv.ParseFloat(strconv.Itoa(n)+"e-2", 64)
	return f
}

// mv05 checks BR-FR-MV-05: the net total of a GROUP line without parent
// equals the sum of the net totals of its children other than INFORMATION,
// within 0.01 per child. As in the schematron, the sum is computed in double
// precision.
func (k *checker) mv05() {
	if !k.special {
		return
	}
	for tx := range all(k.root, pTransaction) {
		for group := range all(tx, "IncludedSupplyChainTradeLineItem") {
			if !isGroupRoot(group) {
				continue
			}
			gid := nsv(first(group, pLineID))
			sum, n, sumOK := 0.0, 0, true
			for child := range all(tx, "IncludedSupplyChainTradeLineItem") {
				if !anyEq(child, pParentLine, gid) || !anyNe(child, pLineStatus, "INFORMATION") {
					continue
				}
				for amt := range all(child, pLineSum+"/LineTotalAmount") {
					f, ok := float(amt)
					if !ok {
						sumOK = false
					}
					sum += f
					n++
				}
			}
			totalNode := first(group, pLineSum+"/LineTotalAmount")
			total, ok := decimal(totalNode)
			if n == 0 {
				// sum(()) is the integer 0: the comparison stays decimal.
				ok = ok && total.IsZero()
			} else {
				ok = ok && sumOK && math.Abs(toFloat(total)-sum) <= tolerance(n)
			}
			if !ok {
				k.fail("BR-FR-MV-05_EXT-FR-FE-BG-12", orNode(totalNode, group), "BR-FR-MV-05/EXT-FR-FE-BG-12 : ligne GROUP %q : le total HT de la ligne GROUP (BT-131) doit être égal, à 0,01 près par sous-ligne, à la somme des totaux HT de ses sous-lignes ; total : %q, somme des sous-lignes : %s, nombre de sous-lignes : %d.", gid, sv(totalNode), strconv.FormatFloat(sum, 'g', -1, 64), n)
			}
		}
	}
}

// parentLines yields the lines of the transaction whose LineID equals one of
// the ParentLineID of line (../ram:IncludedSupplyChainTradeLineItem[LineID =
// $parentlineID]), in document order.
func parentLines(tx, line *xmltree.Node) iter.Seq[*xmltree.Node] {
	return func(yield func(*xmltree.Node) bool) {
		parents := nodes(line, pParentLine)
		if len(parents) == 0 {
			return
		}
		for l := range all(tx, "IncludedSupplyChainTradeLineItem") {
			match := false
			for id := range all(l, pLineID) {
				for _, p := range parents {
					if sv(id) == sv(p) {
						match = true
					}
				}
			}
			if match && !yield(l) {
				return
			}
		}
	}
}

// mv06 checks BR-FR-MV-06: every line has a line seller legal identifier,
// equal to the one of its parent line.
func (k *checker) mv06() {
	if !k.special {
		return
	}
	for tx := range all(k.root, pTransaction) {
		for line := range all(tx, "IncludedSupplyChainTradeLineItem") {
			legal := nsv(first(line, pItemSeller+"/"+pLegalID))
			hasParent := exists(line, pParentLine)
			parentLegal := ""
			for p := range parentLines(tx, line) {
				if id := first(p, pItemSeller+"/"+pLegalID); id != nil {
					parentLegal = nsv(id)
					break
				}
			}
			if legal == "" || hasParent && legal != parentLegal {
				k.fail("BR-FR-MV-06_EXT-FR-FE-167", line, "BR-FR-MV-06/EXT-FR-FE-167 : ligne %q (ligne parent %q) : chaque ligne doit contenir l'identifiant légal du vendeur à la ligne, identique à celui de la ligne parent ; identifiant : %q, identifiant de la ligne parent : %q.", lineID(line), sv(first(line, pParentLine)), legal, parentLegal)
			}
		}
	}
}

// mv07 checks BR-FR-MV-07: every line has a sub-invoice number (AFL, type
// 130), equal to the one of its parent line.
func (k *checker) mv07() {
	if !k.special {
		return
	}
	for tx := range all(k.root, pTransaction) {
		for line := range all(tx, "IncludedSupplyChainTradeLineItem") {
			num := subInvoiceNumber(line)
			hasParent := exists(line, pParentLine)
			parentNum := ""
			for p := range parentLines(tx, line) {
				if ids := subInvoiceNumberNodes(p); len(ids) > 0 {
					parentNum = nsv(ids[0])
					break
				}
			}
			if num == "" || hasParent && num != parentNum {
				k.fail("BR-FR-MV-07_BT-128", line, "BR-FR-MV-07/BT-128 : ligne %q (ligne parent %q) : chaque ligne doit contenir un numéro de facture codifié AFL (type 130), identique à celui de la ligne parent ; numéro : %q, numéro de la ligne parent : %q.", lineID(line), sv(first(line, pParentLine)), num, parentNum)
			}
		}
	}
}

// mv08 checks BR-FR-MV-08: the VAT exemption reason of each DETAIL line
// starts with its sub-invoice number between #.
func (k *checker) mv08() {
	if !k.special {
		return
	}
	for line := range all(k.root, pLines) {
		if !anyEq(line, pLineStatus, "DETAIL") {
			continue
		}
		num := subInvoiceNumber(line)
		reason := first(line, "SpecifiedLineTradeSettlement/ApplicableTradeTax/ExemptionReason")
		if nsv(reason) == "" || num == "" || !strings.HasPrefix(sv(reason), "#"+num+"#") {
			k.fail("BR-FR-MV-08_BT-128", orNode(reason, line), "BR-FR-MV-08/BT-128 : ligne %q : la raison d'exemption de TVA de chaque ligne DETAIL doit commencer par le numéro de sous-facture de la ligne entre # ; raison : %q, numéro de facture de la ligne : %q.", lineID(line), sv(reason), num)
		}
	}
}

// mv09 checks BR-FR-MV-09: the VAT total of a GROUP line without parent
// equals, within 0.01, the sum of the VAT breakdowns (BT-117) whose
// exemption reason starts with its sub-invoice number between #. The sum is
// computed in double precision.
func (k *checker) mv09() {
	if !k.special {
		return
	}
	for tx := range all(k.root, pTransaction) {
		invCur := nodes(tx, "ApplicableHeaderTradeSettlement/InvoiceCurrencyCode")
		for group := range all(tx, "IncludedSupplyChainTradeLineItem") {
			if !isGroupRoot(group) {
				continue
			}
			num := subInvoiceNumber(group)
			prefix := "#" + num + "#"
			sum, sumOK, terms := 0.0, true, 0
			for tax := range all(tx, "ApplicableHeaderTradeSettlement/ApplicableTradeTax") {
				if !anyEq(tax, "TypeCode", "VAT") || !strings.HasPrefix(sv(first(tax, "ExemptionReason")), prefix) {
					continue
				}
				for amt := range all(tax, "CalculatedAmount") {
					f, ok := float(amt)
					if !ok {
						sumOK = false
					}
					sum += f
					terms++
				}
			}
			totalNode := lineTaxTotal(group, invCur)
			total, ok := decimal(totalNode)
			if terms == 0 {
				// sum(()) is the integer 0: the comparison stays decimal.
				ok = ok && total.Abs().Cmp(invoice.NewDecimal(1, 2)) <= 0
			} else {
				ok = ok && sumOK && math.Abs(toFloat(total)-sum) <= 0.01
			}
			if !ok {
				k.fail("BR-FR-MV-09_EXT-FR-FE-181", orNode(totalNode, group), "BR-FR-MV-09/EXT-FR-FE-181 : ligne GROUP %q, numéro de facture %q : le montant total de TVA de la ligne GROUP (EXT-FR-FE-181) doit être égal à la somme des montants de TVA des ventilations (BT-117) dont la raison d'exemption commence par le numéro de facture entre # ; total TVA : %q, somme : %s.", lineID(group), num, sv(totalNode), strconv.FormatFloat(sum, 'g', -1, 64))
			}
		}
	}
}

// mv10 checks BR-FR-MV-10: when a GROUP line without parent gives its gross
// total (EXT-FR-FE-184), it equals the net total plus the VAT total within
// 0.01 per DETAIL or GROUP child.
func (k *checker) mv10() {
	if !k.special {
		return
	}
	for tx := range all(k.root, pTransaction) {
		invCur := nodes(tx, "ApplicableHeaderTradeSettlement/InvoiceCurrencyCode")
		for group := range all(tx, "IncludedSupplyChainTradeLineItem") {
			if !isGroupRoot(group) {
				continue
			}
			grandNode := first(group, pLineSum+"/GrandTotalAmount")
			if grandNode == nil {
				continue
			}
			ids := nodes(group, pLineID)
			n := 0
			for child := range all(k.root, pLines) {
				if !anyIn(child, pLineStatus, "DETAIL", "GROUP") {
					continue
				}
				match := false
				for p := range all(child, pParentLine) {
					for _, id := range ids {
						if sv(p) == sv(id) {
							match = true
						}
					}
				}
				if match {
					n++
				}
			}
			netNode := first(group, pLineSum+"/LineTotalAmount")
			taxNode := lineTaxTotal(group, invCur)
			grand, ok1 := decimal(grandNode)
			net, ok2 := decimal(netNode)
			tax, ok3 := decimal(taxNode)
			ok := nsv(grandNode) != "" && ok1 && ok2 && ok3 &&
				grand.Sub(net).Sub(tax).Abs().Cmp(invoice.NewDecimal(int64(n), 2)) <= 0
			if !ok {
				k.fail("BR-FR-MV-10_EXT-FR-FE-184", grandNode, "BR-FR-MV-10/EXT-FR-FE-184 : ligne GROUP %q : le montant total TTC de la ligne GROUP (ram:GrandTotalAmount) doit être égal à la somme du total HT (ram:LineTotalAmount) et du total TVA (ram:TaxTotalAmount), à 0,01 près par sous-ligne DETAIL ou GROUP ; TTC : %q, HT : %q, TVA : %q, sous-lignes : %d.", lineID(group), sv(grandNode), sv(netNode), sv(taxNode), n)
			}
		}
	}
}

// mv11 checks BR-FR-MV-11: the sub-invoice of the main seller (line seller
// legal identifier equal to BT-30) is numbered with the invoice number BT-1.
func (k *checker) mv11() {
	sellerIDs := nodes(k.root, pSeller+"/"+pLegalID)
	invIDs := nodes(k.root, "ExchangedDocument/ID")
	sellerGroup := func(line *xmltree.Node) bool {
		if !isGroupRoot(line) {
			return false
		}
		for lid := range all(line, pItemSeller+"/"+pLegalID) {
			for _, s := range sellerIDs {
				if sv(lid) == sv(s) {
					return true
				}
			}
		}
		return false
	}
	eqInv := func(s string) bool {
		for _, id := range invIDs {
			if s == sv(id) {
				return true
			}
		}
		return false
	}
	// $nbSubinvoiceSeller counts the AFL documents of type 130 numbered BT-1
	// in the GROUP lines of the main seller, anywhere in the document.
	nb := 0
	k.root.Walk(func(n *xmltree.Node) bool {
		if n.Name != "IncludedSupplyChainTradeLineItem" {
			return true
		}
		if sellerGroup(n) {
			for doc := range all(n, pLineARD) {
				if anyEq(doc, "ReferenceTypeCode", "AFL") && anyEq(doc, "TypeCode", "130") {
					for id := range all(doc, "IssuerAssignedID") {
						if eqInv(sv(id)) {
							nb++
							break
						}
					}
				}
			}
		}
		return true
	})
	if !k.special {
		return
	}
	for line := range all(k.root, pLines) {
		if !sellerGroup(line) {
			continue
		}
		num := subInvoiceNumber(line)
		numIsInv := eqInv(num)
		numNotInv := false // $numFactLine != $invID
		for _, id := range invIDs {
			if num != sv(id) {
				numNotInv = true
			}
		}
		if nb == 1 || nb == 0 && numIsInv || nb > 1 && numNotInv {
			continue
		}
		k.fail("BR-FR-MV-11_BT-128", line, "BR-FR-MV-11/BT-128 : ligne GROUP %q du vendeur principal (identifiant légal %q) : le vendeur principal doit avoir une et une seule ligne GROUP sans parent dont le numéro de facture à la ligne (BT-128, schéma AFL) est égal au numéro de facture (BT-1 = %q) ; lignes GROUP numérotées BT-1 : %d, numéro de la ligne : %q.", lineID(line), sv(first(k.root, pSeller+"/"+pLegalID)), sv(first(k.root, "ExchangedDocument/ID")), nb, num)
	}
}

// mv12 checks BR-FR-MV-12: the sub-invoice numbers of the GROUP lines
// without parent are unique. As in the schematron, a number is compared with
// every ram:IssuerAssignedID that precedes it in the document.
func (k *checker) mv12() {
	if !k.special {
		return
	}
	for tx := range all(k.root, pTransaction) {
		targets := map[*xmltree.Node]bool{}
		for line := range all(tx, "IncludedSupplyChainTradeLineItem") {
			if isGroupRoot(line) {
				for _, id := range subInvoiceNumberNodes(line) {
					targets[id] = true
				}
			}
		}
		if len(targets) == 0 {
			continue
		}
		var dup *xmltree.Node
		seen := map[string]bool{}
		k.root.Walk(func(n *xmltree.Node) bool {
			if dup != nil {
				return false
			}
			if n.Name == "IssuerAssignedID" {
				s := sv(n)
				if targets[n] && seen[s] {
					dup = n
					return false
				}
				seen[s] = true
			}
			return true
		})
		if dup != nil {
			k.fail("BR-FR-MV-12_BT-128", tx, "BR-FR-MV-12/BT-128 : lorsque le cadre de facturation (BT-23) est S8, B8, M8, S9, B9 ou M9, les numéros de facture à la ligne (BT-128, schéma AFL) des lignes GROUP sans parent doivent être uniques ; numéro répété : %q.", sv(dup))
		}
	}
}

// mv13 checks BR-FR-MV-13: a multi-vendor invoice is not self-billed.
func (k *checker) mv13() {
	if !k.mv {
		return
	}
	for n := range all(k.root, pTypeCode) {
		if s := sv(n); norm(s) == "" || in(s, "389", "261", "501", "500", "502", "471", "473") {
			k.fail("BR-FR-MV-13_BT-3", n, "BR-FR-MV-13/BT-3 : lorsque le cadre de facturation (BT-23) est S8, B8 ou M8, le code type de facture (BT-3) ne doit pas être un type auto-facturé (389, 261, 501, 500, 502, 471, 473) ; valeur actuelle : %q.", s)
		}
	}
}

// bd13 checks BR-FR-BD-13: a two-way invoice is self-billed.
func (k *checker) bd13() {
	if !k.bd {
		return
	}
	for n := range all(k.root, pTypeCode) {
		if s := nsv(n); s == "" || !in(s, "389", "261", "501", "500", "502", "471", "473") {
			k.fail("BR-FR-BD-13_BT-3", n, "BR-FR-BD-13/BT-3 : lorsque le cadre de facturation (BT-23) est S9, B9 ou M9, le code type de facture (BT-3) doit être un type auto-facturé (389, 261, 501, 500, 502, 471, 473) ; valeur actuelle : %q.", sv(n))
		}
	}
}

// precedingInvoice checks that the GROUP lines without parent of corrective
// invoices and credit notes reference a preceding invoice with its date.
func (k *checker) precedingInvoice(base, frames string, types ...string) {
	if !anyIn(k.root, pTypeCode, types...) {
		return
	}
	for line := range all(k.root, pLines) {
		if !isGroupRoot(line) {
			continue
		}
		if !exists(line, "SpecifiedLineTradeSettlement/InvoiceReferencedDocument/IssuerAssignedID") {
			k.fail(base+"-EXT-FR-FE-136", line, "%s-EXT-FR-FE-136 : numéro de facture antérieure manquant ligne %q : lorsque le cadre de facturation (BT-23) est %s, pour les factures rectificatives et les avoirs, chaque ligne GROUP sans parent doit comprendre un identifiant de facture antérieure à la ligne (EXT-FR-FE-136) et sa date (EXT-FR-FE-138).", base, lineID(line), frames)
		}
		if !exists(line, "SpecifiedLineTradeSettlement/InvoiceReferencedDocument/FormattedIssueDateTime/DateTimeString") {
			k.fail(base+"-EXT-FR-FE-138", line, "%s-EXT-FR-FE-138 : date de facture antérieure manquante ligne %q : lorsque le cadre de facturation (BT-23) est %s, pour les factures rectificatives et les avoirs, chaque ligne GROUP sans parent doit comprendre un identifiant de facture antérieure à la ligne (EXT-FR-FE-136) et sa date (EXT-FR-FE-138).", base, lineID(line), frames)
		}
	}
}

// mv14 checks BR-FR-MV-14 for the multi-vendor corrective invoices and
// credit notes (384, 472, 381, 396, 503).
func (k *checker) mv14() {
	if k.mv {
		k.precedingInvoice("BR-FR-MV-14", "S8, B8 ou M8", "384", "472", "381", "396", "503")
	}
}

// bd14 checks BR-FR-BD-14 for the two-way corrective invoices and credit
// notes (261, 471, 473, 502).
func (k *checker) bd14() {
	if k.bd {
		k.precedingInvoice("BR-FR-BD-14", "S9, B9 ou M9", "261", "471", "473", "502")
	}
}
