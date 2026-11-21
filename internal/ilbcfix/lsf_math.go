package ilbcfix

// File: lsf_math.go — 1:1 transcription of the LSF/LSP mathematics
// (interpolate.c, bw_expand.c, lsf_to_lsp.c, get_lsp_poly.c, lsf_to_poly.c,
// chebyshev.c, lsf_check.c, simple_lsf_dequant.c,
// lsf_interpolate_to_poly_dec.c, decoder_interpolate_lsf.c).

// interpolate performs out[i] = in1[i]*coef + (1-coef)*in2[i] (rounded).
// coef is Q14.
func interpolate(out, in1, in2 []int16, coef int16, length int) {
	var i int
	invcoef := w16(16384 - int32(coef)) // 16384 = 1.0 (Q14)
	for i = 0; i < length; i++ {
		out[i] = w16(shr(mul16(coef, in1[i])+mul16(invcoef, in2[i])+8192, 14))
	}
}

// bwExpand computes bandwidth-expanded LPC coefficients:
// out[i] = coef[i]*in[i] rounded, in/out Q12, coef Q15.
func bwExpand(out, in, coef []int16, length int) {
	out[0] = in[0]
	for i := 1; i < length; i++ {
		out[i] = w16(shr(mul16(coef[i], in[i])+16384, 15))
	}
}

// lsf2Lsp converts LSF (Q13, 0..pi) to LSP (Q15, -1..1) using the cosine
// lookup tables with a first-order derivative correction.
func lsf2Lsp(lsf []int16, lsp []int16, m int) {
	for i := 0; i < m; i++ {
		// 20861: 1.0/(2*pi) in Q17
		freq := w16(shr(int32(lsf[i])*20861, 15))
		k := int(freq >> 8)
		diff := int(freq & 0xff)
		if k > 63 {
			k = 63
		}
		tmpW32 := int32(kCosDerivative[k]) * int32(diff)
		lsp[i] = w16(int32(kCos[k]) + shr(tmpW32, 12))
	}
}

// getLspPoly builds the half-band polynomial f (Q24) from five LSP values
// (Q15). f must have six elements.
func getLspPoly(lsp []int16, f []int32) {
	var high, low int16
	// f[0] = 1.0 (Q24)
	f[0] = 16777216
	f[1] = w32(int64(mul16(lsp[0], -1024)))
	fp := 2
	lp := 2
	for i := 2; i <= 5; i++ {
		f[fp] = f[fp-2]
		for j := i; j > 1; j-- {
			high = w16(f[fp-1] >> 16)
			low = w16((f[fp-1] & 0xffff) >> 1)
			tmpW32 := 4*int32(high)*int32(lsp[lp]) +
				4*(shr(int32(low)*int32(lsp[lp]), 15))
			f[fp] += f[fp-2]
			f[fp] -= tmpW32
			fp--
		}
		f[fp] -= int32(lsp[lp]) << 10
		fp += i
		lp += 2
	}
}

// lsf2Poly converts LSF (Q13) to LPC predictor coefficients a (Q12, a[0]=4096).
func lsf2Poly(a []int16, lsf []int16) {
	var f [2][6]int32
	var lsp [LPCFilterOrder]int16
	lsf2Lsp(lsf, lsp[:], LPCFilterOrder)
	getLspPoly(lsp[:], f[0][:])
	getLspPoly(lsp[1:], f[1][:])
	for idx := 5; idx > 0; idx-- {
		f[0][idx] += f[0][idx-1]
		f[1][idx] -= f[1][idx-1]
	}
	a[0] = 4096
	a1 := 1
	a2 := LPCFilterOrder
	f1p := 1
	f2p := 1
	for i := 5; i > 0; i-- {
		tmpW32 := f[0][f1p] + f[1][f2p]
		a[a1] = w16(shr(tmpW32+4096, 13))
		tmpW32 = f[0][f1p] - f[1][f2p]
		a[a2] = w16(shr(tmpW32+4096, 13))
		a1++
		a2--
		f1p++
		f2p++
	}
}

// chebyshev evaluates the degree-4 Chebyshev polynomial with coefficients f
// (Q(-1) style as in constants.c) at point x (Q(-2) grid), result int16.
func chebyshev(x int16, f []int16) int16 {
	var b1High, b1Low int16
	var b2 int32
	b2 = 0x1000000 // b2 = 1.0 (Q23)
	tmp1W32 := shl16(x, 10) + shl(int32(f[1]), 14)
	for i := 2; i < 5; i++ {
		tmp2W32 := tmp1W32
		b1High = w16(tmp1W32 >> 16)
		b1Low = w16((tmp1W32 - int32(b1High)<<16) >> 1)
		tmp1W32 = (shl(int32(b1High)*int32(x)+shr(int32(b1Low)*int32(x), 15), 2)) -
			b2 + shl(int32(f[i]), 14)
		b2 = tmp2W32
	}
	b1High = w16(tmp1W32 >> 16)
	b1Low = w16((tmp1W32 - int32(b1High)<<16) >> 1)
	tmp1W32 = shl(int32(b1High)*int32(x), 1) +
		shl(shr(int32(b1Low)*int32(x), 15), 1) -
		b2 + shl(int32(f[5]), 13)
	if tmp1W32 > 33553408 {
		return word16Max
	}
	if tmp1W32 < -33554432 {
		return word16Min
	}
	return w16(shr(tmp1W32, 10))
}

