package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"testing"
)

func TestDeflateRoundTrip(t *testing.T) {
	for i, data := range samples() {
		c := Deflate(data)
		r, err := zlib.NewReader(bytes.NewReader(c))
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		got, err := io.ReadAll(r) // also verifies the Adler-32 checksum
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("sample %d: round trip differs", i)
		}
		got, err = inflate(c, len(data))
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("sample %d: own inflate: %v", i, err)
		}
	}
}

func FuzzDeflate(f *testing.F) {
	for _, s := range samples()[:8] {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := inflate(Deflate(data), 0)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("round trip: %v", err)
		}
	})
}

func BenchmarkDeflate(b *testing.B) {
	x, err := os.ReadFile("../../testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		b.Skip(err)
	}
	b.Logf("%d -> %d bytes", len(x), len(Deflate(x)))
	b.SetBytes(int64(len(x)))
	b.ReportAllocs()
	for b.Loop() {
		Deflate(x)
	}
}
