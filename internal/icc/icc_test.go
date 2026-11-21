package icc

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestSRGB(t *testing.T) {
	p := SRGB()
	be := binary.BigEndian
	if int(be.Uint32(p)) != len(p) || len(p)%4 != 0 {
		t.Fatalf("size %d, header says %d", len(p), be.Uint32(p))
	}
	if p[8] != 2 || p[9] != 0x10 || string(p[12:16]) != "mntr" || string(p[16:20]) != "RGB " ||
		string(p[20:24]) != "XYZ " || string(p[36:40]) != "acsp" {
		t.Fatalf("header %q", p[:40])
	}
	fixed := func(b []byte) float64 { return float64(int32(be.Uint32(b))) / 65536 }
	if x, y, z := fixed(p[68:]), fixed(p[72:]), fixed(p[76:]); math.Abs(x-0.9642) > 1e-4 || y != 1 || math.Abs(z-0.8249) > 1e-4 {
		t.Fatalf("illuminant %g %g %g", x, y, z)
	}

	n := int(be.Uint32(p[128:]))
	tags := map[string][]byte{}
	for i := range n {
		e := p[132+12*i:]
		sig, off, size := string(e[:4]), int(be.Uint32(e[4:])), int(be.Uint32(e[8:]))
		if off%4 != 0 || off < 132+12*n || off+size > len(p) {
			t.Fatalf("tag %s at %d, %d bytes", sig, off, size)
		}
		tags[sig] = p[off : off+size]
	}
	for sig, typ := range map[string]string{"desc": "desc", "cprt": "text", "wtpt": "XYZ ", "rXYZ": "XYZ ",
		"gXYZ": "XYZ ", "bXYZ": "XYZ ", "rTRC": "curv", "gTRC": "curv", "bTRC": "curv"} {
		if d := tags[sig]; len(d) < 12 || string(d[:4]) != typ {
			t.Fatalf("tag %s: %q", sig, d)
		}
	}
	if d := tags["desc"]; string(d[12:12+len(Description)]) != Description {
		t.Fatalf("description %q", d)
	}

	// The colourants add up to the white point.
	var sum [3]float64
	for _, sig := range []string{"rXYZ", "gXYZ", "bXYZ"} {
		for i := range sum {
			sum[i] += fixed(tags[sig][8+4*i:])
		}
	}
	for i, w := range whiteD50 {
		if math.Abs(sum[i]-w) > 5e-4 {
			t.Fatalf("colourants add up to %v, white point %v", sum, whiteD50)
		}
	}

	// The tone curve rises from 0 to 1 and matches sRGB at mid-grey.
	c := tags["rTRC"]
	if int(be.Uint32(c[8:])) != curvePoints || len(c) != 12+2*curvePoints {
		t.Fatalf("curve of %d points, %d bytes", be.Uint32(c[8:]), len(c))
	}
	prev := -1
	for i := range curvePoints {
		v := int(be.Uint16(c[12+2*i:]))
		if v < prev {
			t.Fatalf("curve decreases at point %d", i)
		}
		prev = v
	}
	if be.Uint16(c[12:]) != 0 || be.Uint16(c[len(c)-2:]) != 65535 {
		t.Fatal("curve does not span 0 to 1")
	}
	mid := float64(be.Uint16(c[12+2*511:])) / 65535 // signal 511/1023
	if want := math.Pow((511.0/1023+0.055)/1.055, 2.4); math.Abs(mid-want) > 1e-4 {
		t.Fatalf("curve at 0.5: %g, want %g", mid, want)
	}
}
