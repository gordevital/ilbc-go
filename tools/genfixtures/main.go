// genfixtures regenerates the deterministic iLBC interop test signals.
//
// Usage: go run ./tools/genfixtures -o <testdata dir> [-frames 20]
//
// Signals (16 kHz, signed 16-bit little endian, mono):
//
//	  sine<N>M: 440 Hz sine (amp 10000) + 0.25x harmonic at 880 Hz.
//	  noise<N>M: LCG(seed 12345) white noise, 7-tap box smoothing,
//
//		amplitude limited to +-6000.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

func main() {
	outDir := flag.String("o", ".", "output directory for fixture pcms")
	frames := flag.Int("frames", 20, "number of iLBC frames per fixture")
	flag.Parse()

	names := map[string][]int16{
		"sine":  sineSignal(16000, 16000),
		"noise": noiseSignal(16000, 16000),
	}
	for name, all := range names {
		for _, mode := range []int{20, 30} {
			bl := 160
			if mode == 30 {
				bl = 240
			}
			want := bl * *frames
			if len(all) < want {
				panic(fmt.Sprintf("%s: need %d samples, have %d", name, want, len(all)))
			}
			p := filepath.Join(*outDir, fmt.Sprintf("%s%d.pcm", name, mode))
			if err := writePCM(p, all[:want]); err != nil {
				panic(err)
			}
		}
	}
}

func sineSignal(fs, n int) []int16 {
	out := make([]int16, n)
	w1 := 2 * math.Pi * 440 / float64(fs)
	w2 := 2 * math.Pi * 880 / float64(fs)
	for i := 0; i < n; i++ {
		t := float64(i)
		v := 10000*math.Sin(w1*t) + 2500*math.Sin(w2*t)
		out[i] = int16(math.Round(v))
	}
	return out
}

func noiseSignal(fs, n int) []int16 {
	white := make([]int32, n)
	seed := int64(12345)
	for i := 0; i < n; i++ {
		seed = seed*1103515245 + 12345
		white[i] = int32((seed>>16)&0xFFFF) - 32768
	}
	out := make([]int16, n)
	const taps = 7
	for i := 0; i < n; i++ {
		sum := int64(0)
		for k := 0; k < taps; k++ {
			j := i - k
			if j < 0 {
				continue
			}
			sum += int64(white[j])
		}
		scale := float64(sum) / float64(taps) * 0.15
		v := int32(math.Round(scale))
		if v > 6000 {
			v = 6000
		}
		if v < -6000 {
			v = -6000
		}
		out[i] = int16(v)
	}
	return out
}

func writePCM(path string, samples []int16) error {
	buf := make([]byte, 2*len(samples))
	for i, s := range samples {
		u := uint16(s)
		buf[2*i] = byte(u)
		buf[2*i+1] = byte(u >> 8)
	}
	return os.WriteFile(path, buf, 0o644)
}
