package ilbcfix

import "fmt"

// File: enhancer.go — enhancer.c, get_sync_seq.c, nearest_neighbor.c,
// refiner.c, enh_upsample.c, my_corr.c, smooth.c, smooth_out_data.c.

// nearestNeighbor mirrors WebRtcIlbcfix_NearestNeighbor.
func nearestNeighbor(array []int, value, arlength int) int {
	index := 0
	minDiff := int(^uint(0) >> 1)
	for i := 0; i < arlength; i++ {
		diff := array[i] - value
		if diff < 0 {
			diff = -diff
		}
		if diff < minDiff {
			index = i
			minDiff = diff
		}
	}
	return index
}

// myCorr mirrors WebRtcIlbcfix_MyCorr.
func myCorr(corr []int32, seq1 []int16, dim1 int, seq2 []int16, dim2 int) {
	max1 := uint32(maxAbsValueW16(seq1, dim1)) + 1
	max2 := uint32(maxAbsValueW16(seq2, dim2)) + 1
	rightShift := int(33 - countLeadingZeros64(uint64(max1*max2)*uint64(dim2)))
	if rightShift < 0 {
		rightShift = 0
	}
	loops := dim1 - dim2 + 1
	crossCorrelation(corr, seq2, seq1, 0, dim2, loops, rightShift, 1)
}

// enhUpsample mirrors WebRtcIlbcfix_EnhUpsample.
func enhUpsample(useq1 []int32, seq1 []int16) {
	for j := 0; j < EnhUPS0; j++ {
		poly := kEnhPolyPhaser[j][:]
		for i := 2; i <= 4; i++ {
			var v int32
			for k := 0; k <= i; k++ {
				v += int32(seq1[i-k]) * int32(poly[1+k])
			}
			useq1[(i-2)*EnhUPS0+j] = v
		}
	}
	for j := 0; j < EnhUPS0; j++ {
		poly := kEnhPolyPhaser[j][:]
		for i := 1; i <= 2; i++ {
			var v int32
			for k := 0; k <= 4-i; k++ {
				v += int32(seq1[4-k]) * int32(poly[i+1+k])
			}
			useq1[12+(i-1)*EnhUPS0+j] = v
		}
	}
}

// refiner mirrors WebRtcIlbcfix_Refiner; it returns the updated start pos.
func refiner(idata []int16, idatal, centerStartPos, estSegPos int,
	surround []int16, gain int16) int {

	var filt [7]int16
	var corrVecUps [EnhCorrDim * EnhUPS0]int32
	var corrVecTemp [EnhCorrDim]int32
	var vect [EnhVECTL]int16
	var corrVec [EnhCorrDim]int16

	estSegPosRounded := (estSegPos - 2) >> 2

	searchSegStartPos := 0
	if estSegPosRounded >= EnhSlop {
		searchSegStartPos = estSegPosRounded - EnhSlop
	}
	searchSegEndPos := estSegPosRounded + EnhSlop
	if searchSegEndPos+EnhBlockL >= idatal {
		searchSegEndPos = idatal - EnhBlockL - 1
	}
	corrdim := searchSegEndPos + 1 - searchSegStartPos

	myCorr(corrVecTemp[:], idata[searchSegStartPos:],
		corrdim+EnhBlockL-1, idata[centerStartPos:], EnhBlockL)

	maxtemp := maxAbsValueW32(corrVecTemp[:], corrdim)
	scalefact := getSizeInBits(uint32(maxtemp)) - 15
	if scalefact > 0 {
		for i := 0; i < corrdim; i++ {
			corrVec[i] = int16(shr(corrVecTemp[i], scalefact))
		}
	} else {
		for i := 0; i < corrdim; i++ {
			corrVec[i] = int16(corrVecTemp[i])
		}
	}
	for i := corrdim; i < EnhCorrDim; i++ {
		corrVec[i] = 0
	}

	enhUpsample(corrVecUps[:], corrVec[:])

	tloc := maxIndexW32(corrVecUps[:], EnhUPS0*corrdim)

	updStartPos := searchSegStartPos*4 + tloc + 4

	tloc2 := (tloc + 3) >> 2

	if EnhFL0 > searchSegStartPos+tloc2 {
		st := EnhFL0 - searchSegStartPos - tloc2
		memSetW16(vect[:], 0, st, 0)
		copy(vect[st:], idata[:EnhVECTL-st])
	} else {
		st := searchSegStartPos + tloc2 - EnhFL0
		if st+EnhVECTL > idatal {
			en := st + EnhVECTL - idatal
			copy(vect[:], idata[st:st+EnhVECTL-en])
			memSetW16(vect[:], EnhVECTL-en, en, 0)
		} else {
			copy(vect[:], idata[st:st+EnhVECTL])
		}
	}

	poly := kEnhPolyPhaser[tloc2*EnhUPS0-tloc][:]
	for i := 0; i < 7; i++ {
		filt[6-i] = poly[i]
	}

	filterMAFastQ12(vect[:], 6, vect[:], 0, filt[:], EnhFLOMult2Plus1, EnhBlockL)

	addAffineVectorToVector(surround, vect[:], gain, 32768, 16, EnhBlockL)

	return updStartPos
}

