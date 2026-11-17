package de

import (
	"strings"

	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmltree"
)

// Code lists of common.sch, as space separated strings searched with
// contains(list, concat(' ', code, ' ')).
const (
	digaCodes = " XR01 XR02 XR03 "
	iso6523   = " " +
		"0002 0003 0004 0005 0006 0007 0008 0009 0010 0011 0012 0013 0014 0015 0016 0017 0018 0019 0020 " +
		"0021 0022 0023 0024 0025 0026 0027 0028 0029 0030 0031 0032 0033 0034 0035 0036 0037 0038 0039 " +
		"0040 0041 0042 0043 0044 0045 0046 0047 0048 0049 0050 0051 0052 0053 0054 0055 0056 0057 0058 " +
		"0059 0060 0061 0062 0063 0064 0065 0066 0067 0068 0069 0070 0071 0072 0073 0074 0075 0076 0077 " +
		"0078 0079 0080 0081 0082 0083 0084 0085 0086 0087 0088 0089 0090 0091 0093 0094 0095 0096 0097 " +
		"0098 0099 0100 0101 0102 0104 0105 0106 0107 0108 0109 0110 0111 0112 0113 0114 0115 0116 0117 " +
		"0118 0119 0120 0121 0122 0123 0124 0125 0126 0127 0128 0129 0130 0131 0132 0133 0134 0135 0136 " +
		"0137 0138 0139 0140 0141 0142 0143 0144 0145 0146 0147 0148 0149 0150 0151 0152 0153 0154 0155 " +
		"0156 0157 0158 0159 0160 0161 0162 0163 0164 0165 0166 0167 0168 0169 0170 0171 0172 0173 0174 " +
		"0175 0176 0177 0178 0179 0180 0183 0184 0185 0186 0187 0188 0189 0190 0191 0192 0193 0194 0195 " +
		"0196 0197 0198 0199 0200 0201 0202 0203 0204 0205 0206 0207 0208 0209 0210 0211 0212 0213 0214 " +
		"0215 0216 0217 0218 0219 0220 0221 0222 0223 0224 0225 0226 0227 0228 0229 0230 0231 0232 0233 " +
		"0234 0235 0236 0237 0238 0239 0240 0241 0242 0243 0244 0245 0246 0247 0248 "
	cefEAS = " " +
		"0002 0007 0009 0037 0060 0088 0096 0097 0106 0130 0135 0142 0147 0151 0154 0158 0170 0177 0183 " +
		"0184 0188 0190 0191 0192 0193 0194 0195 0196 0198 0199 0200 0201 0202 0203 0204 0205 0208 0209 " +
		"0210 0211 0212 0213 0215 0216 0217 0218 0219 0220 0221 0225 0230 0235 0240 0244 0242 0245 0246 " +
		"0248 9910 9913 9914 9915 9918 9919 9920 9922 9923 9924 9925 9926 9927 9928 9929 9930 9931 9932 " +
		"9933 9934 9935 9936 9937 9938 9939 9940 9941 9942 9943 9944 9945 9946 9947 9948 9949 9950 9951 " +
		"9952 9953 9957 9959 AN AQ AS AU EM "
	untdid7143 = " " +
		"AA AB AC AD AE AF AG AH AI AJ AK AL AM AN AO AP AQ AR AS AT AU AV AW AX AY AZ BA BB BC BD BE BF " +
		"BG BH BI BJ BK BL BM BN BO BP BQ BR BS BT BU BV BW BX BY BZ CC CG CL CR CV DR DW EC EF EMD EN " +
		"FS GB GN GMN GS HS IB IN IS IT IZ MA MF MN MP NB ON PD PL PO PPI PV QS RC RN RU RY SA SG SK SN " +
		"SRS SRT SRU SRV SRW SRX SRY SRZ SS SSA SSB SSC SSD SSE SSF SSG SSH SSI SSJ SSK SSL SSM SSN SSO " +
		"SSP SSQ SSR SSS SST SSU SSV SSW SSX SSY SSZ ST STA STB STC STD STE STF STG STH STI STJ STK STL " +
		"STM STN STO STP STQ STR STS STT STU STV STW STX STY STZ SUA SUB SUC SUD SUE SUF SUG SUH SUI SUJ " +
		"SUK SUL SUM TG TSN TSO TSP TSQ TSR TSS TST TSU UA UP VN VP VS VX ZZZ "

	iso6523Ext    = digaCodes + iso6523  // $ISO-6523-ICD-EXT-CODES
	cefEASExt     = digaCodes + cefEAS   // $CEF-EAS-EXT-CODES
	untdid7143CVD = " CVD " + untdid7143 // $UNTDID-7143-CVD-CODES
)

