package ilbcfix

// hpOutput mirrors WebRtcIlbcfix_HpOutput (hp_output.c): high-pass filter of
// output and *2 with saturation. ba = {b[0] b[1] b[2] -a[1] -a[2]}.
func hpOutput(signal []int16, ba []int16, y []int16, x []int16, length int) {
	for i := 0; i < length; i++ {
		var tmpW32 int32

		tmpW32 = mul16(y[1], ba[3])  /* (-a[1])*y[i-1] (low part) */
		tmpW32 += mul16(y[3], ba[4]) /* (-a[2])*y[i-2] (low part) */
		tmpW32 = tmpW32 >> 15
		tmpW32 += mul16(y[0], ba[3]) /* (-a[1])*y[i-1] (high part) */
		tmpW32 += mul16(y[2], ba[4]) /* (-a[2])*y[i-2] (high part) */
		tmpW32 *= 2

		tmpW32 += mul16(signal[i], ba[0]) /* b[0]*x[0] */
		tmpW32 += mul16(x[0], ba[1])      /* b[1]*x[i-1] */
		tmpW32 += mul16(x[1], ba[2])      /* b[2]*x[i-2] */

		/* Update state (input part) */
		x[1] = x[0]
		x[0] = signal[i]

		/* Rounding in Q(12-1), i.e. add 2^10 */
		tmpW32b := tmpW32 + 1024

		/* Saturate (to 2^26) so that the HP filtered signal does not overflow */
		tmpW32b = splSat(67108863, tmpW32b, -67108864)

		/* Convert back to Q0 and multiply with 2 */
		signal[i] = int16(tmpW32b >> 11)

		/* Update state (filtered part) */
		y[2] = y[0]
		y[3] = y[1]

		/* upshift tmpW32 by 3 with saturation */
		if tmpW32 > 268435455 {
			tmpW32 = word32Max
		} else if tmpW32 < -268435456 {
			tmpW32 = word32Min
		} else {
			tmpW32 *= 8
		}

		y[0] = int16(tmpW32 >> 16)
		y[1] = int16((tmpW32 & 0xffff) >> 1)
	}
}
