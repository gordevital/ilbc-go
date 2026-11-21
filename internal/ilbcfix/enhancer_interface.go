package ilbcfix

// File: enhancer_interface.go — 1:1 transcription of enhancer_interface.c
// (WebRtcIlbcfix_EnhancerInterface).

// enhancerInterface enhances the frame in in (dec.BlockL samples) into out
// and returns the estimated full-rate pitch lag of the frame.
import (
	"fmt"
)

func enhancerInterface(out, in []int16, dec *DecInst) int {
	blockL := dec.BlockL
	inLen := blockL + 120

	// Stack based.
	var downSampled [(BlockLMax + 120) >> 1]int16 // 180 samples
	var corr32 [50]int32
	var corrmax [3]int32
	var corr16 [3]int16
	var en16 [3]int16
	var totsh [3]int16
	var lagmax [3]int

	// plc_pred reuses the downsampled buffer since plc_pred[ENH_BLOCKL] and
	// downsampled are non-overlapping.
	plcPred := downSampled[:EnhBlockL]

	enhBuf := dec.EnhBuf[:]
	enhPeriod := dec.EnhPeriod[:]

	// Copy the new data into the enhancer buffer.
	copy(enhBuf[:EnhBufl-blockL], enhBuf[blockL:])
	copy(enhBuf[EnhBufl-blockL:EnhBufl], in[:blockL])

	// Set variables that are dependent on the frame size.
	var plcBlockL, newBlocks, startPos int
	if dec.Mode == 30 {
		plcBlockL = EnhBlockL
		newBlocks = 3
		startPos = 320 // start position for enhancement (640-new_blocks*ENH_BLOCKL-80)
	} else {
		plcBlockL = 40
		newBlocks = 2
		startPos = 440 // start position for enhancement (640-new_blocks*ENH_BLOCKL-40)
	}

	// Update the pitch prediction for each enhancer block, move the old ones.
	copy(enhPeriod[:EnhNBlocksTot-newBlocks], enhPeriod[newBlocks:])

	downsampleFast(enhBuf, EnhBufl-inLen, inLen+EnhBuflFilterOVRhead,
		downSampled[:], inLen/2, kLpFiltCoefs[:], FilterOrderDSPlus1, FactorDS, DelayDS)
	if StageDebug {
		for chunk := 0; chunk < 3; chunk++ {
			end := chunk*60 + 60
			if end > len(downSampled) {
				end = len(downSampled)
			}
			stagePrint(fmt.Sprintf("DS %d:", chunk), downSampled[chunk*60:end])
		}
	}

	// Estimate the pitch in the down-sampled domain.
	lag := 20
	tlag := 20
	for iBlock := 0; iBlock < newBlocks; iBlock++ {
		targetOff := 60 + iBlock*EnhBlockLHalf
		regOff := targetOff - 10

		// Scaling.
		max16 := maxAbsValueW16(downSampled[regOff-50:regOff-50+EnhBlockLHalf+50-1], EnhBlockLHalf+50-1)
		shifts := int(getSizeInBits(uint32(mul16(max16, max16))) - 25)
		if shifts < 0 {
			shifts = 0
		}

		// Compute the cross correlation.
		crossCorrelation(corr32[:], downSampled[targetOff:targetOff+EnhBlockLHalf],
			downSampled[:], regOff, EnhBlockLHalf, 50, shifts, -1)

		// Find the three highest correlations that should be compared for
		// the highest (corr*corr)/ener.
		for i := 0; i < 2; i++ {
			lagmax[i] = maxIndexW32(corr32[:], 50)
			corrmax[i] = corr32[lagmax[i]]
			start := maxI(2, lagmax[i]) - 2
			stop := minI(47, lagmax[i]) + 2
			for k := start; k <= stop; k++ {
				corr32[k] = 0
			}
		}
		lagmax[2] = maxIndexW32(corr32[:], 50)
		corrmax[2] = corr32[lagmax[2]]

		// Calculate normalized corr^2 and ener.
		for i := 0; i < 3; i++ {
			corrSh := int16(15 - getSizeInBits(uint32(corrmax[i])))
			ener := dotProductWithScale(downSampled[regOff-lagmax[i]:regOff-lagmax[i]+EnhBlockLHalf],
				downSampled[regOff-lagmax[i]:regOff-lagmax[i]+EnhBlockLHalf], EnhBlockLHalf, shifts)
			enerSh := int16(15 - getSizeInBits(uint32(ener)))
			corr16[i] = w16(splShiftW32(corrmax[i], int32(corrSh)))
			corr16[i] = w16((mul16(corr16[i], corr16[i])) >> 16)
			en16[i] = w16(splShiftW32(ener, int32(enerSh)))
			totsh[i] = enerSh - 2*corrSh
		}

		// Compare lagmax[0..2] for the (corr^2)/ener criteria.
		ind := 0
		for i := 1; i < 3; i++ {
			if totsh[ind] > totsh[i] {
				sh := minI32(31, int32(totsh[ind]-totsh[i]))
				if mul16(corr16[ind], en16[i]) < (mul16(corr16[i], en16[ind]) >> uint(sh)) {
					ind = i
				}
			} else {
				sh := minI32(31, int32(totsh[i]-totsh[ind]))
				if (mul16(corr16[ind], en16[i]) >> uint(sh)) < mul16(corr16[i], en16[ind]) {
					ind = i
				}
			}
		}

		lag = lagmax[ind] + 10

		// Store the estimated lag in the non-downsampled domain.
		enhPeriod[EnhNBlocksTot-newBlocks+iBlock] = lag * 8

		// Store the estimated lag for backward PLC.
		if dec.PrevEnhPL == 1 {
			if iBlock == 0 {
				tlag = lag * 2
			}
		} else if iBlock == 1 {
			tlag = lag * 2
		}

		lag *= 2
	}

	if dec.PrevEnhPL == 1 || dec.PrevEnhPL == 2 {
		// Calculate the best lag of the new frame; it is used to
		// interpolate backwards and mix with the PLC'd data.
		regOff := tlag - 1

		// Scaling.
		max16 := maxAbsValueW16(in[regOff:regOff+plcBlockL+2], plcBlockL+3-1)
		shifts := 0
		if max16 > 5000 {
			shifts = 2
		}

		// Compute the cross correlation.
		crossCorrelation(corr32[:], in[:plcBlockL], in, regOff, plcBlockL, 3, shifts, 1)

		// Find the lag.
		lag = maxIndexW32(corr32[:], 3)
		lag += tlag - 1

		// Copy the backward PLC to plc_pred.
		if dec.PrevEnhPL == 1 {
			if lag > plcBlockL {
				copy(plcPred, in[lag-plcBlockL:lag-plcBlockL+plcBlockL])
			} else {
				copy(plcPred[plcBlockL-lag:], in[:lag])
				copy(plcPred[:plcBlockL-lag],
					enhBuf[EnhBufl-blockL-plcBlockL+lag:EnhBufl-blockL])
			}
		} else {
			pos := plcBlockL
			for lag < pos {
				copy(plcPred[pos-lag:], in[:lag])
				pos -= lag
			}
			copy(plcPred[:pos], in[lag-pos:lag-pos+pos])
		}

		if dec.PrevEnhPL == 1 {
			// Limit the energy change: if the energy of the backward PLC is
			// more than 4 times higher than the forward PLC, reduce the
			// energy of the backward PLC vector (window ramp over the last
			// 16 samples).
			max32 := maxAbsValueW16(enhBuf[EnhBufl-blockL-plcBlockL:EnhBufl-blockL], plcBlockL)
			maxPred := maxAbsValueW16(plcPred[:plcBlockL], plcBlockL)
			if maxPred > max32 {
				max32 = maxPred
			}
			scale := 22 - int16(normW32(int32(max32)))
			if scale < 0 {
				scale = 0
			}

			fwdEner := dotProductWithScale(
				enhBuf[EnhBufl-blockL-plcBlockL:EnhBufl-blockL],
				enhBuf[EnhBufl-blockL-plcBlockL:EnhBufl-blockL], plcBlockL, int(scale))
			bwEner := dotProductWithScale(plcPred[:plcBlockL], plcPred[:plcBlockL], plcBlockL, int(scale))

			// Check the energy difference.
			if bwEner > 0 && (bwEner>>2) > fwdEner {
				// EnChange is now guaranteed to be <0.5; compute it in Q16.
				scale1 := int16(normW32(bwEner))
				bwScaled := splShiftW32(bwEner, int32(scale1-16)) // using 15 bits
				fwdScaled := splShiftW32(fwdEner, int32(scale1))

				enChange := divW32W16(fwdScaled, w16(bwScaled))

				// Sqrt of the energy in Q15 ((14+16)/2).
				sqrtEnChange := w16(sqrtFloor(int32(enChange) << 14))

				// Multiply the first part of the vector with 2*SqrtEnChange.
				scaleVector(plcPred[:], plcPred[:], sqrtEnChange, plcBlockL-16, 14)

				// Increase parameter for the window part: (1-2*SqrtEnChange)/16 in Q15.
				inc := int16(2048 - (sqrtEnChange >> 3))

				win := int16(0)
				ptr := plcBlockL - 16
				for i := 16; i > 0; i-- {
					plcPred[ptr] = w16(int32(plcPred[ptr]) * int32(sqrtEnChange+(win>>1)) >> 14)
					win += inc
					ptr++
				}
			}

			// Linear interpolation between the forward PLC'd data and the
			// backward PLC'd data (from the new frame).
			var inc int16
			if plcBlockL == 40 {
				inc = 400 // 1/41 in Q14
			} else { // plcBlockL==80
				inc = 202 // 1/81 in Q14
			}
			win := int16(0)
			p := EnhBufl - 1 - blockL
			for i := 0; i < plcBlockL; i++ {
				win += inc
				v := int32(enhBuf[p]) * int32(win) >> 14
				v += int32((16384 - int32(win)) * int32(plcPred[plcBlockL-1-i]) >> 14)
				enhBuf[p] = w16(v)
				p--
			}
		} else {
			// Synthesis of the PLC segment with the saved filter state.
			synt := downSampled[:LPCFilterOrder+EnhBlockL]

			src := EnhBufl - blockL - plcBlockL
			copy(enhBuf[src:src+plcBlockL], plcPred[:plcBlockL])

			// Clear the filter memory.
			memSetW16(dec.SyntMem[:], 0, LPCFilterOrder, 0)
			memSetW16(dec.HpInY[:], 0, 4, 0)
			memSetW16(dec.HpInX[:], 0, 2, 0)

			coeffs := dec.OldSyntDenum[(dec.NSub-1)*(LPCFilterOrder+1) : (dec.NSub)*(LPCFilterOrder+1)]

			// Initialize the filter memory by filtering through two lags.
			copy(synt[:LPCFilterOrder], dec.SyntMem[:])
			filterARFastQ12(enhBuf[src:], synt, 0, coeffs, LPCFilterOrder+1, lag)

			copy(synt[:LPCFilterOrder], synt[lag-LPCFilterOrder:lag])
			hpOutput(synt, kHpOutCoefs[:], dec.HpInY[:], dec.HpInX[:], lag)
			filterARFastQ12(enhBuf[src:], synt, 0, coeffs, LPCFilterOrder+1, lag)

			copy(dec.SyntMem[:], synt[lag-LPCFilterOrder:lag])
			hpOutput(synt, kHpOutCoefs[:], dec.HpInY[:], dec.HpInX[:], lag)
		}
	}

	// Perform the enhancement block by block.
	if StageDebug {
		for chunk := 0; chunk+80 <= len(enhBuf); chunk += 80 {
			stagePrint(fmt.Sprintf("EHB data[%d:%d]:", chunk, chunk+80), enhBuf[chunk:chunk+80])
		}
	}
	for iBlock := 0; iBlock < newBlocks; iBlock++ {
		enhancer(out[iBlock*EnhBlockL:(iBlock+1)*EnhBlockL], enhBuf, EnhBufl,
			iBlock*EnhBlockL+startPos, enhPeriod[:], kEnhPlocs[:], EnhNBlocksTot)
	}

	return lag
}
