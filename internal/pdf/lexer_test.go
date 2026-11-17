package pdf

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestSmallInts(t *testing.T) {
	for i, o := range smallInts {
		if o != Object(Int(i)) {
			t.Fatalf("smallInts[%d] = %#v", i, o)
		}
	}
	for _, n := range []int64{-1, 0, 255, 256, smallIntLimit - 1, smallIntLimit, 1 << 40} {
		if o := intObject(n); o != Object(Int(n)) {
			t.Errorf("intObject(%d) = %#v", n, o)
		}
	}
}

// TestNumbers checks the integers read ahead when looking for "gen R".
func TestNumbers(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want Array
	}{
		{"[1 2 3 0 R 4]", Array{Int(1), Int(2), Ref{Num: 3}, Int(4)}},
		{"[506 506 70000 0 R 2.5 -3 4 5]", Array{Int(506), Int(506), Ref{Num: 70000}, Real(2.5), Int(-3), Int(4), Int(5)}},
		{"[1 2 3]", Array{Int(1), Int(2), Int(3)}},
		{"[1 99999 R 2]", Array{Int(1), Int(99999), nil, Int(2)}}, // generation out of range
		{"[7 0 Rx 8]", Array{Int(7), Int(0), nil, Int(8)}},
		{"[1 %c\n 0 R]", Array{Ref{Num: 1}}},
		{"[12abc 3 4]", Array{Int(12), Int(3), Int(4)}},
	} {
		p := parser{b: []byte(tc.src)}
		got, err := p.object()
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s = %#v, %v; want %#v", tc.src, got, err, tc.want)
		}
	}
}

// TestArrayCapacity checks that arrays of numbers, names and references are
// read into an array of their exact size.
func TestArrayCapacity(t *testing.T) {
	widths := "[" + strings.Repeat("506 226 1038\n", 100) + "]"
	for _, src := range []string{
		widths,
		"[0 0 595 842]",
		"[11 0 R 13 0 R]",
		"[/ICCBased 20 0 R]",
		"[1 0 R /XYZ 0 792 null]",
		"[true false]",
		"[]",
	} {
		p := parser{b: []byte(src)}
		o, err := p.object()
		arr, ok := o.(Array)
		if err != nil || !ok {
			t.Fatalf("%.20s: %#v, %v", src, o, err)
		}
		if cap(arr) != len(arr) {
			t.Errorf("%.20s: %d elements, capacity %d", src, len(arr), cap(arr))
		}
	}
}

func TestDictScratch(t *testing.T) {
	src := "<< /A << /B [1 << /C 2 /D (x) >>] /E 3 >> /F 4 /F 5 /G null /H 6 >>"
	p := parser{b: []byte(src)}
	o, err := p.object()
	if err != nil {
		t.Fatal(err)
	}
	want := Dict{
		{"A", Dict{{"B", Array{Int(1), Dict{{"C", Int(2)}, {"D", String("x")}}}}, {"E", Int(3)}}},
		{"F", Int(4)},
		{"H", Int(6)},
	}
	if !reflect.DeepEqual(o, want) {
		t.Fatalf("got %#v", o)
	}
	if d := o.(Dict); cap(d) != len(d) {
		t.Errorf("capacity %d for %d entries", cap(d), len(d))
	}
	if len(*p.kvs) != 0 {
		t.Errorf("scratch space not emptied: %d entries", len(*p.kvs))
	}
}

// skipCases are inputs that skipArray must read as array does.
var skipCases = []string{
	"1 2 0 R]",
	"506 506 226 325]",
	"(a]b) <414243> 3]",
	"<< /A [1] /B (c) >> 4]",
	"[1 [2 [3]]] 5]",
	"1 endobj",
	"1 2 obj 3",
	"1 > 2]",
	"12endobj 3]",
	"%comment ]\n 4]",
	"{ } ) 5]",
	"/Name/Other#20x 6]",
	"+ - . -.5 1.2.3 7]",
	"true false null x 8]",
	"1 0 R2 0 R]",
	"(unterminated",
	"<<",
	"",
	strings.Repeat("[", 300),
}

func FuzzSkipArray(f *testing.F) {
	for _, s := range skipCases {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		src := append([]byte("["), data...)
		full := parser{b: src}
		_, errFull := full.array()
		skip := parser{b: src}
		errSkip := skip.skipArray()
		if (errFull == errTooDeep) != (errSkip == errTooDeep) {
			t.Fatalf("%q: array error %v, skipArray error %v", src, errFull, errSkip)
		}
		if errFull == nil && full.pos != skip.pos {
			t.Fatalf("%q: array stops at %d, skipArray at %d", src, full.pos, skip.pos)
		}
	})
}

// FuzzSelection checks that a dictionary read without its arrays has the
// entries of the whole dictionary but its arrays, those of the kept key K
// excepted.
func FuzzSelection(f *testing.F) {
	for _, s := range []string{
		"/Widths [1 2 3] /K [4 5] /FontDescriptor 6 0 R",
		"/W [1 2] /W 5 /K 1",
		"/W 5 /W [1 2]",
		"/W null /W [1] /X [2]",
		"/A << /B [1 2] >> /C [/D (e]f)] /K [[1]]",
		"/A [1 endobj /B 2",
		"/A [1 > /B [2]",
		"/A [" + strings.Repeat("[", 300) + " /B 1",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		src := []byte("<<" + string(data) + ">>")
		full := parser{b: src}
		o, errFull := full.object()
		part := parser{b: src, sel: &selection{keep: []Name{"K"}}}
		po, errPart := part.object()
		if (errFull == nil) != (errPart == nil) || errFull == nil && full.pos != part.pos {
			t.Fatalf("%q: whole read %v at %d, partial read %v at %d", src, errFull, full.pos, errPart, part.pos)
		}
		d, ok := o.(Dict)
		if !ok {
			return
		}
		var want Dict
		for _, kv := range d {
			if _, isArray := kv.Val.(Array); !isArray || kv.Key == "K" {
				want = append(want, kv)
			}
		}
		got, _ := po.(Dict)
		if fmt.Sprint(got) != fmt.Sprint(want) || len(got) != len(want) {
			t.Fatalf("%q:\npartial %v\nwant    %v", src, got, want)
		}
		for i := range got {
			if got[i].Key != want[i].Key || !reflect.DeepEqual(got[i].Val, want[i].Val) {
				t.Fatalf("%q: entry %d is %v, want %v", src, i, got[i], want[i])
			}
		}
	})
}

func TestSkipArrayCases(t *testing.T) {
	// The fuzz seeds run with go test; this names the keywords array stops
	// at, which skipArray must leave unread.
	for _, kw := range []string{"endobj", "stream", "endstream", "obj"} {
		src := []byte("[1 2 " + kw + " 3]")
		p := parser{b: src}
		if err := p.skipArray(); err != nil || !slices.Equal(src[p.pos:], []byte(kw+" 3]")) {
			t.Errorf("%s: stopped at %q, %v", src, src[p.pos:], err)
		}
	}
}
