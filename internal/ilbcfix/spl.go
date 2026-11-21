package ilbcfix

import (
	"math/bits"
)

// SPL (common_audio/signal_processing) primitives, transcribed verbatim from
// the C reference as compiled with gcc/x86-64 (generic C paths, no
// NEON/MIPS). Names keep the C name minus the WebRtcSpl_ prefix.

// getSizeInBits: 32 - CLZ32(n), for n > 0.
func getSizeInBits(n uint32) int32 {
	return int32(32 - bits.LeadingZeros32(n))
}

// normW32: left-shift steps until overflow; 0 if a == 0.
func normW32(a int32) int32 {
	if a == 0 {
		return 0
	}
	u := uint32(a)
	if a < 0 {
		u = ^u
	}
	return int32(bits.LeadingZeros32(u)) - 1
}

// countLeadingZeros64 mirrors WebRtcSpl_CountLeadingZeros64.
func countLeadingZeros64(n uint64) int32 {
	return int32(bits.LeadingZeros64(n))
}

// memSetW16 fills p[at:at+n] (WebRtcSpl_MemSetW16).
func memSetW16(p []int16, at, n int, v int16) {
	for i := 0; i < n; i++ {
		p[at+i] = v
	}
}

// memSetW32 fills p[at:at+n] (WebRtcSpl_MemSetW32).
func memSetW32(p []int32, at, n int, v int32) {
	for i := 0; i < n; i++ {
		p[at+i] = v
	}
}

// memCpyReversedOrder writes source upwards into dest starting at index at
// and decrementing (the C pointer starts at dest and is decremented).
func memCpyReversedOrder(dest []int16, at int, source []int16, n int) {
	for i := 0; i < n; i++ {
		dest[at-i] = source[i]
	}
}

// maxAbsValueW16 (WebRtcSpl_MaxAbsValueW16C).
func maxAbsValueW16(v []int16, n int) int16 {
	maximum := int32(0)
	for i := 0; i < n; i++ {
		a := splAbsW16(v[i])
		if a > maximum {
			maximum = a
		}
	}
	if maximum > 32767 {
		maximum = 32767
	}
	return int16(maximum)
}

// maxAbsValueW32 (WebRtcSpl_MaxAbsValueW32C); abs(0x80000000) wraps.
func maxAbsValueW32(v []int32, n int) int32 {
	var maximum uint32
	for i := 0; i < n; i++ {
		a := splAbsW32(v[i])
		if uint32(a) > maximum {
			maximum = uint32(a)
		}
	}
	return splMin(int32(maximum), word32Max)
}

// maxAbsElementW16 (WebRtcSpl_MaxAbsElementW16).
func maxAbsElementW16(v []int16, n int) int16 {
	minVal, maxVal := minMaxW16(v, n)
	if minVal == maxVal || minVal < -maxVal {
		return minVal
	}
	return maxVal
}

// minMaxW16 (WebRtcSpl_MinMaxW16).
func minMaxW16(v []int16, n int) (int16, int16) {
	minimum := int16(32767)
	maximum := int16(-32768)
	for i := 0; i < n; i++ {
		if v[i] < minimum {
			minimum = v[i]
		}
		if v[i] > maximum {
			maximum = v[i]
		}
	}
	return minimum, maximum
}

// maxValueW32 (WebRtcSpl_MaxValueW32C).
func maxValueW32(v []int32, n int) int32 {
	maximum := word32Min
	for i := 0; i < n; i++ {
		if v[i] > maximum {
			maximum = v[i]
		}
	}
	return maximum
}

// maxIndexW32 (WebRtcSpl_MaxIndexW32).
func maxIndexW32(v []int32, n int) int {
	index := 0
	maximum := word32Min
	for i := 0; i < n; i++ {
		if v[i] > maximum {
			maximum = v[i]
			index = i
		}
	}
	return index
}

// divW32W16 (WebRtcSpl_DivW32W16): truncated quotient, 0x7FFFFFFF on 0.
func divW32W16(num int32, den int16) int32 {
	if den != 0 {
		return num / int32(den)
	}
	return 0x7FFFFFFF
}

// divW32W16ResW16 (WebRtcSpl_DivW32W16ResW16).
func divW32W16ResW16(num int32, den int16) int16 {
	if den != 0 {
		return int16(num / int32(den))
	}
	return 0x7FFF
}

