package zerechnung

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ritoon/zerechnung/internal/pdf"
	"github.com/ritoon/zerechnung/internal/rules"
	"github.com/ritoon/zerechnung/internal/xmp"
	"github.com/ritoon/zerechnung/invoice"
	"github.com/ritoon/zerechnung/validate"
)

// ContainerCatalog lists the checks of CheckContainer. They cover what the
// Factur-X specification (and ZUGFeRD, XRechnung hybrids) requires from the
// PDF container — the invoice attachment, its file specification and the
// XMP metadata — and the PDF/A-3 requirements a reader can verify without
// parsing the page content: encryption, output intent, file identifier,
// embedded fonts, JavaScript and actions, /Info and XMP consistency, other
// embedded files and forbidden filters. They do not replace a PDF/A
// validator: colour spaces, transparency, the font programs themselves and
// the page content are not checked.
var ContainerCatalog = []rules.RuleInfo{
	{ID: "PDF-01", Severity: rules.Error, Description: "The PDF is not encrypted (PDF/A forbids encryption; the attachment cannot be read)."},
	{ID: "PDF-02", Severity: rules.Error, Description: "The document catalog has an XMP metadata stream (/Metadata)."},
	{ID: "PDF-03", Severity: rules.Error, Description: "The XMP metadata is a readable RDF/XML packet."},
	{ID: "PDF-04", Severity: rules.Error, Description: "The XMP metadata declares PDF/A-3 or PDF/A-4 (pdfaid:part is 3 or 4), the parts that allow embedded XML files."},
	{ID: "PDF-05", Severity: rules.Error, Description: "The XMP metadata declares a PDF/A conformance level that allows embedded XML files (pdfaid:conformance is A, B or U for PDF/A-3, F or E for PDF/A-4)."},
	{ID: "PDF-06", Severity: rules.Warning, Description: "The catalog has a PDF/A output intent (/OutputIntents with /S /GTS_PDFA1). PDF/A requires it as soon as a device-dependent colour space is used, which is not analysed here."},
	{ID: "PDF-07", Severity: rules.Error, Description: "The PDF/A output intent has an ICC profile (/DestOutputProfile stream)."},
	{ID: "PDF-08", Severity: rules.Error, Description: "The trailer has a file identifier (/ID), as PDF/A requires."},
	{ID: "PDF-09", Severity: rules.Error, Description: "The PDF has an invoice XML attachment that can be decoded."},
	{ID: "PDF-10", Severity: rules.Error, Description: "The invoice attachment has an invoice file name: factur-x.xml, xrechnung.xml, or the legacy ZUGFeRD names zugferd-invoice.xml and ZUGFeRD-invoice.xml."},
	{ID: "PDF-11", Severity: rules.Warning, Description: "The invoice attachment has the name of its profile in current specifications: factur-x.xml, or xrechnung.xml for XRECHNUNG. Legacy ZUGFeRD names are reported here."},
	{ID: "PDF-12", Severity: rules.Error, Description: "The file specification of the invoice has an /AFRelationship entry (PDF/A-3 associated file)."},
	{ID: "PDF-13", Severity: rules.Error, Description: "The /AFRelationship of the invoice is Data, Alternative or Source."},
	{ID: "PDF-14", Severity: rules.Warning, Description: "The /AFRelationship matches the profile: Data for MINIMUM and BASIC WL, Alternative or Source for BASIC and above. Data is not reported for BASIC and above when the French rules apply: France admits it (Factur-X 1.09, 6.2.2)."},
	{ID: "PDF-15", Severity: rules.Error, Description: "The file specification of the invoice is listed in the /AF array of the document catalog."},
	{ID: "PDF-16", Severity: rules.Error, Description: "The embedded file stream has an XML MIME type (/Subtype /text#2Fxml)."},
	{ID: "PDF-17", Severity: rules.Warning, Description: "The MIME type of the embedded file is text/xml rather than application/xml."},
	{ID: "PDF-18", Severity: rules.Error, Description: "The embedded file stream has a modification date (/Params /ModDate), as PDF/A-3 requires."},
	{ID: "PDF-19", Severity: rules.Warning, Description: "The modification date of the embedded file is a valid PDF date (D:YYYYMMDDHHmmSSOHH'mm')."},
	{ID: "PDF-20", Severity: rules.Error, Description: "The file specification of the invoice has both a /F and a /UF file name, as PDF/A-3 requires."},
	{ID: "PDF-21", Severity: rules.Error, Description: "The XMP metadata has the hybrid-invoice schema (Factur-X, ZUGFeRD 2.0 or ZUGFeRD 1.0 namespace)."},
	{ID: "PDF-22", Severity: rules.Error, Description: "The XMP DocumentType is INVOICE."},
	{ID: "PDF-23", Severity: rules.Error, Description: "The XMP DocumentFileName is the name of the invoice attachment."},
	{ID: "PDF-24", Severity: rules.Error, Description: "The XMP ConformanceLevel is a known level and matches the profile of the XML (BT-24). EN 16931 is accepted for an XRECHNUNG invoice, EXTENDED for EXTENDED-CTC-FR."},
	{ID: "PDF-25", Severity: rules.Error, Description: "The XMP metadata describes the hybrid-invoice schema and its properties in a PDF/A extension schema (pdfaExtension:schemas)."},
	{ID: "PDF-26", Severity: rules.Warning, Description: "The XMP Version of the hybrid-invoice schema is set (\"1.0\" for Factur-X)."},
	{ID: "PDF-27", Severity: rules.Warning, Description: "The XMP namespace matches the XML syntax: Factur-X or ZUGFeRD 2.0 for CrossIndustryInvoice, ZUGFeRD 1.0 for CrossIndustryDocument."},
	{ID: "PDF-28", Severity: rules.Error, Description: "The PDF has a single invoice attachment: no other embedded file is named factur-x.xml, xrechnung.xml or zugferd-invoice.xml."},
	{ID: "PDF-29", Severity: rules.Warning, Description: "A PDF/A-4 declaration gives the edition of ISO 19005-4 it follows (pdfaid:rev, a four-digit year: 2020)."},
	{ID: "PDF-30", Severity: rules.Warning, Description: "The PDF version matches the declared PDF/A part: PDF 1.7 or earlier for PDF/A-3, PDF 2.0 for PDF/A-4."},
	{ID: "PDF-31", Severity: rules.Warning, Description: "The fonts of the pages (in the resources of the pages, form XObjects, patterns, Type 3 fonts and annotation appearances) embed their font program (/FontFile, /FontFile2 or /FontFile3 in the font descriptor). PDF/A requires it for the fonts used for rendering; whether the content draws visible text with them is not analysed."},
	{ID: "PDF-32", Severity: rules.Error, Description: "The document has no document-level JavaScript (/JavaScript in the catalog /Names), which PDF/A forbids."},
	{ID: "PDF-33", Severity: rules.Error, Description: "No action forbidden by PDF/A (Launch, Sound, Movie, ResetForm, ImportData, Hide, SetOCGState, Rendition, Trans, GoTo3DView, JavaScript, SetState, NOP, or a named action other than NextPage, PrevPage, FirstPage and LastPage) in the /OpenAction, the outlines, the annotations or a chain of /Next actions."},
	{ID: "PDF-34", Severity: rules.Error, Description: "No additional-actions dictionary (/AA) in the document catalog, the pages, the annotations or the form fields, as PDF/A requires."},
	{ID: "PDF-35", Severity: rules.Error, Description: "The entries of the document information dictionary (/Info) that have an XMP equivalent (Title, Author, Subject, Keywords, Creator, Producer, CreationDate, ModDate) have the same value in the XMP metadata, as ISO 19005-3 requires. Not checked for PDF/A-4."},
	{ID: "PDF-36", Severity: rules.Error, Description: "Every embedded file besides the invoice has an /AFRelationship entry (PDF/A-3 associated file)."},
	{ID: "PDF-37", Severity: rules.Error, Description: "Every embedded file besides the invoice has a MIME type (/Subtype of the embedded file stream), as PDF/A-3 requires."},
	{ID: "PDF-38", Severity: rules.Error, Description: "No stream of the pages, their resources, the annotation appearances, the font programs, the embedded files or the metadata uses the LZWDecode filter, which PDF/A forbids. Inline images of the page content are not checked."},
	{ID: "PDF-39", Severity: rules.Error, Description: "Every embedded file besides the invoice is an associated file, listed in an /AF array (of the document catalog, a page, an annotation, a form or image XObject, a marked-content property list of the resources or a structure element), as PDF/A-3 requires. Files associated otherwise (property lists inside the page content, document parts) are not found."},
	{ID: "PDF-40", Severity: rules.Error, Description: "Where the German rules apply, the /AFRelationship of a BASIC, EN 16931, EXTENDED or XRECHNUNG invoice is Alternative: Germany admits no other value (Factur-X 1.09, 5.3 and 6.2.2). They apply to XRECHNUNG invoices; Check also applies them when validate.Options selects the German rule sets (Germany, GermanyProfile, or Auto for a seller in Germany)."},
}

