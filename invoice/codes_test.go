package invoice

import (
	"testing"

	"github.com/ritoon/zerechnung/internal/codes"
)

// TestTaxPointCodes checks the BT-8 constants against the code list of
// BR-CL-06, and the code of the option for VAT on debits (DGFiP external
// specifications v3.2, rule G1.43: 5 in CII).
func TestTaxPointCodes(t *testing.T) {
	for _, c := range []string{TaxPointInvoiceDate, TaxPointDeliveryDate, TaxPointPaymentDate, TaxPointDebits} {
		if !codes.TaxPointDateCode(c) {
			t.Errorf("%q is not a VAT point date code", c)
		}
	}
	if TaxPointDebits != "5" {
		t.Errorf("TaxPointDebits = %q, want 5", TaxPointDebits)
	}
}
