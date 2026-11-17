package en16931

import (
	"strconv"

	"github.com/ritoon/zerechnung/internal/rules"
)

// extendedRules is the subset of the CEN rules kept by the FeRD validation
// XSLT of Factur-X 1.09.2 for the EXTENDED profile, read from
// its rule identifiers. Other rules do not apply as such to
// EXTENDED invoices, which may carry data outside the EN 16931 data set.
var extendedRules = func() map[string]bool {
	m := make(map[string]bool, 160)
	add := func(prefix string, nums ...int) {
		for _, n := range nums {
			s := strconv.Itoa(n)
			if n < 10 {
				s = "0" + s
			}
			m[prefix+s] = true
		}
	}
	span := func(lo, hi int) []int {
		var out []int
		for i := lo; i <= hi; i++ {
			out = append(out, i)
		}
		return out
	}
	add("BR-", span(1, 21)...)
	add("BR-", 25, 29, 30, 31, 32, 33, 36, 37, 41, 42, 43)
	add("BR-", span(45, 53)...)
	add("BR-", 55, 56, 57, 61, 62, 63, 64, 65)
	add("BR-AE-", 2, 3, 4, 5, 6, 7, 9, 10)
	add("BR-B-", 1, 2)
	// BR-CO-05 and BR-CO-07 are also listed, but they assert true() and
	// are not implemented.
	add("BR-CO-", 3, 9, 14, 18, 19, 20, 21, 23, 26)
	add("BR-DEC-", 1, 2, 5, 6)
	add("BR-DEC-", span(9, 20)...)
	add("BR-DEC-", 23, 24, 25, 27, 28)
	add("BR-E-", 2, 3, 4, 5, 6, 7, 9, 10)
	add("BR-G-", 2, 3, 4, 5, 6, 7, 9, 10)
	add("BR-IC-", 2, 3, 4, 5, 6, 7, 9, 10, 11, 12)
	add("BR-O-", 5, 6, 7, 9, 10)
	add("BR-S-", 2, 3, 4, 5, 6, 7)
	add("BR-Z-", 2, 3, 4, 5, 6, 7, 9)
	m["CII-DT-097"] = true
	for _, n := range []int{463, 464, 465, 466, 467, 468, 469, 472, 473, 475, 476} {
		m["CII-SR-"+strconv.Itoa(n)] = true
	}
	return m
}()

// xrechnungLevels are the severities that the KoSIT validator configuration
// for XRechnung 3.0.2 (scenarios.xml, customLevel) gives to CEN rules, per
// scenario: XRechnung, XRechnung extension and XRechnung CVD.
var xrechnungLevels = func() map[mode]map[string]rules.Severity {
	base := map[string]rules.Severity{
		"BR-CL-21":   rules.Warning,
		"BR-CL-23":   rules.Warning,
		"CII-SR-452": rules.Error,
		"CII-SR-453": rules.Error,
		"CII-SR-454": rules.Error,
		"CII-SR-465": rules.Error,
		"CII-SR-466": rules.Error,
		"CII-SR-475": rules.Info,
		"CII-SR-476": rules.Info,
	}
	ext := make(map[string]rules.Severity, len(base)+5)
	cvd := make(map[string]rules.Severity, len(base)+1)
	for k, v := range base {
		ext[k], cvd[k] = v, v
	}
	for _, id := range []string{"BR-CL-21", "BR-CL-10", "BR-CL-11", "BR-CL-24", "BR-CL-25", "BR-CL-26"} {
		ext[id] = rules.Info
	}
	cvd["BR-CL-13"] = rules.Info
	return map[mode]map[string]rules.Severity{
		modeXRechnung:          base,
		modeXRechnungExtension: ext,
		modeXRechnungCVD:       cvd,
	}
}()

// xrechnungLevelsUBL are the severities that the same configuration gives to
// CEN rules for UBL documents, per scenario. In the XRechnung scenario of a
// CreditNote, UBL-CR-646 keeps its severity (see level).
var xrechnungLevelsUBL = map[mode]map[string]rules.Severity{
	modeXRechnung: {
		"BR-CL-21":   rules.Warning,
		"BR-CL-23":   rules.Warning,
		"UBL-CR-646": rules.Error,
	},
	modeXRechnungExtension: {
		"BR-CL-10":   rules.Info,
		"BR-CL-11":   rules.Info,
		"BR-CL-21":   rules.Info,
		"BR-CL-23":   rules.Warning,
		"BR-CL-24":   rules.Info,
		"BR-CL-25":   rules.Info,
		"BR-CL-26":   rules.Info,
		"BR-CO-16":   rules.Info,
		"UBL-CR-470": rules.Info,
		"UBL-CR-646": rules.Info,
	},
	modeXRechnungCVD: {
		"BR-CL-13":   rules.Info,
		"BR-CL-21":   rules.Warning,
		"BR-CL-23":   rules.Warning,
		"UBL-CR-646": rules.Error,
	},
}
