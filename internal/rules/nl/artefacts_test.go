package nl

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// assertion is an assertion of a schematron pattern.
type assertion struct {
	id, flag, message string
}

// patternAsserts returns the assertions of the pattern id of a schematron
// file. An assertion without identifier takes the one in brackets that opens
// its message.
func patternAsserts(t *testing.T, file, id string) []assertion {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	var out []assertion
	found := false
	root.Walk(func(n *xmltree.Node) bool {
		if n.Name != "pattern" || n.Attr("id") != id {
			return true
		}
		found = true
		for _, r := range n.Children {
			if r.Name != "rule" {
				continue
			}
			for _, a := range r.Children {
				if a.Name != "assert" {
					continue
				}
				as := assertion{id: a.Attr("id"), flag: a.Attr("flag"), message: nsv(a)}
				if as.flag == "" {
					as.flag = r.Attr("flag")
				}
				if as.id == "" && strings.HasPrefix(as.message, "[") {
					as.id = as.message[1:strings.Index(as.message, "]")]
				}
				out = append(out, as)
			}
		}
		return false
	})
	if !found {
		t.Fatalf("%s: no pattern %s", file, id)
	}
	return out
}

// TestSchematron compares the catalogs with the assertions of the NPa
// artefacts (FX_NL, a checkout of github.com/peppolautoriteit-nl/validation,
// see conformance/fetch-nl.sh): every assertion of the patterns Check
// implements, the G-account ones included, is in the catalog of its syntax,
// with its flag and its message, except those that never apply (see the
// package documentation), and the catalogs list nothing else.
func TestSchematron(t *testing.T) {
	dir := os.Getenv("FX_NL")
	if dir == "" {
		t.Skip("FX_NL is not set")
	}
	sch := filepath.Join(dir, "schematron")
	cii := patternAsserts(t, filepath.Join(sch, "nlcius-cii", "NLCIUS-CII-validation.sch"), "nlcius")
	ubl := slices.Concat(
		patternAsserts(t, filepath.Join(sch, "si-ubl-2.0.sch"), "SI-UBL-VERSION"),
		patternAsserts(t, filepath.Join(sch, "si-ubl-2.0", "si-ubl-2.0-nlcius.sch"), "nlcius"),
		patternAsserts(t, filepath.Join(sch, "si-ubl-2.0-ext-gaccount.sch"), "g-account-extension"),
	)
	for _, c := range []struct {
		name    string
		asserts []assertion
		catalog []rules.RuleInfo
		skip    func(assertion) bool // the assertion never applies
	}{
		{"CII", cii, Catalog, func(a assertion) bool {
			return a.id == "BR-NL-9" || a.id == "BR-NL-31"
		}},
		{"UBL", ubl, UBLCatalog, func(a assertion) bool {
			return a.id == "SI-INV-R000" || a.id == "BR-NL-32-2" || a.id == "BR-NL-32-3" ||
				strings.HasPrefix(a.message, "[BR-NL-34]")
		}},
	} {
		described := map[string]bool{"BR-NL-8": true} // the catalog describes both assertions
		seen := map[string]bool{}
		for _, a := range c.asserts {
			if c.skip(a) {
				continue
			}
			sev := map[string]rules.Severity{"fatal": rules.Error, "warning": rules.Warning}[a.flag]
			r, ok := catalogEntry(c.catalog, a.id)
			if !ok || sev == 0 || r.Severity != sev {
				t.Errorf("%s: %s (%s) is not in the catalog with its flag: %+v", c.name, a.id, a.flag, r)
				continue
			}
			seen[a.id] = true
			if info(a.id, sev, strings.Replace(a.message, "]-", "] ", 1)).Description == r.Description {
				described[a.id] = true
			}
		}
		for _, r := range c.catalog {
			if !seen[r.ID] || !described[r.ID] {
				t.Errorf("%s: %s (seen %v) is not described as in the schematron: %q", c.name, r.ID, seen[r.ID], r.Description)
			}
		}
	}
}

// nameRule and nameKind read the rule and the expected outcome in the name
// of a test file of SimplerInvoicing: SI-UBL-2.0_BR-NL-3_error_no_city.xml,
// NLCIUS-CII_error_BR-NL-10.xml.
var (
	nameRule = regexp.MustCompile(`BR-NL-[0-9]+`)
	nameKind = regexp.MustCompile(`_(ok|error|warning)[_.]`)
)

