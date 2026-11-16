package invoice

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Decimal is a fixed-point decimal number for amounts, quantities, prices and
// rates. It keeps the number of decimals it was created with ("12.50" stays
// "12.50"), so values read from an invoice are written back unchanged.
//
// The zero value is an absent value: IsSet reports false and String returns
// "". Arithmetic treats an absent operand as zero and always returns a set
// value.
type Decimal struct {
	coef  int64
	scale uint8
	set   bool
}

const maxScale = 18

var pow10 = [maxScale + 1]int64{
	1, 10, 100, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9,
	1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18,
}

// NewDecimal returns coef × 10^-scale. NewDecimal(1250, 2) is 12.50.
// The scale is clamped to [0, 18].
func NewDecimal(coef int64, scale int) Decimal {
	if scale < 0 {
		scale = 0
	}
	if scale > maxScale {
		scale = maxScale
	}
	return Decimal{coef: coef, scale: uint8(scale), set: true}
}

// DecimalFromInt returns the integer i with no decimals.
func DecimalFromInt(i int64) Decimal { return Decimal{coef: i, set: true} }

// ErrDecimalSyntax is returned by ParseDecimal for malformed input.
var ErrDecimalSyntax = errors.New("invalid decimal")

// ParseDecimal parses an xs:decimal lexical value: optional sign, digits,
// optional decimal point and digits ("-12.50", "+3", ".5", "7."). Leading and
// trailing white space is ignored. At most 18 significant digits are
// accepted.
func ParseDecimal(s string) (Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Decimal{}, fmt.Errorf("%w: empty", ErrDecimalSyntax)
	}
	neg := false
	switch s[0] {
	case '-':
		neg = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	var (
		coef   uint64
		scale  int
		digits int
		dot    bool
		any    bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && !dot:
			dot = true
		case c >= '0' && c <= '9':
			any = true
			if coef == 0 && c == '0' && !dot {
				continue // leading zero
			}
			digits++
			if digits > maxScale {
				return Decimal{}, fmt.Errorf("%w: %q has more than %d digits", ErrDecimalSyntax, s, maxScale)
			}
			coef = coef*10 + uint64(c-'0')
			if dot {
				scale++
			}
		default:
			return Decimal{}, fmt.Errorf("%w: %q", ErrDecimalSyntax, s)
		}
	}
	if !any {
		return Decimal{}, fmt.Errorf("%w: %q", ErrDecimalSyntax, s)
	}
	v := int64(coef)
	if neg {
		v = -v
	}
	return Decimal{coef: v, scale: uint8(scale), set: true}, nil
}