// getSyncSeq mirrors WebRtcIlbcfix_GetSyncSeq.
func getSyncSeq(idata []int16, idatal, centerStartPos int, period []int,
	plocs []int, periodl, hl int, surround []int16) {

	var lagBlock [2*EnhHL + 1]int
	var blockStartPos [2*EnhHL + 1]int
	var plocs2 [EnhPlocsL]int

	centerEndPos := centerStartPos + EnhBlockL - 1

	if StageDebug {
		ps := ""
		for i, v := range period {
			ps += fmt.Sprintf(" p%d=%d", i, v)
		}
		stagePrint(fmt.Sprintf("GS block=%d idatal=%d%s", centerStartPos, idatal, ps))
	}

	lagBlock[hl] = nearestNeighbor(plocs, 2*(centerStartPos+centerEndPos), periodl)
	blockStartPos[hl] = 4 * centerStartPos

	for q := hl; q > 0; q-- {
		qq := q - 1
		periodQ := period[lagBlock[q]]
		if blockStartPos[q] < periodQ+4*EnhOverhang {
			break
		}
		blockStartPos[qq] = blockStartPos[q] - periodQ

		value := blockStartPos[qq] + 4*EnhBlockLHalf
		if value > periodQ {
			value -= periodQ
		} else {
			value = 0
		}
		lagBlock[qq] = nearestNeighbor(plocs, value, periodl)

		blockStartPos[qq] = refiner(idata, idatal, centerStartPos,
			blockStartPos[qq], surround, kEnhWt[qq])
	}

	for i := 0; i < periodl; i++ {
		plocs2[i] = plocs[i] - period[i]
	}

	for q := hl + 1; q <= 2*hl; q++ {
		lagBlock[q] = nearestNeighbor(plocs2[:], blockStartPos[q-1]+4*EnhBlockLHalf, periodl)
		blockStartPos[q] = blockStartPos[q-1] + period[lagBlock[q]]

		if blockStartPos[q]+4*(EnhBlockL+EnhOverhang) < 4*idatal {
			blockStartPos[q] = refiner(idata, idatal, centerStartPos,
				blockStartPos[q], surround, kEnhWt[2*hl-q])
		}
	}
}

// smoothOdata mirrors WebRtcIlbcfix_Smooth_odata.
func smoothOdata(odata, psseq, surround []int16, C int16) int32 {
	for i := 0; i < 80; i++ {
		odata[i] = int16((int32(C)*int32(surround[i]) + 1024) >> 11)
	}
	var errs int32
	for i := 0; i < 80; i++ {
		err := int16((int32(psseq[i]) - int32(odata[i])) >> 3)
		errs += int32(err) * int32(err)
	}
	return errs
}

