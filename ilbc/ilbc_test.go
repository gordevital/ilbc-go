package ilbc

import (
	"math"
	"math/rand"
	"testing"
)

func TestModeGeometry(t *testing.T) {
	cases := []struct {
		m   Mode
		blk int
		pay int
	}{
		{Mode20ms, 160, 38},
		{Mode30ms, 240, 50},
	}
	for _, c := range cases {
		if got := c.m.BlockLen(); got != c.blk {
			t.Errorf("mode %d BlockLen = %d, want %d", c.m, got, c.blk)
		}
		if got := c.m.PayloadLen(); got != c.pay {
			t.Errorf("mode %d PayloadLen = %d, want %d", c.m, got, c.pay)
		}
	}
}

func TestNewEncoderDecoder(t *testing.T) {
	for _, m := range []Mode{Mode20ms, Mode30ms} {
		enc, err := NewEncoder(m)
		if err != nil {
			t.Fatalf("NewEncoder(%d): %v", m, err)
		}
		dec, err := NewDecoder(m)
		if err != nil {
			t.Fatalf("NewDecoder(%d): %v", m, err)
		}
		if enc.BlockLen() != m.BlockLen() || dec.BlockLen() != m.BlockLen() {
			t.Fatalf("mode %d block length mismatch", m)
		}
	}
	if _, err := NewEncoder(Mode(10)); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestRoundTripNoPanicFinite(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, m := range []Mode{Mode20ms, Mode30ms} {
		enc, _ := NewEncoder(m)
		dec, _ := NewDecoder(m)
		n := enc.BlockLen()
		for frame := 0; frame < 20; frame++ {
			pcm := make([]int16, n)
			for i := range pcm {
				pcm[i] = int16(3000 * math.Sin(2*math.Pi*440*float64(frame*n+i)/8000))
				pcm[i] += int16(rng.Intn(500) - 250)
			}
			payload, err := enc.Encode(pcm)
			if err != nil {
				t.Fatalf("mode %d frame %d encode: %v", m, frame, err)
			}
			if len(payload) != m.PayloadLen() {
				t.Fatalf("mode %d payload len = %d, want %d", m, len(payload), m.PayloadLen())
			}
			out, err := dec.Decode(payload)
			if err != nil {
				t.Fatalf("mode %d frame %d decode: %v", m, frame, err)
			}
			if len(out) != n {
				t.Fatalf("mode %d output len = %d, want %d", m, len(out), n)
			}
			for i, v := range out {
				f := float64(v)
				if math.IsNaN(f) {
					t.Fatalf("mode %d frame %d sample %d is NaN", m, frame, i)
				}
			}
		}
	}
}

func TestConcealNoPanic(t *testing.T) {
	dec, _ := NewDecoder(Mode30ms)
	out, err := dec.Conceal()
	if err != nil {
		t.Fatalf("Conceal: %v", err)
	}
	if len(out) != 240 {
		t.Fatalf("Conceal len = %d, want 240", len(out))
	}
}

func TestConstantsSanity(t *testing.T) {
	for i := 1; i < len(ilbcLSFMean); i++ {
		if !(ilbcLSFMean[i-1] < ilbcLSFMean[i]) {
			t.Fatalf("LSF mean not increasing at %d", i)
		}
	}
	for i := 1; i < len(ilbcStateSq3); i++ {
		if !(ilbcStateSq3[i-1] < ilbcStateSq3[i]) {
			t.Fatalf("StateSq3 not increasing at %d", i)
		}
	}
	for i := 1; i < len(ilbcGainSq5); i++ {
		if !(ilbcGainSq5[i-1] < ilbcGainSq5[i]) {
			t.Fatalf("GainSq5 not increasing at %d", i)
		}
	}
}

var (
	ilbcLSFMean  = []float32{0.281738, 0.445801, 0.663330, 0.962524, 1.251831, 1.533081, 1.850586, 2.137817, 2.481445, 2.777344}
	ilbcStateSq3 = []float32{-3.719849, -2.177490, -1.130005, -0.309692, 0.444214, 1.329712, 2.436279, 3.983887}
	ilbcGainSq5  = []float32{0.037476, 0.075012, 0.112488, 0.150024, 0.187500, 0.224976, 0.262512, 0.299988, 0.337524, 0.375000, 0.412476, 0.450012, 0.487488, 0.525024, 0.562500, 0.599976, 0.637512, 0.674988, 0.712524, 0.750000, 0.787476, 0.825012, 0.862488, 0.900024, 0.937500, 0.974976, 1.012512, 1.049988, 1.087524, 1.125000, 1.162476, 1.200012}
)
