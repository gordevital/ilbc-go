package ilbcfix

// File: enc_math.go — 1:1 transcription of the encoder-side fixed-point
// mathematics: sort_sq.c, vq3.c, vq4.c, split_vq.c, simple_lsf_quant.c,
// simple_interpolate_lsf.c, lsf_interpolate_to_poly_enc.c,
// simple_lpc_analysis.c, lpc_encode.c, window32_w32.c, poly_to_lsp.c,
// poly_to_lsf.c, lsp_to_lsf.c, energy_inverse.c, gain_quant.c,
// frame_classify.c, state_search.c, abs_quant.c, abs_quant_loop.c,
// index_conv_enc.c.

func sortSq(x int16, cb []int16, cbSize int16) (xq, index int16) {
	if x <= cb[0] {
		return cb[0], 0
	}
	i := 0
	for x > cb[i] && i < int(cbSize)-1 {
		i++
	}
	if int32(x) > (int32(cb[i])+int32(cb[i-1])+1)>>1 {
		return cb[i], int16(i)
	}
	return cb[i-1], int16(i - 1)
}

func vq3(xq, cb, x []int16, nCb int16) int16 {
	pos := 0
	mindist := word32Max
	minindex := int16(0)
	for j := int16(0); j < nCb; j++ {
		tmp := w16(int32(x[0]) - int32(cb[pos]))
		dist := mul16(tmp, tmp)
		for i := 1; i < 3; i++ {
			tmp = w16(int32(x[i]) - int32(cb[pos+i]))
			dist += mul16(tmp, tmp)
		}
		if dist < mindist {
			mindist = dist
			minindex = j
		}
		pos += 3
	}
	for i := 0; i < 3; i++ {
		xq[i] = cb[int(minindex)*3+i]
	}
	return minindex
}

func vq4(xq, cb, x []int16, nCb int16) int16 {
	pos := 0
	mindist := word32Max
	minindex := int16(0)
	for j := int16(0); j < nCb; j++ {
		tmp := w16(int32(x[0]) - int32(cb[pos]))
		dist := mul16(tmp, tmp)
		for i := 1; i < 4; i++ {
			tmp = w16(int32(x[i]) - int32(cb[pos+i]))
			dist += mul16(tmp, tmp)
		}
		if dist < mindist {
			mindist = dist
			minindex = j
		}
		pos += 4
	}
	for i := 0; i < 4; i++ {
		xq[i] = cb[int(minindex)*4+i]
	}
	return minindex
}

func splitVq(qX, index, x, cb, dim, cbsize []int16) {
	qPtr, xPtr, cbPtr := 0, 0, 0
	index[0] = vq3(qX[qPtr:], cb[cbPtr:], x[xPtr:], cbsize[0])
	qPtr += 3
	xPtr += 3
	cbPtr += int(dim[0]) * int(cbsize[0])

	index[1] = vq3(qX[qPtr:], cb[cbPtr:], x[xPtr:], cbsize[1])
	qPtr += 3
	xPtr += 3
	cbPtr += int(dim[1]) * int(cbsize[1])

	index[2] = vq4(qX[qPtr:], cb[cbPtr:], x[xPtr:], cbsize[2])
}

func simpleLsfQ(lsfdeq, index, lsf []int16, lpcN int) {
	splitVq(lsfdeq, index, lsf, kLsfCb[:], kLsfDimCb[:], kLsfSizeCb[:])
	if lpcN == 2 {
		splitVq(lsfdeq[LPCFilterOrder:], index[LSFNSplit:], lsf[LPCFilterOrder:],
			kLsfCb[:], kLsfDimCb[:], kLsfSizeCb[:])
	}
}

func lsfInterpolate2PolyEnc(a, lsf1, lsf2 []int16, coef int16, length int) {
	var lsftmp [LPCFilterOrder]int16
	interpolate(lsftmp[:], lsf1, lsf2, coef, length)
	lsf2Poly(a, lsftmp[:])
}

