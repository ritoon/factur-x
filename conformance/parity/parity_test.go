package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const kositReport = `<?xml version="1.0" encoding="UTF-8"?>
<rep:report xmlns:rep="http://www.xoev.de/de/validator/varl/1" xmlns:s="http://www.xoev.de/de/validator/framework/1/scenarios" valid="false">
  <rep:engine><rep:name>KoSIT Validator 1.6.3</rep:name></rep:engine>
  <rep:timestamp>2026-11-17T10:00:00Z</rep:timestamp>
  <rep:documentIdentification><rep:documentHash><rep:hashAlgorithm>SHA-256</rep:hashAlgorithm><rep:hashValue>x</rep:hashValue></rep:documentHash><rep:documentReference>0001.xml</rep:documentReference></rep:documentIdentification>
  <rep:scenarioMatched>
    <s:scenario><s:name>EN16931 XRechnung (CII)</s:name><s:description/><s:createReport><s:resource><s:name>Validation report for XRechnung</s:name><s:location>r.xsl</s:location></s:resource><s:customLevel level="error">CII-SR-465</s:customLevel><s:customLevel level="information">CII-SR-475</s:customLevel></s:createReport></s:scenario>
    <rep:validationStepResult id="val-xsd" valid="false">
      <s:resource><s:name>XML Schema for UN/CEFACT XML (SCRDM - CII D16B)</s:name><s:location>resources/cii/16b/xsd/x.xsd</s:location></s:resource>
      <rep:message id="val-xsd.1" level="error" code="cvc-complex-type.2.4.a" lineNumber="3">Invalid content</rep:message>
    </rep:validationStepResult>
    <rep:validationStepResult id="val-sch.1" valid="false">
      <s:resource><s:name>Schematron rules for EN16931 (CII)</s:name><s:location>x.xsl</s:location></s:resource>
      <rep:message id="val-sch.1.1" level="error" code="BR-CO-10">[BR-CO-10]-Sum of Invoice line net amount</rep:message>
      <rep:message id="val-sch.1.2" level="information" code="BR-CL-25">[BR-CL-25]-Endpoint identifier scheme</rep:message>
      <rep:message id="val-sch.1.3" level="warning" code="BR-CO-10">again, lower</rep:message>
      <rep:message id="val-sch.1.4" level="warning" code="CII-SR-465">[CII-SR-465] raised to error by the scenario</rep:message>
      <rep:message id="val-sch.1.5" level="warning" code="CII-SR-475">[CII-SR-475] lowered to information by the scenario</rep:message>
    </rep:validationStepResult>
    <rep:validationStepResult id="val-sch.2" valid="false">
      <s:resource><s:name>Schematron rules for CIUS XRechnung (CII)</s:name><s:location>y.xsl</s:location></s:resource>
      <rep:message id="val-sch.2.1" level="warning" code="BR-DE-21">[BR-DE-21] Specification identifier</rep:message>
      <rep:message id="val-sch.2.2" level="error" code="BR-FR-01">not compared</rep:message>
    </rep:validationStepResult>
  </rep:scenarioMatched>
  <rep:assessment><rep:reject><rep:explanation/></rep:reject></rep:assessment>
</rep:report>`

func TestParseKoSIT(t *testing.T) {
	r, err := parseKoSIT([]byte(kositReport))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{schemaToken: levelError, "BR-CO-10": levelError, "BR-CL-25": levelInfo, "BR-DE-21": levelWarning, "CII-SR-465": levelError, "CII-SR-475": levelInfo}
	if !r.Covered || r.Note != "EN16931 XRechnung (CII)" || !reflect.DeepEqual(r.Rules, want) {
		t.Errorf("got %+v, want rules %v", r, want)
	}
	if !reflect.DeepEqual(r.Other, []string{"BR-FR-01"}) {
		t.Errorf("other = %q", r.Other)
	}

	none := `<rep:report xmlns:rep="http://www.xoev.de/de/validator/varl/1" valid="false"><rep:noScenarioMatched><rep:documentData/></rep:noScenarioMatched><rep:assessment/></rep:report>`
	if r, err := parseKoSIT([]byte(none)); err != nil || r.Covered {
		t.Errorf("no scenario: %+v, %v", r, err)
	}
	broken := `<rep:report xmlns:rep="http://www.xoev.de/de/validator/varl/1" valid="false"><rep:noScenarioMatched><rep:validationStepResult id="wf" valid="false"><rep:message id="m" level="error" code="SAXParseException">not well-formed</rep:message></rep:validationStepResult></rep:noScenarioMatched></rep:report>`
	if r, err := parseKoSIT([]byte(broken)); err != nil || !r.Covered || r.Rules[schemaToken] != levelError {
		t.Errorf("not well-formed: %+v, %v", r, err)
	}
	if _, err := parseKoSIT([]byte(`<x/>`)); err == nil {
		t.Error("not a report: no error")
	}
}

