package peppol

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// TestOfficial runs the CII unit tests of the Peppol BIS Billing 3.0
// repository (FX_PEPPOL, the root of a clone of OpenPEPPOL/peppol-bis-invoice-3
// at release 3.0.21 or later): the folders rules/unit-CII-*, which the
// official build runs against PEPPOL-EN16931-CII.sch.
//
// Each test of a test set gives the expected outcome of some rules:
// <success>id</success> when the rule must not fire, <error>id</error> or
// <warning>id</warning> when it fires once (or number times) with that
// severity. Rules not listed are not checked.
func TestOfficial(t *testing.T) {
	dir := os.Getenv("FX_PEPPOL")
	if dir == "" {
		t.Skip("FX_PEPPOL is not set")
	}
	folders, err := filepath.Glob(filepath.Join(dir, "rules", "unit-CII-*"))
	if err != nil {
		t.Fatal(err)
	}
	tests, sets := 0, 0
	covered := map[string]bool{}
	for _, folder := range folders {
		files, _ := filepath.Glob(filepath.Join(folder, "*.xml")) // as the build: .xml only
		for _, file := range files {
			sets++
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			set, err := xmltree.Parse(data)
			if err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			for i, test := range set.Elems("test") {
				tests++
				name := filepath.Base(folder) + "/" + filepath.Base(file) + "#" + strconv.Itoa(i+1)
				root := test.Elem("CrossIndustryInvoice")
				if root == nil {
					t.Errorf("%s: no CrossIndustryInvoice", name)
					continue
				}
				root.Parent = nil
				r := &rules.Report{}
				Check(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileUnknown), Report: r, Source: rules.SourcePeppol})
				checkFindings(t, r)
				for _, a := range test.Elem("assert").Children {
					id := strings.TrimSpace(a.Text)
					covered[id] = true
					if byID[id] == nil {
						t.Errorf("%s: rule %s is not in Catalog", name, id)
					}
					want, sev := 1, rules.Severity(0)
					switch a.Name {
					case "success":
						want = 0
					case "error":
						sev = rules.Error
					case "warning":
						sev = rules.Warning
					default:
						continue
					}
					if n := a.Attr("number"); n != "" {
						want, _ = strconv.Atoi(n)
					}
					got := 0
					for _, f := range r.Findings {
						if f.Rule == id && (sev == 0 || f.Severity == sev) {
							got++
						}
					}
					if got != want {
						t.Errorf("%s: %s %s fired %d times, want %d\n%s", name, a.Name, id, got, want, r)
					}
				}
			}
		}
	}
	t.Logf("%d test sets, %d tests, %d rules covered", sets, tests, len(covered))
	if tests == 0 {
		t.Error("no unit test in FX_PEPPOL")
	}
	for _, r := range Catalog {
		if !covered[r.ID] {
			t.Logf("no official unit test for %s", r.ID)
		}
	}
}
