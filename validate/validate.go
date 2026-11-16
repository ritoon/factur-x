// Package validate checks CII invoices against the XML schema of their
// Factur-X profile, the EN 16931 business rules, the Factur-X profile rules,
// and optionally the French (BR-FR) and German (BR-DE) rules.
//
// Validation never stops at the first problem: every finding is collected in
// a Report. A document that is not well-formed XML, or not a CII invoice,
// gives a report with a single syntax error.
package validate

import (
	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/rules/de"
	"github.com/ritoon/factur-x/internal/rules/en16931"
	"github.com/ritoon/factur-x/internal/rules/facturx"
	"github.com/ritoon/factur-x/internal/rules/fr"
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
)

// RuleSet selects groups of rules.
type RuleSet uint

// Rule sets.
const (
	Schema  RuleSet = 1 << iota // structure against the XML schema of the profile
	EN16931                     // CEN business rules and code lists
	FacturX                     // Factur-X / ZUGFeRD profile rules
	France                      // French rules BR-FR (2026 reform)
	Germany                     // German rules BR-DE (XRechnung)

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
	// Auto adds the French rules when the seller is in France (BT-40 = FR).
	// The German rules need no option: they apply to XRECHNUNG invoices,
	// which declare themselves in BT-24.
	Auto bool
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
	run(France, SourceFR, fr.Check)
	run(Germany, SourceDE, de.Check)
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
		if doc.Agreement.Value("SellerTradeParty/PostalTradeAddress/CountryID") == "FR" {
			set |= France
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
	add(Germany, de.Catalog)
	return out
}
