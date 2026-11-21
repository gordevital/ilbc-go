// Command ilbcrtp2wav decodes a stream of iLBC-over-RTP packets (as produced
// by ilbcwav2rtp) back into a PCM16 8 kHz mono WAV file. Streams produced
// with -key/-salt are accepted transparently (the frame length is recovered
// by trying the 20/30-ms SRTP packet sizes and accepting whichever
// authenticates). With -ref it also computes SNR/RMSE against a reference
// WAV.
//
// Usage: ilbcrtp2wav in.rtp out.wav [-key HEX -salt HEX] [-ref ref.wav]
package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"github.com/gordevital/ilbc-go/internal/cliflags"
	"log"
	"math"
	"os"

	ilbcapi "github.com/gordevital/ilbc-go/ilbc"
	"github.com/gordevital/ilbc-go/internal/srtp"
	"github.com/gordevital/ilbc-go/internal/wavio"
)

func main() {
	ref := flag.String("ref", "", "reference WAV for SNR computation")
	key := flag.String("key", "", "SRTP master key, hex (32 or 64 chars); implies -salt")
	salt := flag.String("salt", "", "SRTP master salt, hex (24 chars)")
	flag.CommandLine.Parse(cliflags.Ordered(os.Args[1:]))
	if flag.NArg() != 2 {
		log.Fatal("usage: ilbcrtp2wav in.rtp out.wav [-key HEX -salt HEX] [-ref ref.wav]")
	}
	raw, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalf("read %s: %v", flag.Arg(0), err)
	}
	var sess *srtp.Session
	if *key != "" || *salt != "" {
		mk, err := hex.DecodeString(*key)
		if err != nil {
			log.Fatalf("bad -key hex: %v", err)
		}
		ms, err := hex.DecodeString(*salt)
		if err != nil {
			log.Fatalf("bad -salt hex: %v", err)
		}
		sess, err = srtp.NewSession(mk, ms)
		if err != nil {
			log.Fatalf("SRTP session: %v", err)
		}
	}

	var decs [2]*ilbcapi.Decoder
	var out []wavio.Sample
	packets := 0
	maxSeq := uint16(0)
	var prevTS uint32
	gaps := 0
	for off := 0; off < len(raw); {
		if raw[off]&0xC0 != 0x80 {
			log.Fatalf("bad RTP header at offset %d", off)
		}
		seq := binary.BigEndian.Uint16(raw[off+2:])
		ts := binary.BigEndian.Uint32(raw[off+4:])
		var packet []byte
		var advance int
		if sess != nil {
			p, adv, err := srtpPacketAt(sess, raw, off)
			if err != nil {
				log.Fatalf("offset %d: %v", off, err)
			}
			packet = p
			advance = adv
		} else {
			packet = raw[off:]
		}
		payload := packet[12:]
		if len(payload) < 1 {
			break
		}
		is20 := payload[0]&1 == 1
		plen := 50
		mode := 30
		if is20 {
			plen = 38
			mode = 20
		}
		idx := 0
		if is20 {
			idx = 1
		}
		if len(payload) < 1+plen {
			log.Fatalf("packet %d: short payload %d < %d", packets, len(payload), 1+plen)
		}
		if decs[idx] == nil {
			d, err := ilbcapi.NewDecoder(ilbcapi.Mode(mode))
			if err != nil {
				log.Fatalf("init decoder: %v", err)
			}
			decs[idx] = d
		}
		frame, err := decs[idx].Decode(payload[1 : 1+plen])
		if err != nil {
			log.Fatalf("packet %d: decode: %v", packets, err)
		}
		for _, v := range frame {
			out = append(out, wavio.Sample(v)/32767)
		}
		if packets > 0 {
			if int32(ts-prevTS) < 0 {
				gaps++
			}
			if seq != maxSeq+1 {
				gaps++
			}
		}
		prevTS = ts
		maxSeq = seq
		packets++
		if sess != nil {
			off += advance
		} else {
			off += 12 + 1 + plen
		}
	}
	if packets == 0 {
		log.Fatal("no RTP packets found")
	}
	if err := wavio.WritePCM16(flag.Arg(1), out, 8000); err != nil {
		log.Fatalf("write %s: %v", flag.Arg(1), err)
	}
	srtpTag := ""
	if sess != nil {
		srtpTag = " (SRTP)"
	}
	fmt.Printf("%s%s: %d packets, %d samples (%d ms), seq 0..%d, ts gap markers: %d\n",
		flag.Arg(0), srtpTag, packets, len(out), len(out)/8, maxSeq, gaps)

	if *ref != "" {
		r, err := wavio.Read(*ref)
		if err != nil {
			log.Fatalf("read ref %s: %v", *ref, err)
		}
		rs := wavio.Resample(r.Samples, r.SampleRate, 8000)
		n := len(rs)
		if n > len(out) {
			n = len(out)
		}
		var es, ts2, peak float64
		rmseMax := 0.0
		for i := 0; i < n; i++ {
			dv := float64(rs[i]) - float64(out[i])
			es += dv * dv
			ts2 += float64(rs[i]) * float64(rs[i])
			if math.Abs(float64(out[i])) > peak {
				peak = math.Abs(float64(out[i]))
			}
			if math.Abs(dv) > rmseMax {
				rmseMax = math.Abs(dv)
			}
		}
		snr := 10 * math.Log10(ts2/es)
		fmt.Printf("vs %s: n=%d SNR=%0.2f dB, RMSE=%.1f/32768, peak=%.1f/32768\n",
			*ref, n, snr, math.Sqrt(es/float64(n))*32768, peak*32768)
	}
}

// srtpPacketAt recovers the SRTP-protected RTP packet starting at raw[off].
// Under GCM the iLBC header bit (20 vs 30 ms) is unreadable, so both
// canonical payload lengths are tried and the one that authenticates wins.
// The second return value is the on-wire size (cleartext length + tag).
func srtpPacketAt(sess *srtp.Session, raw []byte, off int) ([]byte, int, error) {
	for _, plen := range []int{38, 50} {
		size := 12 + 1 + plen + srtp.TagLen
		if off+size > len(raw) {
			continue
		}
		pkt, err := sess.UnprotectRTP(raw[off : off+size])
		if err == nil && len(pkt) == 12+1+plen {
			return pkt, size, nil
		}
	}
	return nil, 0, errors.New("no SRTP packet authenticates (wrong -key/-salt or truncated stream)")
}
