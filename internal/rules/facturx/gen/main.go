// Command gen generates the code list assertions of package facturx from the
// Factur-X 1.09.2 schematrons of the BASIC, EN 16931 and EXTENDED profiles
// (FNFE-MPE / FeRD, ZUGFeRD 2.5.2): FACTUR-X_<profile>.sch and the code
// lists it reads, FACTUR-X_<profile>_codedb.xml.
//
// The schematrons check every coded value with an assertion such as
//
//	string-length($codeValue13)=0 or
//	document('FACTUR-X_EXTENDED_codedb.xml')/codedb/cl[@id=13]/enumeration[@value=$codeValue13]
//
// where $codeValue13 is the element (".") or one of its attributes
// ("@schemeID"). All these assertions sit in one pattern, where a node is
// checked by the first rule whose context matches it only. The contexts are
// absolute paths of child steps, some with predicates that select a variant
// of the element (an allowance or a charge, a document of type 130, a reason
// code with or without a list identifier...). gen writes, for each profile,
// the rules of the paths that hold a code list assertion, in the order of
// the pattern, with their predicates: the rules without such an assertion
// that precede one on the same path are kept too, as they take the nodes
// they match from the following ones. Two contexts with different paths
// never match the same node, which gen checks: every context of the pattern
// must be an absolute path of named child steps.
//
// The assertions on the specification identifier BT-24 (FX-SCH-A-000493,
// 000556, 000636) are left out: package facturx checks BT-24 with its
// FX-PROFILE rules and FX-SCH-A-000026.
//
// FeRD and FNFE-MPE hand their packages out through a form only. The files
// are read from -dir, searched recursively (the official package, or the
// schematron/ZF_250 directory of the Mustang 2.26.0 jar, which embeds the
// same files), or downloaded from a public copy of the official package
// under Apache-2.0, LandrixSoftware/validator-configuration-zugferd, at a
// pinned commit. Each file must have the SHA-256 recorded in sources, line
// ends normalized to LF: the copies differ only by their line ends (CRLF in
// the official package and in Mustang, LF in the git copy).
//
// Usage (from internal/rules/facturx, through go generate):
//
//	go run ./gen [-dir dir] [-o codedb_gen.go]
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// base is the directory of the Factur-X 1.09.2 package in the pinned commit of
// LandrixSoftware/validator-configuration-zugferd.
const base = "https://raw.githubusercontent.com/LandrixSoftware/validator-configuration-zugferd/8399f5459df34b2af9cae0d613870649b3f17a58/Schema/factur-x-1.09.2/"

// profile is a Factur-X profile whose schematron gen reads.
type profile struct {
	name  string // name of the files: FACTUR-X_<name>.sch
	dir   string // directory of the files under base
	ident string // Go identifier of the profile in the generated file
	label string // name of the profile in messages
}

var profiles = []profile{
	{"BASIC", "2_Factur-X_1.09.2_BASIC", "Basic", "BASIC"},
	{"EN16931", "3_Factur-X_1.09.2_EN16931", "EN16931", "EN 16931"},
	{"EXTENDED", "4_Factur-X_1.09.2_EXTENDED", "Extended", "EXTENDED"},
}

// sources maps the files read to their SHA-256, line ends normalized to LF.
var sources = map[string]string{
	"FACTUR-X_BASIC.sch":           "b2a84cee0e497733bf595c435db18ba720a9f2e16f6741a972667a2f277f6bed",
	"FACTUR-X_BASIC_codedb.xml":    "a150b48359a72e1c1629edddb147d5133aabd03adb6ce8b3d287d0c2a6936d21",
	"FACTUR-X_EN16931.sch":         "ef8c8380b499d1267b2dcf1fa709b45631a77d6d5f220c6cc2ade2fdade59473",
	"FACTUR-X_EN16931_codedb.xml":  "70339a41d71b02bb97683f8eea04acbd764e077a88a446b32860d89c3351b62f",
	"FACTUR-X_EXTENDED.sch":        "5b9039e31f86e90f1a34925e84d3fad206efa3ca5639bb98a0cefd3661d3e36a",
	"FACTUR-X_EXTENDED_codedb.xml": "6e23aedbba0e3b32b3b1211d145186193215f56f755201555306c42327e75fbc",
}