const mustangReport = `<validation filename="0002.pdf" datetime="2026-11-17 10:00:00"><pdf><info><pdfAVersion>3</pdfAVersion></info><error type="7">XMP Metadata: ConformanceLevel contains invalid value</error><summary status="invalid"/></pdf><xml><info><version>2</version><profile>urn:cen.eu:en16931:2017</profile><validator version="2.26.0"></validator><rules><fired>460</fired><failed>3</failed></rules><duration unit="ms">350</duration></info><messages><error type="18">cvc-complex-type.2.4.a: Invalid content was found</error><error type="24" location="/*:CrossIndustryInvoice[1]" criterion="(ram:LineTotalAmount)">[BR-CO-10]-Sum of Invoice line net amount [ID BR-CO-10] from /xslt/ZF_250/FACTUR-X_EN16931.xslt)</error><warning type="24" location="/x" criterion="y">[CII-SR-308]-LineID should not be present [ID CII-SR-308] from /xslt/ZF_250/FACTUR-X_EN16931.xslt)</warning><notice type="27">[BR-DE-21] [ID BR-DE-21] from /xslt/XR_30/XRechnung-CII-validation.xslt)</notice><warning type="10">Arithmetical issue: line total</warning><error type="24">[BR-FR-12] French rule [ID BR-FR-12] from /xslt/x.xsl)</error></messages><summary status="invalid"/></xml><summary status="invalid"/></validation>`

func TestParseMustang(t *testing.T) {
	r, err := parseMustang([]byte(mustangReport))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{schemaToken: levelError, "BR-CO-10": levelError, "CII-SR-308": levelWarning, "BR-DE-21": levelInfo}
	if !r.Covered || !reflect.DeepEqual(r.Rules, want) {
		t.Errorf("rules = %v, want %v", r.Rules, want)
	}
	if len(r.Other) != 3 || !strings.HasPrefix(r.Other[0], "pdf: XMP") || !strings.Contains(r.Other[1], "type 10") || r.Other[2] != "BR-FR-12" {
		t.Errorf("other = %q", r.Other)
	}
	if _, err := parseMustang([]byte(`<rep:report/>`)); err == nil {
		t.Error("not a report: no error")
	}
}

func TestKnownAndDiff(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "known.txt")
	os.WriteFile(name, []byte("# comment\n\nmustang corpus/ZUGFeRDv2/fail/* -BR-FR-* reason one\nkosit * +BR-CL-25 reason two\n"), 0o644)
	known, err := readKnown(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 2 || known[0].reason != "reason one" {
		t.Fatalf("known = %+v", known)
	}
	for _, tc := range []struct {
		k              int
		ref, src, diff string
		want           bool
	}{
		{0, "mustang", "corpus/ZUGFeRDv2/fail/a.pdf", "-BR-FR-12", true},
		{0, "kosit", "corpus/ZUGFeRDv2/fail/a.pdf", "-BR-FR-12", false},
		{0, "mustang", "corpus/ZUGFeRDv2/fail/x/a.pdf", "-BR-FR-12", true},
		{0, "mustang", "corpus/ZUGFeRDv2/correct/a.pdf", "-BR-FR-12", false},
		{0, "mustang", "corpus/ZUGFeRDv2/fail/a.pdf", "+BR-FR-12", false},
		{1, "kosit", "kosit/src/test/a.xml", "+BR-CL-25", true},
		{1, "kosit", "kosit/src/test/a.xml", "+BR-CL-2", false},
	} {
		if got := known[tc.k].match(tc.ref, tc.src, tc.diff); got != tc.want {
			t.Errorf("%d %s %s %s: %v, want %v", tc.k, tc.ref, tc.src, tc.diff, got, tc.want)
		}
	}
	for _, tc := range []struct {
		pattern, s string
		want       bool
	}{
		{"*", "corpus/a/b.pdf", true},
		{"corpus/*/b.pdf", "corpus/a/x/b.pdf", true},
		{"corpus/*/b.pdf", "corpus/a/b.pdfx", false},
		{"*XRechnung*", "corpus/fail/XRechnung 1.XML", true},
		{"a*a", "a", false},
		{"a*a", "aa", true},
		{"corpus/a.pdf", "corpus/a.pdf", true},
		{"corpus/a.pdf", "corpus/b.pdf", false},
	} {
		if got := glob(tc.pattern, tc.s); got != tc.want {
			t.Errorf("glob(%q, %q) = %v", tc.pattern, tc.s, got)
		}
	}
	os.WriteFile(name, []byte("kosit * BR-CL-25 no sign\n"), 0o644)
	if _, err := readKnown(name); err == nil {
		t.Error("line without sign accepted")
	}

	// Errors only, a Factur-X code list assertion being the same as its CEN
	// rule (FX-SCH-A-000571, BR-CL-25), and FX-PROFILE an equivalent only.
	mine := map[string]string{schemaToken: levelError, "BR-CO-10": levelError, "BR-CO-11": levelInfo, "BR-CL-25": levelError, "FX-PROFILE-02": levelError, "BR-CL-23": levelWarning}
	ref := map[string]string{"BR-CO-11": levelError, "BR-CO-12": levelWarning, "FX-SCH-A-000571": levelError, "FX-SCH-A-000636": levelError, "FX-SCH-A-000613": levelError}
	got := diff(mine, ref)
	if want := []string{"+BR-CO-10", "+SCHEMA", "-BR-CO-11", "-FX-SCH-A-000613"}; !reflect.DeepEqual(got, want) {
		t.Errorf("diff = %q, want %q", got, want)
	}
	c := comparison{Ours: result{Rules: mine}, Ref: result{Rules: ref}}
	for d, want := range map[string]string{
		"-FX-SCH-A-000613": "`-FX-SCH-A-000613` (warning)",
		"-BR-CO-11":        "`-BR-CO-11` (info)",
		"+BR-CO-10":        "`+BR-CO-10`",
	} {
		if got := level(c, d); got != want {
			t.Errorf("level(%s) = %s, want %s", d, got, want)
		}
	}
}

