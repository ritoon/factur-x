package en16931

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ritoon/factur-x/internal/rules"
	"github.com/ritoon/factur-x/internal/xmltree"
)

func benchCheck(b *testing.B, name string) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", name))
	if err != nil {
		b.Skip(err)
	}
	root, err := xmltree.Parse(data)
	if err != nil {
		b.Fatal(err)
	}
	doc := rules.NewDoc(root, 0)
	r := &rules.Report{}
	c := &rules.Checker{Doc: doc, Report: r, Source: rules.SourceEN16931}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Findings = r.Findings[:0]
		Check(c)
	}
	if len(r.Findings) != 0 {
		b.Fatalf("unexpected findings: %v", r.Findings)
	}
}

// BenchmarkCheck measures the rules alone, on an already parsed invoice.
func BenchmarkCheck(b *testing.B) {
	b.Run("Einfach", func(b *testing.B) { benchCheck(b, "EN16931_Einfach.cii.xml") })
	b.Run("Rabatte", func(b *testing.B) { benchCheck(b, "EN16931_Rabatte.cii.xml") })
}
