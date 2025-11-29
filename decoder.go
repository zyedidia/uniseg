package uniseg

import "unicode/utf8"

type Decoder interface {
	DecodeRuneAt(off int) (rune, int)
	DecodeRuneBefore(off int) (rune, int)
	Slice(start, end int) []byte
	Len() int
}

func Decode(p []byte) (rune, []rune, int) {
	cur, _, _, _ := FirstGraphemeCluster(p, -1)
	return toRunes(cur)
}

func DecodeAt(d Decoder, off int) (rune, []rune, int, int) {
	if off < 0 {
		off = 0
	}
	size, width, _ := FirstGraphemeClusterDecoder(d, off, -1)
	r, combc, sz := toRunes(d.Slice(off, off+size))
	return r, combc, sz, width
}

func DecodeLast(p []byte) (rune, []rune, int) {
	size := lastGrapheme(p)
	return toRunes(p[len(p)-size:])
}

func DecodeBefore(d Decoder, off int) (rune, []rune, int) {
	if off < 0 {
		return 0, nil, 0
	}
	size, _ := lastGraphemeDec(d, off)
	return toRunes(d.Slice(off-size, off))
}

func lastGraphemeDec(d Decoder, off int) (size, width int) {
	for off-size > 0 {
		sz, ok := lastGraphemeSimpleDec(d, off-size)
		size += sz
		if ok {
			break
		}
	}
	return lastGraphemeFullDec(d, off-size, off)
}

func lastGraphemeSimpleDec(d Decoder, off int) (size int, ok bool) {
	r, sz := d.DecodeRuneBefore(off)
	size += sz
	switch property(graphemeCodePoints, r) {
	case prLF:
		r, sz := d.DecodeRuneBefore(off - sz)
		if r == '\r' {
			return size + sz, true
		}
		return size, true
	case prCR:
		return size, true
	case prControl:
		return size, true
	}
	return size, false
}

func lastGraphemeFullDec(d Decoder, from, to int) (size, width int) {
	state := -1
	for from < to {
		size, width, state = FirstGraphemeClusterDecoder(d, from, state)
		from += size
	}
	return
}

func lastGrapheme(b []byte) int {
	var size int
	for len(b)-size > 0 {
		sz, ok := lastGraphemeSimple(b[:len(b)-size])
		size += sz
		if ok {
			break
		}
	}
	return lastGraphemeFull(b[len(b)-size:])
}

func lastGraphemeSimple(b []byte) (size int, ok bool) {
	r, sz := utf8.DecodeLastRune(b)
	size += sz
	switch property(graphemeCodePoints, r) {
	case prLF:
		r, sz := utf8.DecodeLastRune(b[:len(b)-sz])
		if r == '\r' {
			return size + sz, true
		}
		return size, true
	case prCR:
		return size, true
	case prControl:
		return size, true
	}
	return size, false
}

func lastGraphemeFull(b []byte) (width int) {
	state := -1
	for len(b) > 0 {
		var cur []byte
		cur, b, _, state = FirstGraphemeCluster(b, state)
		width = len(cur)
	}
	return
}

func DecodeInString(p string) (rune, []rune, int) {
	cur, _, _, _ := FirstGraphemeClusterInString(p, -1)
	return toRunesString(cur)
}

func toRunes(g []byte) (rune, []rune, int) {
	var size int
	r, sz := utf8.DecodeRune(g)
	if sz == len(g) {
		return r, nil, sz
	}
	size += sz
	g = g[sz:]
	var combc []rune
	for len(g) > 0 {
		r, sz := utf8.DecodeRune(g)
		combc = append(combc, r)
		size += sz
		g = g[sz:]
	}
	return r, combc, size
}

func toRunesString(g string) (rune, []rune, int) {
	var size int
	r, sz := utf8.DecodeRuneInString(g)
	if sz == len(g) {
		return r, nil, sz
	}
	size += sz
	g = g[sz:]
	var combc []rune
	for len(g) > 0 {
		r, sz := utf8.DecodeRuneInString(g)
		combc = append(combc, r)
		size += sz
		g = g[sz:]
	}
	return r, combc, size
}

func FirstGraphemeClusterDecoder(b Decoder, off int, state int) (size, width, newState int) {
	// An empty byte slice returns nothing.
	if off >= b.Len() {
		return
	}

	// Extract the first rune.
	r, length := b.DecodeRuneAt(off)
	if b.Len()-off <= length { // If we're already past the end, there is nothing else to parse.
		var prop int
		if state < 0 {
			prop = property(graphemeCodePoints, r)
		} else {
			prop = state >> shiftGraphemePropState
		}
		return length, runeWidth(r, prop), grAny | (prop << shiftGraphemePropState)
	}

	// If we don't know the state, determine it now.
	var firstProp int
	if state < 0 {
		state, firstProp, _ = transitionGraphemeState(state, r)
	} else {
		firstProp = state >> shiftGraphemePropState
	}
	width += runeWidth(r, firstProp)

	// Transition until we find a boundary.
	for {
		var (
			prop     int
			boundary bool
		)

		r, l := b.DecodeRuneAt(off + length)
		state, prop, boundary = transitionGraphemeState(state&maskGraphemeState, r)

		if boundary {
			return length, width, state | (prop << shiftGraphemePropState)
		}

		if r == vs16 {
			width = 2
		} else if firstProp != prExtendedPictographic && firstProp != prRegionalIndicator && firstProp != prL {
			width += runeWidth(r, prop)
		} else if firstProp == prExtendedPictographic {
			if r == vs15 {
				width = 1
			} else {
				width = 2
			}
		}

		length += l
		if b.Len() <= length {
			return length, width, grAny | (prop << shiftGraphemePropState)
		}
	}
}
