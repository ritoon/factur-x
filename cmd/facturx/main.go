// Command facturx reads, validates and creates Factur-X / ZUGFeRD /
// XRechnung invoices.
//
//	facturx extract [-o out.xml] invoice.pdf
//	facturx info invoice.pdf
//	facturx validate [-fr] [-de] [-peppol] [-auto] [-reject-basicwl] [-choruspro [-service-required] [-engagement-required]] [-profile p] [-json] invoice.pdf|invoice.xml
//	facturx embed -xml invoice.xml [-profile p] [-attach file]... [-o out.pdf] input.pdf
//	facturx json invoice.pdf|invoice.xml
//	facturx rules [-fr] [-de] [-choruspro] [-peppol]
//
// validate exits with status 1 when the invoice has errors.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	facturx "github.com/ritoon/factur-x"
	"github.com/ritoon/factur-x/cii"
	"github.com/ritoon/factur-x/invoice"
	"github.com/ritoon/factur-x/validate"
)

const usage = `facturx reads, validates and creates Factur-X / ZUGFeRD / XRechnung invoices.

Usage:
  facturx extract [-o out.xml] invoice.pdf
  facturx info invoice.pdf
  facturx validate [-fr] [-de] [-peppol] [-auto] [-reject-basicwl] [-choruspro [-service-required] [-engagement-required]] [-profile p] [-json] invoice.pdf|invoice.xml
  facturx embed -xml invoice.xml [-profile p] [-attach file]... [-o out.pdf] input.pdf
  facturx json invoice.pdf|invoice.xml
  facturx rules [-fr] [-de] [-choruspro] [-peppol]

Profiles: minimum, basicwl, basic, en16931, extended, xrechnung, extended-ctc-fr, peppol.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	code := 0
	switch cmd {
	case "extract":
		err = extract(args)
	case "info":
		err = info(args)
	case "validate":
		code, err = validateCmd(args, os.Stdout)
	case "embed":
		err = embed(args)
	case "json":
		err = jsonCmd(args)
	case "rules":
		err = rulesCmd(args)
	case "help", "-h", "-help", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "facturx: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "facturx:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

func oneFile(fs *flag.FlagSet, args []string) (string, []byte, error) {
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	if fs.NArg() != 1 {
		return "", nil, fmt.Errorf("%s: expected one file, got %d", fs.Name(), fs.NArg())
	}
	name := fs.Arg(0)
	var data []byte
	var err error
	if name == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(name)
	}
	return name, data, err
}

func isPDF(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimLeft(data[:min(len(data), 1024)], "\x00\t\n\r "), []byte("%PDF-"))
}

func writeOut(path string, data []byte) error {
	if path == "" || path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func extract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	out := fs.String("o", "", "output file (default: standard output)")
	_, data, err := oneFile(fs, args)
	if err != nil {
		return err
	}
	att, err := facturx.Extract(data)
	if err != nil {
		return err
	}
	return writeOut(*out, att.XML)
}

func info(args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	_, data, err := oneFile(fs, args)
	if err != nil {
		return err
	}
	att, err := facturx.Extract(data)
	if err != nil {
		return err
	}
	w := os.Stdout
	fmt.Fprintf(w, "PDF version:        %s\n", att.PDFVersion)
	fmt.Fprintf(w, "Attachment:         %s (%d bytes, %s)\n", att.Filename, len(att.XML), orDash(att.MimeType))
	fmt.Fprintf(w, "AFRelationship:     %s (in /AF: %v)\n", orDash(att.Relationship), att.InAF)
	if !att.ModDate.IsZero() {
		fmt.Fprintf(w, "Modification date:  %s\n", att.ModDate.Format(time.RFC3339))
	}
	if m := att.Metadata; m != nil {
		if m.PDFARev != "" {
			fmt.Fprintf(w, "PDF/A:              %d%s (rev %s)\n", m.PDFAPart, m.PDFAConformance, m.PDFARev)
		} else {
			fmt.Fprintf(w, "PDF/A:              %d%s\n", m.PDFAPart, m.PDFAConformance)
		}
		fmt.Fprintf(w, "XMP namespace:      %s\n", m.NamespaceURI)
		fmt.Fprintf(w, "XMP profile:        %s (document %s, file %s, version %s)\n", m.ConformanceLevel, m.DocumentType, m.DocumentFileName, m.Version)
	} else {
		fmt.Fprintln(w, "XMP:                none")
	}
	inv, err := cii.Parse(att.XML)
	if inv == nil {
		return err
	}
	fmt.Fprintf(w, "Profile (BT-24):    %s\n", inv.Profile)
	fmt.Fprintf(w, "Invoice:            %s, type %s, issued %s\n", inv.Number, inv.TypeCode, inv.IssueDate)
	fmt.Fprintf(w, "Seller:             %s (%s)\n", inv.Seller.Name, inv.Seller.Address.CountryCode)
	fmt.Fprintf(w, "Buyer:              %s (%s)\n", inv.Buyer.Name, inv.Buyer.Address.CountryCode)
	fmt.Fprintf(w, "Total:              %s %s without VAT, %s with VAT, %s due\n", inv.Totals.TaxBasisTotal, inv.Currency, inv.Totals.GrandTotal, inv.Totals.DuePayable)
	fmt.Fprintf(w, "Lines:              %d\n", len(inv.Lines))
	return nil
}

// validateCmd runs the validate command and writes the report to w.
func validateCmd(args []string, w io.Writer) (int, error) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fr := fs.Bool("fr", false, "apply the French rules BR-FR and the profiles the French reform admits")
	de := fs.Bool("de", false, "apply the German rules BR-DE (XRechnung) and the profiles the BMF accepts (BR-DE applied anyway to XRECHNUNG invoices)")
	auto := fs.Bool("auto", false, "apply the national rules of the seller's country")
	rejectBasicWL := fs.Bool("reject-basicwl", false, "report the profile BASIC WL as an error under the French rules")
	cpro := fs.Bool("choruspro", false, "apply the Chorus Pro checks of French B2G invoices")
	serviceRequired := fs.Bool("service-required", false, "with -choruspro: the public buyer requires a service code (BT-10)")
	engagementRequired := fs.Bool("engagement-required", false, "with -choruspro: the public buyer requires an engagement number (BT-13)")
	pep := fs.Bool("peppol", false, "apply the Peppol BIS Billing 3.0 rules (applied anyway when BT-24 is the Peppol identifier)")
	profile := fs.String("profile", "", "validate against this profile instead of BT-24")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	_, data, err := oneFile(fs, args)
	if err != nil {
		return 0, err
	}
	opts := &validate.Options{Auto: *auto, RejectBasicWL: *rejectBasicWL}
	if *profile != "" {
		if opts.Profile = invoice.ParseProfile(*profile); opts.Profile == invoice.ProfileUnknown {
			return 0, fmt.Errorf("unknown profile %q", *profile)
		}
	}
	if *fr || *de || *cpro || *pep {
		opts.Rules = validate.Default
		if *fr {
			opts.Rules |= validate.France
		}
		if *de {
			opts.Rules |= validate.Germany | validate.GermanyProfile
		}
		if *cpro {
			opts.Rules |= validate.ChorusPro
		}
		if *pep {
			opts.Rules |= validate.Peppol
		}
	}
	opts.PublicBuyer = validate.PublicBuyer{ServiceCodeRequired: *serviceRequired, EngagementRequired: *engagementRequired}
	var r *validate.Report
	if isPDF(data) {
		r, _, err = facturx.Check(data, opts)
		if err != nil {
			return 0, err
		}
	} else {
		r = validate.XML(data, opts)
	}
	if *asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		out := struct {
			Profile  string             `json:"profile"`
			Valid    bool               `json:"valid"`
			Findings []validate.Finding `json:"findings"`
		}{r.Profile.String(), r.Valid(), r.Findings}
		if out.Findings == nil {
			out.Findings = []validate.Finding{}
		}
		if err := enc.Encode(out); err != nil {
			return 0, err
		}
	} else {
		fmt.Fprintf(w, "Profile: %s\n", r.Profile)
		for _, f := range r.Findings {
			fmt.Fprintln(w, f)
		}
		fmt.Fprintf(w, "%d error(s), %d warning(s)\n", len(r.Errors()), len(r.Warnings()))
	}
	if !r.Valid() {
		return 1, nil
	}
	return 0, nil
}

// fileList is a repeatable flag.
type fileList []string

func (l *fileList) String() string     { return strings.Join(*l, ",") }
func (l *fileList) Set(s string) error { *l = append(*l, s); return nil }

func embed(args []string) error {
	fs := flag.NewFlagSet("embed", flag.ContinueOnError)
	xmlPath := fs.String("xml", "", "invoice XML to embed (required)")
	profile := fs.String("profile", "", "profile (default: read from BT-24)")
	rel := fs.String("relationship", "", "AFRelationship (default: Data for MINIMUM and BASIC WL, Alternative otherwise; Germany and XRECHNUNG admit only Alternative)")
	var attach fileList
	fs.Var(&attach, "attach", "supplementary file to embed besides the invoice, with AFRelationship Supplement and the MIME type of its extension (repeatable)")
	out := fs.String("o", "", "output PDF (default: standard output)")
	_, pdf, err := oneFile(fs, args)
	if err != nil {
		return err
	}
	if *xmlPath == "" {
		return errors.New("embed: -xml is required")
	}
	xml, err := os.ReadFile(*xmlPath)
	if err != nil {
		return err
	}
	opts := &facturx.EmbedOptions{Relationship: *rel}
	for _, p := range attach {
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		opts.Attachments = append(opts.Attachments, facturx.File{Name: filepath.Base(p), Data: data})
	}
	if *profile != "" {
		if opts.Profile = invoice.ParseProfile(*profile); opts.Profile == invoice.ProfileUnknown {
			return fmt.Errorf("unknown profile %q", *profile)
		}
	}
	res, err := facturx.Embed(pdf, xml, opts)
	if err != nil {
		return err
	}
	return writeOut(*out, res)
}

func jsonCmd(args []string) error {
	fs := flag.NewFlagSet("json", flag.ContinueOnError)
	_, data, err := oneFile(fs, args)
	if err != nil {
		return err
	}
	if isPDF(data) {
		att, err := facturx.Extract(data)
		if err != nil {
			return err
		}
		data = att.XML
	}
	inv, err := cii.Parse(data)
	if inv == nil {
		return err
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "facturx: warning:", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(inv)
}

func rulesCmd(args []string) error {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fr := fs.Bool("fr", false, "include the French rules")
	de := fs.Bool("de", false, "include the German rules")
	cpro := fs.Bool("choruspro", false, "include the Chorus Pro checks")
	pep := fs.Bool("peppol", false, "include the Peppol BIS Billing 3.0 rules")
	if err := fs.Parse(args); err != nil {
		return err
	}
	set := validate.Default
	if *fr {
		set |= validate.France
	}
	if *de {
		set |= validate.Germany | validate.GermanyProfile
	}
	if *cpro {
		set |= validate.ChorusPro
	}
	if *pep {
		set |= validate.Peppol
	}
	for _, r := range validate.Catalog(set) {
		fmt.Printf("%-28s %-8s %s\n", r.ID, r.Severity, strings.TrimSpace(r.Description))
	}
	return nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
