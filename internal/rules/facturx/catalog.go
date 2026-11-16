package facturx

import "github.com/ritoon/factur-x/internal/rules"

// Profile masks of the rules: the profiles whose official schematron holds
// the rule. mOther stands for the profiles from BASIC up, which only get the
// profile identification rules here.
const (
	mMin uint8 = 1 << iota
	mBWL
	mOther

	mLight = mMin | mBWL
	mAny   = mMin | mBWL | mOther
)

// rule describes a rule of the package.
type rule struct {
	id   string
	sev  rules.Severity
	mask uint8
	fx   string // identifier of the assertion in the official Factur-X schematron, if any
	desc string
}

// table lists the rules. The identifiers are the business rule ones (BR-*,
// CII-SR-*) when the official assertion carries one, the identifier of the
// schematron assertion (FX-SCH-A-*) otherwise. The official schematrons set
// no severity, so every assertion is an error.
var table = []rule{
	// Profile identification, all profiles.
	{"FX-PROFILE-01", rules.Error, mAny, "", "The specification identifier (BT-24) is present, so that the Factur-X profile can be determined."},
	{"FX-PROFILE-02", rules.Error, mAny, "", "The specification identifier (BT-24) is a known Factur-X, ZUGFeRD or XRechnung identifier."},
	{"FX-PROFILE-03", rules.Warning, mAny, "", "The specification identifier (BT-24) uses an obsolete spelling of a known identifier."},
	{"FX-SCH-A-000026", rules.Error, mAny, "FX-SCH-A-000026", "The specification identifier (BT-24) is exactly an identifier of the profile the invoice is checked against."},

	// Invoice level.
	{"BR-01", rules.Error, mLight, "FX-SCH-A-000010", "An invoice has a specification identifier (BT-24)."},
	{"BR-02", rules.Error, mLight, "FX-SCH-A-000011", "An invoice has an invoice number (BT-1)."},
	{"BR-03", rules.Error, mLight, "FX-SCH-A-000012", "An invoice has an issue date (BT-2) in format 102."},
	{"BR-04", rules.Error, mLight, "FX-SCH-A-000013", "An invoice has an invoice type code (BT-3)."},
	{"BR-05", rules.Error, mLight, "FX-SCH-A-000014", "An invoice has an invoice currency code (BT-5)."},
	{"BR-06", rules.Error, mLight, "FX-SCH-A-000015", "An invoice contains the seller name (BT-27)."},
	{"BR-07", rules.Error, mLight, "FX-SCH-A-000016", "An invoice contains the buyer name (BT-44)."},
	{"BR-08", rules.Error, mLight, "FX-SCH-A-000017", "An invoice contains the seller postal address (BG-5)."},
	{"BR-09", rules.Error, mLight, "FX-SCH-A-000018", "The seller postal address contains a country code (BT-40)."},
	{"BR-10", rules.Error, mBWL, "FX-SCH-A-000156", "An invoice contains the buyer postal address (BG-8)."},
	{"BR-11", rules.Error, mBWL, "FX-SCH-A-000157", "The buyer postal address contains a country code (BT-55)."},
	{"BR-62", rules.Error, mBWL, "FX-SCH-A-000158", "The seller electronic address (BT-34) has a scheme identifier."},
	{"BR-63", rules.Error, mBWL, "FX-SCH-A-000159", "The buyer electronic address (BT-49) has a scheme identifier."},
	{"BR-CO-09", rules.Error, mLight, "FX-SCH-A-000002", "VAT identifiers (BT-31, BT-48, BT-63) start with an ISO 3166-1 alpha-2 country prefix."},
	{"BR-CO-25", rules.Error, mBWL, "FX-SCH-A-000155", "When the amount due for payment (BT-115) is positive, a payment due date (BT-9) or payment terms (BT-20) are present."},
	{"BR-CO-26", rules.Error, mLight, "FX-SCH-A-000001", "The seller identifier (BT-29), legal registration identifier (BT-30) or VAT identifier (BT-31) is present."},

	// Document totals.
	{"BR-12", rules.Error, mBWL, "FX-SCH-A-000117", "An invoice has the sum of invoice line net amounts (BT-106)."},
	{"BR-13", rules.Error, mLight, "FX-SCH-A-000003", "An invoice has the invoice total amount without VAT (BT-109)."},
	{"BR-14", rules.Error, mLight, "FX-SCH-A-000004", "An invoice has the invoice total amount with VAT (BT-112)."},
	{"BR-15", rules.Error, mLight, "FX-SCH-A-000005", "An invoice has the amount due for payment (BT-115)."},
	{"BR-CO-11", rules.Error, mBWL, "FX-SCH-A-000118", "The sum of allowances on document level (BT-107) is the sum of the document level allowance amounts (BT-92)."},
	{"BR-CO-12", rules.Error, mBWL, "FX-SCH-A-000119", "The sum of charges on document level (BT-108) is the sum of the document level charge amounts (BT-99)."},
	{"BR-CO-13", rules.Error, mBWL, "FX-SCH-A-000120", "The invoice total without VAT (BT-109) is the sum of line net amounts (BT-106) minus allowances (BT-107) plus charges (BT-108)."},
	{"BR-CO-14", rules.Error, mBWL, "FX-SCH-A-000130", "The invoice total VAT amount (BT-110) is the sum of the VAT category tax amounts (BT-117)."},
	{"BR-CO-15", rules.Error, mBWL, "FX-SCH-A-000121", "The invoice total with VAT (BT-112) is the total without VAT (BT-109) plus the total VAT amount (BT-110)."},
	{"BR-CO-16", rules.Error, mBWL, "FX-SCH-A-000122", "The amount due for payment (BT-115) is the total with VAT (BT-112) minus the paid amount (BT-113) plus the rounding amount (BT-114)."},
	{"FX-MIN-CO-15", rules.Warning, mMin, "", "MINIMUM adaptation of BR-CO-15, not in the official MINIMUM schematron: the invoice total with VAT (BT-112) is the total without VAT (BT-109) plus the total VAT amount (BT-110)."},
	{"BR-53", rules.Error, mBWL, "FX-SCH-A-000129", "When the VAT accounting currency code (BT-6) is present, the total VAT amount in accounting currency (BT-111) is provided, and BT-6 differs from the invoice currency."},
	{"BR-DEC-09", rules.Error, mBWL, "FX-SCH-A-000123", "The sum of invoice line net amounts (BT-106) has at most 2 decimals."},
	{"BR-DEC-10", rules.Error, mBWL, "FX-SCH-A-000124", "The sum of allowances on document level (BT-107) has at most 2 decimals."},
	{"BR-DEC-11", rules.Error, mBWL, "FX-SCH-A-000125", "The sum of charges on document level (BT-108) has at most 2 decimals."},
	{"BR-DEC-12", rules.Error, mLight, "FX-SCH-A-000006", "The invoice total amount without VAT (BT-109) has at most 2 decimals."},
	{"BR-DEC-13", rules.Error, mLight, "FX-SCH-A-000007", "The invoice total VAT amount (BT-110) has at most 2 decimals."},
	{"BR-DEC-14", rules.Error, mLight, "FX-SCH-A-000008", "The invoice total amount with VAT (BT-112) has at most 2 decimals."},
	{"BR-DEC-15", rules.Error, mBWL, "FX-SCH-A-000126", "The invoice total VAT amount in accounting currency (BT-111) has at most 2 decimals."},
	{"BR-DEC-16", rules.Error, mBWL, "FX-SCH-A-000127", "The paid amount (BT-113) has at most 2 decimals."},
	{"BR-DEC-17", rules.Error, mBWL, "FX-SCH-A-000128", "The rounding amount (BT-114) has at most 2 decimals."},
	{"BR-DEC-18", rules.Error, mLight, "FX-SCH-A-000009", "The amount due for payment (BT-115) has at most 2 decimals."},

	// VAT breakdown.
	{"BR-CO-18", rules.Error, mBWL, "FX-SCH-A-000135", "An invoice has at least one VAT breakdown group (BG-23)."},
	{"BR-45", rules.Error, mBWL, "FX-SCH-A-000047", "Each VAT breakdown (BG-23) has a VAT category taxable amount (BT-116)."},
	{"BR-46", rules.Error, mBWL, "FX-SCH-A-000048", "Each VAT breakdown (BG-23) has a VAT category tax amount (BT-117)."},
	{"BR-47", rules.Error, mBWL, "FX-SCH-A-000049", "Each VAT breakdown (BG-23) is defined by a VAT category code (BT-118)."},
	{"BR-48", rules.Error, mBWL, "FX-SCH-A-000050", "Each VAT breakdown (BG-23) has a VAT category rate (BT-119), except for category O."},
	{"BR-CO-03", rules.Error, mBWL, "FX-SCH-A-000051", "The VAT point date (BT-7) and the VAT point date code (BT-8) are mutually exclusive."},
	{"BR-CO-17", rules.Error, mBWL, "FX-SCH-A-000052", "The VAT category tax amount (BT-117) is the taxable amount (BT-116) times the rate (BT-119), rounded to 2 decimals, within 1."},
	{"BR-DEC-19", rules.Error, mBWL, "FX-SCH-A-000053", "The VAT category taxable amount (BT-116) has at most 2 decimals."},
	{"BR-DEC-20", rules.Error, mBWL, "FX-SCH-A-000054", "The VAT category tax amount (BT-117) has at most 2 decimals."},
	{"BR-S-09", rules.Error, mBWL, "FX-SCH-A-000057", "Standard rated: the tax amount (BT-117) is the taxable amount (BT-116) times the rate (BT-119), within 1."},
	{"BR-S-10", rules.Error, mBWL, "FX-SCH-A-000058", "Standard rated: the VAT breakdown has no exemption reason (BT-120, BT-121)."},
	{"BR-Z-09", rules.Error, mBWL, "FX-SCH-A-000055", "Zero rated: the VAT category tax amount (BT-117) is 0."},
	{"BR-Z-10", rules.Error, mBWL, "FX-SCH-A-000056", "Zero rated: the VAT breakdown has no exemption reason (BT-120, BT-121)."},
	{"BR-E-09", rules.Error, mBWL, "FX-SCH-A-000138", "Exempt from VAT: the VAT category tax amount (BT-117) is 0."},
	{"BR-E-10", rules.Error, mBWL, "FX-SCH-A-000139", "Exempt from VAT: the VAT breakdown has an exemption reason (BT-120) or code (BT-121)."},
	{"BR-AE-09", rules.Error, mBWL, "FX-SCH-A-000136", "Reverse charge: the VAT category tax amount (BT-117) is 0."},
	{"BR-AE-10", rules.Error, mBWL, "FX-SCH-A-000137", "Reverse charge: the VAT breakdown has an exemption reason (BT-120) or code (BT-121)."},
	{"BR-G-09", rules.Error, mBWL, "FX-SCH-A-000140", "Export outside the EU: the VAT category tax amount (BT-117) is 0."},
	{"BR-G-10", rules.Error, mBWL, "FX-SCH-A-000141", "Export outside the EU: the VAT breakdown has an exemption reason (BT-120) or code (BT-121)."},
	{"BR-IC-08", rules.Error, mMin, "FX-SCH-A-000230", "Intra-community supply: the taxable amount (BT-116) is the sum of the line net amounts plus charges minus allowances of category K, within 1."},
	{"BR-IC-09", rules.Error, mLight, "FX-SCH-A-000142", "Intra-community supply: the VAT category tax amount (BT-117) is 0."},
	{"BR-IC-10", rules.Error, mLight, "FX-SCH-A-000143", "Intra-community supply: the VAT breakdown has an exemption reason (BT-120) or code (BT-121)."},
	{"BR-IC-11", rules.Error, mLight, "FX-SCH-A-000144", "Intra-community supply: the actual delivery date (BT-72) or the invoicing period (BG-14) is present."},
	{"BR-IC-12", rules.Error, mLight, "FX-SCH-A-000145", "Intra-community supply: the deliver to country code (BT-80) is present."},
	{"BR-AF-10", rules.Error, mBWL, "FX-SCH-A-000147", "IGIC (Canary Islands): the VAT breakdown has no exemption reason (BT-120, BT-121)."},
	{"BR-AG-10", rules.Error, mBWL, "FX-SCH-A-000149", "IPSI (Ceuta and Melilla): the VAT breakdown has no exemption reason (BT-120, BT-121)."},
	{"BR-O-08", rules.Error, mMin, "FX-SCH-A-000233", "Not subject to VAT: the taxable amount (BT-116) is the sum of the line net amounts plus charges minus allowances of category O."},
	{"BR-O-09", rules.Error, mLight, "FX-SCH-A-000150", "Not subject to VAT: the VAT category tax amount (BT-117) is 0."},
	{"BR-O-10", rules.Error, mLight, "FX-SCH-A-000151", "Not subject to VAT: the VAT breakdown has an exemption reason (BT-120) or code (BT-121)."},
	{"BR-O-11", rules.Error, mLight, "FX-SCH-A-000152", "Not subject to VAT: the invoice has no other VAT breakdown category."},
	{"BR-O-12", rules.Error, mMin, "FX-SCH-A-000234", "Not subject to VAT: no invoice line has another VAT category."},
	{"BR-O-13", rules.Error, mLight, "FX-SCH-A-000153", "Not subject to VAT: no document level allowance has another VAT category."},
	{"BR-O-14", rules.Error, mLight, "FX-SCH-A-000154", "Not subject to VAT: no document level charge has another VAT category."},

	// Invoicing period.
	{"BR-29", rules.Error, mBWL, "FX-SCH-A-000059", "The invoicing period end date (BT-74) is not before its start date (BT-73)."},
	{"BR-CO-19", rules.Error, mBWL, "FX-SCH-A-000060", "An invoicing period (BG-14) has a start date (BT-73) or an end date (BT-74)."},

	// Document level allowances and charges.
	{"CII-SR-463", rules.Error, mLight, "FX-SCH-A-000348; FX-SCH-A-000346 in MINIMUM", "A document level allowance or charge has a charge indicator."},
	{"BR-31", rules.Error, mBWL, "FX-SCH-A-000061", "A document level allowance (BG-20) has an amount (BT-92)."},
	{"BR-32", rules.Error, mBWL, "FX-SCH-A-000062", "A document level allowance (BG-20) has a VAT category code (BT-95)."},
	{"BR-33", rules.Error, mBWL, "FX-SCH-A-000063", "A document level allowance (BG-20) has a reason (BT-97) or a reason code (BT-98)."},
	{"BR-CO-21", rules.Error, mBWL, "FX-SCH-A-000065", "A document level allowance (BG-20) has a reason (BT-97) or a reason code (BT-98)."},
	{"BR-DEC-01", rules.Error, mBWL, "FX-SCH-A-000066", "The document level allowance amount (BT-92) has at most 2 decimals."},
	{"BR-DEC-02", rules.Error, mBWL, "FX-SCH-A-000067", "The document level allowance base amount (BT-93) has at most 2 decimals."},
	{"BR-36", rules.Error, mBWL, "FX-SCH-A-000068", "A document level charge (BG-21) has an amount (BT-99)."},
	{"BR-37", rules.Error, mBWL, "FX-SCH-A-000069", "A document level charge (BG-21) has a VAT category code (BT-102)."},
	{"BR-38", rules.Error, mBWL, "FX-SCH-A-000070", "A document level charge (BG-21) has a reason (BT-104) or a reason code (BT-105)."},
	{"BR-CO-22", rules.Error, mBWL, "FX-SCH-A-000072", "A document level charge (BG-21) has a reason (BT-104) or a reason code (BT-105)."},
	{"BR-DEC-05", rules.Error, mBWL, "FX-SCH-A-000073", "The document level charge amount (BT-99) has at most 2 decimals."},
	{"BR-DEC-06", rules.Error, mBWL, "FX-SCH-A-000074", "The document level charge base amount (BT-100) has at most 2 decimals."},

	// VAT category of document level allowances (…-03, …-06) and charges (…-04, …-07).
	{"BR-S-03", rules.Error, mBWL, "FX-SCH-A-000095", "Standard rated allowance: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-S-04", rules.Error, mBWL, "FX-SCH-A-000113", "Standard rated charge: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-S-06", rules.Error, mBWL, "FX-SCH-A-000096", "Standard rated allowance: the VAT rate (BT-96) is greater than 0."},
	{"BR-S-07", rules.Error, mBWL, "FX-SCH-A-000114", "Standard rated charge: the VAT rate (BT-103) is greater than 0."},
	{"BR-Z-03", rules.Error, mBWL, "FX-SCH-A-000097", "Zero rated allowance: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-Z-04", rules.Error, mBWL, "FX-SCH-A-000115", "Zero rated charge: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-Z-06", rules.Error, mBWL, "FX-SCH-A-000098", "Zero rated allowance: the VAT rate (BT-96) is 0."},
	{"BR-Z-07", rules.Error, mBWL, "FX-SCH-A-000116", "Zero rated charge: the VAT rate (BT-103) is 0."},
	{"BR-E-03", rules.Error, mBWL, "FX-SCH-A-000083", "Exempt allowance: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-E-04", rules.Error, mBWL, "FX-SCH-A-000101", "Exempt charge: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-E-06", rules.Error, mBWL, "FX-SCH-A-000084", "Exempt allowance: the VAT rate (BT-96) is 0."},
	{"BR-E-07", rules.Error, mBWL, "FX-SCH-A-000102", "Exempt charge: the VAT rate (BT-103) is 0."},
	{"BR-AE-03", rules.Error, mBWL, "FX-SCH-A-000081", "Reverse charge allowance: a seller VAT or tax identifier and the buyer VAT identifier or legal registration identifier are present."},
	{"BR-AE-04", rules.Error, mBWL, "FX-SCH-A-000099", "Reverse charge charge: a seller VAT or tax identifier and the buyer VAT identifier or legal registration identifier are present."},
	{"BR-AE-06", rules.Error, mBWL, "FX-SCH-A-000082", "Reverse charge allowance: the VAT rate (BT-96) is 0."},
	{"BR-AE-07", rules.Error, mBWL, "FX-SCH-A-000100", "Reverse charge charge: the VAT rate (BT-103) is 0."},
	{"BR-G-03", rules.Error, mBWL, "FX-SCH-A-000085", "Export allowance: the seller VAT identifier (BT-31) or tax representative VAT identifier (BT-63) is present."},
	{"BR-G-04", rules.Error, mBWL, "FX-SCH-A-000103", "Export charge: the seller VAT identifier (BT-31) or tax representative VAT identifier (BT-63) is present."},
	{"BR-G-06", rules.Error, mBWL, "FX-SCH-A-000086", "Export allowance: the VAT rate (BT-96) is 0."},
	{"BR-G-07", rules.Error, mBWL, "FX-SCH-A-000104", "Export charge: the VAT rate (BT-103) is 0."},
	{"BR-IC-03", rules.Error, mLight, "FX-SCH-A-000087", "Intra-community allowance: the seller (or tax representative) and the buyer VAT identifiers are present."},
	{"BR-IC-04", rules.Error, mLight, "FX-SCH-A-000105", "Intra-community charge: the seller (or tax representative) and the buyer VAT identifiers are present."},
	{"BR-IC-06", rules.Error, mLight, "FX-SCH-A-000088", "Intra-community allowance: the VAT rate (BT-96) is 0."},
	{"BR-IC-07", rules.Error, mLight, "FX-SCH-A-000106", "Intra-community charge: the VAT rate (BT-103) is 0."},
	{"BR-AF-03", rules.Error, mBWL, "FX-SCH-A-000089", "IGIC allowance: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-AF-04", rules.Error, mBWL, "FX-SCH-A-000107", "IGIC charge: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-AF-06", rules.Error, mBWL, "FX-SCH-A-000090", "IGIC allowance: the VAT rate (BT-96) is greater than 0."},
	{"BR-AF-07", rules.Error, mBWL, "FX-SCH-A-000108", "IGIC charge: the VAT rate (BT-103) is greater than 0."},
	{"BR-AG-03", rules.Error, mBWL, "FX-SCH-A-000091", "IPSI allowance: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-AG-04", rules.Error, mBWL, "FX-SCH-A-000109", "IPSI charge: the seller VAT identifier (BT-31), tax registration (BT-32) or tax representative VAT identifier (BT-63) is present."},
	{"BR-AG-06", rules.Error, mBWL, "FX-SCH-A-000092", "IPSI allowance: the VAT rate (BT-96) is 0 or greater."},
	{"BR-AG-07", rules.Error, mBWL, "FX-SCH-A-000110", "IPSI charge: the VAT rate (BT-103) is 0 or greater."},
	{"BR-O-03", rules.Error, mLight, "FX-SCH-A-000093", "Not subject to VAT allowance: no seller, tax representative or buyer VAT identifier is present."},
	{"BR-O-04", rules.Error, mLight, "FX-SCH-A-000111", "Not subject to VAT charge: no seller, tax representative or buyer VAT identifier is present."},
	{"BR-O-06", rules.Error, mLight, "FX-SCH-A-000094", "Not subject to VAT allowance: no VAT rate (BT-96) is given."},
	{"BR-O-07", rules.Error, mBWL, "FX-SCH-A-000112", "Not subject to VAT charge: no VAT rate (BT-103) is given."},

	// Invoice lines (contexts that only exist in the MINIMUM schematron).
	{"BR-IC-02", rules.Error, mMin, "FX-SCH-A-000241", "Intra-community line: the seller (or tax representative) and the buyer VAT identifiers are present."},
	{"BR-IC-05", rules.Error, mMin, "FX-SCH-A-000242", "Intra-community line: the VAT rate (BT-152) is 0."},

	// Parties.
	{"BR-17", rules.Error, mBWL, "FX-SCH-A-000075", "The payee (BG-10) has a name (BT-59)."},
	{"BR-18", rules.Error, mBWL, "FX-SCH-A-000076", "The seller tax representative (BG-11) has a name (BT-62)."},
	{"BR-19", rules.Error, mBWL, "FX-SCH-A-000077", "The seller tax representative (BG-11) has a postal address (BG-12)."},
	{"BR-20", rules.Error, mBWL, "FX-SCH-A-000078", "The seller tax representative postal address has a country code (BT-69)."},
	{"BR-56", rules.Error, mBWL, "FX-SCH-A-000079", "The seller tax representative (BG-11) has a VAT identifier (BT-63)."},
	{"BR-57", rules.Error, mBWL, "FX-SCH-A-000170", "The deliver to address (BG-15) has a country code (BT-80)."},
	{"CII-SR-465", rules.Error, mLight, "FX-SCH-A-000350; FX-SCH-A-000346 in MINIMUM", "The seller contact has a person name or a department name, not both."},
	{"CII-SR-466", rules.Error, mLight, "FX-SCH-A-000351; FX-SCH-A-000346 in MINIMUM", "The buyer contact has a person name or a department name, not both."},

	// Payment.
	{"BR-49", rules.Error, mBWL, "FX-SCH-A-000131", "A payment instruction (BG-16) has a payment means type code (BT-81)."},
	{"BR-50", rules.Error, mBWL, "FX-SCH-A-000133", "A credit transfer has a payment account identifier (BT-84)."},
	{"BR-61", rules.Error, mBWL, "FX-SCH-A-000134", "A credit transfer (codes 30, 58) has a payment account identifier (BT-84)."},
	{"BR-CO-27", rules.Error, mBWL, "FX-SCH-A-000132", "A credit transfer (codes 30, 58) has either an IBAN or a proprietary account identifier (BT-84), not both."},
	{"BR-55", rules.Error, mBWL, "FX-SCH-A-000182", "A preceding invoice reference (BG-3) has a preceding invoice number (BT-25)."},

	// Structural constraints of the schematron that the schema does not express.
	{"FX-SCH-A-000019", rules.Error, mMin, "FX-SCH-A-000019", "The buyer legal organization has exactly one identifier (BT-47)."},
	{"FX-SCH-A-000022", rules.Error, mLight, "FX-SCH-A-000022", "A date format code is 102."},
	{"FX-SCH-A-000023", rules.Error, mLight, "FX-SCH-A-000023", "The invoice type code (BT-3) and the VAT type codes are in the code lists of the profile."},
	{"FX-SCH-A-000030", rules.Error, mBWL, "FX-SCH-A-000030", "The seller, buyer, seller tax representative and payee each have exactly one name."},
	{"FX-SCH-A-000032", rules.Error, mLight, "FX-SCH-A-000032", "The seller (and, in BASIC WL, the buyer and the seller tax representative) has exactly one postal address."},
	{"FX-SCH-A-000033", rules.Error, mLight, "FX-SCH-A-000033", "The seller has at most one VAT identifier (scheme VA)."},
	{"FX-SCH-A-000034", rules.Error, mLight, "FX-SCH-A-000034", "The seller has at most one tax registration identifier (scheme FC)."},
	{"FX-SCH-A-000037", rules.Error, mBWL, "FX-SCH-A-000037", "Global identifiers, tax registration identifiers and electronic addresses have a scheme identifier."},
	{"FX-SCH-A-000042", rules.Error, mLight, "FX-SCH-A-000042", "At most one invoice total VAT amount is given in the invoice currency."},
	{"FX-SCH-A-000163", rules.Error, mBWL, "FX-SCH-A-000163", "The buyer, deliver to party and payee have at most one identifier."},
	{"FX-SCH-A-000164", rules.Error, mBWL, "FX-SCH-A-000164", "The buyer, deliver to party and payee have at most one global identifier."},
	{"FX-SCH-A-000166", rules.Error, mBWL, "FX-SCH-A-000166", "The buyer has at most one tax registration."},
	{"FX-SCH-A-000169", rules.Error, mBWL, "FX-SCH-A-000169", "The seller tax representative has exactly one tax registration."},
	{"FX-SCH-A-000176", rules.Error, mBWL, "FX-SCH-A-000176", "Each VAT breakdown has exactly one tax amount."},
	{"FX-SCH-A-000177", rules.Error, mBWL, "FX-SCH-A-000177", "Each VAT breakdown has exactly one taxable amount."},
	{"FX-SCH-A-000179", rules.Error, mBWL, "FX-SCH-A-000179", "VAT category codes are in the code list (AE, E, G, K, L, M, O, S, Z)."},
	{"FX-SCH-A-000180", rules.Error, mBWL, "FX-SCH-A-000180", "The VAT point date code (BT-8) is 5, 29 or 72."},
	{"FX-SCH-A-000186", rules.Error, mBWL, "FX-SCH-A-000186", "The document level allowance reason code (BT-98) is in UNTDID 5189 as restricted by EN 16931."},
	{"FX-SCH-A-000192", rules.Error, mLight, "FX-SCH-A-000192", "At most one invoice total VAT amount is given in the VAT accounting currency."},

	// Additions of this package.
	{"FX-DATE", rules.Error, mLight, "", "A date in format 102 is a valid calendar day written CCYYMMDD (the official schematron only checks the format code)."},
	{"FX-NOT-USED", rules.Warning, mLight, "", "The element or attribute is marked as not used in the profile by the official schematron: receivers ignore it."},
}

// Catalog lists the rules implemented by this package.
var Catalog []rules.RuleInfo

// byID indexes table.
var byID map[string]*rule

func init() {
	byID = make(map[string]*rule, len(table))
	Catalog = make([]rules.RuleInfo, 0, len(table))
	for i := range table {
		r := &table[i]
		if byID[r.id] != nil {
			panic("facturx: rule " + r.id + " listed twice")
		}
		byID[r.id] = r
		desc := r.desc
		if r.fx != "" && r.fx != r.id {
			desc += " (" + r.fx + ")"
		}
		Catalog = append(Catalog, rules.RuleInfo{ID: r.id, Severity: r.sev, Description: desc})
	}
}
