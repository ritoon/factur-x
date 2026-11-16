package xmltree

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// differential parses data with Parse and with the encoding/xml reference:
// both must refuse it, or both must accept it and build the same tree.
func differential(t *testing.T, name string, data []byte) {
	t.Helper()
	want, werr := parseStd(data)
	got, gerr := Parse(data)
	switch {
	case werr != nil && gerr != nil:
		return
	case werr != nil:
		t.Fatalf("%s: Parse accepts what encoding/xml refuses (%v):\n%q", name, werr, clip(data))
	case gerr != nil:
		t.Fatalf("%s: Parse refuses what encoding/xml accepts: %v\n%q", name, gerr, clip(data))
	}
	if diff := compareTrees(want, got, nil); diff != "" {
		t.Fatalf("%s: trees differ: %s\n%q", name, diff, clip(data))
	}
}

func clip(b []byte) []byte {
	if len(b) > 400 {
		return b[:400]
	}
	return b
}

// compareTrees returns "" when a and b hold the same elements, attributes,
// texts and lines, and are linked the same way.
func compareTrees(a, b, parent *Node) string {
	where := a.Path()
	switch {
	case a.Name != b.Name || a.Space != b.Space || a.Prefix != b.Prefix:
		return fmt.Sprintf("%s: name %s:%s {%s}, got %s:%s {%s}", where, a.Prefix, a.Name, a.Space, b.Prefix, b.Name, b.Space)
	case a.Line != b.Line:
		return fmt.Sprintf("%s: line %d, got %d", where, a.Line, b.Line)
	case a.Text != b.Text:
		return fmt.Sprintf("%s: text %q, got %q", where, a.Text, b.Text)
	case (a.Attrs == nil) != (b.Attrs == nil) || len(a.Attrs) != len(b.Attrs):
		return fmt.Sprintf("%s: attributes %q, got %q", where, a.Attrs, b.Attrs)
	case (a.Children == nil) != (b.Children == nil) || len(a.Children) != len(b.Children):
		return fmt.Sprintf("%s: %d children, got %d", where, len(a.Children), len(b.Children))
	case b.Parent != parent:
		return fmt.Sprintf("%s: wrong parent", where)
	}
	for i := range a.Attrs {
		if a.Attrs[i] != b.Attrs[i] {
			return fmt.Sprintf("%s: attribute %q, got %q", where, a.Attrs[i], b.Attrs[i])
		}
	}
	if cap(b.Attrs) != len(b.Attrs) || cap(b.Children) != len(b.Children) {
		return fmt.Sprintf("%s: attribute or child list with spare capacity", where)
	}
	for i := range a.Children {
		if d := compareTrees(a.Children[i], b.Children[i], b); d != "" {
			return d
		}
	}
	return ""
}

