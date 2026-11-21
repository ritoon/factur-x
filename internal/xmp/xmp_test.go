package xmp

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// zugferd1 is a ZUGFeRD 1.0 packet: properties as elements, extension
// schema with rdf:parseType="Resource", and an unrelated description.
const zugferd1 = `<?xpacket begin="` + "\uFEFF" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">
   <pdfaid:part>3</pdfaid:part>
   <pdfaid:conformance>A</pdfaid:conformance>
  </rdf:Description>
  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
   <dc:title><rdf:Alt><rdf:li xml:lang="x-default">Rechnung &amp; Co</rdf:li></rdf:Alt></dc:title>
  </rdf:Description>
  <rdf:Description rdf:about="" xmlns:zf="urn:ferd:pdfa:CrossIndustryDocument:invoice:1p0#">
   <zf:DocumentType>INVOICE</zf:DocumentType>
   <zf:DocumentFileName>ZUGFeRD-invoice.xml</zf:DocumentFileName>
   <zf:Version>1.0</zf:Version>
   <zf:ConformanceLevel>COMFORT</zf:ConformanceLevel>
  </rdf:Description>
  <rdf:Description rdf:about="" xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/"
    xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#" xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#">
   <pdfaExtension:schemas>
    <rdf:Bag>
     <rdf:li rdf:parseType="Resource">
      <pdfaSchema:schema>ZUGFeRD PDFA Extension Schema</pdfaSchema:schema>
      <pdfaSchema:namespaceURI>urn:ferd:pdfa:CrossIndustryDocument:invoice:1p0#</pdfaSchema:namespaceURI>
      <pdfaSchema:prefix>zf</pdfaSchema:prefix>
      <pdfaSchema:property>
       <rdf:Seq>
        <rdf:li rdf:parseType="Resource"><pdfaProperty:name>DocumentFileName</pdfaProperty:name></rdf:li>
        <rdf:li rdf:parseType="Resource"><pdfaProperty:name>DocumentType</pdfaProperty:name></rdf:li>
        <rdf:li rdf:parseType="Resource"><pdfaProperty:name>Version</pdfaProperty:name></rdf:li>
        <rdf:li rdf:parseType="Resource"><pdfaProperty:name>ConformanceLevel</pdfaProperty:name></rdf:li>
       </rdf:Seq>
      </pdfaSchema:property>
     </rdf:li>
     <rdf:li rdf:parseType="Resource">
      <pdfaSchema:schema>Other</pdfaSchema:schema>
      <pdfaSchema:namespaceURI>http://example.com/other#</pdfaSchema:namespaceURI>
      <pdfaSchema:prefix>o</pdfaSchema:prefix>
     </rdf:li>
    </rdf:Bag>
   </pdfaExtension:schemas>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`

// attrForm has the properties as attributes and schema descriptions in
// nested rdf:Description elements.
const attrForm = `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="uuid:1" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/" xmlns:fx="urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#"
  xmlns:xmp="http://ns.adobe.com/xap/1.0/"
  pdfaid:part="3" pdfaid:conformance="B" fx:DocumentType="INVOICE" fx:DocumentFileName="factur-x.xml"
  fx:Version="1.0" fx:ConformanceLevel="EN 16931" xmp:ModifyDate="2026-11-16T12:00:00+01:00"/>
<rdf:Description rdf:about="uuid:2" xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/"
  xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#" xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#">
 <pdfaExtension:schemas><rdf:Bag><rdf:li>
  <rdf:Description pdfaSchema:namespaceURI="urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#" pdfaSchema:prefix="fx" pdfaSchema:schema="Factur-X">
   <pdfaSchema:property><rdf:Seq>
    <rdf:li><rdf:Description pdfaProperty:name="DocumentType"/></rdf:li>
    <rdf:li><rdf:Description pdfaProperty:name="ConformanceLevel"/></rdf:li>
   </rdf:Seq></pdfaSchema:property>
  </rdf:Description>
 </rdf:li></rdf:Bag></pdfaExtension:schemas>