func simpleInterpolateLsf(syntdenum, weightdenum, lsf, lsfdeq, lsfold, lsfdeqold []int16, length int, enc *EncInst) {
	lpLength := length + 1
	var lp [LPCFilterOrder + 1]int16
	if enc.Mode == 30 {
		lsfInterpolate2PolyEnc(lp[:], lsfdeqold, lsfdeq, kLsfWeight30ms[0], length)
		copy(syntdenum, lp[:])
		lsfInterpolate2PolyEnc(lp[:], lsfold, lsf, kLsfWeight30ms[0], length)
		bwExpand(weightdenum, lp[:], kLpcChirpWeightDenum[:], lpLength)

		pos := lpLength
		for i := 1; i < enc.NSub; i++ {
			lsfInterpolate2PolyEnc(lp[:], lsfdeq, lsfdeq[length:], kLsfWeight30ms[i], length)
			copy(syntdenum[pos:], lp[:])
			lsfInterpolate2PolyEnc(lp[:], lsf, lsf[length:], kLsfWeight30ms[i], length)
			bwExpand(weightdenum[pos:], lp[:], kLpcChirpWeightDenum[:], lpLength)
			pos += lpLength
		}
		copy(lsfold, lsf[length:length+LPCFilterOrder])
		copy(lsfdeqold, lsfdeq[length:length+LPCFilterOrder])
	} else {
		pos := 0
		for i := 0; i < enc.NSub; i++ {
			lsfInterpolate2PolyEnc(lp[:], lsfdeqold, lsfdeq, kLsfWeight20ms[i], length)
			copy(syntdenum[pos:], lp[:])
			lsfInterpolate2PolyEnc(lp[:], lsfold, lsf, kLsfWeight20ms[i], length)
			bwExpand(weightdenum[pos:], lp[:], kLpcChirpWeightDenum[:], lpLength)
			pos += lpLength
		}
		copy(lsfold, lsf[:length])
		copy(lsfdeqold, lsfdeq[:length])
	}
}

func window32W32(z, x []int32, y []int32, n int) {
	leftShifts := int16(normW32(x[0]))
	vectorBitShiftW32(x, x, -leftShifts)
	for i := 0; i < n; i++ {
		xHi := int16(x[i] >> 16)
		yHi := int16(y[i] >> 16)
		xLow := int16((x[i] - (int32(xHi) << 16)) >> 1)
		yLow := int16((y[i] - (int32(yHi) << 16)) >> 1)

		temp := (int32(xHi) * int32(yHi)) << 1
		temp += int32(xHi) * int32(yLow) >> 14
		z[i] = temp + (int32(xLow)*int32(yHi))>>14
	}
	vectorBitShiftW32(z, z, leftShifts)
}

func simpleLpcAnalysis(lsf, data []int16, enc *EncInst) {
	var A [LPCFilterOrder + 1]int16
	var R [LPCFilterOrder + 1]int32
	var windowedData [BlockLMax]int16
	var rc [LPCFilterOrder]int16

	is := LPCLookBack + BlockLMax - enc.BlockL
	copy(enc.LpcBuffer[is:], data[:enc.BlockL])

	for k := 0; k < enc.LpcN; k++ {
		if k < enc.LpcN-1 {
			elementwiseVectorMult(windowedData[:], enc.LpcBuffer[:], kLpcWin[:], 0, BlockLMax, 15)
		} else {
			elementwiseVectorMult(windowedData[:], enc.LpcBuffer[LPCLookBack:], kLpcAsymWin[:], 0, BlockLMax, 15)
		}

		Rres, _ := autoCorrelation(windowedData[:], BlockLMax, LPCFilterOrder)
		copy(R[:], Rres)
		window32W32(R[:], R[:], kLpcLagWin[:], LPCFilterOrder+1)

		stability := levinsonDurbin(R[:], A[:], rc[:], LPCFilterOrder)
		if stability != 1 {
			A[0] = 4096
			memSetW16(A[:], 1, LPCFilterOrder, 0)
		}

		bwExpand(A[:], A[:], kLpcChirpSyntDenum[:], LPCFilterOrder+1)
		poly2Lsf(lsf[k*LPCFilterOrder:], A[:])
	}

	is = LPCLookBack + BlockLMax - enc.BlockL
	copy(enc.LpcBuffer[:is], enc.LpcBuffer[LPCLookBack+BlockLMax-is:])
}

