package pdf

import (
	"encoding/binary"
	"hash/adler32"
	"math/bits"
	"slices"
	"sync"
)

// Deflate compresses data in zlib format (RFC 1950/1951) for a FlateDecode
// stream. It uses greedy LZ77 matching on a small hash table and, for each
// block, the smallest of a dynamic Huffman, fixed Huffman or stored
// encoding. Compression is close to compress/flate at its fastest level,
// but the encoder needs no large state (compress/flate allocates more than
// a megabyte per writer) and its output does not depend on the Go version.
func Deflate(data []byte) []byte {
	out := make([]byte, 0, len(data)/3+128)
	out = append(out, 0x78, 0x01) // zlib header: deflate, 32 KiB window, fastest
	tp := tokenPool.Get().(*[]uint32)
	e := encoder{w: bitWriter{out: out}, data: data, tokens: (*tp)[:0]}
	e.run()
	*tp = e.tokens[:0]
	tokenPool.Put(tp)
	out = e.w.flush()
	return binary.BigEndian.AppendUint32(out, adler32.Checksum(data))
}

// tokenPool recycles the token buffers of the encoder.
var tokenPool = sync.Pool{New: func() any { return new([]uint32) }}

// bitWriter writes deflate bits, least significant first.
type bitWriter struct {
	out  []byte
	bits uint64
	n    uint
}

func (w *bitWriter) write(v uint64, n uint) {
	w.bits |= v << w.n
	w.n += n
	if w.n >= 32 {
		w.out = binary.LittleEndian.AppendUint32(w.out, uint32(w.bits))
		w.bits >>= 32
		w.n -= 32
	}
}

// code writes a Huffman code from a table built by codes.
func (w *bitWriter) code(c uint32) {
	w.write(uint64(c>>8), uint(c&0xff))
}

// flush pads the last byte with zeros and returns the output.
func (w *bitWriter) flush() []byte {
	for w.n > 0 {
		w.out = append(w.out, byte(w.bits))
		w.bits >>= 8
		w.n -= min(w.n, 8)
	}
	w.bits, w.n = 0, 0
	return w.out
}

const (
	lzHashBits  = 13
	lzMinMatch  = 4
	lzMaxMatch  = 258
	lzWindow    = 32768
	blockTokens = 1 << 15
	matchFlag   = 1 << 31
)

func lzHash(u uint32) uint32 { return u * 0x1e35a7bd >> (32 - lzHashBits) }

// encoder turns the input into LZ77 tokens and writes them block by block.
// A token is a literal byte, or a match: matchFlag | lengthSymbol<<23 |
// lengthExtra<<18 | distanceCode<<13 | distanceExtra.
type encoder struct {
	w      bitWriter
	data   []byte
	tokens []uint32
	start  int         // input offset of the current block
	lf     [286]uint32 // literal/length symbol frequencies of the block
	df     [30]uint32  // distance code frequencies of the block
	extra  uint64      // extra bits of the block
}

// match appends a match token.
func (e *encoder) match(length, dist int) {
	s := lenSym[length]
	d := distCode(dist)
	e.lf[257+int(s)]++
	e.df[d]++
	e.extra += uint64(lenExtra[s]) + uint64(distExtr[d])
	e.tokens = append(e.tokens, matchFlag|uint32(s)<<23|uint32(length-int(lenBase[s]))<<18|
		uint32(d)<<13|uint32(dist-int(distBase[d])))
}

func (e *encoder) run() {
	data := e.data
	var table [1 << lzHashBits]int32 // position+1 of the last occurrence of a hash
	if want := min(blockTokens, len(data)/2+16); cap(e.tokens) < want {
		e.tokens = make([]uint32, 0, want)
	}
	i := 0
	for i+lzMinMatch <= len(data) {
		if len(e.tokens) >= blockTokens {
			e.block(i, false)
		}
		cur := binary.LittleEndian.Uint32(data[i:])
		h := lzHash(cur)
		cand := int(table[h]) - 1
		table[h] = int32(i + 1)
		if cand >= 0 && i-cand <= lzWindow && binary.LittleEndian.Uint32(data[cand:]) == cur {
			n := matchLen(data, cand+lzMinMatch, i+lzMinMatch, min(len(data)-i, lzMaxMatch)-lzMinMatch) + lzMinMatch
			e.match(n, i-cand)
			// Index the positions inside the match, so that later
			// repeats find the most recent occurrence.
			end := i + n
			for j := i + 1; j < end && j+lzMinMatch <= len(data); j++ {
				table[lzHash(binary.LittleEndian.Uint32(data[j:]))] = int32(j + 1)
			}
			i = end
			continue
		}
		e.tokens = append(e.tokens, uint32(data[i]))
		e.lf[data[i]]++
		i++
	}
	for ; i < len(data); i++ {
		e.tokens = append(e.tokens, uint32(data[i]))
		e.lf[data[i]]++
	}
	e.block(len(data), true)
}