</rdf:Description>
</rdf:RDF></x:xmpmeta>`

func TestParseElements(t *testing.T) {
	info, err := Parse([]byte(zugferd1))
	if err != nil {
		t.Fatal(err)
	}
	if info.PDFAPart != "3" || info.PDFAConformance != "A" {
		t.Errorf("pdfaid %q %q", info.PDFAPart, info.PDFAConformance)
	}
	want := Invoice{NSZUGFeRD1, "INVOICE", "ZUGFeRD-invoice.xml", "1.0", "COMFORT"}
	if info.Invoice == nil || *info.Invoice != want {
		t.Fatalf("invoice %+v", info.Invoice)
	}
	s := info.Schema(NSZUGFeRD1)
	if s == nil || s.Prefix != "zf" || strings.Join(s.Properties, ",") != "DocumentFileName,DocumentType,Version,ConformanceLevel" {
		t.Fatalf("schema %+v", s)
	}
	if len(info.Schemas) != 2 || info.AboutMismatch {
		t.Errorf("schemas %d, mismatch %v", len(info.Schemas), info.AboutMismatch)
	}
}

func TestParseAttributes(t *testing.T) {
	info, err := Parse([]byte(attrForm))
	if err != nil {
		t.Fatal(err)
	}
	want := Invoice{NSFacturX, "INVOICE", "factur-x.xml", "1.0", "EN 16931"}
	if info.Invoice == nil || *info.Invoice != want {
		t.Fatalf("invoice %+v", info.Invoice)
	}
	if info.PDFAPart != "3" || info.PDFAConformance != "B" || info.ModifyDate != "2026-11-16T12:00:00+01:00" {
		t.Errorf("info %+v", info)
	}
	s := info.Schema(NSFacturX)
	if s == nil || s.Name != "Factur-X" || strings.Join(s.Properties, ",") != "DocumentType,ConformanceLevel" {
		t.Fatalf("schema %+v", s)
	}
	if !info.AboutMismatch || info.About != "uuid:1" {
		t.Errorf("about %q, mismatch %v", info.About, info.AboutMismatch)
	}
}

func TestParseUTF16(t *testing.T) {
	u := utf16.Encode([]rune(attrForm))
	b := []byte{0xfe, 0xff}
	for _, c := range u {
		b = binary.BigEndian.AppendUint16(b, c)
	}
	info, err := Parse(b)
	if err != nil || info.Invoice == nil || info.Invoice.ConformanceLevel != "EN 16931" {
		t.Fatalf("UTF-16: %+v, %v", info, err)
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/"></x:xmpmeta>`)); err != ErrNoRDF {
		t.Errorf("no RDF: %v", err)
	}
	for _, s := range []string{`<a><b></a>`, `<a`, `<a x="1></a>`} {
		if _, err := Parse([]byte(s)); err == nil {
			t.Errorf("%q: no error", s)
		}
	}
}

func TestParseInfoProps(t *testing.T) {
	const packet = `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:pdf="http://ns.adobe.com/pdf/1.3/"
  xmlns:xmp="http://ns.adobe.com/xap/1.0/" pdf:Producer="Producteur &amp; Cie" xmp:CreateDate="2026-01-01T00:00:00Z">
 <dc:title><rdf:Alt><rdf:li xml:lang="fr">Facture</rdf:li><rdf:li xml:lang="x-default">Invoice</rdf:li></rdf:Alt></dc:title>
 <dc:description><rdf:Alt><rdf:li xml:lang="de">Rechnung</rdf:li></rdf:Alt></dc:description>
 <dc:creator><rdf:Seq><rdf:li>Jean</rdf:li><rdf:li>Marie</rdf:li></rdf:Seq></dc:creator>
 <dc:subject><rdf:Bag><rdf:li>ignored</rdf:li></rdf:Bag></dc:subject>
 <pdf:Keywords></pdf:Keywords>
 <xmp:CreatorTool>Writer</xmp:CreatorTool>
</rdf:Description>
</rdf:RDF></x:xmpmeta>`
	info, err := Parse([]byte(packet))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Title":        "Invoice",
		"Subject":      "Rechnung",
		"Author":       "Jean, Marie",
		"Keywords":     "",
		"Creator":      "Writer",
		"Producer":     "Producteur & Cie",
		"CreationDate": "2026-01-01T00:00:00Z",
	}
	if len(info.InfoProps) != len(want) {
		t.Errorf("InfoProps %q", info.InfoProps)
	}
	for k, v := range want {
		if got, ok := info.InfoProps[k]; !ok || got != v {
			t.Errorf("%s: %q, want %q", k, got, v)
		}
	}

	info, _ = Parse([]byte(zugferd1))
	if len(info.InfoProps) != 1 || info.InfoProps["Title"] != "Rechnung & Co" {
		t.Errorf("ZUGFeRD 1.0: %q", info.InfoProps)
	}
	info, _ = Parse([]byte(attrForm))
	if len(info.InfoProps) != 1 || info.InfoProps["ModDate"] != "2026-11-16T12:00:00+01:00" {
		t.Errorf("attributes: %q", info.InfoProps)
	}
}