// bt24 is the path of the specification identifier, checked by package
// facturx itself.
const bt24 = "CrossIndustryInvoice/ExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/ID"

func main() {
	dir := flag.String("dir", "", "directory holding the schematrons and code lists (default: download them)")
	out := flag.String("o", "codedb_gen.go", "output file")
	flag.Parse()
	src, err := generate(*dir)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatal(err)
	}
}

// generate reads the files from dir, or downloads them when dir is empty,
// and returns the generated source.
func generate(dir string) ([]byte, error) {
	files, err := read(dir)
	if err != nil {
		return nil, err
	}
	g := &generator{lists: map[string]int{}}
	for _, p := range profiles {
		if err := g.profile(p, files); err != nil {
			return nil, err
		}
	}
	return g.source()
}

// read returns the content of the files of sources, line ends normalized,
// after checking their SHA-256.
func read(dir string) (map[string][]byte, error) {
	paths := map[string]string{}
	if dir != "" {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if _, ok := sources[d.Name()]; ok && paths[d.Name()] == "" {
				paths[d.Name()] = p
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	out := map[string][]byte{}
	for name, sum := range sources {
		var data []byte
		var err error
		if p := paths[name]; p != "" {
			data, err = os.ReadFile(p)
		} else if dir != "" {
			err = fmt.Errorf("%s not found in %s", name, dir)
		} else {
			data, err = download(name)
		}
		if err != nil {
			return nil, err
		}
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		h := sha256.Sum256(data)
		if got := hex.EncodeToString(h[:]); got != sum {
			return nil, fmt.Errorf("%s: SHA-256 %s, want %s", name, got, sum)
		}
		out[name] = data
	}
	return out, nil
}

// download fetches a file of the Factur-X package from the pinned copy.
func download(name string) ([]byte, error) {
	var dir string
	for _, p := range profiles {
		if strings.HasPrefix(name, "FACTUR-X_"+p.name+".") || strings.HasPrefix(name, "FACTUR-X_"+p.name+"_") {
			dir = p.dir
		}
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(base + dir + "/" + name)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", name, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// XML of the schematron and of the code lists.
type (
	schSchema struct {
		Patterns []schPattern `xml:"pattern"`
	}
	schPattern struct {
		Rules []schRule `xml:"rule"`
	}
	schRule struct {
		Context string      `xml:"context,attr"`
		Lets    []schLet    `xml:"let"`
		Asserts []schAssert `xml:"assert"`
	}
	schLet struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}
	schAssert struct {
		ID   string `xml:"id,attr"`
		Flag string `xml:"flag,attr"`
		Test string `xml:"test,attr"`
		Text string `xml:",chardata"`
	}
	codeDB struct {
		Lists []struct {
			ID     int `xml:"id,attr"`
			Values []struct {
				Value string `xml:"value,attr"`
			} `xml:"enumeration"`
		} `xml:"cl"`
	}
)

// codeTest matches the test of a code list assertion.
var codeTest = regexp.MustCompile(`^string-length\(\$(\w+)\)=0 or document\('([^']+)'\)/codedb/cl\[@id=(\d+)\]/enumeration\[@value=\$(\w+)\]$`)

// rule is a rule of the generated tables.
type rule struct {
	path  []string // local names of the steps
	preds []pred   // predicates, by step
	id    string   // code list assertion, "" for a rule that only takes nodes
	attr  string   // attribute holding the code, "" for the element
	cl    int      // number of the list in the codedb file
	list  int      // index of the list in the generated codeLists
	msg   string   // message of the assertion
	ctx   string   // context of the rule, for the catalog
}

// pred is a predicate on a step of a context.
type pred struct {
	step int
	cond cond
}

type generator struct {
	lists     map[string]int // values joined by "\x00" → index in order
	order     [][]string
	byProfile [][]rule
	skipped   []string
}

func (g *generator) profile(p profile, files map[string][]byte) error {
	var sch schSchema
	if err := xml.Unmarshal(files["FACTUR-X_"+p.name+".sch"], &sch); err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	var db codeDB
	if err := xml.Unmarshal(files["FACTUR-X_"+p.name+"_codedb.xml"], &db); err != nil {
		return fmt.Errorf("%s codedb: %w", p.name, err)
	}
	lists := map[int][]string{}
	for _, l := range db.Lists {
		var vals []string
		for _, v := range l.Values {
			vals = append(vals, v.Value)
		}
		slices.Sort(vals)
		lists[l.ID] = slices.Compact(vals)
	}

	// The pattern of the code list assertions.
	var pat *schPattern
	for i := range sch.Patterns {
		for _, r := range sch.Patterns[i].Rules {
			for _, a := range r.Asserts {
				if strings.Contains(a.Test, "/codedb/") {
					if pat != nil && pat != &sch.Patterns[i] {
						return fmt.Errorf("%s: code list assertions in two patterns", p.name)
					}
					pat = &sch.Patterns[i]
				}
			}
		}
	}
	if pat == nil {
		return fmt.Errorf("%s: no code list assertion", p.name)
	}

	// Parse every context, and find the paths that hold an assertion.
	type parsed struct {
		r     *schRule
		path  []string
		preds []rawPred
	}
	var all []parsed
	hasCode := map[string]bool{}
	for i := range pat.Rules {
		r := &pat.Rules[i]
		path, preds, err := parseContext(r.Context)
		if err != nil {
			return fmt.Errorf("%s: context %q: %w", p.name, r.Context, err)
		}
		all = append(all, parsed{r, path, preds})
		key := strings.Join(path, "/")
		for _, a := range r.Asserts {
			if strings.Contains(a.Test, "/codedb/") && key != bt24 {
				hasCode[key] = true
			}
		}
	}
	// Last rule with an assertion, per path: the rules after it on the same
	// path can be dropped.
	last := map[string]int{}
	for i, x := range all {
		key := strings.Join(x.path, "/")
		for _, a := range x.r.Asserts {
			if strings.Contains(a.Test, "/codedb/") && hasCode[key] {
				last[key] = i
			}
		}
	}

	var out []rule
	for i, x := range all {
		key := strings.Join(x.path, "/")
		if !hasCode[key] || i > last[key] {
			continue
		}
		r := rule{path: x.path, ctx: x.r.Context}
		for _, rp := range x.preds {
			c, err := parsePredicate(rp.expr)
			if err != nil {
				return fmt.Errorf("%s: context %q: predicate [%s]: %w", p.name, x.r.Context, rp.expr, err)
			}
			r.preds = append(r.preds, pred{rp.step, c})
		}
		lets := map[string]string{}
		for _, l := range x.r.Lets {
			lets[l.Name] = l.Value
		}
		for _, a := range x.r.Asserts {
			if !strings.Contains(a.Test, "/codedb/") {
				continue
			}
			m := codeTest.FindStringSubmatch(a.Test)
			if m == nil || m[1] != m[4] || m[2] != "FACTUR-X_"+p.name+"_codedb.xml" {
				return fmt.Errorf("%s: %s: unexpected test %q", p.name, a.ID, a.Test)
			}
			if a.Flag != "" {
				return fmt.Errorf("%s: %s: flag %q, want none (fatal)", p.name, a.ID, a.Flag)
			}
			if r.id != "" {
				return fmt.Errorf("%s: %s: two code list assertions in one rule", p.name, a.ID)
			}
			v, ok := lets[m[1]]
			switch {
			case !ok:
				return fmt.Errorf("%s: %s: no let for $%s", p.name, a.ID, m[1])
			case v == ".":
			case strings.HasPrefix(v, "@") && isName(v[1:]):
				r.attr = v[1:]
			default:
				return fmt.Errorf("%s: %s: unexpected value %q", p.name, a.ID, v)
			}
			r.id = a.ID
			r.cl, _ = strconv.Atoi(m[3])
			vals, ok := lists[r.cl]
			if !ok || len(vals) == 0 {
				return fmt.Errorf("%s: %s: no code list %d", p.name, a.ID, r.cl)
			}
			r.list = g.list(vals)
			r.msg = strings.Join(strings.Fields(a.Text), " ")
		}
		out = append(out, r)
	}
	for _, x := range all {
		if strings.Join(x.path, "/") != bt24 {
			continue
		}
		for _, a := range x.r.Asserts {
			if strings.Contains(a.Test, "/codedb/") {
				g.skipped = append(g.skipped, a.ID)
			}
		}
	}
	g.byProfile = append(g.byProfile, out)
	return nil
}

// list returns the index of a list of values in the generated codeLists.
func (g *generator) list(vals []string) int {
	key := strings.Join(vals, "\x00")
	if i, ok := g.lists[key]; ok {
		return i
	}
	g.lists[key] = len(g.order)
	g.order = append(g.order, vals)
	return len(g.order) - 1
}

// rawPred is a predicate of a context, not compiled yet.
type rawPred struct {
	step int
	expr string
}

// parseContext splits an absolute context into the local names of its steps
// and the predicates of each step.
func parseContext(ctx string) ([]string, []rawPred, error) {
	if !strings.HasPrefix(ctx, "/") || strings.HasPrefix(ctx, "//") {
		return nil, nil, errors.New("not an absolute path of child steps")
	}
	steps, err := splitSteps(ctx[1:])
	if err != nil {
		return nil, nil, err
	}
	var path []string
	var preds []rawPred
	for i, s := range steps {
		name, rest, _ := strings.Cut(s, "[")
		if !isQName(name) {
			return nil, nil, fmt.Errorf("step %q is not a named child step", s)
		}
		_, local, _ := strings.Cut(name, ":")
		path = append(path, local)
		for rest != "" {
			body, after, err := bracket("[" + rest)
			if err != nil {
				return nil, nil, err
			}
			preds = append(preds, rawPred{i, body})
			rest = strings.TrimPrefix(after, "[")
			if after != "" && !strings.HasPrefix(after, "[") {
				return nil, nil, fmt.Errorf("unexpected %q after a predicate", after)
			}
		}
	}
	return path, preds, nil
}

// splitSteps splits a path at the slashes outside brackets and quotes.
func splitSteps(s string) ([]string, error) {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == '/' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if depth != 0 || quote != 0 {
		return nil, errors.New("unbalanced brackets or quotes")
	}
	return append(out, s[start:]), nil
}

// bracket returns the content of the bracket that starts s and what follows.
func bracket(s string) (string, string, error) {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[':
			depth++
		case c == ']':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], nil
			}
		}
	}
	return "", "", errors.New("unbalanced brackets")
}

