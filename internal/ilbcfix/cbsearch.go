package ilbcfix

// File: cbsearch.go — 1:1 transcription of cb_search.c,
// cb_search_core.c, cb_update_best_index.c, filtered_cb_vecs.c,
// cb_mem_energy.c, cb_mem_energy_calc.c, cb_mem_energy_augmentation.c,
// augmented_cb_corr.c, interpolate_samples.c.

func interpolateSamples(interpSamples, cb []int16, cbOff, lMem int) {
	out := 0
	for j := 0; j < 20; j++ {
		temp1 := 0
		temp2 := 3
		ppo := cbOff + lMem - 4
		ppi := cbOff + lMem - j - 24
		for i := 0; i < 4; i++ {
			interpSamples[out] = w16(
				int32((int32(kAlpha[temp2])*int32(cb[ppo]))>>15) +
					int32((int32(kAlpha[temp1])*int32(cb[ppi]))>>15))
			out++
			ppo++
			ppi++
			temp1++
			temp2--
		}
	}
}

func cbMemEnergyAugmentation(interpSamples, cb []int16, scale, baseSize int,
	energyW16, energyShifts []int16) {
	cbMemPtr := 147
	interpPtr := 0
	enPtr := baseSize - 20
	nrjRecursive := dotProductWithScale(cb[cbMemPtr-19:], cb[cbMemPtr-19:], 15, scale)
	ppe := cbMemPtr - 20

	for lagcount := 20; lagcount <= 39; lagcount++ {
		nrjRecursive += mul16(cb[ppe], cb[ppe]) >> uint(scale)
		ppe--
		energy := nrjRecursive
		energy += dotProductWithScale(interpSamples[interpPtr:], interpSamples[interpPtr:], 4, scale)
		interpPtr += 4
		pp := cbMemPtr - lagcount
		energy += dotProductWithScale(cb[pp:], cb[pp:], SubL-lagcount, scale)

		sh := min16(normW32(energy))
		energyShifts[enPtr] = sh
		energyW16[enPtr] = w16(shl(energy, int32(sh)) >> 16)
		enPtr++
	}
}

func cbMemEnergyCalc(energy int32, range_, ppi, ppo int, cb []int16,
	energyW16, energyShifts []int16, scale, baseSize int) {
	eSh := 1 + baseSize
	eW := 1 + baseSize
	for j := 0; j+1 < range_; j++ {
		tmp := mul16(cb[ppi], cb[ppi]) - mul16(cb[ppo], cb[ppo])
		energy += tmp >> uint(scale)
		if energy < 0 {
			energy = 0
		}
		ppi--
		ppo--
		sh := min16(normW32(energy))
		energyShifts[eSh] = sh
		eSh++
		energyW16[eW] = w16(shl(energy, int32(sh)) >> 16)
		eW++
	}
}

func cbMemEnergy(range_ int, cb []int16, cbOff int, filteredCB []int16,
	lMem, lTarget int, energyW16, energyShifts []int16, scale, baseSize int) {
	ppi := cbOff + lMem - lTarget - 1
	ppo := cbOff + lMem - 1
	pp := cbOff + lMem - lTarget
	energy := dotProductWithScale(cb[pp:], cb[pp:], lTarget, scale)

	energyShifts[0] = min16(normW32(energy))
	energyW16[0] = w16(shl(energy, int32(energyShifts[0])) >> 16)
	cbMemEnergyCalc(energy, range_, ppi, ppo, cb, energyW16, energyShifts, scale, 0)

	pp = lMem - lTarget
	energy = dotProductWithScale(filteredCB[pp:], filteredCB[pp:], lTarget, scale)
	energyShifts[baseSize] = min16(normW32(energy))
	energyW16[baseSize] = w16(shl(energy, int32(energyShifts[baseSize])) >> 16)

	ppi = lMem - 1 - lTarget
	ppo = lMem - 1
	cbMemEnergyCalc(energy, range_, ppi, ppo, filteredCB, energyW16, energyShifts, scale, baseSize)
}

func filteredCbVecs(cbvectors, cbFull []int16, cbOff, lMem, samples int) {
	memSetW16(cbFull, cbOff+lMem, CBHalfFilter, 0)
	memSetW16(cbFull, cbOff-CBHalfFilter, CBHalfFilter, 0)
	memSetW16(cbvectors, 0, lMem-samples, 0)
	filterMAFastQ12(cbFull, cbOff+CBHalfFilter+lMem-samples, cbvectors,
		lMem-samples, kCbFiltersRev[:], CBFilterLen, samples)
}

