// Package ilbcfix is a 1:1 transcription of the WebRTC libilbc fixed-point
// C implementation (modules/audio_coding/codecs/ilbc, BSD license) into Go.
//
// Transcription conventions (must be followed exactly for bit-exactness):
//   - C int16_t/int32_t map to Go int16/int32; C size_t maps to Go int.
//   - Any C arithmetic over int16_t values happens in the int (32-bit)
//     domain; the result is truncated back to int16 only where C assigns
//     into an int16_t lvalue (use w16()).
//   - int32/int16 addition, subtraction and multiplication wrap around,
//     exactly like gcc -fwrapv semantics on the C reference build.
//   - Shifts with a runtime count use shl/shr (x86 shift-count masking,
//     matching how gcc compiles the C reference on x86-64); shifts with
//     in-range constant counts may use Go << / >> directly.
//   - Function names keep the C names minus the WebRtcIlbcfix_ prefix.
package ilbcfix

// Geometry constants (defines.h).
const (
	fs8kHz            = 8000
	BlockL20ms        = 160
	BlockL30ms        = 240
	BlockLMax         = 240
	NSub20ms          = 4
	NSub30ms          = 6
	NSubMax           = 6
	NaSub20ms         = 2
	NaSub30ms         = 4
	NaSubMax          = 4
	SubL              = 40
	StateLen          = 80
	StateShortLen30ms = 58
	StateShortLen20ms = 57
)

// LPC settings (defines.h).
const (
	LPCFilterOrder = 10
	LPCLookBack    = 60
	LpcN20ms       = 1
	LpcN30ms       = 2
	LpcNMax        = 2
	LpcAsymDiff    = 20
	LSFNSplit      = 3
	LSFNumSteps    = 4
	LPCHalfOrder   = 5
	CosGridPoints  = 60
)

// Codebook settings (defines.h).
const (
	CBNStages      = 3
	CBExpand       = 2
	CBMemL         = 147
	CBFilterLen    = 2 * 4
	CBHalfFilter   = 4
	CBResRange     = 34
	CBMaxGainFixQ6 = 83 // error = -0.24%
)

// Enhancer settings (defines.h). Note ENH_NBLOCKS = BLOCKL/ENH_BLOCKL is
// mode dependent (2 for 20 ms, 3 for 30 ms) — see enhNBlocksFor.
const (
	EnhBlockL            = 80
	EnhBlockLHalf        = 40
	EnhHL                = 3
	EnhSlop              = 2
	EnhPlocsL            = 8
	EnhOverhang          = 2
	EnhUPS0              = 4
	EnhFL0               = 3
	EnhFLOMult2Plus1     = 7
	EnhVECTL             = EnhBlockL + 2*EnhFL0
	EnhCorrDim           = 2*EnhSlop + 1
	EnhNBlocksExtra      = 5
	EnhNBlocksTot        = 8 // EnhNBlocks(30ms) + EnhNBlocksExtra
	EnhBufl              = EnhNBlocksTot * EnhBlockL
	EnhBuflFilterOVRhead = 3
)

// enhNBlocksFor returns ENH_NBLOCKS for a frame length (BLOCKL/ENH_BLOCKL).
func enhNBlocksFor(blockl int) int { return blockl / EnhBlockL }

// EnhA0 constants (defines.h).
const (
	enhA0          = 819       // Q14
	enhA0MinusA0A0 = 848256041 // ENH_A0_MINUS_A0A0DIV4, Q34
	enhA0Div2      = 26843546  // Q30
)

// PLC / downsampling / bitstream / help constants (defines.h).
const (
	FilterOrderDSPlus1 = 7
	DelayDS            = 3
	FactorDS           = 2
	NoOfBytes20ms      = 38
	NoOfBytes30ms      = 50
	NoOfWords20ms      = 19
	NoOfWords30ms      = 25
	StateBits          = 3
	ByteLen            = 8
	ULPClasses         = 3
	TwoPiFix           = 25736 // Q12
	stMemLTbl          = 85
	memLfTbl           = 147
)

// SPL numeric limits.
const (
	word16Max = 32767
	word16Min = -32768
	word32Max = int32(0x7fffffff)
)

// word32Min is INT32_MIN as an int32 constant.
const word32Min = int32(-2147483648)

// w16 truncates an int32 to int16 exactly like a C (int16_t) cast.
func w16(x int32) int16 { return int16(x) }

// w32 truncates a 64-bit value to int32 like a C (int32_t) cast.
func w32(x int64) int32 { return int32(x) }

// mul16 multiplies two int16 in the 32-bit int domain (C integer promotion).
func mul16(a, b int16) int32 { return int32(a) * int32(b) }

// mulW32 multiplies two int32 with wraparound (WEBRTC_SPL_MUL).
func mulW32(a, b int32) int32 { return a * b }

// shl shifts left with x86 shift-count masking (gcc behavior).
func shl(x int32, c int32) int32 {
	if c >= 32 {
		return x << (uint32(c) & 31)
	}
	return x << uint32(c)
}

// shr shifts right (arithmetic) with x86 shift-count masking.
func shr(x int32, c int32) int32 {
	if c >= 32 {
		return x >> (uint32(c) & 31)
	}
	return x >> uint32(c)
}

// shl16/shr16 are shl/shr for the int16 domain; the operand is promoted to
// int32 first, as in C, and the truncated result must be assigned with w16.
func shl16(x int16, c int32) int32 { return shl(int32(x), c) }
func shr16(x int16, c int32) int32 { return shr(int32(x), c) }

// splSat is WEBRTC_SPL_SAT(a, b, c) = clamp b into [c, a].
func splSat(a, b, c int32) int32 {
	if b > a {
		return a
	}
	if b < c {
		return c
	}
	return b
}

// splShiftW32 is WEBRTC_SPL_SHIFT_W32(x, c):
// c >= 0 ? x * (1 << c) : x >> -c.
func splShiftW32(x, c int32) int32 {
	if c >= 0 {
		return x * shl(1, c)
	}
	return shr(x, -c)
}

// splSatW32ToW16 saturates an int32 into int16 range.
func splSatW32ToW16(v int32) int16 {
	if v > 32767 {
		return 32767
	}
	if v < -32768 {
		return -32768
	}
	return int16(v)
}

// splAbsW16 is WEBRTC_SPL_ABS_W16: result stays in the int domain, so
// abs(-32768) is 32768.
func splAbsW16(a int16) int32 {
	if a >= 0 {
		return int32(a)
	}
	return -int32(a)
}

// splAbsW32 is WEBRTC_SPL_ABS_W32 with gcc wrap: abs(INT32_MIN) = INT32_MIN.
func splAbsW32(a int32) int32 {
	if a >= 0 {
		return a
	}
	return -a
}

// splMax/splMin are WEBRTC_SPL_MAX/MIN.
func splMax(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func splMin(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
