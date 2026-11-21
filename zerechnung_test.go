package zerechnung_test

import (
	"errors"
	"os"
	"testing"

	"github.com/ritoon/zerechnung"
	"github.com/ritoon/zerechnung/invoice"
	"github.com/ritoon/zerechnung/validate"
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
	inv, att, err := zerechnung.Read(readFile(t, "testdata/EN16931_Einfach.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if att.Filename != "factur-x.xml" || inv.Profile != invoice.ProfileEN16931 || len(inv.Lines) == 0 {
		t.Errorf("Read: %s %s %d lines", att.Filename, inv.Profile, len(inv.Lines))
	}
	if _, _, err := zerechnung.Read(readFile(t, "testdata/host.pdf")); !errors.Is(err, zerechnung.ErrNoInvoice) {
		t.Errorf("Read of a plain PDF: %v, want ErrNoInvoice", err)
	}
}

func TestCheck(t *testing.T) {
	r, att, err := zerechnung.Check(readFile(t, "testdata/EN16931_Einfach.pdf"), nil)
	if err != nil || att == nil {
		t.Fatal(att, err)
	}
	if !r.Valid() {
		t.Errorf("official sample flagged:\n%s", r)
	}
	r, att, err = zerechnung.Check(readFile(t, "testdata/host.pdf"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if att != nil || r.Valid() {
		t.Errorf("plain PDF: attachment %v, valid %v", att, r.Valid())
	}
	// A UBL invoice in a PDF is not a Factur-X invoice.
	pdf, err := zerechnung.Embed(readFile(t, "testdata/host.pdf"), readFile(t, "testdata/EN16931_Einfach.ubl.xml"), &zerechnung.EmbedOptions{Profile: invoice.ProfileEN16931})
	if err != nil {
		t.Fatal(err)
	}
	r, att, err = zerechnung.Check(pdf, nil)
	if err != nil || att == nil {
		t.Fatal(att, err)
	}
	if r.Valid() || !r.Has("CII") {
		t.Errorf("UBL in a PDF:\n%s", r)
	}
}

func TestCreateProfiles(t *testing.T) {
	host := readFile(t, "testdata/host.pdf")
	for _, p := range invoice.Profiles {
		inv := newInvoice()
		pdf, err := zerechnung.Create(host, inv, &zerechnung.CreateOptions{Profile: p, Calculate: true})
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		back, att, err := zerechnung.Read(pdf)
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

func TestCreateAttachments(t *testing.T) {
	order := []byte("%PDF-1.7 purchase order")
	pdf, err := zerechnung.Create(readFile(t, "testdata/host.pdf"), newInvoice(), &zerechnung.CreateOptions{
		Calculate: true,
		Embed:     zerechnung.EmbedOptions{Attachments: []zerechnung.File{{Name: "commande.pdf", Data: order}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := zerechnung.ExtractAll(pdf)
	if err != nil || len(all) != 2 {
		t.Fatalf("%d attachments, %v", len(all), err)
	}
	if a := all[0]; a.Filename != "commande.pdf" || string(a.XML) != string(order) || a.Relationship != "Supplement" || a.MimeType != "application/pdf" {
		t.Errorf("attachment %+v", a)
	}
}

// TestCreateRelationship checks the /AFRelationship that Create admits for
// the seller's country, and how Check reports it (Factur-X 1.09, 6.2.2).
func TestCreateRelationship(t *testing.T) {
	host := readFile(t, "testdata/host.pdf")
	inv := newInvoice()
	inv.Seller.Address.CountryCode = "DE"
	if _, err := zerechnung.Create(host, inv, &zerechnung.CreateOptions{
		Calculate: true, Embed: zerechnung.EmbedOptions{Relationship: "Source"},
	}); err == nil {
		t.Error("Source for a seller in Germany: no error")
	}

	inv.Seller.Address.CountryCode = "FR"
	pdf, err := zerechnung.Create(host, inv, &zerechnung.CreateOptions{
		Calculate: true, Embed: zerechnung.EmbedOptions{Relationship: "Data"},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, att, err := zerechnung.Check(pdf, &validate.Options{Auto: true})
	if err != nil || att.Relationship != "Data" {
		t.Fatal(att, err)
	}
	if r.Has("PDF-14") || r.Has("PDF-40") {
		t.Errorf("Data for a seller in France flagged:\n%s", r)
	}
	if r, _, _ := zerechnung.Check(pdf, nil); !r.Has("PDF-14") {
		t.Error("Data without the French rules: no PDF-14")
	}
	if r, _, _ := zerechnung.Check(pdf, &validate.Options{Rules: validate.Default | validate.Germany}); !r.Has("PDF-40") {
		t.Error("Data with the German rules: no PDF-40")
	}
}

func TestCreateInvalid(t *testing.T) {
	inv := newInvoice()
	inv.Seller.Name = "" // BR-6
	_, err := zerechnung.Create(readFile(t, "testdata/host.pdf"), inv, &zerechnung.CreateOptions{
		Calculate: true, Validate: &validate.Options{},
	})
	var verr *zerechnung.ValidationError
	if !errors.As(err, &verr) || !verr.Report.Has("BR-06") {
		t.Fatalf("Create of an invalid invoice: %v", err)
	}
}

// TestCreateRequirePDFA checks that Create, with Embed.RequirePDFA, refuses
// a host that does not declare PDF/A (testdata/host.pdf has no XMP
// metadata) and accepts one that declares PDF/A-3.
func TestCreateRequirePDFA(t *testing.T) {
	host := readFile(t, "testdata/host.pdf")
	strict := &zerechnung.CreateOptions{Calculate: true, Embed: zerechnung.EmbedOptions{RequirePDFA: true}}
	if _, err := zerechnung.Create(host, newInvoice(), strict); !errors.Is(err, zerechnung.ErrNotPDFA) {
		t.Fatalf("host without PDF/A declaration: %v, want ErrNotPDFA", err)
	}
	pdfa, err := zerechnung.Create(host, newInvoice(), &zerechnung.CreateOptions{Calculate: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zerechnung.Create(pdfa, newInvoice(), strict); err != nil {
		t.Errorf("host declared PDF/A-3: %v", err)
	}
}

func TestDetectProfile(t *testing.T) {
	p, err := zerechnung.DetectProfile(readFile(t, "testdata/XRECHNUNG_Einfach.cii.xml"))
	if err != nil || p != invoice.ProfileXRechnung {
		t.Errorf("DetectProfile = %s, %v", p, err)
	}
}
