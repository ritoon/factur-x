package pdf

import "strconv"

// WriteFile writes a complete PDF file, not an incremental update: the
// header %PDF-version followed by a comment of four bytes above 127, the
// objects numbered from 1 in the order of objs (objs[i] is object i+1, of
// generation 0), a classic cross-reference table and a trailer holding
// /Size and the entries of trailer (normally /Root, /Info and /ID).
//
// A *Stream is written with its raw data and a /Length that counts it. The
// layout follows the strict syntax of PDF/A (ISO 19005-2 and 19005-3, 6.1):
// single end-of-line markers around the obj, endobj, stream, endstream and
// xref keywords, and nothing after %%EOF but an end-of-line marker.
func WriteFile(version string, objs []Object, trailer Dict) []byte {
	size := 64
	for _, o := range objs {
		size += 48
		if st, ok := o.(*Stream); ok {
			size += len(st.Raw) + len(st.Dict)*16
		}
	}
	out := make([]byte, 0, size+len(objs)*20)
	out = append(out, "%PDF-"...)
	out = append(out, version...)
	out = append(out, "\n%\xe2\xe3\xcf\xd3\n"...)

	entries := make(map[uint32]entry, len(objs)+1)
	entries[0] = entry{typ: 0, gen: 65535}
	for i, o := range objs {
		num := uint32(i + 1)
		entries[num] = entry{typ: 1, off: int64(len(out))}
		out = strconv.AppendUint(out, uint64(num), 10)
		out = append(out, " 0 obj\n"...)
		if st, ok := o.(*Stream); ok {
			d := st.Dict.Clone()
			d.Set("Length", Int(len(st.Raw)))
			out = AppendObject(out, d)
			out = append(out, "\nstream\n"...)
			out = append(out, st.Raw...)
			out = append(out, "\nendstream"...)
		} else {
			out = AppendObject(out, o)
		}
		out = append(out, "\nendobj\n"...)
	}

	t := Dict{{"Size", Int(len(objs) + 1)}}
	for _, kv := range trailer {
		t.Set(kv.Key, kv.Val)
	}
	xoff := len(out)
	out = appendXrefTable(out, entries, t)
	out = append(out, "startxref\n"...)
	out = strconv.AppendInt(out, int64(xoff), 10)
	return append(out, "\n%%EOF\n"...)
}