// countries holds the countries whose usage of the container applies to an
// invoice. Factur-X 1.09 (5.3 and 6.2.2) lets France and Germany differ on
// the /AFRelationship of BASIC, EN 16931 and EXTENDED invoices: France
// admits Alternative, Source and Data, Germany only Alternative.
type countries uint8

const (
	inFrance countries = 1 << iota
	inGermany
)

// countriesOf returns the countries whose rules the rule sets select.
func countriesOf(set validate.RuleSet) countries {
	var c countries
	if set&validate.France != 0 {
		c |= inFrance
	}
	if set&(validate.Germany|validate.GermanyProfile) != 0 {
		c |= inGermany
	}
	return c
}

// containerSeverity returns the severity of a rule of ContainerCatalog.
func containerSeverity(id string) rules.Severity {
	for _, r := range ContainerCatalog {
		if r.ID == id {
			return r.Severity
		}
	}
	return rules.Error
}

// containerCheck accumulates findings.
type containerCheck struct {
	findings []rules.Finding
}

func (k *containerCheck) add(id, path, format string, args ...any) {
	k.findings = append(k.findings, rules.Finding{
		Rule:     id,
		Severity: containerSeverity(id),
		Message:  fmt.Sprintf(format, args...),
		Path:     path,
		Source:   rules.SourcePDF,
	})
}

