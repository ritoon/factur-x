package pdf

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"
)

// lzwEncode encodes data as LZWDecode expects it, for the tests.
func lzwEncode(data []byte, earlyChange bool) []byte {
	early := 0
	if earlyChange {
		early = 1
	}
	var out []byte
	var bits uint32
	var nbits uint
	width := 9
	put := func(code int) {
		bits = bits<<uint(width) | uint32(code)
		nbits += uint(width)
		for nbits >= 8 {
			out = append(out, byte(bits>>(nbits-8)))
			nbits -= 8
		}
	}
	table := map[string]int{}
	next := 258
	put(256)
	cur := ""
	for _, c := range data {
		s := cur + string([]byte{c})
		if _, ok := table[s]; ok || len(s) == 1 {
			cur = s
			continue
		}
		code := int(cur[0])
		if len(cur) > 1 {
			code = table[cur]
		}
		put(code)
		table[s] = next
		next++
		// The decoder defines each code one code later than the encoder:
		// its widths follow next-1.
		switch {
		case next == 4096:
			put(256)
			table, next, width = map[string]int{}, 258, 9
		case next-1+early >= 1<<width:
			width++
		}
		cur = string([]byte{c})
	}
	if cur != "" {
		code := int(cur[0])
		if len(cur) > 1 {
			code = table[cur]
		}
		put(code)
		next++
		if next-1+early >= 1<<width && width < 12 {
			width++
		}
	}
	put(257)
	if nbits > 0 {
		out = append(out, byte(bits<<(8-nbits)))
	}
	return out
}

func TestLZWDecode(t *testing.T) {
	// ISO 32000-1, 7.4.4.2, Example 2.
	got, err := lzwDecode([]byte{0x80, 0x0B, 0x60, 0x50, 0x22, 0x0C, 0x0C, 0x85, 0x01}, true)
	if err != nil || string(got) != "-----A---B" {
		t.Fatalf("example of ISO 32000-1: %q, %v", got, err)
	}

	rng := rand.New(rand.NewSource(1))
	text := bytes.Repeat([]byte("BT /F1 12 Tf 72 712 Td (Facture n. 42) Tj ET\n"), 300)
	noise := make([]byte, 20000)
	for i := range noise {
		noise[i] = byte(rng.Intn(256))
	}
	for _, data := range [][]byte{nil, []byte("a"), text, noise, append(append([]byte{}, text...), noise...)} {
		for _, early := range []bool{true, false} {
			got, err := lzwDecode(lzwEncode(data, early), early)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("round trip of %d bytes (early change %v): %d bytes, %v", len(data), early, len(got), err)
			}
		}
	}

	if _, err := lzwDecode([]byte{0xff, 0xff}, true); !errors.Is(err, errLZW) {
		t.Fatalf("first code 511: err = %v", err)
	}
	old := maxDecoded
	maxDecoded = 1000
	defer func() { maxDecoded = old }()
	if _, err := lzwDecode(lzwEncode(text, true), true); !errors.Is(err, errTooLarge) {
		t.Fatalf("size limit: err = %v", err)
	}
}

func TestLZWStream(t *testing.T) {
	data := bytes.Repeat([]byte("0 0 1 rg 10 10 100 100 re f\n"), 50)
	// /Predictor 2 (TIFF) on rows of 7 bytes, after LZW.
	pred := make([]byte, len(data))
	copy(pred, data)
	for i := len(pred) - 1; i >= 0; i-- {
		if i%7 != 0 {
			pred[i] -= pred[i-1]
		}
	}
	r := &Reader{}
	for _, tc := range []struct {
		d   Dict
		raw []byte
	}{
		{Dict{{"Filter", Name("LZWDecode")}}, lzwEncode(data, true)},
		{Dict{{"Filter", Name("LZW")}, {"DecodeParms", Dict{{"EarlyChange", Int(0)}}}}, lzwEncode(data, false)},
		{Dict{{"Filter", Name("LZWDecode")}, {"DecodeParms", Dict{{"Predictor", Int(2)}, {"Columns", Int(7)}}}}, lzwEncode(pred, true)},
		{Dict{{"Filter", Array{Name("ASCIIHexDecode"), Name("LZWDecode")}}}, []byte(hexString(lzwEncode(data, true)) + ">")},
	} {
		got, err := r.Decode(&Stream{Dict: tc.d, Raw: tc.raw})
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("%v: %q, %v", tc.d, got, err)
		}
	}
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, 2*len(b))
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&15])
	}
	return string(out)
}

func TestRunLengthDecode(t *testing.T) {
	got, err := runLengthDecode([]byte{2, 'a', 'b', 'c', 254, 'x', 0, 'z', 128, 'i', 'g'})
	if err != nil || string(got) != "abcxxxz" {
		t.Fatalf("got %q, %v", got, err)
	}
	r := &Reader{}
	got, err = r.Decode(&Stream{Dict: Dict{{"Filter", Name("RunLengthDecode")}}, Raw: []byte{255, '-', 128}})
	if err != nil || string(got) != "--" {
		t.Fatalf("stream: %q, %v", got, err)
	}
}
