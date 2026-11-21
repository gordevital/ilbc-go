// Package srtp implements the SRTP/SRTCP AEAD AES-GCM cipher profiles
// (AES-128-GCM and AES-256-GCM, 16-octet authentication tag, no MKI) as
// realized by libsrtp 2.5.0 (OpenSSL build, legacy CTR/ICM KDF).
//
// Key schedule: the master key/salt are NOT used verbatim. Like classic
// SRTP, the actual per-use keys are derived with the SRTP KDF (AES-ICM
// keystream: block = master salt || 0x00000000 as a big-endian u128, with
// the label XOR-ed into octet 7 and a consecutive +1 per output block;
// a 256-bit key chains two blocks):
//
//	RTP  cipher key = KDF(label 0x00), RTP  GCM salt = KDF(label 0x02)[:12]
//	RTCP cipher key = KDF(label 0x03), RTCP GCM salt = KDF(label 0x05)[:12]
//
// Construction parameters (pinned against vectors generated from a local
// build of libsrtp 2.5.0; see testdata/vectors_gcm.txt and the generator
// notes in srtp_test.go):
//
//	SRTP profile (RTP packets):
//	  GCM key   = derived RTP cipher key (16 or 32 octets)
//	  GCM nonce = first 12 octets of
//	            [0x0000 | SSRC(4,BE) | ROC(4,BE) | SEQ(2,BE) | 0x0000]
//	            XOR [RTP salt(12) | 0x0000]
//	  AAD       = the 12-octet RTP header
//	  layout    = header(12) || ciphertext(payload) || tag(16)
//
//	SRTCP profile (RTCP compound packets):
//	  cleartext prefix = first 8 octets (v/p/pt/len + ssrc)
//	  AAD          = prefix(8) || trailer(4)
//	  GCM nonce    = first 12 octets of
//	              [0x0000 | SSRC(4,BE) | 0x0000 | IDX(4,BE) | 0x0000000000]
//	              XOR [RTCP salt(12) | 0x0000]
//	  layout     = prefix(8) || ciphertext(rest) || tag(16) || trailer(4)
//	  trailer    = BE32(0x80000000 | IDX); the most significant bit of the
//	              31-bit index must stay zero
//	  index      = per-SSRC monotonic counter, first packet carries index 1
package srtp

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

const (
	SaltLen     = 12
	TagLen      = 16
	rtpHdrLen   = 12
	rtcpHeadLen = 8
	rtcpTailLen = 4
	rtcpWinBits = 64
	trailerFlag = uint32(1) << 31
	trailerMask = uint32(1<<31) - 1
	seqHalfRing = int32(1) << 15
)

var (
	ErrPacketTooShort = errors.New("srtp: packet too short")
	ErrAuthFail       = errors.New("srtp: authentication failed")
	ErrBadKeySize     = errors.New("srtp: master key must be 16 or 32 octets")
	ErrBadSaltSize    = errors.New("srtp: master salt must be 12 octets")
	ErrBadRTCP        = errors.New("srtp: malformed SRTCP packet")
	ErrRTCPReplay     = errors.New("srtp: RTCP index rejected (replay or out of window)")
	ErrRTCPIndex      = errors.New("srtp: RTCP index has forbidden bit 31 set")
)

type streamState struct {
	txROC     uint32
	txSeq     uint16
	txSeqSeen bool

	rxROC     uint32
	rxSeq     uint16
	rxSeqSeen bool

	rtcpTxIdx uint32

	rtcpRxHighest uint32
	rtcpRxLowest  uint32
	rtcpRxMask    uint64
	rtcpRxSeen    bool
}

// Session protects and unprotects one direction of SRTP/SRTCP traffic.
// Multiple SSRCs may be interleaved on one session; per-SSRC counters are
// tracked independently. A session is safe for concurrent use.
type Session struct {
	mu       sync.Mutex
	rtpGcm   cipher.AEAD
	rtcpGcm  cipher.AEAD
	rtpSalt  [SaltLen]byte
	rtcpSalt [SaltLen]byte
	streams  map[uint32]*streamState
}

// srtpKDF evaluates the SRTP KDF (libsrtp legacy ICM construction):
// out_i = AES(masterKey, (masterSalt(12) || 0x00000000)_BE + i) with the
// label XOR-ed into octet 7 of the base block; enough blocks are chained
// to reach outLen (a 256-bit key spans two consecutive blocks).
func srtpKDF(masterKey []byte, salt [SaltLen]byte, label byte, outLen int) []byte {
	c, err := aes.NewCipher(masterKey)
	if err != nil {
		panic(err) // key size validated by the caller
	}
	base := make([]byte, 16)
	copy(base[:], salt[:])
	base[7] ^= label
	out := make([]byte, 0, ((outLen+15)/16)*16)
	buf := make([]byte, 16)
	for i := 0; len(out) < outLen; i++ {
		copy(buf, base)
		binary.BigEndian.PutUint32(buf[12:], binary.BigEndian.Uint32(buf[12:])+uint32(i))
		tmp := make([]byte, 16)
		c.Encrypt(tmp, buf)
		out = append(out, tmp...)
	}
	return out[:outLen]
}

