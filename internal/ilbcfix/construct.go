package ilbcfix

import (
	"fmt"
	"strings"
)

// File: construct.go — state_construct.c, get_cd_vec.c,
// create_augmented_vec.c, cb_construct.c.

// stateConstruct decodes the start state vector.
//
// sintDenum is the (start-subframe) synthesis denominator, syntdenum[(startIdx-1)*11].
// out is len samples (stateShortLen).
func stateConstruct(idxForMax int, idxVec []int16, syntDenum []int16, out []int16, length int) {
	var numerator [LPCFilterOrder + 1]int16
	var sampleValVec [2*StateShortLen30ms + LPCFilterOrder]int16
	var sampleMaVec [2*StateShortLen30ms + LPCFilterOrder]int16
	sampleVal := sampleValVec[LPCFilterOrder:]
	sampleMa := sampleMaVec[LPCFilterOrder:]

	for k := 0; k < LPCFilterOrder+1; k++ {
		numerator[k] = syntDenum[LPCFilterOrder-k]
	}

	maxVal := kFrgQuantMod[idxForMax]

	idxPos := length - 1
	if idxForMax < 37 {
		// maxVal Q8, result Q(-1)
		for k := 0; k < length; k++ {
			sampleVal[k] = w16((int32(maxVal)*int32(kStateSq3[idxVec[idxPos]]) + 2097152) >> 22)
			idxPos--
		}
	} else if idxForMax < 59 {
		// maxVal Q5
		for k := 0; k < length; k++ {
			sampleVal[k] = w16((int32(maxVal)*int32(kStateSq3[idxVec[idxPos]]) + 262144) >> 19)
			idxPos--
		}
	} else {
		// maxVal Q3
		for k := 0; k < length; k++ {
			sampleVal[k] = w16((int32(maxVal)*int32(kStateSq3[idxVec[idxPos]]) + 65536) >> 17)
			idxPos--
		}
	}

	memSetW16(sampleVal[length:], 0, length, 0)
	memSetW16(sampleValVec[:LPCFilterOrder], 0, LPCFilterOrder, 0)

	filterMAFastQ12(sampleValVec[:], LPCFilterOrder, sampleMaVec[:], LPCFilterOrder, numerator[:], LPCFilterOrder+1, length+LPCFilterOrder)
	memSetW16(sampleMaVec[:], LPCFilterOrder+length+LPCFilterOrder, length-LPCFilterOrder, 0)
	filterARFastQ12(sampleMa, sampleValVec[:], LPCFilterOrder, syntDenum, LPCFilterOrder+1, 2*length)

	t1 := LPCFilterOrder + length - 1
	t2 := LPCFilterOrder + 2*length - 1
	for k := 0; k < length; k++ {
		out[k] = w16(int32(sampleValVec[t1]) + int32(sampleValVec[t2]))
		t1--
		t2--
	}
}

// createAugmentedVec constructs the augmented (interpolated) codebook vector.
//
// index is the lag. base/off describe the codebook buffer: base[off] points
// ONE ELEMENT PAST THE END of the buffer (C: buffer pointer); cbVec receives
// SubL samples.
func createAugmentedVec(index int, base []int16, off int, cbVec []int16) {
	var cbVecTmp [4]int16
	interpLen := index
	if 4 < interpLen {
		interpLen = 4
	}
	iLow := index - interpLen

	// copy the first non-interpolated part: cbVec[0:index] = base[off-index : off]
	copy(cbVec, base[off-index:off])

	// interpolation: cbVec[iLow+k] = ppi[k]*kAlpha[k]>>15 + ppo[k]*kAlpha[interpLen-1-k]>>15
	ppi := off - index - interpLen // base index of ppi[0]
	ppo := off - interpLen         // base index of ppo[0]
	elementwiseVectorMult(cbVec[iLow:iLow+interpLen], base[ppi:ppi+interpLen], kAlpha[:], 0, interpLen, 15)
	reverseOrderMultArrayElements(cbVecTmp[:], base[ppo:ppo+interpLen], kAlpha[:], interpLen-1, interpLen, 15)
	addVectorsAndShift(cbVec[iLow:iLow+interpLen], cbVec[iLow:iLow+interpLen], cbVecTmp[:interpLen], interpLen, 0)

	// copy the second non-interpolated part
	n := SubL - index
	if n > index {
		n = index
	}
	copy(cbVec[index:], base[off-index:off-index+n])
}

