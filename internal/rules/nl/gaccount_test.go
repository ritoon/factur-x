package nl

import (
	"slices"
	"testing"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// G-account invoice without finding: ublBase with two payment means and two
// payment terms.
const gaBase = "testdata/gaccount.ubl.xml"

// Anchors of the G-account invoice.
const (
	gSpec    = "<cbc:CustomizationID>" + GAccountID + "</cbc:CustomizationID>"
	gID1     = "<cbc:ID>BENEFICIARY</cbc:ID>"
	gID2     = "<cbc:ID>GACCOUNT</cbc:ID>"
	gPM1     = "<cac:PaymentMeans>\n    " + gID1 + "\n    " + uPMCode + "\n    <cbc:PaymentID>F-2026-0042</cbc:PaymentID>\n    <cac:PayeeFinancialAccount>\n      " + uAccount + "\n    </cac:PayeeFinancialAccount>\n  </cac:PaymentMeans>"
	gPM2End  = "<cbc:ID>NL20INGB0001234567</cbc:ID>\n    </cac:PayeeFinancialAccount>\n  </cac:PaymentMeans>"
	gRef1    = "<cbc:PaymentMeansID>BENEFICIARY</cbc:PaymentMeansID>"
	gRef2    = "<cbc:PaymentMeansID>GACCOUNT</cbc:PaymentMeansID>"
	gAmount1 = `<cbc:Amount currencyID="EUR">907.50</cbc:Amount>`
	gAmount2 = `<cbc:Amount currencyID="EUR">302.50</cbc:Amount>`
	gTerms2  = "<cac:PaymentTerms>\n    " + gRef2 + "\n    " + gAmount2 + "\n  </cac:PaymentTerms>"
)

func gAmount(v string) string { return `<cbc:Amount currencyID="EUR">` + v + "</cbc:Amount>" }

// gTerms returns payment terms of amount v referring to the payment means
// ref.
func gTerms(ref, v string) string {
	return "<cac:PaymentTerms>" + cbc("PaymentMeansID", ref) + gAmount(v) + "</cac:PaymentTerms>"
}

// gaccountExtension wraps a cbc:CustomizationID in the UBL extensions, which
// come first in the document.
func gaccountExtension(id string) string {
	return `<ext:UBLExtensions xmlns:ext="urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2"><ext:UBLExtension><ext:ExtensionContent>` +
		cbc("CustomizationID", id) + "</ext:ExtensionContent></ext:UBLExtension></ext:UBLExtensions>"
}

func gaccountCases() []mutationCase {
	return []mutationCase{
		{"base", nil, nil},
		{"credit note: identifier rules only", edits(creditNote, one(remove(gTerms2), remove(gID2))), nil},
		{"NLCIUS rules", one(remove(uBuyerRef)), ids("BR-NL-2")},
		{"foreign supplier", edits(uForeignSupplier, one(remove(gTerms2))), ids("BR-GA-1", "BR-GA-3")},
		{"identifier normalized", one(rep(gSpec, cbc("CustomizationID", "\n  "+GAccountID+" "))), nil},

		{"SI-V20-INV-R000 without the NLCIUS prefix", one(rep(gSpec, cbc("CustomizationID", "urn:cen.eu:en16931:2017"+rules.GAccountExtension))), ids("SI-V20-INV-R000")},
		{"BR-GA-0 identifier in the extensions", one(before(gSpec, gaccountExtension(CIUSID))), ids("BR-GA-0")},
		{"BR-GA-0 and SI-V20-INV-R000 in the extensions", one(before(gSpec, gaccountExtension("urn:example"))), ids("BR-GA-0", "SI-V20-INV-R000")},
		{"BR-GA-0 extension identifier in the extensions", one(before(gSpec, gaccountExtension(GAccountID))), nil},
		{"BR-GA-1 one payment terms", one(remove(gTerms2), rep(gAmount1, gAmount("1210.00"))), ids("BR-GA-1")},
		{"BR-GA-1 three payment terms", one(after(gTerms2, gTerms("GACCOUNT", "0.00"))), ids("BR-GA-1")},
		{"BR-GA-2 one payment means", one(remove(gPM1), rep(gRef1, gRef2)), ids("BR-GA-2")},
		{"BR-GA-2 three payment means", one(after(gPM2End, "<cac:PaymentMeans>"+cbc("ID", "OTHER")+uPMCode+"<cac:PayeeFinancialAccount>"+uAccount+"</cac:PayeeFinancialAccount></cac:PaymentMeans>")), ids("BR-GA-2")},
		{"BR-GA-3 sum", one(rep(gAmount2, gAmount("402.50"))), ids("BR-GA-3")},
		{"BR-GA-3 compared as decimals", one(rep(gAmount1, gAmount(" 907.5")), rep(gAmount2, gAmount("+302.500"))), nil},
		{"BR-GA-3 exact sum", one(rep(gAmount1, gAmount("907.499999999999999999")), rep(gAmount2, gAmount("302.500000000000000001"))), nil},
		{"BR-GA-3 amount not a decimal", one(rep(gAmount2, gAmount("3.025E2"))), ids("BR-GA-3")},
		{"BR-GA-3 payment terms without amount", one(remove(gAmount2), rep(gAmount1, gAmount("1210"))), nil},
		{"BR-GA-3 two amounts", one(after(gAmount2, gAmount("0"))), ids("BR-GA-3")},
		{"BR-GA-3 no payable amount", one(remove(uPayable)), ids("BR-GA-3")},
		{"BR-GA-3 no payment terms amount", one(remove(gAmount1), remove(gAmount2), rep(uPayable, `<cbc:PayableAmount currencyID="EUR">0</cbc:PayableAmount>`)), nil},
		{"BR-GA-4 no reference", one(remove(gRef2)), ids("BR-GA-4")},
		{"BR-GA-4 two references", one(after(gRef2, gRef2)), ids("BR-GA-4")},
		{"BR-GA-5 no identifier", one(remove(gID1), rep(gRef1, gRef2)), ids("BR-GA-5")},
		{"BR-GA-5 two identifiers", one(after(gID1, cbc("ID", "OTHER"))), ids("BR-GA-5")},
		{"BR-GA-6 unknown reference", one(rep(gRef1, cbc("PaymentMeansID", "BENEFICIARY_OTHER"))), ids("BR-GA-6")},
		{"BR-GA-6 compared untrimmed", one(rep(gRef1, cbc("PaymentMeansID", " BENEFICIARY"))), ids("BR-GA-6")},
		{"BR-GA-6 empty reference and identifier", one(rep(gID1, "<cbc:ID/>"), rep(gRef1, "<cbc:PaymentMeansID/>")), ids("BR-GA-6", "SI-UBL-2")},
		{"BR-GA-7 no G-account", one(rep(gID2, cbc("ID", "OTHER")), rep(gRef2, cbc("PaymentMeansID", "OTHER"))), ids("BR-GA-7")},
		{"BR-GA-7 two G-accounts", one(rep(gID1, gID2), rep(gRef1, gRef2)), ids("BR-GA-7")},
		{"BR-GA-7 compared untrimmed", one(rep(gID2, cbc("ID", "GACCOUNT ")), rep(gRef2, cbc("PaymentMeansID", "GACCOUNT "))), ids("BR-GA-7")},
	}
}

func TestGAccountMutations(t *testing.T) {
	doc := load(t, gaBase)
	for _, c := range gaccountCases() {
		t.Run(c.name, func(t *testing.T) {
			r := run(t, []byte(apply(t, doc, c.edits)))
			want := slices.Clone(c.want)
			slices.Sort(want)
			if got := r.Rules(); !slices.Equal(got, want) {
				t.Errorf("rules = %q, want %q\n%s", got, want, r)
			}
			checkUBLFindings(t, r)
		})
	}
}

// TestDecimalOf checks the cast of BR-GA-3 to xs:decimal, and that several
// values do not cast.
func TestDecimalOf(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string // "" when the cast fails
	}{
		{"1210.00", "1210"},
		{" +302.500\n", "605/2"},
		{"-.5", "-1/2"},
		{"5.", "5"},
		{"1e3", ""},
		{"0x10", ""},
		{"1/2", ""},
		{"1_000", ""},
		{"", ""},
		{".", ""},
		{"NaN", ""},
	} {
		v, ok := decimalOf([]*xmltree.Node{{Text: c.in}})
		got := ""
		if ok && v != nil {
			got = v.RatString()
		}
		if got != c.want || ok != (c.want != "") {
			t.Errorf("decimalOf(%q) = %s, %v; want %q", c.in, got, ok, c.want)
		}
	}
	if v, ok := decimalOf(nil); v != nil || !ok {
		t.Errorf("decimalOf(nil) = %v, %v; want nil, true", v, ok)
	}
	if _, ok := decimalOf([]*xmltree.Node{{Text: "1"}, {Text: "2"}}); ok {
		t.Error("decimalOf of two values casts")
	}
}
