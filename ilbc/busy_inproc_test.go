package ilbc

import (
	"math"
	"testing"

	"github.com/gordevital/ilbc-go/internal/wavio"
)

func TestBusyInProc(t *testing.T) {
	wav, err := wavio.Read("/usr/share/baresip/busy.wav")
	if err != nil {
		t.Skip(err.Error())
	}
	smp := wavio.Resample(wav.Samples, wav.SampleRate, 8000)
	for _, mode := range []Mode{Mode20ms, Mode30ms} {
		en, _ := NewEncoder(mode)
		de, _ := NewDecoder(mode)
		var dec []float64
		for i := 0; i+en.BlockLen() <= len(smp); i += en.BlockLen() {
			fr := make([]int16, en.BlockLen())
			for j := 0; j < en.BlockLen(); j++ {
				s := float64(smp[i+j]) * 32767
				if s > 32767 {
					s = 32767
				} else if s < -32768 {
					s = -32768
				}
				fr[j] = int16(math.Round(s))
			}
			pl, _ := en.Encode(fr)
			out, err := de.Decode(pl)
			if err != nil {
				t.Fatalf("%d: %v", mode, err)
			}
			for _, v := range out {
				dec = append(dec, float64(v)/32767)
			}
		}
		var es, rs, cross float64
		n := len(dec)
		if n > len(smp) {
			n = len(smp)
		}
		for i := 0; i < n; i++ {
			d := float64(smp[i]) - dec[i]
			es += d * d
			rs += float64(smp[i]) * float64(smp[i])
			cross += float64(smp[i]) * dec[i]
		}
		corr := cross / math.Sqrt(rs*(rs-es)+1e-9)
		t.Logf("mode %d: in-proc SNR=%0.2f dB corr=%0.3f", mode, 10*math.Log10(rs/es), corr)
	}
}
