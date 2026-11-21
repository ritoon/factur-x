package pdf_test

import (
	"bytes"
	"testing"

	"github.com/ritoon/zerechnung/internal/pdf"
)

func TestWriteFile(t *testing.T) {
	content := []byte("BT /F1 12 Tf (x) Tj ET")
	objs := []pdf.Object{
		pdf.Dict{{"Type", pdf.Name("Catalog")}, {"Pages", pdf.Ref{Num: 2}}},
		pdf.Dict{{"Type", pdf.Name("Pages")}, {"Kids", pdf.Array{pdf.Ref{Num: 3}}}, {"Count", pdf.Int(1)}},
		pdf.Dict{{"Type", pdf.Name("Page")}, {"Parent", pdf.Ref{Num: 2}}, {"Contents", pdf.Ref{Num: 4}},
			{"MediaBox", pdf.Array{pdf.Int(0), pdf.Int(0), pdf.Int(595), pdf.Real(841.89)}}},
		&pdf.Stream{Dict: pdf.Dict{{"Length", pdf.Int(999)}}, Raw: content},
		pdf.Dict{{"Title", pdf.String("Facture \xe9")}},
	}
	data := pdf.WriteFile("1.7", objs, pdf.Dict{{"Root", pdf.Ref{Num: 1}}, {"Info", pdf.Ref{Num: 5}},
		{"ID", pdf.Array{pdf.String("0123456789abcdef"), pdf.String("fedcba9876543210")}}})
	if !bytes.HasPrefix(data, []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")) {
		t.Fatalf("header: %q", data[:20])
	}
	if !bytes.HasSuffix(data, []byte("%%EOF\n")) {
		t.Fatalf("end: %q", data[len(data)-20:])
	}
	r, err := pdf.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	if r.Repaired() || r.Version() != "1.7" {
		t.Fatalf("repaired %v, version %s", r.Repaired(), r.Version())
	}
	pages, _ := r.Dict(r.Catalog().Get("Pages"))
	kids := r.Resolve(pages.Get("Kids")).(pdf.Array)
	page, _ := r.Dict(kids[0])
	st := r.Resolve(page.Get("Contents")).(*pdf.Stream)
	if !bytes.Equal(st.Raw, content) {
		t.Fatalf("content %q", st.Raw)
	}
	if n, _ := st.Dict.Int("Length"); n != int64(len(content)) {
		t.Fatalf("/Length %d", n)
	}
	info, _ := r.Dict(r.Trailer().Get("Info"))
	if s, _ := info.Get("Title").(pdf.String); string(s) != "Facture \xe9" {
		t.Fatalf("title %q", s)
	}
	locs, total, _ := r.InUse(100)
	if total != 5 {
		t.Fatalf("%d objects in use", total)
	}
	for _, loc := range locs {
		r.Syntax(loc, func(is pdf.SyntaxIssue) { t.Errorf("object %d: %s", is.Num, is.Detail) })
	}
}