// CheckContainer checks the PDF container of a hybrid invoice: the PDF/A-3
// or PDF/A-4 identification and XMP metadata, the invoice attachment and its
// file specification, the consistency of the XMP hybrid-invoice schema
// with the attachment and its profile (BT-24), and the PDF/A requirements
// that do not depend on rendering the pages (fonts, actions, document
// information, other attachments, filters). The rules are listed in
// ContainerCatalog; every finding has Source rules.SourcePDF.
//
// The national usage of the /AFRelationship depends on the seller's country,
// which the container does not tell: CheckContainer applies the German one
// (PDF-40) to XRECHNUNG invoices only. Check applies the French and German
// usages whenever validate.Options selects their rule sets.
//
// The returned attachment is the one Extract returns, or nil when the PDF is
// encrypted (finding PDF-01) or has no readable invoice attachment (finding
// PDF-09). The error is reserved for files that cannot be read as a PDF.
func CheckContainer(pdfData []byte) (*Attachment, []rules.Finding, error) {
	return checkContainer(pdfData, nil)
}

// checkContainer implements CheckContainer. national, when not nil, returns
// the countries whose rules apply to the invoice XML; the German rules
// always apply to XRECHNUNG.
func checkContainer(pdfData []byte, national func(xml []byte) countries) (*Attachment, []rules.Finding, error) {
	k := &containerCheck{}
	c, err := openContainer(pdfData)
	if err != nil {
		if errors.Is(err, ErrEncrypted) {
			k.add("PDF-01", "trailer /Encrypt", "the PDF is encrypted")
			return nil, k.findings, nil
		}
		return nil, nil, err
	}

	info := k.document(c)

	fs, data, st, err := c.invoice()
	if err != nil {
		if errors.Is(err, ErrNoInvoice) {
			k.add("PDF-09", "catalog /Names /EmbeddedFiles", "no invoice XML attachment (factur-x.xml, xrechnung.xml, zugferd-invoice.xml or a CII XML file)")
		} else {
			k.add("PDF-09", specPath(fs), "the invoice attachment cannot be decoded: %v", err)
		}
		k.pdfa(c, fs)
		return nil, k.findings, nil
	}
	a := c.attachment(fs, data, st, metadataOf(info))
	var others []string
	for _, o := range c.specs {
		if o != fs && o.rank() >= 0 {
			others = append(others, strconv.Quote(o.filename()))
		}
	}
	if len(others) > 0 {
		k.add("PDF-28", "catalog /Names /EmbeddedFiles", "besides %q, the PDF has other invoice attachments: %s", a.Filename, strings.Join(others, ", "))
	}
	root, id := specificationID(data)
	profile, zf1 := xmlProfile(root, id)
	var where countries
	if national != nil {
		where = national(data)
	}
	if profile == invoice.ProfileXRechnung {
		where |= inGermany
	}
	k.attachment(c, fs, st, a, profile, zf1, where)
	if info != nil {
		k.metadata(info, a, root, profile, zf1)
	}
	// Last: the scan adds the files of page-level /AF arrays to c.specs,
	// which PDF-28 does not look at.
	k.pdfa(c, fs)
	return a, k.findings, nil
}

