package ilbcfix

// swapBytes mirrors WebRtcIlbcfix_SwapBytes (swap_bytes.c).
func swapBytes(input []uint16, wordLength int, output []uint16) {
	for k := 0; k < wordLength; k++ {
		output[k] = uint16(input[k]>>8) | uint16(input[k]<<8)
	}
}
