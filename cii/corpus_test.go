package cii

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/xmltree"
	"github.com/ritoon/factur-x/invoice"
)

// The corpus test reads every CII document it can find:
//
//   - FX_CORPUS: the ZUGFeRD corpus (XML-Rechnung/CII/*.cii.xml and the
//     ZUGFeRDv2/correct tree; ZUGFeRDv1 documents must be refused);
//   - FX_CORPUS_XML: any tree of XML files extracted from PDF/A-3 invoices;
//   - ../testdata/*.xml.
//
// For each document it checks the round trip Parse → Marshal (same profile)
// → Parse, the XSD conformance of the output, and the lossless EXTENDED round
// trip of the model. When the same-profile output reports no loss, it must
// hold the whole tree of the original (see canonTree for the forms that are
// normalized). It also measures which leaves of the original document are
// missing from the rewritten ones: none may be missing silently.

// corpusFile is a document of the corpus. Documents of FX_CORPUS and
// testdata must be well-formed (strict); extracted ones may be broken.
type corpusFile struct {
	name, path string
	strict     bool
}

func corpusFiles() []corpusFile {
	var files []corpusFile
	strict := true
	add := func(root, rel string) {
		files = append(files, corpusFile{name: rel, path: filepath.Join(root, rel), strict: strict})
	}
	walk := func(root, sub string, keep func(string) bool) {
		base := filepath.Join(root, sub)
		_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !keep(p) {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			add(root, rel)
			return nil
		})
	}
	isXML := func(p string) bool { return strings.HasSuffix(strings.ToLower(p), ".xml") }
	if root := os.Getenv("FX_CORPUS"); root != "" {
		walk(root, "XML-Rechnung/CII", isXML)
		walk(root, "ZUGFeRDv2/correct", isXML)
		walk(root, "ZUGFeRDv1", isXML)
	}
	walk("../testdata", ".", isXML)
	if root := os.Getenv("FX_CORPUS_XML"); root != "" {
		strict = false
		walk(root, ".", isXML)
	}
	return files
}

// lossStats aggregates the leaves lost by the round trip, per path.
type lossStats struct {
	count map[string]int
	files map[string]map[string]bool
}

func (s *lossStats) add(path, file string) {
	if s.count == nil {
		s.count = map[string]int{}
		s.files = map[string]map[string]bool{}
	}
	s.count[path]++
	if s.files[path] == nil {
		s.files[path] = map[string]bool{}
	}
	s.files[path][file] = true
}

func (s *lossStats) report() string {
	paths := make([]string, 0, len(s.count))
	for p := range s.count {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if s.count[paths[i]] != s.count[paths[j]] {
			return s.count[paths[i]] > s.count[paths[j]]
		}
		return paths[i] < paths[j]
	})
	var b strings.Builder
	for _, p := range paths {
		var sample string
		for f := range s.files[p] {
			if sample == "" || f < sample {
				sample = f
			}
		}
		fmt.Fprintf(&b, "  %5d leaves in %3d files  %s  (e.g. %s)\n", s.count[p], len(s.files[p]), p, filepath.Base(sample))
	}
	return b.String()
}

// normalizePath strips the root element and makes the leaf paths of a
// document comparable across documents.
func normalizePath(p string) string {
	return strings.TrimPrefix(p, "/CrossIndustryInvoice")
}

