package ilbcfix

// File: decode_residual.go — decode_residual.c.

// decodeResidual decodes the residual frame from the cb/gain indices.
// dec.PrevResidual supplies the codebook memory, dec.EnhBuf supplies the
// reversed-decode scratch (matching the C memory reuse).
func decodeResidual(dec *DecInst, bits *iLBCBits, decresidual []int16, syntdenum []int16) bool {
	reverseDecresidual := dec.EnhBuf[:]
	memVec := dec.PrevResidual[:]
	diff := StateLen - dec.StateShortLen

	startPos := (bits.StartIdx - 1) * SubL
	if bits.StateFirst == 0 {
		startPos = (bits.StartIdx-1)*SubL + diff
	}

	memBaseMain := CBHalfFilter
	memBaseState := CBHalfFilter + CBMemL - stMemLTbl

	// decode scalar part of start state
	stateConstruct(bits.IdxForMax, bits.IdxVec[:],
		syntdenum[(bits.StartIdx-1)*(LPCFilterOrder+1):],
		decresidual[startPos:], dec.StateShortLen)

	if bits.StateFirst != 0 { // put adaptive part in the end
		memSetW16(memVec, CBHalfFilter, CBMemL-dec.StateShortLen, 0)
		copy(memVec[CBHalfFilter+CBMemL-dec.StateShortLen:],
			decresidual[startPos:startPos+dec.StateShortLen])

		if !cbConstruct(decresidual[startPos+dec.StateShortLen:],
			bits.CbIndex[:], bits.GainIndex[:],
			memVec, memBaseState, stMemLTbl, diff) {
			return false
		}
	} else { // put adaptive part in the beginning
		memlGotten := dec.StateShortLen
		memCpyReversedOrder(memVec, CBHalfFilter+CBMemL-1,
			decresidual[startPos:], memlGotten)
		memSetW16(memVec, CBHalfFilter, CBMemL-memlGotten, 0)

		if !cbConstruct(reverseDecresidual, bits.CbIndex[:], bits.GainIndex[:],
			memVec, memBaseState, stMemLTbl, diff) {
			return false
		}

		memCpyReversedOrder(decresidual, startPos-1, reverseDecresidual, diff)
	}

	subcount := 1

	// forward prediction of subframes
	if dec.NSub > bits.StartIdx+1 {
		memSetW16(memVec, CBHalfFilter, CBMemL-StateLen, 0)
		src := (bits.StartIdx - 1) * SubL
		copy(memVec[CBHalfFilter+CBMemL-StateLen:CBHalfFilter+CBMemL],
			decresidual[src:src+StateLen])

		nfor := dec.NSub - bits.StartIdx - 1
		for subframe := 0; subframe < nfor; subframe++ {
			dst := (bits.StartIdx + 1 + subframe) * SubL
			if !cbConstruct(decresidual[dst:], bits.CbIndex[subcount*CBNStages:],
				bits.GainIndex[subcount*CBNStages:],
				memVec, memBaseMain, memLfTbl, SubL) {
				return false
			}
			copy(memVec[CBHalfFilter:CBHalfFilter+CBMemL-SubL],
				memVec[CBHalfFilter+SubL:])
			copy(memVec[CBHalfFilter+CBMemL-SubL:], decresidual[dst:dst+SubL])
			subcount++
		}
	}

	// backward prediction of subframes
	if bits.StartIdx > 1 {
		memlGotten := SubL * (dec.NSub + 1 - bits.StartIdx)
		if memlGotten > CBMemL {
			memlGotten = CBMemL
		}
		memCpyReversedOrder(memVec, CBHalfFilter+CBMemL-1,
			decresidual[(bits.StartIdx-1)*SubL:], memlGotten)
		memSetW16(memVec, CBHalfFilter, CBMemL-memlGotten, 0)

		nback := bits.StartIdx - 1
		for subframe := 0; subframe < nback; subframe++ {
			if !cbConstruct(reverseDecresidual[subframe*SubL:],
				bits.CbIndex[subcount*CBNStages:],
				bits.GainIndex[subcount*CBNStages:],
				memVec, memBaseMain, memLfTbl, SubL) {
				return false
			}
			copy(memVec[CBHalfFilter:CBHalfFilter+CBMemL-SubL],
				memVec[CBHalfFilter+SubL:])
			copy(memVec[CBHalfFilter+CBMemL-SubL:],
				reverseDecresidual[subframe*SubL:subframe*SubL+SubL])
			subcount++
		}

		memCpyReversedOrder(decresidual, SubL*nback-1, reverseDecresidual, SubL*nback)
	}

	return true
}