// divW32HiLow (WebRtcSpl_DivW32HiLow): Newton-refined division.
func divW32HiLow(num int32, denHi, denLow int16) int32 {
	approx := int16(divW32W16(0x1FFFFFFF, denHi))
	// result in Q14 (Note: 3FFFFFFF = 0.5 in Q30)

	// tmpW32 = 1/den = approx * (2.0 - den * approx) (in Q30)
	tmpW32 := (int32(denHi) * int32(approx) << 1) + ((int32(denLow) * int32(approx) >> 15) << 1)
	// tmpW32 = den * approx

	tmpW32 = 0x7fffffff - tmpW32 // result in Q30 (tmpW32 = 2.0-(den*approx))

	// Store tmpW32 in hi and low format
	tmpHi := int16(tmpW32 >> 16)
	tmpLow := int16((tmpW32 - (int32(tmpHi) << 16)) >> 1)
	tmpW32 = (int32(tmpHi)*int32(approx) + (int32(tmpLow) * int32(approx) >> 15)) << 1
	tmpHi = int16(tmpW32 >> 16)
	tmpLow = int16((tmpW32 - (int32(tmpHi) << 16)) >> 1)

	numHi := int16(num >> 16)
	numLow := int16((num - (int32(numHi) << 16)) >> 1)

	tmpW32 = int32(numHi)*int32(tmpHi) + (int32(numHi) * int32(tmpLow) >> 15) +
		(int32(numLow) * int32(tmpHi) >> 15)
	return shl(tmpW32, 3)
}

// dotProductWithScale (WebRtcSpl_DotProductWithScale): 64-bit accumulator
// with per-term scaling, saturated to int32 at the end.
func dotProductWithScale(v1, v2 []int16, length, scaling int) int32 {
	var sum int64
	i := 0
	for ; i+3 < length; i += 4 {
		sum += int64((int32(v1[i+0]) * int32(v2[i+0])) >> uint(scaling))
		sum += int64((int32(v1[i+1]) * int32(v2[i+1])) >> uint(scaling))
		sum += int64((int32(v1[i+2]) * int32(v2[i+2])) >> uint(scaling))
		sum += int64((int32(v1[i+3]) * int32(v2[i+3])) >> uint(scaling))
	}
	for ; i < length; i++ {
		sum += int64((int32(v1[i]) * int32(v2[i])) >> uint(scaling))
	}
	// rtc::saturated_cast<int32_t>
	if sum > int64(word32Max) {
		return word32Max
	}
	if sum < int64(word32Min) {
		return word32Min
	}
	return int32(sum)
}

// filterMAFastQ12 (WebRtcSpl_FilterMAFastQ12). The C in pointer reads back
// to inPtr[-B_length+1]; in Go the caller passes the full buffer and inOff
// such that index inOff+i-j is the C element inPtr[i-j]. Output is written
// at out[outOff+i].
func filterMAFastQ12(in []int16, inOff int, out []int16, outOff int, b []int16, bLen, length int) {
	for i := 0; i < length; i++ {
		var o int32
		for j := 0; j < bLen; j++ {
			o += int32(b[j]) * int32(in[inOff+i-j])
		}
		o = splSat(134215679, o, -134217728)
		out[outOff+i] = int16((o + 2048) >> 12)
	}
}

// filterARFastQ12 (WebRtcSpl_FilterARFastQ12). The C data_out pointer holds
// state samples at negative offsets; out[outOff+i-j] accesses that memory.
func filterARFastQ12(dataIn []int16, dataOut []int16, outOff int, coefficients []int16, coeffLen, dataLen int) {
	for i := 0; i < dataLen; i++ {
		var sum int64
		for j := coeffLen - 1; j > 0; j-- {
			sum += int64(coefficients[j]) * int64(dataOut[outOff+i-j])
		}
		output := int64(coefficients[0]) * int64(dataIn[i])
		output -= sum
		if output > 134215679 {
			output = 134215679
		} else if output < -134217728 {
			output = -134217728
		}
		dataOut[outOff+i] = int16((output + 2048) >> 12)
	}
}