// document checks the XMP identification, the output intent and the file
// identifier. It returns the parsed XMP metadata, or nil.
func (k *containerCheck) document(c *container) *xmp.Info {
	info, present, err := c.xmpInfo()
	switch {
	case !present:
		k.add("PDF-02", "catalog /Metadata", "the document catalog has no XMP metadata stream")
	case info == nil && err == nil:
		k.add("PDF-03", "catalog /Metadata", "the XMP metadata stream cannot be decoded")
	case err != nil:
		k.add("PDF-03", "catalog /Metadata", "the XMP metadata cannot be read: %v", err)
		info = nil
	}
	if info != nil {
		k.identification(c, info)
		// PDF/A-4 deprecates the document information dictionary.
		if strings.TrimSpace(info.PDFAPart) != "4" {
			k.docInfo(c, info)
		}
	}

	found := false
	if intents, ok := c.r.Resolve(c.catalog.Get("OutputIntents")).(pdf.Array); ok {
		for i, o := range intents {
			d, ok := c.r.Dict(o)
			if !ok {
				continue
			}
			if s, _ := c.r.Resolve(d.Get("S")).(pdf.Name); s != "GTS_PDFA1" {
				continue
			}
			found = true
			if _, ok := c.r.Resolve(d.Get("DestOutputProfile")).(*pdf.Stream); !ok {
				k.add("PDF-07", fmt.Sprintf("catalog /OutputIntents [%d]", i), "the PDF/A output intent has no /DestOutputProfile ICC profile stream")
			}
			break
		}
	}
	if !found {
		k.add("PDF-06", "catalog /OutputIntents", "the catalog has no PDF/A output intent (/S /GTS_PDFA1)")
	}

	if id, ok := c.r.Resolve(c.r.Trailer().Get("ID")).(pdf.Array); !ok || len(id) != 2 {
		k.add("PDF-08", "trailer /ID", "the trailer has no file identifier (/ID array of two strings)")
	}
	return info
}

// identification checks the PDF/A identification of the XMP metadata
// against what a hybrid invoice needs: PDF/A-3 (any level), or PDF/A-4 at
// level F or E, which allow embedded files of any type. Plain PDF/A-4 only
// allows embedded PDF/A files.
func (k *containerCheck) identification(c *container, info *xmp.Info) {
	part := strings.TrimSpace(info.PDFAPart)
	switch part {
	case "3", "4":
	case "":
		k.add("PDF-04", "XMP pdfaid:part", "pdfaid:part is missing: the document declares neither PDF/A-3 nor PDF/A-4")
	default:
		k.add("PDF-04", "XMP pdfaid:part", "pdfaid:part is %q, not 3 or 4: PDF/A-%s does not allow embedded XML files", part, part)
	}

	conf := strings.TrimSpace(info.PDFAConformance)
	if part == "4" {
		switch conf {
		case "F", "E":
		case "":
			k.add("PDF-05", "XMP pdfaid:conformance", "pdfaid:conformance is missing: plain PDF/A-4 only allows embedded PDF/A files; an XML attachment needs PDF/A-4f (F) or PDF/A-4e (E)")
		default:
			k.add("PDF-05", "XMP pdfaid:conformance", "pdfaid:conformance is %q; the PDF/A-4 levels that allow an XML attachment are F and E", conf)
		}
		switch rev := strings.TrimSpace(info.PDFARev); {
		case rev == "":
			k.add("PDF-29", "XMP pdfaid:rev", "pdfaid:rev is missing: a PDF/A-4 file gives the year of the ISO 19005-4 edition it follows (2020)")
		case !isYear(rev):
			k.add("PDF-29", "XMP pdfaid:rev", "pdfaid:rev is %q, not a four-digit year", rev)
		}
	} else {
		switch conf {
		case "A", "B", "U":
		case "":
			k.add("PDF-05", "XMP pdfaid:conformance", "pdfaid:conformance is missing")
		default:
			k.add("PDF-05", "XMP pdfaid:conformance", "pdfaid:conformance is %q, not A, B or U", conf)
		}
	}

	v, where := c.version(), "header"
	if v != c.r.Version() {
		where = "catalog /Version"
	}
	switch {
	case part == "3" && !versionLess(v, "2.0"):
		k.add("PDF-30", where, "the PDF version is %s, but PDF/A-3 is based on PDF 1.7: a PDF 2.0 file can only be PDF/A-4", v)
	case part == "4" && versionLess(v, "2.0"):
		k.add("PDF-30", where, "the PDF version is %s, but PDF/A-4 is based on PDF 2.0", v)
	}
}