// MustDecimal is like ParseDecimal but panics on malformed input. It is meant
// for constants and tests.
func MustDecimal(s string) Decimal {
	d, err := ParseDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

// IsSet reports whether d holds a value.
func (d Decimal) IsSet() bool { return d.set }

// IsZero reports whether d is zero or absent.
func (d Decimal) IsZero() bool { return d.coef == 0 }

// Sign returns -1, 0 or +1.
func (d Decimal) Sign() int {
	switch {
	case d.coef < 0:
		return -1
	case d.coef > 0:
		return 1
	}
	return 0
}

// Scale returns the number of decimals.
func (d Decimal) Scale() int { return int(d.scale) }

// Coef returns the unscaled value: d = Coef() × 10^-Scale().
func (d Decimal) Coef() int64 { return d.coef }

// String returns the value with its own number of decimals ("12.50", "-3",
// "0.075"), or "" when d is absent.
func (d Decimal) String() string {
	if !d.set {
		return ""
	}
	return string(d.Append(nil))
}

// Append appends the textual form of d to b, as String would.
func (d Decimal) Append(b []byte) []byte {
	if !d.set {
		return b
	}
	if d.scale == 0 {
		return strconv.AppendInt(b, d.coef, 10)
	}
	u := uint64(d.coef)
	if d.coef < 0 {
		b = append(b, '-')
		u = uint64(-d.coef)
	}
	var buf [24]byte
	digits := strconv.AppendUint(buf[:0], u, 10)
	sc := int(d.scale)
	if len(digits) <= sc {
		b = append(b, '0', '.')
		for i := len(digits); i < sc; i++ {
			b = append(b, '0')
		}
		return append(b, digits...)
	}
	b = append(b, digits[:len(digits)-sc]...)
	b = append(b, '.')
	return append(b, digits[len(digits)-sc:]...)
}

// StringFixed returns the value rounded (half away from zero) or padded to
// exactly scale decimals.
func (d Decimal) StringFixed(scale int) string { return d.Round(scale).String() }

// Float64 returns the nearest float64. Use it for display only.
func (d Decimal) Float64() float64 {
	f, _ := strconv.ParseFloat(d.String(), 64)
	return f
}

// Neg returns -d.
func (d Decimal) Neg() Decimal {
	if d.coef == math.MinInt64 {
		return fromBig(new(big.Int).Neg(d.big()), int(d.scale))
	}
	return Decimal{coef: -d.coef, scale: d.scale, set: true}
}

// Abs returns |d|.
func (d Decimal) Abs() Decimal {
	if d.coef < 0 {
		return d.Neg()
	}
	return Decimal{coef: d.coef, scale: d.scale, set: true}
}

// Add returns d + e, with the larger of the two scales.
func (d Decimal) Add(e Decimal) Decimal {
	a, b, sc, ok := align(d, e)
	if ok {
		if c, ok := add64(a, b); ok {
			return Decimal{coef: c, scale: uint8(sc), set: true}
		}
	}
	x, y, sc := alignBig(d, e)
	return fromBig(x.Add(x, y), sc)
}

// Sub returns d - e, with the larger of the two scales.
func (d Decimal) Sub(e Decimal) Decimal {
	a, b, sc, ok := align(d, e)
	if ok {
		if c, ok := add64(a, -b); ok && b != math.MinInt64 {
			return Decimal{coef: c, scale: uint8(sc), set: true}
		}
	}
	x, y, sc := alignBig(d, e)
	return fromBig(x.Sub(x, y), sc)
}

// Mul returns d × e. The scale of the result is the sum of the scales, capped
// at 18 decimals (the product is then rounded half away from zero).
func (d Decimal) Mul(e Decimal) Decimal {
	sc := int(d.scale) + int(e.scale)
	if sc <= maxScale {
		if c, ok := mul64(d.coef, e.coef); ok {
			return Decimal{coef: c, scale: uint8(sc), set: true}
		}
	}
	return fromBig(new(big.Int).Mul(d.big(), e.big()), sc)
}

// Div returns d / e rounded half away from zero to scale decimals. It returns
// an absent value when e is zero.
func (d Decimal) Div(e Decimal, scale int) Decimal {
	if e.coef == 0 {
		return Decimal{}
	}
	if scale < 0 {
		scale = 0
	}
	if scale > maxScale {
		scale = maxScale
	}
	// d/e = (dc × 10^-ds) / (ec × 10^-es); result coef = dc × 10^(scale+es-ds) / ec.
	num := d.big()
	den := e.big()
	shift := scale + int(e.scale) - int(d.scale)
	if shift >= 0 {
		num.Mul(num, pow10Big(shift))
	} else {
		den.Mul(den, pow10Big(-shift))
	}
	return fromBig(divRound(num, den), scale)
}

// Round returns d rounded half away from zero to scale decimals. When d has
// fewer decimals, it is padded with zeros: MustDecimal("3").Round(2) is 3.00.
func (d Decimal) Round(scale int) Decimal {
	return d.round(scale, false)
}

// RoundHalfCeil returns d rounded to scale decimals, halves going towards
// positive infinity, as the XPath round() function used by the official
// schematrons does: -2.5 becomes -2.
func (d Decimal) RoundHalfCeil(scale int) Decimal {
	return d.round(scale, true)
}

func (d Decimal) round(scale int, ceil bool) Decimal {
	if scale < 0 {
		scale = 0
	}
	if scale > maxScale {
		scale = maxScale
	}
	cur := int(d.scale)
	if scale >= cur {
		if c, ok := mul64(d.coef, pow10[scale-cur]); ok {
			return Decimal{coef: c, scale: uint8(scale), set: true}
		}
		return fromBig(new(big.Int).Mul(d.big(), pow10Big(scale-cur)), scale)
	}
	div := pow10[cur-scale]
	q, r := d.coef/div, d.coef%div
	half := div / 2
	switch {
	case r > 0 && (r > half || r == half):
		q++
	case r < 0 && (-r > half || (-r == half && !ceil)):
		q--
	}
	return Decimal{coef: q, scale: uint8(scale), set: true}
}

// Cmp compares d and e numerically and returns -1, 0 or +1. Absent values
// compare as zero.
func (d Decimal) Cmp(e Decimal) int {
	a, b, _, ok := align(d, e)
	if ok {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		}
		return 0
	}
	x, y, _ := alignBig(d, e)
	return x.Cmp(y)
}

