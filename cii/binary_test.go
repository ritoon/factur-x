package cii

import (
	"bytes"
	"encoding/base64"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/internal/xmltree"
)

// binaryRef is the former implementation of reader.binary: the decoded bytes,
// or the issue reported.
func binaryRef(text string) ([]byte, string) {
	s := strings.Map(func(c rune) rune {
		switch c {
		case ' ', '\t', '\n', '\r':
			return -1
		}
		return c
	}, text)
	if s == "" {
		return nil, ""
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		var err2 error
		if b, err2 = base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "=")); err2 != nil {
			return nil, "invalid base64 content: " + err.Error()
		}
	}
	return b, ""
}

func TestBinary(t *testing.T) {
	inputs := []string{
		"", " \n\t\r ", "aGVsbG8=", "aGVsbG8", "aGVs\nbG8=", " aG Vs\r\nbG8 = ", "aGVsbG8==", "aGVsbG8===",
		"aGVs*bG8=", "=", "aGVsbG8=aGVs", "a", "ab", "abc", "abcd", "\xffabc", "ab\x00cd", "YWJj ZA==",
	}
	rnd := rand.New(rand.NewPCG(1, 2))
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/= \t\r\n*"
	for range 2000 {
		b := make([]byte, rnd.IntN(24))
		for i := range b {
			b[i] = alphabet[rnd.IntN(len(alphabet))]
		}
		inputs = append(inputs, string(b))
	}
	for _, in := range inputs {
		want, wantIssue := binaryRef(in)
		r := &reader{}
		got := r.binary(&xmltree.Node{Name: "AttachmentBinaryObject", Text: in})
		var issue string
		if len(r.issues) > 0 {
			issue = r.issues[0].Message
		}
		if !bytes.Equal(got, want) || (got == nil) != (want == nil) || issue != wantIssue {
			t.Errorf("binary(%q) = %q, %q; want %q, %q", in, got, issue, want, wantIssue)
		}
	}
}
