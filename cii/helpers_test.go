package cii

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// diffInvoices lists the fields that differ between two invoices.
func diffInvoices(a, b *invoice.Invoice) string {
	var out []string
	diffValue(reflect.ValueOf(a).Elem(), reflect.ValueOf(b).Elem(), "Invoice", &out)
	if len(out) > 20 {
		out = append(out[:20], fmt.Sprintf("… %d more", len(out)-20))
	}
	return strings.Join(out, "\n")
}

func diffValue(a, b reflect.Value, path string, out *[]string) {
	if a.Type() == reflect.TypeOf(invoice.Decimal{}) || a.Type() == reflect.TypeOf(invoice.Date{}) {
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			*out = append(*out, fmt.Sprintf("%s: %v != %v", path, a.Interface(), b.Interface()))
		}
		return
	}
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() != b.IsNil() {
				*out = append(*out, fmt.Sprintf("%s: nil=%v != nil=%v", path, a.IsNil(), b.IsNil()))
			}
			return
		}
		diffValue(a.Elem(), b.Elem(), path, out)
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			diffValue(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name, out)
		}
	case reflect.Slice:
		if a.Type().Elem().Kind() == reflect.Uint8 {
			if !reflect.DeepEqual(a.Interface(), b.Interface()) {
				*out = append(*out, fmt.Sprintf("%s: %q != %q", path, a.Bytes(), b.Bytes()))
			}
			return
		}
		if a.Len() != b.Len() || a.IsNil() != b.IsNil() {
			*out = append(*out, fmt.Sprintf("%s: len %d (nil=%v) != len %d (nil=%v)", path, a.Len(), a.IsNil(), b.Len(), b.IsNil()))
			return
		}
		for i := 0; i < a.Len(); i++ {
			diffValue(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i), out)
		}
	default:
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			*out = append(*out, fmt.Sprintf("%s: %#v != %#v", path, a.Interface(), b.Interface()))
		}
	}
}

// leaf is a leaf element or attribute of a document: its path of local
// names without positions ("SupplyChainTradeTransaction/…/ram:ID" →
// ".../ID", attributes as ".../ID/@schemeID") and its trimmed text.
type leaf struct {
	path, text string
}

// leaves returns the multiset of leaves of a document.
func leaves(root *xmltree.Node) map[leaf]int {
	m := map[leaf]int{}
	var walk func(n *xmltree.Node, path string)
	walk = func(n *xmltree.Node, path string) {
		path += "/" + n.Name
		for _, a := range n.Attrs {
			m[leaf{path + "/@" + a.Name, strings.TrimSpace(a.Value)}]++
		}
		if len(n.Children) == 0 {
			m[leaf{path, strings.TrimSpace(n.Text)}]++
			return
		}
		for _, c := range n.Children {
			walk(c, path)
		}
	}
	walk(root, "")
	return m
}

// treeNorm tunes canonTree: cur and taxCur are the currencies whose
// currencyID attributes are implied, spec keeps the specification identifier.
type treeNorm struct {
	cur, taxCur string
	spec        bool
}

// canonTree returns the content of a document as one line per attribute and
// per leaf element, in document order, each line giving the path of local
// names with the position of each element among its siblings of the same
// name: "SupplyChainTradeTransaction[1]/…/ID[2]=value",
// "…/ID[1]/@schemeID=0088". The forms that Parse and Marshal do not keep are
// normalized:
//
//   - namespace prefixes, the order of the attributes, and attributes in
//     another namespace than the element's (xsi:schemaLocation);
//   - the spaces around the text of a leaf;
//   - currencyID attributes equal to the invoice currency (BT-5) or to the
//     VAT accounting currency (BT-6);
//   - elements that hold no value (no text and no attribute in their
//     content), which carry no data;
//   - the order of the tax registrations of a party: the model has a field
//     for each scheme, written VA first, then FC;
//   - a party identifier with a scheme: it is written as a ram:GlobalID,
//     the CII form of an identifier with a scheme in EN 16931;
//   - the tax point date (BT-7) and its code (BT-8), document level terms
//     that CII repeats in the VAT breakdown entries: each value is listed
//     once, without the position of the entry.
//
// With norm.spec unset, the specification identifier (BT-24) is left out.
func canonTree(root *xmltree.Node, norm treeNorm) []string {
	var out []string
	vat := map[string]bool{}
	var walk func(n *xmltree.Node, path string)
	walk = func(n *xmltree.Node, path string) {
		if !holdsData(n, norm) {
			return
		}
		mark := len(out)
		for _, a := range n.Attrs {
			if keptAttr(a, norm) {
				out = append(out, path+"/@"+a.Name+"="+a.Value)
			}
		}
		slices.Sort(out[mark:])
		if len(n.Children) == 0 {
			if s := strings.TrimSpace(n.Text); s != "" {
				out = append(out, path+"="+s)
			}
			return
		}
		seen := map[string]int{}
		for _, c := range sortedChildren(n) {
			if !holdsData(c, norm) {
				continue
			}
			name := c.Name
			if name == "ID" && strings.HasSuffix(n.Name, "TradeParty") && attr(c, "schemeID") != "" {
				name = "GlobalID"
			}
			seen[name]++
			p := fmt.Sprintf("%s/%s[%d]", path, name, seen[name])
			switch {
			case c.Name == "GuidelineSpecifiedDocumentContextParameter" && !norm.spec:
				continue
			case (c.Name == "TaxPointDate" || c.Name == "DueDateTypeCode") && n.Name == "ApplicableTradeTax" &&
				n.Parent != nil && n.Parent.Name == "ApplicableHeaderTradeSettlement":
				// Listed once per value, after the breakdown.
				mark := len(out)
				walk(c, c.Name)
				for _, l := range out[mark:] {
					vat[l] = true
				}
				out = out[:mark]
				continue
			}
			walk(c, p)
		}
		if n.Name == "ApplicableHeaderTradeSettlement" && len(vat) > 0 {
			keys := make([]string, 0, len(vat))
			for k := range vat {
				keys = append(keys, path+"/ApplicableTradeTax/"+k)
			}
			slices.Sort(keys)
			out = append(out, keys...)
			clear(vat)
		}
	}
	walk(root, root.Name)
	return out
}