var (
	qname = regexp.MustCompile(`^[A-Za-z_][\w.-]*:[A-Za-z_][\w.-]*$`)
	name  = regexp.MustCompile(`^[A-Za-z_][\w.-]*$`)
)

func isQName(s string) bool { return qname.MatchString(s) }
func isName(s string) bool  { return name.MatchString(s) }

// cond is a predicate of a context, compiled to the operations of package
// facturx (see codeCond there).
type cond struct {
	op   string // codeText, codeAttr, codeAttrValue, codeAttrText, codeNot, codeAnd, codeOr
	path string // codeText, codeAttrText: path of the compared nodes
	attr string // codeAttr, codeAttrValue, codeAttrText
	val  string // codeText, codeAttrValue: the literal
	sub  []cond // codeNot, codeAnd, codeOr
}

// parsePredicate compiles the XPath of a predicate. Only the forms used by
// the Factur-X schematrons are accepted: comparisons of an attribute or of a
// path with a string literal, of an attribute with a path, the existence of
// an attribute, and not(), and, or.
func parsePredicate(s string) (cond, error) {
	toks, err := tokenize(s)
	if err != nil {
		return cond{}, err
	}
	p := &parser{toks: toks}
	c, err := p.or()
	if err != nil {
		return cond{}, err
	}
	if p.i != len(p.toks) {
		return cond{}, fmt.Errorf("unexpected %q", p.toks[p.i])
	}
	return c, nil
}