// differentialSeeds exercise every construct of the scanner, valid or not.
var differentialSeeds = []string{
	// Structure.
	``, ` `, `<`, `<a`, `<a>`, `<a/>`, `<a></a>`, `<a></b>`, `<a/><b/>`, `</a>`, `<a/></a>`,
	`text<a/>`, `<a/>text`, "\n<a/>\n", "<a/>\u00a0", "<a/>\u3000", "<a/>\ufeff", "\ufeff<a/>", "\xef\xbb\xbf<a/>",
	`<a><b><c/></b><b>x</b></a>`, `<a >`, `< a/>`, `<a/ >`, `<a / >`, `<a/b>`, `</ a>`, `<a></a >`, `<a></a b>`,
	"<a\n>\n<b\r\n/>\r\n</a\t>",
	// Names.
	`<:a/>`, `<a:/>`, `<:/>`, `<a:b:c/>`, `<1a/>`, `<-a/>`, `<.a/>`, `<_a/>`, `<a-b.c_d1/>`, `<x:a xmlns:x="u"/>`,
	"<\u00e9t\u00e9/>", "<a\u00b7/>", "<\u00b7a/>", "<a\u0300/>", "<\u4e00/>", "<a\U00010000/>", "<\xff/>", "<a\xc3/>",
	`<a b:c:d="1"/>`, `<a :b="1"/>`, `<a b:="1"/>`,
	// Attributes.
	`<a b="1" c='2'/>`, `<a b="1"c="2"/>`, `<a b = "1"/>`, `<a b/>`, `<a b=1/>`, `<a b="1/>`, `<a b="<"/>`,
	`<a b="]]>"/>`, `<a b="x&amp;y&lt;&gt;&apos;&quot;"/>`, `<a b="&#65;&#x42;&#X43;"/>`, `<a b="'"/>`, `<a b='"'/>`,
	"<a b=\"x\r\ny\rz\n\"/>", "<a b=\"\t\"/>", `<a b="1" b="2"/>`, `<a b="&#0;"/>`, "<a b=\"\x01\"/>", "<a b=\"\xff\"/>",
	// Namespaces.
	`<a xmlns="u"><b/></a>`, `<x:a xmlns:x="u"><x:b/></x:a>`, `<x:a/>`, `<a xmlns:x="u"/><x:b/>`, `<x:a xmlns:x=""/>`,
	`<a xmlns:x="u"><x:b xmlns:x="v"/><x:c/></a>`, `<xml:a/>`, `<a xml:lang="fr"/>`, `<a xmlns:="u"/>`,
	`<a xmlns:x="1" xmlns:x="2"><x:b/></a>`, `<a xmlns="u" xmlns=""/>`, `<x:a xmlns:x="u"></a>`, `<x:a xmlns:x="u"></y:a>`,
	`<a x:b="1"/>`, `<a xmlns:x="&amp;"><x:b/></a>`,
	// Text and references.
	`<a> x </a>`, `<a>&amp;&lt;&gt;&apos;&quot;</a>`, `<a>&amp</a>`, `<a>&foo;</a>`, `<a>&;</a>`, `<a>& b</a>`, `<a>&</a>`,
	`<a>&#65;&#x41;&#x0041;&#0000065;</a>`, `<a>&#xD800;</a>`, `<a>&#xDFFF;</a>`, `<a>&#xFFFE;</a>`, `<a>&#xFFFD;</a>`,
	`<a>&#x10FFFF;</a>`, `<a>&#x110000;</a>`, `<a>&#99999999999999999999999;</a>`, `<a>&#;</a>`, `<a>&#x;</a>`,
	`<a>&#9;&#10;&#13;</a>`, `<a>&#0;</a>`, `<a>&#31;</a>`, `<a>&#32;</a>`, `<a>&#65</a>`, `<a>&#xG;</a>`, `<a>&#-1;</a>`,
	`<a>]]></a>`, `<a>]]]></a>`, `<a>] ]></a>`, `<a>]]&gt;</a>`, `<a>]</a>`, `<a>]]</a>`, `<a>x]]`, `<a>&lt:x;</a>`,
	"<a>x\r\ny\rz\r\r\nw</a>", "<a>\r</a>", "<a>\r&amp;\n</a>", "<a>\x00</a>", "<a>\x0b</a>", "<a>\t\n</a>",
	"<a>\xc3\xa9\xe2\x82\xac\xf0\x9f\x98\x80</a>", "<a>\xc3</a>", "<a>\xed\xa0\x80</a>", "<a>\xef\xbf\xbe</a>",
	"<a>\xef\xbf\xbf</a>", "<a>\xc2\x85</a>", "<a>\xf4\x90\x80\x80</a>",
	`<a>x<b/>y<c/>z</a>`, `<a>x<!-- c -->y</a>`, `<a><?pi?>x</a>`,
	// CDATA.
	`<a><![CDATA[x<y&z]]></a>`, `<a><![CDATA[]]></a>`, `<a>x<![CDATA[y]]>z</a>`, `<a><![CDATA[x]]]></a>`,
	`<a><![CDATA[x</a>`, `<a><![CDAT[x]]></a>`, `<a><![cdata[x]]></a>`, `<![CDATA[ ]]><a/>`, `<![CDATA[x]]><a/>`,
	"<a><![CDATA[\r\n\r]]></a>", "<a><![CDATA[\x01]]></a>", "<a><![CDATA[\xff]]></a>", `<a><![`, `<a><![CDATA[`,
	// Comments.
	`<!-- c --><a/>`, `<a/><!-- c -->`, `<!----><a/>`, `<!---><a/>`, `<!-- a -- b --><a/>`, `<!-- a ---><a/>`,
	`<!- c --><a/>`, `<!--`, `<a><!-- <b> --></a>`, "<!-- \x00\xff --><a/>",
	// Processing instructions and XML declarations.
	`<?xml version="1.0"?><a/>`, `<?xml version="1.1"?><a/>`, `<?xml version='2.0'?><a/>`, `<?xml?><a/>`,
	`<?xml version="1.0" encoding="UTF-8"?><a/>`, `<?xml version="1.0" encoding="utf8"?><a/>`,
	`<?xml encoding="US-ASCII"?><a>é</a>`, `<?xml encoding="EBCDIC"?><a/>`, `<?xml encoding="utf-16"?><a/>`,
	"<?xml encoding=\"ISO-8859-1\"?><a b=\"\xe9\">caf\xe9</a>", "<?xml encoding=\"windows-1252\"?><a>\x80\x81\x9f</a>",
	"<?xml encoding=\"latin1\"?>\n<a>\n<b>\xff</b></a>", "<a><?xml encoding=\"latin1\"?>\xe9</a>",
	"<?xml encoding=\"latin1\"?><?xml encoding=\"latin1\"?><a>\xe9</a>", `<?xml encoding=latin1?><a/>`,
	`<?xml version="1.0"`, `<?pi ?><a/>`, `<?pi data?><a/>`, `<??><a/>`, `<? pi?><a/>`, `<?a??><a/>`, `<?a:b:c?><a/>`,
	`<?xml-stylesheet href="x"?><a/>`, `<?XML version="2"?><a/>`, `<?xml version="1.0" version="2"?><a/>`,
	`<?xml versionx="2"?><a/>`, `<?xml xversion="2"?><a/>`, `<?xml version=?><a/>`, `<?xml version="?><a/>`,
	// Declarations.
	`<!DOCTYPE a><a/>`, `<!DOCTYPE a SYSTEM "x>y"><a/>`, `<!DOCTYPE a [<!ENTITY e "v">]><a>&e;</a>`,
	`<!DOCTYPE a [<!ENTITY e "v">]><a/>`, `<!DOCTYPE a [<!-- > -->]><a/>`, `<!DOCTYPE a [<!ELEMENT a (#PCDATA)>]><a/>`,
	`<!><a/>`, `<!>><a/>`, `<!"><a/>`, `<!x '>'><a/>`, `<!x <>><a/>`, `<!x <<>>><a/>`, `<!x <!-x>><a/>`,
	`<!x <!-->><a/>`, `<!x <!-- --> ><a/>`, `<!DOCTYPE a`, `<!DOCTYPE a [`, `<a><!DOCTYPE b></a>`, `<!`,
	`<!DOCTYPE a [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><a>&xxe;</a>`,
	// Depth.
	strings.Repeat("<a>", MaxDepth) + strings.Repeat("</a>", MaxDepth),
	strings.Repeat("<a>", MaxDepth+1) + strings.Repeat("</a>", MaxDepth+1),
	strings.Repeat("<a>", MaxDepth) + "<b/>" + strings.Repeat("</a>", MaxDepth),
	// Lines.
	"<a>\n\n<b\nc='1\n2'>\n</b><!--\n--><c/><![CDATA[\n]]><d/>\n</a>",
	"\r\n<a>\r\n<b/>\r<c/></a>",
}