// crossCorrelation (WebRtcSpl_CrossCorrelationC). seq2 steps by stepSeq2
// (may be negative) per output entry; off2 is the start index in seq2.
func crossCorrelation(cc []int32, seq1 []int16, seq2 []int16, off2 int, dimSeq, dimCC int, rightShifts, stepSeq2 int) {
	for i := 0; i < dimCC; i++ {
		var corr int32
		base := off2 + i*stepSeq2
		for j := 0; j < dimSeq; j++ {
			corr += (int32(seq1[j]) * int32(seq2[base+j])) >> uint(rightShifts)
		}
		cc[i] = corr
	}
}

// autoCorrelation (WebRtcSpl_AutoCorrelation) with the scaling output.
func autoCorrelation(in []int16, inLen, order int) (result []int32, scale int) {
	smax := maxAbsValueW16(in, inLen)
	scaling := 0
	if smax != 0 {
		nbits := int(getSizeInBits(uint32(inLen)))
		t := int(normW32(mulW32(int32(smax), int32(smax))))
		if t > nbits {
			scaling = 0
		} else {
			scaling = nbits - t
		}
	}
	result = make([]int32, order+1)
	for i := 0; i <= order; i++ {
		var sum int32
		j := 0
		for ; i+j+3 < inLen; j += 4 {
			sum += (int32(in[j+0]) * int32(in[i+j+0])) >> uint(scaling)
			sum += (int32(in[j+1]) * int32(in[i+j+1])) >> uint(scaling)
			sum += (int32(in[j+2]) * int32(in[i+j+2])) >> uint(scaling)
			sum += (int32(in[j+3]) * int32(in[i+j+3])) >> uint(scaling)
		}
		for ; j < inLen-i; j++ {
			sum += (int32(in[j]) * int32(in[i+j])) >> uint(scaling)
		}
		result[i] = sum
	}
	return result, scaling
}

// getScalingSquare (WebRtcSpl_GetScalingSquare).
func getScalingSquare(in []int16, inLen, times int) int16 {
	nbits := int16(getSizeInBits(uint32(times)))
	smax := int16(-1)
	for i := 0; i < inLen; i++ {
		sabs := in[i]
		if sabs < 0 {
			sabs = -sabs
		}
		if sabs > smax {
			smax = sabs
		}
	}
	t := normW32(mulW32(int32(smax), int32(smax)))
	if smax == 0 {
		return 0
	}
	if int(t) > int(nbits) {
		return 0
	}
	return int16(int(nbits) - int(t))
}

// energyOf (WebRtcSpl_Energy).
func energyOf(v []int16, vLen int) (en int32, scaleFactor int) {
	scaling := int(getScalingSquare(v, vLen, vLen))
	for i := 0; i < vLen; i++ {
		en += (int32(v[i]) * int32(v[i])) >> uint(scaling)
	}
	return en, scaling
}

// sqrtLocal (WebRtcSpl_SqrtLocal).
func sqrtLocal(in int32) int32 {
	var xHalf, t16 int16
	var A, B, x2 int32

	B = in / 2

	B -= 0x40000000 // B = in/2 - 1/2
	xHalf = int16(B >> 16)
	B += 0x40000000 // B = 1 + x/2
	B += 0x40000000 // add 0.5 twice (since 1.0 does not exist in Q31)

	x2 = int32(xHalf) * int32(xHalf) * 2 // (x/2)^2
	A = -x2
	B += A >> 1

	A >>= 16
	A = A * A * 2 // (x/2)^4
	t16 = int16(A >> 16)
	B += int32(-20480) * int32(t16) * 2 // B - 0.625*A

	A = int32(xHalf) * int32(t16) * 2 // (x/2)^5
	t16 = int16(A >> 16)
	B += int32(28672) * int32(t16) * 2 // B + 0.875*A

	t16 = int16(x2 >> 16)
	A = int32(xHalf) * int32(t16) * 2 // x/2^3

	B += A >> 1

	B += 32768 // round off bit

	return B
}

// sqrtFloor (WebRtcSpl_SqrtFloor) — exact integer floor square root,
// digit-by-digit extraction (mirrors spl_sqrt_floor.c).
func sqrtFloor(value int32) int32 {
	var root, try1 int32
	for N := 15; N >= 0; N-- {
		try1 = root + (1 << N)
		if value >= try1<<N {
			value -= try1 << N
			root |= 2 << N
		}
	}
	return root >> 1
}