// isYear reports whether s is made of four digits.
func isYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// attachment checks the file specification and the embedded file stream.
func (k *containerCheck) attachment(c *container, fs *fileSpec, st *pdf.Stream, a *Attachment, profile invoice.Profile, zf1 bool, where countries) {
	path := specPath(fs)
	name := a.Filename
	want := "factur-x.xml"
	if profile == invoice.ProfileXRechnung {
		want = "xrechnung.xml"
	}
	switch {
	case name == want:
	case strings.EqualFold(name, "zugferd-invoice.xml"):
		k.add("PDF-11", path, "the attachment uses the legacy ZUGFeRD name %q; Factur-X readers expect %q", name, want)
	case nameRank(name) >= 0:
		k.add("PDF-11", path, "the attachment is named %q; profile %s expects %q", name, profileLabel(profile, zf1), want)
	default:
		k.add("PDF-10", path, "the invoice attachment is named %q, which is not an invoice file name (expected %q)", name, want)
	}

	switch rel := a.Relationship; rel {
	case "":
		k.add("PDF-12", path+" /AFRelationship", "the file specification has no /AFRelationship")
	case "Data", "Alternative", "Source":
		k.relationship(path+" /AFRelationship", rel, profile, zf1, where)
	default:
		k.add("PDF-13", path+" /AFRelationship", "/AFRelationship is %s, not Data, Alternative or Source", rel)
	}

	if !a.InAF {
		k.add("PDF-15", "catalog /AF", "the file specification of %q is not listed in the /AF array of the catalog", name)
	}

	switch mime := a.MimeType; mime {
	case "text/xml":
	case "application/xml":
		k.add("PDF-17", "embedded file /Subtype", "the MIME type is application/xml; Factur-X uses text/xml")
	case "":
		k.add("PDF-16", "embedded file /Subtype", "the embedded file stream has no /Subtype MIME type")
	default:
		k.add("PDF-16", "embedded file /Subtype", "the MIME type is %q, not text/xml", mime)
	}

	var modDate pdf.Object
	if st != nil {
		if params, ok := c.r.Dict(st.Dict.Get("Params")); ok {
			modDate = c.r.Resolve(params.Get("ModDate"))
		}
	}
	switch s, ok := modDate.(pdf.String); {
	case modDate == nil:
		k.add("PDF-18", "embedded file /Params /ModDate", "the embedded file stream has no /Params /ModDate")
	case !ok:
		k.add("PDF-19", "embedded file /Params /ModDate", "/ModDate is not a string")
	default:
		if !strictDate(pdf.Text(s)) {
			k.add("PDF-19", "embedded file /Params /ModDate", "/ModDate %q is not a valid PDF date", pdf.Text(s))
		}
	}

	var missing []string
	for _, key := range []pdf.Name{"F", "UF"} {
		if s, ok := c.r.Resolve(fs.dict.Get(key)).(pdf.String); !ok || len(s) == 0 {
			missing = append(missing, "/"+string(key))
		}
	}
	if len(missing) > 0 {
		k.add("PDF-20", path, "the file specification has no %s entry", strings.Join(missing, " or "))
	}
}

