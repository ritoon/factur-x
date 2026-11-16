package pdf

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"io"
	"math/rand"
	"os"
	"testing"
)

// samples returns inputs of various shapes for the differential tests.
func samples() [][]byte {
	rng := rand.New(rand.NewSource(1))
	random := func(n int) []byte {
		b := make([]byte, n)
		rng.Read(b)
		return b
	}
	text := func(n int) []byte {
		words := []string{"<ram:ID>", "</ram:ID>", "Rechnung", "factur-x", " ", "\n", "12.50", "EUR", "<rsm:CrossIndustryInvoice>", "é", "\t\t"}
		var b bytes.Buffer
		for b.Len() < n {
			b.WriteString(words[rng.Intn(len(words))])
		}
		return b.Bytes()
	}
	var out [][]byte
	out = append(out, nil, []byte("a"), bytes.Repeat([]byte("a"), 100000), random(10), random(70000))
	for _, n := range []int{100, 1000, 20000, 100000, 300000} {
		out = append(out, text(n))
	}
	// Low-entropy data with long-distance repeats.
	b := text(40000)
	b = append(b, b[:30000]...)
	b = append(b, random(5000)...)
	b = append(b, b[1000:33000]...)
	out = append(out, b)
	if x, err := os.ReadFile("../../testdata/EN16931_Einfach.cii.xml"); err == nil {
		out = append(out, x)
	}
	return out
}

func deflateRaw(t testing.TB, data []byte, level int, flush bool) []byte {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, level)
	if err != nil {
		t.Fatal(err)
	}
	if flush {
		// Sync flushes produce empty stored blocks between chunks.
		for i := 0; i < len(data); i += 7000 {
			w.Write(data[i:min(i+7000, len(data))])
			w.Flush()
		}
	} else {
		w.Write(data)
	}
	w.Close()
	return buf.Bytes()
}

func TestInflateDifferential(t *testing.T) {
	for i, data := range samples() {
		for _, level := range []int{flate.HuffmanOnly, flate.NoCompression, 1, 2, 5, 6, 9} {
			for _, flush := range []bool{false, true} {
				c := deflateRaw(t, data, level, flush)
				got, err := inflateRaw(c, 0)
				if err != nil {
					t.Fatalf("sample %d level %d flush %v: %v", i, level, flush, err)
				}
				if !bytes.Equal(got, data) {
					t.Fatalf("sample %d level %d flush %v: output differs (%d vs %d bytes)", i, level, flush, len(got), len(data))
				}
				// A small hint forces the output buffer to grow.
				got, err = inflateRaw(c, 1)
				if err != nil || !bytes.Equal(got, data) {
					t.Fatalf("sample %d level %d with hint 1: %v", i, level, err)
				}
			}
		}
	}
}

func TestInflateZlib(t *testing.T) {
	data := samples()[6]
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(data)
	w.Close()
	got, err := inflate(buf.Bytes(), len(data))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("zlib: %v", err)
	}
	// Raw deflate without zlib header is accepted too.
	got, err = inflate(deflateRaw(t, data, 6, false), 0)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("raw: %v", err)
	}
}

func TestInflateTruncated(t *testing.T) {
	data := samples()[7]
	for _, level := range []int{flate.NoCompression, 1, 9} {
		c := deflateRaw(t, data, level, false)
		for _, cut := range []int{1, 2, 5, len(c) / 2, len(c) - 1} {
			got, err := inflateRaw(c[:cut], 0)
			if err == nil {
				t.Fatalf("level %d cut %d: no error", level, cut)
			}
			if !bytes.HasPrefix(data, got) {
				t.Fatalf("level %d cut %d: partial output is not a prefix", level, cut)
			}
		}
	}
}

func TestInflateCorrupt(t *testing.T) {
	for _, c := range [][]byte{
		{0x07},                   // reserved block type
		{0x01, 0x05, 0x00, 0x00}, // stored block with bad NLEN
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	} {
		if _, err := inflateRaw(c, 0); err == nil {
			t.Errorf("%x: no error", c)
		}
	}
}

// FuzzInflate checks that the decoder agrees with compress/flate.
func FuzzInflate(f *testing.F) {
	for _, s := range samples()[:8] {
		f.Add(deflateRaw(f, s, 6, false))
	}
	f.Fuzz(func(t *testing.T, c []byte) {
		want, werr := io.ReadAll(flate.NewReader(bytes.NewReader(c)))
		got, err := inflateRaw(c, 0)
		if werr == nil {
			if err != nil {
				t.Fatalf("compress/flate accepts, we fail: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("outputs differ")
			}
		}
	})
}

func BenchmarkInflate(b *testing.B) {
	x, err := os.ReadFile("../../testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		b.Skip(err)
	}
	c := deflateRaw(b, x, 6, false)
	b.SetBytes(int64(len(x)))
	b.ReportAllocs()
	for b.Loop() {
		inflateRaw(c, len(x))
	}
}

func BenchmarkInflateStdlib(b *testing.B) {
	x, err := os.ReadFile("../../testdata/EN16931_Einfach.cii.xml")
	if err != nil {
		b.Skip(err)
	}
	c := deflateRaw(b, x, 6, false)
	b.SetBytes(int64(len(x)))
	b.ReportAllocs()
	r := flate.NewReader(nil)
	for b.Loop() {
		r.(flate.Resetter).Reset(bytes.NewReader(c), nil)
		out := make([]byte, len(x))
		io.ReadFull(r, out)
	}
}
