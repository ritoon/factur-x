// Package facturx reads, validates and creates Factur-X, ZUGFeRD and
// XRechnung hybrid invoices: a PDF that carries the invoice as CII XML.
//
// The package does not draw invoices. It takes a PDF produced by your own
// rendering engine and adds what the standard requires (the XML attachment,
// its file specification, the XMP metadata), with an incremental update that
// leaves the original bytes untouched. It reads and checks any hybrid
// invoice, whatever its profile.
//
// The main entry points are:
//
//   - Extract and Read get the XML, or the parsed invoice, out of a PDF.
//   - Embed adds an XML invoice to a PDF; Create does the same from an
//     invoice.Invoice.
//   - Check validates a hybrid PDF: container, XML schema, EN 16931 rules,
//     profile rules and, on request, French and German rules.
//
// Sub-packages give direct access to each layer: invoice (the EN 16931
// model), cii (CII XML), validate (validation of XML invoices).
package facturx

import (
	"errors"
	"fmt"

	"github.com/ritoon/factur-x/cii"
	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
	"github.com/ritoon/factur-x/validate"
)

// Read extracts the invoice XML from a hybrid PDF and parses it.
func Read(pdf []byte) (*invoice.Invoice, *Attachment, error) {
	att, err := Extract(pdf)
	if err != nil {
		return nil, nil, err
	}
	inv, err := cii.Parse(att.XML)
	if err != nil {
		return inv, att, err
	}
	return inv, att, nil
}

// CreateOptions tune Create.
type CreateOptions struct {
	// Profile is the profile to write; zero means inv.Profile, or EN 16931
	// when inv.Profile is not set.
	Profile invoice.Profile
	// Calculate runs inv.Calculate before writing the XML.
	Calculate bool
	// Validate, when not nil, validates the XML before embedding it and makes
	// Create fail with a *ValidationError when the report has errors.
	Validate *validate.Options
	// Embed tunes the PDF part. Its Profile field is ignored.
	Embed EmbedOptions
}

// ValidationError is returned by Create when the invoice is not valid.
type ValidationError struct {
	Report *validate.Report
}

func (e *ValidationError) Error() string {
	errs := e.Report.Errors()
	if len(errs) == 0 {
		return "facturx: invalid invoice"
	}
	return fmt.Sprintf("facturx: invalid invoice: %d error(s), first: %s", len(errs), errs[0])
}

// Create writes inv as CII XML for its profile and embeds it in pdf.
func Create(pdf []byte, inv *invoice.Invoice, opts *CreateOptions) ([]byte, error) {
	if opts == nil {
		opts = &CreateOptions{}
	}
	p := opts.Profile
	if p == invoice.ProfileUnknown {
		p = inv.Profile
	}
	if p == invoice.ProfileUnknown {
		p = invoice.ProfileEN16931
	}
	if opts.Calculate {
		inv.Calculate()
	}
	xml, err := cii.Marshal(inv, &cii.MarshalOptions{Profile: p})
	if err != nil {
		return nil, err
	}
	if opts.Validate != nil {
		vo := *opts.Validate
		vo.Profile = p
		if r := validate.XML(xml, &vo); !r.Valid() {
			return nil, &ValidationError{Report: r}
		}
	}
	eo := opts.Embed
	eo.Profile = p
	return Embed(pdf, xml, &eo)
}

// Check validates a hybrid PDF: the PDF container (PDF/A-3, attachment, XMP)
// and the embedded XML. The returned attachment is nil when no invoice was
// found. The error is reserved for PDFs that cannot be read at all.
//
// The national rule sets that apply to the XML (France, Germany,
// GermanyProfile, selected explicitly or through Auto, and Germany for an
// XRECHNUNG invoice, see validate.RulesFor) also select the national usage
// of the container: with the German rules, the /AFRelationship of a
// BASIC invoice or above must be Alternative (PDF-40); with the French
// rules, Data is admitted besides Alternative and Source (no PDF-14).
func Check(pdf []byte, opts *validate.Options) (*validate.Report, *Attachment, error) {
	if opts == nil {
		opts = &validate.Options{}
	}
	// The XML is parsed once: to find the national rules that apply to the
	// container, then to validate it.
	var root *xmltree.Node
	att, findings, err := checkContainer(pdf, func(xml []byte) countries {
		t, err := xmltree.Parse(xml)
		if err != nil {
			return 0
		}
		root = t
		if t.Name != "CrossIndustryInvoice" || t.Space != rules.NSRsm {
			return 0
		}
		return countriesOf(validate.RulesFor(rules.NewDoc(t, opts.Profile), opts))
	})
	if err != nil && att == nil && len(findings) == 0 {
		return nil, nil, err
	}
	r := &validate.Report{}
	switch {
	case att == nil || len(att.XML) == 0:
	case root != nil:
		r = validate.Tree(root, opts)
	default:
		r = validate.XML(att.XML, opts) // reports why the XML cannot be parsed
	}
	r.Findings = append(findings, r.Findings...)
	return r, att, nil
}

// DetectProfile returns the profile declared by the BT-24 of a CII invoice.
func DetectProfile(xml []byte) (invoice.Profile, error) {
	root, err := xmltree.Parse(xml)
	if err != nil {
		return invoice.ProfileUnknown, err
	}
	id := root.Value("ExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/ID")
	if id == "" {
		return invoice.ProfileUnknown, errors.New("facturx: no specification identifier (BT-24)")
	}
	return invoice.ProfileFromURN(id), nil
}
