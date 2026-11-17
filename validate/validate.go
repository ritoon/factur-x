// Package validate checks CII invoices against the XML schema of their
// Factur-X profile, the EN 16931 business rules, the Factur-X profile rules,
// and optionally the French (BR-FR) and German (BR-DE) rules, the profiles
// that France and Germany admit, the checks of Chorus Pro for French B2G
// invoices, and the Peppol BIS Billing 3.0 rules.
//
// Validation never stops at the first problem: every finding is collected in
// a Report. A document that is not well-formed XML, or not a CII invoice,
// gives a report with a single syntax error.
package validate

import (
	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/rules/choruspro"
	"github.com/ritoon/factur-x/internal/rules/de"
	"github.com/ritoon/factur-x/internal/rules/en16931"
	"github.com/ritoon/factur-x/internal/rules/facturx"
	"github.com/ritoon/factur-x/internal/rules/fr"
	"github.com/ritoon/factur-x/internal/rules/peppol"
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
	SourcePeppol    = rules.SourcePeppol
)

// RuleSet selects groups of rules.
type RuleSet uint

// Rule sets.
const (
	Schema         RuleSet = 1 << iota // structure against the XML schema of the profile
	EN16931                            // CEN business rules and code lists
	FacturX                            // Factur-X / ZUGFeRD profile rules
	France                             // French rules BR-FR (2026 reform) and the profiles the reform admits (FR-PROFILE-*)
	Germany                            // German rules BR-DE (XRechnung)
	GermanyProfile                     // profiles the BMF accepts as e-invoices (DE-PROFILE-01: not MINIMUM nor BASIC WL)
	ChorusPro                          // French B2G checks for Chorus Pro (CPRO-*), see Options.PublicBuyer
	Peppol                             // Peppol BIS Billing 3.0 rules (PEPPOL-*) and their national rules (NO, DK, IT, SE, NL)

	// Default is the set used when Options.Rules is zero: schema, EN 16931
	// and profile rules. Whatever the rule sets, the German rules apply to
	// XRECHNUNG invoices and the Peppol rules to invoices whose BT-24 is the
	// Peppol BIS Billing 3.0 identifier (see RulesFor).
	Default = Schema | EN16931 | FacturX
)

// Options tune a validation. The zero value is ready to use.
type Options struct {
	// Profile overrides the profile read from BT-24.
	Profile invoice.Profile
	// Rules selects the rule sets; zero means Default. The German rules for
	// an XRECHNUNG invoice and the Peppol rules for a Peppol invoice are
	// added whatever Rules: selecting other sets, such as Default | France,
	// never drops them.
	Rules RuleSet
	// Auto adds the national rules of the seller's country (BT-40): the
	// French rules (France) for a seller in France, the BMF profile check
	// (GermanyProfile) for a seller in Germany. The German rules BR-DE and
	// the Peppol rules need no option: they apply to XRECHNUNG and Peppol
	// invoices, which declare themselves in BT-24. Auto never adds the
	// Peppol rules otherwise: most Factur-X and ZUGFeRD invoices do not meet
	// them (no electronic address, no buyer reference).
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

// XML validates a CII invoice.
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

// Tree validates an already parsed CII invoice.
func Tree(root *xmltree.Node, opts *Options) *Report {
	if opts == nil {
		opts = &Options{}
	}
	r := &Report{}
	if root.Name != "CrossIndustryInvoice" || root.Space != rules.NSRsm {
		r.Add(Finding{Rule: "CII", Severity: Error, Source: SourceSyntax, Path: "/" + root.Name, Line: root.Line,
			Message: "the root element is not rsm:CrossIndustryInvoice in namespace " + rules.NSRsm})
		return r
	}
	doc := rules.NewDoc(root, opts.Profile)
	r.Profile = doc.Profile
	set := RulesFor(doc, opts)
	run := func(s RuleSet, source string, check func(*rules.Checker)) {
		if set&s != 0 {
			check(&rules.Checker{Doc: doc, Report: r, Source: source})
		}
	}
	run(Schema, SourceSyntax, schema.Check)
	run(EN16931, SourceEN16931, en16931.Check)
	run(FacturX, SourceFacturX, facturx.Check)
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
	run(Peppol, SourcePeppol, peppol.Check)
	return r
}

// RulesFor returns the rule sets that apply to doc with opts: opts.Rules
// (Default when zero), plus the German rules for an XRECHNUNG invoice and
// the Peppol rules for a Peppol invoice whatever opts.Rules, plus the
// national rules of the seller's country with opts.Auto. The profile is the
// one of doc, which Options.Profile overrides.
func RulesFor(doc *rules.Doc, opts *Options) RuleSet {
	set := opts.Rules
	if set == 0 {
		set = Default
	}
	switch doc.Profile {
	case invoice.ProfileXRechnung:
		set |= Germany
	case invoice.ProfilePeppol:
		set |= Peppol
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
	add(EN16931, en16931.Catalog)
	add(FacturX, facturx.Catalog)
	add(France, fr.Catalog)
	add(France, fr.ProfileCatalog)
	add(Germany, de.Catalog)
	add(GermanyProfile, de.ProfileCatalog)
	add(ChorusPro, choruspro.Catalog)
	add(Peppol, peppol.Catalog)
	return out
}