func TestParseDate(t *testing.T) {
	cet := time.FixedZone("", 3600)
	for s, want := range map[string]time.Time{
		"2026":                          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"2026-11":                       time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		"2026-11-16":                    time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
		"2026-11-16T12:30+01:00":        time.Date(2026, 11, 16, 12, 30, 0, 0, cet),
		"2026-11-16T12:30:15Z":          time.Date(2026, 11, 16, 12, 30, 15, 0, time.UTC),
		" 2026-11-16T12:30:15.5+01:00 ": time.Date(2026, 11, 16, 12, 30, 15, 5e8, cet),
		"2026-11-16T12:30:15":           time.Date(2026, 11, 16, 12, 30, 15, 0, time.UTC),
	} {
		if got, ok := ParseDate(s); !ok || !got.Equal(want) {
			t.Errorf("ParseDate(%q) = %v, %v", s, got, ok)
		}
	}
	for _, s := range []string{"", "16/11/2026", "2026-13-01", "D:20261116"} {
		if _, ok := ParseDate(s); ok {
			t.Errorf("ParseDate(%q): no error", s)
		}
	}
}

var params = Params{
	DocumentType:     "INVOICE",
	DocumentFileName: "factur-x.xml",
	Version:          "1.0",
	ConformanceLevel: "EN 16931",
	Date:             time.Date(2026, 11, 16, 12, 0, 0, 0, time.FixedZone("", 3600)),
}

// checkMerged verifies the managed properties of a merged packet and that
// it is well-formed XML.
func checkMerged(t *testing.T, out []byte, conformance string) *Info {
	t.Helper()
	d := xml.NewDecoder(bytes.NewReader(out))
	for {
		if _, err := d.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("not well-formed: %v\n%s", err, out)
		}
	}
	info, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v\n%s", err, out)
	}
	if info.PDFAPart != "3" || info.PDFAConformance != conformance {
		t.Errorf("pdfaid %q %q", info.PDFAPart, info.PDFAConformance)
	}
	want := Invoice{NSFacturX, "INVOICE", "factur-x.xml", "1.0", "EN 16931"}
	if info.Invoice == nil || *info.Invoice != want {
		t.Errorf("invoice %+v\n%s", info.Invoice, out)
	}
	s := info.Schema(NSFacturX)
	if s == nil || s.Prefix != "fx" || len(s.Properties) != 4 {
		t.Errorf("schema %+v", s)
	}
	if info.ModifyDate != "2026-11-16T12:00:00+01:00" || info.MetadataDate != info.ModifyDate {
		t.Errorf("dates %q %q", info.ModifyDate, info.MetadataDate)
	}
	return info
}

func TestMergeInPlace(t *testing.T) {
	out := Merge([]byte(zugferd1), params)
	info := checkMerged(t, out, "A")
	// The ZUGFeRD 1.0 properties and schema are replaced, the rest kept.
	if bytes.Contains(out, []byte("ZUGFeRD-invoice.xml")) || info.Schema(NSZUGFeRD1) != nil {
		t.Errorf("old invoice schema kept:\n%s", out)
	}
	if info.Schema("http://example.com/other#") == nil {
		t.Error("unrelated schema dropped")
	}
	if !bytes.Contains(out, []byte(`<rdf:li xml:lang="x-default">Rechnung &amp; Co</rdf:li>`)) {
		t.Error("dc:title not kept byte for byte")
	}
	if !bytes.HasSuffix(out, []byte(`<?xpacket end="w"?>`)) {
		t.Error("packet trailer lost")
	}
	if len(info.Schemas) != 2 {
		t.Errorf("%d schemas", len(info.Schemas))
	}
	// Merging again is stable.
	again := Merge(out, params)
	if !bytes.Equal(again, out) {
		t.Errorf("second merge differs:\n%s\n----\n%s", out, again)
	}
}

