// Package wavio is a minimal stdlib-only WAV reader/writer covering the
// formats needed for iLBC interop tests: 16-bit PCM (mono/stereo) and
// G.711 A/u-law, sampled at any rate (callers resample).
package wavio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// Sample is a mono audio sample in [-1, 1].
type Sample float32

// Data holds decoded mono audio.
type Data struct {
	SampleRate int
	Samples    []Sample
}

// Read parses a RIFF/WAVE file into mono float samples. Supported formats:
// 1 (PCM 16-bit LE), 6 (A-law), 7 (u-law). Stereo inputs take the average
// of the channels.
func Read(path string) (*Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f)
}

func Decode(r io.Reader) (*Data, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("wav: read: %w", err)
	}
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, errors.New("wav: not a RIFF/WAVE file")
	}
	var (
		formatTag  uint16
		channels   uint16
		sampleRate uint32
		bits       uint16
		dataLen    int64 = -1
		dataOff    int64 = -1
	)
	off := 12
	for off+8 <= len(raw) {
		id := string(raw[off : off+4])
		size := int64(binary.LittleEndian.Uint32(raw[off+4:]))
		off += 8
		if id == "fmt " && size >= 16 {
			fb := raw[off : off+16]
			formatTag = binary.LittleEndian.Uint16(fb[0:])
			channels = binary.LittleEndian.Uint16(fb[2:])
			sampleRate = binary.LittleEndian.Uint32(fb[4:])
			bits = binary.LittleEndian.Uint16(fb[14:])
			off += 16
			size -= 16
		}
		if dataOff < 0 && id == "data" {
			dataOff = int64(off)
			dataLen = size
		}
		off += int(size)
		if int(size)&1 == 1 {
			off++
		}
	}
	if sampleRate == 0 || channels == 0 {
		return nil, errors.New("wav: missing fmt chunk")
	}
	if dataOff < 0 {
		return nil, errors.New("wav: no data chunk")
	}
	if channels > 16 {
		return nil, fmt.Errorf("wav: unsupported channel count %d", channels)
	}

	var buf []byte
	if dataOff >= int64(len(raw)) {
		buf = nil
	} else if dataLen <= 0 || dataOff+dataLen > int64(len(raw)) {
		buf = raw[dataOff:]
	} else {
		buf = raw[dataOff : dataOff+dataLen]
	}
	if len(buf)%2 != 0 {
		buf = buf[:len(buf)-1]
	}

	switch formatTag {
	case 1:
		if bits != 16 {
			return nil, fmt.Errorf("wav: unsupported PCM width %d", bits)
		}
		frame := 2 * int(channels)
		n := len(buf) / frame
		d := &Data{SampleRate: int(sampleRate), Samples: make([]Sample, n)}
		for i := 0; i < n; i++ {
			var acc float64
			for c := 0; c < int(channels); c++ {
				off := i*frame + c*2
				v := int16(binary.LittleEndian.Uint16(buf[off:]))
				acc += float64(v) / 32768.0
			}
			d.Samples[i] = Sample(acc / float64(channels))
		}
		return d, nil
	case 6: // A-law
		d := &Data{SampleRate: int(sampleRate)}
		return g711(d, buf, channels, AlawLin)
	case 7: // u-law
		d := &Data{SampleRate: int(sampleRate)}
		return g711(d, buf, channels, MulawLin)
	default:
		return nil, fmt.Errorf("wav: unsupported format tag %d", formatTag)
	}
}

func g711(d *Data, buf []byte, channels uint16, tbl [256]int16) (*Data, error) {
	n := len(buf) / int(channels)
	d.Samples = make([]Sample, n)
	for i := 0; i < n; i++ {
		var acc float64
		for c := 0; c < int(channels); c++ {
			acc += float64(tbl[buf[i*int(channels)+c]]) / 32768.0
		}
		d.Samples[i] = Sample(acc / float64(channels))
	}
	return d, nil
}

// Resample linearly interpolates samples from inRate to outRate.
func Resample(in []Sample, inRate, outRate int) []Sample {
	if inRate == outRate {
		out := make([]Sample, len(in))
		copy(out, in)
		return out
	}
	ratio := float64(inRate) / float64(outRate)
	n := int(float64(len(in))/ratio + 0.5)
	out := make([]Sample, n)
	for i := 0; i < n; i++ {
		pos := float64(i) * ratio
		i0 := int(pos)
		frac := float64(pos - float64(i0))
		if i0+1 < len(in) {
			out[i] = Sample(float64(in[i0])*(1-frac) + float64(in[i0+1])*frac)
		} else if i0 < len(in) {
			out[i] = in[i0]
		}
	}
	return out
}

// WritePCM16 writes samples ([-1,1]) as a 16-bit PCM mono RIFF/WAVE file.
func WritePCM16(path string, s []Sample, sampleRate int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	n := len(s)
	frame := n * 2
	fmt.Fprint(f, "RIFF")
	binary.Write(f, binary.LittleEndian, uint32(36+frame))
	fmt.Fprint(f, "WAVEfmt ")
	binary.Write(f, binary.LittleEndian, uint32(16))
	binary.Write(f, binary.LittleEndian, uint16(1))                    // PCM
	binary.Write(f, binary.LittleEndian, uint16(1))                    // mono
	binary.Write(f, binary.LittleEndian, uint32(uint32(sampleRate)))   // rate
	binary.Write(f, binary.LittleEndian, uint32(uint32(sampleRate)*2)) // byte rate
	binary.Write(f, binary.LittleEndian, uint16(2))                    // block align
	binary.Write(f, binary.LittleEndian, uint16(16))                   // bits
	fmt.Fprint(f, "data")
	binary.Write(f, binary.LittleEndian, uint32(frame))
	buf := make([]byte, frame)
	for i, v := range s {
		x := int32(v * 32767.0)
		if x > 32767 {
			x = 32767
		}
		if x < -32768 {
			x = -32768
		}
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(x))
	}
	_, err = f.Write(buf)
	return err
}
