package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestUpToDate checks that codedb_gen.go is the output of the generator for
// the schematrons of FX_FACTURX_SCH (conformance/fetch.sh).
func TestUpToDate(t *testing.T) {
	dir := os.Getenv("FX_FACTURX_SCH")
	if dir == "" {
		t.Skip("FX_FACTURX_SCH not set")
	}
	got, err := generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../codedb_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("codedb_gen.go is not up to date: run go generate in internal/rules/facturx")
	}
}

func TestParseContext(t *testing.T) {
	path, preds, err := parseContext(`/rsm:CrossIndustryInvoice/ram:A[ram:B/udt:Indicator="true"]/ram:C[ not(@listID="5153") and  not(not (@listID))]`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(path, "/") != "CrossIndustryInvoice/A/C" || len(preds) != 2 || preds[0].step != 1 || preds[1].step != 2 {
		t.Fatalf("path %v, predicates %+v", path, preds)
	}
	c, err := parsePredicate(preds[1].expr)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	writeCond(&b, c, true)
	want := `codeCond{op: codeAnd, sub: []codeCond{{op: codeNot, sub: []codeCond{{op: codeAttrValue, attr: "listID", val: "5153"}}}, {op: codeNot, sub: []codeCond{{op: codeNot, sub: []codeCond{{op: codeAttr, attr: "listID"}}}}}}}`
	if b.String() != want {
		t.Errorf("predicate:\n%s\nwant\n%s", b.String(), want)
	}
	for _, bad := range []string{"//ram:A", "rsm:A/ram:B", "/rsm:A/ram:B/@x", "/rsm:A/*", "/rsm:A[ram:B"} {
		if _, _, err := parseContext(bad); err == nil {
			t.Errorf("parseContext(%q): no error", bad)
		}
	}
}

func TestParsePredicate(t *testing.T) {
	for in, want := range map[string]string{
		`@currencyID=../../ram:InvoiceCurrencyCode`: `codeCond{op: codeAttrText, path: "../../InvoiceCurrencyCode", attr: "currencyID"}`,
		`@currencyID=/rsm:A/ram:B`:                  `codeCond{op: codeAttrText, path: "/A/B", attr: "currencyID"}`,
		`ram:TypeCode='30' or ram:TypeCode="58"`:    `codeCond{op: codeOr, sub: []codeCond{{op: codeText, path: "TypeCode", val: "30"}, {op: codeText, path: "TypeCode", val: "58"}}}`,
		`(@format = '102')`:                         `codeCond{op: codeAttrValue, attr: "format", val: "102"}`,
		`ram:ID/@schemeID="VA"`:                     "",
		`normalize-space(ram:ID) != ''`:             "",
		`exists(ram:ID)`:                            "",
		`some $code in tokenize('30 58', '\s') satisfies normalize-space(ram:TypeCode) = $code`: "",
	} {
		c, err := parsePredicate(in)
		if want == "" {
			if err == nil {
				t.Errorf("parsePredicate(%q): no error", in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePredicate(%q): %v", in, err)
			continue
		}
		var b bytes.Buffer
		writeCond(&b, c, true)
		if b.String() != want {
			t.Errorf("parsePredicate(%q) = %s, want %s", in, b.String(), want)
		}
	}
}

func TestCompact(t *testing.T) {
	for in, want := range map[string]string{
		`/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:A[ram:B/udt:C="x:y"]/ram:D`: `A[B/C="x:y"]/D`,
		`/rsm:CrossIndustryInvoice/rsm:ExchangedDocument/ram:TypeCode`:                             `ExchangedDocument/TypeCode`,
		`/rsm:CrossIndustryInvoice/ram:A[ not (@listID)]`:                                          `A[ not (@listID)]`,
	} {
		if got := compact(in); got != want {
			t.Errorf("compact(%q) = %q, want %q", in, got, want)
		}
	}
}