// NewSession builds a GCM session from the master key (16 or 32 octets)
// and the 12-octet master salt. The per-profile cipher keys and salts are
// derived via the SRTP KDF.
func NewSession(masterKey, masterSalt []byte) (*Session, error) {
	if len(masterKey) != 16 && len(masterKey) != 32 {
		return nil, ErrBadKeySize
	}
	if len(masterSalt) != SaltLen {
		return nil, ErrBadSaltSize
	}
	ms := [SaltLen]byte(masterSalt)
	mk := masterKey

	rtpKey := srtpKDF(mk, ms, 0x00, len(mk))
	rtpBlock, err := aes.NewCipher(rtpKey)
	if err != nil {
		return nil, err
	}
	rtpGcm, err := cipher.NewGCMWithNonceSize(rtpBlock, 12)
	if err != nil {
		return nil, err
	}
	rtcpKey := srtpKDF(mk, ms, 0x03, len(mk))
	rtcpBlock, err := aes.NewCipher(rtcpKey)
	if err != nil {
		return nil, err
	}
	rtcpGcm, err := cipher.NewGCMWithNonceSize(rtcpBlock, 12)
	if err != nil {
		return nil, err
	}
	s := &Session{
		rtpGcm:  rtpGcm,
		rtcpGcm: rtcpGcm,
		streams: make(map[uint32]*streamState),
	}
	copy(s.rtpSalt[:], srtpKDF(mk, ms, 0x02, SaltLen))
	copy(s.rtcpSalt[:], srtpKDF(mk, ms, 0x05, SaltLen))
	return s, nil
}

func (s *Session) stream(ssrc uint32) *streamState {
	st, ok := s.streams[ssrc]
	if !ok {
		st = &streamState{}
		s.streams[ssrc] = st
	}
	return st
}

func nonceRTP(salt [SaltLen]byte, ssrc, roc uint32, seq uint16) []byte {
	var in [16]byte
	binary.BigEndian.PutUint32(in[2:6], ssrc)
	binary.BigEndian.PutUint32(in[6:10], roc)
	binary.BigEndian.PutUint16(in[10:12], seq)
	n := make([]byte, 12)
	for i := range n {
		n[i] = in[i] ^ salt[i]
	}
	return n
}

func nonceRTCP(salt [SaltLen]byte, ssrc, idx uint32) []byte {
	var in [16]byte
	binary.BigEndian.PutUint32(in[2:6], ssrc)
	binary.BigEndian.PutUint32(in[8:12], idx)
	n := make([]byte, 12)
	for i := range n {
		n[i] = in[i] ^ salt[i]
	}
	return n
}

// ProtectRTP takes a plaintext RTP packet (12-byte header + payload) and
// returns the protected packet: header || ciphertext || 16-byte tag.
// The input is not modified.
func (s *Session) ProtectRTP(pkt []byte) ([]byte, error) {
	if len(pkt) < rtpHdrLen+1 {
		return nil, ErrPacketTooShort
	}
	ssrc := binary.BigEndian.Uint32(pkt[8:12])
	seq := binary.BigEndian.Uint16(pkt[2:4])
	s.mu.Lock()
	st := s.stream(ssrc)
	if st.txSeqSeen {
		d := int32(seq) - int32(st.txSeq)
		if d < -seqHalfRing {
			st.txROC++ // forward wrap
		} else if d > seqHalfRing && st.txROC > 0 {
			st.txROC-- // backward correction
		}
	}
	st.txSeq, st.txSeqSeen = seq, true
	nonce := nonceRTP(s.rtpSalt, ssrc, st.txROC, seq)
	out := make([]byte, 0, len(pkt)+TagLen)
	out = append(out, pkt[:rtpHdrLen]...)
	out = s.rtpGcm.Seal(out, nonce, pkt[rtpHdrLen:], pkt[:rtpHdrLen])
	s.mu.Unlock()
	return out, nil
}

// UnprotectRTP verifies and removes the SRTP protection, returning the
// plaintext packet. An authentication failure yields ErrAuthFail and the
// packet must be dropped.
func (s *Session) UnprotectRTP(pkt []byte) ([]byte, error) {
	if len(pkt) < rtpHdrLen+TagLen {
		return nil, ErrPacketTooShort
	}
	ssrc := binary.BigEndian.Uint32(pkt[8:12])
	seq := binary.BigEndian.Uint16(pkt[2:4])
	s.mu.Lock()
	st := s.stream(ssrc)
	roc := st.rxROC
	if st.rxSeqSeen {
		d := int32(seq) - int32(st.rxSeq)
		switch {
		case d < -seqHalfRing:
			roc++
		case d > seqHalfRing && roc > 0:
			roc--
		}
	}
	plain, err := s.rtpGcm.Open(nil, nonceRTP(s.rtpSalt, ssrc, roc, seq),
		pkt[rtpHdrLen:], pkt[:rtpHdrLen])
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %v", ErrAuthFail, err)
	}
	st.rxSeq, st.rxSeqSeen, st.rxROC = seq, true, roc
	out := make([]byte, 0, len(pkt)-TagLen)
	out = append(out, pkt[:rtpHdrLen]...)
	out = append(out, plain...)
	s.mu.Unlock()
	return out, nil
}