// matchLen returns the number of equal bytes at a and b, at most max.
func matchLen(data []byte, a, b, max int) int {
	n := 0
	for n+8 <= max {
		x := binary.LittleEndian.Uint64(data[a+n:]) ^ binary.LittleEndian.Uint64(data[b+n:])
		if x != 0 {
			return n + bits.TrailingZeros64(x)>>3
		}
		n += 8
	}
	for n < max && data[a+n] == data[b+n] {
		n++
	}
	return n
}

// lenSym maps a match length (3 to 258) to its length symbol minus 257;
// distSym maps a distance to its distance code: distSym[d-1] for d <= 256,
// distSym[256+(d-1)>>7] above.
var lenSym, distSym = func() (ls [lzMaxMatch + 1]uint8, ds [512]uint8) {
	for s := 0; s < 29; s++ {
		end := lzMaxMatch + 1
		if s < 28 {
			end = int(lenBase[s+1])
		}
		for l := int(lenBase[s]); l < end; l++ {
			ls[l] = uint8(s)
		}
	}
	ls[lzMaxMatch] = 28
	for d := 0; d < 30; d++ {
		end := lzWindow + 1
		if d < 29 {
			end = int(distBase[d+1])
		}
		for x := int(distBase[d]); x < end; x++ {
			if x <= 256 {
				ds[x-1] = uint8(d)
			} else {
				ds[256+(x-1)>>7] = uint8(d)
			}
		}
	}
	return
}()

func distCode(dist int) uint8 {
	if dist <= 256 {
		return distSym[dist-1]
	}
	return distSym[256+(dist-1)>>7]
}

// Fixed Huffman code lengths (RFC 1951, 3.2.6). The literal/length code
// has 288 symbols: the two unused ones take part in the canonical code
// assignment.
var fixedLitLen, fixedDistLen = func() (l [288]uint8, d [30]uint8) {
	for s := range l {
		switch {
		case s < 144:
			l[s] = 8
		case s < 256:
			l[s] = 9
		case s < 280:
			l[s] = 7
		default:
			l[s] = 8
		}
	}
	for s := range d {
		d[s] = 5
	}
	return
}()

// block writes the tokens of the current block, which ends at input offset
// end, with the cheapest encoding.
func (e *encoder) block(end int, final bool) {
	lf, df, extra := &e.lf, &e.df, e.extra
	lf[256] = 1

	cost := func(f []uint32, l []uint8) (n uint64) {
		for i, x := range f {
			n += uint64(x) * uint64(l[i])
		}
		return n
	}
	fixedBits := 3 + cost(lf[:], fixedLitLen[:]) + cost(df[:], fixedDistLen[:]) + extra

	var ll [286]uint8
	var dl [30]uint8
	huffLengths(lf[:], ll[:], 15)
	huffLengths(df[:], dl[:], 15)
	h := dynamicHeader(ll[:], dl[:])
	dynBits := 3 + h.bits + cost(lf[:], ll[:]) + cost(df[:], dl[:]) + extra

	raw := e.data[e.start:end]
	chunks := uint64(len(raw)/65535 + 1)
	storedBits := chunks*(3+7+32) + 8*uint64(len(raw))

	switch {
	case storedBits < fixedBits && storedBits < dynBits:
		e.stored(raw, final)
	case fixedBits <= dynBits:
		e.header(final, 1)
		var lc [288]uint32
		var dc [30]uint32
		codes(fixedLitLen[:], lc[:])
		codes(fixedDistLen[:], dc[:])
		e.symbols(lc[:], dc[:])
	default:
		e.header(final, 2)
		h.write(&e.w)
		var lc [286]uint32
		var dc [30]uint32
		codes(ll[:], lc[:])
		codes(dl[:], dc[:])
		e.symbols(lc[:], dc[:])
	}
	e.tokens = e.tokens[:0]
	e.start = end
	e.lf, e.df, e.extra = [286]uint32{}, [30]uint32{}, 0
}

