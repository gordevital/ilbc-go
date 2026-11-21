// Command ilbcwav2rtp converts a WAV file into a stream of iLBC-over-RTP
// packets (RFC 3952 framing: 12-byte RTP header + 1-byte iLBC header +
// 38/50-byte payload), written consecutively to the output file.
//
// With -key/-salt each packet is additionally secured with SRTP
// (AEAD AES-GCM, 16-byte tag appended after the payload).
//
// Usage: ilbcwav2rtp [-mode 20|30] [-pt n] [-ssrc n] [-key HEX -salt HEX] in.wav out.rtp
package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"github.com/gordevital/ilbc-go/internal/cliflags"
	"log"
	"math"
	"os"

	ilbcapi "github.com/gordevital/ilbc-go/ilbc"
	"github.com/gordevital/ilbc-go/internal/srtp"
	"github.com/gordevital/ilbc-go/internal/wavio"
)

func main() {
	mode := flag.Int("mode", 20, "iLBC frame length in ms (20 or 30)")
	pt := flag.Int("pt", 98, "RTP payload type")
	ssrc := flag.Int("ssrc", 0xA1BCC0DE, "RTP synchronization source")
	key := flag.String("key", "", "SRTP master key, hex (32 or 64 chars); implies -salt")
	salt := flag.String("salt", "", "SRTP master salt, hex (24 chars)")
	flag.CommandLine.Parse(cliflags.Ordered(os.Args[1:]))
	if flag.NArg() != 2 {
		log.Fatal("usage: ilbcwav2rtp [-mode 20|30] [-pt n] [-ssrc n] [-key HEX -salt HEX] in.wav out.rtp")
	}
	if *mode != 20 && *mode != 30 {
		log.Fatal("-mode must be 20 or 30")
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
	in, outPath := flag.Arg(0), flag.Arg(1)

	wav, err := wavio.Read(in)
	if err != nil {
		log.Fatalf("read %s: %v", in, err)
	}
	samples := wavio.Resample(wav.Samples, wav.SampleRate, 8000)

	en, err := ilbcapi.NewEncoder(ilbcapi.Mode(*mode))
	if err != nil {
		log.Fatalf("init encoder: %v", err)
	}
	frames := len(samples) / en.BlockLen()
	if tail := len(samples) % en.BlockLen(); tail != 0 {
		samples = append(samples, make([]wavio.Sample, en.BlockLen()-tail)...)
		frames++
	}

	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create %s: %v", outPath, err)
	}
	defer f.Close()
	hdr := make([]byte, 12)
	seq := uint16(0)
	ts := uint32(0)
	frameTs := uint32(160)
	if *mode == 30 {
		frameTs = 240
	}
	plen := 0
	for fr := 0; fr < frames; fr++ {
		frame := make([]int16, en.BlockLen())
		for i, v := range samples[fr*en.BlockLen() : (fr+1)*en.BlockLen()] {
			s := float64(v) * 32767
			if s > 32767 {
				s = 32767
			} else if s < -32768 {
				s = -32768
			}
			frame[i] = int16(math.Round(s))
		}
		payload, err := en.Encode(frame)
		if err != nil {
			log.Fatalf("frame %d: %v", fr, err)
		}
		plen = len(payload)
		hdr[0] = 0x80                             // version 2
		hdr[1] = byte(*pt)                        // payload type
		binary.BigEndian.PutUint16(hdr[2:4], seq) // sequence
		binary.BigEndian.PutUint32(hdr[4:8], ts)  // timestamp
		binary.BigEndian.PutUint32(hdr[8:12], uint32(*ssrc))
		ilbcHdr := byte(0)
		if *mode == 20 {
			ilbcHdr = 1
		}
		pkt := make([]byte, 0, 12+1+plen)
		pkt = append(pkt, hdr...)
		pkt = append(pkt, ilbcHdr)
		pkt = append(pkt, payload...)
		if sess != nil {
			prot, err := sess.ProtectRTP(pkt)
			if err != nil {
				log.Fatalf("frame %d: SRTP protect: %v", fr, err)
			}
			pkt = prot
		}
		f.Write(pkt)
		seq++
		ts += frameTs
	}
	pktSize := plen + 13
	srtpTag := ""
	if sess != nil {
		pktSize += srtp.TagLen
		srtpTag = ", SRTP AES-GCM"
	}
	mediaKbps := float64(frames) * float64(plen) * 8.0 / (float64(frames) * float64(*mode) / 1000.0) / 1000.0
	packetKbps := mediaKbps * float64(pktSize) / float64(plen)
	log.Printf("%s: %d frames (%d ms) -> %s: packet=%d B (payload=%d B)%s, %0.1f kbps media, %0.1f kbps with RTP",
		in, frames, frames*(*mode), outPath, pktSize, plen, srtpTag, mediaKbps, packetKbps)

}