// TestOurs runs the package on invoices of the repository: a valid CII
// invoice and a hybrid PDF.
func TestOurs(t *testing.T) {
	data, err := os.ReadFile("../../testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		t.Fatal(err)
	}
	if r := ours("0001.xml", data); !r.Covered || r.Failure != "" || !r.valid() {
		t.Errorf("EN16931_Einfach: %+v", r)
	}
	if r := ours("0002.xml", []byte("<a>")); r.Rules[schemaToken] != levelError {
		t.Errorf("not well-formed: %+v", r)
	}
	pdfs, _ := filepath.Glob("../../testdata/*.pdf")
	if len(pdfs) == 0 {
		t.Skip("no PDF in testdata")
	}
	data, err = os.ReadFile(pdfs[0])
	if err != nil {
		t.Fatal(err)
	}
	if r := ours("0003.pdf", data); !r.Covered || r.Failure != "" {
		t.Errorf("%s: %+v", pdfs[0], r)
	}
}

func TestStage(t *testing.T) {
	corpus, kosit, work := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(root, name, content string) {
		p := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	for _, d := range corpusDirs {
		os.MkdirAll(filepath.Join(corpus, d), 0o755)
	}
	write(corpus, "XML-Rechnung/CII/a.cii.xml", "<a/>")
	write(corpus, "ZUGFeRDv2/fail/b.pdf", "%PDF")
	write(corpus, "ZUGFeRDv2/fail/XRechnung 1.XML", "<c/>")
	write(corpus, "ZUGFeRDv2/fail/notes.txt", "x")
	write(corpus, "ZUGFeRDv1/correct/d.pdf", "%PDF")
	write(kosit, "src/test/business-cases/standard/01.01a-INVOICE_ubl.xml", "<e/>")
	if err := stage(corpus, kosit, work); err != nil {
		t.Fatal(err)
	}
	entries, err := readIndex(work)
	if err != nil {
		t.Fatal(err)
	}
	want := []entry{
		{"0001.xml", "corpus/XML-Rechnung/CII/a.cii.xml"},
		{"0002.xml", "corpus/ZUGFeRDv2/fail/XRechnung 1.XML"},
		{"0003.pdf", "corpus/ZUGFeRDv2/fail/b.pdf"},
		{"0004.xml", "kosit/src/test/business-cases/standard/01.01a-INVOICE_ubl.xml"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %q", entries)
	}
	if !entries[1].negative() || entries[0].negative() {
		t.Error("negative() is wrong")
	}
	list, _ := os.ReadFile(filepath.Join(work, "kosit.list"))
	if n := strings.Count(string(list), "\n"); n != 3 || strings.Contains(string(list), ".pdf") {
		t.Errorf("kosit.list:\n%s", list)
	}
	if b, _ := os.ReadFile(filepath.Join(work, "in", "0002.xml")); string(b) != "<c/>" {
		t.Errorf("staged copy = %q", b)
	}
}