func lpcEncode(syntdenum, weightdenum, lsfIndex, data []int16, enc *EncInst) {
	var lsf [LPCFilterOrder * LpcNMax]int16
	var lsfdeq [LPCFilterOrder * LpcNMax]int16

	simpleLpcAnalysis(lsf[:], data, enc)
	simpleLsfQ(lsfdeq[:], lsfIndex, lsf[:], enc.LpcN)
	lsfCheck(lsfdeq[:], LPCFilterOrder, enc.LpcN)
	simpleInterpolateLsf(syntdenum, weightdenum, lsf[:], lsfdeq[:],
		enc.LsfOld[:], enc.LsfDeqOld[:], LPCFilterOrder, enc)
}

func poly2Lsp(a, lsp, oldLsp []int16) {
	var f [2][6]int16
	f[0][0] = 1024
	f[1][0] = 1024
	for i := 0; i < 5; i++ {
		f[0][i+1] = w16(((int32(a[i+1]) + int32(a[10-i])) >> 2) - int32(f[0][i]))
		f[1][i+1] = w16(((int32(a[i+1]) - int32(a[10-i])) >> 2) + int32(f[1][i]))
	}

	fiSelect := 0
	foundFreqs := 0
	xlow := kCosGrid[0]
	ylow := chebyshev(xlow, f[fiSelect][:])
	for j := 1; j < CosGridPoints && foundFreqs < 10; j++ {
		xhigh := xlow
		yhigh := ylow
		xlow = kCosGrid[j]
		ylow = chebyshev(xlow, f[fiSelect][:])

		if int32(ylow)*int32(yhigh) <= 0 {
			var xmid, ymid int16
			for i := 0; i < 4; i++ {
				xmid = w16(int32(xlow>>1) + int32(xhigh>>1))
				ymid = chebyshev(xmid, f[fiSelect][:])
				if int32(ylow)*int32(ymid) <= 0 {
					yhigh = ymid
					xhigh = xmid
				} else {
					ylow = ymid
					xlow = xmid
				}
			}
			x := w16(int32(xhigh) - int32(xlow))
			y := w16(int32(yhigh) - int32(ylow))
			var xint int16
			if y == 0 {
				xint = xlow
			} else {
				sign := y
				y = w16(splAbsW16(y))
				shifts := int16(normW32(int32(y)) - 16)
				y = w16(shl(int32(y), int32(shifts)))
				y = w16(divW32W16(536838144, y))
				tmpW32 := (int32(x) * int32(y)) >> uint(19-int(shifts))
				y = w16(tmpW32 & 0xFFFF)
				if sign < 0 {
					y = w16(-int32(y))
				}
				tmpW32 = (int32(ylow) * int32(y)) >> 10
				xint = w16(int32(xlow) - int32(w16(tmpW32&0xFFFF)))
			}
			lsp[foundFreqs] = xint
			foundFreqs++
			if foundFreqs < 10 {
				xlow = xint
				fiSelect = (fiSelect + 1) & 1
				ylow = chebyshev(xlow, f[fiSelect][:])
			}
		}
	}
	if foundFreqs < 10 {
		copy(lsp[:10], oldLsp[:10])
	}
}

func lsp2Lsf(lsp, lsf []int16, m int) {
	k := 63
	for i := m - 1; i >= 0; i-- {
		for (int32(kCos[k])-int32(lsp[i])) < 0 && k > 0 {
			k--
		}
		diff := w16(int32(lsp[i]) - int32(kCos[k]))
		tmp := w16((int32(kAcosDerivative[k]) * int32(diff)) >> 11)
		freq := w16((int32(k) << 9) + int32(tmp))
		lsf[i] = w16((int32(freq) * 25736) >> 15)
	}
}

func poly2Lsf(lsf, a []int16) {
	var lsp [10]int16
	poly2Lsp(a, lsp[:], kLspMean[:])
	lsp2Lsf(lsp[:], lsf, 10)
}

