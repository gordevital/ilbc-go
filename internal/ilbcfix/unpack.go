package ilbcfix

// File: unpack.go — 1:1 transcription of unpack_bits.c.
// bitstream is the byte-swapped (little-endian host) sequence of int16 words,
// as prepared by swapBytes; it must hold at least NoOfWords(mode) entries.

// unpackBits decodes the parameters from the bitstream. Returns the last
// ("empty frame") bit.
func unpackBits(bitstream []int16, bits *iLBCBits, mode int) int16 {
	u := func(p int) uint16 { return uint16(bitstream[p]) }
	var p int

	// First int16
	w := u(0)
	bits.Lsf[0] = int16(w >> 10) // bits 0..5
	bits.Lsf[1] = int16((w >> 3) & 0x7F)
	bits.Lsf[2] = int16((w & 0x7) << 4)
	p++

	// Second int16
	w = u(p)
	bits.Lsf[2] |= int16((w >> 12) & 0xF)
	if mode == 20 {
		bits.StartIdx = int((w >> 10) & 0x3)
		bits.StateFirst = int16((w >> 9) & 0x1)
		bits.IdxForMax = int((w >> 3) & 0x3F)
		bits.CbIndex[0] = int16((w & 0x7) << 4)
		p++
		w = u(p)
		bits.CbIndex[0] |= int16((w >> 12) & 0xE)
		bits.GainIndex[0] = int16((w >> 8) & 0x18)
		bits.GainIndex[1] = int16((w >> 7) & 0x8)
		bits.CbIndex[3] = int16((w >> 2) & 0xFE)
		bits.GainIndex[3] = int16((w << 2) & 0x10)
		bits.GainIndex[4] = int16((w << 2) & 0x8)
		bits.GainIndex[6] = int16((w << 4) & 0x10)
	} else {
		bits.Lsf[3] = int16((w >> 6) & 0x3F)
		bits.Lsf[4] = int16((w << 1) & 0x7E)
		p++
		w = u(p)
		bits.Lsf[4] |= int16((w >> 15) & 0x1)
		bits.Lsf[5] = int16((w >> 8) & 0x7F)
		bits.StartIdx = int((w >> 5) & 0x7)
		bits.StateFirst = int16((w >> 4) & 0x1)
		bits.IdxForMax = int((w << 2) & 0x3C)
		p++
		w = u(p)
		bits.IdxForMax |= int((w >> 14) & 0x3)
		bits.CbIndex[0] = int16((w >> 7) & 0x78)
		bits.GainIndex[0] = int16((w >> 5) & 0x10)
		bits.GainIndex[1] = int16((w >> 5) & 0x8)
		bits.CbIndex[3] = int16(w & 0xFC)
		bits.GainIndex[3] = int16((w << 3) & 0x10)
		bits.GainIndex[4] = int16((w << 3) & 0x8)
	}

	// Class 2 bits of ULP
	p++
	idxPos := 0
	for k := 0; k < 3; k++ {
		w = u(p)
		for i := 15; i >= 0; i-- {
			bits.IdxVec[idxPos] = int16(((w >> uint(i)) << 2) & 0x4)
			idxPos++
		}
		p++
	}

	if mode == 20 {
		w = u(p)
		for i := 15; i > 6; i-- {
			bits.IdxVec[idxPos] = int16(((w >> uint(i)) << 2) & 0x4)
			idxPos++
		}
		bits.GainIndex[1] |= int16((w >> 4) & 0x4)
		bits.GainIndex[3] |= int16((w >> 2) & 0xC)
		bits.GainIndex[4] |= int16((w >> 1) & 0x4)
		bits.GainIndex[6] |= int16((w << 1) & 0x8)
		bits.GainIndex[7] = int16((w << 2) & 0xC)
	} else {
		w = u(p)
		for i := 15; i > 5; i-- {
			bits.IdxVec[idxPos] = int16(((w >> uint(i)) << 2) & 0x4)
			idxPos++
		}
		bits.CbIndex[0] |= int16((w >> 3) & 0x6)
		bits.GainIndex[0] |= int16(w & 0x8)
		bits.GainIndex[1] |= int16(w & 0x4)
		bits.CbIndex[3] |= int16(w & 0x2)
		bits.CbIndex[6] = int16((w << 7) & 0x80)
		p++
		w = u(p)
		bits.CbIndex[6] |= int16((w >> 9) & 0x7E)
		bits.CbIndex[9] = int16((w >> 2) & 0xFE)
		bits.CbIndex[12] = int16((w << 5) & 0xE0)
		p++
		w = u(p)
		bits.CbIndex[12] |= int16((w >> 11) & 0x1E)
		bits.GainIndex[3] |= int16((w >> 8) & 0xC)
		bits.GainIndex[4] |= int16((w >> 7) & 0x6)
		bits.GainIndex[6] = int16((w >> 3) & 0x18)
		bits.GainIndex[7] = int16((w >> 2) & 0xC)
		bits.GainIndex[9] = int16((w << 1) & 0x10)
		bits.GainIndex[10] = int16((w << 1) & 0x8)
		bits.GainIndex[12] = int16((w << 3) & 0x10)
		bits.GainIndex[13] = int16((w << 3) & 0x8)
	}

	p++
	// Class 3 bits of ULP
	idxPos = 0
	for k := 0; k < 7; k++ {
		w = u(p)
		for i := 14; i >= 0; i -= 2 {
			bits.IdxVec[idxPos] |= int16(w>>uint(i)) & 0x3
			idxPos++
		}
		p++
	}

	if mode == 20 {
		w = u(p)
		bits.IdxVec[56] |= int16((w >> 14) & 0x3)
		bits.CbIndex[0] |= int16((w >> 13) & 0x1)
		bits.CbIndex[1] = int16((w >> 6) & 0x7F)
		bits.CbIndex[2] = int16((w << 1) & 0x7E)
		p++
		w = u(p)
		bits.CbIndex[2] |= int16((w >> 15) & 0x1)
		bits.GainIndex[0] |= int16((w >> 12) & 0x7)
		bits.GainIndex[1] |= int16((w >> 10) & 0x3)
		bits.GainIndex[2] = int16((w >> 7) & 0x7)
		bits.CbIndex[3] |= int16((w >> 6) & 0x1)
		bits.CbIndex[4] = int16((w << 1) & 0x7E)
		p++
		w = u(p)
		bits.CbIndex[4] |= int16((w >> 15) & 0x1)
		bits.CbIndex[5] = int16((w >> 8) & 0x7F)
		bits.CbIndex[6] = int16(w & 0xFF)
		p++
		w = u(p)
		bits.CbIndex[7] = int16(w >> 8)
		bits.CbIndex[8] = int16(w & 0xFF)
		p++
		w = u(p)
		bits.GainIndex[3] |= int16((w >> 14) & 0x3)
		bits.GainIndex[4] |= int16((w >> 12) & 0x3)
		bits.GainIndex[5] = int16((w >> 9) & 0x7)
		bits.GainIndex[6] |= int16((w >> 6) & 0x7)
		bits.GainIndex[7] |= int16((w >> 4) & 0x3)
		bits.GainIndex[8] = int16((w >> 1) & 0x7)
	} else {
		w = u(p)
		bits.IdxVec[56] |= int16((w >> 14) & 0x3)
		bits.IdxVec[57] |= int16((w >> 12) & 0x3)
		bits.CbIndex[0] |= int16((w >> 11) & 1)
		bits.CbIndex[1] = int16((w >> 4) & 0x7F)
		bits.CbIndex[2] = int16((w << 3) & 0x78)
		p++
		w = u(p)
		bits.CbIndex[2] |= int16((w >> 13) & 0x7)
		bits.GainIndex[0] |= int16((w >> 10) & 0x7)
		bits.GainIndex[1] |= int16((w >> 8) & 0x3)
		bits.GainIndex[2] = int16((w >> 5) & 0x7)
		bits.CbIndex[3] |= int16((w >> 4) & 0x1)
		bits.CbIndex[4] = int16((w << 3) & 0x78)
		p++
		w = u(p)
		bits.CbIndex[4] |= int16((w >> 13) & 0x7)
		bits.CbIndex[5] = int16((w >> 6) & 0x7F)
		bits.CbIndex[6] |= int16((w >> 5) & 0x1)
		bits.CbIndex[7] = int16((w << 3) & 0xF8)
		p++
		w = u(p)
		bits.CbIndex[7] |= int16((w >> 13) & 0x7)
		bits.CbIndex[8] = int16((w >> 5) & 0xFF)
		bits.CbIndex[9] |= int16((w >> 4) & 0x1)
		bits.CbIndex[10] = int16((w << 4) & 0xF0)
		p++
		w = u(p)
		bits.CbIndex[10] |= int16((w >> 12) & 0xF)
		bits.CbIndex[11] = int16((w >> 4) & 0xFF)
		bits.CbIndex[12] |= int16((w >> 3) & 0x1)
		bits.CbIndex[13] = int16((w << 5) & 0xE0)
		p++
		w = u(p)
		bits.CbIndex[13] |= int16((w >> 11) & 0x1F)
		bits.CbIndex[14] = int16((w >> 3) & 0xFF)
		bits.GainIndex[3] |= int16((w >> 1) & 0x3)
		bits.GainIndex[4] |= int16(w & 0x1)
		p++
		w = u(p)
		bits.GainIndex[5] = int16((w >> 13) & 0x7)
		bits.GainIndex[6] |= int16((w >> 10) & 0x7)
		bits.GainIndex[7] |= int16((w >> 8) & 0x3)
		bits.GainIndex[8] = int16((w >> 5) & 0x7)
		bits.GainIndex[9] |= int16((w >> 1) & 0xF)
		bits.GainIndex[10] |= int16((w << 2) & 0x4)
		p++
		w = u(p)
		bits.GainIndex[10] |= int16((w >> 14) & 0x3)
		bits.GainIndex[11] = int16((w >> 11) & 0x7)
		bits.GainIndex[12] |= int16((w >> 7) & 0xF)
		bits.GainIndex[13] |= int16((w >> 4) & 0x7)
		bits.GainIndex[14] = int16((w >> 1) & 0x7)
	}

	// Last bit is 1 iff "empty" frame
	if bitstream[p]&1 != 0 {
		return 1
	}
	return 0
}
