package main

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// schemaTemplate is a small schema in the style of the Factur-X ones; %s
// receives extra particles of the root sequence and %s extra declarations.
const schemaTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
    targetNamespace="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
    elementFormDefault="qualified">
  <xs:element name="CrossIndustryInvoice" type="rsm:InvoiceType"/>
  <xs:complexType name="InvoiceType">
    <xs:sequence>
      <xs:element name="ID" type="rsm:IDType"/>
      <xs:element name="Amount" type="rsm:AmountType" minOccurs="0" maxOccurs="unbounded"/>
      %s
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="IDType">
    <xs:simpleContent>
      <xs:extension base="rsm:CodeContentType">
        <xs:attribute name="schemeID" type="xs:string" use="required"/>
      </xs:extension>
    </xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="AmountType">
    <xs:simpleContent>
      <xs:extension base="xs:decimal"/>
    </xs:simpleContent>
  </xs:complexType>
  <xs:simpleType name="CodeContentType">
    <xs:restriction base="xs:string">
      <xs:pattern value="[A-Z]{2}"/>
      <xs:maxLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:complexType name="NoteType">
    <xs:choice maxOccurs="2">
      <xs:element name="Text" type="rsm:AmountType"/>
      <xs:element name="Flag" type="rsm:AmountType"/>
    </xs:choice>
  </xs:complexType>
  %s
</xs:schema>
`

// writeSchemas writes the five main schemas into a new directory; the
// EXTENDED one gets the extra particles and declarations.
func writeSchemas(t *testing.T, extraParticles, extraDecls string) string {
	t.Helper()
	dir := t.TempDir()
	for _, p := range []string{"MINIMUM", "BASICWL", "BASIC", "EN16931", "EXTENDED"} {
		parts, decls := "", ""
		if p == "EXTENDED" {
			parts, decls = extraParticles, extraDecls
		}
		name := filepath.Join(dir, "Test_2.3_"+p+".xsd")
		if err := os.WriteFile(name, []byte(fmt.Sprintf(schemaTemplate, parts, decls)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGenerate(t *testing.T) {
	dir := writeSchemas(t, `<xs:element name="Note" type="rsm:NoteType" minOccurs="0"/>`, "")
	src, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "tables_gen.go", src, 0); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, src)
	}
	for _, want := range []string{
		`const schemaVersion = "2.3"`,
		`"[A-Z]{2}"`, // pattern facet
		`"schemeID"`, // attribute
		`"Note"`,     // EXTENDED only particle
		`"Flag"`,     // choice alternative
	} {
		if !bytes.Contains(src, []byte(want)) {
			t.Errorf("generated source lacks %s\n%s", want, src)
		}
	}
	// IDType is shared by the five profiles, InvoiceType is not.
	if n := bytes.Count(src, []byte(`name: "rsm:IDType"`)); n != 1 {
		t.Errorf("IDType emitted %d times\n%s", n, src)
	}
	if n := bytes.Count(src, []byte(`name: "rsm:InvoiceType"`)); n != 2 {
		t.Errorf("InvoiceType emitted %d times\n%s", n, src)
	}
}

func TestUnsupported(t *testing.T) {
	for _, tc := range []struct {
		name, parts, decls, err string
	}{
		{"all", `<xs:element name="X" type="rsm:AllType"/>`,
			`<xs:complexType name="AllType"><xs:all><xs:element name="A" type="rsm:AmountType"/></xs:all></xs:complexType>`, "all"},
		{"nested group", `<xs:element name="X" type="rsm:NestedType"/>`,
			`<xs:complexType name="NestedType"><xs:sequence><xs:choice><xs:element name="A" type="rsm:AmountType"/></xs:choice></xs:sequence></xs:complexType>`, "nested"},
		{"duplicate name", `<xs:element name="ID" type="rsm:AmountType" minOccurs="0"/>`, "", "twice"},
		{"recursion", `<xs:element name="X" type="rsm:LoopType"/>`,
			`<xs:complexType name="LoopType"><xs:sequence><xs:element name="Y" type="rsm:LoopType" minOccurs="0"/></xs:sequence></xs:complexType>`, "recursive"},
		{"unknown type", `<xs:element name="X" type="rsm:MissingType"/>`, "", "MissingType"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := generate(writeSchemas(t, tc.parts, tc.decls))
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("error = %v, want one mentioning %q", err, tc.err)
			}
		})
	}
}

// TestCorrigendumVersion checks that the tables take the highest version of
// the main schemas, as when a corrigendum renames only the changed profiles.
func TestCorrigendumVersion(t *testing.T) {
	dir := writeSchemas(t, "", "")
	for _, p := range []string{"BASIC", "EXTENDED"} {
		if err := os.Rename(filepath.Join(dir, "Test_2.3_"+p+".xsd"), filepath.Join(dir, "Test_2.3.10_"+p+".xsd")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(filepath.Join(dir, "Test_2.3_EN16931.xsd"), filepath.Join(dir, "Test_2.3.9_EN16931.xsd")); err != nil {
		t.Fatal(err)
	}
	src, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte(`const schemaVersion = "2.3.10"`)) {
		t.Errorf("generated source lacks version 2.3.10\n%s", src[:300])
	}
	for _, c := range []struct {
		v, cur string
		want   bool
	}{
		{"1.09", "", true}, {"1.09.2", "1.09", true}, {"1.09", "1.09.2", false},
		{"1.10", "1.09.2", true}, {"1.09", "1.09", false}, {"2.3.10", "2.3.9", true},
	} {
		if got := newerVersion(c.v, c.cur); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v", c.v, c.cur, got)
		}
	}
}

func TestMissingProfile(t *testing.T) {
	dir := writeSchemas(t, "", "")
	if err := os.Remove(filepath.Join(dir, "Test_2.3_BASICWL.xsd")); err != nil {
		t.Fatal(err)
	}
	if _, err := generate(dir); err == nil || !strings.Contains(err.Error(), "BASIC WL") {
		t.Errorf("error = %v", err)
	}
}

// TestUpToDate checks that tables_gen.go matches the schemas in the
// directory named by FX_XSD_DIR (the official Factur-X package).
func TestUpToDate(t *testing.T) {
	dir := os.Getenv("FX_XSD_DIR")
	if dir == "" {
		t.Skip("FX_XSD_DIR not set")
	}
	src, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	cur, err := os.ReadFile(filepath.Join("..", "tables_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, cur) {
		t.Error("tables_gen.go is out of date: run go generate in internal/schema")
	}
}