// downsampleFast (WebRtcSpl_DownsampleFastC). dataIn is the full input
// buffer; inOff is the index of the logical start (the C data_in pointer);
// the filter reads dataIn[inOff+i-j], i.e. it may reach into the history
// before inOff. Returns 0 on success, -1 on argument error.
func downsampleFast(dataIn []int16, inOff, dataInLen int, dataOut []int16, dataOutLen int, coefficients []int16, coeffLen, factor, delay int) int {
	out := 0
	endpos := delay + factor*(dataOutLen-1) + 1
	if dataOutLen == 0 || coeffLen == 0 || dataInLen < endpos {
		return -1
	}
	for i := delay; i < endpos; i += factor {
		outS32 := int32(2048)
		for j := 0; j < coeffLen; j++ {
			outS32 += int32(coefficients[j]) * int32(dataIn[inOff+i-j])
		}
		outS32 >>= 12
		dataOut[out] = splSatW32ToW16(outS32)
		out++
	}
	return 0
}

// reverseOrderMultArrayElements (WebRtcSpl_ReverseOrderMultArrayElements):
// out[i] = (in[i] * win[winOff-i]) >> rightShifts.
func reverseOrderMultArrayElements(out []int16, in []int16, win []int16, winOff, vectorLen int, rightShifts int16) {
	for i := 0; i < vectorLen; i++ {
		out[i] = int16((int32(in[i]) * int32(win[winOff-i])) >> uint(int(rightShifts)))
	}
}

// elementwiseVectorMult (WebRtcSpl_ElementwiseVectorMult):
// out[i] = (in[i] * win[winOff+i]) >> rightShifts.
func elementwiseVectorMult(out []int16, in []int16, win []int16, winOff, vectorLen int, rightShifts int16) {
	for i := 0; i < vectorLen; i++ {
		out[i] = int16((int32(in[i]) * int32(win[winOff+i])) >> uint(int(rightShifts)))
	}
}

// addVectorsAndShift (WebRtcSpl_AddVectorsAndShift).
func addVectorsAndShift(out []int16, in1, in2 []int16, vectorLen int, rightShifts int16) {
	for i := 0; i < vectorLen; i++ {
		out[i] = int16((int32(in1[i]) + int32(in2[i])) >> uint(int(rightShifts)))
	}
}

// addAffineVectorToVector (WebRtcSpl_AddAffineVectorToVector):
// out[i] += (in[i]*gain + addConstant) >> rightShifts.
func addAffineVectorToVector(out []int16, in []int16, gain int16, addConstant int32, rightShifts int16, vectorLen int) {
	for i := 0; i < vectorLen; i++ {
		v := int16((int32(in[i])*int32(gain) + addConstant) >> uint(int(rightShifts)))
		out[i] = int16(int32(out[i]) + int32(v))
	}
}

// affineTransformVector (WebRtcSpl_AffineTransformVector).
func affineTransformVector(out []int16, in []int16, gain int16, addConstant int32, rightShifts int16, vectorLen int) {
	for i := 0; i < vectorLen; i++ {
		out[i] = int16((int32(in[i])*int32(gain) + addConstant) >> uint(int(rightShifts)))
	}
}

// scaleVector (WebRtcSpl_ScaleVector).
func scaleVector(in, out []int16, gain int16, inVectorLen int, rightShifts int16) {
	for i := 0; i < inVectorLen; i++ {
		out[i] = int16((int32(in[i]) * int32(gain)) >> uint(int(rightShifts)))
	}
}

// scaleVectorWithSat (WebRtcSpl_ScaleVectorWithSat).
func scaleVectorWithSat(in, out []int16, gain int16, inVectorLen int, rightShifts int16) {
	for i := 0; i < inVectorLen; i++ {
		out[i] = splSatW32ToW16((int32(in[i]) * int32(gain)) >> uint(int(rightShifts)))
	}
}

// scaleAndAddVectors (WebRtcSpl_ScaleAndAddVectors).
func scaleAndAddVectors(in1 []int16, gain1 int16, shift1 int16, in2 []int16, gain2 int16, shift2 int16, out []int16, vectorLen int) {
	for i := 0; i < vectorLen; i++ {
		a := int16((int32(gain1) * int32(in1[i])) >> uint(int(shift1)))
		b := int16((int32(gain2) * int32(in2[i])) >> uint(int(shift2)))
		out[i] = int16(int32(a) + int32(b))
	}
}

