package wavio

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestGG711VsFFmpeg compares our G.711 expansion against FFmpeg's reference.
func TestG711VsFFmpeg(t *testing.T) {
	busy := "/usr/share/baresip/busy.wav"
	if _, err := os.Stat(busy); err != nil {
		t.Skip("busy.wav not present")
	}
	our, err := Read(busy)
	if err != nil {
		t.Fatal(err)
	}
	lin := "/tmp/opencode/busy_lin.wav"
	cmd := exec.Command("ffmpeg", "-y", "-loglevel", "error", "-i", busy, "-c:a", "pcm_s16le", lin)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %s %v", out, err)
	}
	ff, err := Read(lin)
	if err != nil {
		t.Fatal(err)
	}
	n := len(our.Samples)
	if n > len(ff.Samples) {
		n = len(ff.Samples)
	}
	var maxD, acc float64
	for i := 0; i < n; i++ {
		dv := float64(our.Samples[i]) - float64(ff.Samples[i])
		if dv < 0 {
			dv = -dv
		}
		if dv > maxD {
			maxD = dv
		}
		acc += dv
	}
	fmt.Printf("busy.wav: n=%d maxdiff=%.6f (%.1f/32768) mean=%.6f\n", n, maxD, maxD*32768, acc/float64(n))
}
