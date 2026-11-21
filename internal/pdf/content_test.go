package pdf_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/pdf"
)

func TestScanContent(t *testing.T) {
	data := "q 1 0 0 1 72 720 cm % comment\n/GS1 gs 0 0 1 rg BT /F1 12 Tf [(Fac) -20 (ture)] TJ ET\n" +
		"/OC << /MCID 3 >> BDC EMC\n" +
		"BI /W 2 /H 1 /CS /RGB /BPC 8 /F /AHx ID 00FF00EI0000FF> EI Q\n" +
		"BI /W 1 /H 1 /IM true ID \x80 EI\n" +
		"0.5 g ] 1 0 0 RG"
	var got []string
	err := pdf.ScanContent([]byte(data), func(op string, args []pdf.Object) error {
		got = append(got, fmt.Sprintf("%s%v", op, args))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"q[]", "cm[1 0 0 1 72 720]", "gs[GS1]", "rg[0 0 1]", "BT[]", "Tf[F1 12]",
		"TJ[[[70 97 99] -20 [116 117 114 101]]]", "ET[]", "BDC[OC [{MCID 3}]]", "EMC[]",
		"BI[[{W 2} {H 1} {CS RGB} {BPC 8} {F AHx}]]", "Q[]",
		"BI[[{W 1} {H 1} {IM true}]]",
		"g[0.5]", "RG[1 0 0]",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if err := pdf.ScanContent([]byte("BI /W 1 ID \x00\x01"), func(string, []pdf.Object) error { return nil }); !errors.Is(err, pdf.ErrInlineImage) {
		t.Fatalf("inline image without EI: err = %v", err)
	}
	stop := errors.New("stop")
	n := 0
	err = pdf.ScanContent([]byte("q Q q Q"), func(string, []pdf.Object) error {
		n++
		if n == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || n != 2 {
		t.Fatalf("stop: err = %v after %d operators", err, n)
	}
}

func TestContentSyntax(t *testing.T) {
	for _, tc := range []struct {
		content string
		want    []pdf.SyntaxKind
	}{
		{"BT /F1 12 Tf <4142> Tj (abc) Tj ET 1 0 0 1 2147483647 -2147483648 cm", nil},
		{"<414> Tj", []pdf.SyntaxKind{pdf.SyntaxHex}},
		{"<41 4G> Tj", []pdf.SyntaxKind{pdf.SyntaxHex}},
		{"2147483648 0 m -2147483649 0 l", []pdf.SyntaxKind{pdf.SyntaxLimit, pdf.SyntaxLimit}},
		{"(" + strings.Repeat("x", 32768) + ") Tj", []pdf.SyntaxKind{pdf.SyntaxLimit}},
		{"/" + strings.Repeat("n", 128) + " gs", []pdf.SyntaxKind{pdf.SyntaxLimit}},
		// The data of an inline image is not read as tokens.
		{"BI /W 2 /H 1 /CS /G /BPC 8 ID <( EI <4142> Tj", nil},
	} {
		var got []pdf.SyntaxKind
		pdf.ContentSyntax([]byte(tc.content), func(is pdf.SyntaxIssue) { got = append(got, is.Kind) })
		if !slices.Equal(got, tc.want) {
			t.Errorf("%.40q: issues %v, want %v", tc.content, got, tc.want)
		}
	}
}
