package de

import (
	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/invoice"
)

// RuleProfile identifies the profile check of CheckProfile. The KoSIT
// schematron does not check the profile, so the identifier is specific to
// this package.
const RuleProfile = "DE-PROFILE-01"

// ProfileCatalog lists the profile check of CheckProfile.
var ProfileCatalog = []rules.RuleInfo{
	{ID: RuleProfile, Severity: rules.Error, Description: "The profiles MINIMUM and BASIC WL are not e-invoices in the sense of the BMF, which accepts ZUGFeRD from version 2.0.1 except these two profiles: use BASIC, EN 16931, EXTENDED or XRECHNUNG."},
}

// CheckProfile reports the profiles that the German Federal Ministry of
// Finance (BMF) does not accept as e-invoices: its letters on e-invoicing
// list ZUGFeRD from version 2.0.1 as a structured format, except the profiles
// MINIMUM and BASIC WL. Their XML lacks mandatory invoice data; such a file
// is at most an "other invoice" (sonstige Rechnung), which no longer
// satisfies the e-invoicing obligation of domestic B2B supplies.
//
// The check applies to the invoices of sellers established in Germany; the
// caller selects them. The profile is c.Doc.Profile, which callers may have
// forced; the finding is located at BT-24.
func CheckProfile(c *rules.Checker) {
	if c == nil || c.Doc == nil || c.Doc.Root == nil {
		return
	}
	p := c.Doc.Profile
	if p != invoice.ProfileMinimum && p != invoice.ProfileBasicWL {
		return
	}
	at := c.Doc.Context.Find("GuidelineSpecifiedDocumentContextParameter/ID")
	if at == nil {
		at = c.Doc.Root
	}
	c.Fail(RuleProfile, rules.Error, at, "[%s] The profile %s is not an e-invoice in the sense of the BMF (ZUGFeRD from version 2.0.1 except the profiles MINIMUM and BASIC WL): its XML lacks mandatory invoice data and the file is at most an other invoice (sonstige Rechnung). Use BASIC, EN 16931, EXTENDED or XRECHNUNG.", RuleProfile, p)
}