// getCbVec constructs a codebook vector from the memory buffer.
//
// mem must expose CBHalfFilter guard cells before and after the lMem-window:
// mem is passed as a slice starting at the guard origin, i.e. mem[CBHalfFilter :
// CBHalfFilter+lMem] is the logical buffer; the function zeroes the guards as
// in the C (WebRtcSpl_MemSetW16(mem-CB_HALFFILTERLEN / mem+lMem)).
func getCbVec(cbvec, mem []int16, memBase int, index, lMem, cbVeclen int) bool {
	var tempbuff2 [SubL + 5]int16
	baseSize := lMem - cbVeclen + 1
	if cbVeclen == SubL {
		baseSize += cbVeclen / 2
	}

	if index < lMem-cbVeclen+1 {
		k := index + cbVeclen
		copy(cbvec, mem[memBase+lMem-k:memBase+lMem-k+cbVeclen])
	} else if index < baseSize {
		k := 2*(index-(lMem-cbVeclen+1)) + cbVeclen
		lag := k / 2
		createAugmentedVec(lag, mem, memBase+lMem, cbvec)
	} else {
		if index-baseSize < lMem-cbVeclen+1 {
			memIndTest := lMem - (index - baseSize + cbVeclen)
			memSetW16(mem, memBase-CBHalfFilter, CBHalfFilter, 0)
			memSetW16(mem, memBase+lMem, CBHalfFilter, 0)
			filterMAFastQ12(mem, memBase+memIndTest+CBHalfFilter, cbvec, 0, kCbFiltersRev[:], CBFilterLen, cbVeclen)
		} else {
			if cbVeclen < SubL {
				// Bad index coming from the stream; decoder state corrupt.
				return false
			}
			memIndTest := lMem - cbVeclen - CBFilterLen
			memSetW16(mem, memBase+lMem, CBHalfFilter, 0)
			filterMAFastQ12(mem, memBase+memIndTest+7, tempbuff2[:], 0, kCbFiltersRev[:], CBFilterLen, cbVeclen+5)
			lag := (cbVeclen << 1) - 20 + index - baseSize - lMem - 1
			createAugmentedVec(lag, tempbuff2[:], len(tempbuff2), cbvec)
		}
	}
	return true
}

func cbConstruct(decvector, index, gainIndex, mem []int16, memBase, lMem, veclen int) bool {
	var cbvec0, cbvec1, cbvec2 [SubL]int16
	var gain [CBNStages]int16

	gain[0] = gainDequant(gainIndex[0], 16384, 0)
	gain[1] = gainDequant(gainIndex[1], gain[0], 1)
	gain[2] = gainDequant(gainIndex[2], gain[1], 2)

	if !getCbVec(cbvec0[:], mem, memBase, int(index[0]), lMem, veclen) {
		return false
	}
	if !getCbVec(cbvec1[:], mem, memBase, int(index[1]), lMem, veclen) {
		return false
	}
	if !getCbVec(cbvec2[:], mem, memBase, int(index[2]), lMem, veclen) {
		return false
	}

	if StageDebug {
		giStr := ""
		for i := 0; i < len(gainIndex); i++ {
			giStr += fmt.Sprintf("%d ", gainIndex[i])
		}
		idStr := ""
		for i := 0; i < len(index); i++ {
			idStr += fmt.Sprintf("%d ", index[i])
		}
		stagePrint(fmt.Sprintf("GC idx=%s gi=%s g0=%d vmem=%d veclen=%d",
			strings.TrimRight(idStr, " "), strings.TrimRight(giStr, " "), gain[0], lMem, veclen))
		stagePrint("GV0:", cbvec0[:veclen])
		stagePrint("GV1:", cbvec1[:veclen])
		stagePrint("GV2:", cbvec2[:veclen])
	}

	for j := 0; j < veclen; j++ {
		a32 := mul16(gain[0], cbvec0[j])
		a32 += mul16(gain[1], cbvec1[j])
		a32 += mul16(gain[2], cbvec2[j])
		decvector[j] = w16(shr(a32+8192, 14))
	}
	if StageDebug {
		stagePrint("CV:", decvector[:veclen])
	}
	return true
}
