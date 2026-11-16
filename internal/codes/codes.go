// Package codes holds the code lists of EN 16931 and lookup functions for
// them.
//
// The lists are generated (codes_gen.go) from the CII validation artefacts of
// CEN TC 434, release Release, so that a value is accepted exactly when the
// corresponding BR-CL rule of the official schematron accepts it. Lookups are
// exact and case sensitive (VATEX excepted, see VATEX), allocate nothing and
// use a binary search on sorted tables.
//
// The functions compare the value as given: callers that follow the
// schematron semantics first apply normalize-space and reject values that
// still contain a space.
package codes

//go:generate go run ./gen -o codes_gen.go

// DocumentType reports whether s is an invoice or credit note type code of
// UNTDID 1001 accepted for BT-3 (BR-CL-01).
func DocumentType(s string) bool { return in(documentTypes, s) }

// Currency reports whether s is an ISO 4217 alpha-3 currency code (BR-CL-03,
// BR-CL-04, BR-CL-05).
func Currency(s string) bool { return in(currencies, s) }

// TaxPointDateCode reports whether s is a value added tax point date code of
// UNTDID 2005 as restricted by EN 16931 for BT-8: 5, 29 or 72 (BR-CL-06).
func TaxPointDateCode(s string) bool { return in(taxPointDateCodes, s) }

// ReferenceType reports whether s is a reference code qualifier of UNTDID
// 1153, used for the invoiced object identifier scheme BT-18 (BR-CL-07).
func ReferenceType(s string) bool { return in(referenceTypes, s) }

// TextSubject reports whether s is a text subject code qualifier of UNTDID
// 4451, used for the invoice note subject BT-21 (BR-CL-08).
func TextSubject(s string) bool { return in(textSubjects, s) }

// ICD reports whether s is an identifier scheme of ISO 6523 ICD (BR-CL-10,
// BR-CL-11, BR-CL-21, BR-CL-26).
func ICD(s string) bool { return in(icd, s) }

// ItemClassification reports whether s is an item type identification code of
// UNTDID 7143, used for the item classification scheme BT-158 (BR-CL-13).
func ItemClassification(s string) bool { return in(itemClassifications, s) }

// Country reports whether s is an ISO 3166-1 alpha-2 country code accepted by
// EN 16931, which includes 1A (Kosovo) and XI (Northern Ireland) (BR-CL-14,
// BR-CL-15).
func Country(s string) bool { return in(countries, s) }

// PaymentMeans reports whether s is a payment means code of UNTDID 4461
// (BR-CL-16).
func PaymentMeans(s string) bool { return in(paymentMeans, s) }

// VATCategory reports whether s is a VAT category code of the EN 16931 subset
// of UNTDID 5305: AE, B, E, G, K, L, M, O, S or Z (BR-CL-17, BR-CL-18).
func VATCategory(s string) bool { return in(vatCategories, s) }

// AllowanceReason reports whether s is an allowance reason code of UNTDID 5189
// (BR-CL-19).
func AllowanceReason(s string) bool { return in(allowanceReasons, s) }

// ChargeReason reports whether s is a charge reason code of UNTDID 7161
// (BR-CL-20).
func ChargeReason(s string) bool { return in(chargeReasons, s) }

// VATEX reports whether s is a VAT exemption reason code of the CEF VATEX
// list (BR-CL-22). As in the official rule, which upper-cases the value
// first, the comparison ignores ASCII case.
func VATEX(s string) bool { return inFold(vatex, s) }

// Unit reports whether s is a unit of measure code of UN/ECE Recommendation
// 20 or 21 (BR-CL-23).
func Unit(s string) bool { return in(units, s) }

// MIME reports whether s is a MIME code accepted for attached documents:
// application/pdf, image/png, image/jpeg, text/csv, and the Office Open XML
// and OpenDocument spreadsheet types (BR-CL-24).
func MIME(s string) bool { return in(mimeCodes, s) }

// EAS reports whether s is an electronic address scheme of the CEF EAS list
// (BR-CL-25).
func EAS(s string) bool { return in(eas, s) }

// VATPrefix reports whether s is a country prefix accepted at the start of a
// VAT identifier by BR-CO-09.
//
// The prefixes are those of the official rule. In release 1.3.17 the CII
// artefacts do not list EL, although the rule text allows Greece to use it.
func VATPrefix(s string) bool { return in(vatPrefixes, s) }

// in reports whether s is in the sorted list.
func in(list []string, s string) bool {
	lo, hi := 0, len(list)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if list[m] < s {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo < len(list) && list[lo] == s
}

// inFold reports whether s, upper-cased (ASCII only), is in the sorted list of
// upper-case codes.
func inFold(list []string, s string) bool {
	lo, hi := 0, len(list)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if cmpUpper(list[m], s) < 0 {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo < len(list) && cmpUpper(list[lo], s) == 0
}

// cmpUpper compares a with s upper-cased (ASCII letters only).
func cmpUpper(a, s string) int {
	n := min(len(a), len(s))
	for i := 0; i < n; i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		switch {
		case a[i] < c:
			return -1
		case a[i] > c:
			return 1
		}
	}
	switch {
	case len(a) < len(s):
		return -1
	case len(a) > len(s):
		return 1
	}
	return 0
}