func (e *encoder) header(final bool, typ uint64) {
	f := uint64(0)
	if final {
		f = 1
	}
	e.w.write(f|typ<<1, 3)
}

// symbols writes the tokens and the end-of-block code. The bit buffer is
// kept in locals: it holds fewer than 32 bits between tokens, and a length
// or a distance with its extra bits takes at most 28.
func (e *encoder) symbols(lc, dc []uint32) {
	bits, n, out := e.w.bits, e.w.n, e.w.out
	for _, t := range e.tokens {
		if t&matchFlag == 0 {
			c := lc[t&0xff]
			bits |= uint64(c>>8) << n
			n += uint(c & 0xff)
		} else {
			s := t >> 23 & 31
			c := lc[257+s]
			bits |= (uint64(c>>8) | uint64(t>>18&31)<<(c&0xff)) << n
			n += uint(c&0xff) + uint(lenExtra[s])
			if n >= 32 {
				out = binary.LittleEndian.AppendUint32(out, uint32(bits))
				bits >>= 32
				n -= 32
			}
			d := t >> 13 & 31
			c = dc[d]
			bits |= (uint64(c>>8) | uint64(t&0x1fff)<<(c&0xff)) << n
			n += uint(c&0xff) + uint(distExtr[d])
		}
		if n >= 32 {
			out = binary.LittleEndian.AppendUint32(out, uint32(bits))
			bits >>= 32
			n -= 32
		}
	}
	e.w.bits, e.w.n, e.w.out = bits, n, out
	e.w.code(lc[256])
}

// stored writes raw as stored blocks.
func (e *encoder) stored(raw []byte, final bool) {
	for {
		n := min(len(raw), 65535)
		last := final && n == len(raw)
		e.header(last, 0)
		e.w.out = e.w.flush()
		e.w.out = binary.LittleEndian.AppendUint16(e.w.out, uint16(n))
		e.w.out = binary.LittleEndian.AppendUint16(e.w.out, ^uint16(n))
		e.w.out = append(e.w.out, raw[:n]...)
		raw = raw[n:]
		if len(raw) == 0 {
			return
		}
	}
}

// huffLengths computes length-limited Huffman code lengths for freq. At
// least two symbols get a code, so that every code is complete.
func huffLengths(freq []uint32, lengths []uint8, limit int) {
	var f [286]uint32
	copy(f[:], freq)
	f2 := f[:len(freq)]
	used := 0
	for _, x := range f2 {
		if x > 0 {
			used++
		}
	}
	for s := 0; used < 2; s++ {
		if f2[s] == 0 {
			f2[s] = 1
			used++
		}
	}
	for {
		if huffBuild(f2, lengths) <= limit {
			return
		}
		// Too deep: flatten the distribution and retry.
		for i, x := range f2 {
			if x > 0 {
				f2[i] = (x + 1) / 2
			}
		}
	}
}

// huffBuild computes Huffman code lengths and returns the longest.
func huffBuild(freq []uint32, lengths []uint8) int {
	var keys [286]uint64
	n := 0
	for s, x := range freq {
		lengths[s] = 0
		if x > 0 {
			keys[n] = uint64(x)<<16 | uint64(s)
			n++
		}
	}
	slices.Sort(keys[:n])
	// Two-queue construction: leaves 0..n-1 sorted by weight, internal
	// nodes n..2n-2 created in increasing weight order.
	var weight [2 * 286]uint64
	var parent [2 * 286]int16
	for i := 0; i < n; i++ {
		weight[i] = keys[i] >> 16
	}
	leaf, node := 0, n
	pick := func(k int) int {
		if leaf < n && (node >= k || weight[leaf] <= weight[node]) {
			leaf++
			return leaf - 1
		}
		node++
		return node - 1
	}
	for k := n; k < 2*n-1; k++ {
		a, b := pick(k), pick(k)
		weight[k] = weight[a] + weight[b]
		parent[a], parent[b] = int16(k), int16(k)
	}
	var depth [2 * 286]uint8
	maxLen := 0
	for k := 2*n - 3; k >= 0; k-- {
		depth[k] = depth[parent[k]] + 1
		if k < n {
			lengths[keys[k]&0xffff] = depth[k]
			maxLen = max(maxLen, int(depth[k]))
		}
	}
	return maxLen
}

