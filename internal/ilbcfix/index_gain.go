package ilbcfix

// File: index_gain.go — index_conv_dec.c and gain_dequant.c.

// indexConvDec adjusts codebook indexes 4..5 to the "same-as-first" lag form.
func indexConvDec(index []int16) {
	for k := 4; k < 6; k++ {
		if index[k] >= 44 && index[k] < 108 {
			index[k] += 64
		} else if index[k] >= 108 && index[k] < 128 {
			index[k] += 128
		}
	}
}

// gainDequant decodes one quantized gain (output Q14).
func gainDequant(index int16, maxIn int16, stage int) int16 {
	scale := splAbsW16(maxIn)
	if scale < 1638 { // if lower than 0.1, set it to 0.1
		scale = 1638
	}
	gain := kGain[stage]
	return w16(shr(mul16(int16(scale), gain[int(index)])+8192, 14))
}
