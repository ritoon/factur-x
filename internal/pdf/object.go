// Package pdf is a small, lazy PDF reader and an incremental-update writer,
// sized for what Factur-X needs: reaching the catalog, the embedded files,
// the XMP metadata and the output intents of a PDF, and appending new
// objects without touching the original bytes.
//
// The reader parses the cross-reference data and only the objects it is
// asked for. It supports classic xref tables, cross-reference streams (with
// PNG predictors), hybrid files (/XRefStm), /Prev chains and object streams.
// When the cross-reference data is missing, damaged or shifted, it rebuilds
// an index by scanning the file for "n g obj" headers.
package pdf

import (
	"strconv"
)

// Object is a PDF object. Its dynamic type is one of nil (the null object),
// Bool, Int, Real, Name, String, Array, Dict, Ref or *Stream.
type Object any

// Bool is a PDF boolean.
type Bool bool

// Int is a PDF integer.
type Int int64

// Real is a PDF real number.
type Real float64

// Name is a PDF name, without the leading slash and with #xx escapes decoded.
type Name string

// String is a PDF string, with escapes decoded. It may alias the input data
// and must not be modified.
type String []byte

// Array is a PDF array.
type Array []Object

// Ref is an indirect reference.
type Ref struct {
	Num uint32
	Gen uint16
}

// KV is a dictionary entry.
type KV struct {
	Key Name
	Val Object
}

// Dict is a PDF dictionary. Entries keep the order of the source.
type Dict []KV

// Stream is a PDF stream: its dictionary and its raw (still encoded) data.
type Stream struct {
	Dict Dict
	Raw  []byte // encoded data; aliases the input and must not be modified
}

// String returns "n g R".
func (r Ref) String() string {
	return strconv.FormatUint(uint64(r.Num), 10) + " " + strconv.FormatUint(uint64(r.Gen), 10) + " R"
}

// Get returns the value of key, or nil when it is absent.
func (d Dict) Get(key Name) Object {
	for i := range d {
		if d[i].Key == key {
			return d[i].Val
		}
	}
	return nil
}

// Has reports whether key is present.
func (d Dict) Has(key Name) bool {
	for i := range d {
		if d[i].Key == key {
			return true
		}
	}
	return false
}

// Set sets key to val, replacing an existing entry or appending a new one.
// A nil val removes the entry.
func (d *Dict) Set(key Name, val Object) {
	for i := range *d {
		if (*d)[i].Key == key {
			if val == nil {
				*d = append((*d)[:i], (*d)[i+1:]...)
			} else {
				(*d)[i].Val = val
			}
			return
		}
	}
	if val != nil {
		*d = append(*d, KV{key, val})
	}
}

// Clone returns a shallow copy of d.
func (d Dict) Clone() Dict {
	out := make(Dict, len(d), len(d)+4)
	copy(out, d)
	return out
}

// Name returns the value of key when it is a name.
func (d Dict) Name(key Name) (Name, bool) {
	n, ok := d.Get(key).(Name)
	return n, ok
}

// Int returns the value of key when it is an integer (or an integral real).
func (d Dict) Int(key Name) (int64, bool) {
	return AsInt(d.Get(key))
}

// AsInt returns o as an integer when it is an Int or an integral Real.
func AsInt(o Object) (int64, bool) {
	switch v := o.(type) {
	case Int:
		return int64(v), true
	case Real:
		if float64(int64(v)) == float64(v) {
			return int64(v), true
		}
	}
	return 0, false
}

// AsDict returns o as a dictionary; a stream yields its dictionary.
func AsDict(o Object) (Dict, bool) {
	switch v := o.(type) {
	case Dict:
		return v, true
	case *Stream:
		return v.Dict, true
	}
	return nil, false
}
