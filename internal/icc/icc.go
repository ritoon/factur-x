// Package icc builds the sRGB ICC profile that the PDF/A conversion embeds
// in its output intent. The profile is computed, not copied from a file: a
// version 2.1 monitor profile (matrix and tone curves) of the sRGB colour
// space of IEC 61966-2-1, adapted to the D50 illuminant of the profile
// connection space with the Bradford transform.
package icc

import (
	"encoding/binary"
	"math"
)

// Description is the profile description of the sRGB profile, also the
// output condition of the PDF/A output intent that holds it.
const Description = "sRGB IEC61966-2.1"

// curvePoints is the number of points of the tone curves: with the linear
// interpolation of colour engines, 1024 points follow the sRGB curve within
// 1/65535 of the signal.
const curvePoints = 1024

// D50 white point and the sRGB primaries adapted to D50 (Bradford), as
// tabulated by the ICC for its sRGB profiles.
var (
	whiteD50 = [3]float64{0.9642, 1.0, 0.8249}
	red      = [3]float64{0.4361, 0.2225, 0.0139}
	green    = [3]float64{0.3851, 0.7169, 0.0971}
	blue     = [3]float64{0.1431, 0.0606, 0.7141}
)

// SRGB returns the sRGB ICC profile.
func SRGB() []byte {
	type tag struct {
		sig  string
		data []byte
	}
	curve := curveTag()
	tags := []tag{
		{"desc", descTag(Description)},
		{"cprt", textTag("No copyright, use freely")},
		{"wtpt", xyzTag(whiteD50)},
		{"rXYZ", xyzTag(red)},
		{"gXYZ", xyzTag(green)},
		{"bXYZ", xyzTag(blue)},
		{"rTRC", curve},
		{"gTRC", curve},
		{"bTRC", curve},
	}

	// Header, tag table, then the tag data, each element on a four-byte
	// boundary. The three tone curves share one element.
	table := 4 + 12*len(tags)
	out := make([]byte, 128+table, 128+table+len(curve)+256)
	binary.BigEndian.PutUint32(out[128:], uint32(len(tags)))
	offsets := map[*byte]int{}
	for i, t := range tags {
		off, ok := offsets[&t.data[0]]
		if !ok {
			for len(out)%4 != 0 {
				out = append(out, 0)
			}
			off = len(out)
			offsets[&t.data[0]] = off
			out = append(out, t.data...)
		}
		e := out[128+4+12*i:]
		copy(e[0:4], t.sig)
		binary.BigEndian.PutUint32(e[4:], uint32(off))
		binary.BigEndian.PutUint32(e[8:], uint32(len(t.data)))
	}
	for len(out)%4 != 0 {
		out = append(out, 0)
	}

	h := out[:128]
	binary.BigEndian.PutUint32(h[0:], uint32(len(out)))
	binary.BigEndian.PutUint32(h[8:], 0x02100000) // version 2.1.0
	copy(h[12:16], "mntr")
	copy(h[16:20], "RGB ")
	copy(h[20:24], "XYZ ")
	for i, v := range []uint16{2026, 1, 1, 0, 0, 0} { // creation date
		binary.BigEndian.PutUint16(h[24+2*i:], v)
	}
	copy(h[36:40], "acsp")
	// Rendering intent 0 (perceptual), then the PCS illuminant.
	putXYZ(h[68:80], whiteD50)
	return out
}

// s15Fixed16 encodes v as a signed 15.16 fixed-point number.
func s15Fixed16(v float64) uint32 {
	return uint32(int32(math.Round(v * 65536)))
}

func putXYZ(b []byte, v [3]float64) {
	for i, c := range v {
		binary.BigEndian.PutUint32(b[4*i:], s15Fixed16(c))
	}
}

// xyzTag returns an XYZType element.
func xyzTag(v [3]float64) []byte {
	b := make([]byte, 20)
	copy(b, "XYZ ")
	putXYZ(b[8:], v)
	return b
}

// textTag returns a textType element.
func textTag(s string) []byte {
	b := make([]byte, 8, 8+len(s)+1)
	copy(b, "text")
	b = append(b, s...)
	return append(b, 0)
}

// descTag returns a textDescriptionType element (ICC.1:2001-04, 6.5.17):
// the ASCII description, and empty Unicode and Macintosh ones.
func descTag(s string) []byte {
	b := make([]byte, 12, 12+len(s)+1+78)
	copy(b, "desc")
	binary.BigEndian.PutUint32(b[8:], uint32(len(s)+1))
	b = append(b, s...)
	b = append(b, 0)
	// Unicode language code and count, ScriptCode code and count, then 67
	// bytes of Macintosh description.
	return append(b, make([]byte, 4+4+2+1+67)...)
}

// curveTag returns the sRGB tone curve as a curveType element.
func curveTag() []byte {
	b := make([]byte, 12+2*curvePoints)
	copy(b, "curv")
	binary.BigEndian.PutUint32(b[8:], curvePoints)
	for i := range curvePoints {
		v := float64(i) / (curvePoints - 1)
		var lin float64
		if v <= 0.04045 {
			lin = v / 12.92
		} else {
			lin = math.Pow((v+0.055)/1.055, 2.4)
		}
		binary.BigEndian.PutUint16(b[12+2*i:], uint16(math.Round(lin*65535)))
	}
	return b
}
