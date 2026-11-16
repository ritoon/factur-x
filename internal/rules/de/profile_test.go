package de

import (
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

func TestCheckProfile(t *testing.T) {
	doc := load(t, base)
	for _, p := range invoice.Profiles {
		data := strings.Replace(doc, "<ram:ID>"+CIUSID+"</ram:ID>", "<ram:ID>"+p.URN()+"</ram:ID>", 1)
		root, err := xmltree.Parse([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		r := &rules.Report{}
		CheckProfile(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileUnknown), Report: r, Source: rules.SourceDE})
		want := p == invoice.ProfileMinimum || p == invoice.ProfileBasicWL
		if !want {
			if len(r.Findings) != 0 {
				t.Errorf("%s: unexpected findings\n%s", p, r)
			}
			continue
		}
		if len(r.Findings) != 1 {
			t.Fatalf("%s: %d findings, want 1\n%s", p, len(r.Findings), r)
		}
		f := r.Findings[0]
		if f.Rule != RuleProfile || f.Severity != rules.Error || f.Source != rules.SourceDE ||
			!strings.HasPrefix(f.Message, "["+RuleProfile+"] The profile "+p.String()+" ") ||
			!strings.HasSuffix(f.Path, "GuidelineSpecifiedDocumentContextParameter/ram:ID") || f.Line == 0 {
			t.Errorf("%s: bad finding %+v", p, f)
		}
	}

	// A forced profile wins over BT-24; a missing BT-24 locates the finding
	// at the root element.
	root, err := xmltree.Parse([]byte(`<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"/>`))
	if err != nil {
		t.Fatal(err)
	}
	r := &rules.Report{}
	CheckProfile(&rules.Checker{Doc: rules.NewDoc(root, invoice.ProfileBasicWL), Report: r, Source: rules.SourceDE})
	if len(r.Findings) != 1 || r.Findings[0].Path != "/rsm:CrossIndustryInvoice" {
		t.Errorf("forced BASIC WL without BT-24:\n%s", r)
	}
	CheckProfile(nil)
	CheckProfile(&rules.Checker{})
}

func TestProfileCatalog(t *testing.T) {
	if len(ProfileCatalog) != 1 || ProfileCatalog[0].ID != RuleProfile || ProfileCatalog[0].Severity != rules.Error {
		t.Errorf("ProfileCatalog = %+v", ProfileCatalog)
	}
	for _, r := range Catalog {
		if r.ID == RuleProfile {
			t.Errorf("%s is in Catalog, which lists the KoSIT assertions only", r.ID)
		}
	}
}
