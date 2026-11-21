package ilbc

import (
	"math"
	"testing"
)

func sinePCM(n, phase int, freq, amp float64) []int16 {
	p := make([]int16, n)
	for i := range p {
		tt := float64(phase + i)
		p[i] = int16(amp * math.Sin(2*math.Pi*freq*tt/8000))
	}
	return p
}

func TestFacadeRoundTripBothModes(t *testing.T) {
	for _, m := range []Mode{Mode20ms, Mode30ms} {
		enc, err := NewEncoder(m)
		if err != nil {
			t.Fatalf("NewEncoder(%d): %v", m, err)
		}
		dec, err := NewDecoder(m)
		if err != nil {
			t.Fatalf("NewDecoder(%d): %v", m, err)
		}
		var ref, rec []int16
		phase := 0
		for f := 0; f < 30; f++ {
			in := sinePCM(enc.BlockLen(), phase, 440, 5000)
			phase += enc.BlockLen()
			ref = append(ref, in...)
			payload, err := enc.Encode(in)
			if err != nil {
				t.Fatalf("mode %d frame %d encode: %v", m, f, err)
			}
			if len(payload) != m.PayloadLen() {
				t.Fatalf("mode %d payload len = %d, want %d", m, len(payload), m.PayloadLen())
			}
			out, err := dec.Decode(payload)
			if err != nil {
				t.Fatalf("mode %d frame %d decode: %v", m, f, err)
			}
			if len(out) != m.BlockLen() {
				t.Fatalf("mode %d out len = %d, want %d", m, len(out), m.BlockLen())
			}
			rec = append(rec, out...)
		}
		// The codec has a fixed algorithmic delay (~10-25 ms), so a
		// zero-lag comparison against a narrow-band tone is meaningless
		// (it lands near anti-phase). Score with a lag-compensated
		// normalized cross-correlation instead.
		best := 0.0
		for lag := 0; lag <= 2*m.BlockLen(); lag += 4 {
			n := len(ref) - lag
			if rn := len(rec); n > rn {
				n = rn
			}
			if n < m.BlockLen() {
				break
			}
			var mr, mc, vr, vc, cov float64
			for i := 0; i < n; i++ {
				a := float64(ref[i+lag])
				b := float64(rec[i])
				mr += a
				mc += b
			}
			mr /= float64(n)
			mc /= float64(n)
			for i := 0; i < n; i++ {
				a := float64(ref[i+lag]) - mr
				b := float64(rec[i]) - mc
				cov += a * b
				vr += a * a
				vc += b * b
			}
			c := cov / math.Sqrt(vr*vc+1e-9)
			if c > best {
				best = c
			}
		}
		t.Logf("mode %d: best lag-compensated corr = %.3f", m, best)
		if best < 0.9 {
			t.Errorf("mode %d: correlation %.3f < 0.9", m, best)
		}
	}
}

func TestFacadeEncodeWrongSize(t *testing.T) {
	enc, _ := NewEncoder(Mode30ms)
	if _, err := enc.Encode(make([]int16, 100)); err != ErrInvalidFrameSize {
		t.Fatalf("err = %v, want ErrInvalidFrameSize", err)
	}
}

func TestFacadeDecodeWrongSize(t *testing.T) {
	dec, _ := NewDecoder(Mode20ms)
	// As in the C reference, a 50-byte (30 ms) payload handed to a 20 ms
	// decoder switches the mode instead of failing.
	out, err := dec.Decode(make([]byte, 50))
	if err != nil {
		t.Fatalf("auto mode switch should succeed: %v", err)
	}
	if len(out) != Mode30ms.BlockLen() {
		t.Fatalf("switched payload len = %d, want %d", len(out), Mode30ms.BlockLen())
	}
	if dec.Mode() != Mode30ms {
		t.Fatalf("mode after switch = %d, want %d", dec.Mode(), Mode30ms)
	}
	// A length that belongs to neither mode is still an error.
	if _, err := dec.Decode(make([]byte, 40)); err != ErrInvalidPayload {
		t.Fatalf("40-byte payload: err = %v, want ErrInvalidPayload", err)
	}
}

func TestFacadeConceal(t *testing.T) {
	dec, _ := NewDecoder(Mode30ms)
	enc, _ := NewEncoder(Mode30ms)
	payload, _ := enc.Encode(sinePCM(240, 0, 300, 3000))
	if _, err := dec.Decode(payload); err != nil {
		t.Fatal(err)
	}
	out, err := dec.Conceal()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 240 {
		t.Fatalf("len = %d, want 240", len(out))
	}
}

func TestFacadeFloat32Encode(t *testing.T) {
	enc, _ := NewEncoder(Mode20ms)
	pcm := make([]float32, 160)
	for i := range pcm {
		pcm[i] = float32(1000 * math.Sin(2*math.Pi*440*float64(i)/8000))
	}
	out, err := enc.EncodeFloat32(pcm)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 38 {
		t.Fatalf("payload len = %d, want 38", len(out))
	}
}

func TestFacadeClamping(t *testing.T) {
	dec, _ := NewDecoder(Mode20ms)
	out, err := dec.Decode(make([]byte, 38))
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range out {
		if v > 32767 || v < -32768 {
			t.Fatalf("sample %d out of int16 range: %d", i, v)
		}
	}
}
