package cii

import (
	"testing"

	"github.com/ritoon/zerechnung/internal/xmltree"
	"github.com/ritoon/zerechnung/invoice"
)

func benchInvoice(b *testing.B) *invoice.Invoice {
	b.Helper()
	inv, err := Parse(readTestdata(b, "EN16931_Einfach.cii.xml"))
	if err != nil {
		b.Fatal(err)
	}
	return inv
}

func BenchmarkMarshal(b *testing.B) {
	inv := benchInvoice(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(inv, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalIndent(b *testing.B) {
	inv := benchInvoice(b)
	opts := &MarshalOptions{Indent: true}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(inv, opts); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMarshalFull writes the invoice that sets every field of the model.
func BenchmarkMarshalFull(b *testing.B) {
	inv := fullInvoice()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(inv, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseTree measures the mapping of an already parsed tree to the
// model, without the XML parsing.
func BenchmarkParseTree(b *testing.B) {
	root, err := xmltree.Parse(readTestdata(b, "EN16931_Einfach.cii.xml"))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, issues := ParseTree(root); len(issues) > 0 {
			b.Fatal(issues)
		}
	}
}

// BenchmarkParse includes the XML parsing (see BenchmarkXMLTreeParse).
func BenchmarkParse(b *testing.B) {
	data := readTestdata(b, "EN16931_Einfach.cii.xml")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkXMLTreeParse is the cost of xmltree.Parse alone, for comparison.
func BenchmarkXMLTreeParse(b *testing.B) {
	data := readTestdata(b, "EN16931_Einfach.cii.xml")
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := xmltree.Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}
