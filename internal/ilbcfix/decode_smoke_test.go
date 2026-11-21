package ilbcfix

import (
	"math"
	"testing"
)

func smokeAbs(s int16) int16 {
	if s < 0 {
		return -s
	}
	return s
}

// sineBlock renders blockL samples of a 300 Hz sine wave at 8 kHz amplitude
// ~10000.
func sineBlock(blockL int) []int16 {
	b := make([]int16, blockL)
	for i := range b {
		b[i] = int16(math.Sin(2*math.Pi*300*float64(i)/8000) * 10000)
	}
	return b
}

// encodeFrame encodes one frame with the Go encoder and returns the wire
// byte payload (wire words serialized as little-endian byte pairs, exactly
// as C WebRtcIlbcfix_Encode stores them).
func encodeFrame(t *testing.T, mode int, block []int16) []byte {
	t.Helper()
	var enc EncInst
	if nb := initEncode(&enc, mode); nb < 0 {
		t.Fatalf("initEncode(%d) failed", mode)
	}
	words := make([]uint16, enc.NoOfWords)
	EncodeImpl(words, block, &enc)
	payload := make([]byte, enc.NoOfBytes)
	for k, w := range words {
		payload[2*k] = byte(w)
		payload[2*k+1] = byte(w >> 8)
	}
	return payload
}

func TestDecodeSmokeDeterminism(t *testing.T) {
	for _, mode := range []int{20, 30} {
		var d1, d2 DecInst
		if r := InitDecoder(&d1, mode); r != 0 {
			t.Fatalf("InitDecoder(%d)=%d", mode, r)
		}
		InitDecoder(&d2, mode)

		var block []int16
		if mode == 20 {
			block = sineBlock(BlockL20ms)
		} else {
			block = sineBlock(BlockL30ms)
		}

		payload := encodeFrame(t, mode, block)
		blockL := d1.BlockL
		got1 := make([]int16, blockL)
		got2 := make([]int16, blockL)

		var st1, st2 int16
		if n := Decode(&d1, payload, got1, &st1); n != blockL {
			t.Fatalf("%d ms: Decode returned %d, want %d", mode, n, blockL)
		}
		if st1 != 1 {
			t.Fatalf("%d ms: speechType=%d, want 1", mode, st1)
		}
		if n := Decode(&d2, payload, got2, &st2); n != blockL {
			t.Fatalf("%d ms: second decode returned %d, want %d", mode, n, blockL)
		}
		for i := range got1 {
			if got1[i] != got2[i] {
				t.Fatalf("%d ms: nondeterministic sample %d: %d != %d", mode, i, got1[i], got2[i])
			}
		}
		// Sanity: output should not be silent.
		var peak int16
		for _, s := range got1 {
			if a := smokeAbs(s); a > peak {
				peak = a
			}
		}
		if peak < 100 {
			t.Fatalf("%d ms: decoded frame nearly silent (peak %d)", mode, peak)
		}
	}
}

func TestDecodePlcSmoke(t *testing.T) {
	for _, mode := range []int{20, 30} {
		var dec DecInst
		InitDecoder(&dec, mode)
		blockL := dec.BlockL

		var block []int16
		if mode == 20 {
			block = sineBlock(BlockL20ms)
		} else {
			block = sineBlock(BlockL30ms)
		}
		payload := encodeFrame(t, mode, block)

		var st int16
		good := make([]int16, blockL)
		if n := Decode(&dec, payload, good, &st); n != blockL {
			t.Fatalf("%d ms: Decode returned %d", mode, n)
		}

		lost := make([]int16, 2*blockL)
		if n := DecodePlc(&dec, lost, 2); n != 2*blockL {
			t.Fatalf("%d ms: DecodePlc returned %d", mode, n)
		}

		// Recovery with a real frame after the loss.
		var st2 int16
		recovered := make([]int16, blockL)
		if n := Decode(&dec, payload, recovered, &st2); n != blockL {
			t.Fatalf("%d ms: post-loss Decode returned %d", mode, n)
		}
	}
}

func TestDecodeCorruptedPayload(t *testing.T) {
	for _, mode := range []int{20, 30} {
		var dec DecInst
		InitDecoder(&dec, mode)
		blockL := dec.BlockL

		var block []int16
		if mode == 20 {
			block = sineBlock(BlockL20ms)
		} else {
			block = sineBlock(BlockL30ms)
		}
		payload := append([]byte(nil), encodeFrame(t, mode, block)...)
		// Destroy the middle of the payload.
		for i := len(payload) / 4; i < 3*len(payload)/4; i++ {
			payload[i] ^= 0xFF
		}

		var st int16
		got := make([]int16, blockL)
		// Must not panic; outcome is PLC fallback, a hard error, or reset.
		n := Decode(&dec, payload, got, &st)
		if n != 0 && n != blockL && n != -1 {
			t.Fatalf("%d ms: corrupted payload gave n=%d", mode, n)
		}
	}
}
