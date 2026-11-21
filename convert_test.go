package zerechnung

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ritoon/zerechnung/internal/icc"
	"github.com/ritoon/zerechnung/internal/pdf"
	"github.com/ritoon/zerechnung/internal/pdf/pdftest"
	"github.com/ritoon/zerechnung/internal/xmp"
	"github.com/ritoon/zerechnung/invoice"
)

// convHost builds a one-page PDF for the conversion tests: the catalog is
// object 1, the root of the page tree 2, the page 3, its content stream 4;
// the objects setup adds follow.
type convHost struct {
	version   string
	content   string // content stream of the page
	lzw       bool   // encode the content stream with LZWDecode
	resources string // resource dictionary of the page, "" for none
	inherited bool   // resources in the root of the page tree, not in the page
	info      string // document information dictionary, "" for none
	// setup, when set, adds objects and returns entries of the catalog and
	// of the page.
	setup func(b *pdftest.Builder) (catalog, page string)
}

func (h convHost) build() []byte {
	b := &pdftest.Builder{Version: h.version, Trailer: "/Root 1 0 R"}
	cat, pages, page := b.Reserve(), b.Reserve(), b.Reserve()
	var contents int
	if h.lzw {
		contents = b.AddStream("<< /Filter /LZWDecode >>", lzwLiteral([]byte(h.content)))
	} else {
		contents = b.AddStream("<< >>", []byte(h.content))
	}
	var catExtra, pageExtra string
	if h.setup != nil {
		catExtra, pageExtra = h.setup(b)
	}
	if h.info != "" {
		b.Trailer += fmt.Sprintf(" /Info %d 0 R", b.Add(h.info))
	}
	pagesRes, pageRes := "", ""
	if h.resources != "" {
		if h.inherited {
			pagesRes = "/Resources " + h.resources
		} else {
			pageRes = "/Resources " + h.resources
		}
	}
	b.Set(cat, fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R %s >>", pages, catExtra))
	b.Set(pages, fmt.Sprintf("<< /Type /Pages /Kids [%d 0 R] /Count 1 %s >>", page, pagesRes))
	b.Set(page, fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 595 842] /Contents %d 0 R %s %s >>",
		pages, contents, pageRes, pageExtra))
	return b.Bytes(pdftest.Options{})
}

// lzwLiteral encodes data with LZW without compressing it: a clear code,
// one 9-bit code per byte, the end code. data is short enough for the code
// width to stay 9 bits.
func lzwLiteral(data []byte) []byte {
	var out []byte
	var acc uint32
	bits := 0
	put := func(code uint32) {
		acc = acc<<9 | code
		bits += 9
		for bits >= 8 {
			out = append(out, byte(acc>>(bits-8)))
			bits -= 8
		}
	}
	put(256)
	for _, c := range data {
		put(uint32(c))
	}
	put(257)
	if bits > 0 {
		out = append(out, byte(acc<<(8-bits)))
	}
	return out
}

// testFile reads a file of the tests.
func testFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// calRGB is a device-independent colour space for the tests.
const calRGB = "[/CalRGB << /WhitePoint [0.9505 1 1.089] >>]"

