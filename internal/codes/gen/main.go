// Command gen generates the code lists of package codes from the CII
// validation artefacts of CEN TC 434 (EN 16931).
//
// It reads the release archive en16931-cii-<version>.zip, either from a local
// file (-zip) or downloaded from the GitHub release of
// ConnectingEurope/eInvoicing-EN16931, checks its SHA-256 and writes the code
// lists found in schematron/codelist/EN16931-CII-codes.sch (rules BR-CL-*) and
// the VAT identifier prefixes of BR-CO-09 (schematron/CII/EN16931-CII-model.sch).
//
// Usage (from internal/codes, through go generate):
//
//	go run ./gen [-zip en16931-cii-1.3.17.zip] [-o codes_gen.go]
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	release = "validation-1.3.17"
	archive = "en16931-cii-1.3.17.zip"
	url     = "https://github.com/ConnectingEurope/eInvoicing-EN16931/releases/download/" + release + "/" + archive
	sum     = "e5f3235824b6b38454b1d54cf3f14e01efb5b33971fbefc4e64997d0ef619e89"

	codesFile = "schematron/codelist/EN16931-CII-codes.sch"
	modelFile = "schematron/CII/EN16931-CII-model.sch"
)

// list describes one generated code list.
type list struct {
	Var   string   // Go variable name
	Doc   string   // comment
	Rules []string // schematron rules carrying the list; they must agree
	codes []string
}

var lists = []*list{
	{Var: "documentTypes", Doc: "UNTDID 1001 invoice and credit note type codes (BR-CL-01)", Rules: []string{"BR-CL-01"}},
	{Var: "currencies", Doc: "ISO 4217 alpha-3 currency codes (BR-CL-03, BR-CL-04, BR-CL-05)", Rules: []string{"BR-CL-03", "BR-CL-04", "BR-CL-05"}},
	{Var: "taxPointDateCodes", Doc: "UNTDID 2005 VAT point date codes, restricted by UNTDID 2475 (BR-CL-06)", Rules: []string{"BR-CL-06"}},
	{Var: "referenceTypes", Doc: "UNTDID 1153 reference code qualifiers (BR-CL-07)", Rules: []string{"BR-CL-07"}},
	{Var: "textSubjects", Doc: "UNTDID 4451 text subject code qualifiers (BR-CL-08)", Rules: []string{"BR-CL-08"}},
	{Var: "icd", Doc: "ISO 6523 ICD identifier schemes (BR-CL-10, BR-CL-11, BR-CL-21, BR-CL-26)", Rules: []string{"BR-CL-10", "BR-CL-11", "BR-CL-21", "BR-CL-26"}},
	{Var: "itemClassifications", Doc: "UNTDID 7143 item type identification codes (BR-CL-13)", Rules: []string{"BR-CL-13"}},
	{Var: "countries", Doc: "ISO 3166-1 alpha-2 country codes, with 1A (Kosovo) and XI (Northern Ireland) (BR-CL-14, BR-CL-15)", Rules: []string{"BR-CL-14", "BR-CL-15"}},
	{Var: "paymentMeans", Doc: "UNTDID 4461 payment means codes (BR-CL-16)", Rules: []string{"BR-CL-16"}},
	{Var: "vatCategories", Doc: "UNTDID 5305 duty or tax or fee category codes, EN 16931 subset (BR-CL-17, BR-CL-18)", Rules: []string{"BR-CL-17", "BR-CL-18"}},
	{Var: "allowanceReasons", Doc: "UNTDID 5189 allowance reason codes (BR-CL-19)", Rules: []string{"BR-CL-19"}},
	{Var: "chargeReasons", Doc: "UNTDID 7161 special service description codes (BR-CL-20)", Rules: []string{"BR-CL-20"}},
	{Var: "vatex", Doc: "CEF VATEX VAT exemption reason codes, upper case (BR-CL-22)", Rules: []string{"BR-CL-22"}},
	{Var: "units", Doc: "UN/ECE Recommendations 20 and 21 unit codes (BR-CL-23)", Rules: []string{"BR-CL-23"}},
	{Var: "mimeCodes", Doc: "MIME codes of attached documents (BR-CL-24)", Rules: []string{"BR-CL-24"}},
	{Var: "eas", Doc: "CEF EAS electronic address schemes (BR-CL-25)", Rules: []string{"BR-CL-25"}},
	{Var: "vatPrefixes", Doc: "country prefixes of VAT identifiers (BR-CO-09)", Rules: []string{"BR-CO-09"}},
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("codes/gen: ")
	zipPath := flag.String("zip", "", "path of "+archive+" (downloaded from GitHub when empty)")
	out := flag.String("o", "codes_gen.go", "output file")
	flag.Parse()

	data, err := load(*zipPath)
	if err != nil {
		log.Fatal(err)
	}
	h := sha256.Sum256(data)
	if got := hex.EncodeToString(h[:]); got != sum {
		log.Fatalf("%s: SHA-256 %s, want %s", archive, got, sum)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		log.Fatal(err)
	}
	codes, err := readFile(zr, codesFile)
	if err != nil {
		log.Fatal(err)
	}
	model, err := readFile(zr, modelFile)
	if err != nil {
		log.Fatal(err)
	}
	found, err := codeLists(codes)
	if err != nil {
		log.Fatal(err)
	}
	prefixes, err := vatPrefixes(model)
	if err != nil {
		log.Fatal(err)
	}
	found["BR-CO-09"] = prefixes

	for _, l := range lists {
		for i, r := range l.Rules {
			c, ok := found[r]
			if !ok {
				log.Fatalf("rule %s not found", r)
			}
			c = sorted(c)
			if i == 0 {
				l.codes = c
			} else if !slices.Equal(c, l.codes) {
				log.Fatalf("rules %s and %s carry different code lists", l.Rules[0], r)
			}
		}
	}
	src, err := generate()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatal(err)
	}
}

