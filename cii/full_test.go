package cii

import (
	"github.com/ritoon/factur-x/invoice"
)

var (
	d  = invoice.MustDecimal
	dt = invoice.MustDate
)

func party(prefix string) invoice.Party {
	return invoice.Party{
		IDs: []invoice.Identifier{
			{ID: prefix + "-ID-1"},
			{ID: prefix + "-GLN", Scheme: "0088"},
		},
		Name:              prefix + " Name",
		Description:       prefix + " legal information",
		LegalRegistration: invoice.Identifier{ID: "123456789", Scheme: "0002"},
		TradingName:       prefix + " Trading",
		Contact: &invoice.Contact{
			PersonName:     prefix + " Person",
			DepartmentName: prefix + " Dept",
			Phone:          "+33 1 23 45 67 89",
			Email:          prefix + "@example.com",
		},
		Address: invoice.Address{
			Line1: "1 rue " + prefix, Line2: "Bât. B", Line3: "3e étage",
			City: "Paris", PostCode: "75001", CountrySubdivision: "IDF", CountryCode: "FR",
		},
		ElectronicAddress: invoice.Identifier{ID: prefix + "@peppol.example", Scheme: "EM"},
		VATID:             "FR12123456789",
		TaxRegistrationID: "FC-" + prefix,
	}
}

func partyPtr(prefix string) *invoice.Party {
	p := party(prefix)
	return &p
}