func TestMergeAttributes(t *testing.T) {
	p := params
	p.ConformanceLevel = "EN 16931"
	out := Merge([]byte(attrForm), p)
	checkMerged(t, out, "B")
	// The emptied description is removed; the new one keeps its rdf:about.
	if !bytes.Contains(out, []byte(`rdf:about="uuid:1"`)) {
		t.Error("rdf:about not kept")
	}
	if n := bytes.Count(out, []byte("<fx:DocumentType>")) + bytes.Count(out, []byte("fx:DocumentType=")); n != 1 {
		t.Errorf("fx:DocumentType written %d times:\n%s", n, out)
	}
}

func TestMergeSelfClosingBag(t *testing.T) {
	in := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/"><pdfaExtension:schemas><rdf:Bag/></pdfaExtension:schemas></rdf:Description>
</rdf:RDF></x:xmpmeta>`
	out := Merge([]byte(in), params)
	info := checkMerged(t, out, "B")
	if len(info.Schemas) != 1 {
		t.Errorf("%d schemas:\n%s", len(info.Schemas), out)
	}
}

func TestMergeFresh(t *testing.T) {
	p := params
	p.DocInfo = DocInfo{Title: "Facture <1>", Author: "ACME", Producer: "zerechnung", CreationDate: params.Date}
	for _, in := range []string{"", "not xml <", `<x:xmpmeta xmlns:x="adobe:ns:meta/"/>`} {
		out := Merge([]byte(in), p)
		checkMerged(t, out, "B")
		for _, s := range []string{"Facture &lt;1&gt;", "ACME", "<pdf:Producer>zerechnung</pdf:Producer>", "2026-11-16T12:00:00+01:00", `<?xpacket end="w"?>`} {
			if !bytes.Contains(out, []byte(s)) {
				t.Errorf("%q: %q missing:\n%s", in, s, out)
			}
		}
	}
}

func TestMergeDocInfoNotDuplicated(t *testing.T) {
	p := params
	p.DocInfo = DocInfo{Title: "Other title"}
	out := Merge([]byte(zugferd1), p)
	if bytes.Contains(out, []byte("Other title")) {
		t.Errorf("dc:title added although present:\n%s", out)
	}
}

func TestMergeNamespace(t *testing.T) {
	p := params
	p.NamespaceURI = NSZUGFeRD2
	out := Merge(nil, p)
	info, err := Parse(out)
	if err != nil || info.Invoice == nil || info.Invoice.NamespaceURI != NSZUGFeRD2 || info.Schema(NSZUGFeRD2) == nil {
		t.Fatalf("%+v %v", info, err)
	}
}

func TestMergePDFAIdentification(t *testing.T) {
	// pkt returns a packet whose PDF/A identification is given as
	// elements, or as attributes when attr is set.
	pkt := func(attr bool, props ...string) string {
		var attrs, elems string
		for i := 0; i+1 < len(props); i += 2 {
			if attr {
				attrs += " pdfaid:" + props[i] + `="` + props[i+1] + `"`
			} else {
				elems += "<pdfaid:" + props[i] + ">" + props[i+1] + "</pdfaid:" + props[i] + ">"
			}
		}
		return `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/" xmlns:dc="http://purl.org/dc/elements/1.1/"` + attrs + `>` + elems +
			`<dc:format>application/pdf</dc:format></rdf:Description>
</rdf:RDF></x:xmpmeta>`
	}
	for _, tc := range []struct {
		name       string
		in         string
		part, conf string // Params.PDFAPart and Params.PDFAConformance
		want       [3]string
	}{
		{"fresh", "", "", "", [3]string{"3", "B", ""}},
		{"fresh, part 4", "", "4", "", [3]string{"4", "F", "2020"}},
		{"fresh, part 4, level E", "", "4", "E", [3]string{"4", "E", "2020"}},
		{"fresh, part 4, level B ignored", "", "4", "B", [3]string{"4", "F", "2020"}},
		{"fresh, unknown part", "", "2", "", [3]string{"3", "B", ""}},
		{"PDF/A-3a kept", pkt(false, "part", "3", "conformance", "A"), "", "", [3]string{"3", "A", ""}},
		{"PDF/A-2u to 3u", pkt(false, "part", "2", "conformance", "U"), "", "", [3]string{"3", "U", ""}},
		{"lower-case level", pkt(false, "part", "3", "conformance", "b"), "", "", [3]string{"3", "B", ""}},
		{"PDF/A-4f kept", pkt(false, "part", "4", "rev", "2020", "conformance", "F"), "", "", [3]string{"4", "F", "2020"}},
		{"PDF/A-4e kept", pkt(false, "part", "4", "rev", "2020", "conformance", "E"), "", "", [3]string{"4", "E", "2020"}},
		{"PDF/A-4e kept with part 4", pkt(false, "part", "4", "conformance", "E"), "4", "F", [3]string{"4", "E", "2020"}},
		{"later revision kept", pkt(false, "part", "4", "rev", "2026", "conformance", "F"), "", "", [3]string{"4", "F", "2026"}},
		{"plain PDF/A-4 gets F and rev", pkt(false, "part", "4"), "", "", [3]string{"4", "F", "2020"}},
		{"PDF/A-4 attributes", pkt(true, "part", "4", "rev", "2020", "conformance", "E"), "", "", [3]string{"4", "E", "2020"}},
		{"PDF/A-3b to 4f", pkt(false, "part", "3", "conformance", "B"), "4", "", [3]string{"4", "F", "2020"}},
		{"PDF/A-4f to 3b", pkt(true, "part", "4", "rev", "2020", "conformance", "F"), "3", "", [3]string{"3", "B", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := params
			p.PDFAPart, p.PDFAConformance = tc.part, tc.conf
			out := Merge([]byte(tc.in), p)
			info, err := Parse(out)
			if err != nil {
				t.Fatalf("Parse: %v\n%s", err, out)
			}
			if got := [3]string{info.PDFAPart, info.PDFAConformance, info.PDFARev}; got != tc.want {
				t.Errorf("part, conformance, rev %q, want %q\n%s", got, tc.want, out)
			}
			for _, prop := range []string{"part", "conformance", "rev"} {
				if n := bytes.Count(out, []byte("pdfaid:"+prop+">"))/2 + bytes.Count(out, []byte("pdfaid:"+prop+"=")); n > 1 {
					t.Errorf("pdfaid:%s written %d times:\n%s", prop, n, out)
				}
			}
			if tc.in != "" && !bytes.Contains(out, []byte("<dc:format>application/pdf</dc:format>")) {
				t.Errorf("dc:format dropped:\n%s", out)
			}
		})
	}
}