func energyInverse(energy []int16, noOfEnergies int) {
	const nom = int32(0x1FFFFFFF)
	for i := 0; i < noOfEnergies; i++ {
		if energy[i] < 16384 {
			energy[i] = 16384
		}
	}
	for i := 0; i < noOfEnergies; i++ {
		energy[i] = w16(divW32W16(nom, energy[i]))
	}
}

func gainQuant(gain, maxIn int16, stage int16) (qgain, index int16) {
	scale := maxIn
	if scale < 1638 {
		scale = 1638
	}
	cb := kGain[stage]
	cblen := int16(32 >> uint(stage))
	noChecks := int(4 - stage)

	gainW32 := shl(int32(gain), 14)

	loc := int(cblen >> 1)
	noMoves := loc
	cbPtr := loc
	for i := noChecks; i > 0; i-- {
		noMoves >>= 1
		measure1 := mul16(scale, cb[cbPtr]) - gainW32
		if measure1 < 0 {
			cbPtr += noMoves
			loc += noMoves
		} else {
			cbPtr -= noMoves
			loc -= noMoves
		}
	}

	measure1 := mul16(scale, cb[cbPtr])
	if gainW32 > measure1 {
		measure2 := mul16(scale, cb[cbPtr+1])
		if measure2-gainW32 < gainW32-measure1 {
			loc++
		}
	} else {
		measure2 := mul16(scale, cb[cbPtr-1])
		if gainW32-measure2 <= measure1-gainW32 {
			loc--
		}
	}
	if loc > int(cblen)-1 {
		loc = int(cblen) - 1
	}
	index = int16(loc)
	qgain = w16((mul16(scale, cb[loc]) + 8192) >> 14)
	return qgain, index
}

func frameClassify(enc *EncInst, residual []int16) int {
	max := maxAbsValueW16(residual, enc.BlockL)
	scale := getSizeInBits(uint32(int32(max)*int32(max))) - 24
	scale1 := scale
	if scale1 < 0 {
		scale1 = 0
	}

	ssqEn := make([]int32, NSubMax-1)
	ssqPtr := 2
	for n := enc.NSub - 1; n > 0; n-- {
		ssqEn[enc.NSub-1-n] = dotProductWithScale(residual[ssqPtr:], residual[ssqPtr:], 76, int(scale1))
		ssqPtr += 40
	}

	maxW32 := maxValueW32(ssqEn, enc.NSub-1)
	scale = getSizeInBits(uint32(maxW32)) - 20
	scale1 = scale
	if scale1 < 0 {
		scale1 = 0
	}

	ssqPtr = 0
	if enc.Mode == 20 {
		ssqPtr = 1
	}
	idx := 0
	for n := enc.NSub - 1; n > 0; n-- {
		ssqEn[idx] = mulW32(ssqEn[idx]>>uint(scale1), int32(kStartSequenceEnrgWin[ssqPtr]))
		ssqPtr++
		idx++
	}

	return maxIndexW32(ssqEn, enc.NSub-1) + 1
}

func stateSearch(enc *EncInst, bits *iLBCBits, residual, syntDenum, weightDenum []int16) {
	var numerator [LPCFilterOrder + 1]int16
	var residualLongVec [2*StateShortLen30ms + LPCFilterOrder]int16
	var sampleMa [2 * StateShortLen30ms]int16
	sampleAr := residualLongVec[LPCFilterOrder:]
	shortLen := enc.StateShortLen

	max := maxAbsValueW16(residual, shortLen)
	scaleRes := getSizeInBits(uint32(max)) - 12
	if scaleRes < 0 {
		scaleRes = 0
	}

	for i := 0; i < LPCFilterOrder+1; i++ {
		numerator[i] = w16(shr16(syntDenum[LPCFilterOrder-i], scaleRes))
	}

	copy(residualLongVec[LPCFilterOrder:], residual[:shortLen])
	memSetW16(residualLongVec[:], LPCFilterOrder+shortLen, shortLen, 0)
	memSetW16(residualLongVec[:], 0, LPCFilterOrder, 0)

	filterMAFastQ12(residualLongVec[:], LPCFilterOrder, sampleMa[:], 0,
		numerator[:], LPCFilterOrder+1, shortLen+LPCFilterOrder)
	memSetW16(sampleMa[:], shortLen+LPCFilterOrder, shortLen-LPCFilterOrder, 0)
	filterARFastQ12(sampleMa[:], residualLongVec[:], LPCFilterOrder,
		syntDenum, LPCFilterOrder+1, 2*shortLen)

	for k := 0; k < shortLen; k++ {
		sampleAr[k] = w16(int32(sampleAr[k]) + int32(sampleAr[k+shortLen]))
	}

	maxVal := maxAbsValueW16(sampleAr[:shortLen], shortLen)
	var maxValsq int32
	if (int32(maxVal) << uint(scaleRes)) < 23170 {
		maxValsq = shl(mul16(maxVal, maxVal), 2+2*scaleRes)
	} else {
		maxValsq = word32Max
	}

	index := 0
	for i := 0; i < 63; i++ {
		if maxValsq >= kChooseFrgQuant[i] {
			index = i + 1
		} else {
			break
		}
	}
	bits.IdxForMax = index

	scale := kScale[index]
	var shift int16
	if index < 27 {
		shift = 4
	} else {
		shift = 9
	}

	scaleVectorWithSat(sampleAr[:shortLen], sampleAr[:shortLen], scale, shortLen, int16(int32(shift)-scaleRes))
	absQuant(enc, bits, sampleAr[:shortLen], weightDenum)
}

