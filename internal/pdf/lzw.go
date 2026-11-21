package pdf

import "errors"

var errLZW = errors.New("pdf: LZWDecode: invalid code")

// lzwDecode decodes LZWDecode data (ISO 32000-1, 7.4.4): codes of 9 to 12
// bits, most significant bit first, 256 to clear the table and 257 to end
// the data. With earlyChange (the default, /EarlyChange 1), the code width
// grows one code early, as most encoders do. Data that ends without the
// end-of-data code is accepted.
func lzwDecode(src []byte, earlyChange bool) ([]byte, error) {
	const clear, eod = 256, 257
	var (
		prefix [4096]uint16
		suffix [4096]byte
		length [4096]int
	)
	for c := range 256 {
		suffix[c] = byte(c)
		length[c] = 1
	}
	early := 0
	if earlyChange {
		early = 1
	}
	out := make([]byte, 0, len(src)*3)
	// emit appends the string of code.
	emit := func(code int) error {
		n := length[code]
		if len(out)+n > maxDecoded {
			return errTooLarge
		}
		start := len(out)
		out = append(out, make([]byte, n)...)
		for i := start + n - 1; i >= start; i-- {
			out[i] = suffix[code]
			code = int(prefix[code])
		}
		return nil
	}
	next, width, prev := 258, 9, -1
	var bits uint32
	var nbits uint
	for i := 0; ; {
		for nbits < uint(width) {
			if i >= len(src) {
				return out, nil
			}
			bits = bits<<8 | uint32(src[i])
			i++
			nbits += 8
		}
		code := int(bits>>(nbits-uint(width))) & (1<<width - 1)
		nbits -= uint(width)
		switch {
		case code == clear:
			next, width, prev = 258, 9, -1
			continue
		case code == eod:
			return out, nil
		case prev < 0:
			if code > 255 {
				return out, errLZW
			}
			if err := emit(code); err != nil {
				return out, err
			}
			prev = code
			continue
		case code > next || (code == next && next >= 4096):
			return out, errLZW
		}
		start := len(out)
		if code == next {
			// The code being defined: the previous string and its first
			// byte.
			if err := emit(prev); err != nil {
				return out, err
			}
			out = append(out, out[start])
		} else if err := emit(code); err != nil {
			return out, err
		}
		if next < 4096 {
			prefix[next] = uint16(prev)
			suffix[next] = out[start]
			length[next] = length[prev] + 1
			next++
		}
		prev = code
		if next+early >= 1<<width && width < 12 {
			width++
		}
	}
}

// runLengthDecode decodes RunLengthDecode data (ISO 32000-1, 7.4.5).
func runLengthDecode(src []byte) ([]byte, error) {
	out := make([]byte, 0, len(src)*2)
	for i := 0; i < len(src); {
		n := int(src[i])
		i++
		switch {
		case n == 128:
			return out, nil
		case n < 128:
			end := min(i+n+1, len(src))
			out = append(out, src[i:end]...)
			i = end
		default:
			if i >= len(src) {
				return out, nil
			}
			for k := 0; k < 257-n; k++ {
				out = append(out, src[i])
			}
			i++
		}
		if len(out) > maxDecoded {
			return nil, errTooLarge
		}
	}
	return out, nil
}
