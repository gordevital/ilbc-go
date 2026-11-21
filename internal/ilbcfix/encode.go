package ilbcfix

// File: encode.go — 1:1 transcription of WebRtcIlbcfix_EncodeImpl (encode.c).

// EncodeImpl encodes one block of enc.BlockL samples into enc.NoOfWords
// 16-bit words (host order, as produced by PackBits on a little-endian
// machine after SwapBytes).
func EncodeImpl(bytes []uint16, block []int16, enc *EncInst) {
	var weightDenum [(LPCFilterOrder + 1) * NSubMax]int16
	var dataVec [BlockLMax + LPCFilterOrder]int16
	var memVec [CBMemL + CBFilterLen]int16
	var bits iLBCBits

	data := dataVec[LPCFilterOrder:]
	memOff := CBHalfFilter

	residual := enc.LpcBuffer[LPCLookBack+BlockLMax-enc.BlockL:]
	syntDenum := make([]int16, (LPCFilterOrder+1)*NSubMax)
	decresidual := residual
	reverseResidual := data
	reverseDecresidual := reverseResidual

	copy(data, block[:enc.BlockL])

	hpInput(data[:enc.BlockL], kHpInCoefs[:], enc.HpInY[:], enc.HpInX[:], enc.BlockL)

	lpcEncode(syntDenum, weightDenum[:], bits.Lsf[:], data[:enc.BlockL], enc)

	copy(dataVec[:LPCFilterOrder], enc.AnaMem[:])

	for n := 0; n < enc.NSub; n++ {
		filterMAFastQ12(dataVec[:], LPCFilterOrder+n*SubL, residual, n*SubL,
			syntDenum[n*(LPCFilterOrder+1):], LPCFilterOrder+1, SubL)
	}

	copy(enc.AnaMem[:], data[enc.BlockL-LPCFilterOrder:enc.BlockL])

	bits.StartIdx = frameClassify(enc, residual)

	index := (bits.StartIdx - 1) * SubL
	max := maxAbsValueW16(residual[index:], 2*SubL)
	scale := getSizeInBits(uint32(mul16(max, max))) - 25
	if scale < 0 {
		scale = 0
	}

	diff := StateLen - enc.StateShortLen
	en1 := dotProductWithScale(residual[index:], residual[index:], enc.StateShortLen, int(scale))
	index += diff
	en2 := dotProductWithScale(residual[index:], residual[index:], enc.StateShortLen, int(scale))
	var startPos int
	if en1 > en2 {
		bits.StateFirst = 1
		startPos = (bits.StartIdx - 1) * SubL
	} else {
		bits.StateFirst = 0
		startPos = (bits.StartIdx-1)*SubL + diff
	}

	stateSearch(enc, &bits, residual[startPos:],
		syntDenum[(bits.StartIdx-1)*(LPCFilterOrder+1):],
		weightDenum[(bits.StartIdx-1)*(LPCFilterOrder+1):])

	stateConstruct(bits.IdxForMax, bits.IdxVec[:],
		syntDenum[(bits.StartIdx-1)*(LPCFilterOrder+1):],
		decresidual[startPos:], enc.StateShortLen)

	if bits.StateFirst != 0 {
		memSetW16(memVec[:], memOff, CBMemL-enc.StateShortLen, 0)
		copy(memVec[memOff+CBMemL-enc.StateShortLen:],
			decresidual[startPos:startPos+enc.StateShortLen])

		cbSearch(enc, bits.CbIndex[:], bits.GainIndex[:],
			residual[startPos+enc.StateShortLen:], memVec[memOff+CBMemL-stMemLTbl:],
			stMemLTbl, diff,
			weightDenum[bits.StartIdx*(LPCFilterOrder+1):], 0)

		cbConstruct(decresidual[startPos+enc.StateShortLen:],
			bits.CbIndex[:], bits.GainIndex[:],
			memVec[:], memOff+CBMemL-stMemLTbl, stMemLTbl, diff)
	} else {
		memCpyReversedOrder(reverseResidual, diff-1,
			residual[(bits.StartIdx+1)*SubL-StateLen:], diff)

		memCpyReversedOrder(memVec[:], memOff+CBMemL-1,
			decresidual[startPos:], enc.StateShortLen)
		memSetW16(memVec[:], memOff, CBMemL-enc.StateShortLen, 0)

		cbSearch(enc, bits.CbIndex[:], bits.GainIndex[:],
			reverseResidual[:diff], memVec[memOff+CBMemL-stMemLTbl:],
			stMemLTbl, diff,
			weightDenum[(bits.StartIdx-1)*(LPCFilterOrder+1):], 0)

		cbConstruct(reverseDecresidual, bits.CbIndex[:], bits.GainIndex[:],
			memVec[:], memOff+CBMemL-stMemLTbl, stMemLTbl, diff)

		memCpyReversedOrder(decresidual, startPos-1, reverseDecresidual, diff)
	}

	nfor := enc.NSub - bits.StartIdx - 1
	subcount := 1

	if nfor > 0 {
		memSetW16(memVec[:], memOff, CBMemL-StateLen, 0)
		copy(memVec[memOff+CBMemL-StateLen:],
			decresidual[(bits.StartIdx-1)*SubL:(bits.StartIdx-1)*SubL+StateLen])

		for subframe := 0; subframe < nfor; subframe++ {
			dst := (bits.StartIdx + 1 + subframe) * SubL

			cbSearch(enc, bits.CbIndex[subcount*CBNStages:],
				bits.GainIndex[subcount*CBNStages:], residual[dst:],
				memVec[memOff:], memLfTbl, SubL,
				weightDenum[(bits.StartIdx+1+subframe)*(LPCFilterOrder+1):], subcount)

			cbConstruct(decresidual[dst:],
				bits.CbIndex[subcount*CBNStages:],
				bits.GainIndex[subcount*CBNStages:], memVec[:], memOff, memLfTbl, SubL)

			copy(memVec[memOff:memOff+CBMemL-SubL], memVec[memOff+SubL:memOff+CBMemL])
			copy(memVec[memOff+CBMemL-SubL:], decresidual[dst:dst+SubL])

			subcount++
		}
	}

	if bits.StartIdx > 1 {
		nback := bits.StartIdx - 1

		memCpyReversedOrder(reverseResidual, nback*SubL-1, residual, nback*SubL)

		memlGotten := SubL * (enc.NSub + 1 - bits.StartIdx)
		if memlGotten > CBMemL {
			memlGotten = CBMemL
		}

		memCpyReversedOrder(memVec[:], memOff+CBMemL-1,
			decresidual[nback*SubL:], memlGotten)
		memSetW16(memVec[:], memOff, CBMemL-memlGotten, 0)

		for subframe := 0; subframe < nback; subframe++ {
			cbSearch(enc, bits.CbIndex[subcount*CBNStages:],
				bits.GainIndex[subcount*CBNStages:],
				reverseResidual[subframe*SubL:], memVec[memOff:],
				memLfTbl, SubL,
				weightDenum[(bits.StartIdx-2-subframe)*(LPCFilterOrder+1):], subcount)

			cbConstruct(reverseDecresidual[subframe*SubL:],
				bits.CbIndex[subcount*CBNStages:],
				bits.GainIndex[subcount*CBNStages:], memVec[:], memOff, memLfTbl, SubL)

			copy(memVec[memOff:memOff+CBMemL-SubL], memVec[memOff+SubL:memOff+CBMemL])
			copy(memVec[memOff+CBMemL-SubL:], reverseDecresidual[subframe*SubL:subframe*SubL+SubL])

			subcount++
		}

		memCpyReversedOrder(decresidual, SubL*nback-1, reverseDecresidual, SubL*nback)
	}

	indexConvEnc(bits.CbIndex[:])

	packBits(bytes, &bits, enc.Mode)
	swapBytes(bytes, enc.NoOfWords, bytes)
}
