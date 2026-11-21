package nl

import (
	"strings"

	"github.com/ritoon/zerechnung/internal/rules"
)

// info describes an assertion by its message, without the identifier in
// brackets that opens it.
func info(id string, sev rules.Severity, msg string) rules.RuleInfo {
	if i := strings.Index(msg, "] "); strings.HasPrefix(msg, "[") && i > 0 {
		msg = msg[i+2:]
	}
	if rest, ok := strings.CutPrefix(msg, "/ [BR-NL-34] "); ok {
		msg = rest
	}
	return rules.RuleInfo{ID: id, Severity: sev, Description: msg}
}

// Catalog lists the 32 assertions of NLCIUS-CII-validation.sch that Check
// applies to a CII invoice, in the order of the schematron. UBLCatalog lists
// those it applies to a UBL invoice.
var Catalog = []rules.RuleInfo{
	info("BR-NL-1", rules.Error, msgCII1),
	info("BR-NL-2", rules.Error, msgCII2),
	info("BR-NL-3", rules.Error, msgCII3),
	info("BR-NL-4", rules.Error, msgCII4),
	info("BR-NL-5", rules.Error, msgCII5),
	info("BR-NL-7", rules.Error, msgCII7),
	info("BR-NL-10", rules.Error, msgCII10),
	info("BR-NL-11", rules.Error, msgCII11),
	info("BR-NL-12", rules.Error, msgCII12),
	info("BR-NL-13", rules.Error, msgCII13),
	info("BR-NL-19", rules.Warning, msgCII19),
	info("BR-NL-20", rules.Warning, msgCII20),
	info("BR-NL-21", rules.Warning, msgCII21),
	info("BR-NL-22", rules.Warning, msgCII22),
	info("BR-NL-23", rules.Warning, msgCII23),
	info("BR-NL-24", rules.Warning, msgCII24),
	info("BR-NL-25", rules.Warning, msgCII25),
	info("BR-NL-26", rules.Warning, msgCII26),
	info("BR-NL-27-1", rules.Warning, lineThree("SellerTradeParty", "seller")),
	info("BR-NL-27-2", rules.Warning, lineThree("BuyerTradeParty", "customer")),
	info("BR-NL-27-3", rules.Warning, lineThree("SellerTaxRepresentativeTradeParty", "tax representative")),
	info("BR-NL-27-4", rules.Warning, lineThree("ShipToTradeParty", "delivery")),
	info("BR-NL-28-1", rules.Warning, subdivision("SellerTradeParty")),
	info("BR-NL-28-2", rules.Warning, subdivision("BuyerTradeParty")),
	info("BR-NL-28-3", rules.Warning, subdivision("SellerTaxRepresentativeTradeParty")),
	info("BR-NL-28-4", rules.Warning, subdivision("ShipToTradeParty")),
	info("BR-NL-29", rules.Warning, msgCII29),
	info("BR-NL-30", rules.Warning, msgCII30),
	info("BR-NL-32-and-34", rules.Warning, msgCII32),
	info("BR-NL-33", rules.Warning, msgCII33),
	info("BR-NL-35", rules.Warning, msgCII35),
	info("empty-element-check", rules.Warning, "Document should not contain empty elements."),
}

// UBLCatalog lists the 43 assertions of the SI-UBL 2.0 artefacts that Check
// applies to a UBL invoice, in the order of the schematrons: the pattern
// SI-UBL-VERSION, the pattern nlcius, then the pattern g-account-extension of
// the G-account extension, whose version check is SI-V20-INV-R000 again.
// BR-NL-8, of two assertions, and SI-V20-INV-R000 are listed once.
var UBLCatalog = []rules.RuleInfo{
	info("BII2-T10-R001", rules.Error, msgInvoiceID),
	info("SI-V20-INV-R000", rules.Error, strings.Replace(msgVersion, "]-", "] ", 1)),
	info("BR-NL-1", rules.Error, msgUBL1),
	info("BR-NL-2", rules.Error, msgUBL2),
	info("BR-NL-3", rules.Error, msgUBL3),
	info("BR-NL-4", rules.Error, msgUBL4),
	info("BR-NL-5", rules.Error, msgUBL5),
	info("BR-NL-7", rules.Error, msgUBL7),
	info("BR-NL-8", rules.Error, "The document MUST use the CreditNote scheme if its type code is 381, the Invoice scheme otherwise"),
	info("BR-NL-9", rules.Error, msgUBL9),
	info("BR-NL-10", rules.Error, msgUBL10),
	info("BR-NL-11", rules.Error, msgUBL11),
	info("BR-NL-12", rules.Error, msgUBL12),
	info("BR-NL-31", rules.Warning, msgUBL31),
	info("BR-NL-13", rules.Error, msgUBL13),
	info("BR-NL-19", rules.Warning, msgUBL19),
	info("BR-NL-20", rules.Warning, msgUBL20),
	info("BR-NL-21", rules.Warning, msgUBL21),
	info("BR-NL-24", rules.Warning, msgUBL24),
	info("BR-NL-25", rules.Warning, msgUBL25),
	info("BR-NL-26", rules.Warning, msgUBL26),
	info("BR-NL-27-1", rules.Warning, msgUBL271),
	info("BR-NL-27-2", rules.Warning, msgUBL272),
	info("BR-NL-27-3", rules.Warning, msgUBL273),
	info("BR-NL-27-4", rules.Warning, msgUBL274),
	info("BR-NL-28-1", rules.Warning, msgUBL281),
	info("BR-NL-28-2", rules.Warning, msgUBL282),
	info("BR-NL-28-3", rules.Warning, msgUBL283),
	info("BR-NL-28-4", rules.Warning, msgUBL284),
	info("BR-NL-29", rules.Warning, msgUBL29),
	info("BR-NL-30", rules.Warning, msgUBL30),
	info("BR-NL-32-1", rules.Warning, msgUBL32),
	info("BR-NL-33", rules.Warning, msgUBL33),
	info("BR-NL-35", rules.Warning, msgUBL35),
	info("SI-UBL-2", rules.Warning, "Document should not contain empty elements."),
	info("BR-GA-0", rules.Error, msgGA0),
	info("BR-GA-1", rules.Error, msgGA1),
	info("BR-GA-2", rules.Error, msgGA2),
	info("BR-GA-3", rules.Error, msgGA3),
	info("BR-GA-7", rules.Error, msgGA7),
	info("BR-GA-4", rules.Error, msgGA4),
	info("BR-GA-5", rules.Error, msgGA5),
	info("BR-GA-6", rules.Error, msgGA6),
}