// fullInvoice returns an invoice that sets every field of the model, with
// values that read back identically (so that an EXTENDED round trip is
// lossless).
func fullInvoice() *invoice.Invoice {
	seller := party("Seller")
	buyer := party("Buyer")
	inv := &invoice.Invoice{
		Profile:         invoice.ProfileExtended,
		SpecificationID: invoice.URNExtended,
		BusinessProcess: "B1",
		TestIndicator:   true,
		Number:          "F-2026-001 <&>",
		TypeCode:        "380",
		IssueDate:       dt("2026-11-16"),
		Notes: []invoice.Note{
			{SubjectCode: "AAI", Content: "Note 1 \"quoted\" & <tagged>", ContentCode: "ST1"},
			{SubjectCode: "PMT", Content: "Indemnité forfaitaire pour frais de recouvrement : 40 €"},
		},
		Language:         "fr",
		Currency:         "EUR",
		TaxCurrency:      "USD",
		TaxPointDate:     dt("2026-11-10"),
		TaxPointDateCode: "5",
		BuyerReference:   "BUYER-REF",
		ProjectReference: "PRJ-1",
		ProjectName:      "Project One",

		ContractReference:        "CT-1",
		PurchaseOrderReference:   "PO-1",
		SalesOrderReference:      "SO-1",
		ReceivingAdviceReference: "RA-1",
		DespatchAdviceReference:  "DA-1",
		AdditionalDocuments: []invoice.AdditionalDocument{
			{ID: "TENDER-1", TypeCode: "50", IssueDate: dt("2026-01-01")},
			{ID: "OBJ-1", TypeCode: "130", ReferenceTypeCode: "AAA"},
			{
				ID: "DOC-1", TypeCode: "916", Description: "Timesheet", URI: "https://example.com/doc?a=1&b=2",
				Attachment: []byte("hello, world\x00\xff"), AttachmentMimeCode: "application/pdf", AttachmentFilename: "doc.pdf",
			},
		},
		BuyerAccountingReference: "ACC-1",
		PrecedingInvoices: []invoice.DocumentReference{
			{ID: "F-2026-000", IssueDate: dt("2026-10-01"), TypeCode: "380"},
		},

		Seller:                  seller,
		Buyer:                   buyer,
		SellerTaxRepresentative: partyPtr("TaxRep"),
		Payee:                   partyPtr("Payee"),
		SalesAgent:              partyPtr("SalesAgent"),
		BuyerAgent:              partyPtr("BuyerAgent"),
		Invoicer:                partyPtr("Invoicer"),
		Invoicee:                partyPtr("Invoicee"),
		Payer:                   partyPtr("Payer"),
		ShipTo:                  partyPtr("ShipTo"),
		DeliveryDate:            dt("2026-11-05"),
		InvoicingPeriod:         invoice.Period{Start: dt("2026-10-01"), End: dt("2026-10-31"), Description: "October"},

		RemittanceInformation: "REMIT-1",
		CreditorReferenceID:   "FR00ZZZ123456",
		PaymentMeans: []invoice.PaymentMeans{
			{
				TypeCode: "58", Information: "SEPA transfer",
				PayeeIBAN: "FR7630006000011234567890189", PayeeAccountName: "Seller account", PayeeBIC: "AGRIFRPP",
			},
			{TypeCode: "30", PayeeProprietaryID: "LOCAL-ACCOUNT-1"},
			{TypeCode: "48", Information: "Card", CardID: "1234", CardholderName: "J. Doe"},
			{TypeCode: "59", PayerIBAN: "DE02120300000000202051", PayerAccountName: "Buyer account", PayerBIC: "DEUTDEFF"},
		},
		PaymentTerms: []invoice.PaymentTerms{
			{Description: "30 days", DueDate: dt("2026-12-16"), DirectDebitMandateID: "MANDATE-1", PartialAmount: d("100.00")},
			{Description: "Balance", DueDate: dt("2027-01-16"), PartialAmount: d("140.00")},
		},
		AllowancesCharges: []invoice.AllowanceCharge{
			{Charge: false, Amount: d("10.00"), BaseAmount: d("100.00"), Percentage: d("10"), Reason: "Discount", ReasonCode: "95", VATCategoryCode: "S", VATRate: d("20.00")},
			{Charge: true, Amount: d("5.00"), Reason: "Freight", ReasonCode: "FC", VATCategoryCode: "S", VATRate: d("20.00")},
		},
		VATBreakdown: []invoice.VATBreakdown{
			{CategoryCode: "S", Rate: d("20.00"), TaxableAmount: d("195.00"), TaxAmount: d("39.00")},
			{CategoryCode: "E", Rate: d("0"), TaxableAmount: d("10.00"), TaxAmount: d("0.00"), ExemptionReason: "Exempt", ExemptionReasonCode: "VATEX-EU-132"},
		},
		Totals: invoice.Totals{
			LineTotal: d("210.00"), AllowanceTotal: d("10.00"), ChargeTotal: d("5.00"),
			TaxBasisTotal: d("205.00"), TaxTotal: d("39.00"), TaxTotalInTaxCurrency: d("42.12"),
			GrandTotal: d("244.00"), PaidAmount: d("4.00"), RoundingAmount: d("0.00"), DuePayable: d("252.50"),
		},
		ThirdPartyCharges: []invoice.ThirdPartyCharge{{Reason: "Stamp duty", Amount: d("12.50")}},
		Lines: []invoice.Line{
			{
				ID:   "1",
				Note: "Line note <1>",
				Item: invoice.Item{
					GlobalID: invoice.Identifier{ID: "4012345001235", Scheme: "0160"},
					SellerID: "SKU-1", BuyerID: "BUY-1", Name: "Widget", Description: "A blue widget",
					Attributes: []invoice.ItemAttribute{{Name: "Color", Value: "Blue", TypeCode: "COL"}},
					Classifications: []invoice.Classification{
						{Code: "44121600", ListID: "IB", ListVersionID: "v1", Name: "Office supplies"},
					},
					OriginCountry: "DE",
				},
				GrossPrice: d("110.00"), PriceDiscount: d("10.00"), NetPrice: d("100.00"),
				PriceBaseQuantity: d("1"), PriceBaseQuantityUnit: "C62",
				OrderReference: "PO-1", OrderLineReference: "10",
				Quantity: d("2.000"), UnitCode: "C62",
				ShipTo:          partyPtr("LineShipTo"),
				DeliveryDate:    dt("2026-11-04"),
				VATCategoryCode: "S", VATRate: d("20.00"),
				Period: invoice.Period{Start: dt("2026-10-01"), End: dt("2026-10-15"), Description: "First half"},
				AllowancesCharges: []invoice.AllowanceCharge{
					{Amount: d("5.00"), BaseAmount: d("200.00"), Percentage: d("2.5"), Reason: "Line discount", ReasonCode: "95", VATCategoryCode: "S", VATRate: d("20.00")},
					{Charge: true, Amount: d("5.00"), Reason: "Line charge", ReasonCode: "ABL"},
				},
				NetAmount:           d("200.00"),
				InvoiceReference:    &invoice.DocumentReference{ID: "F-2026-000", LineID: "3", TypeCode: "380", IssueDate: dt("2026-10-01")},
				ObjectIdentifier:    invoice.Identifier{ID: "OBJ-L1", Scheme: "AAB"},
				AccountingReference: "ACC-L1",
			},
			{
				ID: "2", ParentID: "1", StatusCode: "TYPE_LINE", StatusReasonCode: "DETAIL",
				Item:     invoice.Item{Name: "Sub-item"},
				NetPrice: d("10.00"), Quantity: d("1"), UnitCode: "H87",
				VATCategoryCode: "E", VATRate: d("0"),
				NetAmount: d("10.00"),
			},
		},
	}
	// The seller is the only party with a description in EN 16931; buyer
	// descriptions only exist in EXTENDED. Other roles carry none.
	for _, p := range []*invoice.Party{inv.SellerTaxRepresentative, inv.Payee, inv.SalesAgent, inv.BuyerAgent, inv.Invoicer, inv.Invoicee, inv.Payer, inv.ShipTo, inv.Lines[0].ShipTo} {
		p.Description = ""
	}
	return inv
}