// codes builds the canonical codes for lengths, bit-reversed and packed as
// code<<8 | length for bitWriter.code.
func codes(lengths []uint8, out []uint32) {
	var count [16]int
	for _, l := range lengths {
		count[l]++
	}
	count[0] = 0
	var next [16]uint32
	code := uint32(0)
	for l := 1; l < 16; l++ {
		code = (code + uint32(count[l-1])) << 1
		next[l] = code
	}
	for s, l := range lengths {
		if l == 0 {
			out[s] = 0
			continue
		}
		c := next[l]
		next[l]++
		out[s] = bits.Reverse32(c)>>(32-uint32(l))<<8 | uint32(l)
	}
}

// header is an encoded dynamic block header.
type header struct {
	nlit, ndist, ncl int
	cl               [19]uint8 // code length code lengths
	rle              []uint16  // symbol | extra<<5
	bits             uint64
}

// dynamicHeader run-length encodes the code lengths (RFC 1951, 3.2.7).
func dynamicHeader(ll, dl []uint8) *header {
	h := &header{nlit: 257, ndist: 1}
	for i := len(ll) - 1; i >= 257; i-- {
		if ll[i] != 0 {
			h.nlit = i + 1
			break
		}
	}
	for i := len(dl) - 1; i >= 1; i-- {
		if dl[i] != 0 {
			h.ndist = i + 1
			break
		}
	}
	all := make([]uint8, 0, h.nlit+h.ndist)
	all = append(append(all, ll[:h.nlit]...), dl[:h.ndist]...)
	var cf [19]uint32
	h.rle = make([]uint16, 0, len(all))
	for i := 0; i < len(all); {
		v := all[i]
		run := 1
		for i+run < len(all) && all[i+run] == v {
			run++
		}
		i += run
		if v == 0 {
			for run >= 11 {
				r := min(run, 138)
				h.rle = append(h.rle, 18|uint16(r-11)<<5)
				cf[18]++
				run -= r
			}
			if run >= 3 {
				h.rle = append(h.rle, 17|uint16(run-3)<<5)
				cf[17]++
				run = 0
			}
		} else {
			h.rle = append(h.rle, uint16(v))
			cf[v]++
			run--
			for run >= 3 {
				r := min(run, 6)
				h.rle = append(h.rle, 16|uint16(r-3)<<5)
				cf[16]++
				run -= r
			}
		}
		for ; run > 0; run-- {
			h.rle = append(h.rle, uint16(v))
			cf[v]++
		}
	}
	huffLengths(cf[:], h.cl[:], 7)
	h.ncl = 4
	for i := 18; i >= 4; i-- {
		if h.cl[clOrder[i]] != 0 {
			h.ncl = i + 1
			break
		}
	}
	h.bits = 5 + 5 + 4 + 3*uint64(h.ncl)
	for _, r := range h.rle {
		s := r & 31
		h.bits += uint64(h.cl[s])
		switch s {
		case 16:
			h.bits += 2
		case 17:
			h.bits += 3
		case 18:
			h.bits += 7
		}
	}
	return h
}

func (h *header) write(w *bitWriter) {
	w.write(uint64(h.nlit-257), 5)
	w.write(uint64(h.ndist-1), 5)
	w.write(uint64(h.ncl-4), 4)
	for i := 0; i < h.ncl; i++ {
		w.write(uint64(h.cl[clOrder[i]]), 3)
	}
	var cc [19]uint32
	codes(h.cl[:], cc[:])
	for _, r := range h.rle {
		s := r & 31
		w.code(cc[s])
		switch s {
		case 16:
			w.write(uint64(r>>5), 2)
		case 17:
			w.write(uint64(r>>5), 3)
		case 18:
			w.write(uint64(r>>5), 7)
		}
	}
}
