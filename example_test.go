package facturx_test

import (
	"fmt"
	"log"
	"os"
	"time"

	facturx "github.com/ritoon/factur-x"
	"github.com/ritoon/factur-x/invoice"
	"github.com/ritoon/factur-x/validate"
)

// newInvoice builds a French B2B invoice for services at the EN 16931
// profile, with the mentions required by the 2026 reform.
func newInvoice() *invoice.Invoice {
	d := invoice.MustDecimal
	seller := invoice.Party{
		Name:              "Exemple Informatique SAS",
		IDs:               []invoice.Identifier{{ID: "12345678900017", Scheme: invoice.SchemeSIRET}},
		LegalRegistration: invoice.Identifier{ID: "123456789", Scheme: invoice.SchemeSIREN},
		VATID:             "FR32123456789",
		Address:           invoice.Address{Line1: "1 rue de l'Exemple", PostCode: "75001", City: "Paris", CountryCode: "FR"},
		ElectronicAddress: invoice.Identifier{ID: "123456789_FACTURES", Scheme: invoice.SchemeFRDirectory},
	}
	buyer := invoice.Party{
		Name:              "Client Exemple SARL",
		IDs:               []invoice.Identifier{{ID: "98765432100015", Scheme: invoice.SchemeSIRET}},
		LegalRegistration: invoice.Identifier{ID: "987654321", Scheme: invoice.SchemeSIREN},
		VATID:             "FR58987654321",
		Address:           invoice.Address{Line1: "2 place de l'Exemple", PostCode: "69002", City: "Lyon", CountryCode: "FR"},
		ElectronicAddress: invoice.Identifier{ID: "987654321_ACHATS", Scheme: invoice.SchemeFRDirectory},
	}
	return &invoice.Invoice{
		Profile:         invoice.ProfileEN16931,
		BusinessProcess: "S1", // services
		Number:          "FA-2026-00042",
		TypeCode:        invoice.TypeCommercialInvoice,
		IssueDate:       invoice.NewDate(2026, time.October, 15),
		Currency:        "EUR",
		Notes: []invoice.Note{
			{SubjectCode: "BAR", Content: "B2B"},
			{SubjectCode: "PMT", Content: "Indemnité forfaitaire pour frais de recouvrement : 40 euros."},
			{SubjectCode: "PMD", Content: "Pénalités de retard : trois fois le taux d'intérêt légal."},
			{SubjectCode: "AAB", Content: "Pas d'escompte pour paiement anticipé."},
		},
		BuyerReference: "SERVICE-ACHATS",
		Seller:         seller,
		Buyer:          buyer,
		DeliveryDate:   invoice.NewDate(2026, time.October, 12),
		PaymentMeans:   []invoice.PaymentMeans{{TypeCode: invoice.PaymentSEPACreditTransfer, PayeeIBAN: "FR7630006000011234567890189"}},
		PaymentTerms:   []invoice.PaymentTerms{{Description: "Paiement à 30 jours par virement.", DueDate: invoice.NewDate(2026, time.November, 14)}},
		Lines: []invoice.Line{{
			ID:              "1",
			Item:            invoice.Item{Name: "Maintenance applicative, octobre 2026"},
			Quantity:        d("2"),
			UnitCode:        "DAY",
			NetPrice:        d("450.00"),
			VATCategoryCode: invoice.VATStandard,
			VATRate:         d("20.00"),
		}},
	}
}

// Example creates a Factur-X invoice from a PDF produced elsewhere, then reads
// it back and checks it with the French rules.
func Example() {
	host, err := os.ReadFile("testdata/host.pdf") // any PDF, ideally PDF/A-3
	if err != nil {
		log.Fatal(err)
	}

	inv := newInvoice()
	pdf, err := facturx.Create(host, inv, &facturx.CreateOptions{
		Calculate: true, // line amounts, totals and VAT breakdown
		Validate:  &validate.Options{Rules: validate.Default | validate.France},
	})
	if err != nil {
		log.Fatal(err)
	}

	back, att, err := facturx.Read(pdf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(att.Filename, att.Relationship, att.Metadata.ConformanceLevel)
	fmt.Println(back.Number, back.Totals.TaxBasisTotal, back.Totals.TaxTotal, back.Totals.DuePayable, back.Currency)

	report, _, err := facturx.Check(pdf, &validate.Options{Rules: validate.Default | validate.France})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("errors:", len(report.Errors()))
	// Output:
	// factur-x.xml Alternative EN 16931
	// FA-2026-00042 900.00 180.00 1080.00 EUR
	// errors: 0
}