func TestMergePDFAOnly(t *testing.T) {
	p := Params{PDFAOnly: true, Date: params.Date, DocInfo: DocInfo{Title: "Devis"}}
	for _, in := range []string{"", zugferd1} {
		out := Merge([]byte(in), p)
		info, err := Parse(out)
		if err != nil {
			t.Fatalf("Parse: %v\n%s", err, out)
		}
		if info.PDFAPart != "3" || info.ModifyDate != "2026-11-16T12:00:00+01:00" {
			t.Errorf("pdfaid:part %q, xmp:ModifyDate %q", info.PDFAPart, info.ModifyDate)
		}
		if in == "" {
			// A new packet: no hybrid-invoice property nor schema.
			if info.Invoice != nil || len(info.Schemas) != 0 || bytes.Contains(out, []byte("fx:")) || info.PDFAConformance != "B" {
				t.Errorf("fresh packet with invoice data:\n%s", out)
			}
			if !bytes.Contains(out, []byte("Devis")) {
				t.Errorf("dc:title missing:\n%s", out)
			}
			continue
		}
		// The ZUGFeRD 1.0 properties and their schema are kept.
		if info.Invoice == nil || info.Invoice.DocumentFileName != "ZUGFeRD-invoice.xml" || info.Schema(NSZUGFeRD1) == nil {
			t.Errorf("invoice data of the packet not kept:\n%s", out)
		}
		if again := Merge(out, p); !bytes.Equal(again, out) {
			t.Errorf("second merge differs:\n%s\n----\n%s", out, again)
		}
	}
}
