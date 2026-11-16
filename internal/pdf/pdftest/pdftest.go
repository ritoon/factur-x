// Package pdftest builds small PDF files for tests: classic cross-reference
// tables, compressed cross-reference streams with object streams, hybrid
// files and damaged files.
package pdftest

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strconv"
)

// Layout is the cross-reference layout of a generated file.
type Layout int

// Layouts.
const (
	// Classic writes a cross-reference table and a trailer dictionary.
	Classic Layout = iota
	// XrefStream writes a cross-reference stream (PDF 1.5).
	XrefStream
	// Hybrid writes a cross-reference table whose trailer has an /XRefStm
	// stream listing the objects stored in an object stream. The table
	// omits those objects (see Options.HybridFree).
	Hybrid
	// NoXref writes the objects and a trailer, without cross-reference data
	// nor startxref: readers must rebuild the index by scanning.
	NoXref
)

// Options control the output of Builder.Bytes.
type Options struct {
	Layout Layout
	// ObjStm stores the non-stream objects (except the trailer targets of
	// /Encrypt) in a compressed object stream. XrefStream and Hybrid only.
	ObjStm bool
	// Predictor encodes the cross-reference stream with the PNG Up
	// predictor (/DecodeParms << /Predictor 12 /Columns n >>).
	Predictor bool
	// Uncompressed leaves the cross-reference and object streams
	// unfiltered.
	Uncompressed bool
	// Shift is added to every offset of the cross-reference data, which
	// simulates a file whose offsets are wrong.
	Shift int
	// Garbage is inserted before the %PDF- header.
	Garbage string
	// HybridFree lists the objects of the object stream as free entries
	// of the table of a Hybrid file, instead of omitting them.
	HybridFree bool
}

type object struct {
	body   string
	stream []byte
	isStm  bool
}

// Builder accumulates objects, numbered from 1 in order of addition.
type Builder struct {
	// Version is the header version, "1.7" when empty.
	Version string
	// Trailer holds the trailer entries besides /Size, /Prev and /XRefStm,
	// for instance "/Root 1 0 R /ID [<01> <01>]".
	Trailer string
	objs    []object
}

// Add adds an object, written as body, and returns its number.
func (b *Builder) Add(body string) int {
	b.objs = append(b.objs, object{body: body})
	return len(b.objs)
}

// AddStream adds a stream object and returns its number. dict is the
// stream dictionary without /Length, which is added.
func (b *Builder) AddStream(dict string, data []byte) int {
	b.objs = append(b.objs, object{body: dict, stream: data, isStm: true})
	return len(b.objs)
}

// Reserve adds a placeholder object to be defined later with Set or
// SetStream, and returns its number.
func (b *Builder) Reserve() int { return b.Add("null") }

// Set replaces the body of object num.
func (b *Builder) Set(num int, body string) { b.objs[num-1] = object{body: body} }

// SetStream replaces object num with a stream.
func (b *Builder) SetStream(num int, dict string, data []byte) {
	b.objs[num-1] = object{body: dict, stream: data, isStm: true}
}

