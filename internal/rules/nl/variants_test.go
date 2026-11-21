package nl

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// variantsDir receives the documents of the mutation cases:
//
//	go test ./internal/rules/nl -run '^TestVariants$' -variants DIR
//
// The parity job (conformance/parity.sh) compares them with the NPa XSLT.
var variantsDir = flag.String("variants", "", "write the documents of the mutation cases to this directory")

// TestVariants builds the document of every mutation case of the CII, UBL
// and G-account invoices, and writes them to -variants when it is set:
// cii-001-<case>.xml, ubl-001-<case>.xml, gaccount-001-<case>.xml, numbered
// in the order of the cases. The base invoices, which are in testdata, are
// left out.
func TestVariants(t *testing.T) {
	if *variantsDir != "" {
		if err := os.MkdirAll(*variantsDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for _, set := range []struct {
		prefix, base string
		cases        []mutationCase
	}{
		{"cii", base, mutationCases()},
		{"ubl", ublBase, ublCases()},
		{"gaccount", gaBase, gaccountCases()},
	} {
		doc := load(t, set.base)
		for i, c := range set.cases {
			if len(c.edits) == 0 {
				continue
			}
			name := fmt.Sprintf("%s-%03d-%s.xml", set.prefix, i, slug(c.name))
			data := apply(t, doc, c.edits)
			n++
			if *variantsDir == "" {
				continue
			}
			if err := os.WriteFile(filepath.Join(*variantsDir, name), []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if *variantsDir != "" {
		t.Logf("%d variants written to %s", n, *variantsDir)
	}
}

// slug turns a case name into a file name: lower case, a hyphen for every
// run of other characters than letters and digits.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	return b.String()
}
