package invoice

import (
	"fmt"
	"strings"
)

// Profile is a Factur-X / ZUGFeRD profile, identified in the XML by the
// specification identifier BT-24.
type Profile int

// Profiles, from the smallest to the largest data set. XRECHNUNG and PEPPOL
// (Peppol BIS Billing 3.0) are CIUS of EN 16931 and sit at its level;
// EXTENDED-CTC-FR is the French subset of EXTENDED.
const (
	ProfileUnknown Profile = iota
	ProfileMinimum
	ProfileBasicWL
	ProfileBasic
	ProfileEN16931
	ProfileXRechnung
	ProfilePeppol
	ProfileExtendedCTCFR
	ProfileExtended
)

// Profiles lists the known profiles.
var Profiles = []Profile{
	ProfileMinimum, ProfileBasicWL, ProfileBasic, ProfileEN16931,
	ProfileXRechnung, ProfilePeppol, ProfileExtendedCTCFR, ProfileExtended,
}

// Specification identifiers (BT-24) written for each profile.
const (
	URNMinimum        = "urn:factur-x.eu:1p0:minimum"
	URNBasicWL        = "urn:factur-x.eu:1p0:basicwl"
	URNBasic          = "urn:cen.eu:en16931:2017#compliant#urn:factur-x.eu:1p0:basic"
	URNEN16931        = "urn:cen.eu:en16931:2017"
	URNExtended       = "urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended"
	URNXRechnung      = "urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_3.0"
	URNExtendedCTCFR  = "urn:cen.eu:en16931:2017#conformant#urn.cpro.gouv.fr:1p0:extended-ctc-fr"
	URNPeppol         = "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0"
	urnXRechnungOld   = "urn:cen.eu:en16931:2017#compliant#urn:xoev-de:kosit:standard:xrechnung_"
	urnXRechnungNew   = "urn:cen.eu:en16931:2017#compliant#urn:xeinkauf.de:kosit:xrechnung_"
	urnExtendedCTCFR2 = "urn:cen.eu:en16931:2017#conformant#urn:cpro.gouv.fr:1p0:extended-ctc-fr"
)

// String returns the profile name as written in the XMP ConformanceLevel
// ("BASIC WL", "EN 16931"), or "PEPPOL", "EXTENDED-CTC-FR" and "UNKNOWN".
func (p Profile) String() string {
	switch p {
	case ProfileMinimum:
		return "MINIMUM"
	case ProfileBasicWL:
		return "BASIC WL"
	case ProfileBasic:
		return "BASIC"
	case ProfileEN16931:
		return "EN 16931"
	case ProfileXRechnung:
		return "XRECHNUNG"
	case ProfilePeppol:
		return "PEPPOL"
	case ProfileExtendedCTCFR:
		return "EXTENDED-CTC-FR"
	case ProfileExtended:
		return "EXTENDED"
	}
	return "UNKNOWN"
}

// URN returns the specification identifier (BT-24) of the profile.
func (p Profile) URN() string {
	switch p {
	case ProfileMinimum:
		return URNMinimum
	case ProfileBasicWL:
		return URNBasicWL
	case ProfileBasic:
		return URNBasic
	case ProfileEN16931:
		return URNEN16931
	case ProfileXRechnung:
		return URNXRechnung
	case ProfilePeppol:
		return URNPeppol
	case ProfileExtendedCTCFR:
		return URNExtendedCTCFR
	case ProfileExtended:
		return URNExtended
	}
	return ""
}

// ConformanceLevel returns the value of fx:ConformanceLevel in the XMP
// metadata of a Factur-X PDF. The Factur-X specification defines no value for
// EXTENDED-CTC-FR nor for Peppol BIS Billing 3.0: such invoices are declared
// EXTENDED and EN 16931.
func (p Profile) ConformanceLevel() string {
	switch p {
	case ProfileExtendedCTCFR:
		return "EXTENDED"
	case ProfilePeppol:
		return "EN 16931"
	case ProfileUnknown:
		return ""
	}
	return p.String()
}

// Filename returns the name of the XML attachment in a Factur-X PDF:
// "xrechnung.xml" for XRECHNUNG, "factur-x.xml" otherwise.
func (p Profile) Filename() string {
	if p == ProfileXRechnung {
		return "xrechnung.xml"
	}
	return "factur-x.xml"
}

// AFRelationship returns the default /AFRelationship of the XML attachment:
// "Data" for MINIMUM and BASIC WL, whose XML is not a full invoice,
// "Alternative" otherwise. Both values suit France and Germany. For BASIC
// and above, France also admits "Source" and "Data", while Germany admits
// only "Alternative" (Factur-X 1.09, 6.2.2).
func (p Profile) AFRelationship() string {
	if p == ProfileMinimum || p == ProfileBasicWL {
		return "Data"
	}
	return "Alternative"
}

// HasLines reports whether the profile carries invoice lines (BG-25).
func (p Profile) HasLines() bool { return p >= ProfileBasic }

// EN16931 reports whether invoices of this profile are complete EN 16931
// invoices (BASIC and above). MINIMUM and BASIC WL are not.
func (p Profile) EN16931() bool { return p >= ProfileBasic }

