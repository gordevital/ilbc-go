package ilbcfix

// File: plc.go — do_plc.c, comp_corr.c, xcorr_coef.c.

import (
	"fmt"
)

func minI32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// shiftW32 mirrors WEBRTC_SPL_SHIFT_W32.
func shiftW32(x, s int32) int32 {
	if s >= 0 {
		return shl(x, s)
	}
	return shr(x, -s)
}

// compCorr mirrors WebRtcIlbcfix_CompCorr.
func compCorr(buffer []int16, lag, bLen, sRange int, scale int16) (int32, int32) {
	w := bLen - sRange - lag
	corr := dotProductWithScale(buffer[bLen-sRange:], buffer[w:], sRange, int(scale))
	ener := dotProductWithScale(buffer[w:], buffer[w:], sRange, int(scale))
	if ener == 0 {
		corr = 0
		ener = 1
	}
	return corr, ener
}

// doPlc mirrors WebRtcIlbcfix_DoThePlc.
func doPlc(plcResidual, plcLpc []int16, pli int16, decresidual, lpc []int16,
	inlag int, dec *DecInst) {

	var randvec [BlockLMax]int16

	if pli == 1 {
		dec.ConsPLICount++

		var lag int
		var maxPerSquare int16

		if dec.PrevPLI != 1 {
			max := maxAbsValueW16(dec.PrevResidual[:], dec.BlockL)
			scale3 := (getSizeInBits(uint32(max)) << 1) - 25
			if scale3 < 0 {
				scale3 = 0
			}
			dec.PrevScale = int16(scale3)

			lag = inlag - 3
			corrLen := 60
			if dec.BlockL-(inlag+3) < corrLen {
				corrLen = dec.BlockL - (inlag + 3)
			}

			cross, ener := compCorr(dec.PrevResidual[:], lag, dec.BlockL, corrLen, int16(scale3))

			shiftMax := getSizeInBits(uint32(splAbsW32(cross))) - 15
			crossSquareMax := int16((mul16(int16(shiftW32(cross, -shiftMax)), int16(shiftW32(cross, -shiftMax)))) >> 15)

			var enerComp int32
			for j := inlag - 2; j <= inlag+3; j++ {
				var crossComp int32
				crossComp, enerComp = compCorr(dec.PrevResidual[:], j, dec.BlockL, corrLen, int16(scale3))

				shift1 := getSizeInBits(uint32(splAbsW32(crossComp))) - 15
				crossSquare := int16((mul16(int16(shiftW32(crossComp, -shift1)), int16(shiftW32(crossComp, -shift1)))) >> 15)

				shift2 := getSizeInBits(uint32(ener)) - 15
				measure := mul16(int16(shiftW32(ener, -shift2)), crossSquare)

				shift3 := getSizeInBits(uint32(enerComp)) - 15
				maxMeasure := mul16(int16(shiftW32(enerComp, -shift3)), crossSquareMax)

				var tmp1, tmp2 int16
				if 2*shiftMax+shift3 > 2*shift1+shift2 {
					tmp1 = int16(minI32(31, 2*shiftMax+shift3-2*shift1-shift2))
					tmp2 = 0
				} else {
					tmp1 = 0
					tmp2 = int16(minI32(31, 2*shift1+shift2-2*shiftMax-shift3))
				}

				if shr(measure, int32(tmp1)) > shr(maxMeasure, int32(tmp2)) {
					lag = j
					crossSquareMax = crossSquare
					cross = crossComp
					shiftMax = shift1
					ener = enerComp
				}
			}

			tmp2W32 := dotProductWithScale(dec.PrevResidual[dec.BlockL-corrLen:],
				dec.PrevResidual[dec.BlockL-corrLen:], corrLen, int(scale3))

			if tmp2W32 > 0 && enerComp > 0 {
				scale1 := int16(normW32(tmp2W32) - 16)
				tmp1 := int16(shiftW32(tmp2W32, int32(scale1)))

				scale2 := int16(normW32(ener) - 16)
				tmp2 := int16(shiftW32(ener, int32(scale2)))
				denom := int16((mul16(tmp1, tmp2)) >> 16)

				totscale := scale1 + scale2 - 1
				tmp1 = int16(shiftW32(cross, int32(totscale>>1)))
				tmp2 = int16(shiftW32(cross, int32(totscale-(totscale>>1))))

				nom := mul16(tmp1, tmp2)
				maxPerSquare = int16(divW32W16(nom, denom))
			} else {
				maxPerSquare = 0
			}
		} else {
			lag = dec.PrevLag
			maxPerSquare = dec.PerSquare
		}

		useGain := int16(32767)
		if dec.ConsPLICount*dec.BlockL > 320 {
			useGain = 29491
		} else if dec.ConsPLICount*dec.BlockL > 640 {
			useGain = 22938
		} else if dec.ConsPLICount*dec.BlockL > 960 {
			useGain = 16384
		} else if dec.ConsPLICount*dec.BlockL > 1280 {
			useGain = 0
		}

		var pitchfact int16
		if maxPerSquare > 7868 {
			pitchfact = 32767
		} else if maxPerSquare > 839 {
			ind := 5
			for maxPerSquare < kPlcPerSqr[ind] && ind > 0 {
				ind--
			}
			tmpW32 := int32(kPlcPitchFact[ind]) +
				((int32(kPlcPfSlope[ind]) * int32(maxPerSquare-int16(kPlcPerSqr[ind]))) >> 11)
			pitchfact = int16(minI32(tmpW32, 32767))
		} else {
			pitchfact = 0
		}

		useLag := lag
		if lag < 80 {
			useLag = 2 * lag
		}
		if StageDebug {
			stagePrint(fmt.Sprintf("PL inlag=%d lag=%d useLag=%d maxPer=%d pf=%d cons=%d seed=%d scale3saved=%d",
				inlag, lag, useLag, int(maxPerSquare), int(pitchfact), dec.ConsPLICount, int(dec.Seed), int(dec.PrevScale)))
			for prs := 0; prs < dec.BlockL; prs += 80 {
				pre := prs + 80
				if pre > dec.BlockL {
					pre = dec.BlockL
				}
				stagePrint(fmt.Sprintf("PPR data[%d:%d]:", prs, pre), dec.PrevResidual[prs:pre])
			}
		}

		noiseEnergyThreshold := int32(dec.BlockL) * 900
		var energy int32
		for i := 0; i < dec.BlockL; i++ {
			dec.Seed = int16(int32(dec.Seed)*31821 + 13849)
			randlag := 53 + int(dec.Seed&63)
			if randlag > i {
				randvec[i] = dec.PrevResidual[dec.BlockL+i-randlag]
			} else {
				randvec[i] = dec.PrevResidual[i-randlag]
			}

			if useLag > i {
				plcResidual[i] = dec.PrevResidual[dec.BlockL+i-useLag]
			} else {
				plcResidual[i] = plcResidual[i-useLag]
			}

			var totGain int16
			if i < 80 {
				totGain = useGain
			} else if i < 160 {
				totGain = int16((31130 * int32(useGain)) >> 15)
			} else {
				totGain = int16((29491 * int32(useGain)) >> 15)
			}

			plcResidual[i] = int16((int32(totGain) *
				((int32(pitchfact)*int32(plcResidual[i]) +
					int32(32767-pitchfact)*int32(randvec[i]) + 16384) >> 15)) >> 15)

			if energy < noiseEnergyThreshold {
				energy += int32(plcResidual[i]) * int32(plcResidual[i])
			}
		}

		if energy < noiseEnergyThreshold {
			copy(plcResidual[:dec.BlockL], randvec[:dec.BlockL])
		}

		copy(plcLpc, dec.PrevLpc[:LPCFilterOrder+1])

		dec.PrevLag = lag
		dec.PerSquare = maxPerSquare
	} else {
		copy(plcResidual[:dec.BlockL], decresidual[:dec.BlockL])
		copy(plcLpc, lpc[:LPCFilterOrder+1])
		dec.ConsPLICount = 0
	}

	dec.PrevPLI = pli
	copy(dec.PrevLpc[:], plcLpc[:LPCFilterOrder+1])
	copy(dec.PrevResidual[:dec.BlockL], plcResidual[:dec.BlockL])
}