// tokenize splits a predicate into parentheses, "=", string literals (kept
// with their quotes) and words (names, paths, attributes, keywords).
func tokenize(s string) ([]string, error) {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			i++
		case '(', ')', '=':
			out = append(out, string(c))
			i++
		case '"', '\'':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				return nil, errors.New("unterminated string")
			}
			out = append(out, s[i:i+j+2])
			i += j + 2
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t\n\r()='\"", rune(s[j])) {
				j++
			}
			out = append(out, s[i:j])
			i = j
		}
	}
	return out, nil
}

type parser struct {
	toks []string
	i    int
}

func (p *parser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *parser) or() (cond, error) {
	c, err := p.and()
	if err != nil {
		return c, err
	}
	for p.peek() == "or" {
		p.i++
		d, err := p.and()
		if err != nil {
			return c, err
		}
		c = cond{op: "codeOr", sub: []cond{c, d}}
	}
	return c, nil
}

func (p *parser) and() (cond, error) {
	c, err := p.unary()
	if err != nil {
		return c, err
	}
	for p.peek() == "and" {
		p.i++
		d, err := p.unary()
		if err != nil {
			return c, err
		}
		c = cond{op: "codeAnd", sub: []cond{c, d}}
	}
	return c, nil
}

func (p *parser) unary() (cond, error) {
	switch p.peek() {
	case "not":
		p.i++
		if p.peek() != "(" {
			return cond{}, errors.New("not without (")
		}
		p.i++
		c, err := p.or()
		if err != nil {
			return c, err
		}
		if p.peek() != ")" {
			return cond{}, errors.New("missing )")
		}
		p.i++
		return cond{op: "codeNot", sub: []cond{c}}, nil
	case "(":
		p.i++
		c, err := p.or()
		if err != nil {
			return c, err
		}
		if p.peek() != ")" {
			return cond{}, errors.New("missing )")
		}
		p.i++
		return c, nil
	}
	left := p.peek()
	p.i++
	if p.peek() != "=" {
		if attr, ok := strings.CutPrefix(left, "@"); ok && isName(attr) {
			return cond{op: "codeAttr", attr: attr}, nil
		}
		return cond{}, fmt.Errorf("unsupported operand %q", left)
	}
	p.i++
	right := p.peek()
	p.i++
	lit, isLit := literal(right)
	attr, isAttr := strings.CutPrefix(left, "@")
	isAttr = isAttr && isName(attr)
	switch {
	case isAttr && isLit:
		return cond{op: "codeAttrValue", attr: attr, val: lit}, nil
	case isAttr:
		path, err := nodePath(right)
		if err != nil {
			return cond{}, err
		}
		return cond{op: "codeAttrText", attr: attr, path: path}, nil
	case isLit:
		path, err := nodePath(left)
		if err != nil {
			return cond{}, err
		}
		return cond{op: "codeText", path: path, val: lit}, nil
	}
	return cond{}, fmt.Errorf("unsupported comparison %s = %s", left, right)
}

