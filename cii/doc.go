// Package cii reads and writes invoices in the UN/CEFACT Cross Industry
// Invoice (CII D16B) syntax used by Factur-X, ZUGFeRD 2.x and XRechnung.
//
// Parse turns a rsm:CrossIndustryInvoice document into an invoice.Invoice.
// Reading is tolerant: any namespace prefix is accepted, values are trimmed,
// ZUGFeRD 2.0 and 2.1 documents are understood, and a value that cannot be
// read (a malformed amount, date or indicator) is reported as an Issue while
// the rest of the document is still read. ParseTree does the same from a tree
// already parsed with internal/xmltree, so that a caller that also validates
// the document parses the XML only once.
//
// Marshal writes an invoice for a target Factur-X profile. Only the elements
// that the profile allows are written, in the exact order of the profile's
// XML schema. Data that the profile cannot carry is dropped, or reported as a
// *LossError when MarshalOptions.Strict is set.
package cii
