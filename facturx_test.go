package facturx_test

import (
	"errors"
	"os"
	"testing"

	facturx "github.com/ritoon/factur-x"
	"github.com/ritoon/factur-x/invoice"
	"github.com/ritoon/factur-x/validate"
)

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRead(t *testing.T) {
	inv, att, err := facturx.Read(readFile(t, "testdata/EN16931_Einfach.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if att.Filename != "factur-x.xml" || inv.Profile != invoice.ProfileEN16931 || len(inv.Lines) == 0 {
		t.Errorf("Read: %s %s %d lines", att.Filename, inv.Profile, len(inv.Lines))
	}
	if _, _, err := facturx.Read(readFile(t, "testdata/host.pdf")); !errors.Is(err, facturx.ErrNoInvoice) {
		t.Errorf("Read of a plain PDF: %v, want ErrNoInvoice", err)
	}
}

func TestCheck(t *testing.T) {
	r, att, err := facturx.Check(readFile(t, "testdata/EN16931_Einfach.pdf"), nil)
	if err != nil || att == nil {
		t.Fatal(att, err)
	}
	if !r.Valid() {
		t.Errorf("official sample flagged:\n%s", r)
	}
	r, att, err = facturx.Check(readFile(t, "testdata/host.pdf"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if att != nil || r.Valid() {
		t.Errorf("plain PDF: attachment %v, valid %v", att, r.Valid())
	}
}

func TestCreateProfiles(t *testing.T) {
	host := readFile(t, "testdata/host.pdf")
	for _, p := range invoice.Profiles {
		inv := newInvoice()
		pdf, err := facturx.Create(host, inv, &facturx.CreateOptions{Profile: p, Calculate: true})
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		back, att, err := facturx.Read(pdf)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if back.Profile != p || att.Filename != p.Filename() || att.Relationship != p.AFRelationship() ||
			att.Metadata == nil || att.Metadata.ConformanceLevel != p.ConformanceLevel() {
			t.Errorf("%s: read back %s, %s, %s, %+v", p, back.Profile, att.Filename, att.Relationship, att.Metadata)
		}
		if back.Totals.DuePayable.String() != "1080.00" {
			t.Errorf("%s: amount due %s", p, back.Totals.DuePayable)
		}
	}
}

func TestCreateInvalid(t *testing.T) {
	inv := newInvoice()
	inv.Seller.Name = "" // BR-6
	_, err := facturx.Create(readFile(t, "testdata/host.pdf"), inv, &facturx.CreateOptions{
		Calculate: true, Validate: &validate.Options{},
	})
	var verr *facturx.ValidationError
	if !errors.As(err, &verr) || !verr.Report.Has("BR-06") {
		t.Fatalf("Create of an invalid invoice: %v", err)
	}
}

func TestDetectProfile(t *testing.T) {
	p, err := facturx.DetectProfile(readFile(t, "testdata/XRECHNUNG_Einfach.cii.xml"))
	if err != nil || p != invoice.ProfileXRechnung {
		t.Errorf("DetectProfile = %s, %v", p, err)
	}
}