func augmentedCbCorr(target, buffer []int16, bufferEnd int, interpSamples []int16,
	crossDot []int32, low, high, scale int) {
	iSPtr := 0
	for lagcount := low; lagcount <= high; lagcount++ {
		ilow := lagcount - 4
		cd := dotProductWithScale(target, buffer[bufferEnd-lagcount:], ilow, scale)
		cd += dotProductWithScale(target[ilow:], interpSamples[iSPtr:], 4, scale)
		targetPtr := lagcount
		iSPtr += lagcount - ilow
		cd += dotProductWithScale(target[targetPtr:], buffer[bufferEnd-lagcount:], SubL-lagcount, scale)
		crossDot[lagcount-low] = cd
	}
}

func cbSearchCore(cDot []int32, range_ int, stage int16, inverseEnergy,
	inverseEnergyShift []int16, crit []int32) (bestIndex int, bestCrit int32, bestCritSh int16) {
	if stage == 0 {
		for i := 0; i < range_; i++ {
			if cDot[i] < 0 {
				cDot[i] = 0
			}
		}
	}

	maxW32 := maxAbsValueW32(cDot, range_)
	sh := min16(normW32(maxW32))
	max := int16(word16Min)

	for i := 0; i < range_; i++ {
		tmp32 := shl(cDot[i], int32(sh))
		tmp16 := w16(tmp32 >> 16)
		cDotSqW16 := w16((mul16(tmp16, tmp16)) >> 16)
		crit[i] = mul16(cDotSqW16, inverseEnergy[i])
		if crit[i] != 0 {
			if inverseEnergyShift[i] > max {
				max = inverseEnergyShift[i]
			}
		}
	}
	if max == word16Min {
		max = 0
	}

	for i := 0; i < range_; i++ {
		tmp16 := int16(16)
		v := int16(int32(max) - int32(inverseEnergyShift[i]))
		if v < 16 {
			tmp16 = v
		}
		crit[i] = splShiftW32(crit[i], -int32(tmp16))
	}

	bestIndex = maxIndexW32(crit, range_)
	bestCrit = crit[bestIndex]
	bestCritSh = w16(32 - 2*int32(sh) + int32(max))
	return
}

func cbUpdateBestIndex(critNew int32, critNewSh int16, indexNew int, cDotNew int32,
	invEnergyNew, energyShiftNew int16, critMax *int32, shTotMax *int16,
	bestIndex *int, bestGain *int16) {
	var shOld, shNew int16
	if critNewSh > *shTotMax {
		shOld = min16(int32(critNewSh) - int32(*shTotMax))
		shNew = 0
	} else {
		shOld = 0
		shNew = min16(int32(*shTotMax) - int32(critNewSh))
	}

	if (critNew >> uint(shNew)) > (*critMax >> uint(shOld)) {
		tmp16 := min16(normW32(cDotNew))
		tmp16 = 16 - tmp16

		scaleTmp := int16(-int32(energyShiftNew) - int32(tmp16) + 31)
		if scaleTmp > 31 {
			scaleTmp = 31
		}

		gainW32 := (mul16(w16(splShiftW32(cDotNew, -int32(tmp16))), invEnergyNew)) >> uint(scaleTmp)
		if gainW32 > 21299 {
			*bestGain = 21299
		} else if gainW32 < -21299 {
			*bestGain = -21299
		} else {
			*bestGain = int16(gainW32)
		}

		*critMax = critNew
		*shTotMax = critNewSh
		*bestIndex = indexNew
	}
}

