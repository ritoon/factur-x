// Package validate checks CII and UBL invoices: CII invoices against the XML
// schema of their Factur-X profile, the EN 16931 business rules, the
// Factur-X profile rules, and optionally the French (BR-FR) and German
// (BR-DE) rules, the profiles that France and Germany admit, and the checks
// of Chorus Pro for French B2G invoices.
//
// The syntax is detected from the root element. A UBL 2.1 Invoice or
// CreditNote is checked against the EN 16931 rules of the UBL artefacts:
// the business rules and code lists, and the UBL syntax rules UBL-CR, UBL-SR
// and UBL-DT. The Factur-X profile rules, which are CII rules, do not apply.
// The French, German and Chorus Pro rules run on a CII view of the UBL
// document, with the rule identifiers of their CII schematrons; their
// findings are located in the UBL document. The UBL 2.1 XML schema is not
// checked: the Schema rule set only reports it (XSD-UBL, information).
//
// Validation never stops at the first problem: every finding is collected in
// a Report. A document that is not well-formed XML, or neither a CII nor a
// UBL invoice, gives a report with a single syntax error.
package validate

import (
	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/rules/choruspro"
	"github.com/ritoon/factur-x/internal/rules/de"
	"github.com/ritoon/factur-x/internal/rules/en16931"
	"github.com/ritoon/factur-x/internal/rules/facturx"
	"github.com/ritoon/factur-x/internal/rules/fr"
	"github.com/ritoon/factur-x/internal/rules/ubl"
	"github.com/ritoon/factur-x/internal/schema"
	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// Finding is a rule violation.
type Finding = rules.Finding

// Severity is the severity of a finding.
type Severity = rules.Severity

// Report is the result of a validation.
type Report = rules.Report

// RuleInfo describes an implemented rule.
type RuleInfo = rules.RuleInfo

// Severities.
const (
	Error   = rules.Error
	Warning = rules.Warning
	Info    = rules.Info
)

// Sources of findings.
const (
	SourceSyntax  = rules.SourceSyntax
	SourceEN16931 = rules.SourceEN16931
	SourceFacturX = rules.SourceFacturX
	SourceFR      = rules.SourceFR
	SourceDE      = rules.SourceDE
	SourcePDF     = rules.SourcePDF

	SourceChorusPro = rules.SourceChorusPro
)

// RuleSet selects groups of rules.
type RuleSet uint

// Rule sets.
const (
	Schema         RuleSet = 1 << iota // structure against the XML schema of the profile (CII only)
	EN16931                            // CEN business rules and code lists, and the UBL syntax rules for UBL
	FacturX                            // Factur-X / ZUGFeRD profile rules (CII only)
	France                             // French rules BR-FR (2026 reform) and the profiles the reform admits (FR-PROFILE-*)
	Germany                            // German rules BR-DE (XRechnung)
	GermanyProfile                     // profiles the BMF accepts as e-invoices (DE-PROFILE-01: not MINIMUM nor BASIC WL)
	ChorusPro                          // French B2G checks for Chorus Pro (CPRO-*), see Options.PublicBuyer

	// Default is the set used when Options.Rules is zero: schema, EN 16931
	// and profile rules, plus the German rules for XRECHNUNG invoices.
	Default = Schema | EN16931 | FacturX
)

// Options tune a validation. The zero value is ready to use.
type Options struct {
	// Profile overrides the profile read from BT-24.
	Profile invoice.Profile
	// Rules selects the rule sets; zero means Default.
	Rules RuleSet
	// Auto adds the national rules of the seller's country (BT-40): the
	// French rules (France) for a seller in France, the BMF profile check
	// (GermanyProfile) for a seller in Germany. The German rules BR-DE need
	// no option: they apply to XRECHNUNG invoices, which declare themselves
	// in BT-24.
	Auto bool
	// RejectBasicWL makes the French rules report the profile BASIC WL as an
	// error (FR-PROFILE-03). The reform admits BASIC WL for now, and no end
	// date has been published: set it to refuse BASIC WL already, or once
	// the profile is no longer admitted.
	RejectBasicWL bool

	// PublicBuyer gives the settings of the public buyer in the Chorus Pro
	// directory, used by the ChorusPro rules.
	PublicBuyer PublicBuyer
}

// PublicBuyer holds the settings of a public structure in the Chorus Pro
// directory that change the ChorusPro rules. The zero value requires
// neither a service code nor an engagement number.
type PublicBuyer struct {
	// ServiceCodeRequired: the structure requires a service code, carried by
	// the buyer reference (BT-10).
	ServiceCodeRequired bool
	// EngagementRequired: the structure requires an engagement number,
	// carried by the purchase order reference (BT-13); Chorus Pro also
	// accepts the contract reference (BT-12) instead.
	EngagementRequired bool
}

// RuleUBLSchema is the information reported by the Schema rule set for a
// UBL invoice, whose XML schema is not checked.
const RuleUBLSchema = "XSD-UBL"

var ublSchemaInfo = RuleInfo{ID: RuleUBLSchema, Severity: Info,
	Description: "The structure of a UBL invoice is not checked against the UBL 2.1 XML schema; the UBL syntax rules (UBL-CR, UBL-SR, UBL-DT) check the elements EN 16931 uses."}

// XML validates a CII or UBL invoice.
func XML(data []byte, opts *Options) *Report {
	if opts == nil {
		opts = &Options{}
	}
	root, err := xmltree.Parse(data)
	if err != nil {
		r := &Report{}
		r.Add(Finding{Rule: "XML", Severity: Error, Source: SourceSyntax, Message: err.Error()})
		return r
	}
	return Tree(root, opts)
}

// Tree validates an already parsed CII or UBL invoice.
func Tree(root *xmltree.Node, opts *Options) *Report {
	if opts == nil {
		opts = &Options{}
	}
	r := &Report{}
	var doc *rules.Doc
	switch {
	case root.Name == "CrossIndustryInvoice" && root.Space == rules.NSRsm:
		doc = rules.NewDoc(root, opts.Profile)
	case ubl.IsUBL(root):
		doc = ubl.View(root, opts.Profile)
	default:
		r.Add(Finding{Rule: "CII", Severity: Error, Source: SourceSyntax, Path: "/" + root.Name, Line: root.Line,
			Message: "the root element is neither rsm:CrossIndustryInvoice in namespace " + rules.NSRsm +
				" nor a UBL 2.1 Invoice or CreditNote"})
		return r
	}
	r.Profile = doc.Profile
	set := RulesFor(doc, opts)
	isUBL := doc.UBL != nil
	run := func(s RuleSet, source string, check func(*rules.Checker)) {
		if set&s != 0 {
			check(&rules.Checker{Doc: doc, Report: r, Source: source})
		}
	}
	if isUBL {
		if set&Schema != 0 {
			r.Add(Finding{Rule: RuleUBLSchema, Severity: Info, Source: SourceSyntax, Path: "/" + root.Name, Line: root.Line,
				Message: "the structure of the UBL invoice is not checked against the UBL 2.1 XML schema"})
		}
		run(EN16931, SourceEN16931, func(c *rules.Checker) {
			en16931.Check(c)
			ubl.Check(c)
		})
	} else {
		run(Schema, SourceSyntax, schema.Check)
		run(EN16931, SourceEN16931, en16931.Check)
		run(FacturX, SourceFacturX, facturx.Check)
	}
	run(France, SourceFR, func(c *rules.Checker) {
		fr.Check(c)
		fr.CheckProfile(c, opts.RejectBasicWL)
	})
	run(Germany, SourceDE, de.Check)
	run(GermanyProfile, SourceDE, de.CheckProfile)
	run(ChorusPro, SourceChorusPro, func(c *rules.Checker) {
		choruspro.Check(c, choruspro.Settings{
			ServiceCodeRequired: opts.PublicBuyer.ServiceCodeRequired,
			EngagementRequired:  opts.PublicBuyer.EngagementRequired,
		})
	})
	return r
}

// RulesFor returns the rule sets that apply to doc with opts.
func RulesFor(doc *rules.Doc, opts *Options) RuleSet {
	set := opts.Rules
	if set == 0 {
		set = Default
		if doc.Profile == invoice.ProfileXRechnung {
			set |= Germany
		}
	}
	if opts.Auto {
		switch doc.Agreement.Value("SellerTradeParty/PostalTradeAddress/CountryID") {
		case "FR":
			set |= France
		case "DE":
			set |= GermanyProfile
		}
	}
	return set
}

// Catalog returns the rules implemented by the selected rule sets.
func Catalog(set RuleSet) []RuleInfo {
	var out []RuleInfo
	add := func(s RuleSet, c []RuleInfo) {
		if set&s != 0 {
			out = append(out, c...)
		}
	}
	add(Schema, schema.Catalog)
	add(Schema, []RuleInfo{ublSchemaInfo})
	add(EN16931, en16931.Catalog)
	add(EN16931, ubl.Catalog)
	add(FacturX, facturx.Catalog)
	add(France, fr.Catalog)
	add(France, fr.ProfileCatalog)
	add(Germany, de.Catalog)
	add(GermanyProfile, de.ProfileCatalog)
	add(ChorusPro, choruspro.Catalog)
	return out
}
