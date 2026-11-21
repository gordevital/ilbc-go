package ilbcfix

// File: api.go — public entry points mirroring ilbc.c
// (WebRtcIlbcfix_DecoderInit / _Decode / _DecodePlc / _NetEqPlc).

// InitDecoder initializes the decoder for mode 20 or 30 with the enhancer
// enabled, mirroring WebRtcIlbcfix_DecoderInit. Returns 0 on success and -1
// for an unsupported mode.
func InitDecoder(inst *DecInst, mode int) int {
	if mode == 20 || mode == 30 {
		initDecode(inst, mode, 1)
		return 0
	}
	return -1
}

// InitEncoder initializes the encoder for mode 20 or 30. It returns the
// number of payload bytes (NoOfBytes) on success and -1 for an unsupported
// mode, mirroring WebRtcIlbcfix_InitEncode.
func InitEncoder(inst *EncInst, mode int) int {
	return initEncode(inst, mode)
}

// Decode decodes one to three consecutive frames from encoded (whose length
// must be k*inst.NoOfBytes for k=1..3) into decoded (which must hold
// k*inst.BlockL samples). The frame mode switches automatically when the
// byte length belongs to the other mode, mirroring WebRtcIlbcfix_Decode.
// On success it returns the number of decoded samples and sets *speechType
// to 1 (iLBC does not support VAD/CNG); it returns -1 on an unsupported
// length or a corrupted frame.
func Decode(inst *DecInst, encoded []byte, decoded []int16, speechType *int16) int {
	// Allow for automatic switching between the frame sizes (although you
	// do get some discontinuity).
	switch {
	case len(encoded) == inst.NoOfBytes || len(encoded) == 2*inst.NoOfBytes ||
		len(encoded) == 3*inst.NoOfBytes:
		// ok, do nothing
	case inst.Mode == 20:
		if len(encoded) == NoOfBytes30ms || len(encoded) == 2*NoOfBytes30ms ||
			len(encoded) == 3*NoOfBytes30ms {
			initDecode(inst, 30, inst.UseEnhancer)
		} else {
			return -1 // unsupported frame length
		}
	default:
		if len(encoded) == NoOfBytes20ms || len(encoded) == 2*NoOfBytes20ms ||
			len(encoded) == 3*NoOfBytes20ms {
			initDecode(inst, 20, inst.UseEnhancer)
		} else {
			return -1 // unsupported frame length
		}
	}

	var words [NoOfWords30ms]uint16
	i := 0
	for i*inst.NoOfBytes < len(encoded) {
		off := 2 * i * inst.NoOfWords
		for k := 0; k < inst.NoOfWords; k++ {
			words[k] = uint16(encoded[off+2*k]) | uint16(encoded[off+2*k+1])<<8
		}
		if decodeImpl(decoded[i*inst.BlockL:], words[:inst.NoOfWords], inst, 1) == -1 {
			return -1
		}
		i++
	}
	// iLBC does not support VAD/CNG yet.
	*speechType = 1
	return i * inst.BlockL
}

// DecodePlc produces noOfLostFrames worth of PLC output in decoded,
// mirroring WebRtcIlbcfix_DecodePlc. It returns the number of decoded
// samples. PLC decoding cannot fail (there is no external input data that
// can be bad); the C reference guards the internal failure with RTC_CHECK,
// so the Go version aborts as well.
func DecodePlc(inst *DecInst, decoded []int16, noOfLostFrames int) int {
	for i := 0; i < noOfLostFrames; i++ {
		if r := decodeImpl(decoded[i*inst.BlockL:], nil, inst, 0); r != 0 {
			panic("ilbcfix: PLC decoding failed unexpectedly")
		}
	}
	return noOfLostFrames * inst.BlockL
}

// NetEqPlc marks the decoder state after a packet loss handled outside the
// decoder (NetEQ) and clears the enhancer buffer, mirroring
// WebRtcIlbcfix_NetEqPlc. decoded and noOfLostFrames exist for signature
// compatibility with the C callback table and are ignored.
func NetEqPlc(inst *DecInst, decoded []int16, noOfLostFrames int) int {
	_ = decoded
	_ = noOfLostFrames
	memSetW16(inst.EnhBuf[:], 0, EnhBufl, 0)
	inst.PrevEnhPL = 2
	return 0
}
