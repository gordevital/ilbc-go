package main

import (
	"fmt"
	"math"

	"github.com/gordevital/ilbc-go/ilbc"
)

func main() {
	for _, m := range []ilbc.Mode{ilbc.Mode20ms, ilbc.Mode30ms} {
		enc, err := ilbc.NewEncoder(m)
		if err != nil {
			panic(err)
		}
		dec, err := ilbc.NewDecoder(m)
		if err != nil {
			panic(err)
		}
		n := enc.BlockLen()
		frames := 50

		var sig, rec []float64
		for f := 0; f < frames; f++ {
			pcm := make([]int16, n)
			for i := 0; i < n; i++ {
				t := float64(f*n + i)
				v := 6000 * math.Sin(2*math.Pi*440*t/8000)
				pcm[i] = int16(v)
				sig = append(sig, v)
			}
			payload, err := enc.Encode(pcm)
			if err != nil {
				panic(err)
			}
			out, err := dec.Decode(payload)
			if err != nil {
				panic(err)
			}
			for _, v := range out {
				rec = append(rec, float64(v))
			}
		}

		// The codec has a fixed algorithmic delay (~10-25 ms); score the
		// result with a lag-compensated normalized cross-correlation,
		// since a zero-lag SNR against a narrow-band tone is misleading.
		best, bestLag := 0.0, 0
		for lag := 0; lag <= 2*n; lag += 4 {
			k := len(sig) - lag
			if k < n || k <= 0 {
				break
			}
			var ma, mb, va, vb, cov float64
			for i := 0; i < k; i++ {
				ma += sig[i+lag]
				mb += rec[i]
			}
			ma /= float64(k)
			mb /= float64(k)
			for i := 0; i < k; i++ {
				a := sig[i+lag] - ma
				b := rec[i] - mb
				cov += a * b
				va += a * a
				vb += b * b
			}
			if c := cov / math.Sqrt(va*vb+1e-9); c > best {
				best, bestLag = c, lag
			}
		}
		fmt.Printf("mode %dms: frames=%d payload=%dB best corr=%.3f @ %d samples (%.1f ms delay)\n",
			int(m), frames, m.PayloadLen(), best, bestLag, float64(bestLag)/8)
	}
}