// vectorBitShiftW32 (WebRtcSpl_VectorBitShiftW32).
func vectorBitShiftW32(out, in []int32, rightShifts int16) {
	if rightShifts > 0 {
		for i := range out {
			out[i] = in[i] >> uint(int(rightShifts))
		}
	} else {
		s := uint(-int(rightShifts))
		for i := range out {
			out[i] = shl(in[i], int32(s))
		}
	}
}

// levinsonDurbin (WebRtcSpl_LevinsonDurbin): returns 1 if stable, 0 if the
// computed reflection coefficient exceeds the stability bound.
func levinsonDurbin(R []int32, A []int16, K []int16, order int) int16 {
	var RHi, RLow [21]int16
	var AHi, ALow [21]int16
	var AupdHi, AupdLow [21]int16
	var KHi, KLow int16
	var AlphaHi, AlphaLow, AlphaExp int16
	var tmpHi, tmpLow int16
	var temp1W32, temp2W32, temp3W32 int32
	var norm int32

	// Normalize the autocorrelation R[0]...R[order]
	norm = normW32(R[0])

	for i := 0; i <= order; i++ {
		temp1W32 = shl(R[i], norm)
		// Put R in hi and low format
		RHi[i] = int16(temp1W32 >> 16)
		RLow[i] = int16((temp1W32 - (int32(RHi[i]) * 65536)) >> 1)
	}

	// K = A[1] = -R[1] / R[0]
	temp2W32 = shl(R[1], norm) // R[1] in Q31
	temp3W32 = splAbsW32(temp2W32)
	temp1W32 = divW32HiLow(temp3W32, RHi[0], RLow[0]) // abs(R[1])/R[0] in Q31
	// Put back the sign on R[1]
	if temp2W32 > 0 {
		temp1W32 = -temp1W32
	}

	// Put K in hi and low format
	KHi = int16(temp1W32 >> 16)
	KLow = int16((temp1W32 - (int32(KHi) * 65536)) >> 1)

	// Store first reflection coefficient
	K[0] = KHi

	temp1W32 >>= 4 // A[1] in Q27

	// Put A[1] in hi and low format
	AHi[1] = int16(temp1W32 >> 16)
	ALow[1] = int16((temp1W32 - (int32(AHi[1]) * 65536)) >> 1)

	// Alpha = R[0] * (1-K^2)
	temp1W32 = ((int32(KHi) * int32(KLow) >> 14) + int32(KHi)*int32(KHi)) * 2 // = k^2 in Q31
	temp1W32 = splAbsW32(temp1W32)
	temp1W32 = 0x7fffffff - temp1W32 // temp1W32 = (1 - K[0]*K[0]) in Q31

	// Store temp1W32 = 1 - K[0]*K[0] on hi and low format
	tmpHi = int16(temp1W32 >> 16)
	tmpLow = int16((temp1W32 - (int32(tmpHi) << 16)) >> 1)

	// Calculate Alpha in Q31
	temp1W32 = (int32(RHi[0])*int32(tmpHi) + (int32(RHi[0]) * int32(tmpLow) >> 15) +
		(int32(RLow[0]) * int32(tmpHi) >> 15)) << 1

	// Normalize Alpha and put it in hi and low format
	AlphaExp = int16(normW32(temp1W32))
	temp1W32 = shl(temp1W32, int32(AlphaExp))
	AlphaHi = int16(temp1W32 >> 16)
	AlphaLow = int16((temp1W32 - (int32(AlphaHi) << 16)) >> 1)

	// Perform the iterative calculations in the Levinson-Durbin algorithm
	for i := 2; i <= order; i++ {
		// temp1W32 = R[i] + sum R[j]*A[i-j]
		temp1W32 = 0
		for j := 1; j < i; j++ {
			// temp1W32 is in Q31
			temp1W32 += (int32(RHi[j]) * int32(AHi[i-j]) * 2) +
				(((int32(RHi[j]) * int32(ALow[i-j]) >> 15) +
					(int32(RLow[j]) * int32(AHi[i-j]) >> 15)) * 2)
		}

		temp1W32 = temp1W32 * 16
		temp1W32 += int32(RHi[i])*65536 + shl(int32(RLow[i]), 1)

		// K = -temp1W32 / Alpha
		temp2W32 = splAbsW32(temp1W32)                      // abs(temp1W32)
		temp3W32 = divW32HiLow(temp2W32, AlphaHi, AlphaLow) // abs(temp1W32)/Alpha
		if temp1W32 > 0 {
			temp3W32 = -temp3W32
		}

		// Use the Alpha shifts from earlier to de-normalize
		norm = normW32(temp3W32)
		if int32(AlphaExp) <= norm || temp3W32 == 0 {
			temp3W32 = temp3W32 * shl(1, int32(AlphaExp))
		} else {
			if temp3W32 > 0 {
				temp3W32 = 0x7fffffff
			} else {
				temp3W32 = word32Min
			}
		}

		// Put K on hi and low format
		KHi = int16(temp3W32 >> 16)
		KLow = int16((temp3W32 - (int32(KHi) * 65536)) >> 1)

		// Store reflection coefficient in Q15
		K[i-1] = KHi

		// Test for unstable filter
		if splAbsW16(KHi) > 32750 {
			return 0 // unstable filter
		}

		// Compute updated LPC coefficients: Anew[j] = A[j] + K*A[i-j]
		for j := 1; j < i; j++ {
			// temp1W32 = A[j] in Q27
			temp1W32 = int32(AHi[j])*65536 + shl(int32(ALow[j]), 1)
			// temp1W32 += K*A[i-j] in Q27
			temp1W32 += (int32(KHi)*int32(AHi[i-j]) + (int32(KHi) * int32(ALow[i-j]) >> 15) +
				(int32(KLow) * int32(AHi[i-j]) >> 15)) * 2
			// Put Anew in hi and low format
			AupdHi[j] = int16(temp1W32 >> 16)
			AupdLow[j] = int16((temp1W32 - (int32(AupdHi[j]) * 65536)) >> 1)
		}

		// temp3W32 = K in Q27 (convert from Q31 to Q27)
		temp3W32 >>= 4

		// Store Anew in hi and low format
		AupdHi[i] = int16(temp3W32 >> 16)
		AupdLow[i] = int16((temp3W32 - (int32(AupdHi[i]) * 65536)) >> 1)

		// Alpha = Alpha * (1-K^2)
		temp1W32 = ((int32(KHi) * int32(KLow) >> 14) + int32(KHi)*int32(KHi)) * 2 // K*K in Q31
		temp1W32 = splAbsW32(temp1W32)
		temp1W32 = 0x7fffffff - temp1W32 // 1 - K*K in Q31

		// Convert 1 - K^2 to hi and low format
		tmpHi = int16(temp1W32 >> 16)
		tmpLow = int16((temp1W32 - (int32(tmpHi) << 16)) >> 1)

		// Calculate Alpha = Alpha * (1-K^2) in Q31
		temp1W32 = (int32(AlphaHi)*int32(tmpHi) + (int32(AlphaHi) * int32(tmpLow) >> 15) +
			(int32(AlphaLow) * int32(tmpHi) >> 15)) << 1

		// Normalize Alpha and store it on hi and low format
		norm = normW32(temp1W32)
		temp1W32 = shl(temp1W32, norm)
		AlphaHi = int16(temp1W32 >> 16)
		AlphaLow = int16((temp1W32 - (int32(AlphaHi) << 16)) >> 1)

		// Update the total normalization of Alpha
		AlphaExp = int16(int32(AlphaExp) + norm)

		// Update A[]
		for j := 1; j <= i; j++ {
			AHi[j] = AupdHi[j]
			ALow[j] = AupdLow[j]
		}
	}

	// Set A[0] to 1.0 and store the A[i] in Q12 (convert from Q27, rounding)
	A[0] = 4096
	for i := 1; i <= order; i++ {
		// temp1W32 in Q27
		temp1W32 = int32(AHi[i])*65536 + shl(int32(ALow[i]), 1)
		// Round and store upper word
		A[i] = int16((temp1W32*2 + 32768) >> 16)
	}
	return 1 // stable filters
}
