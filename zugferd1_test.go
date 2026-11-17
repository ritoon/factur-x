package zerechnung

import (
	"slices"
	"testing"

	"github.com/ritoon/zerechnung/internal/xmp"
	"github.com/ritoon/zerechnung/invoice"
)

// TestZUGFeRD1PDF reads and checks a ZUGFeRD 1.0 PDF.
func TestZUGFeRD1PDF(t *testing.T) {
	data := readTestdata(t, "ZUGFeRD_1p0_COMFORT_SEPA_Prenotification.xml")
	f := valid(data)
	f.files[0].name = "ZUGFeRD-invoice.xml"
	f.xmp = xmpWith(func(p *xmp.Params) {
		p.NamespaceURI, p.DocumentFileName, p.ConformanceLevel = xmp.NSZUGFeRD1, "ZUGFeRD-invoice.xml", "COMFORT"
	})
	pdf := f.build()

	inv, att, err := Read(pdf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if att.Filename != "ZUGFeRD-invoice.xml" || inv.Profile != invoice.ProfileEN16931 ||
		inv.SpecificationID != invoice.URNZUGFeRD1Comfort || inv.Number != "471102" || inv.Totals.GrandTotal.String() != "529.87" {
		t.Errorf("Read = %q, %v (%q), %q, %v", att.Filename, inv.Profile, inv.SpecificationID, inv.Number, inv.Totals.GrandTotal)
	}

	// The legacy attachment name and the scope of the ZUGFeRD 1.0 check are
	// reported, as warnings.
	r, _, err := Check(pdf, nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var got []string
	for _, f := range r.Findings {
		got = append(got, f.Rule)
	}
	if !slices.Equal(got, []string{"PDF-11", "ZF1-04"}) || !r.Valid() || r.Profile != invoice.ProfileEN16931 {
		t.Errorf("Check: valid %v, profile %v, findings %v", r.Valid(), r.Profile, r.Findings)
	}
}

func TestDetectProfileZUGFeRD1(t *testing.T) {
	cases := []struct {
		level string
		want  invoice.Profile
	}{
		{"basic", invoice.ProfileBasic},
		{"comfort", invoice.ProfileEN16931},
		{"extended", invoice.ProfileExtended},
		{"komfort", invoice.ProfileUnknown},
	}
	for _, c := range cases {
		if p, err := DetectProfile(zf1XML(c.level)); err != nil || p != c.want {
			t.Errorf("DetectProfile(%s) = %v, %v; want %v", c.level, p, err, c.want)
		}
	}
}
