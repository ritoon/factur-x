package invoice

import (
	"slices"
	"testing"
)

func TestExtension(t *testing.T) {
	for _, c := range []struct {
		e          Extension
		name, path string
	}{
		{Extension{XML: "<ram:BrandName>Acme</ram:BrandName>", Path: "SpecifiedTradeProduct"}, "BrandName", "SpecifiedTradeProduct/BrandName"},
		{Extension{XML: `<ram:PackageQuantity unitCode="XPK">2</ram:PackageQuantity>`}, "PackageQuantity", "PackageQuantity"},
		{Extension{Attr: "listID", Value: "IB", Path: "ClassCode"}, "@listID", "ClassCode/@listID"},
		{Extension{Attr: "schemeID", Value: "X", Path: "ID[2]"}, "@schemeID", "ID[2]/@schemeID"},
	} {
		if got := c.e.Name(); got != c.name {
			t.Errorf("%+v: Name = %q, want %q", c.e, got, c.name)
		}
		if got := c.e.String(); got != c.path {
			t.Errorf("%+v: String = %q, want %q", c.e, got, c.path)
		}
	}
}

func TestUnmodeled(t *testing.T) {
	inv := &Invoice{}
	if inv.HasExtensions() || inv.Unmodeled() != nil {
		t.Fatal("empty invoice has extensions")
	}
	inv.Extensions = []Extension{{Path: "ExchangedDocument", XML: "<ram:Name>N</ram:Name>"}}
	inv.Seller.Extensions = []Extension{{XML: "<ram:RoleCode>SE</ram:RoleCode>"}}
	inv.ShipFrom = &Party{Extensions: []Extension{{Path: "DefinedTradeContact", XML: "<ram:TypeCode>X</ram:TypeCode>"}}}
	inv.Lines = []Line{
		{},
		{
			Extensions: []Extension{{Path: "SpecifiedTradeProduct", XML: "<ram:BrandName>Acme</ram:BrandName>"}},
			Item:       Item{Classifications: []Classification{{}, {Extensions: []Extension{{Path: "ClassCode", Attr: "listURI", Value: "u"}}}}},
		},
	}
	inv.PaymentTerms = []PaymentTerms{{Extensions: []Extension{{XML: "<ram:ApplicableTradePaymentDiscountTerms/>"}}}}
	if !inv.HasExtensions() {
		t.Fatal("HasExtensions = false")
	}
	want := []string{
		"ExchangedDocument/Name",
		"Lines[1]: SpecifiedTradeProduct/BrandName",
		"Lines[1].Item.Classifications[1]: ClassCode/@listURI",
		"Seller: RoleCode",
		"ShipFrom: DefinedTradeContact/TypeCode",
		"PaymentTerms[0]: ApplicableTradePaymentDiscountTerms",
	}
	if got := inv.Unmodeled(); !slices.Equal(got, want) {
		t.Errorf("Unmodeled =\n%q\nwant\n%q", got, want)
	}
}

// TestCalculateLogistics checks that logistics service charges (EXT) count as
// document level charges, in BT-108 and in the VAT breakdown, and that the
// extensions of the breakdown are kept.
func TestCalculateLogistics(t *testing.T) {
	d := MustDecimal
	ext := []Extension{{XML: "<ram:LineTotalBasisAmount>100.00</ram:LineTotalBasisAmount>"}}
	inv := &Invoice{
		Lines: []Line{{ID: "1", Quantity: d("1"), NetPrice: d("100.00"), VATCategoryCode: VATStandard, VATRate: d("20")}},
		AllowancesCharges: []AllowanceCharge{
			{Charge: true, Amount: d("5.00"), VATCategoryCode: VATStandard, VATRate: d("20")},
		},
		LogisticsCharges: []LogisticsCharge{
			{Description: "Freight", Amount: d("10.00"), VATCategoryCode: VATStandard, VATRate: d("20")},
			{Description: "Insurance", Amount: d("2.00"), VATCategoryCode: VATStandard, VATRate: d("10")},
		},
		VATBreakdown: []VATBreakdown{{CategoryCode: VATStandard, Rate: d("20"), Extensions: ext}},
	}
	inv.Calculate()
	tt := inv.Totals
	if tt.ChargeTotal.String() != "17.00" || tt.TaxBasisTotal.String() != "117.00" || tt.TaxTotal.String() != "23.20" {
		t.Errorf("BT-108 = %s, BT-109 = %s, BT-110 = %s", tt.ChargeTotal, tt.TaxBasisTotal, tt.TaxTotal)
	}
	if len(inv.VATBreakdown) != 2 {
		t.Fatalf("VAT breakdown = %+v", inv.VATBreakdown)
	}
	if b := inv.VATBreakdown[0]; b.TaxableAmount.String() != "115.00" || b.TaxAmount.String() != "23.00" || !slices.Equal(b.Extensions, ext) {
		t.Errorf("S 20 %% = %+v", b)
	}
	if b := inv.VATBreakdown[1]; b.TaxableAmount.String() != "2.00" || b.TaxAmount.String() != "0.20" {
		t.Errorf("S 10 %% = %+v", b)
	}
}