func load(path string) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func readFile(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name || strings.HasSuffix(f.Name, "/"+name) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%s: not in %s", name, archive)
}

// latin1 returns a reader converting ISO-8859-1 to UTF-8.
func latin1(label string, r io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "utf-8", "utf8":
		return r, nil
	case "iso-8859-1", "latin1":
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, len(b))
		for _, c := range b {
			out = utf8.AppendRune(out, rune(c))
		}
		return bytes.NewReader(out), nil
	}
	return nil, fmt.Errorf("unsupported encoding %q", label)
}

type schPattern struct {
	Rules []struct {
		Context string `xml:"context,attr"`
		Asserts []struct {
			ID   string `xml:"id,attr"`
			Test string `xml:"test,attr"`
		} `xml:"assert"`
	} `xml:"rule"`
}

var (
	reContains = regexp.MustCompile(`contains\('\s*([^']*?)\s*',\s*concat`)
	reMime     = regexp.MustCompile(`@mimeCode\s*=\s*'([^']*)'`)
)

// codeLists returns the codes of each BR-CL rule.
func codeLists(src []byte) (map[string][]string, error) {
	var p schPattern
	d := xml.NewDecoder(bytes.NewReader(src))
	d.CharsetReader = latin1
	if err := d.Decode(&p); err != nil {
		return nil, fmt.Errorf("%s: %w", codesFile, err)
	}
	out := map[string][]string{}
	for _, r := range p.Rules {
		for _, a := range r.Asserts {
			var codes []string
			if m := reContains.FindStringSubmatch(a.Test); m != nil {
				codes = strings.Fields(m[1])
			} else {
				for _, m := range reMime.FindAllStringSubmatch(a.Test, -1) {
					codes = append(codes, m[1])
				}
			}
			if len(codes) == 0 {
				return nil, fmt.Errorf("%s: no code list in %s", codesFile, a.ID)
			}
			out[a.ID] = codes
		}
	}
	return out, nil
}

// vatPrefixes returns the prefixes listed by the BR-CO-09 parameter.
func vatPrefixes(src []byte) ([]string, error) {
	var p struct {
		Params []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		} `xml:"param"`
	}
	d := xml.NewDecoder(bytes.NewReader(src))
	d.CharsetReader = latin1
	if err := d.Decode(&p); err != nil {
		return nil, fmt.Errorf("%s: %w", modelFile, err)
	}
	for _, prm := range p.Params {
		if prm.Name == "BR-CO-09" {
			m := reContains.FindStringSubmatch(prm.Value)
			if m == nil {
				return nil, fmt.Errorf("%s: no list in BR-CO-09", modelFile)
			}
			return strings.Fields(m[1]), nil
		}
	}
	return nil, fmt.Errorf("%s: BR-CO-09 not found", modelFile)
}

func sorted(c []string) []string {
	c = slices.Clone(c)
	slices.Sort(c)
	return slices.Compact(c)
}

func generate() ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, `// Code generated by internal/codes/gen; DO NOT EDIT.

// Source: CEN TC 434 EN 16931 validation artefacts for CII, release %s
// (%s, SHA-256 %s),
// files %s and %s.
// The artefacts are licensed under the European Union Public Licence (EUPL)
// version 1.2; only the code values are reproduced here.

package codes

// Release is the release of the EN 16931 validation artefacts the code
// lists are taken from.
const Release = %q

`, release, url, sum, codesFile, modelFile, release)
	for _, l := range lists {
		fmt.Fprintf(&b, "// %s lists the %s, sorted.\nvar %s = []string{", l.Var, l.Doc, l.Var)
		for i, c := range l.codes {
			if i%10 == 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%q, ", c)
		}
		b.WriteString("\n}\n\n")
	}
	return format.Source(b.Bytes())
}
