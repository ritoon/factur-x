package facturx

import (
	"os"
	"testing"
)

func BenchmarkExtract(b *testing.B) {
	data, err := os.ReadFile("testdata/EN16931_Einfach.pdf")
	if err != nil {
		b.Skip(err)
	}
	if _, err := Extract(data); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Extract(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckContainer(b *testing.B) {
	data, err := os.ReadFile("testdata/EN16931_Einfach.pdf")
	if err != nil {
		b.Skip(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := CheckContainer(data); err != nil {
			b.Fatal(err)
		}
	}
}

func benchEmbed(b *testing.B, host string, opts *EmbedOptions) {
	data, err := os.ReadFile(host)
	if err != nil {
		b.Skip(err)
	}
	xml, err := os.ReadFile("testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := Embed(data, xml, opts); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Embed(data, xml, opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEmbed(b *testing.B)        { benchEmbed(b, "bench/ecosystem/host.pdf", nil) }
func BenchmarkEmbedClassic(b *testing.B) { benchEmbed(b, "bench/ecosystem/host_classic.pdf", nil) }