// inList implements not(contains(normalize-space(code), ' ')) and
// contains(list, concat(' ', normalize-space(code), ' ')). Because the lists
// are concatenations with a double space, the empty code passes.
func inList(list, code string) bool {
	c := norm(code)
	return !strings.Contains(c, " ") && strings.Contains(list, " "+c+" ")
}

// mimeCodes are the MIME codes allowed by BR-DEX-01.
var mimeCodes = []string{
	"application/pdf", "image/png", "image/jpeg", "text/csv",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.oasis.opendocument.spreadsheet",
	"application/xml",
}

// extension runs the rules of cii-extension-pattern, which apply when BT-24
// is ExtensionID.
func (k *checker) extension() {
	if !k.ext {
		return
	}
	// BR-DEX-15
	subLines := false
	k.root.Walk(func(n *xmltree.Node) bool {
		if n.Name == "ParentLineID" {
			subLines = true
		}
		return !subLines
	})
	if subLines {
		for doc := range all(k.root, pLines+"/AssociatedDocumentLineDocument") {
			k.fail("BR-DEX-15", rules.Warning, doc, "[BR-DEX-15] This CII file might use the concept of sub invoice lines (ram:ParentLineID); however XRechnung does not support this; line: %q.", sv(first(doc, "LineID")))
		}
	}
	// BR-DEX-04 to BR-DEX-08, BR-DEX-01. Each element is checked by the first
	// rule of the pattern whose context it matches.
	k.root.Walk(func(n *xmltree.Node) bool {
		switch n.Name {
		case "GlobalID":
			scheme, ok := n.LookupAttr("schemeID")
			if !ok {
				break
			}
			switch {
			case !hasAncestor(n, "SpecifiedTradeProduct") && !hasAncestor(n, "ShipToTradeParty"):
				if !inList(iso6523Ext, scheme) {
					k.fail("BR-DEX-04", rules.Error, n, "[BR-DEX-04] Any scheme identifier of a global identifier (ram:GlobalID) must be coded using the ISO 6523 ICD list, extended with XR01, XR02 and XR03; schemeID: %q.", scheme)
				}
			case parentIs(n, "SpecifiedTradeProduct"):
				if !inList(iso6523Ext, scheme) {
					k.fail("BR-DEX-06", rules.Error, n, "[BR-DEX-06] Any scheme identifier of an Item standard identifier (BT-157) must be coded using the ISO 6523 ICD list, extended with XR01, XR02 and XR03; schemeID: %q.", scheme)
				}
			case parentIs(n, "ShipToTradeParty") && parentIs(n.Parent, "ApplicableHeaderTradeDelivery"):
				if !inList(iso6523Ext, scheme) {
					k.fail("BR-DEX-08", rules.Error, n, "[BR-DEX-08] Any scheme identifier of a Deliver to location identifier (BT-71) must be coded using the ISO 6523 ICD list, extended with XR01, XR02 and XR03; schemeID: %q.", scheme)
				}
			}
		case "ID":
			if scheme, ok := n.LookupAttr("schemeID"); ok && !hasAncestor(n, "SpecifiedTaxRegistration") && !inList(iso6523Ext, scheme) {
				k.fail("BR-DEX-05", rules.Error, n, "[BR-DEX-05] Any scheme identifier of an identifier (ram:ID) must be coded using the ISO 6523 ICD list, extended with XR01, XR02 and XR03; schemeID: %q.", scheme)
			}
		case "URIID":
			if scheme, ok := n.LookupAttr("schemeID"); ok && parentIs(n, "URIUniversalCommunication") && !inList(cefEASExt, scheme) {
				k.fail("BR-DEX-07", rules.Error, n, "[BR-DEX-07] Any scheme identifier of an electronic address (BT-34, BT-49) must belong to the CEF EAS code list, extended with XR01, XR02 and XR03; schemeID: %q.", scheme)
			}
		case "AttachmentBinaryObject":
			if mime, ok := n.LookupAttr("mimeCode"); !ok || !in(mime, mimeCodes...) {
				k.fail("BR-DEX-01", rules.Error, n, "[BR-DEX-01] The Attached document (BT-125) uses a MIME code that is not allowed: %q; with the extension, application/xml is allowed in addition to application/pdf, image/png, image/jpeg, text/csv and the OOXML and ODF spreadsheet codes.", mime)
			}
		}
		return true
	})
}

