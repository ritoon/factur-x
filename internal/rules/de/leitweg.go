package de

import (
	"fmt"
	"strings"
)

// leitwegFormat returns "" when s has the format of a Leitweg-ID, or why it
// has not. The format is that of the Formatspezifikation Leitweg-ID 2.0.2
// (KoSIT, 28.07.2021), chapter 2:
//
//   - the coarse addressing (Grobadressierung), 2, 3, 5, 8, 9 or 12 digits:
//     the code of the Land (01 to 16) or of the Federation (99), then
//     optionally the district (1 digit), the county (2 digits, only after the
//     district) and the municipality (3, 4 or 7 digits, only after the
//     county), as in table 1;
//   - optionally '-' and the fine addressing (Feinadressierung), 1 to 30
//     letters A to Z, in either case, and digits;
//   - '-' and two check digits (Prüfziffer).
//
// The separator is the hyphen-minus U+002D. The format makes a Leitweg-ID 5
// to 46 characters long. The check digits are not verified (see
// leitwegCheckDigits).
func leitwegFormat(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) < 2 || len(parts) > 3 {
		return "it must have two or three parts separated by '-': the coarse addressing, an optional fine addressing and the check digits"
	}
	coarse, key := parts[0], parts[len(parts)-1]
	if n := len(coarse); n != 2 && n != 3 && n != 5 && n != 8 && n != 9 && n != 12 || !digits(coarse) {
		return "the coarse addressing (Grobadressierung) must have 2, 3, 5, 8, 9 or 12 digits"
	}
	if !landCode(coarse[:2]) {
		return "the coarse addressing (Grobadressierung) must start with the code of a Land (01 to 16) or of the Federation (99)"
	}
	if len(parts) == 3 {
		if fine := parts[1]; len(fine) == 0 || len(fine) > 30 || !alnum(fine) {
			return "the fine addressing (Feinadressierung) must have 1 to 30 letters A to Z or digits"
		}
	}
	if len(key) != 2 || !digits(key) {
		return "the check digits (Prüfziffer) must be two digits"
	}
	return ""
}

// leitwegCheckDigits returns the check digits of a Leitweg-ID whose coarse
// and fine addressing, without the hyphens, are addr: ISO/IEC 7064 MOD 97-10
// as in chapter 2.4 of the specification, 98 minus the remainder of the
// division by 97 of addr followed by "00", letters A to Z (in either case)
// counting as 10 to 35. A Leitweg-ID is valid when its check digits are those
// of its addressing; the whole number, check digits included, then leaves a
// remainder of 1.
func leitwegCheckDigits(addr string) string {
	return fmt.Sprintf("%02d", 98-mod97(addr+"00"))
}

// ValidLeitwegID reports whether s is a Leitweg-ID: its format (see
// leitwegFormat) and its check digits are valid. Surrounding white space is
// not accepted.
func ValidLeitwegID(s string) bool {
	if leitwegFormat(s) != "" {
		return false
	}
	addr, key := leitwegParts(s)
	return leitwegCheckDigits(addr) == key
}

// leitwegParts returns the coarse and fine addressing of a Leitweg-ID in the
// right format, without the hyphens, and its check digits.
func leitwegParts(s string) (addr, key string) {
	parts := strings.Split(s, "-")
	for _, p := range parts[:len(parts)-1] {
		addr += p
	}
	return addr, parts[len(parts)-1]
}

// mod97 returns the remainder of the division by 97 of the number written
// by s, a letter A to Z, in either case, standing for the two digits 10 to
// 35. Other characters are ignored.
func mod97(s string) int {
	r := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			r = (r*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			r = (r*100 + int(c-'A') + 10) % 97
		case c >= 'a' && c <= 'z':
			r = (r*100 + int(c-'a') + 10) % 97
		}
	}
	return r
}

// landCode reports whether s is the code of a Land (01 to 16) or of the
// Federation (99), chapter 2.2.1 of the specification.
func landCode(s string) bool {
	return s == "99" || (s >= "01" && s <= "16" && len(s) == 2)
}

// digits reports whether s is a non empty string of ASCII digits.
func digits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// alnum reports whether s is a non empty string of ASCII letters and digits.
func alnum(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}