// ProtectRTCP takes an unsecured RTCP compound packet (its first 8 octets
// are v/p/pt/len + sender SSRC) and returns the protected form:
// prefix(8) || ciphertext || tag(16) || trailer(4).
func (s *Session) ProtectRTCP(compound []byte) ([]byte, error) {
	if len(compound) < rtcpHeadLen+1 {
		return nil, ErrBadRTCP
	}
	ssrc := binary.BigEndian.Uint32(compound[4:8])
	s.mu.Lock()
	st := s.stream(ssrc)
	st.rtcpTxIdx++
	idx := st.rtcpTxIdx
	body := compound[rtcpHeadLen:]
	aad := make([]byte, 0, rtcpHeadLen+rtcpTailLen)
	aad = append(aad, compound[:rtcpHeadLen]...)
	aad = append(aad, srtcpTrailerBytes(idx)...)
	out := make([]byte, 0, len(compound)+TagLen+rtcpTailLen)
	out = append(out, compound[:rtcpHeadLen]...)
	out = s.rtcpGcm.Seal(out, nonceRTCP(s.rtcpSalt, ssrc, idx), body, aad)
	out = append(out, srtcpTrailerBytes(idx)...)
	s.mu.Unlock()
	return out, nil
}

func srtcpTrailerBytes(idx uint32) []byte {
	w := make([]byte, 4)
	binary.BigEndian.PutUint32(w, trailerFlag|idx)
	return w
}

func parseSRTCPTrailer(w []byte) (uint32, bool) {
	v := binary.BigEndian.Uint32(w)
	if v&trailerFlag == 0 {
		return 0, false
	}
	return v & trailerMask, true
}

// UnprotectRTCP verifies and removes SRTCP protection, returning the
// unsecured RTCP compound. Auth failures yield ErrAuthFail; stale or
// duplicated indices yield ErrRTCPReplay.
func (s *Session) UnprotectRTCP(compound []byte) ([]byte, error) {
	minLen := rtcpHeadLen + TagLen + rtcpTailLen
	if len(compound) < minLen {
		return nil, ErrBadRTCP
	}
	ssrc := binary.BigEndian.Uint32(compound[4:8])
	trailer := compound[len(compound)-rtcpTailLen:]
	idx, ok := parseSRTCPTrailer(trailer)
	if !ok {
		return nil, ErrBadRTCP
	}
	s.mu.Lock()
	st := s.stream(ssrc)
	aad := make([]byte, 0, rtcpHeadLen+rtcpTailLen)
	aad = append(aad, compound[:rtcpHeadLen]...)
	aad = append(aad, trailer...)
	plain, err := s.rtcpGcm.Open(nil, nonceRTCP(s.rtcpSalt, ssrc, idx),
		compound[rtcpHeadLen:len(compound)-rtcpTailLen], aad)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %v", ErrAuthFail, err)
	}
	if !s.acceptRTCPIdxLocked(st, idx) {
		s.mu.Unlock()
		return nil, ErrRTCPReplay
	}
	out := make([]byte, 0, len(compound)-TagLen-rtcpTailLen)
	out = append(out, compound[:rtcpHeadLen]...)
	out = append(out, plain...)
	s.mu.Unlock()
	return out, nil
}

func (s *Session) acceptRTCPIdxLocked(st *streamState, idx uint32) bool {
	if idx == 0 {
		return false
	}
	if !st.rtcpRxSeen {
		st.rtcpRxHighest = idx
		st.rtcpRxLowest = idx
		st.rtcpRxMask = 1
		st.rtcpRxSeen = true
		return true
	}
	if idx > st.rtcpRxHighest {
		dist := idx - st.rtcpRxHighest
		if dist >= rtcpWinBits {
			st.rtcpRxHighest, st.rtcpRxLowest, st.rtcpRxMask = idx, idx, 1
			return true
		}
		st.rtcpRxMask = st.rtcpRxMask<<dist | 1
		st.rtcpRxHighest = idx
		if cap := uint32(rtcpWinBits) - 1; idx > cap && st.rtcpRxLowest < cap {
			st.rtcpRxLowest = cap
		}
		return true
	}
	dist := st.rtcpRxHighest - idx
	if dist >= rtcpWinBits {
		return false
	}
	bit := uint64(1) << dist
	if st.rtcpRxMask&bit != 0 {
		return false // replay
	}
	st.rtcpRxMask |= bit
	if idx < st.rtcpRxLowest {
		st.rtcpRxLowest = idx
	}
	return true
}
