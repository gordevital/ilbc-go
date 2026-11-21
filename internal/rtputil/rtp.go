// Package rtputil provides minimal RTP/RTCP (RFC 3550) primitives for the
// iLBC transport tools: RTP header marshal/parse, NTP time conversion and
// Sender Report / Receiver Report packing.
package rtputil

import (
	"encoding/binary"
	"time"
)

// ClockRate is the RTP clock rate for iLBC (8 kHz PCM).
const ClockRate = 8000

// RTCP payload types.
const (
	RTCPBye       = 203
	RTCPPayloadSR = 200
	RTCPPayloadRR = 201
)

// Header is the 12-byte RTP fixed header.
type Header struct {
	Version     uint8 // must be 2
	Padding     bool
	Extension   bool
	CC          uint8
	Marker      bool
	PayloadType uint8
	Sequence    uint16
	Timestamp   uint32
	SSRC        uint32
}

const MinRTPPktLen = 12

// Marshal renders the fixed header (CSRC/extension not supported).
func (h Header) Marshal() []byte {
	b := make([]byte, MinRTPPktLen)
	b[0] = 0x80 | (h.CC & 0x0F)
	if h.Padding {
		b[0] |= 0x20
	}
	if h.Extension {
		b[0] |= 0x10
	}
	b[1] = byte(h.PayloadType) & 0x7F
	if h.Marker {
		b[1] |= 0x80
	}
	binary.BigEndian.PutUint16(b[2:4], h.Sequence)
	binary.BigEndian.PutUint32(b[4:8], h.Timestamp)
	binary.BigEndian.PutUint32(b[8:12], h.SSRC)
	return b
}

// ParseHeader splits a datagram into the fixed header and the payload.
// It requires version 2, CC 0 and no extension/padding.
func ParseHeader(pkt []byte) (Header, []byte, bool) {
	h := Header{}
	if len(pkt) < MinRTPPktLen {
		return h, nil, false
	}
	b0 := pkt[0]
	if b0>>6 != 2 {
		return h, nil, false
	}
	cc := b0 & 0x0F
	if cc != 0 || b0&0x30 != 0 {
		return h, nil, false
	}
	h.Padding = pkt[0]&0x20 != 0
	h.Extension = pkt[0]&0x10 != 0
	h.CC = cc
	h.Marker = pkt[1]&0x80 != 0
	h.PayloadType = pkt[1] & 0x7F
	h.Sequence = binary.BigEndian.Uint16(pkt[2:4])
	h.Timestamp = binary.BigEndian.Uint32(pkt[4:8])
	h.SSRC = binary.BigEndian.Uint32(pkt[8:12])
	return h, pkt[MinRTPPktLen:], true
}

// The NTP epoch (1900-01-01) leads the Unix epoch (1970-01-01) by this many
// seconds (RFC 3550, appendix B).
const ntpEpochOffset = 2208988800

// NTPSplit converts an instant into NTP seconds and fractional seconds.
func NTPSplit(t time.Time) (uint32, uint32) {
	sec := t.Unix() + ntpEpochOffset
	frac := uint32(float64(t.Nanosecond()) / 1e9 * 4294967296.0)
	return uint32(sec), frac
}

// NTPNow returns the current NTP (secs, frac).
func NTPNow() (uint32, uint32) {
	return NTPSplit(time.Now())
}

// NTPMidpoint packs (secs, frac) into the 64-bit midpoint representation
// used by SR.timestamp.
func NTPMidpoint(secs, frac uint32) uint64 {
	return uint64(secs)<<32 | uint64(frac)
}
