package validate

import (
	"os"
	"strings"
	"testing"
)

func TestChorusPro(t *testing.T) {
	b, err := os.ReadFile("../internal/rules/choruspro/testdata/b2g.xml")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	required := PublicBuyer{ServiceCodeRequired: true, EngagementRequired: true}

	r := XML(b, &Options{Rules: Default | France | ChorusPro, PublicBuyer: required})
	if len(r.Findings) != 0 {
		t.Errorf("B2G invoice:\n%s", r)
	}

	noService := []byte(strings.Replace(doc, "<ram:BuyerReference>SERVICE-FINANCES</ram:BuyerReference>", "", 1))
	r = XML(noService, &Options{Rules: Default | ChorusPro, PublicBuyer: required})
	if !r.Has("CPRO-03") || r.Valid() {
		t.Errorf("missing service code not reported:\n%s", r)
	}
	for _, f := range r.Findings {
		if f.Rule == "CPRO-03" && f.Source != SourceChorusPro {
			t.Errorf("source = %q", f.Source)
		}
	}
	// The rule set is opt-in: neither Default nor Auto apply it.
	for _, opts := range []*Options{{PublicBuyer: required}, {Auto: true, PublicBuyer: required}} {
		if r := XML(noService, opts); r.Has("CPRO-03") {
			t.Errorf("ChorusPro applied with %+v", opts)
		}
	}
	// Without the setting, the service code is not required.
	if r := XML(noService, &Options{Rules: Default | ChorusPro}); r.Has("CPRO-03") {
		t.Errorf("service code required without PublicBuyer.ServiceCodeRequired:\n%s", r)
	}

	n := 0
	for _, ri := range Catalog(ChorusPro) {
		if strings.HasPrefix(ri.ID, "CPRO-") {
			n++
		}
	}
	if n == 0 || n != len(Catalog(ChorusPro)) {
		t.Errorf("Catalog(ChorusPro): %d CPRO rules out of %d", n, len(Catalog(ChorusPro)))
	}
}
