package ilbcfix

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

const goldenFrames = 24
const goldenLossPattern = 3 // good,lost = 3,1

func readGoldenS16(t *testing.T, name string, blockL int, frames int) []int16 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	if want := 2 * blockL * frames; len(data) != want {
		t.Fatalf("%s: got %d bytes, want %d", name, len(data), want)
	}
	out := make([]int16, blockL*frames)
	for i := 0; i < len(out); i++ {
		out[i] = int16(uint16(data[2*i]) | uint16(data[2*i+1])<<8)
	}
	return out
}

func readGoldenPayload(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return data
}

// TestGoldenEncode verifies that the Go encoder emits exactly the C
// reference's bitstream for the fixture signals.
func TestGoldenEncode(t *testing.T) {
	cases := []struct {
		sig  string
		mode int
	}{
		{"sine", 20}, {"sine", 30}, {"noise", 20}, {"noise", 30},
	}
	for _, c := range cases {
		mode := c.mode
		name := c.sig + strconv.Itoa(mode)
		t.Run(name, func(t *testing.T) {
			blockL := mode * 8
			input := readGoldenS16(t, name, blockL, goldenFrames)
			payload := readGoldenPayload(t, name+".payload")
			var enc EncInst
			if nb := initEncode(&enc, mode); nb < 0 {
				t.Fatalf("initEncode(%d) failed", mode)
			}
			if len(payload) != enc.NoOfBytes*goldenFrames {
				t.Fatalf("payload: got %d bytes, want %d", len(payload), enc.NoOfBytes*goldenFrames)
			}
			words := make([]uint16, enc.NoOfWords)
			for f := 0; f < goldenFrames; f++ {
				EncodeImpl(words, input[f*blockL:(f+1)*blockL], &enc)
				got := make([]byte, enc.NoOfBytes)
				for k, w := range words {
					got[2*k] = byte(w)
					got[2*k+1] = byte(w >> 8)
				}
				want := payload[f*enc.NoOfBytes : (f+1)*enc.NoOfBytes]
				if !bytes.Equal(got, want) {
					t.Fatalf("%ds frame %d: payload differs\ngot  %x\nwant %x", mode, f, got, want)
				}
			}
		})
	}
}

// TestGoldenDecodeGood replays the reference payloads through the Go decoder
// and demands sample-exact PCM against the reference decoder output.
func TestGoldenDecodeGood(t *testing.T) {
	cases := []struct {
		sig  string
		mode int
	}{
		{"sine", 20}, {"sine", 30}, {"noise", 20}, {"noise", 30},
	}
	for _, c := range cases {
		mode := c.mode
		name := c.sig + strconv.Itoa(mode)
		t.Run(name, func(t *testing.T) {
			blockL := mode * 8
			payload := readGoldenPayload(t, name+".payload")
			want := readGoldenS16(t, name+".good.pcm", blockL, goldenFrames)

			var dec DecInst
			if r := InitDecoder(&dec, mode); r != 0 {
				t.Fatalf("InitDecoder(%d)=%d", mode, r)
			}
			st := int16(0)
			for f := 0; f < goldenFrames; f++ {
				byteOff := f * dec.NoOfBytes
				got := make([]int16, blockL)
				n := Decode(&dec, payload[byteOff:byteOff+dec.NoOfBytes], got, &st)
				if n != blockL {
					t.Fatalf("%ds frame %d: Decode returned %d", mode, f, n)
				}
				if st != 1 {
					t.Fatalf("%ds frame %d: speechType=%d", mode, f, st)
				}
				if !slices.Equal(got, want[f*blockL:(f+1)*blockL]) {
					t.Fatalf("%ds frame %d: PCM differs\nfirst diff @ %s", mode, f, firstDiff(got, want[f*blockL:(f+1)*blockL]))
				}
			}
		})
	}
}

// TestGoldenDecodePLC replays the 3,1 good/loss pattern (18 payloads out of
// 24 slots) and demands sample-exact PCM, covering doPlc + enhancer loss
// handling + recovery.
func TestGoldenDecodePLC(t *testing.T) {
	cases := []struct {
		sig  string
		mode int
	}{
		{"sine", 20}, {"sine", 30}, {"noise", 20}, {"noise", 30},
	}
	for _, c := range cases {
		mode := c.mode
		name := c.sig + strconv.Itoa(mode)
		t.Run(name, func(t *testing.T) {
			blockL := mode * 8
			payload := readGoldenPayload(t, name+".payload")
			want := readGoldenS16(t, name+".plc.pcm", blockL, goldenFrames)

			var dec DecInst
			if r := InitDecoder(&dec, mode); r != 0 {
				t.Fatalf("InitDecoder(%d)=%d", mode, r)
			}
			st := int16(0)
			pos := 0
			pf := 0
			for slot := 0; slot < goldenFrames; slot++ {
				got := make([]int16, blockL)
				if pos < goldenLossPattern {
					byteOff := pf * dec.NoOfBytes
					n := Decode(&dec, payload[byteOff:byteOff+dec.NoOfBytes], got, &st)
					if n != blockL {
						t.Fatalf("%ds slot %d: Decode returned %d", mode, slot, n)
					}
					pf++
				} else {
					if n := DecodePlc(&dec, got, 1); n != blockL {
						t.Fatalf("%ds slot %d: DecodePlc returned %d", mode, slot, n)
					}
				}
				if !slices.Equal(got, want[slot*blockL:(slot+1)*blockL]) {
					t.Fatalf("%ds slot %d: PCM differs\nfirst diff @ %s", mode, slot, firstDiff(got, want[slot*blockL:(slot+1)*blockL]))
				}
				pos = (pos + 1) % (goldenLossPattern + 1)
			}
		})
	}
}

func firstDiff(a, b []int16) string {
	for i := range a {
		if a[i] != b[i] {
			return fmt.Sprintf("sample %d: %d != %d", i, a[i], b[i])
		}
	}
	return "none (length mismatch?)"
}