// cvdVehicleCategories and cvaCodes are $CVD-VEHICLE-CATEGORY and $CVA-CODES.
var (
	cvdVehicleCategories = []string{"M1", "M2", "M3", "N1", "N2", "N3"}
	cvaCodes             = []string{"clean", "zero-emission", "other"}
)

// vehicles runs the rules of cii-cvd-pattern, which apply when BT-24 is
// CVDID.
func (k *checker) vehicles() {
	if !k.cvd {
		return
	}
	const class = "DesignatedProductClassification/ClassCode"
	const char = "ApplicableProductCharacteristic"
	for tx := range all(k.root, pTransaction) {
		found := false
		for p := range all(tx, "IncludedSupplyChainTradeLineItem/SpecifiedTradeProduct") {
			if anyAttrEq(p, class, "listID", "CVD") && anyEq(p, char+"/Description", "cva") {
				found = true
				break
			}
		}
		if !found {
			k.fail("BR-DE-CVD-03", rules.Error, tx, "[BR-DE-CVD-03] An invoice must contain at least one INVOICE LINE (BG-25) whose Item classification identifier (BT-158) has the scheme identifier 'CVD' and whose Item attribute name (BT-160) is 'cva'.")
		}
	}
	for p := range all(k.root, pLines+"/SpecifiedTradeProduct") {
		cvdClasses := 0
		for c := range all(p, class) {
			if a, ok := c.LookupAttr("listID"); ok && a == "CVD" {
				cvdClasses++
			}
		}
		cvaChars := 0
		for ch := range all(p, char) {
			if anyEq(ch, "Description", "cva") {
				cvaChars++
			}
		}
		if anyEq(p, char+"/Description", "cva") && cvdClasses != 1 {
			k.fail("BR-DE-CVD-06-b", rules.Error, p, "[BR-DE-CVD-06-b] If the Item attribute name (BT-160) 'cva' is given, the same invoice line must contain exactly one Item classification identifier (BT-158) with the scheme identifier 'CVD'; found: %d.", cvdClasses)
		}
		if cvdClasses > 0 && cvaChars != 1 {
			k.fail("BR-DE-CVD-06-a", rules.Error, p, "[BR-DE-CVD-06-a] If an Item classification identifier (BT-158) with the scheme identifier 'CVD' is given, the same invoice line must contain exactly one Item attribute name (BT-160) 'cva'; found: %d.", cvaChars)
		}
	}
	for c := range all(k.root, pLines+"/SpecifiedTradeProduct/"+class) {
		list := attr(c, "listID")
		if !inList(untdid7143CVD, list) {
			k.fail("BR-TMP-CVD-01", rules.Error, c, "[BR-TMP-CVD-01] The scheme identifier of the Item classification identifier (BT-158) must be taken from the code list UNTDID 7143; listID: %q.", list)
		}
		if norm(list) == "CVD" && !in(nsv(c), cvdVehicleCategories...) {
			k.fail("BR-DE-CVD-04", rules.Error, c, "[BR-DE-CVD-04] An Item classification identifier (BT-158) with the scheme identifier 'CVD' must contain one of the vehicle categories M1, M2, M3, N1, N2, N3; value: %q.", nsv(c))
		}
	}
	for ch := range all(k.root, pLines+"/SpecifiedTradeProduct/"+char) {
		if !anyEq(ch, "Description", "cva") {
			continue
		}
		if v := nsv(first(ch, "Value")); !in(v, cvaCodes...) {
			k.fail("BR-DE-CVD-05", rules.Error, ch, "[BR-DE-CVD-05] If the Item attribute name (BT-160) is 'cva', the Item attribute value (BT-161) must be one of clean, zero-emission, other; value: %q.", v)
		}
	}
	for agr := range all(k.root, pAgreement) {
		if !nonBlank(agr, "ContractReferencedDocument/IssuerAssignedID") {
			k.fail("BR-DE-CVD-01", rules.Error, agr, "[BR-DE-CVD-01] The element Contract reference (BT-12) must be transmitted.")
		}
		tender := false
		for doc := range all(agr, "AdditionalReferencedDocument") {
			if nsv(first(doc, "TypeCode")) == "50" && nsv(first(doc, "IssuerAssignedID")) != "" {
				tender = true
				break
			}
		}
		if !tender {
			k.fail("BR-DE-CVD-02", rules.Error, agr, "[BR-DE-CVD-02] The element Tender or lot reference (BT-17) must be transmitted.")
		}
	}
}