// literal returns the value of a string literal token.
func literal(tok string) (string, bool) {
	if len(tok) >= 2 && (tok[0] == '"' || tok[0] == '\'') && tok[len(tok)-1] == tok[0] {
		return tok[1 : len(tok)-1], true
	}
	return "", false
}

// nodePath compiles a path of elements: child steps relative to the context
// node, possibly after parent steps (..), or an absolute path. Prefixes are
// dropped, as in the other rules of package facturx.
func nodePath(s string) (string, error) {
	abs := strings.HasPrefix(s, "/")
	parts := strings.Split(strings.TrimPrefix(s, "/"), "/")
	for i, part := range parts {
		switch {
		case part == ".." && !abs:
		case isQName(part):
			_, parts[i], _ = strings.Cut(part, ":")
		default:
			return "", fmt.Errorf("unsupported path %q", s)
		}
	}
	out := strings.Join(parts, "/")
	if abs {
		out = "/" + out
	}
	return out, nil
}

// source writes the generated file.
func (g *generator) source() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("// Code generated by go run ./gen; DO NOT EDIT.\n\n")
	b.WriteString("// Source: the Factur-X 1.09.2 schematrons of BASIC, EN 16931 and EXTENDED\n")
	b.WriteString("// (FACTUR-X_<profile>.sch) and their code lists (FACTUR-X_<profile>_codedb.xml).\n")
	fmt.Fprintf(&b, "// Assertions on BT-24 left out (see package facturx): %s.\n\n", strings.Join(g.skipped, ", "))
	b.WriteString("package facturx\n\n")
	for i, p := range profiles {
		fmt.Fprintf(&b, "// codeRules%s are the code list assertions of FACTUR-X_%s.sch and the\n", p.ident, p.name)
		b.WriteString("// rules of the same paths that precede them, in the order of the pattern.\n")
		fmt.Fprintf(&b, "var codeRules%s = []codeRule{\n", p.ident)
		for _, r := range g.byProfile[i] {
			fmt.Fprintf(&b, "\t{path: %q", strings.Join(r.path, "/"))
			if len(r.preds) > 0 {
				b.WriteString(", when: []codeStepCond{")
				for j, pr := range r.preds {
					if j > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "{%d, ", pr.step)
					writeCond(&b, pr.cond, true)
					b.WriteString("}")
				}
				b.WriteString("}")
			}
			if r.id != "" {
				fmt.Fprintf(&b, ", id: %q", r.id)
				if r.attr != "" {
					fmt.Fprintf(&b, ", attr: %q", r.attr)
				}
				fmt.Fprintf(&b, ", cl: %d, list: %d, msg: %q, ctx: %q", r.cl, r.list, r.msg, compact(r.ctx))
			}
			b.WriteString("},\n")
		}
		b.WriteString("}\n\n")
	}
	b.WriteString("// codeLists are the code lists of the codedb files, sorted, each one once.\n")
	b.WriteString("var codeLists = [...][]string{\n")
	for i, vals := range g.order {
		fmt.Fprintf(&b, "\t%d: {", i)
		width := 100 // wrap long lists
		for j, v := range vals {
			q := strconv.Quote(v)
			if j > 0 && width+len(q) > 96 {
				b.WriteString(",\n\t\t")
				width = 0
			} else if j > 0 {
				b.WriteString(", ")
			}
			if j == 0 && len(vals) > 1 && len(strings.Join(vals, `", "`)) > 80 {
				b.WriteString("\n\t\t")
				width = 0
			}
			b.WriteString(q)
			width += len(q) + 2
		}
		if len(vals) > 1 && len(strings.Join(vals, `", "`)) > 80 {
			b.WriteString(",\n\t")
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	return src, nil
}

// compact returns a context without its namespace prefixes, white space
// runs and first steps, for the catalog.
func compact(ctx string) string {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(ctx); i++ {
		c := ctx[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ':' && i+1 < len(ctx) && ctx[i+1] != ':':
			// Drop the prefix just written.
			s := b.String()
			j := len(s)
			for j > 0 && (isNameByte(s[j-1])) {
				j--
			}
			b.Reset()
			b.WriteString(s[:j])
			continue
		}
		b.WriteByte(c)
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	for _, prefix := range []string{"/CrossIndustryInvoice/SupplyChainTradeTransaction/", "/CrossIndustryInvoice/"} {
		if rest, ok := strings.CutPrefix(out, prefix); ok {
			return rest
		}
	}
	return out
}

func isNameByte(c byte) bool {
	return c == '_' || c == '-' || c == '.' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// writeCond writes c as a codeCond literal, its type elided inside a slice.
func writeCond(b *bytes.Buffer, c cond, typed bool) {
	if typed {
		b.WriteString("codeCond")
	}
	fmt.Fprintf(b, "{op: %s", c.op)
	if c.path != "" {
		fmt.Fprintf(b, ", path: %q", c.path)
	}
	if c.attr != "" {
		fmt.Fprintf(b, ", attr: %q", c.attr)
	}
	if c.op == "codeText" || c.op == "codeAttrValue" {
		fmt.Fprintf(b, ", val: %q", c.val)
	}
	if len(c.sub) > 0 {
		b.WriteString(", sub: []codeCond{")
		for i, s := range c.sub {
			if i > 0 {
				b.WriteString(", ")
			}
			writeCond(b, s, false)
		}
		b.WriteString("}")
	}
	b.WriteString("}")
}
