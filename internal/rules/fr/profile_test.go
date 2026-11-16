package fr

import (
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

func TestCheckProfile(t *testing.T) {
	doc := load(t, baseEN16931)
	for _, c := range []struct {
		profile       invoice.Profile
		rejectBasicWL bool
		rule          string
		sev           rules.Severity
	}{
		{invoice.ProfileMinimum, false, RuleProfileMinimum, rules.Error},
		{invoice.ProfileBasicWL, false, "", 0},
		{invoice.ProfileBasicWL, true, RuleProfileBasicWL, rules.Error},
		{invoice.ProfileBasic, false, RuleProfileBasic, rules.Warning},
		{invoice.ProfileBasic, true, RuleProfileBasic, rules.Warning},
		{invoice.ProfileEN16931, true, "", 0},
		{invoice.ProfileExtended, true, "", 0},
		{invoice.ProfileExtendedCTCFR, true, "", 0},
		{invoice.ProfileXRechnung, true, "", 0},
		{invoice.ProfileUnknown, true, "", 0},
	} {
		urn := c.profile.URN()
		if c.profile == invoice.ProfileUnknown {
			urn = "urn:example:unknown"
		}
		data := strings.Replace(doc, "<ram:ID>urn:cen.eu:en16931:2017</ram:ID>", "<ram:ID>"+urn+"</ram:ID>", 1)
		root, err := xmltree.Parse([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		r := &rules.Report{}
		CheckProfile(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileUnknown), Report: r, Source: rules.SourceFR}, c.rejectBasicWL)
		if c.rule == "" {
			if len(r.Findings) != 0 {
				t.Errorf("%s (reject BASIC WL %v): unexpected findings\n%s", c.profile, c.rejectBasicWL, r)
			}
			continue
		}
		if len(r.Findings) != 1 {
			t.Fatalf("%s (reject BASIC WL %v): %d findings, want 1\n%s", c.profile, c.rejectBasicWL, len(r.Findings), r)
		}
		f := r.Findings[0]
		if f.Rule != c.rule || f.Severity != c.sev || f.Source != rules.SourceFR ||
			!strings.HasPrefix(f.Message, c.rule+" : ") || !strings.HasSuffix(f.Path, "GuidelineSpecifiedDocumentContextParameter/ram:ID") || f.Line == 0 {
			t.Errorf("%s: bad finding %+v", c.profile, f)
		}
		if !inProfileCatalog(f.Rule, f.Severity) {
			t.Errorf("%s: %s (%s) is not in ProfileCatalog", c.profile, f.Rule, f.Severity)
		}
	}
}

// TestCheckProfileForced checks that the profile forced by the caller wins
// over BT-24, and that a document without BT-24 is located at the root.
func TestCheckProfileForced(t *testing.T) {
	root, err := xmltree.Parse([]byte(load(t, baseEN16931)))
	if err != nil {
		t.Fatal(err)
	}
	r := &rules.Report{}
	CheckProfile(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileMinimum), Report: r, Source: rules.SourceFR}, false)
	if !r.Has(RuleProfileMinimum) {
		t.Errorf("forced MINIMUM not reported:\n%s", r)
	}

	root, err = xmltree.Parse([]byte(`<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"/>`))
	if err != nil {
		t.Fatal(err)
	}
	r = &rules.Report{}
	CheckProfile(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileBasic), Report: r, Source: rules.SourceFR}, false)
	if len(r.Findings) != 1 || r.Findings[0].Path != "/rsm:CrossIndustryInvoice" {
		t.Errorf("document without BT-24:\n%s", r)
	}

	CheckProfile(nil, true)
	CheckProfile(&rules.Checker{}, true)
}

func TestProfileCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range ProfileCatalog {
		if seen[r.ID] || !strings.HasPrefix(r.ID, "FR-PROFILE-") || r.Description == "" || inCatalog(r.ID) {
			t.Errorf("bad entry %+v", r)
		}
		seen[r.ID] = true
	}
	if len(ProfileCatalog) != 3 {
		t.Errorf("%d entries, want 3", len(ProfileCatalog))
	}
}

func inProfileCatalog(id string, sev rules.Severity) bool {
	for _, r := range ProfileCatalog {
		if r.ID == id && r.Severity == sev {
			return true
		}
	}
	return false
}
