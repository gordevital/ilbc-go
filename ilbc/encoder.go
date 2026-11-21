package ilbc

import (
	"math"

	"github.com/gordevital/ilbc-go/internal/ilbcfix"
)

// Encoder is the idiomatic Go facade over the low-level iLBC encoder.
// It is backed by the fixed-point engine (internal/ilbcfix), which is
// bit-exact against the C reference implementation.
type Encoder struct {
	inst ilbcfix.EncInst
	mode Mode
}

// NewEncoder creates an encoder for the given frame mode.
func NewEncoder(m Mode) (*Encoder, error) {
	if !m.valid() {
		return nil, ErrMode
	}
	enc := ilbcfix.EncInst{}
	if ilbcfix.InitEncoder(&enc, int(m)) < 0 {
		return nil, ErrMode
	}
	return &Encoder{inst: enc, mode: m}, nil
}

// Mode returns the encoder frame mode.
func (e *Encoder) Mode() Mode { return e.mode }

// BlockLen returns the number of PCM samples per input frame.
func (e *Encoder) BlockLen() int { return e.mode.BlockLen() }

// PayloadLen returns the number of encoded bytes per frame.
func (e *Encoder) PayloadLen() int { return e.mode.PayloadLen() }

// Encode encodes one frame of 16-bit PCM (sample values, little-endian
// wire format on output). len(pcm) must equal BlockLen().
//
// The returned slice is a fresh payload of PayloadLen() bytes.
func (e *Encoder) Encode(pcm []int16) ([]byte, error) {
	if len(pcm) != e.inst.BlockL {
		return nil, ErrInvalidFrameSize
	}
	return e.encodeInt16(pcm)
}

// EncodeFloat32 is like Encode but accepts float32 PCM directly. Values
// are expected on the int16 amplitude scale (e.g. the same numbers
// int16 PCM would hold) and are rounded to the nearest sample.
func (e *Encoder) EncodeFloat32(pcm []float32) ([]byte, error) {
	if len(pcm) != e.inst.BlockL {
		return nil, ErrInvalidFrameSize
	}
	conv := make([]int16, len(pcm))
	for i, v := range pcm {
		conv[i] = int16(clampSigned(math.Round(float64(v))))
	}
	return e.encodeInt16(conv)
}

func (e *Encoder) encodeInt16(pcm []int16) ([]byte, error) {
	words := make([]uint16, e.inst.NoOfWords)
	ilbcfix.EncodeImpl(words, pcm, &e.inst)
	payload := make([]byte, e.inst.NoOfBytes)
	for i, w := range words {
		payload[2*i] = byte(w)
		payload[2*i+1] = byte(w >> 8)
	}
	return payload, nil
}

func clampSigned(v float64) float64 {
	const lim = 32767.0
	if v > lim {
		return lim
	}
	if v < -lim {
		return -lim
	}
	return v
}