// xcorrCoef mirrors WebRtcIlbcfix_XcorrCoef.
//
// buf holds both signals: the C target and regressor pointers correspond to
// buf[targetOff] and buf[regOff]. For step -1 the function reads
// buf[regOff-searchLen .. regOff+subl-1]; for step +1 it reads
// buf[regOff .. regOff+subl+searchLen-1].
func xcorrCoef(buf []int16, targetOff, regOff, subl, searchLen, offset int, step int16) int {
	crossCorrSqModMax := int16(0)
	energyModMax := int16(word16Max)
	totscaleMax := int16(-500)
	maxlag := 0
	pos := 0

	var maxVal int16
	rpBeg, rpEnd := 0, 0
	if step == 1 {
		maxVal = maxAbsValueW16(buf[regOff:regOff+subl+searchLen-1], subl+searchLen-1)
		rpBeg = regOff
		rpEnd = regOff + subl
	} else {
		maxVal = maxAbsValueW16(buf[regOff-searchLen:regOff-searchLen+subl+searchLen-1], subl+searchLen-1)
		rpBeg = regOff - 1
		rpEnd = regOff + subl - 1
	}

	shifts := 0
	if maxVal > 5000 {
		shifts = 2
	}

	energy := dotProductWithScale(buf[regOff:regOff+subl], buf[regOff:regOff+subl], subl, shifts)

	for k := 0; k < searchLen; k++ {
		crossCorr := dotProductWithScale(buf[targetOff:targetOff+subl], buf[regOff+pos:regOff+pos+subl], subl, shifts)

		if energy > 0 && crossCorr > 0 {
			crossCorrScale := int16(normW32(crossCorr) - 16)
			crossCorrmod := int16(shiftW32(crossCorr, int32(crossCorrScale)))
			energyScale := int16(normW32(energy) - 16)
			energyMod := int16(shiftW32(energy, int32(energyScale)))

			crossCorrSqMod := int16((mul16(crossCorrmod, crossCorrmod)) >> 16)

			totscale := energyScale - (crossCorrScale << 1)

			scalediff := totscale - totscaleMax
			scalediff = int16(minI32(int32(scalediff), 31))
			scalediff = int16(maxI32(int32(scalediff), -31))

			var newCrit, maxCrit int32
			if scalediff < 0 {
				newCrit = mulW32(int32(crossCorrSqMod), int32(energyModMax)) >> uint(-scalediff)
				maxCrit = mulW32(int32(crossCorrSqModMax), int32(energyMod))
			} else {
				newCrit = mulW32(int32(crossCorrSqMod), int32(energyModMax))
				maxCrit = mulW32(int32(crossCorrSqModMax), int32(energyMod)) >> uint(scalediff)
			}

			if newCrit > maxCrit {
				crossCorrSqModMax = crossCorrSqMod
				energyModMax = energyMod
				totscaleMax = totscale
				maxlag = k
			}
		}

		energy += int32(step) * ((int32(buf[rpEnd])*int32(buf[rpEnd]) -
			int32(buf[rpBeg])*int32(buf[rpBeg])) >> uint(shifts))
		pos += int(step)
		rpBeg += int(step)
		rpEnd += int(step)
	}

	return maxlag + offset
}
