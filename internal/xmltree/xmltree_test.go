package xmltree

import (
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">
  <rsm:ExchangedDocument>
    <ram:ID> F-1 </ram:ID>
    <ram:IncludedNote><ram:Content>a &amp; b</ram:Content></ram:IncludedNote>
    <ram:IncludedNote><ram:Content>second</ram:Content><ram:SubjectCode>PMT</ram:SubjectCode></ram:IncludedNote>
  </rsm:ExchangedDocument>
  <rsm:SupplyChainTradeTransaction>
    <ram:ApplicableHeaderTradeSettlement><ram:TaxTotalAmount currencyID="EUR">19.60</ram:TaxTotalAmount></ram:ApplicableHeaderTradeSettlement>
  </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`
	root, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != "CrossIndustryInvoice" || root.Space != "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" {
		t.Fatalf("root = %s {%s}", root.Name, root.Space)
	}
	if v := root.Value("rsm:ExchangedDocument/ram:ID"); v != "F-1" {
		t.Errorf("ID = %q", v)
	}
	notes := root.FindAll("ExchangedDocument/IncludedNote/Content")
	if len(notes) != 2 || notes[0].Text != "a & b" {
		t.Errorf("notes = %d %q", len(notes), notes[0].Text)
	}
	if v := root.Value("ExchangedDocument/IncludedNote/SubjectCode"); v != "PMT" {
		t.Errorf("first SubjectCode across branches = %q", v)
	}
	tax := root.Find("SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement/TaxTotalAmount")
	if tax.Attr("currencyID") != "EUR" || tax.Line != 9 {
		t.Errorf("attr %q line %d", tax.Attr("currencyID"), tax.Line)
	}
	if p := notes[1].Path(); p != "/rsm:CrossIndustryInvoice/rsm:ExchangedDocument/ram:IncludedNote[2]/ram:Content" {
		t.Errorf("Path = %s", p)
	}
	var nilNode *Node
	if nilNode.Value("x") != "" || nilNode.Elem("x") != nil || nilNode.FindAll("x") != nil {
		t.Error("nil node accessors")
	}
}

func TestParseErrors(t *testing.T) {
	for _, doc := range []string{
		"", "<a>", "<a></b>", "<a/><b/>", "<x:a/>", "text<a/>",
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("Parse(%q) succeeded", doc)
		}
	}
}

func TestMaxDepth(t *testing.T) {
	deep := strings.Repeat("<a>", MaxDepth+1) + strings.Repeat("</a>", MaxDepth+1)
	if _, err := Parse([]byte(deep)); err == nil {
		t.Error("document nested deeper than MaxDepth accepted")
	}
	ok := strings.Repeat("<a>", MaxDepth) + strings.Repeat("</a>", MaxDepth)
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("document nested MaxDepth levels refused: %v", err)
	}
}

func TestLatin1(t *testing.T) {
	doc := append([]byte(`<?xml version="1.0" encoding="ISO-8859-1"?><a>caf`), 0xe9, '<', '/', 'a', '>')
	root, err := Parse(doc)
	if err != nil || root.Text != "café" {
		t.Fatalf("%q %v", root.Text, err)
	}
}

func TestCorpusSample(t *testing.T) {
	data, err := os.ReadFile("../../testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		t.Skip(err)
	}
	root, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if n := root.Count("SupplyChainTradeTransaction/IncludedSupplyChainTradeLineItem"); n == 0 {
		t.Error("no line found")
	}
	if !strings.HasPrefix(root.Value("ExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/ID"), "urn:cen.eu:en16931") {
		t.Error("BT-24 not found")
	}
}

func BenchmarkParse(b *testing.B) {
	data, err := os.ReadFile("../../testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		b.Skip(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}