// Equal reports whether d and e are numerically equal (1.5 equals 1.50).
func (d Decimal) Equal(e Decimal) bool { return d.Cmp(e) == 0 }

// MarshalText implements encoding.TextMarshaler.
func (d Decimal) MarshalText() ([]byte, error) { return d.Append(nil), nil }

// UnmarshalText implements encoding.TextUnmarshaler. Empty text gives an
// absent value.
func (d *Decimal) UnmarshalText(b []byte) error {
	if len(strings.TrimSpace(string(b))) == 0 {
		*d = Decimal{}
		return nil
	}
	v, err := ParseDecimal(string(b))
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// Sum returns the sum of the values; the result is set even for no value.
func Sum(values ...Decimal) Decimal {
	s := Decimal{set: true}
	for _, v := range values {
		s = s.Add(v)
	}
	return s
}

func align(d, e Decimal) (a, b int64, scale int, ok bool) {
	switch {
	case d.scale == e.scale:
		return d.coef, e.coef, int(d.scale), true
	case d.scale < e.scale:
		a, ok = mul64(d.coef, pow10[e.scale-d.scale])
		return a, e.coef, int(e.scale), ok
	default:
		b, ok = mul64(e.coef, pow10[d.scale-e.scale])
		return d.coef, b, int(d.scale), ok
	}
}

func alignBig(d, e Decimal) (x, y *big.Int, scale int) {
	x, y = d.big(), e.big()
	switch {
	case d.scale < e.scale:
		x.Mul(x, pow10Big(int(e.scale-d.scale)))
		return x, y, int(e.scale)
	case d.scale > e.scale:
		y.Mul(y, pow10Big(int(d.scale-e.scale)))
	}
	return x, y, int(d.scale)
}

func (d Decimal) big() *big.Int { return big.NewInt(d.coef) }

func pow10Big(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// fromBig builds a Decimal from coef × 10^-scale, reducing the scale (with
// rounding half away from zero) until the coefficient fits in an int64. Values
// beyond ±9.2e18 saturate.
func fromBig(coef *big.Int, scale int) Decimal {
	ten := big.NewInt(10)
	for scale > maxScale || !coef.IsInt64() {
		if scale == 0 {
			if coef.Sign() < 0 {
				return Decimal{coef: math.MinInt64 + 1, set: true}
			}
			return Decimal{coef: math.MaxInt64, set: true}
		}
		coef = divRound(coef, ten)
		scale--
	}
	return Decimal{coef: coef.Int64(), scale: uint8(scale), set: true}
}

// divRound returns num/den rounded half away from zero.
func divRound(num, den *big.Int) *big.Int {
	q, r := new(big.Int).QuoRem(num, den, new(big.Int))
	if r.Sign() == 0 {
		return q
	}
	r2 := new(big.Int).Abs(r)
	r2.Lsh(r2, 1)
	if r2.Cmp(new(big.Int).Abs(den)) >= 0 {
		if (num.Sign() < 0) != (den.Sign() < 0) {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q
}

func add64(a, b int64) (int64, bool) {
	c := a + b
	if (c > a) != (b > 0) {
		return 0, false
	}
	return c, true
}

func mul64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	c := a * b
	if (c < 0) != ((a < 0) != (b < 0)) || c/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
		return 0, false
	}
	return c, true
}