// sortedChildren returns the children of n, the tax registrations of a
// party being put in the order of the writer (see canonTree).
func sortedChildren(n *xmltree.Node) []*xmltree.Node {
	if !strings.HasSuffix(n.Name, "TradeParty") {
		return n.Children
	}
	rank := func(c *xmltree.Node) int {
		id := child(c, "ID")
		switch {
		case id != nil && strings.EqualFold(attr(id, "schemeID"), "VA"):
			return 0
		case id != nil && strings.EqualFold(attr(id, "schemeID"), "FC"):
			return 1
		}
		return 2
	}
	var regs []*xmltree.Node
	for _, c := range n.Children {
		if c.Name == "SpecifiedTaxRegistration" {
			regs = append(regs, c)
		}
	}
	if len(regs) < 2 {
		return n.Children
	}
	slices.SortStableFunc(regs, func(a, b *xmltree.Node) int { return rank(a) - rank(b) })
	kids := slices.Clone(n.Children)
	for i, c := range kids {
		if c.Name == "SpecifiedTaxRegistration" {
			kids[i], regs = regs[0], regs[1:]
		}
	}
	return kids
}

func keptAttr(a xmltree.Attr, norm treeNorm) bool {
	if a.Prefix != "" {
		return false
	}
	return a.Name != "currencyID" || (a.Value != norm.cur && a.Value != norm.taxCur)
}

// holdsData reports whether n or its content has a text or a kept attribute.
func holdsData(n *xmltree.Node, norm treeNorm) bool {
	for _, a := range n.Attrs {
		if keptAttr(a, norm) {
			return true
		}
	}
	if len(n.Children) == 0 {
		return strings.TrimSpace(n.Text) != ""
	}
	for _, c := range n.Children {
		if holdsData(c, norm) {
			return true
		}
	}
	return false
}

// treeDiff is the difference between the canonical contents of two
// documents (see canonTree).
type treeDiff struct {
	missing []string // lines of the first document missing from the second
	added   []string // lines of the second document missing from the first
	order   string   // first pair of common lines out of order, if any
}

func (d treeDiff) String() string {
	var out []string
	for _, l := range d.missing {
		out = append(out, "- "+l)
	}
	for _, l := range d.added {
		out = append(out, "+ "+l)
	}
	if d.order != "" {
		out = append(out, "order differs: "+d.order)
	}
	if len(out) > 20 {
		out = append(out[:20], fmt.Sprintf("… %d more", len(out)-20))
	}
	return strings.Join(out, "\n")
}

// diffTrees compares the canonical contents of two documents.
func diffTrees(a, b *xmltree.Node, norm treeNorm) treeDiff {
	var d treeDiff
	la, lb := canonTree(a, norm), canonTree(b, norm)
	if slices.Equal(la, lb) {
		return d
	}
	count := map[string]int{}
	for _, l := range la {
		count[l]++
	}
	for _, l := range lb {
		count[l]--
	}
	// The lines common to both documents, in the order of each.
	var ca, cb []string
	left := maps.Clone(count)
	for _, l := range la {
		if left[l] > 0 {
			left[l]--
			d.missing = append(d.missing, l)
		} else {
			ca = append(ca, l)
		}
	}
	for _, l := range lb {
		if left[l] < 0 {
			left[l]++
			d.added = append(d.added, l)
		} else {
			cb = append(cb, l)
		}
	}
	for i := range ca {
		if ca[i] != cb[i] {
			d.order = ca[i] + " / " + cb[i]
			break
		}
	}
	return d
}
