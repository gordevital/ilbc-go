// Command ilbcgenspeech generates a synthetic speech-like 8 kHz WAV:
// a glottal pulse train through a cascaded second-order formant filter,
// with F0 drift, syllabic gating and short unvoiced (noise) gaps.
// Deterministic (fixed seed), for codec QA.
//
// Usage: ilbcgenspeech out.wav [-sec n]
package main

import (
	"flag"
	"github.com/gordevital/ilbc-go/internal/cliflags"
	"log"
	"math"
	"math/rand"
	"os"

	"github.com/gordevital/ilbc-go/internal/wavio"
)

// Vowel formant sets (F1 F2 F3 Hz, bandwidths, gains).
type vowelSet struct {
	f, bw, g [3]float64
}

var vowels = []vowelSet{
	{f: [3]float64{730, 1090, 2440}, bw: [3]float64{90, 110, 130}, g: [3]float64{1.0, 0.5, 0.25}},
	{f: [3]float64{300, 870, 2240}, bw: [3]float64{70, 80, 100}, g: [3]float64{1.0, 0.6, 0.2}},
	{f: [3]float64{270, 2290, 3010}, bw: [3]float64{60, 90, 120}, g: [3]float64{1.0, 0.7, 0.3}},
	{f: [3]float64{450, 800, 2830}, bw: [3]float64{80, 100, 110}, g: [3]float64{1.0, 0.6, 0.25}},
}

type so struct {
	a1, a2, x1, x2 float64
}

func (s *so) run(in, w0, Q float64) float64 {
	s.a1 = -2 * math.Exp(-w0/(2*Q)) * math.Cos(w0)
	s.a2 = math.Exp(-w0 / Q)
	out := in - s.a1*s.x1 - s.a2*s.x2
	s.x2, s.x1 = s.x1, out
	return out
}

func main() {
	sec := flag.Float64("sec", 6, "duration in seconds")
	flag.CommandLine.Parse(cliflags.Ordered(os.Args[1:]))
	if flag.NArg() != 1 {
		log.Fatal("usage: ilbcgenspeech out.wav [-sec n]")
	}
	const sr = 8000
	n := int(*sec * sr)
	rng := rand.New(rand.NewSource(42))
	out := make([]wavio.Sample, n)

	t := 0
	syll := 0
	for t < n {
		dur := int(float64(sr) * (0.08 + rng.Float64()*0.05))
		if t+dur > n {
			dur = n - t
		}
		end := t + dur
		v := vowels[syll%len(vowels)]
		syll++
		f0 := 110 + 25*math.Sin(2*math.Pi*float64(t)/(sr*0.9))
		ph := rng.Float64()
		var res [3]so
		for i := t; i < end; i++ {
			ph += f0 / sr
			if ph >= 1.0 {
				ph -= 1.0
			}
			// Trapezoidal glottal-flow derivative, closed phase = 0.4.
			var g float64
			switch {
			case ph < 0.15:
				g = ph / 0.15 * 2.0
			case ph < 0.4:
				g = 2.0
			case ph < 0.9:
				g = (0.9 - ph) / 0.5 * 2.0
			default:
				g = 0
			}
			sig := g
			for k := 0; k < 3; k++ {
				w0 := 2 * math.Pi * v.f[k] / sr
				sig = res[k].run(sig, w0, v.f[k]/v.bw[k]) * v.g[k] * 0.2
			}
			prog := float64(i-t) / float64(dur)
			env := 1.0 - 0.4*math.Abs(prog-0.5)*2
			out[i] = wavio.Sample(0.8 * sig * env)
		}
		t = end
		gap := int(float64(sr) * (0.03 + rng.Float64()*0.03))
		if t+gap > n {
			gap = n - t
		}
		for i := t; i < t+gap; i++ {
			out[i] = wavio.Sample((rng.Float64()*2 - 1) * 0.02)
		}
		t += gap
	}
	var peak float64 = 1e-9
	for _, vv := range out {
		if a := math.Abs(float64(vv)); a > peak {
			peak = a
		}
	}
	sc := 0.707 / peak
	for i := range out {
		x := float64(out[i]) * sc
		if x > 1 {
			x = 1
		}
		if x < -1 {
			x = -1
		}
		out[i] = wavio.Sample(x)
	}
	if err := wavio.WritePCM16(flag.Arg(0), out, sr); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("%s: %d samples (%.2f s), 8 kHz", flag.Arg(0), len(out), float64(len(out))/sr)
}
