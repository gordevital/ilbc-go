package rtputil

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// SR is the data of a Sender Report (RFC 3550 6.4.1).
type SR struct {
	SSRC    uint32
	NTP     uint64 // NTP midpoint: secs<<32|frac
	RTPTS   uint32
	Packets uint32
	Octets  uint32
}

// Pack serializes the SR as a complete RTCP packet (no compound framing).
func (s SR) Pack() []byte {
	const size = 24 // 1 RTCP receiver block, in 32-bit words
	b := make([]byte, 4+4*size)
	b[1] = RTCPPayloadSR
	binary.BigEndian.PutUint16(b[2:4], uint16(size-1))
	binary.BigEndian.PutUint32(b[4:8], s.SSRC)
	binary.BigEndian.PutUint64(b[8:16], s.NTP)
	binary.BigEndian.PutUint32(b[16:20], s.RTPTS)
	binary.BigEndian.PutUint32(b[20:24], s.Packets)
	binary.BigEndian.PutUint32(b[24:28], s.Octets)
	return b
}

// RRSample is one receiver block of a Receiver Report (RFC 3550 6.4.2).
type RRSample struct {
	SSRC           uint32
	FractionLost   uint8
	CumulativeLost uint32 // lower 24 bits honored
	ExtHighestSeq  uint32
	Jitter         uint32
	LSR            uint32
	DLSR           uint32
}

// PackRR serializes a Receiver Report carrying one receiver block.
func PackRR(receiver uint32, blk RRSample) []byte {
	const size = 6
	b := make([]byte, 4+4*size)
	b[1] = RTCPPayloadRR
	binary.BigEndian.PutUint16(b[2:4], uint16(size-1))
	binary.BigEndian.PutUint32(b[4:8], receiver)
	binary.BigEndian.PutUint32(b[8:12], blk.SSRC)
	b[12] = blk.FractionLost
	lost := blk.CumulativeLost & 0x00FFFFFF
	b[13] = byte(lost >> 16)
	b[14] = byte(lost >> 8)
	b[15] = byte(lost)
	binary.BigEndian.PutUint32(b[16:20], blk.ExtHighestSeq)
	binary.BigEndian.PutUint32(b[20:24], blk.Jitter)
	binary.BigEndian.PutUint32(b[24:28], blk.LSR)
	binary.BigEndian.PutUint32(b[28:32], blk.DLSR)
	return b
}

// ParsedRTCP is the view of a single RTCP packet we care about.
type ParsedRTCP struct {
	Type   int
	SSRC   uint32
	SR     *SR
	RRSpec *RRSample // receiver = SSRC, block in RRSpec
}

// ErrUnsupportedRTCP marks packets accepted by the parser loop but not
// decoded (BYE, SDES, APP...).
var ErrUnsupportedRTCP = errors.New("rtputil: unsupported RTCP type")

// ParseCompound walks a compound RTCP datagram and returns the understood
// packets; unknown types yield an entry with Type set and nil details.
func ParseCompound(dg []byte) ([]ParsedRTCP, error) {
	var out []ParsedRTCP
	for len(dg) >= 4 {
		ptype := int(dg[1])
		length := int(binary.BigEndian.Uint16(dg[2:4])) * 4
		size := 4 + length
		if len(dg) < size {
			return out, fmt.Errorf("rtputil: truncated RTCP packet (have %d, need %d)", len(dg), size)
		}
		chunk := dg[:size]
		ssrc := binary.BigEndian.Uint32(chunk[4:8])
		p := ParsedRTCP{Type: ptype, SSRC: ssrc}
		switch ptype {
		case RTCPPayloadSR:
			sr := SR{
				SSRC:    ssrc,
				NTP:     binary.BigEndian.Uint64(chunk[8:16]),
				RTPTS:   binary.BigEndian.Uint32(chunk[16:20]),
				Packets: binary.BigEndian.Uint32(chunk[20:24]),
				Octets:  binary.BigEndian.Uint32(chunk[24:28]),
			}
			p.SR = &sr
		case RTCPPayloadRR:
			blk := RRSample{
				SSRC:           binary.BigEndian.Uint32(chunk[8:12]),
				FractionLost:   chunk[12],
				CumulativeLost: uint32(chunk[13])<<16 | uint32(chunk[14])<<8 | uint32(chunk[15]),
				ExtHighestSeq:  binary.BigEndian.Uint32(chunk[16:20]),
				Jitter:         binary.BigEndian.Uint32(chunk[20:24]),
				LSR:            binary.BigEndian.Uint32(chunk[24:28]),
				DLSR:           binary.BigEndian.Uint32(chunk[28:32]),
			}
			p.RRSpec = &blk
		}
		out = append(out, p)
		dg = dg[size:]
	}
	return out, nil
}

// ElapsedRTPTicks returns (elapsed nanoseconds, rtp ticks) between two NTP
// midpoints, useful for DLSR computation (RFC 3550 6.4.2).
func ElapsedRTPTicks(now, lsr uint64) (time.Duration, uint32) {
	ns := int64(now-lsr) * 1e9 / 4294967296
	if ns < 0 {
		ns = 0
	}
	return time.Duration(ns), uint32(float64(ns) / 1e9 * float64(ClockRate))
}
