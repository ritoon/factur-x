package fr

import (
	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/invoice"
)

// Identifiers of the profile checks of the French reform. The FNFE-MPE
// schematron does not check the profile, so these identifiers are specific
// to this package.
const (
	RuleProfileMinimum = "FR-PROFILE-01" // MINIMUM is excluded from the reform
	RuleProfileBasic   = "FR-PROFILE-02" // BASIC is not listed by the reform
	RuleProfileBasicWL = "FR-PROFILE-03" // BASIC WL is no longer admitted (on option)
)

// ProfileCatalog lists the profile checks of CheckProfile.
var ProfileCatalog = []rules.RuleInfo{
	{ID: RuleProfileMinimum, Severity: rules.Error, Description: "Le profil MINIMUM n'est pas admis par la réforme française : utiliser EN 16931 ou EXTENDED-CTC-FR (socle), ou EXTENDED."},
	{ID: RuleProfileBasic, Severity: rules.Warning, Description: "Le profil BASIC n'est pas listé par la réforme française (absent du paquet FNFE-MPE et des capacités de l'annuaire) : préférer EN 16931 ou EXTENDED-CTC-FR."},
	{ID: RuleProfileBasicWL, Severity: rules.Error, Description: "Le profil BASIC WL n'est plus admis par la réforme française (signalé seulement sur option, tant que le profil reste admis)."},
}

// CheckProfile reports the profiles that the French e-invoicing reform does
// not admit. The base profiles are EN 16931 and EXTENDED-CTC-FR; EXTENDED is
// admitted, and BASIC WL is admitted for now: with rejectBasicWL, BASIC WL is
// reported as an error, for callers that no longer accept it. MINIMUM is
// excluded (error) and BASIC is not listed (warning). XRECHNUNG and an
// unknown profile are left to the other rule sets.
//
// The profile is c.Doc.Profile, which callers may have forced; the finding
// is located at BT-24.
func CheckProfile(c *rules.Checker, rejectBasicWL bool) {
	if c == nil || c.Doc == nil || c.Doc.Root == nil {
		return
	}
	at := c.Doc.Context.Find("GuidelineSpecifiedDocumentContextParameter/ID")
	if at == nil {
		at = c.Doc.Root
	}
	switch c.Doc.Profile {
	case invoice.ProfileMinimum:
		c.Fail(RuleProfileMinimum, rules.Error, at, "%s : le profil MINIMUM n'est pas admis par la réforme française de la facturation électronique, il ne porte pas les mentions obligatoires d'une facture ; utiliser EN 16931 ou EXTENDED-CTC-FR (socle), ou EXTENDED.", RuleProfileMinimum)
	case invoice.ProfileBasic:
		c.Fail(RuleProfileBasic, rules.Warning, at, "%s : le profil BASIC n'est pas listé par la réforme française (absent du paquet FNFE-MPE et des capacités de l'annuaire), une plateforme agréée peut le refuser ; préférer EN 16931 ou EXTENDED-CTC-FR.", RuleProfileBasic)
	case invoice.ProfileBasicWL:
		if rejectBasicWL {
			c.Fail(RuleProfileBasicWL, rules.Error, at, "%s : le profil BASIC WL n'est plus admis par la réforme française ; utiliser EN 16931 ou EXTENDED-CTC-FR (socle), ou EXTENDED.", RuleProfileBasicWL)
		}
	}
}