// testFileRules lists the rules of the test files whose name does not give
// them all: BR-NL-8 reports the type code 381 of an Invoice since SI-UBL
// 2.0.3, the type code 532 of a credit note also breaks BR-NL-7, the
// assertions with the message of BR-NL-34 never apply (their identifier is
// BR-NL-32-1 anyway), two files name no rule of BR-NL, and the files of the
// G-account extension name the case they test, not its rule.
var testFileRules = map[string][]string{
	"SI-UBL-2.0_BR-NL-7_error_381_invoice.xml":        {"BR-NL-8"},
	"SI-UBL-2.0_BR-NL-8_error_CN_not_381_2.xml":       {"BR-NL-7", "BR-NL-8"},
	"SI-UBL-2.0_BR-NL-34_warning_reasoncode.xml":      {"BR-NL-32-1"},
	"SI-UBL-2.0_BR-NL-34_warning_reasoncode_line.xml": {"BR-NL-32-1"},
	"SI-UBL-2.0_warning_empty_elements.xml":           {"SI-UBL-2"},
	"SI-UBL-2.0_error_no_customizationid.xml":         {"BII2-T10-R001"},

	// The test files of the G-account extension name no rule.
	"si-ubl-2.0-ext-gaccount_error_no_gaccount.xml":                  {"BR-GA-7"},
	"si-ubl-2.0-ext-gaccount_error_no_identifier.xml":                {"BR-GA-5"},
	"si-ubl-2.0-ext-gaccount_error_no_reference.xml":                 {"BR-GA-4"},
	"si-ubl-2.0-ext-gaccount_error_one-paymentmeans.xml":             {"BR-GA-2"},
	"si-ubl-2.0-ext-gaccount_error_one-paymentterm.xml":              {"BR-GA-1"},
	"si-ubl-2.0-ext-gaccount_error_paymentterms_wrong_reference.xml": {"BR-GA-6"},
	"si-ubl-2.0-ext-gaccount_error_sum_amount.xml":                   {"BR-GA-3"},
	"si-ubl-2.0-ext-gaccount_error_three-paymentmeans.xml":           {"BR-GA-2"},
	"si-ubl-2.0-ext-gaccount_error_three-paymentterms.xml":           {"BR-GA-1"},
}

// TestTestFiles runs Check on the test files of SimplerInvoicing for SI-UBL
// 2.0.3.2, NLCIUS-CII 1.0.3 and the G-account extension 1.0 (FX_NL_TESTS,
// the directory simplerinvoicing of the test files of phax/phive-rules, see
// conformance/fetch-nl.sh). Their name tells the rule they break and how
// (BR-NL-3_error, BR-NL-19_warning) or that they break no rule (_ok): Check
// reports that rule with that severity, and nothing else but SI-UBL-2 for an
// empty element (_empty). The files of the other rule sets (BR-S-08,
// UBL-SR-09...) give no finding.
func TestTestFiles(t *testing.T) {
	dir := os.Getenv("FX_NL_TESTS")
	if dir == "" {
		t.Skip("FX_NL_TESTS is not set")
	}
	files := 0
	for _, sub := range []string{"SI-UBL-2.0.3.2", "NLCIUS-CII-1.0.3", "si-ubl-2.0-ext-gaccount-1.0"} {
		eachInvoice(t, filepath.Join(dir, sub), func(path string, doc *rules.Doc) {
			files++
			name := filepath.Base(path)
			kind := ""
			if m := nameKind.FindStringSubmatch(name); m != nil {
				kind = m[1]
			}
			want, listed := testFileRules[name]
			if rule := nameRule.FindString(name); !listed && rule != "" && kind != "ok" {
				want = []string{rule}
				if strings.Contains(name, "_empty") {
					want = append(want, "SI-UBL-2")
				}
			}
			r := checkDoc(doc)
			checkFindings(t, r, catalogOf(doc))
			matched := map[string]bool{}
			for _, f := range r.Findings {
				i := slices.IndexFunc(want, func(w string) bool { return f.Rule == w || strings.HasPrefix(f.Rule, w+"-") })
				if i < 0 {
					t.Errorf("%s: unexpected %s", name, f)
					continue
				}
				sev := rules.Warning
				if kind == "error" && want[i] != "SI-UBL-2" {
					sev = rules.Error
				}
				if f.Severity != sev {
					t.Errorf("%s: %s is not a %s", name, f, sev)
				}
				matched[want[i]] = true
			}
			for _, w := range want {
				if !matched[w] {
					t.Errorf("%s: %s not reported:\n%s", name, w, r)
				}
			}
		})
	}
	if files < 100 {
		t.Errorf("%d test files, want at least 100", files)
	}
	t.Logf("%d files", files)
}
