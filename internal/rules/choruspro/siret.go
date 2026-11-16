package choruspro

// sirenLaPoste is the SIREN of La Poste, whose establishments are numbered
// outside the Luhn key: their SIRET is valid when the sum of its digits is a
// multiple of 5.
const sirenLaPoste = "356000000"

// ValidSIRET reports whether s is a SIRET: 14 ASCII digits whose Luhn key is
// valid, or, for an establishment of La Poste, whose digit sum is a multiple
// of 5. Spaces are not accepted.
func ValidSIRET(s string) bool {
	if len(s) != 14 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if s[:9] == sirenLaPoste {
		sum := 0
		for i := 0; i < len(s); i++ {
			sum += int(s[i] - '0')
		}
		return sum%5 == 0
	}
	return luhn(s)
}

// ValidSIREN reports whether s is a SIREN: 9 ASCII digits with a valid Luhn
// key.
func ValidSIREN(s string) bool {
	if len(s) != 9 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return luhn(s)
}

// luhn reports whether the decimal digits of s pass the Luhn check: from the
// rightmost digit, every second digit is doubled (minus 9 above 9) and the
// sum is a multiple of 10.
func luhn(s string) bool {
	sum := 0
	double := false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