func TestDifferentialSeeds(t *testing.T) {
	for i, s := range differentialSeeds {
		differential(t, fmt.Sprintf("seed %d", i), []byte(s))
	}
}

// TestDifferentialFiles compares both parsers on the test files of the
// repository and on the corpus named by FX_CORPUS and FX_CORPUS_XML.
func TestDifferentialFiles(t *testing.T) {
	dirs := []string{"../../testdata", "../../cii/testdata", "../../validate/testdata"}
	for _, env := range []string{"FX_CORPUS", "FX_CORPUS_XML"} {
		if dir := os.Getenv(env); dir != "" {
			dirs = append(dirs, dir)
		}
	}
	n := 0
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".xml") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			differential(t, path, data)
			n++
			return nil
		})
	}
	if n == 0 {
		t.Skip("no XML file found")
	}
	t.Logf("%d files", n)
}

// TestNameTables checks the name characters against encoding/xml, for every
// character of the Basic Multilingual Plane and a sample of the others.
func TestNameTables(t *testing.T) {
	accepts := func(doc string) bool {
		_, err := xml.NewDecoder(strings.NewReader(doc)).RawToken()
		return err == nil
	}
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if r > 0xFFFF && r%97 != 0 || 0xD800 <= r && r <= 0xDFFF {
			continue
		}
		s := string(r)
		if strings.ContainsRune("!?/", r) {
			// After '<' they start something else than a name.
			if validName([]byte(s)) {
				t.Errorf("%U first in a name: validName true", r)
			}
		} else if b := []byte(s); validName(b) != accepts("<"+s+"/>") {
			t.Errorf("%U first in a name: validName %v", r, validName(b))
		}
		if r < utf8.RuneSelf && nameClass[r]&inName == 0 {
			continue // ends the name
		}
		if b := []byte("a" + s); validName(b) != accepts("<a"+s+"/>") {
			t.Errorf("%U in a name: validName %v", r, validName(b))
		}
	}
}

// FuzzParse is the differential fuzzing of Parse against encoding/xml:
//
//	go test -run '^$' -fuzz FuzzParse ./internal/xmltree
func FuzzParse(f *testing.F) {
	for _, s := range differentialSeeds {
		f.Add([]byte(s))
	}
	for _, name := range []string{"EN16931_Einfach.cii.xml"} {
		if data, err := os.ReadFile(filepath.Join("../../testdata", name)); err == nil {
			f.Add(data)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		differential(t, "input", data)
	})
}