// lsfCheck enforces LSF separation (50 Hz margin) and bounds. Returns 1 if
// anything changed.
func lsfCheck(lsf []int16, dim, noAn int) int {
	const eps = 319  // 0.039 in Q13 (50 Hz)
	const eps2 = 160 // eps/2 in Q13
	const maxLsf = 25723
	const minLsf = 82
	change := 0
	for n := 0; n < 2; n++ {
		for m := 0; m < noAn; m++ {
			for k := 0; k < dim-1; k++ {
				pos := m*dim + k
				if lsf[pos+1]-lsf[pos] < eps {
					if lsf[pos+1] < lsf[pos] {
						lsf[pos+1] = lsf[pos] + eps2
						lsf[pos] = lsf[pos+1] - eps2
					} else {
						lsf[pos] -= eps2
						lsf[pos+1] += eps2
					}
					change = 1
				}
				if lsf[pos] < minLsf {
					lsf[pos] = minLsf
					change = 1
				}
				if lsf[pos] > maxLsf {
					lsf[pos] = maxLsf
					change = 1
				}
			}
		}
	}
	return change
}

// simpleLsfDeq dequantizes LSF coefficients from the 3-split codebook.
func simpleLsfDeq(lsfdeq []int16, index []int16, lpcN int) {
	var i, j, pos, cbPos int
	pos = 0
	cbPos = 0
	for i = 0; i < LSFNSplit; i++ {
		for j = 0; j < int(kLsfDimCb[i]); j++ {
			lsfdeq[pos+j] = kLsfCb[cbPos+j+int(index[i])*int(kLsfDimCb[i])]
		}
		pos += int(kLsfDimCb[i])
		cbPos += int(kLsfSizeCb[i]) * int(kLsfDimCb[i])
	}
	if lpcN > 1 {
		pos = 0
		cbPos = 0
		for i = 0; i < LSFNSplit; i++ {
			for j = 0; j < int(kLsfDimCb[i]); j++ {
				lsfdeq[LPCFilterOrder+pos+j] = kLsfCb[cbPos+int(index[LSFNSplit+i])*int(kLsfDimCb[i])+j]
			}
			pos += int(kLsfDimCb[i])
			cbPos += int(kLsfSizeCb[i]) * int(kLsfDimCb[i])
		}
	}
}

// lsfInterpolate2PolyDec interpolates two LSF sets (coef Q14 weight of lsf1)
// and turns the result into LPC coefficients (Q12).
func lsfInterpolate2PolyDec(a, lsf1, lsf2 []int16, coef int16, length int) {
	var lsftmp [LPCFilterOrder]int16
	interpolate(lsftmp[:], lsf1, lsf2, coef, length)
	lsf2Poly(a, lsftmp[:])
}

// decoderInterpolateLsp fills syntdenum/weightdenum for all subframes by
// interpolating between the previous (old) and the current frame LSFs, and
// updates lsfdeqold. length is LPCFilterOrder.
func decoderInterpolateLsp(syntdenum, weightdenum, lsfdeq []int16, length int, dec *DecInst) {
	var lp [LPCFilterOrder + 1]int16
	lpLength := length + 1
	if dec.Mode == 30 {
		lsfInterpolate2PolyDec(lp[:], dec.LsfDeqOld[:], lsfdeq[:length], kLsfWeight30ms[0], length)
		copy(syntdenum, lp[:])
		bwExpand(weightdenum, lp[:], kLpcChirpSyntDenum[:], lpLength)
		pos := lpLength
		for i := 1; i < 6; i++ {
			lsfInterpolate2PolyDec(lp[:], lsfdeq[:length], lsfdeq[length:], kLsfWeight30ms[i], length)
			copy(syntdenum[pos:], lp[:])
			bwExpand(weightdenum[pos:], lp[:], kLpcChirpSyntDenum[:], lpLength)
			pos += lpLength
		}
	} else {
		pos := 0
		for i := 0; i < dec.NSub; i++ {
			lsfInterpolate2PolyDec(lp[:], dec.LsfDeqOld[:], lsfdeq[:length], kLsfWeight20ms[i], length)
			copy(syntdenum[pos:], lp[:])
			bwExpand(weightdenum[pos:], lp[:], kLpcChirpSyntDenum[:], lpLength)
			pos += lpLength
		}
	}
	if dec.Mode == 30 {
		copy(dec.LsfDeqOld[:], lsfdeq[length:])
	} else {
		copy(dec.LsfDeqOld[:], lsfdeq[:length])
	}
}