// level orders profiles by data set: XRECHNUNG and PEPPOL are at the
// EN 16931 level, EXTENDED-CTC-FR below EXTENDED.
func (p Profile) level() int {
	switch p {
	case ProfileXRechnung, ProfilePeppol:
		return int(ProfileEN16931)
	default:
		return int(p)
	}
}

// AtLeast reports whether p carries at least the data set of q.
func (p Profile) AtLeast(q Profile) bool { return p.level() >= q.level() }

// MarshalText implements encoding.TextMarshaler: the profile name, as String.
func (p Profile) MarshalText() ([]byte, error) { return []byte(p.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler. It accepts what
// ParseProfile accepts; "UNKNOWN" and empty text give ProfileUnknown.
func (p *Profile) UnmarshalText(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || strings.EqualFold(s, "UNKNOWN") {
		*p = ProfileUnknown
		return nil
	}
	q := ParseProfile(s)
	if q == ProfileUnknown {
		return fmt.Errorf("invoice: unknown profile %q", s)
	}
	*p = q
	return nil
}

// ProfileFromURN returns the profile identified by a specification identifier
// (BT-24). It recognizes the Factur-X identifiers, the ZUGFeRD 2.x ones
// (urn:zugferd.de:2p0:…), all XRechnung versions, both spellings of the
// EXTENDED-CTC-FR identifier and the Peppol BIS Billing 3.0 identifier, alone
// or followed by more text as rule PEPPOL-EN16931-R004 admits. Surrounding
// white space is ignored.
func ProfileFromURN(urn string) Profile {
	u := strings.TrimSpace(urn)
	switch u {
	case URNMinimum, "urn:zugferd.de:2p0:minimum":
		return ProfileMinimum
	case URNBasicWL, "urn:zugferd.de:2p0:basicwl":
		return ProfileBasicWL
	case URNBasic, "urn:cen.eu:en16931:2017#compliant#urn:zugferd.de:2p0:basic":
		return ProfileBasic
	case URNEN16931:
		return ProfileEN16931
	case URNExtended, "urn:cen.eu:en16931:2017#conformant#urn:zugferd.de:2p0:extended":
		return ProfileExtended
	case URNExtendedCTCFR, urnExtendedCTCFR2:
		return ProfileExtendedCTCFR
	}
	if strings.HasPrefix(u, urnXRechnungNew) || strings.HasPrefix(u, urnXRechnungOld) {
		// "…xrechnung_3.0" and the extension variant "…xrechnung_3.0#conformant#…".
		return ProfileXRechnung
	}
	if strings.HasPrefix(u, URNPeppol) {
		return ProfilePeppol
	}
	return ProfileUnknown
}

// DetectProfile is a lenient ProfileFromURN: when the identifier is not one
// of the official ones, it looks for a profile name in it, which recognizes
// spellings found in older or non-conforming invoices such as
// "urn:cen.eu:en16931:2017:compliant:factur-x.eu:1p0:basic". Use it to read
// invoices; validators report such identifiers with ProfileFromURN.
func DetectProfile(urn string) Profile {
	if p := ProfileFromURN(urn); p != ProfileUnknown {
		return p
	}
	u := strings.ToLower(strings.TrimSpace(urn))
	switch {
	case u == "":
		return ProfileUnknown
	case strings.Contains(u, "extended-ctc-fr"):
		return ProfileExtendedCTCFR
	case strings.Contains(u, "extended"):
		return ProfileExtended
	case strings.Contains(u, "xrechnung"):
		return ProfileXRechnung
	case strings.Contains(u, "basicwl"), strings.Contains(u, "basic-wl"):
		return ProfileBasicWL
	case strings.Contains(u, "basic"):
		return ProfileBasic
	case strings.Contains(u, "minimum"):
		return ProfileMinimum
	case strings.HasPrefix(u, "urn:cen.eu:en16931"):
		return ProfileEN16931
	}
	return ProfileUnknown
}

// ProfileFromConformanceLevel returns the profile named by an XMP
// ConformanceLevel value. Case, spaces, dashes and underscores are ignored, so
// "BASIC WL", "basicwl" and "EN16931" are recognized, as is the legacy
// ZUGFeRD 1 value "COMFORT" (EN 16931).
func ProfileFromConformanceLevel(level string) Profile {
	r := strings.NewReplacer(" ", "", "-", "", "_", "")
	switch strings.ToUpper(r.Replace(strings.TrimSpace(level))) {
	case "MINIMUM":
		return ProfileMinimum
	case "BASICWL":
		return ProfileBasicWL
	case "BASIC":
		return ProfileBasic
	case "EN16931", "COMFORT":
		return ProfileEN16931
	case "XRECHNUNG":
		return ProfileXRechnung
	case "EXTENDED":
		return ProfileExtended
	case "EXTENDEDCTCFR":
		return ProfileExtendedCTCFR
	}
	return ProfileUnknown
}

// ParseProfile returns the profile named by s, which may be a profile name
// ("en16931", "basic-wl", "extended-ctc-fr", "peppol") or a specification
// identifier.
func ParseProfile(s string) Profile {
	if p := ProfileFromURN(s); p != ProfileUnknown {
		return p
	}
	if strings.EqualFold(strings.TrimSpace(s), "PEPPOL") {
		// Not an XMP ConformanceLevel: Peppol invoices are declared EN 16931.
		return ProfilePeppol
	}
	return ProfileFromConformanceLevel(s)
}