// smooth mirrors WebRtcIlbcfix_Smooth.
func smooth(odata, current, surround []int16) {
	var w00, w10, w11, endiff, crit int32
	var w00prim, w10prim, w11DivW00 int32
	var w11prim int16
	var bitsw00, bitsw10, bitsw11 int16
	var B_W32, denom, num int32
	var scale, scale1, scale2 int16
	var A, B, C, denomW16 int16

	max1 := uint32(maxAbsValueW16(current, EnhBlockL)) + 1
	max2 := uint32(maxAbsValueW16(surround, EnhBlockL)) + 1
	max12 := max1
	if max2 > max12 {
		max12 = max2
	}
	scale = int16((33) - countLeadingZeros64(uint64(max12*max12)*EnhBlockL))
	if scale < 0 {
		scale = 0
	}

	w00 = dotProductWithScale(current, current, EnhBlockL, int(scale))
	w11 = dotProductWithScale(surround, surround, EnhBlockL, int(scale))
	w10 = dotProductWithScale(surround, current, EnhBlockL, int(scale))

	if w00 < 0 {
		w00 = word32Max
	}
	if w11 < 0 {
		w11 = word32Max
	}

	bitsw00 = int16(getSizeInBits(uint32(w00)))
	bitsw11 = int16(getSizeInBits(uint32(w11)))
	bitsw10 = int16(getSizeInBits(uint32(splAbsW32(w10))))
	scale1 = 31 - bitsw00
	scale2 = 15 - bitsw11

	if scale2 > scale1-16 {
		scale2 = scale1 - 16
	} else {
		scale1 = scale2 + 16
	}

	w00prim = shl(w00, int32(scale1))
	w11prim = int16(shiftW32(w11, int32(scale2)))

	if w11prim > 64 {
		endiff = shl(divW32W16(w00prim, w11prim), 6)
		C = int16(sqrtFloor(endiff))
	} else {
		C = 1
	}

	errs := smoothOdata(odata, current, surround, C)

	if StageDebug {
		od := ""
		for i := 0; i < 8; i++ {
			od += fmt.Sprintf(" od[%d]=%d", i, odata[i])
		}
		stagePrint(fmt.Sprintf("SM w00=%d w11=%d w10=%d sc=%d b00=%d b11=%d b10=%d s1=%d s2=%d w00p=%d w11p=%d C=%d errs=%d%s",
			w00, w11, w10, scale, bitsw00, bitsw11, bitsw10, scale1, scale2, w00prim, w11prim, C, errs, od))
		bwLen := len(current)
		if bwLen > 80 {
			bwLen = 80
		}
		stagePrint("BW:", current[:bwLen])
	}

	if (6 - scale + scale1) > 31 {
		crit = 0
	} else {
		crit = shiftW32(mulW32(enhA0, shr(w00prim, 14)), -(6 - int32(scale) + int32(scale1)))
	}

	if errs > crit {
		if StageDebug {
			stagePrint(fmt.Sprintf("SMC crit=%d", crit))
		}
		if w00 < 1 {
			w00 = 1
		}

		scale1 = bitsw00 - 15
		scale2 = bitsw11 - 15

		if scale2 > scale1 {
			scale = scale2
		} else {
			scale = scale1
		}

		w11w00 := mul16(int16(shiftW32(w11, -int32(scale))), int16(shiftW32(w00, -int32(scale))))
		w10w10 := mul16(int16(shiftW32(w10, -int32(scale))), int16(shiftW32(w10, -int32(scale))))
		w00w00 := mul16(int16(shiftW32(w00, -int32(scale))), int16(shiftW32(w00, -int32(scale))))

		if w00w00 > 65536 {
			endiff = w11w00 - w10w10
			if endiff < 0 {
				endiff = 0
			}
			denom = divW32W16(endiff, int16(w00w00>>16))
		} else {
			denom = 65536
		}

		if denom > 7 {
			scale = int16(getSizeInBits(uint32(denom)) - 15)
			if scale > 0 {
				denomW16 = int16(denom >> scale)
				num = shr(enhA0MinusA0A0, int32(scale))
			} else {
				denomW16 = int16(denom)
				num = enhA0MinusA0A0
			}

			A = int16(sqrtFloor(divW32W16(num, denomW16)))

			scale1 = 31 - bitsw10
			scale2 = 21 - scale1
			if w10 == 0 {
				w10prim = 0
			} else {
				w10prim = mulW32(w10, shl(1, int32(scale1)))
			}
			w00prim = shiftW32(w00, -int32(scale2))
			scale = bitsw00 - scale2 - 15

			if scale > 0 {
				w10prim = shr(w10prim, int32(scale))
				w00prim = shr(w00prim, int32(scale))
			}

			if w00prim > 0 && w10prim > 0 {
				w11DivW00 = divW32W16(w10prim, int16(w00prim))
				if getSizeInBits(uint32(w11DivW00))+getSizeInBits(uint32(A)) > 31 {
					B_W32 = 0
				} else {
					B_W32 = 1073741824 - enhA0Div2 - mulW32(int32(A), w11DivW00)
				}
				B = int16(shr(B_W32, 16))
			} else {
				A = 0
				B = 16384
			}
		} else {
			A = 0
			B = 16384
		}

		scaleAndAddVectors(surround, A, 9, current, B, 14, odata, EnhBlockL)
		if StageDebug {
			od := ""
			for i := 0; i < 8; i++ {
				od += fmt.Sprintf(" od[%d]=%d", i, odata[i])
			}
			stagePrint(fmt.Sprintf("SMB A=%d B=%d%s", A, B, od))
		}
	}
}

// enhancer mirrors WebRtcIlbcfix_Enhancer.
func enhancer(odata, idata []int16, idatal, centerStartPos int, period []int,
	plocs []int, periodl int) {

	var surround [EnhBlockL]int16
	memSetW16(surround[:], 0, EnhBlockL, 0)

	getSyncSeq(idata, idatal, centerStartPos, period, plocs, periodl, EnhHL, surround[:])

	smooth(odata, idata[centerStartPos:], surround[:])
}
