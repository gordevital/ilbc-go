package ilbcfix

import (
	"fmt"
	"os"
)

// File: decode.go — 1:1 transcription of decode.c (WebRtcIlbcfix_DecodeImpl).

// StageDebug toggles temporary stage dumps (stderr) for C-parity debugging.
var StageDebug bool

func stagePrint(tag string, vals ...any) {
	fmt.Fprint(os.Stderr, tag)
	for _, v := range vals {
		switch vv := v.(type) {
		case []int16:
			for _, x := range vv {
				fmt.Fprintf(os.Stderr, " %d", x)
			}
		default:
			fmt.Fprintf(os.Stderr, " %v", vv)
		}
	}
	fmt.Fprintln(os.Stderr)
}

// decodeImpl decodes one frame of dec.BlockL samples into decBlock.
//
// words carries the frame bitstream as dec.NoOfWords 16-bit words in the
// wire byte layout (each word serialized as two little-endian bytes, the
// layout produced by encodeImpl); the word bytes are swapped back before
// unpacking, mirroring the C reference compiled for little-endian hosts.
//
// mode selects 1 = good packet, 0 = packet loss. A malformed bitstream
// transparently falls back to PLC. Returns 0 on success and -1 when
// residual decoding failed and the decoder state was re-initialised.
func decodeImpl(decBlock []int16, words []uint16, dec *DecInst, mode int) int {
	oldMode := dec.Mode
	oldUseEnhancer := dec.UseEnhancer

	var decResidual [BlockLMax]int16
	var plcResidual [BlockLMax + LPCFilterOrder]int16
	var syntdenum [NSubMax * (LPCFilterOrder + 1)]int16
	var plcLpc [LPCFilterOrder + 1]int16
	var swapped [NoOfWords30ms]uint16
	var bitStream [NoOfWords30ms]int16
	var bits iLBCBits

	// The C reference overlays the iLBC_bits struct on top of PLCresidual
	// to save stack; the aliased regions are never live at the same time,
	// so the Go version keeps the storages separate with identical behavior.
	data := plcResidual[LPCFilterOrder:]

	lastBit := int16(0)

	if StageDebug {
		stagePrint(fmt.Sprintf("P0 mode=%d prev_enh_pl=%d last_lag=%d use_enh=%d lpc_n=%d nsub=%d",
			dec.Mode, dec.PrevEnhPL, dec.LastLag, dec.UseEnhancer, dec.LpcN, dec.NSub))
	}

	if mode != 0 { // the data are good

		// Unpack the bits into parameters.
		swapBytes(words[:dec.NoOfWords], dec.NoOfWords, swapped[:])
		for i := 0; i < dec.NoOfWords; i++ {
			bitStream[i] = int16(swapped[i])
		}
		lastBit = unpackBits(bitStream[:dec.NoOfWords], &bits, dec.Mode)
		if StageDebug {
			stagePrint(fmt.Sprintf("UB si=%d sf=%d fx=%d lb=%d", bits.StartIdx, int(bits.StateFirst), bits.IdxForMax, int(lastBit)))
			cbl := ""
			for i := 0; i < 15; i++ {
				cbl += fmt.Sprintf("cb%d=%d ", i, bits.CbIndex[i])
			}
			stagePrint("UB " + cbl)
			gil := ""
			for i := 0; i < 15; i++ {
				gil += fmt.Sprintf("gi%d=%d ", i, bits.GainIndex[i])
			}
			stagePrint("UB " + gil)
		}

		// Check for bit errors.
		if bits.StartIdx < 1 {
			mode = 0
		}
		if dec.Mode == 20 && bits.StartIdx > 3 {
			mode = 0
		}
		if dec.Mode == 30 && bits.StartIdx > 5 {
			mode = 0
		}
		if lastBit == 1 {
			mode = 0
		}
		if StageDebug {
			stagePrint(fmt.Sprintf("SX startIdx=%d", bits.StartIdx))
		}

		if mode != 0 { // no bit errors were detected, continue decoding
			var lsfDeq [LPCFilterOrder * LpcNMax]int16
			var weightDenum [(LPCFilterOrder + 1) * NSubMax]int16

			// Adjust the indices.
			indexConvDec(bits.CbIndex[:])

			// Decode the LSFs.
			simpleLsfDeq(lsfDeq[:], bits.Lsf[:], dec.LpcN)
			lsfCheck(lsfDeq[:], LPCFilterOrder, dec.LpcN)
			decoderInterpolateLsp(syntdenum[:], weightDenum[:], lsfDeq[:], LPCFilterOrder, dec)
			if StageDebug {
				stagePrint(fmt.Sprintf("P1 lpcN=%d nsub=%d", dec.LpcN, dec.NSub))
				for r := 0; r < dec.NSub; r++ {
					row := syntdenum[r*(LPCFilterOrder+1) : (r+1)*(LPCFilterOrder+1)]
					stagePrint(fmt.Sprintf("P1 sd[%d]:", r), row)
					wrow := weightDenum[r*(LPCFilterOrder+1) : (r+1)*(LPCFilterOrder+1)]
					stagePrint(fmt.Sprintf("P1 wd[%d]:", r), wrow)
				}
				for r := 0; r < NSubMax; r++ {
					stagePrint(fmt.Sprintf("P1 osd[%d]:", r),
						dec.OldSyntDenum[r*(LPCFilterOrder+1):(r+1)*(LPCFilterOrder+1)])
				}
			}

			// Decode the residual using the cb and gain indices.
			if !decodeResidual(dec, &bits, decResidual[:], syntdenum[:]) {
				// The decoder got sick from eating that data. Reset it.
				initDecode(dec, oldMode, oldUseEnhancer)
				return -1
			}

			// Preparing the PLC for a future loss!
			doPlc(plcResidual[:], plcLpc[:], 0, decResidual[:],
				syntdenum[(dec.NSub-1)*(LPCFilterOrder+1):], dec.LastLag, dec)

			// Use the output of doThePLC.
			copy(decResidual[:dec.BlockL], plcResidual[:dec.BlockL])

			if StageDebug {
				for s := 0; s < dec.BlockL; s += 80 {
					e := s + 80
					if e > dec.BlockL {
						e = dec.BlockL
					}
					stagePrint(fmt.Sprintf("DR data[%d:%d]:", s, e), plcResidual[s:e])
				}
			}
			if StageDebug {
				stagePrint(fmt.Sprintf("P2 lastLag=%d cons=%d PLCLPC:", dec.LastLag, dec.ConsPLICount), plcLpc[:])
				b := dec.BlockL
				for s := 0; s < b; s += 80 {
					e := s + 80
					if e > b {
						e = b
					}
					stagePrint(fmt.Sprintf("P2 data[%d:%d]:", s, e), decResidual[s:e])
				}
			}
		}
	}

	if mode == 0 {
		// The data is bad (either a PLC call was made or a bit error was
		// detected): packet loss concealment.
		doPlc(plcResidual[:], plcLpc[:], 1, decResidual[:], syntdenum[:], dec.LastLag, dec)

		copy(decResidual[:dec.BlockL], plcResidual[:dec.BlockL])

		orderPlusOne := LPCFilterOrder + 1
		for i := 0; i < dec.NSub; i++ {
			copy(syntdenum[i*orderPlusOne:i*orderPlusOne+orderPlusOne], plcLpc[:])
		}
	}

	if dec.UseEnhancer == 1 { // enhancer activated

		// Update the filter and filter coefficients if there was a packet loss.
		if dec.PrevEnhPL == 2 {
			for i := 0; i < dec.NSub; i++ {
				copy(dec.OldSyntDenum[i*(LPCFilterOrder+1):(i+1)*(LPCFilterOrder+1)],
					syntdenum[:LPCFilterOrder+1])
			}
		}

		// Post filtering.
		dec.LastLag = enhancerInterface(data, decResidual[:], dec)
		if StageDebug {
			stagePrint(fmt.Sprintf("P3 lastLag=%d prevEnhPL=%d", dec.LastLag, dec.PrevEnhPL))
			stagePrint("P3 data[0:16]:", data[:16])
			stagePrint("P3 data[76:84]:", data[76:84])
		}

		// Synthesis filtering.
		// Set up the filter state.
		copy(plcResidual[:LPCFilterOrder], dec.SyntMem[:])

		if dec.Mode == 20 {
			// Enhancer has 40 samples delay.
			filterARFastQ12(plcResidual[10:], plcResidual[:], 10,
				dec.OldSyntDenum[(dec.NSub-1)*(LPCFilterOrder+1):(dec.NSub)*(LPCFilterOrder+1)],
				LPCFilterOrder+1, SubL)
			for i := 1; i < dec.NSub; i++ {
				filterARFastQ12(plcResidual[10+i*SubL:], plcResidual[:], 10+i*SubL,
					syntdenum[(i-1)*(LPCFilterOrder+1):i*(LPCFilterOrder+1)],
					LPCFilterOrder+1, SubL)
			}
		} else if dec.Mode == 30 {
			// Enhancer has 80 samples delay.
			for i := 0; i < 2; i++ {
				filterARFastQ12(plcResidual[10+i*SubL:], plcResidual[:], 10+i*SubL,
					dec.OldSyntDenum[(i+4)*(LPCFilterOrder+1):(i+5)*(LPCFilterOrder+1)],
					LPCFilterOrder+1, SubL)
			}
			for i := 2; i < dec.NSub; i++ {
				filterARFastQ12(plcResidual[10+i*SubL:], plcResidual[:], 10+i*SubL,
					syntdenum[(i-2)*(LPCFilterOrder+1):(i-1)*(LPCFilterOrder+1)],
					LPCFilterOrder+1, SubL)
			}
		}

		// Save the filter state.
		copy(dec.SyntMem[:], plcResidual[dec.BlockL:dec.BlockL+LPCFilterOrder])
		if StageDebug {
			stagePrint("P4 data[0:16]:", data[:16])
			stagePrint("P4 data[76:88]:", data[76:88])
		}
	} else { // enhancer not activated

		// Find last lag (since the enhancer is not called to give this info).
		lag := 20
		if dec.Mode == 20 {
			lag = xcorrCoef(decResidual[:], dec.BlockL-60, dec.BlockL-60-lag, 60, 80, lag, -1)
		} else {
			lag = xcorrCoef(decResidual[:], dec.BlockL-EnhBlockL, dec.BlockL-EnhBlockL-lag,
				EnhBlockL, 100, lag, -1)
		}

		// Store lag (it is needed if the next packet is lost).
		dec.LastLag = lag

		// Copy data and run the synthesis filter.
		copy(data[:dec.BlockL], decResidual[:dec.BlockL])

		// Set up the filter state.
		copy(plcResidual[:LPCFilterOrder], dec.SyntMem[:])

		for i := 0; i < dec.NSub; i++ {
			filterARFastQ12(plcResidual[10+i*SubL:], plcResidual[:], 10+i*SubL,
				syntdenum[i*(LPCFilterOrder+1):(i+1)*(LPCFilterOrder+1)],
				LPCFilterOrder+1, SubL)
		}

		// Save the filter state.
		copy(dec.SyntMem[:], plcResidual[dec.BlockL:dec.BlockL+LPCFilterOrder])
	}

	copy(decBlock, data[:dec.BlockL])

	// High-pass filter the signal (with upscaling factor 2 and saturation).
	hpOutput(decBlock, kHpOutCoefs[:], dec.HpInY[:], dec.HpInX[:], dec.BlockL)
	if StageDebug {
		stagePrint("P5 out[0:16]:", decBlock[:16])
		stagePrint("P5 out[76:88]:", decBlock[76:88])
	}

	copy(dec.OldSyntDenum[:dec.NSub*(LPCFilterOrder+1)],
		syntdenum[:dec.NSub*(LPCFilterOrder+1)])

	dec.PrevEnhPL = 0
	if mode == 0 { // PLC was used
		dec.PrevEnhPL = 1
	}

	return 0
}