func TestCorpusRoundTrip(t *testing.T) {
	files := corpusFiles()
	if os.Getenv("FX_CORPUS") == "" && os.Getenv("FX_CORPUS_XML") == "" {
		t.Skip("FX_CORPUS and FX_CORPUS_XML not set")
	}
	var (
		read, notXML, zugferd1, otherRoot, withIssues, withLosses int
		trees, extTrees                                           int
		invalid                                                   int
		unexpected, profile, notFacturX                           lossStats
		added                                                     lossStats
		perProfile                                                = map[invoice.Profile]int{}
	)
	for _, f := range files {
		data, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		inv, err := Parse(data)
		var ie *IssuesError
		switch {
		case errors.As(err, &ie):
			withIssues++
			t.Logf("%s: %v", f.name, ie.Issues)
		case errors.Is(err, ErrNotCII):
			if strings.Contains(err.Error(), "ZUGFeRD 1") {
				zugferd1++
			} else {
				otherRoot++
			}
			continue
		case err != nil:
			if f.strict {
				t.Errorf("%s: %v", f.name, err)
			}
			notXML++
			continue
		}
		if f.strict && strings.Contains(f.name, "ZUGFeRDv1") {
			t.Errorf("%s: ZUGFeRD 1 document not refused", f.name)
		}
		read++
		perProfile[inv.Profile]++

		target := inv.Profile
		if target == invoice.ProfileUnknown {
			target = invoice.ProfileEN16931
		}
		out, err := Marshal(inv, &MarshalOptions{Profile: target, Strict: true})
		var le *LossError
		lossy := errors.As(err, &le)
		if lossy {
			withLosses++
			t.Logf("%s: %v cannot carry %v", f.name, target, le.Terms)
			out, err = Marshal(inv, &MarshalOptions{Profile: target})
		}
		if err != nil {
			t.Errorf("%s: Marshal: %v", f.name, err)
			continue
		}
		checkXSDNamed(t, f.name, target, out)
		back, err := Parse(out)
		if err != nil {
			t.Errorf("%s: re-reading the output: %v", f.name, err)
			continue
		}
		if !lossy {
			if d := diffInvoices(inv, back); d != "" {
				t.Errorf("%s: round trip in %v changes the model:\n%s", f.name, target, d)
			}
		}
		// Writing what was read back is a fixed point.
		if out2, _ := Marshal(back, &MarshalOptions{Profile: target}); string(out2) != string(out) {
			t.Errorf("%s: second write differs from the first", f.name)
		}
		// The model survives an EXTENDED round trip unchanged, unless the
		// source holds data that no Factur-X profile allows.
		ext, err := Marshal(inv, &MarshalOptions{Profile: invoice.ProfileExtended, Strict: true})
		outsideFacturX := errors.As(err, &le)
		if outsideFacturX {
			invalid++
			t.Logf("%s: no Factur-X profile can carry %v", f.name, le.Terms)
			ext, err = Marshal(inv, &MarshalOptions{Profile: invoice.ProfileExtended})
		} else if err == nil {
			backExt := mustParse(t, ext)
			backExt.Profile, backExt.SpecificationID = inv.Profile, inv.SpecificationID
			if d := diffInvoices(inv, backExt); d != "" {
				t.Errorf("%s: EXTENDED round trip changes the model:\n%s", f.name, d)
			}
		}
		if err != nil {
			t.Errorf("%s: EXTENDED: %v", f.name, err)
			continue
		}

		// Leaves of the original missing from the outputs: those missing
		// from the EXTENDED output are not carried by the model; those only
		// missing from the same-profile output are outside that profile.
		orig, _ := xmltree.Parse(data)
		same, _ := xmltree.Parse(out)
		full, _ := xmltree.Parse(ext)
		norm := treeNorm{cur: inv.Currency, taxCur: inv.TaxCurrency, spec: true}
		// Marshal may only add what the target schema requires (an empty
		// mandatory element, a missing indicator…): the leaves added are
		// listed below.
		if !lossy {
			trees++
			if d := diffTrees(orig, same, norm); len(d.missing) > 0 || d.order != "" {
				t.Errorf("%s: the %v output does not hold the tree of the original:\n%v", f.name, target, d)
			}
		}
		// The model carries the whole tree of any document that follows a
		// Factur-X profile: the EXTENDED output holds it, BT-24 aside.
		if !outsideFacturX {
			extTrees++
			norm.spec = false
			if d := diffTrees(orig, full, norm); len(d.missing) > 0 || d.order != "" {
				t.Errorf("%s: the EXTENDED output does not hold the tree of the original:\n%v", f.name, d)
			}
		}
		lo, ls, lf := leaves(orig), leaves(same), leaves(full)
		for l, n := range lo {
			if l.text == "" && !strings.Contains(l.path, "/@") {
				continue // empty element
			}
			p := normalizePath(l.path)
			for i := lf[l]; i < n; i++ {
				switch {
				case strings.HasPrefix(p, "/ExchangedDocumentContext/GuidelineSpecifiedDocumentContextParameter/"):
					// BT-24 changes with the profile.
				case strings.HasSuffix(p, "/@schemaLocation"),
					strings.HasSuffix(p, "/@currencyID") && (l.text == inv.Currency || l.text == inv.TaxCurrency):
					// Normalized forms, see canonTree.
				case outsideFacturX:
					// Reported by Marshal: the source breaks the Factur-X
					// rules (e.g. a reference type on a non-130 document).
					notFacturX.add(p, f.name)
				default:
					unexpected.add(p, f.name)
					t.Logf("%s: lost %s = %q", f.name, p, l.text)
				}
			}
			for i := ls[l]; i < min(n, lf[l]); i++ {
				profile.add(p, f.name)
				if !lossy {
					t.Errorf("%s: %s = %q dropped without a loss being reported", f.name, p, l.text)
				}
			}
		}
		for l, n := range ls {
			for i := lo[l]; i < n; i++ {
				added.add(normalizePath(l.path), f.name)
			}
		}
	}
	t.Logf("%d documents read (%v), %d with unreadable values, %d not fitting their profile, %d not fitting any profile; skipped: %d not XML, %d ZUGFeRD 1 (refused), %d other roots",
		read, perProfile, withIssues, withLosses, invalid, notXML, zugferd1, otherRoot)
	t.Logf("leaves outside the profile of their document (reported as losses):\n%s", profile.report())
	t.Logf("%d documents rewritten with their whole tree in their profile, %d in EXTENDED", trees, extTrees)
	t.Logf("leaves no Factur-X profile allows (reported as losses):\n%s", notFacturX.report())
	t.Logf("leaves added or rewritten by Marshal:\n%s", added.report())
	if len(unexpected.count) > 0 {
		t.Errorf("unexpected losses:\n%s", unexpected.report())
	}
	if read == 0 {
		t.Error("no CII document read")
	}
}

// checkXSDNamed is checkXSD with the document name in the message.
func checkXSDNamed(t *testing.T, name string, p invoice.Profile, out []byte) {
	t.Helper()
	s := loadXSD(t, p)
	if s == nil {
		return
	}
	root, err := xmltree.Parse(out)
	if err != nil {
		t.Errorf("%s: output is not well-formed: %v", name, err)
		return
	}
	if errs := s.check(root); len(errs) > 0 {
		if len(errs) > 5 {
			errs = append(errs[:5], fmt.Sprintf("… %d more", len(errs)-5))
		}
		t.Errorf("%s: %v output does not follow the XSD:\n  %s", name, p, strings.Join(errs, "\n  "))
	}
}
