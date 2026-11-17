package de

// gebaFormat returns "" when s has the format of a GEBA, or why it has not.
// The format is that of the GEBA Formatspezifikation 1.0.1 (KoSIT,
// 15.12.2025), chapter 2:
//
//   - the W-IdNr (Wirtschafts-Identifikationsnummer): DE and 9 digits, the
//     last one a check digit (see widnrCheckDigit);
//   - optionally '-' and the distinguishing feature (Unterscheidungsmerkmal)
//     that the tax administration assigns, 5 digits;
//   - optionally '.' and the sub-address (Unter-Adressierung) that the
//     business assigns, 1 to 8 letters a to z, A to Z and digits.
//
// The separators are the hyphen-minus U+002D and the full stop U+002E, which
// may only appear at the 12th position, or at the 18th for the full stop
// after a distinguishing feature. A GEBA is 11 to 26 characters long. The
// check digit is not verified.
func gebaFormat(s string) string {
	if len(s) < 11 || s[:2] != "DE" || !digits(s[2:11]) {
		return "it must start with DE and the 9 digits of the W-IdNr"
	}
	rest := s[11:]
	if rest != "" && rest[0] == '-' {
		if len(rest) < 6 || !digits(rest[1:6]) {
			return "the distinguishing feature (Unterscheidungsmerkmal) after '-' must have 5 digits"
		}
		rest = rest[6:]
	}
	if rest != "" && rest[0] == '.' {
		if sub := rest[1:]; len(sub) == 0 || len(sub) > 8 || !alnum(sub) {
			return "the sub-address (Unter-Adressierung) after '.' must have 1 to 8 letters or digits"
		}
		return ""
	}
	if rest != "" {
		return "the W-IdNr may only be followed by '-' and the 5 digits of a distinguishing feature, then by '.' and a sub-address of 1 to 8 letters or digits"
	}
	return ""
}

// widnrCheckDigit returns the check digit of a W-IdNr whose first 8 digits
// are d. The W-IdNr has the structure of the USt-IdNr (GEBA specification,
// chapter 2.1), whose check digit is computed with ISO 7064 MOD 11,10 (ZMDV,
// Anlage 3).
func widnrCheckDigit(d string) byte {
	p := 10
	for i := range 8 {
		s := (int(d[i]-'0') + p) % 10
		if s == 0 {
			s = 10
		}
		p = 2 * s % 11
	}
	c := 11 - p
	if c == 10 {
		c = 0
	}
	return byte('0' + c)
}

// ValidGEBA reports whether s is a GEBA: its format (see gebaFormat) and the
// check digit of its W-IdNr are valid. Surrounding white space is not
// accepted.
func ValidGEBA(s string) bool {
	return gebaFormat(s) == "" && widnrCheckDigit(s[2:10]) == s[10]
}
