package cii

import (
	"fmt"
	"reflect"
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
