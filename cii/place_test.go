package cii

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ritoon/factur-x/invoice"
)

// TestExtensionPlacement checks where Marshal puts the extensions back, and
// which ones it reports as lost.
func TestExtensionPlacement(t *testing.T) {
	const (
		agreement = "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement"
		settle    = "SupplyChainTradeTransaction/ApplicableHeaderTradeSettlement"
	)
	tests := []struct {
		name    string
		setup   func(inv *invoice.Invoice)
		target  invoice.Profile
		want    []string // substrings of the output
		notWant []string
		lost    []string // terms of the LossError, in order
	}{
		{
			name: "element in schema order",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{{Path: "ExchangedDocument", XML: "<ram:Name>N</ram:Name>"}}
			},
			want: []string{"<ram:ID>1</ram:ID><ram:Name>N</ram:Name><ram:TypeCode>380</ram:TypeCode>"},
		},
		{
			name: "attribute",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{{Path: "ExchangedDocument/ID", Attr: "schemeID", Value: "S&1"}}
			},
			want: []string{`<ram:ID schemeID="S&amp;1">1</ram:ID>`},
		},
		{
			name: "attribute of an occurrence",
			setup: func(inv *invoice.Invoice) {
				inv.Seller.IDs = []invoice.Identifier{{ID: "A"}, {ID: "B"}}
				inv.Seller.Extensions = []invoice.Extension{{Path: "ID[2]", Attr: "schemeID", Value: "X"}}
			},
			want: []string{`<ram:ID>A</ram:ID><ram:ID schemeID="X">B</ram:ID>`},
		},
		{
			name: "attribute of a holder element",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{{Attr: "languageID", Value: "fr"}}
			},
			lost: []string{"EXT @languageID"},
		},
		{
			name: "containers created",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{{Path: agreement + "/ApplicableTradeDeliveryTerms/RelevantTradeLocation", XML: "<ram:Name>Lyon</ram:Name>"}}
			},
			want: []string{"</ram:BuyerTradeParty><ram:ApplicableTradeDeliveryTerms><ram:RelevantTradeLocation><ram:Name>Lyon</ram:Name></ram:RelevantTradeLocation></ram:ApplicableTradeDeliveryTerms></ram:ApplicableHeaderTradeAgreement>"},
		},
		{
			name: "container left empty",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{{Path: agreement + "/ApplicableTradeDeliveryTerms/RelevantTradeLocation", XML: "<ram:Unknown>x</ram:Unknown>"}}
			},
			notWant: []string{"ApplicableTradeDeliveryTerms"},
			lost:    []string{"EXT " + agreement + "/ApplicableTradeDeliveryTerms/RelevantTradeLocation/Unknown"},
		},
		{
			name: "beyond the maximum occurrences",
			setup: func(inv *invoice.Invoice) {
				inv.Language = "fr"
				inv.Extensions = []invoice.Extension{
					{Path: "ExchangedDocument", XML: "<ram:Name>N1</ram:Name>"},
					{Path: "ExchangedDocument", XML: "<ram:Name>N2</ram:Name>"},
					{Path: "ExchangedDocument", XML: "<ram:LanguageID>de</ram:LanguageID>"},
				}
			},
			want:    []string{"<ram:Name>N1</ram:Name>", "<ram:LanguageID>fr</ram:LanguageID>"},
			notWant: []string{"N2", ">de<"},
			lost:    []string{"EXT ExchangedDocument/Name", "EXT ExchangedDocument/LanguageID"},
		},
		{
			name: "repeated element after the written ones",
			setup: func(inv *invoice.Invoice) {
				inv.Notes = []invoice.Note{{Content: "A"}}
				inv.Extensions = []invoice.Extension{{Path: "ExchangedDocument", XML: "<ram:IncludedNote><ram:Content>B</ram:Content></ram:IncludedNote>"}}
			},
			want: []string{"<ram:IncludedNote><ram:Content>A</ram:Content></ram:IncludedNote><ram:IncludedNote><ram:Content>B</ram:Content></ram:IncludedNote></rsm:ExchangedDocument>"},
		},
		{
			name: "outside the schema",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{
					{Path: "ExchangedDocument", XML: "<ram:Unknown>x</ram:Unknown>"},
					{Path: "ExchangedDocument", XML: `<x:Name xmlns:x="urn:example">x</x:Name>`},
					{Path: "ExchangedDocument/ID", Attr: "unknown", Value: "x"},
					{Path: "ExchangedDocument/IssueDateTime/DateTimeString", Attr: "format", Value: "203"},
					{Path: "ExchangedDocument", XML: "<ram:Name>"},
					{Path: agreement + "/Unknown", XML: "<ram:Name>x</ram:Name>"},
				}
			},
			want:    []string{`<rsm:ExchangedDocument><ram:ID>1</ram:ID><ram:TypeCode>380</ram:TypeCode><ram:IssueDateTime><udt:DateTimeString format="102">`},
			notWant: []string{"Unknown", "urn:example", `format="203"`},
			// Reported when the element that should hold them is closed.
			lost: []string{
				"EXT ExchangedDocument/IssueDateTime/DateTimeString/@format", "EXT ExchangedDocument/ID/@unknown",
				"EXT ExchangedDocument/Unknown", "EXT ExchangedDocument/Name",
				"EXT " + agreement + "/Unknown/Name",
			},
		},
		{
			name: "EXTENDED data written as EN 16931",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{{Path: "ExchangedDocument/ID", Attr: "schemeID", Value: "S"}}
			},
			target:  invoice.ProfileEN16931,
			notWant: []string{"schemeID"},
			lost:    []string{"EXT ExchangedDocument/ID/@schemeID"},
		},
		{
			name: "EN 16931 data written as EN 16931",
			setup: func(inv *invoice.Invoice) {
				inv.Profile = invoice.ProfileEN16931
				inv.Extensions = []invoice.Extension{{Path: "ExchangedDocument/ID", Attr: "schemeID", Value: "S"}}
			},
			target: invoice.ProfileEN16931,
			want:   []string{`<ram:ID schemeID="S">1</ram:ID>`},
		},
		{
			name: "holder value",
			setup: func(inv *invoice.Invoice) {
				inv.PaymentTerms = []invoice.PaymentTerms{{Extensions: []invoice.Extension{
					{XML: "<ram:ApplicableTradePaymentPenaltyTerms><ram:CalculationPercent>3</ram:CalculationPercent></ram:ApplicableTradePaymentPenaltyTerms>"},
				}}}
			},
			want: []string{"<ram:SpecifiedTradePaymentTerms><ram:ApplicableTradePaymentPenaltyTerms><ram:CalculationPercent>3</ram:CalculationPercent></ram:ApplicableTradePaymentPenaltyTerms></ram:SpecifiedTradePaymentTerms>"},
		},
		{
			name: "settlement",
			setup: func(inv *invoice.Invoice) {
				inv.Extensions = []invoice.Extension{{Path: settle, XML: "<ram:InvoiceIssuerReference>I</ram:InvoiceIssuerReference>"}}
			},
			want: []string{"<ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode><ram:InvoiceIssuerReference>I</ram:InvoiceIssuerReference>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := &invoice.Invoice{Profile: invoice.ProfileExtended, Number: "1", TypeCode: "380", Currency: "EUR"}
			tt.setup(inv)
			target := tt.target
			if target == invoice.ProfileUnknown {
				target = invoice.ProfileExtended
			}
			out := string(mustMarshal(t, inv, &MarshalOptions{Profile: target}))
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %s:\n%s", w, out)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(out, w) {
					t.Errorf("output holds %s:\n%s", w, out)
				}
			}
			_, err := Marshal(inv, &MarshalOptions{Profile: target, Strict: true})
			var le *LossError
			switch {
			case len(tt.lost) == 0 && err != nil:
				t.Errorf("strict: %v", err)
			case len(tt.lost) > 0 && (!errors.As(err, &le) || !slices.Equal(le.Terms, tt.lost)):
				t.Errorf("strict: %v, want losses %v", err, tt.lost)
			}
		})
	}
}

// TestExtensionIndent checks that the extensions follow the indentation of
// the document.
func TestExtensionIndent(t *testing.T) {
	inv := &invoice.Invoice{
		Profile: invoice.ProfileExtended, Number: "1", TypeCode: "380", Currency: "EUR",
		Extensions: []invoice.Extension{{
			Path: "SupplyChainTradeTransaction/ApplicableHeaderTradeAgreement",
			XML:  "<ram:QuotationReferencedDocument><ram:IssuerAssignedID>Q</ram:IssuerAssignedID></ram:QuotationReferencedDocument>",
		}},
	}
	out := string(mustMarshal(t, inv, &MarshalOptions{Indent: true}))
	want := `
      </ram:BuyerTradeParty>
      <ram:QuotationReferencedDocument>
        <ram:IssuerAssignedID>Q</ram:IssuerAssignedID>
      </ram:QuotationReferencedDocument>
    </ram:ApplicableHeaderTradeAgreement>`
	if !strings.Contains(out, want) {
		t.Errorf("output lacks%s\n%s", want, out)
	}
}
