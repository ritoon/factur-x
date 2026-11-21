package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung"
)

// mutated writes to a temporary file the invoice name with old replaced by
// new, and returns its path.
func mutated(t *testing.T, name, old, new string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(old)) {
		t.Fatalf("%s: %q not found", name, old)
	}
	path := filepath.Join(t.TempDir(), filepath.Base(name))
	if err := os.WriteFile(path, bytes.ReplaceAll(b, []byte(old), []byte(new)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runValidate runs zerechnung validate -json with args and returns the exit
// code and the rules of the findings.
func runValidate(t *testing.T, args ...string) (int, map[string]bool) {
	t.Helper()
	var out bytes.Buffer
	code, err := validateCmd(append([]string{"-json"}, args...), &out)
	if err != nil {
		t.Fatalf("validate %s: %v", strings.Join(args, " "), err)
	}
	var report struct {
		Findings []struct{ Rule string }
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("validate %s: %v\n%s", strings.Join(args, " "), err, out.Bytes())
	}
	rules := map[string]bool{}
	for _, f := range report.Findings {
		rules[f.Rule] = true
	}
	return code, rules
}

// TestValidateDeclaredRules checks that the rule options of validate do not
// drop the rules an invoice declares in BT-24: BR-DE for XRECHNUNG, Peppol
// for a Peppol invoice, BR-NL for an NLCIUS invoice.
func TestValidateDeclaredRules(t *testing.T) {
	xrechnung := mutated(t, "../../testdata/XRECHNUNG_Einfach.cii.xml",
		"<ram:BuyerReference>04011000-12345-34</ram:BuyerReference>", "")
	peppol := mutated(t, "../../internal/rules/peppol/testdata/peppol.xml",
		"URIUniversalCommunication>", "X>")
	nlcius := mutated(t, "../../internal/rules/nl/testdata/nlcius.ubl.xml",
		"<cbc:BuyerReference>PO-4512</cbc:BuyerReference>", "")
	for _, flags := range [][]string{
		nil,
		{"-fr"},
		{"-de"},
		{"-choruspro"},
		{"-peppol"},
		{"-nl"},
		{"-fr", "-auto"},
	} {
		code, got := runValidate(t, append(flags, xrechnung)...)
		if code != 1 || !got["BR-DE-15"] {
			t.Errorf("validate %v on XRECHNUNG without BT-10: code %d, BR-DE-15 reported %v", flags, code, got["BR-DE-15"])
		}
		code, got = runValidate(t, append(flags, peppol)...)
		if code != 1 || !got["PEPPOL-EN16931-R010"] {
			t.Errorf("validate %v on Peppol invoice without electronic address: code %d, PEPPOL-EN16931-R010 reported %v", flags, code, got["PEPPOL-EN16931-R010"])
		}
		code, got = runValidate(t, append(flags, nlcius)...)
		if code != 1 || !got["BR-NL-2"] {
			t.Errorf("validate %v on NLCIUS invoice without BT-10: code %d, BR-NL-2 reported %v", flags, code, got["BR-NL-2"])
		}
	}
}

// TestValidateNLOption checks that -nl applies the Dutch rules to an invoice
// that does not declare NLCIUS (on a UBL invoice, the SI-UBL 2.0 version
// check reports it), and that they are not applied otherwise.
func TestValidateNLOption(t *testing.T) {
	const peppol = "../../internal/rules/peppol/testdata/peppol.ubl.xml"
	for _, c := range []struct {
		flags []string
		want  bool
	}{
		{nil, false},
		{[]string{"-auto"}, false},
		{[]string{"-peppol"}, false},
		{[]string{"-nl"}, true},
	} {
		_, got := runValidate(t, append(c.flags, peppol)...)
		if got["SI-V20-INV-R000"] != c.want {
			t.Errorf("validate %v on a Peppol invoice: SI-V20-INV-R000 reported %v, want %v", c.flags, got["SI-V20-INV-R000"], c.want)
		}
	}
}

// TestValidatePeppolOption checks that -peppol applies the Peppol rules to an
// invoice that does not declare itself Peppol, and that they are not applied
// otherwise.
func TestValidatePeppolOption(t *testing.T) {
	const french = "../../internal/rules/fr/testdata/en16931.xml"
	for _, c := range []struct {
		flags []string
		want  bool
	}{
		{nil, false},
		{[]string{"-fr"}, false},
		{[]string{"-auto"}, false},
		{[]string{"-peppol"}, true},
		{[]string{"-fr", "-peppol"}, true},
	} {
		_, got := runValidate(t, append(c.flags, french)...)
		if got["PEPPOL-EN16931-R004"] != c.want {
			t.Errorf("validate %v on a Factur-X invoice: PEPPOL-EN16931-R004 reported %v, want %v", c.flags, got["PEPPOL-EN16931-R004"], c.want)
		}
	}
}

// TestEmbedRequirePDFA checks that embed -require-pdfa refuses an input PDF
// that does not declare PDF/A and writes nothing.
func TestEmbedRequirePDFA(t *testing.T) {
	const host, xml = "../../testdata/host.pdf", "../../testdata/EN16931_Einfach.cii.xml"
	out := filepath.Join(t.TempDir(), "out.pdf")
	err := embed([]string{"-xml", xml, "-require-pdfa", "-o", out, host})
	if !errors.Is(err, zerechnung.ErrNotPDFA) {
		t.Fatalf("embed -require-pdfa on a PDF without PDF/A declaration: %v, want ErrNotPDFA", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("output written: %v", err)
	}
	if err := embed([]string{"-xml", xml, "-o", out, host}); err != nil {
		t.Fatalf("embed without -require-pdfa: %v", err)
	}
	if err := embed([]string{"-xml", xml, "-require-pdfa", "-o", out, out}); err != nil {
		t.Errorf("embed -require-pdfa on a PDF declared PDF/A-3: %v", err)
	}
}