func cbSearch(enc *EncInst, index, gainIndex []int16, intarget, decResidual []int16,
	lMem, lTarget int, weightDenum []int16, block int) {
	var CBbuf [CBMemL + LPCFilterOrder + CBHalfFilter]int16
	var cDot [128]int32
	var crit [128]int32
	var targetVec [SubL + LPCFilterOrder]int16
	var cbvectors [CBMemL + 1]int16
	var codedVec [SubL]int16
	var interpSamples [20 * 4]int16
	var interpSamplesFilt [20 * 4]int16
	var energyW16 [CBExpand * 128]int16
	var energyShifts [CBExpand * 128]int16

	cbOff := LPCFilterOrder
	buf := CBbuf[cbOff:]
	target := targetVec[LPCFilterOrder:]

	baseSize := lMem - lTarget + 1
	if lTarget == SubL {
		baseSize = lMem - 19
	}

	noOfZeros := lMem - kFilterRange[block]
	memSetW16(CBbuf[:], 0, noOfZeros+LPCFilterOrder, 0)
	filterARFastQ12(decResidual[noOfZeros:], CBbuf[:], cbOff+noOfZeros,
		weightDenum, LPCFilterOrder+1, kFilterRange[block])

	copy(targetVec[:LPCFilterOrder],
		CBbuf[cbOff+noOfZeros+kFilterRange[block]-LPCFilterOrder:])
	filterARFastQ12(intarget, targetVec[:], LPCFilterOrder, weightDenum,
		LPCFilterOrder+1, lTarget)

	copy(codedVec[:], target[:lTarget])

	temp1 := maxAbsValueW16(buf, lMem)
	temp2 := maxAbsValueW16(target, lTarget)
	var scale int
	if temp1 > 0 && temp2 > 0 {
		m := temp1
		if temp2 > m {
			m = temp2
		}
		scale = int(getSizeInBits(uint32(mul16(m, m))))
	} else {
		scale = 30
	}
	scale -= 25
	if scale < 0 {
		scale = 0
	}

	targetEner := dotProductWithScale(target, target, lTarget, scale)

	filteredCbVecs(cbvectors[:], CBbuf[:], cbOff, lMem, kFilterRange[block])

	range_ := kSearchRange[block][0]

	if lTarget == SubL {
		interpolateSamples(interpSamples[:], CBbuf[:], cbOff, lMem)
		interpolateSamples(interpSamplesFilt[:], cbvectors[:], 0, lMem)

		cbMemEnergyAugmentation(interpSamples[:], buf, scale, 20, energyW16[:], energyShifts[:])
		cbMemEnergyAugmentation(interpSamplesFilt[:], cbvectors[:], scale,
			baseSize+20, energyW16[:], energyShifts[:])

		cbMemEnergy(range_, CBbuf[:], cbOff, cbvectors[:], lMem, lTarget,
			energyW16[20:], energyShifts[20:], scale, baseSize)
	} else {
		cbMemEnergy(range_, CBbuf[:], cbOff, cbvectors[:], lMem, lTarget,
			energyW16[:], energyShifts[:], scale, baseSize)
		memSetW16(energyW16[:], range_, baseSize-range_, 0)
		memSetW16(energyW16[:], range_+baseSize, baseSize-range_, 0)
	}

	energyInverse(energyW16[:], baseSize*CBExpand)

	var gains [CBNStages + 1]int16
	gains[0] = 16384

	for stage := int16(0); stage < CBNStages; stage++ {
		range_ = kSearchRange[block][stage]

		critMax := int32(0)
		shTotMax := int16(-100)
		bestIndex := 0
		bestGain := int16(0)

		cDotPtr := 0
		if lTarget == SubL {
			augmentedCbCorr(target, buf, lMem, interpSamples[:], cDot[:], 20, 39, scale)
			cDotPtr = 20
		}

		crossCorrelation(cDot[cDotPtr:], target, buf, lMem-lTarget,
			lTarget, range_, scale, -1)

		if lTarget == SubL {
			range_ = kSearchRange[block][stage] + 20
		} else {
			range_ = kSearchRange[block][stage]
		}

		indexOffset := 0

		indexNew, critNew, critNewSh := cbSearchCore(cDot[:], range_, stage,
			energyW16[:], energyShifts[:], crit[:])
		cbUpdateBestIndex(critNew, critNewSh, indexNew+indexOffset,
			cDot[indexNew+indexOffset], energyW16[indexNew+indexOffset],
			energyShifts[indexNew+indexOffset], &critMax, &shTotMax, &bestIndex, &bestGain)

		sInd := 0
		if bestIndex > CBResRange>>1 {
			sInd = bestIndex - (CBResRange >> 1)
		}
		eInd := sInd + CBResRange
		if eInd >= range_ {
			eInd = range_ - 1
			sInd = eInd - CBResRange
		}

		range_ = kSearchRange[block][stage]

		if lTarget == SubL {
			i := sInd
			if sInd < 20 {
				high := eInd + 20
				if high > 39 {
					high = 39
				}
				augmentedCbCorr(target, cbvectors[:], lMem, interpSamplesFilt[:],
					cDot[:], sInd+20, high, scale)
				i = 20
				cDotPtr = 20 - sInd
			} else {
				cDotPtr = 0
			}

			cbVecOff := lMem - 20 - i
			crossCorrelation(cDot[cDotPtr:], target, cbvectors[:], cbVecOff,
				lTarget, eInd-i+1, scale, -1)
		} else {
			cDotPtr = 0
			cbVecOff := lMem - lTarget - sInd
			crossCorrelation(cDot[cDotPtr:], target, cbvectors[:], cbVecOff,
				lTarget, eInd-sInd+1, scale, -1)
		}

		indexOffset = baseSize + sInd

		indexNew, critNew, critNewSh = cbSearchCore(cDot[:], eInd-sInd+1, stage,
			energyW16[indexOffset:], energyShifts[indexOffset:], crit[:])
		cbUpdateBestIndex(critNew, critNewSh, indexNew+indexOffset,
			cDot[indexNew], energyW16[indexNew+indexOffset],
			energyShifts[indexNew+indexOffset], &critMax, &shTotMax,
			&bestIndex, &bestGain)

		index[stage] = int16(bestIndex)

		bestGain, gainIndex[stage] = gainQuant(bestGain,
			w16(splAbsW16(gains[stage])), stage)

		var pp []int16
		var aug [SubL]int16
		if lTarget == StateLen-enc.StateShortLen {
			if int(index[stage]) < baseSize {
				pp = CBbuf[cbOff+lMem-lTarget-int(index[stage]):]
			} else {
				pp = cbvectors[lMem-lTarget-int(index[stage])+baseSize:]
			}
		} else {
			if int(index[stage]) < baseSize {
				if index[stage] >= 20 {
					index[stage] -= 20
					pp = CBbuf[cbOff+lMem-lTarget-int(index[stage]):]
				} else {
					index[stage] += int16(baseSize - 20)
					createAugmentedVec(int(index[stage])-baseSize+40,
						CBbuf[:], cbOff+lMem, aug[:])
					pp = aug[:]
				}
			} else {
				if int(index[stage])-baseSize >= 20 {
					index[stage] -= 20
					pp = cbvectors[lMem-lTarget-int(index[stage])+baseSize:]
				} else {
					index[stage] += int16(baseSize - 20)
					createAugmentedVec(int(index[stage])-2*baseSize+40,
						cbvectors[:], lMem, aug[:])
					pp = aug[:]
				}
			}
		}

		addAffineVectorToVector(target[:lTarget], pp, w16(-int32(bestGain)),
			8192, 14, lTarget)

		gains[stage+1] = bestGain
	}

	for i := 0; i < lTarget; i++ {
		codedVec[i] = w16(int32(codedVec[i]) - int32(target[i]))
	}

	codedEner := dotProductWithScale(codedVec[:], codedVec[:], lTarget, scale)
	j := gainIndex[0]

	temp1 = int16(normW32(codedEner))
	temp2 = int16(normW32(targetEner))
	var bits int16
	if temp1 < temp2 {
		bits = 16 - temp1
	} else {
		bits = 16 - temp2
	}

	tmp := w16(mul16(gains[1], gains[1]) >> 14)
	targetEner = mul16(int16(splShiftW32(targetEner, -int32(bits))), tmp)

	tmpW32 := shl(int32(gains[1]-1), 1)

	gainPtr := gainIndex[0]
	temp1 = int16(splShiftW32(codedEner, -int32(bits)))
	kGainSq5Ptr := int32(kGainSq5[j])

	for ii := gainIndex[0]; ii < 32; ii++ {
		t32 := mul16(temp1, kGainSq5Sq[gainPtr])
		t32 -= targetEner
		if t32 < 0 {
			if kGainSq5Ptr < tmpW32 {
				j = ii
				kGainSq5Ptr = int32(kGainSq5[ii])
			}
		}
		gainPtr++
	}
	gainIndex[0] = j
}

func min16(v int32) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}