func absQuant(enc *EncInst, bits *iLBCBits, in, weightDenum []int16) {
	var syntOutBuf [LPCFilterOrder + StateShortLen30ms]int16
	var inWeightedVec [StateShortLen30ms + LPCFilterOrder]int16

	memSetW16(syntOutBuf[:], 0, LPCFilterOrder+StateShortLen30ms, 0)
	memSetW16(inWeightedVec[:], 0, LPCFilterOrder, 0)

	var quantLen [2]int
	if bits.StateFirst != 0 {
		quantLen[0] = SubL
		quantLen[1] = enc.StateShortLen - SubL
	} else {
		quantLen[0] = enc.StateShortLen - SubL
		quantLen[1] = SubL
	}

	filterARFastQ12(in, inWeightedVec[:], LPCFilterOrder, weightDenum,
		LPCFilterOrder+1, quantLen[0])
	filterARFastQ12(in[quantLen[0]:], inWeightedVec[:], LPCFilterOrder+quantLen[0],
		weightDenum[LPCFilterOrder+1:], LPCFilterOrder+1, quantLen[1])

	absQuantLoop(syntOutBuf[:], LPCFilterOrder, inWeightedVec[:], LPCFilterOrder,
		weightDenum, quantLen, bits.IdxVec[:])
}

func absQuantLoop(syntOut []int16, syntOff int, inWeighted []int16, inOff int,
	weightDenum []int16, quantLen [2]int, idxVec []int16) {
	idx := 0
	w := 0
	for k1 := 0; k1 < 2; k1++ {
		for k2 := 0; k2 < quantLen[k1]; k2++ {
			filterARFastQ12(syntOut[syntOff:], syntOut, syntOff, weightDenum[w:],
				LPCFilterOrder+1, 1)

			toQW32 := int32(inWeighted[inOff]) - int32(syntOut[syntOff])
			toQ32 := toQW32 << 2
			if toQ32 > 32767 {
				toQ32 = 32767
			} else if toQ32 < -32768 {
				toQ32 = -32768
			}

			var index int16
			var xq int16
			if toQW32 < -7577 {
				index = 0
			} else if toQW32 > 8151 {
				index = 7
			} else {
				xq, index = sortSq(int16(toQ32), kStateSq3[:], 8)
				_ = xq
			}

			idxVec[idx] = index
			idx++

			tmp16a := w16((int32(kStateSq3[index]) + 2) >> 2)
			syntOut[syntOff] = w16(int32(tmp16a) + int32(inWeighted[inOff]) - toQW32)

			syntOff++
			inOff++
		}
		w += LPCFilterOrder + 1
	}
}

func indexConvEnc(index []int16) {
	for k := 4; k < 6; k++ {
		if index[k] >= 108 && index[k] < 172 {
			index[k] -= 64
		} else if index[k] >= 236 {
			index[k] -= 128
		}
	}
}
