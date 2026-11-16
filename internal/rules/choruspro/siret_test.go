package choruspro

import "testing"

func TestValidSIRET(t *testing.T) {
	for _, c := range []struct {
		s    string
		want bool
	}{
		{"12002701600357", true},  // INSEE, Luhn key valid
		{"12002701600358", false}, // last digit changed
		{"21002701600357", false}, // two digits swapped
		{"35600000049837", true},  // La Poste: digit sum 45
		{"35600000049838", false}, // La Poste: digit sum 46
		{"35600000000048", false}, // La Poste: digit sum 26, although the Luhn key is checked otherwise
		{"1200270160035", false},  // 13 digits
		{"120027016003570", false},
		{"1200270160035A", false},
		{"120 027 016 00357", false},
		{"１２００２７０１６００３５７", false}, // full-width digits
		{"", false},
		{"00000000000000", true}, // passes the key; existence is not checked
	} {
		if got := ValidSIRET(c.s); got != c.want {
			t.Errorf("ValidSIRET(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestValidSIREN(t *testing.T) {
	for _, c := range []struct {
		s    string
		want bool
	}{
		{"120027016", true},
		{"120027017", false},
		{"356000000", true},
		{"12002701", false},
		{"12002701A", false},
	} {
		if got := ValidSIREN(c.s); got != c.want {
			t.Errorf("ValidSIREN(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}
