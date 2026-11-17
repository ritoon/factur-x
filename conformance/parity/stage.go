package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// corpusDirs are the directories of ZUGFeRD/corpus that the comparison
// covers: the CII, UBL and Factur-X samples of XML-Rechnung, and the valid
// and invalid ZUGFeRD 2 / Factur-X invoices. ZUGFeRD 1.0 (whose schematron in
// Mustang has no rule identifiers), the Peppol samples of 25 and 61 MB,
// fatturaPA and the unsorted directories are left out.
var corpusDirs = []string{
	"XML-Rechnung/CII",
	"XML-Rechnung/UBL",
	"XML-Rechnung/FX",
	"ZUGFeRDv2/correct",
	"ZUGFeRDv2/fail",
}

// entry is one invoice of the comparison.
type entry struct {
	Name   string // staged file name: 0001.xml, 0002.pdf…
	Source string // origin: corpus/<path> or kosit/<path>
}

func (e entry) isPDF() bool { return strings.HasSuffix(e.Name, ".pdf") }

// negative reports whether the invoice is invalid on purpose: the "fail"
// directories of the corpus, and the not_validating sample of XML-Rechnung.
func (e entry) negative() bool {
	return strings.Contains(e.Source, "/fail/") || strings.Contains(path.Base(e.Source), "not_validating")
}

func stage(corpus, kosit, work string) error {
	var sources []string // origin
	files := map[string]string{}
	add := func(root, prefix, dir string) error {
		return filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext != ".xml" && ext != ".pdf" {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			src := prefix + "/" + filepath.ToSlash(rel)
			sources = append(sources, src)
			files[src] = p
			return nil
		})
	}
	for _, d := range corpusDirs {
		if err := add(corpus, "corpus", d); err != nil {
			return err
		}
	}
	if err := add(kosit, "kosit", "src/test"); err != nil {
		return err
	}
	sort.Strings(sources)

	in := filepath.Join(work, "in")
	if err := os.RemoveAll(in); err != nil {
		return err
	}
	if err := os.MkdirAll(in, 0o755); err != nil {
		return err
	}
	var index, kositList, mustangList strings.Builder
	for i, src := range sources {
		ext := strings.ToLower(filepath.Ext(src))
		e := entry{Name: fmt.Sprintf("%04d%s", i+1, ext), Source: src}
		data, err := os.ReadFile(files[src])
		if err != nil {
			return err
		}
		staged := filepath.Join(in, e.Name)
		if err := os.WriteFile(staged, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(&index, "%s\t%s\n", e.Name, e.Source)
		abs, err := filepath.Abs(staged)
		if err != nil {
			return err
		}
		if !e.isPDF() {
			fmt.Fprintln(&kositList, abs)
		}
		fmt.Fprintln(&mustangList, abs)
	}
	for name, s := range map[string]string{"index.tsv": index.String(), "kosit.list": kositList.String(), "mustang.list": mustangList.String()} {
		if err := os.WriteFile(filepath.Join(work, name), []byte(s), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("%d invoices staged in %s\n", len(sources), in)
	return nil
}

// readIndex reads DIR/index.tsv.
func readIndex(work string) ([]entry, error) {
	data, err := os.ReadFile(filepath.Join(work, "index.tsv"))
	if err != nil {
		return nil, err
	}
	var entries []entry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		name, src, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("index.tsv: bad line %q", line)
		}
		entries = append(entries, entry{Name: name, Source: src})
	}
	return entries, nil
}