// relationship checks the /AFRelationship of the invoice against its profile
// and the countries whose rules apply, as Factur-X 1.09 (6.2.2) tabulates
// them: Data for MINIMUM and BASIC WL in France and Germany; for BASIC and
// above, Alternative in Germany, and Alternative, Source or Data in France.
// Without national rules, Data is reported for BASIC and above: the XML is
// a full invoice, which Alternative (or Source) describes.
func (k *containerCheck) relationship(path, rel string, profile invoice.Profile, zf1 bool, where countries) {
	switch {
	case profile == invoice.ProfileUnknown:
	case profile == invoice.ProfileMinimum || profile == invoice.ProfileBasicWL:
		if rel != "Data" {
			k.add("PDF-14", path, "/AFRelationship is %s; profile %s expects Data", rel, profile)
		}
	case rel == "Alternative":
	case where&inGermany != 0:
		k.add("PDF-40", path, "/AFRelationship is %s; Germany admits only Alternative for profile %s", rel, profileLabel(profile, zf1))
	case rel == "Data" && where&inFrance == 0:
		k.add("PDF-14", path, "/AFRelationship is Data; profile %s expects Alternative or Source (France also admits Data)", profileLabel(profile, zf1))
	}
}

// metadata checks the hybrid-invoice schema of the XMP metadata.
func (k *containerCheck) metadata(info *xmp.Info, a *Attachment, root string, profile invoice.Profile, zf1 bool) {
	inv := info.Invoice
	if inv == nil {
		k.add("PDF-21", "XMP", "the XMP metadata has no Factur-X / ZUGFeRD schema (namespace %s)", xmp.NSFacturX)
		return
	}
	prefix := "XMP " + invoicePrefix(inv.NamespaceURI) + ":"

	if t := strings.TrimSpace(inv.DocumentType); t != "INVOICE" {
		if t == "" {
			k.add("PDF-22", prefix+"DocumentType", "DocumentType is missing")
		} else {
			k.add("PDF-22", prefix+"DocumentType", "DocumentType is %q, not INVOICE", t)
		}
	}

	if n := strings.TrimSpace(inv.DocumentFileName); n != a.Filename {
		if n == "" {
			k.add("PDF-23", prefix+"DocumentFileName", "DocumentFileName is missing")
		} else {
			k.add("PDF-23", prefix+"DocumentFileName", "DocumentFileName is %q, the attachment is named %q", n, a.Filename)
		}
	}

	level := strings.TrimSpace(inv.ConformanceLevel)
	declared := invoice.ProfileFromConformanceLevel(level)
	switch {
	case level == "":
		k.add("PDF-24", prefix+"ConformanceLevel", "ConformanceLevel is missing")
	case declared == invoice.ProfileUnknown:
		k.add("PDF-24", prefix+"ConformanceLevel", "ConformanceLevel %q is not a known level", level)
	case profile != invoice.ProfileUnknown && sameLevel(declared) != sameLevel(profile) &&
		!(profile == invoice.ProfileXRechnung && declared == invoice.ProfileEN16931):
		k.add("PDF-24", prefix+"ConformanceLevel", "ConformanceLevel is %q, the XML is %s (BT-24)", level, profileLabel(profile, zf1))
	}

	if s := info.Schema(inv.NamespaceURI); s == nil {
		k.add("PDF-25", "XMP pdfaExtension:schemas", "no PDF/A extension schema describes the namespace %s", inv.NamespaceURI)
	} else {
		var undescribed []string
		for _, p := range []struct{ name, value string }{
			{"DocumentType", inv.DocumentType},
			{"DocumentFileName", inv.DocumentFileName},
			{"Version", inv.Version},
			{"ConformanceLevel", inv.ConformanceLevel},
		} {
			if p.value != "" && !contains(s.Properties, p.name) {
				undescribed = append(undescribed, p.name)
			}
		}
		if len(undescribed) > 0 {
			k.add("PDF-25", "XMP pdfaExtension:schemas", "the PDF/A extension schema of %s does not describe %s", inv.NamespaceURI, strings.Join(undescribed, ", "))
		}
	}

	if strings.TrimSpace(inv.Version) == "" {
		k.add("PDF-26", prefix+"Version", "Version is missing")
	}

	switch root {
	case "CrossIndustryInvoice":
		if inv.NamespaceURI != xmp.NSFacturX && inv.NamespaceURI != xmp.NSZUGFeRD2 {
			k.add("PDF-27", "XMP", "the XMP uses the ZUGFeRD 1.0 namespace %s, but the XML is a CrossIndustryInvoice (Factur-X / ZUGFeRD 2)", inv.NamespaceURI)
		}
	case "CrossIndustryDocument":
		if inv.NamespaceURI != xmp.NSZUGFeRD1 && inv.NamespaceURI != xmp.NSZUGFeRD1RC {
			k.add("PDF-27", "XMP", "the XMP uses the namespace %s, but the XML is a ZUGFeRD 1.0 CrossIndustryDocument", inv.NamespaceURI)
		}
	}
}