// writeConverted saves a converted file in the directory FX_PDFA_OUT, when
// set, for the veraPDF job of the CI.
func writeConverted(t *testing.T, name string, data []byte) {
	t.Helper()
	dir := os.Getenv("FX_PDFA_OUT")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name = strings.NewReplacer(" ", "-", "/", "-", ",", "").Replace(name)
	if err := os.WriteFile(filepath.Join(dir, "generated-"+name+".pdf"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// converted reads a converted file and checks what every conversion gives:
// the PDF/A rules of CheckContainer met, PDF/A-3 declared, PDF 1.7, a file
// identifier and a PDF/A output intent.
func converted(t *testing.T, out []byte) *container {
	t.Helper()
	if !bytes.HasPrefix(out, []byte("%PDF-1.7\n")) {
		t.Errorf("header %q", out[:min(len(out), 10)])
	}
	for _, e := range pdfaErrors(out) {
		t.Errorf("PDF/A: %s", e)
	}
	c, err := openContainer(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.requirePDFA(); err != nil {
		t.Errorf("PDF/A not declared: %v", err)
	}
	if id, ok := c.r.Resolve(c.r.Trailer().Get("ID")).(pdf.Array); !ok || len(id) != 2 {
		t.Errorf("trailer /ID %v", c.r.Trailer().Get("ID"))
	}
	pdfa := 0
	intents, _ := c.r.Resolve(c.catalog.Get("OutputIntents")).(pdf.Array)
	for _, o := range intents {
		if d, ok := c.r.Dict(o); ok && d.Get("S") == pdf.Object(pdf.Name("GTS_PDFA1")) {
			pdfa++
		}
	}
	if pdfa != 1 {
		t.Errorf("%d PDF/A output intents", pdfa)
	}
	return c
}

// inventory lists the keys of the dictionaries and the names of the objects
// of a file.
func inventory(c *container) (keys, names map[pdf.Name]int) {
	keys, names = map[pdf.Name]int{}, map[pdf.Name]int{}
	var walk func(o pdf.Object)
	walk = func(o pdf.Object) {
		switch v := o.(type) {
		case pdf.Name:
			names[v]++
		case pdf.Dict:
			for _, kv := range v {
				keys[kv.Key]++
				walk(kv.Val)
			}
		case pdf.Array:
			for _, x := range v {
				walk(x)
			}
		case *pdf.Stream:
			walk(v.Dict)
		}
	}
	locs, _, _ := c.r.InUse(1 << 16)
	for _, loc := range locs {
		walk(c.r.Object(loc.Num))
	}
	return keys, names
}

// page returns the dictionary of the first page.
func firstPage(t *testing.T, c *container) pdf.Dict {
	t.Helper()
	pages, _ := c.r.Dict(c.catalog.Get("Pages"))
	kids, _ := c.r.Resolve(pages.Get("Kids")).(pdf.Array)
	if len(kids) == 0 {
		t.Fatal("no page")
	}
	page, _ := c.r.Dict(kids[0])
	return page
}

// resourceKeys returns the entries of a category of a resource dictionary.
func resourceKeys(c *container, res pdf.Object, category pdf.Name) []string {
	rd, _ := c.r.Dict(res)
	sub, _ := c.r.Dict(rd.Get(category))
	var out []string
	for _, kv := range sub {
		out = append(out, string(kv.Key))
	}
	return out
}

func TestConvertPDFA(t *testing.T) {
	ap := func(b *pdftest.Builder) int {
		return b.AddStream("<< /Type /XObject /Subtype /Form /BBox [0 0 40 40] >>", []byte("0 0 1 rg 0 0 40 40 re f"))
	}
	for _, tc := range []struct {
		name  string
		host  []byte
		check func(t *testing.T, c *container, out []byte)
	}{
		{"vector page", convHost{content: "0.2 0.4 0.6 rg 10 10 100 100 re f"}.build(), func(t *testing.T, c *container, _ []byte) {
			intents, _ := c.r.Resolve(c.catalog.Get("OutputIntents")).(pdf.Array)
			oi, _ := c.r.Dict(intents[0])
			st, _ := c.r.Resolve(oi.Get("DestOutputProfile")).(*pdf.Stream)
			if data, err := c.r.Decode(st); err != nil || !bytes.Equal(data, icc.SRGB()) {
				t.Errorf("output intent profile: %v", err)
			}
			if id, _ := oi.Get("OutputConditionIdentifier").(pdf.String); string(id) != icc.Description {
				t.Errorf("OutputConditionIdentifier %q", id)
			}
		}},

		{"forbidden entries", convHost{
			content: "/G0 gs q 10 0 0 10 0 0 cm /Im0 Do Q /Ps0 Do /Fm0 Do",
			lzw:     true,
			setup: func(b *pdftest.Builder) (string, string) {
				js := b.Add("<< /S /JavaScript /JS (app.alert\\(1\\)) >>")
				launch := b.Add("<< /S /Launch /F (calc.exe) >>")
				uri := b.Add(fmt.Sprintf("<< /S /URI /URI (https://example.org) /Next %d 0 R >>", launch))
				gs := b.Add("<< /Type /ExtGState /TR /Identity /BM /Foo /OPM 1 /OP true /RI /Custom >>")
				img := b.AddStream("<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8 /Interpolate true /Alternates [] /OPI << /2.0 << >> >> >>", []byte{0x80})
				ps := b.AddStream("<< /Type /XObject /Subtype /PS >>", []byte("0 0 moveto"))
				psData := b.AddStream("<< >>", []byte("%!PS"))
				form := b.AddStream(fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /PS %d 0 R /Subtype2 /PS >>", psData), []byte("0 0 5 5 re f"))
				ocg := b.Add("<< /Type /OCG /Name (Layer) >>")
				link := b.Add(fmt.Sprintf("<< /Type /Annot /Subtype /Link /Rect [0 0 10 10] /Border [0 0 0] /F 4 /A %d 0 R >>", uri))
				square := b.Add(fmt.Sprintf("<< /Type /Annot /Subtype /Square /Rect [10 10 50 50] /F 0 /AP << /N %d 0 R /D %d 0 R >> /AA << /E %d 0 R >> >>", ap(b), ap(b), js))
				catalog := fmt.Sprintf("/OpenAction %d 0 R /Names << /JavaScript << /Names [(a) %d 0 R] >> >> /AcroForm << /Fields [] /XFA (x) >> /Perms << /UR3 << >> >> /Requirements [] /Version /1.7 /AA << /WC %d 0 R >> /OCProperties << /OCGs [%d 0 R] /D << /AS [] /Order [] >> >>", js, js, js, ocg)
				page := fmt.Sprintf("/AA << /O %d 0 R >> /PresSteps << >> /Annots [%d 0 R %d 0 R] /Resources << /ExtGState << /G0 %d 0 R >> /XObject << /Im0 %d 0 R /Ps0 %d 0 R /Fm0 %d 0 R >> >>", js, link, square, gs, img, ps, form)
				return catalog, page
			},
		}.build(), func(t *testing.T, c *container, _ []byte) {
			keys, names := inventory(c)
			for _, k := range []pdf.Name{"AA", "OpenAction", "JavaScript", "XFA", "Perms", "UR3", "PresSteps", "Requirements",
				"Version", "TR", "OPI", "Alternates", "Interpolate", "AS", "OPM", "RI", "PS", "Subtype2", "Next"} {
				if keys[k] > 0 {
					t.Errorf("key /%s kept", k)
				}
			}
			for _, n := range []pdf.Name{"JavaScript", "Launch", "LZWDecode", "Foo", "PS"} {
				if names[n] > 0 {
					t.Errorf("name /%s kept", n)
				}
			}
			page := firstPage(t, c)
			annots, _ := c.r.Resolve(page.Get("Annots")).(pdf.Array)
			if len(annots) != 2 {
				t.Fatalf("%d annotations", len(annots))
			}
			link, _ := c.r.Dict(annots[0])
			if a, _ := c.r.Dict(link.Get("A")); a.Get("S") != pdf.Object(pdf.Name("URI")) {
				t.Errorf("link action %v", a)
			}
			square, _ := c.r.Dict(annots[1])
			if f, _ := c.r.Resolve(square.Get("F")).(pdf.Int); f != 4 {
				t.Errorf("annotation flags %d", f)
			}
			if apd, _ := c.r.Dict(square.Get("AP")); len(apd) != 1 || apd.Get("N") == nil {
				t.Errorf("appearance %v", apd)
			}
			st, _ := c.r.Resolve(page.Get("Contents")).(*pdf.Stream)
			if data, err := c.r.Decode(st); err != nil || string(data) != "/G0 gs q 10 0 0 10 0 0 cm /Im0 Do Q /Ps0 Do /Fm0 Do" {
				t.Errorf("content %q, %v", data, err)
			}
			if f := st.Dict.Get("Filter"); f != pdf.Object(pdf.Name("FlateDecode")) {
				t.Errorf("content filter %v", f)
			}
		}},

		{"resources", convHost{
			content:   "/Fm0 Do BT /F1 12 Tf 100 700 Td (a) Tj ET",
			resources: "<< /ColorSpace << /CS0 " + calRGB + " >> /XObject << /Fm0 5 0 R >> /Font << /F1 7 0 R >> >>",
			inherited: true,
			setup: func(b *pdftest.Builder) (string, string) {
				b.AddStream("<< /Type /XObject /Subtype /Form /BBox [0 0 100 100] >>", []byte("/CS0 cs 0 1 0 sc 0 0 50 50 re f"))                                                                                                                                 // 5
				glyph := b.AddStream("<< >>", []byte("500 0 d0 /CS0 cs 1 0 0 sc 0 0 500 700 re f"))                                                                                                                                                               // 6
				b.Add(fmt.Sprintf("<< /Type /Font /Subtype /Type3 /FontBBox [0 0 500 700] /FontMatrix [0.001 0 0 0.001 0 0] /CharProcs << /a %d 0 R >> /Encoding << /Type /Encoding /Differences [97 /a] >> /FirstChar 97 /LastChar 97 /Widths [500] >>", glyph)) // 7
				return "", ""
			},
		}.build(), func(t *testing.T, c *container, _ []byte) {
			page := firstPage(t, c)
			if fmt.Sprint(resourceKeys(c, page.Get("Resources"), "XObject")) != "[Fm0]" {
				t.Errorf("page resources %v", page.Get("Resources"))
			}
			rd, _ := c.r.Dict(page.Get("Resources"))
			xobjects, _ := c.r.Dict(rd.Get("XObject"))
			form, _ := c.r.Resolve(xobjects.Get("Fm0")).(*pdf.Stream)
			fonts, _ := c.r.Dict(rd.Get("Font"))
			font, _ := c.r.Dict(fonts.Get("F1"))
			for what, res := range map[string]pdf.Object{"form": form.Dict.Get("Resources"), "Type 3 font": font.Get("Resources")} {
				d, _ := c.r.Dict(res)
				if len(d) != 1 || fmt.Sprint(resourceKeys(c, res, "ColorSpace")) != "[CS0]" {
					t.Errorf("%s resources %v", what, d)
				}
			}
		}},

		{"existing output intent", convHost{
			content: "1 0 0 rg 0 0 10 10 re f",
			setup: func(b *pdftest.Builder) (string, string) {
				profile := b.AddStream("<< /N 3 /Filter /FlateDecode >>", pdftest.Flate(icc.SRGB()))
				return fmt.Sprintf("/OutputIntents [<< /Type /OutputIntent /S /GTS_PDFX /OutputConditionIdentifier (sRGB) /DestOutputProfile %d 0 R >>]", profile), ""
			},
		}.build(), func(t *testing.T, c *container, _ []byte) {
			intents, _ := c.r.Resolve(c.catalog.Get("OutputIntents")).(pdf.Array)
			if len(intents) != 2 {
				t.Fatalf("%d output intents", len(intents))
			}
			x, _ := c.r.Dict(intents[0])
			a, _ := c.r.Dict(intents[1])
			if x.Get("DestOutputProfile") != a.Get("DestOutputProfile") {
				t.Errorf("profiles %v and %v", x.Get("DestOutputProfile"), a.Get("DestOutputProfile"))
			}
		}},

		{"files", convHost{
			content: "0 g 0 0 10 10 re f",
			setup: func(b *pdftest.Builder) (string, string) {
				ef := b.AddStream("<< /Type /EmbeddedFile >>", []byte("notes"))
				spec := b.Add(fmt.Sprintf("<< /Type /Filespec /F (notes.txt) /EF << /F %d 0 R >> >>", ef))
				attach := b.Add(fmt.Sprintf("<< /Type /Annot /Subtype /FileAttachment /Rect [0 0 20 20] /F 4 /FS << /Type /Filespec /F (other.pdf) >> /AP << /N %d 0 R >> >>", ap(b)))
				return fmt.Sprintf("/Names << /EmbeddedFiles << /Names [(notes.txt) %d 0 R] >> >>", spec), fmt.Sprintf("/Annots [%d 0 R]", attach)
			},
		}.build(), func(t *testing.T, c *container, _ []byte) {
			names, _ := c.r.Dict(c.catalog.Get("Names"))
			tree, _ := c.r.Dict(names.Get("EmbeddedFiles"))
			list, _ := c.r.Resolve(tree.Get("Names")).(pdf.Array)
			spec, _ := c.r.Dict(list[1])
			if spec.Get("AFRelationship") != pdf.Object(pdf.Name("Unspecified")) || pdf.Text(spec.Get("UF").(pdf.String)) != "notes.txt" {
				t.Errorf("file specification %v", spec)
			}
			ef, _ := c.r.Dict(spec.Get("EF"))
			st, _ := c.r.Resolve(ef.Get("F")).(*pdf.Stream)
			params, _ := c.r.Dict(st.Dict.Get("Params"))
			if st.Dict.Get("Subtype") != pdf.Object(pdf.Name("text/plain")) || params.Get("ModDate") == nil {
				t.Errorf("embedded file %v", st.Dict)
			}
			af, _ := c.r.Resolve(c.catalog.Get("AF")).(pdf.Array)
			if len(af) != 1 || af[0] != list[1] {
				t.Errorf("catalog /AF %v", af)
			}
			annots, _ := c.r.Resolve(firstPage(t, c).Get("Annots")).(pdf.Array)
			annot, _ := c.r.Dict(annots[0])
			fs, ok := annot.Get("FS").(pdf.Ref)
			other, _ := c.r.Dict(fs)
			if aaf, _ := c.r.Resolve(annot.Get("AF")).(pdf.Array); !ok || len(aaf) != 1 || aaf[0] != pdf.Object(fs) ||
				other.Get("AFRelationship") != pdf.Object(pdf.Name("Unspecified")) {
				t.Errorf("file attachment %v, file specification %v", annot, other)
			}
		}},

		{"metadata", convHost{
			content: "0 g 0 0 10 10 re f",
			info:    "<< /Title (Facture 42) /Author (ACME) /ModDate (D:20200101000000Z) >>",
			setup: func(b *pdftest.Builder) (string, string) {
				packet := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
					`<rdf:Description rdf:about="" xmlns:foo="http://example.org/foo/"><foo:bar>x</foo:bar></rdf:Description></rdf:RDF></x:xmpmeta>`
				return fmt.Sprintf("/Metadata %d 0 R", b.AddStream("<< /Type /Metadata /Subtype /XML >>", []byte(packet))), ""
			},
		}.build(), func(t *testing.T, c *container, _ []byte) {
			st, _ := c.r.Resolve(c.catalog.Get("Metadata")).(*pdf.Stream)
			data, _ := c.r.Decode(st)
			// The property PDF/A does not admit drops the packet, rebuilt
			// from the information dictionary.
			if !bytes.Contains(data, []byte("Facture 42")) || bytes.Contains(data, []byte("foo:bar")) {
				t.Errorf("XMP:\n%s", data)
			}
			info, _ := c.r.Dict(c.r.Trailer().Get("Info"))
			if s, _ := info.Get("ModDate").(pdf.String); string(s) != pdf.FormatDate(testDate) {
				t.Errorf("ModDate %q", s)
			}
		}},

		{"invisible text in a font not embedded", convHost{
			content:   "BT 3 Tr /F1 12 Tf 10 10 Td (x) Tj ET",
			resources: "<< /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >>",
		}.build(), nil},

		{"DeviceCMYK in an unused form", convHost{
			content:   "0 g 0 0 10 10 re f",
			resources: "<< /XObject << /Fm0 5 0 R >> >>",
			setup: func(b *pdftest.Builder) (string, string) {
				b.AddStream("<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] >>", []byte("0 0 0 1 k 0 0 10 10 re f"))
				return "", ""
			},
		}.build(), nil},

		{"hybrid invoice", testFile(t, "testdata/EN16931_Einfach.pdf"), func(t *testing.T, _ *container, out []byte) {
			if _, err := Extract(out); err != nil {
				t.Errorf("invoice: %v", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ConvertPDFA(tc.host, &ConvertOptions{ModDate: testDate})
			if err != nil {
				t.Fatal(err)
			}
			writeConverted(t, tc.name, out)
			c := converted(t, out)
			if tc.check != nil {
				tc.check(t, c, out)
			}

			// The same date gives the same file; the converted file
			// converts again.
			again, err := ConvertPDFA(tc.host, &ConvertOptions{ModDate: testDate})
			if err != nil || !bytes.Equal(again, out) {
				t.Errorf("second conversion differs: %v", err)
			}
			twice, err := ConvertPDFA(out, &ConvertOptions{ModDate: testDate})
			if err != nil {
				t.Fatalf("converting the converted file: %v", err)
			}
			converted(t, twice)
		})
	}
}

func TestConvertPDFARefused(t *testing.T) {
	helvetica := "<< /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >>"
	square := func(flags string, appearance bool) func(b *pdftest.Builder) (string, string) {
		return func(b *pdftest.Builder) (string, string) {
			annot := "<< /Type /Annot /Subtype /Square /Rect [10 10 50 50] " + flags
			if appearance {
				annot += fmt.Sprintf(" /AP << /N %d 0 R >>", b.AddStream("<< /Type /XObject /Subtype /Form /BBox [0 0 40 40] >>", []byte("0 0 40 40 re S")))
			}
			return "", fmt.Sprintf("/Annots [%d 0 R]", b.Add(annot+" >>"))
		}
	}
	for _, tc := range []struct {
		name string
		host []byte
		want string
	}{
		{"font not embedded", convHost{content: "BT /F1 12 Tf 10 10 Td (x) Tj ET", resources: helvetica}.build(),
			`page 1 /Contents: the font "Helvetica" (Type1) shows text but its font program is not embedded`},
		{"host of the tests", testFile(t, "testdata/host.pdf"), `the font "Helvetica"`},
		{"DeviceCMYK", convHost{content: "0 0 0 1 k 0 0 10 10 re f"}.build(),
			"page 1 /Contents: DeviceCMYK is used without a DefaultCMYK colour space nor a CMYK output intent"},
		{"DeviceCMYK in a form", convHost{
			content:   "/Fm0 Do",
			resources: "<< /XObject << /Fm0 5 0 R >> >>",
			setup: func(b *pdftest.Builder) (string, string) {
				b.AddStream("<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] >>", []byte("0 0 0 1 K 0 0 10 10 re S"))
				return "", ""
			},
		}.build(), "page 1 /Contents /XObject /Fm0: DeviceCMYK"},
		{"signed", convHost{content: "0 g", setup: func(b *pdftest.Builder) (string, string) {
			sig := b.Add("<< /FT /Sig /T (Signature1) /V << /Type /Sig >> >>")
			return fmt.Sprintf("/AcroForm << /Fields [%d 0 R] >>", sig), ""
		}}.build(), `form field "Signature1": the document is signed`},
		{"PDF 2.0", convHost{version: "2.0", content: "0 g"}.build(), "the PDF version is 2.0"},
		{"dynamic XFA form", convHost{content: "0 g", setup: func(*pdftest.Builder) (string, string) {
			return "/NeedsRendering true", ""
		}}.build(), "dynamic XFA form"},
		{"hidden annotation", convHost{content: "0 g", setup: square("/F 6", true)}.build(), "PDF-62 (page 1 /Annots [0]"},
		{"annotation without appearance", convHost{content: "0 g", setup: square("/F 4", false)}.build(), "PDF-63 (page 1 /Annots [0] /AP)"},
		{"undefined operator", convHost{content: "0 0 10 10 re f foo"}.build(), `the operator "foo", which ISO 32000-1 does not define`},
		{"undefined operator between BX and EX", convHost{content: "BX foo EX"}.build(), `the operator "foo"`},
		{"odd hexadecimal string", convHost{content: "BT <414> Tj ET"}.build(), "a hexadecimal string has an odd number of digits"},
		{"integer beyond the limits", convHost{content: "2147483648 0 m"}.build(), "the integer 2147483648 is outside the range"},
		{"q nested too deep", convHost{content: strings.Repeat("q ", 29) + strings.Repeat("Q ", 29)}.build(), "q operators are nested 29 deep"},
		{"inline image with LZW data", convHost{content: "BI /W 1 /H 1 /CS /G /BPC 8 /F /LZW ID \x80 EI"}.build(), "an inline image uses the LZWDecode filter"},
		{"ri operator", convHost{content: "/Custom ri"}.build(), "the ri operator sets the rendering intent Custom"},
		{"form with different resources", convHost{
			content:   "/Fm0 Do /Fm1 Do",
			resources: "<< /ColorSpace << /CS0 " + calRGB + " >> /XObject << /Fm0 5 0 R /Fm1 6 0 R >> >>",
			setup: func(b *pdftest.Builder) (string, string) {
				b.AddStream("<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] >>", []byte("/CS0 cs 1 0 0 sc 0 0 10 10 re f")) // 5
				b.AddStream("<< /Type /XObject /Subtype /Form /BBox [0 0 10 10] /Resources << /ColorSpace << /CS0 [/CalRGB << /WhitePoint [0.9505 1 1.089] /Gamma [2.2 2.2 2.2] >>] >> /XObject << /Fm0 5 0 R >> >> >>",
					[]byte("/Fm0 Do")) // 6
				return "", ""
			},
		}.build(), "page 1 /Contents /XObject /Fm1 /XObject /Fm0: the content stream has no resource dictionary of its own and is used with different resources"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ConvertPDFA(tc.host, &ConvertOptions{ModDate: testDate})
			var ce *ConversionError
			if !errors.As(err, &ce) || !errors.Is(err, ErrNotConvertible) || out != nil {
				t.Fatalf("error %v, want a ConversionError", err)
			}
			if !strings.Contains(strings.Join(ce.Reasons, "\n"), tc.want) {
				t.Errorf("reasons:\n%s\nwant %q", strings.Join(ce.Reasons, "\n"), tc.want)
			}
		})
	}

	enc := &fixture{encrypted: true}
	if _, err := ConvertPDFA(enc.build(), nil); !errors.Is(err, ErrEncrypted) {
		t.Errorf("encrypted: %v", err)
	}
	if _, err := ConvertPDFA([]byte("not a PDF"), nil); err == nil || errors.Is(err, ErrNotConvertible) {
		t.Errorf("not a PDF: %v", err)
	}
}

func TestConversionError(t *testing.T) {
	var reasons []string
	for i := range 10 {
		reasons = append(reasons, fmt.Sprintf("reason %d", i))
	}
	err := error(&ConversionError{Reasons: reasons})
	msg := err.Error()
	if !strings.HasPrefix(msg, ErrNotConvertible.Error()+": reason 0; ") || !strings.HasSuffix(msg, "reason 7; and 2 more") {
		t.Errorf("message %q", msg)
	}
	if !errors.Is(err, ErrNotConvertible) {
		t.Error("does not wrap ErrNotConvertible")
	}
}

func TestEmbedConvertPDFA(t *testing.T) {
	x := ciiXML(invoice.URNEN16931)
	host := convHost{content: "0.2 0.4 0.6 rg 10 10 100 100 re f"}.build()
	if _, err := Embed(host, x, &EmbedOptions{ModDate: testDate, RequirePDFA: true}); !errors.Is(err, ErrNotPDFA) {
		t.Fatalf("without ConvertPDFA: %v", err)
	}
	out, err := Embed(host, x, &EmbedOptions{ModDate: testDate, RequirePDFA: true, ConvertPDFA: true})
	if err != nil {
		t.Fatal(err)
	}
	writeConverted(t, "embed", out)
	a, findings, err := CheckContainer(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		t.Errorf("finding: %s", f)
	}
	if !bytes.Equal(a.XML, x) {
		t.Error("invoice not embedded")
	}
	c, _ := openContainer(out)
	st, _ := c.r.Resolve(c.catalog.Get("Metadata")).(*pdf.Stream)
	data, _ := c.r.Decode(st)
	if info, err := xmp.Parse(data); err != nil || info.PDFAPart != "3" {
		t.Errorf("XMP: %+v, %v", info, err)
	}

	// A host declared PDF/A is not converted: the invoice is added by an
	// incremental update.
	pdfa, err := ConvertPDFA(host, &ConvertOptions{ModDate: testDate})
	if err != nil {
		t.Fatal(err)
	}
	out, err = Embed(pdfa, x, &EmbedOptions{ModDate: testDate, ConvertPDFA: true})
	if err != nil || !bytes.HasPrefix(out, pdfa) {
		t.Errorf("PDF/A host: %v, incremental update %v", err, bytes.HasPrefix(out, pdfa))
	}

	// A host that cannot be converted.
	_, err = Embed(testFile(t, "testdata/host.pdf"), x, &EmbedOptions{ModDate: testDate, ConvertPDFA: true})
	if !errors.Is(err, ErrNotConvertible) {
		t.Errorf("host.pdf: %v", err)
	}
}
