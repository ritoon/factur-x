package pdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"testing"
)

// TestInflateLimit checks the protection against decompression bombs: the
// decoded size is bounded, and an announced size larger than what the input
// can produce is not allocated.
func TestInflateLimit(t *testing.T) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(make([]byte, 1<<20)) // 1 MiB of zeros compresses to about 1 KiB
	w.Close()
	src := buf.Bytes()

	old := maxDecoded
	maxDecoded = 64 << 10
	defer func() { maxDecoded = old }()
	if _, err := inflate(src, 0); !errors.Is(err, errTooLarge) {
		t.Fatalf("1 MiB bomb with a 64 KiB limit: err = %v, want errTooLarge", err)
	}
	maxDecoded = old

	out, err := inflate(src, 1<<40) // absurd /Size: must not be allocated
	if err != nil || len(out) != 1<<20 {
		t.Fatalf("inflate with absurd hint: %d bytes, %v", len(out), err)
	}
}