// xmlProfile returns the profile of an invoice XML from its root element
// and BT-24. ZUGFeRD 1.0 identifiers (urn:ferd:CrossIndustryDocument:
// invoice:1p0:basic, comfort, extended) map to BASIC, EN 16931 and EXTENDED.
func xmlProfile(root, id string) (p invoice.Profile, zf1 bool) {
	if root == "CrossIndustryDocument" || strings.HasPrefix(strings.TrimSpace(id), "urn:ferd:") {
		id = strings.ToLower(strings.TrimSpace(id))
		switch {
		case strings.HasSuffix(id, ":basic"):
			return invoice.ProfileBasic, true
		case strings.HasSuffix(id, ":comfort"):
			return invoice.ProfileEN16931, true
		case strings.HasSuffix(id, ":extended"):
			return invoice.ProfileExtended, true
		}
		return invoice.ProfileUnknown, true
	}
	return invoice.ProfileFromURN(id), false
}

// sameLevel maps EXTENDED-CTC-FR to EXTENDED and PEPPOL to EN 16931: the
// Factur-X specification defines no XMP ConformanceLevel of their own.
func sameLevel(p invoice.Profile) invoice.Profile {
	switch p {
	case invoice.ProfileExtendedCTCFR:
		return invoice.ProfileExtended
	case invoice.ProfilePeppol:
		return invoice.ProfileEN16931
	}
	return p
}

// profileLabel names a profile in messages; ZUGFeRD 1.0 COMFORT is shown as
// such.
func profileLabel(p invoice.Profile, zf1 bool) string {
	if zf1 {
		switch p {
		case invoice.ProfileEN16931:
			return "ZUGFeRD 1.0 COMFORT"
		case invoice.ProfileBasic, invoice.ProfileExtended:
			return "ZUGFeRD 1.0 " + p.String()
		}
	}
	return p.String()
}

// invoicePrefix returns the conventional XMP prefix of a hybrid-invoice
// namespace.
func invoicePrefix(ns string) string {
	switch ns {
	case xmp.NSZUGFeRD2, xmp.NSZUGFeRD1, xmp.NSZUGFeRD1RC:
		return "zf"
	}
	return "fx"
}

// specPath locates a file specification in findings.
func specPath(fs *fileSpec) string {
	if fs == nil {
		return "file specification"
	}
	if fs.ref != (pdf.Ref{}) {
		return "file specification " + fs.ref.String()
	}
	return "file specification"
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// strictDate reports whether s is a PDF date as ISO 32000 defines it:
// "D:YYYY" optionally followed by MM, DD, HH, mm, SS and a time zone (Z, or
// +HH'mm' / -HH'mm', the apostrophes being optional). Unlike
// pdf.ParseDate, which accepts what writers produce in practice, it rejects
// missing prefixes, out-of-range fields and trailing garbage.
func strictDate(s string) bool {
	s, ok := strings.CutPrefix(s, "D:")
	if !ok || len(s) < 4 {
		return false
	}
	num := func(n int, lo, hi int) bool {
		if len(s) < n {
			return false
		}
		v := 0
		for i := 0; i < n; i++ {
			if s[i] < '0' || s[i] > '9' {
				return false
			}
			v = v*10 + int(s[i]-'0')
		}
		s = s[n:]
		return v >= lo && v <= hi
	}
	if !num(4, 0, 9999) {
		return false
	}
	for _, f := range [][2]int{{1, 12}, {1, 31}, {0, 23}, {0, 59}, {0, 59}} {
		if len(s) == 0 || s[0] < '0' || s[0] > '9' {
			break
		}
		if !num(2, f[0], f[1]) {
			return false
		}
	}
	if len(s) == 0 {
		return true
	}
	switch s[0] {
	case 'Z':
		s = s[1:]
		// Some writers append 00'00' after Z.
		s = strings.TrimPrefix(s, "00'00'")
		s = strings.TrimPrefix(s, "00'00")
		return s == ""
	case '+', '-':
		s = s[1:]
		if !num(2, 0, 23) {
			return false
		}
		if len(s) == 0 {
			return true
		}
		s = strings.TrimPrefix(s, "'")
		if len(s) == 0 {
			return true
		}
		if !num(2, 0, 59) {
			return false
		}
		s = strings.TrimPrefix(s, "'")
		return s == ""
	}
	return false
}
