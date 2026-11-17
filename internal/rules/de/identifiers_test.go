package de

import (
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

func TestValidLeitwegID(t *testing.T) {
	for _, s := range []string{
		"04011000-1234512345-06", // example of the specification, chapter 2.4
		"04011000-12345-03",
		"991-33333TEST-33",
		"991-33333test-33", // the fine addressing is not case sensitive
		// Entries of the Leitweg-ID directory (verzeichnis.leitweg-id.de).
		"16066069-0001-38",
		"13-L75810002000-60",
		"057660004004-31001-55",
		"991-03730-19",
		"992-90009-96",
		"08315033-ESCHBACH6626-66",
		"09780119-RATHAUSDIETMANNSRIED7247-90",
		// Coarse addressing only, of every length.
		"04-" + leitwegCheckDigits("04"),
		"991-" + leitwegCheckDigits("991"),
		"04011-" + leitwegCheckDigits("04011"),
		"040110001-" + leitwegCheckDigits("040110001"),
		"040110001234-" + leitwegCheckDigits("040110001234"),
		"04-" + strings.Repeat("Z", 30) + "-" + leitwegCheckDigits("04"+strings.Repeat("Z", 30)),
	} {
		if !ValidLeitwegID(s) {
			t.Errorf("ValidLeitwegID(%q) = false (%s, check digits %s)", s, leitwegFormat(s), leitwegCheckDigits(strings.ReplaceAll(s[:len(s)-3], "-", "")))
		}
	}

	for _, c := range []struct {
		s, why string // why: start of the reason of leitwegFormat, "" for wrong check digits
	}{
		{"04011000-12345-34", ""},
		{"992-90009-97", ""},
		{"99000000-05676-98", ""},
		{"", "it must have two or three parts"},
		{"Leitweg-ID", "the coarse addressing"},
		{"0401100012345", "it must have two or three parts"},
		{"04011000-12-345-03", "it must have two or three parts"},
		{"0-03", "the coarse addressing"},
		{"0401-" + leitwegCheckDigits("0401"), "the coarse addressing"},
		{"0401100-" + leitwegCheckDigits("0401100"), "the coarse addressing"},
		{"0401100012-" + leitwegCheckDigits("0401100012"), "the coarse addressing"},
		{"1234567890-54321-40", "the coarse addressing"}, // KoSIT test suite
		{"0401100012345-" + leitwegCheckDigits("0401100012345"), "the coarse addressing"},
		{"04O11000-12345-03", "the coarse addressing"},
		{"90000000-03083-72", "the coarse addressing (Grobadressierung) must start"}, // KoSIT test suite: valid check digits
		{"00-" + leitwegCheckDigits("00"), "the coarse addressing (Grobadressierung) must start"},
		{"17-" + leitwegCheckDigits("17"), "the coarse addressing (Grobadressierung) must start"},
		{"04011000--03", "the fine addressing"},
		{"04011000-1234_5-03", "the fine addressing"},
		{"04011000-12345 -03", "the fine addressing"},
		{"04011000-Ä1234-03", "the fine addressing"},
		{"04-" + strings.Repeat("Z", 31) + "-03", "the fine addressing"},
		{"04011000-12345-3", "the check digits"},
		{"04011000-12345-003", "the check digits"},
		{"04011000-12345-0A", "the check digits"},
		{" 04011000-12345-03", "the coarse addressing"},
		{"04011000-12345-03 ", "the check digits"},
	} {
		if ValidLeitwegID(c.s) {
			t.Errorf("ValidLeitwegID(%q) = true", c.s)
		}
		if got := leitwegFormat(c.s); !strings.HasPrefix(got, c.why) || (c.why == "") != (got == "") {
			t.Errorf("leitwegFormat(%q) = %q, want %q...", c.s, got, c.why)
		}
	}
}

func TestLeitwegCheckDigits(t *testing.T) {
	for _, c := range []struct{ addr, want string }{
		{"040110001234512345", "06"}, // specification, chapter 2.4: remainder 92
		{"0401100012345", "03"},
		{"99133333TEST", "33"},
		{"99133333test", "33"},
		{"13L75810002000", "60"},
	} {
		if got := leitwegCheckDigits(c.addr); got != c.want {
			t.Errorf("leitwegCheckDigits(%q) = %s, want %s", c.addr, got, c.want)
		}
	}
	// The whole number leaves a remainder of 1 (chapter 2.4).
	if r := mod97("04011000123451234506"); r != 1 {
		t.Errorf("mod97 = %d, want 1", r)
	}
	if r := mod97("04011000123451234500"); r != 92 {
		t.Errorf("mod97 = %d, want 92", r)
	}
}

func TestValidGEBA(t *testing.T) {
	const widnr = "DE123456788" // fictitious W-IdNr with a valid check digit
	for _, s := range []string{
		widnr,
		widnr + "-00001",
		widnr + ".RECH4711",
		widnr + "-00001.RECH4711",
		widnr + "-00000.a",
		widnr + ".rech4711",
		widnr + ".12345678",
		"DE136695976",
	} {
		if !ValidGEBA(s) {
			t.Errorf("ValidGEBA(%q) = false (%s)", s, gebaFormat(s))
		}
	}

	for _, c := range []struct {
		s, why string // why: start of the reason of gebaFormat, "" for a wrong check digit
	}{
		// The examples of the specification (chapter 2.5) have the format
		// of a GEBA, not the check digit of a W-IdNr.
		{"DE999999999", ""},
		{"DE999999999-00001.RECH4711", ""},
		{"DE123456789", ""},
		{"", "it must start with DE"},
		{"de123456788", "it must start with DE"},
		{"AT123456788", "it must start with DE"},
		{"DE12345678", "it must start with DE"},
		{"DE12345678X", "it must start with DE"},
		{"DE 123456788", "it must start with DE"},
		{" " + widnr, "it must start with DE"},
		{widnr + " ", "the W-IdNr may only be followed"},
		{widnr + "0", "the W-IdNr may only be followed"},
		{widnr + "/00001", "the W-IdNr may only be followed"},
		{widnr + "-", "the distinguishing feature"},
		{widnr + "-0001", "the distinguishing feature"},
		{widnr + "-0000A", "the distinguishing feature"},
		{widnr + "-000001", "the W-IdNr may only be followed"},
		{widnr + "-00001-00002", "the W-IdNr may only be followed"},
		{widnr + ".", "the sub-address"},
		{widnr + ".RECH47110", "the sub-address"},
		{widnr + ".RECH-4711", "the sub-address"},
		{widnr + ".RECH.4711", "the sub-address"},
		{widnr + ".RÉCH", "the sub-address"},
		{widnr + ".RECH4711-00001", "the sub-address"},
	} {
		if ValidGEBA(c.s) {
			t.Errorf("ValidGEBA(%q) = true", c.s)
		}
		if got := gebaFormat(c.s); !strings.HasPrefix(got, c.why) || (c.why == "") != (got == "") {
			t.Errorf("gebaFormat(%q) = %q, want %q...", c.s, got, c.why)
		}
	}
}

func TestWIdNrCheckDigit(t *testing.T) {
	for _, c := range []struct {
		d    string
		want byte
	}{
		{"13669597", '6'},
		{"12345678", '8'},
		{"99999999", '5'},
	} {
		if got := widnrCheckDigit(c.d); got != c.want {
			t.Errorf("widnrCheckDigit(%q) = %c, want %c", c.d, got, c.want)
		}
	}
}

// checkIdentifiers runs CheckIdentifiers on a CII or UBL document.
func checkIdentifiers(t *testing.T, data string) *rules.Report {
	t.Helper()
	root, err := xmltree.Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	r := &rules.Report{}
	CheckIdentifiers(&rules.Checker{Doc: newDoc(root), Report: r, Source: rules.SourceDE})
	return r
}

func TestCheckIdentifiers(t *testing.T) {
	const (
		cBuyerRef = "<ram:BuyerReference>04011000-12345-03</ram:BuyerReference>"
		cBuyerURI = `<ram:URIID schemeID="EM">eingang@amt.example</ram:URIID>`
		pBuyerRef = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement/ram:BuyerReference"
		pSellerID = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement/ram:SellerTradeParty/ram:URIUniversalCommunication/ram:URIID"
		pBuyerID  = "/rsm:CrossIndustryInvoice/rsm:SupplyChainTradeTransaction/ram:ApplicableHeaderTradeAgreement/ram:BuyerTradeParty/ram:URIUniversalCommunication/ram:URIID"
	)
	uri := func(scheme, v string) string { return `<ram:URIID schemeID="` + scheme + `">` + v + `</ram:URIID>` }
	base := load(t, base)
	for _, c := range []struct {
		name  string
		edits []edit
		rule  string // "" for no finding
		path  string
		msg   string // part of the message
	}{
		{"base invoice", nil, "", "", ""},
		{"BT-10 format", one(rep(cBuyerRef, elem("BuyerReference", "04011000-12345"))), RuleLeitwegFormat, pBuyerRef,
			`[DE-LEITWEG-01] The Buyer reference (BT-10) should be a Leitweg-ID (Formatspezifikation Leitweg-ID 2.0.2): the check digits (Prüfziffer) must be two digits; value: "04011000-12345".`},
		{"BT-10 other reference", one(rep(cBuyerRef, elem("BuyerReference", "PO-4512"))), RuleLeitwegFormat, pBuyerRef, `value: "PO-4512"`},
		{"BT-10 check digits", one(rep(cBuyerRef, elem("BuyerReference", "04011000-12345-34"))), RuleLeitwegKey, pBuyerRef,
			`[DE-LEITWEG-02] The check digits of the Leitweg-ID in the Buyer reference (BT-10) should be 03, not 34 (ISO/IEC 7064 MOD 97-10); value: "04011000-12345-34".`},
		{"BT-10 trimmed", one(rep(cBuyerRef, elem("BuyerReference", "\n  04011000-12345-03 "))), "", "", ""},
		{"BT-10 blank", one(rep(cBuyerRef, elem("BuyerReference", " "))), "", "", ""},
		{"BT-10 missing", one(remove(cBuyerRef)), "", "", ""},
		{"BT-49 Leitweg-ID", one(rep(cBuyerURI, uri("0204", "991-33333TEST-33"))), "", "", ""},
		{"BT-49 Leitweg-ID format", one(rep(cBuyerURI, uri("0204", "eingang@amt.example"))), RuleLeitwegFormat, pBuyerID,
			"The Buyer electronic address (BT-49) of scheme 0204 should be a Leitweg-ID"},
		{"BT-49 Leitweg-ID check digits", one(rep(cBuyerURI, uri(" 0204 ", "991-33333TEST-34"))), RuleLeitwegKey, pBuyerID,
			"in the Buyer electronic address (BT-49) of scheme 0204 should be 33, not 34"},
		{"BT-34 Leitweg-ID check digits", one(rep(aSellerURI, uri("0204", "992-90009-97"))), RuleLeitwegKey, pSellerID,
			"in the Seller electronic address (BT-34) of scheme 0204 should be 96, not 97"},
		{"BT-49 GEBA", one(rep(cBuyerURI, uri("0246", "DE123456788-00001.RECH4711"))), "", "", ""},
		{"BT-49 GEBA format", one(rep(cBuyerURI, uri("0246", "DE123456788.RECHNUNG4711"))), RuleGEBAFormat, pBuyerID,
			`[DE-GEBA-01] The Buyer electronic address (BT-49) of scheme 0246 should be a GEBA (GEBA Formatspezifikation 1.0.1): the sub-address (Unter-Adressierung) after '.' must have 1 to 8 letters or digits; value: "DE123456788.RECHNUNG4711".`},
		{"BT-34 GEBA check digit", one(rep(aSellerURI, uri("0246", "DE999999999-00001"))), RuleGEBAKey, pSellerID,
			`[DE-GEBA-02] The check digit of the W-IdNr DE999999999 in the Seller electronic address (BT-34) of scheme 0246 should be 5, not 9 (ISO 7064 MOD 11,10, as for the USt-IdNr); value: "DE999999999-00001".`},
		{"BT-49 blank GEBA", one(rep(cBuyerURI, uri("0246", " "))), "", "", ""},
		{"other scheme", one(rep(cBuyerURI, uri("0088", "04011000-12345-34"))), "", "", ""},
	} {
		r := checkIdentifiers(t, apply(t, base, c.edits))
		if c.rule == "" {
			if len(r.Findings) != 0 {
				t.Errorf("%s: unexpected findings\n%s", c.name, r)
			}
			continue
		}
		if len(r.Findings) != 1 {
			t.Errorf("%s: %d findings, want 1 %s\n%s", c.name, len(r.Findings), c.rule, r)
			continue
		}
		if f := r.Findings[0]; f.Rule != c.rule || f.Severity != rules.Warning || f.Source != rules.SourceDE ||
			f.Path != c.path || f.Line == 0 || !strings.Contains(f.Message, c.msg) {
			t.Errorf("%s: finding %+v, want %s at %s with %q", c.name, f, c.rule, c.path, c.msg)
		}
	}

	// Both electronic addresses and BT-10 are checked.
	r := checkIdentifiers(t, apply(t, base, one(
		rep(cBuyerRef, elem("BuyerReference", "04011000-12345-34")),
		rep(cBuyerURI, uri("0204", "04011000-12345-34")),
		rep(aSellerURI, uri("0246", "DE12345678")),
	)))
	if got := strings.Join(r.Rules(), " "); got != "DE-GEBA-01 DE-LEITWEG-02" || len(r.Findings) != 3 {
		t.Errorf("three wrong identifiers:\n%s", r)
	}

	CheckIdentifiers(nil)
	CheckIdentifiers(&rules.Checker{})
	CheckIdentifiers(&rules.Checker{Doc: &rules.Doc{}})
}

// TestCheckIdentifiersUBL checks that the findings on a UBL invoice are
// located in the UBL document.
func TestCheckIdentifiersUBL(t *testing.T) {
	doc := load(t, "testdata/xrechnung.ubl.xml")
	if r := checkIdentifiers(t, doc); len(r.Findings) != 0 {
		t.Errorf("base UBL invoice:\n%s", r)
	}
	doc = apply(t, doc, one(
		rep("<cbc:BuyerReference>04011000-12345-03</cbc:BuyerReference>", "<cbc:BuyerReference>04011000-12345-34</cbc:BuyerReference>"),
		rep(`<cbc:EndpointID schemeID="EM">eingang@amt.example</cbc:EndpointID>`, `<cbc:EndpointID schemeID="0246">DE123456789</cbc:EndpointID>`),
	))
	r := checkIdentifiers(t, doc)
	want := []struct{ rule, path string }{
		{RuleLeitwegKey, "/Invoice/cbc:BuyerReference"},
		{RuleGEBAKey, "/Invoice/cac:AccountingCustomerParty/cac:Party/cbc:EndpointID"},
	}
	if len(r.Findings) != len(want) {
		t.Fatalf("%d findings, want %d\n%s", len(r.Findings), len(want), r)
	}
	for i, w := range want {
		if f := r.Findings[i]; f.Rule != w.rule || f.Path != w.path || f.Line == 0 {
			t.Errorf("finding %d: %+v, want %s at %s", i, f, w.rule, w.path)
		}
	}
}

func TestIdentifierCatalog(t *testing.T) {
	want := []string{RuleLeitwegFormat, RuleLeitwegKey, RuleGEBAFormat, RuleGEBAKey}
	if len(IdentifierCatalog) != len(want) {
		t.Fatalf("IdentifierCatalog = %+v", IdentifierCatalog)
	}
	for i, r := range IdentifierCatalog {
		if r.ID != want[i] || r.Severity != rules.Warning || r.Description == "" {
			t.Errorf("IdentifierCatalog[%d] = %+v, want %s", i, r, want[i])
		}
	}
	for _, c := range [][]rules.RuleInfo{Catalog, UBLCatalog, ProfileCatalog} {
		for _, r := range c {
			if strings.HasPrefix(r.ID, "DE-LEITWEG") || strings.HasPrefix(r.ID, "DE-GEBA") {
				t.Errorf("%s is in a catalog of the KoSIT assertions", r.ID)
			}
		}
	}
}
