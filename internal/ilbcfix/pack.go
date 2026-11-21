package ilbcfix

// File: pack.go — pack_bits.c.

func packBits(bitstream []uint16, b *iLBCBits, mode int) {
	p := 0

	bitstream[p] = uint16(b.Lsf[0]) << 10
	bitstream[p] |= uint16(b.Lsf[1]) << 3
	bitstream[p] |= uint16(b.Lsf[2]&0x70) >> 4
	p++

	bitstream[p] = uint16(b.Lsf[2]&0xF) << 12
	if mode == 20 {
		bitstream[p] |= uint16(b.StartIdx) << 10
		bitstream[p] |= uint16(b.StateFirst) << 9
		bitstream[p] |= uint16(b.IdxForMax) << 3
		bitstream[p] |= uint16(b.CbIndex[0]&0x70) >> 4
		p++

		bitstream[p] = uint16(b.CbIndex[0]&0xE) << 12
		bitstream[p] |= uint16(b.GainIndex[0]&0x18) << 8
		bitstream[p] |= uint16(b.GainIndex[1]&0x8) << 7
		bitstream[p] |= uint16(b.CbIndex[3]&0xFE) << 2
		bitstream[p] |= uint16(b.GainIndex[3]&0x10) >> 2
		bitstream[p] |= uint16(b.GainIndex[4]&0x8) >> 2
		bitstream[p] |= uint16(b.GainIndex[6]&0x10) >> 4
	} else {
		bitstream[p] |= uint16(b.Lsf[3]) << 6
		bitstream[p] |= uint16(b.Lsf[4]&0x7E) >> 1
		p++

		bitstream[p] = uint16(b.Lsf[4]&0x1) << 15
		bitstream[p] |= uint16(b.Lsf[5]) << 8
		bitstream[p] |= uint16(b.StartIdx) << 5
		bitstream[p] |= uint16(b.StateFirst) << 4
		bitstream[p] |= uint16(b.IdxForMax&0x3C) >> 2
		p++

		bitstream[p] = uint16(b.IdxForMax&0x3) << 14
		bitstream[p] |= uint16(b.CbIndex[0]&0x78) << 7
		bitstream[p] |= uint16(b.GainIndex[0]&0x10) << 5
		bitstream[p] |= uint16(b.GainIndex[1]&0x8) << 5
		bitstream[p] |= uint16(b.CbIndex[3] & 0xFC)
		bitstream[p] |= uint16(b.GainIndex[3]&0x10) >> 3
		bitstream[p] |= uint16(b.GainIndex[4]&0x8) >> 3
	}
	p++

	tmp := 0
	for k := 0; k < 3; k++ {
		bitstream[p] = 0
		for i := 15; i >= 0; i-- {
			bitstream[p] |= uint16((b.IdxVec[tmp]&0x4)>>2) << uint(i)
			tmp++
		}
		p++
	}

	if mode == 20 {
		bitstream[p] = 0
		for i := 15; i > 6; i-- {
			bitstream[p] |= uint16((b.IdxVec[tmp]&0x4)>>2) << uint(i)
			tmp++
		}
		bitstream[p] |= uint16(b.GainIndex[1]&0x4) << 4
		bitstream[p] |= uint16(b.GainIndex[3]&0xC) << 2
		bitstream[p] |= uint16(b.GainIndex[4]&0x4) << 1
		bitstream[p] |= uint16(b.GainIndex[6]&0x8) >> 1
		bitstream[p] |= uint16(b.GainIndex[7]&0xC) >> 2
	} else {
		bitstream[p] = 0
		for i := 15; i > 5; i-- {
			bitstream[p] |= uint16((b.IdxVec[tmp]&0x4)>>2) << uint(i)
			tmp++
		}
		bitstream[p] |= uint16(b.CbIndex[0]&0x6) << 3
		bitstream[p] |= uint16(b.GainIndex[0] & 0x8)
		bitstream[p] |= uint16(b.GainIndex[1] & 0x4)
		bitstream[p] |= uint16(b.CbIndex[3] & 0x2)
		bitstream[p] |= uint16(b.CbIndex[6]&0x80) >> 7
		p++

		bitstream[p] = uint16(b.CbIndex[6]&0x7E) << 9
		bitstream[p] |= uint16(b.CbIndex[9]&0xFE) << 2
		bitstream[p] |= uint16(b.CbIndex[12]&0xE0) >> 5
		p++

		bitstream[p] = uint16(b.CbIndex[12]&0x1E) << 11
		bitstream[p] |= uint16(b.GainIndex[3]&0xC) << 8
		bitstream[p] |= uint16(b.GainIndex[4]&0x6) << 7
		bitstream[p] |= uint16(b.GainIndex[6]&0x18) << 3
		bitstream[p] |= uint16(b.GainIndex[7]&0xC) << 2
		bitstream[p] |= uint16(b.GainIndex[9]&0x10) >> 1
		bitstream[p] |= uint16(b.GainIndex[10]&0x8) >> 1
		bitstream[p] |= uint16(b.GainIndex[12]&0x10) >> 3
		bitstream[p] |= uint16(b.GainIndex[13]&0x8) >> 3
	}
	p++

	tmp = 0
	for k := 0; k < 7; k++ {
		bitstream[p] = 0
		for i := 14; i >= 0; i -= 2 {
			bitstream[p] |= uint16(b.IdxVec[tmp]&0x3) << uint(i)
			tmp++
		}
		p++
	}

	if mode == 20 {
		bitstream[p] = uint16(b.IdxVec[56]&0x3) << 14
		bitstream[p] |= uint16(b.CbIndex[0]&1) << 13
		bitstream[p] |= uint16(b.CbIndex[1]) << 6
		bitstream[p] |= uint16(b.CbIndex[2]&0x7E) >> 1
		p++

		bitstream[p] = uint16(b.CbIndex[2]&0x1) << 15
		bitstream[p] |= uint16(b.GainIndex[0]&0x7) << 12
		bitstream[p] |= uint16(b.GainIndex[1]&0x3) << 10
		bitstream[p] |= uint16(b.GainIndex[2]) << 7
		bitstream[p] |= uint16(b.CbIndex[3]&0x1) << 6
		bitstream[p] |= uint16(b.CbIndex[4]&0x7E) >> 1
		p++

		bitstream[p] = uint16(b.CbIndex[4]&0x1) << 15
		bitstream[p] |= uint16(b.CbIndex[5]) << 8
		bitstream[p] |= uint16(b.CbIndex[6])
		p++

		bitstream[p] = uint16(b.CbIndex[7]) << 8
		bitstream[p] |= uint16(b.CbIndex[8])
		p++

		bitstream[p] = uint16(b.GainIndex[3]&0x3) << 14
		bitstream[p] |= uint16(b.GainIndex[4]&0x3) << 12
		bitstream[p] |= uint16(b.GainIndex[5]) << 9
		bitstream[p] |= uint16(b.GainIndex[6]&0x7) << 6
		bitstream[p] |= uint16(b.GainIndex[7]&0x3) << 4
		bitstream[p] |= uint16(b.GainIndex[8]) << 1
	} else {
		bitstream[p] = uint16(b.IdxVec[56]&0x3) << 14
		bitstream[p] |= uint16(b.IdxVec[57]&0x3) << 12
		bitstream[p] |= uint16(b.CbIndex[0]&1) << 11
		bitstream[p] |= uint16(b.CbIndex[1]) << 4
		bitstream[p] |= uint16(b.CbIndex[2]&0x78) >> 3
		p++

		bitstream[p] = uint16(b.CbIndex[2]&0x7) << 13
		bitstream[p] |= uint16(b.GainIndex[0]&0x7) << 10
		bitstream[p] |= uint16(b.GainIndex[1]&0x3) << 8
		bitstream[p] |= uint16(b.GainIndex[2]&0x7) << 5
		bitstream[p] |= uint16(b.CbIndex[3]&0x1) << 4
		bitstream[p] |= uint16(b.CbIndex[4]&0x78) >> 3
		p++

		bitstream[p] = uint16(b.CbIndex[4]&0x7) << 13
		bitstream[p] |= uint16(b.CbIndex[5]) << 6
		bitstream[p] |= uint16(b.CbIndex[6]&0x1) << 5
		bitstream[p] |= uint16(b.CbIndex[7]&0xF8) >> 3
		p++

		bitstream[p] = uint16(b.CbIndex[7]&0x7) << 13
		bitstream[p] |= uint16(b.CbIndex[8]) << 5
		bitstream[p] |= uint16(b.CbIndex[9]&0x1) << 4
		bitstream[p] |= uint16(b.CbIndex[10]&0xF0) >> 4
		p++

		bitstream[p] = uint16(b.CbIndex[10]&0xF) << 12
		bitstream[p] |= uint16(b.CbIndex[11]) << 4
		bitstream[p] |= uint16(b.CbIndex[12]&0x1) << 3
		bitstream[p] |= uint16(b.CbIndex[13]&0xE0) >> 5
		p++

		bitstream[p] = uint16(b.CbIndex[13]&0x1F) << 11
		bitstream[p] |= uint16(b.CbIndex[14]) << 3
		bitstream[p] |= uint16(b.GainIndex[3]&0x3) << 1
		bitstream[p] |= uint16(b.GainIndex[4] & 0x1)
		p++

		bitstream[p] = uint16(b.GainIndex[5]) << 13
		bitstream[p] |= uint16(b.GainIndex[6]&0x7) << 10
		bitstream[p] |= uint16(b.GainIndex[7]&0x3) << 8
		bitstream[p] |= uint16(b.GainIndex[8]) << 5
		bitstream[p] |= uint16(b.GainIndex[9]&0xF) << 1
		bitstream[p] |= uint16(b.GainIndex[10]&0x4) >> 2
		p++

		bitstream[p] = uint16(b.GainIndex[10]&0x3) << 14
		bitstream[p] |= uint16(b.GainIndex[11]) << 11
		bitstream[p] |= uint16(b.GainIndex[12]&0xF) << 7
		bitstream[p] |= uint16(b.GainIndex[13]&0x7) << 4
		bitstream[p] |= uint16(b.GainIndex[14]) << 1
	}
}
