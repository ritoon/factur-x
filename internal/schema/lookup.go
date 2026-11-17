package schema

import "github.com/ritoon/zerechnung/invoice"

// Type is the type of an element in the schema of a profile, as reached from
// the root element. It lets the CII writer place the elements it keeps
// verbatim at their position in the schema, and the tests walk a schema. The
// zero Type is invalid: it declares no child and no attribute.
type Type struct {
	t     uint16 // index in types, plus one
	pos   uint16 // position of the element among the children of its parent
	max   uint8  // maxOccurs of the element (unbounded = 255)
	min   uint8  // minOccurs of the element
	name  string // local name of the element
	space string // namespace URI of the element
}

// Root returns the type of the root element (rsm:CrossIndustryInvoice) in the
// schema used for profile p (see Check for the mapping of the profiles).
func Root(p invoice.Profile) Type {
	r := &roots[profileIndex(p)]
	return Type{t: r.typ + 1, min: 1, max: 1, name: r.name, space: namespaces[r.ns]}
}

// Valid reports whether t is the type of an element of the schema.
func (t Type) Valid() bool { return t.t != 0 }

// Name returns the local name of the element.
func (t Type) Name() string { return t.name }

// Space returns the namespace URI of the element.
func (t Type) Space() string { return t.space }

// Pos returns the position of the element among the children declared by
// its parent type, from 0.
func (t Type) Pos() int { return int(t.pos) }

// Min returns the minimum number of occurrences of the element.
func (t Type) Min() int { return int(t.min) }

// Max returns the maximum number of occurrences of the element, or -1 when
// it is unbounded.
func (t Type) Max() int {
	if t.max == unbounded {
		return -1
	}
	return int(t.max)
}

// Simple reports whether the element holds a value rather than elements.
func (t Type) Simple() bool { return t.Valid() && types[t.t-1].content == contentSimple }

// TypeName returns the name of the type of the element, with its
// conventional prefix ("ram:TradePartyType", "udt:AmountType").
func (t Type) TypeName() string {
	if !t.Valid() {
		return ""
	}
	return types[t.t-1].name
}

// ValueKind returns the lexical space of the value of a simple element:
// "string", "decimal", "integer", "boolean", "base64" or "date"; "" for an
// element that holds elements.
func (t Type) ValueKind() string {
	if !t.Simple() {
		return ""
	}
	switch types[t.t-1].simple {
	case kDecimal:
		return "decimal"
	case kInteger:
		return "integer"
	case kBoolean:
		return "boolean"
	case kBase64:
		return "base64"
	case kDate:
		return "date"
	}
	return "string"
}

// Child returns the type of the child element name of t. The result is
// invalid when t declares no such child.
func (t Type) Child(name string) Type {
	if !t.Valid() {
		return Type{}
	}
	ct := &types[t.t-1]
	ps := particles[ct.first : ct.first+ct.n]
	for i := range ps {
		if ps[i].name == name {
			return child(&ps[i], i)
		}
	}
	return Type{}
}

// Children returns the child elements declared by t, in schema order.
func (t Type) Children() []Type {
	if !t.Valid() {
		return nil
	}
	ct := &types[t.t-1]
	ps := particles[ct.first : ct.first+ct.n]
	out := make([]Type, len(ps))
	for i := range ps {
		out[i] = child(&ps[i], i)
	}
	return out
}

func child(p *particle, i int) Type {
	return Type{t: p.typ + 1, pos: uint16(i), max: p.max, min: p.min, name: p.name, space: namespaces[p.ns]}
}

// Attrs returns the names of the attributes declared by t, and for each
// whether it is required.
func (t Type) Attrs() (names []string, required []bool) {
	if !t.Valid() {
		return nil, nil
	}
	ct := &types[t.t-1]
	for _, a := range attributes[ct.afirst : ct.afirst+ct.an] {
		names = append(names, a.name)
		required = append(required, a.required)
	}
	return names, required
}

// HasAttr reports whether t declares the attribute name.
func (t Type) HasAttr(name string) bool {
	if !t.Valid() {
		return false
	}
	ct := &types[t.t-1]
	for _, a := range attributes[ct.afirst : ct.afirst+ct.an] {
		if a.name == name {
			return true
		}
	}
	return false
}