// Flate compresses data in zlib format.
func Flate(data []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

// streamDict adds /Length to a stream dictionary.
func streamDict(dict string, n int) string {
	d := bytes.TrimSpace([]byte(dict))
	if len(d) >= 4 && string(d[len(d)-2:]) == ">>" {
		return string(d[:len(d)-2]) + " /Length " + strconv.Itoa(n) + " >>"
	}
	return "<< /Length " + strconv.Itoa(n) + " >>"
}

// Bytes writes the file.
func (b *Builder) Bytes(o Options) []byte {
	var out bytes.Buffer
	out.WriteString(o.Garbage)
	base := out.Len()
	v := b.Version
	if v == "" {
		v = "1.7"
	}
	fmt.Fprintf(&out, "%%PDF-%s\n%%\xe2\xe3\xcf\xd3\n", v)

	n := len(b.objs)
	offsets := make([]int, n+3) // by object number, 0 when not written directly
	inStm := make([]int, n+1)   // index+1 in the object stream
	useStm := o.ObjStm && (o.Layout == XrefStream || o.Layout == Hybrid)

	write := func(num int, ob object) {
		offsets[num] = out.Len() - base
		fmt.Fprintf(&out, "%d 0 obj\n", num)
		if ob.isStm {
			out.WriteString(streamDict(ob.body, len(ob.stream)))
			out.WriteString("\nstream\n")
			out.Write(ob.stream)
			out.WriteString("\nendstream")
		} else {
			out.WriteString(ob.body)
		}
		out.WriteString("\nendobj\n")
	}

	var stmMembers []int
	for i, ob := range b.objs {
		num := i + 1
		if useStm && !ob.isStm {
			stmMembers = append(stmMembers, num)
			inStm[num] = len(stmMembers)
			continue
		}
		write(num, ob)
	}
	stmNum := 0
	if len(stmMembers) > 0 {
		stmNum = n + 1
		var head, body bytes.Buffer
		for _, num := range stmMembers {
			fmt.Fprintf(&head, "%d %d ", num, body.Len())
			body.WriteString(b.objs[num-1].body)
			body.WriteByte('\n')
		}
		data := append(head.Bytes(), body.Bytes()...)
		dict := fmt.Sprintf("<< /Type /ObjStm /N %d /First %d", len(stmMembers), head.Len())
		if !o.Uncompressed {
			data = Flate(data)
			dict += " /Filter /FlateDecode"
		}
		write(stmNum, object{body: dict + " >>", stream: data, isStm: true})
	}

	shift := func(off int) int {
		if off == 0 {
			return 0
		}
		return off + o.Shift
	}

	switch o.Layout {
	case NoXref:
		fmt.Fprintf(&out, "trailer\n<< /Size %d %s >>\n%%%%EOF\n", n+1, b.Trailer)
		return out.Bytes()

	case Classic, Hybrid:
		var xsOff int
		size := n + 1
		if stmNum != 0 {
			size = stmNum + 1
		}
		if o.Layout == Hybrid {
			// The /XRefStm stream lists every object; the table lists
			// the compressed ones as free.
			xsNum := size
			size++
			xsOff = out.Len() - base
			b.writeXrefStream(&out, o, xsNum, size, offsets, inStm, stmNum, xsOff, shift, "")
		}
		xref := out.Len() - base
		out.WriteString("xref\n")
		if o.Layout == Hybrid && !o.HybridFree {
			// One subsection per run of objects written directly.
			out.WriteString("0 1\n0000000000 65535 f\r\n")
			for num := 1; num < size; {
				if offsets[num] == 0 {
					num++
					continue
				}
				end := num
				for end < size && offsets[end] != 0 {
					end++
				}
				fmt.Fprintf(&out, "%d %d\n", num, end-num)
				for ; num < end; num++ {
					fmt.Fprintf(&out, "%010d 00000 n\r\n", shift(offsets[num]))
				}
			}
		} else {
			fmt.Fprintf(&out, "0 %d\n0000000000 65535 f\r\n", size)
			for num := 1; num < size; num++ {
				if off := offsets[num]; off != 0 {
					fmt.Fprintf(&out, "%010d 00000 n\r\n", shift(off))
				} else {
					out.WriteString("0000000000 65535 f\r\n")
				}
			}
		}
		fmt.Fprintf(&out, "trailer\n<< /Size %d %s", size, b.Trailer)
		if o.Layout == Hybrid {
			fmt.Fprintf(&out, " /XRefStm %d", shift(xsOff))
		}
		fmt.Fprintf(&out, " >>\nstartxref\n%d\n%%%%EOF\n", shift(xref))
		return out.Bytes()

	default: // XrefStream
		size := n + 1
		if stmNum != 0 {
			size = stmNum + 1
		}
		xsNum := size
		size++
		xref := out.Len() - base
		b.writeXrefStream(&out, o, xsNum, size, offsets, inStm, stmNum, xref, shift, b.Trailer)
		fmt.Fprintf(&out, "startxref\n%d\n%%%%EOF\n", shift(xref))
		return out.Bytes()
	}
}

// writeXrefStream writes a cross-reference stream object listing objects
// 0 to size-1, with W [1 4 2].
func (b *Builder) writeXrefStream(out *bytes.Buffer, o Options, xsNum, size int, offsets, inStm []int, stmNum, self int, shift func(int) int, trailer string) {
	const cols = 7
	rows := make([]byte, 0, size*cols)
	for num := 0; num < size; num++ {
		var row [cols]byte
		switch {
		case num == xsNum:
			row[0] = 1
			putBE(row[1:5], shift(self))
		case num < len(inStm) && inStm[num] > 0:
			row[0] = 2
			putBE(row[1:5], stmNum)
			putBE(row[5:7], inStm[num]-1)
		case num < len(offsets) && offsets[num] != 0:
			row[0] = 1
			putBE(row[1:5], shift(offsets[num]))
		default:
			row[0] = 0
			if num == 0 {
				row[5], row[6] = 0xff, 0xff
			}
		}
		rows = append(rows, row[:]...)
	}
	dict := fmt.Sprintf("<< /Type /XRef /Size %d /W [1 4 2] %s", size, trailer)
	data := rows
	if o.Predictor {
		// PNG Up: each row is prefixed with filter type 2 and holds the
		// difference with the previous row.
		enc := make([]byte, 0, size*(cols+1))
		prev := make([]byte, cols)
		for i := 0; i < len(rows); i += cols {
			enc = append(enc, 2)
			for j := 0; j < cols; j++ {
				enc = append(enc, rows[i+j]-prev[j])
			}
			copy(prev, rows[i:i+cols])
		}
		data = enc
		dict += fmt.Sprintf(" /DecodeParms << /Predictor 12 /Columns %d >>", cols)
	}
	if !o.Uncompressed {
		data = Flate(data)
		dict += " /Filter /FlateDecode"
	}
	fmt.Fprintf(out, "%d 0 obj\n%s\nstream\n", xsNum, streamDict(dict+" >>", len(data)))
	out.Write(data)
	out.WriteString("\nendstream\nendobj\n")
}

func putBE(b []byte, v int) {
	for i := len(b) - 1; i >= 0; i-- {
		b[i] = byte(v)
		v >>= 8
	}
}
