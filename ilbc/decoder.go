package ilbc

import "github.com/gordevital/ilbc-go/internal/ilbcfix"

// Decoder is the idiomatic Go facade over the low-level iLBC decoder.
// It is backed by the fixed-point engine (internal/ilbcfix), which is
// bit-exact against the C reference implementation.
type Decoder struct {
	inst       ilbcfix.DecInst
	mode       Mode
	speechType int16
}

// NewDecoder creates a decoder for the given frame mode.
func NewDecoder(m Mode) (*Decoder, error) {
	if !m.valid() {
		return nil, ErrMode
	}
	dec := ilbcfix.DecInst{}
	if ilbcfix.InitDecoder(&dec, int(m)) != 0 {
		return nil, ErrMode
	}
	return &Decoder{inst: dec, mode: m}, nil
}

// Mode returns the decoder frame mode.
func (d *Decoder) Mode() Mode { return d.mode }

// BlockLen returns the number of output PCM samples per frame.
func (d *Decoder) BlockLen() int { return d.mode.BlockLen() }

// Decode decodes a payload of 1 to 3 consecutive frames into int16 PCM.
// As in the C reference, a payload whose byte length belongs to the other
// frame length switches the decoder to that mode automatically.
func (d *Decoder) Decode(payload []byte) ([]int16, error) {
	bl := d.inst.BlockL
	decoded := make([]int16, 3*bl)
	n := ilbcfix.Decode(&d.inst, payload, decoded, &d.speechType)
	if n < 0 {
		return nil, ErrInvalidPayload
	}
	if Mode(d.inst.Mode) != d.mode {
		d.mode = Mode(d.inst.Mode)
	}
	return decoded[:n], nil
}

// Conceal produces one concealment frame for a lost payload.
func (d *Decoder) Conceal() ([]int16, error) {
	bl := d.inst.BlockL
	out := make([]int16, bl)
	ilbcfix.DecodePlc(&d.inst, out, 1)
	return out, nil
}
