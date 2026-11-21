package srtp

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gordevital/ilbc-go/internal/rtputil"
)

// Golden vectors were generated with tools/srtpvectors/vectorgen.c linked
// against libsrtp 2.5.0 (upstream commit a566a9c, CMake
// -DENABLE_OPENSSL=ON to enable the AEAD AES-GCM profiles); regenerate via
// tools/srtpvectors/build_vectors.sh.
//
// Inputs used by the generator:
//
//	A: mk = 00..0f, salt = f0..fb, ssrc = 0x11223344
//	   rtp1: seq 100, ts 1600, pt 98, payload "abcdefghijklmnopqrstuvwx"
//	   rtp2: seq 101, ts 1680, payload "ABCDEFGHIJKLMNOPQRSTUVWX"
//	   rtcp: 28-octet SR (see srWant below)
//	B: mk = 80..9f, salt = a0..ab, ssrc 0xdeadc0de (RTP context),
//	   0xfedcbafe (RTCP receiver context)
//	   rtp1: seq 65535 ts 200 payload "iLBC-payload-123456789"
//	   rtp2: seq     0 ts 360 payload "iLBC-payload-after-wrap"
//	   rtp3: seq     1 ts 520 payload "iLBC-payload-final-xyz!"
//	   rtcp: 32-octet RR (see rrWant below)
func loadGoldens(t *testing.T) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "vectors_gcm.txt"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	m := make(map[string][]byte)
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		b := make([]byte, len(f[1])/2)
		for i := range b {
			v, err := strconv.ParseUint(f[1][2*i:2*i+2], 16, 8)
			if err != nil {
				t.Fatalf("bad vector %s: %v", f[0], err)
			}
			b[i] = byte(v)
		}
		m[f[0]] = b
	}
	return m
}

func assertHexEq(t *testing.T, tag string, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("%s:\n got  %x\n want %x", tag, got, want)
	}
}

func mkRTP(t *testing.T, ssrc uint32, seq uint16, ts uint32, payload []byte) []byte {
	t.Helper()
	h := rtputil.Header{
		Version: 2, PayloadType: 98, Sequence: seq, Timestamp: ts, SSRC: ssrc,
	}
	out := h.Marshal()
	return append(out, payload...)
}

func key128() []byte {
	k := make([]byte, 16)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}
func key256() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(0x80 + i)
	}
	return k
}
func saltA() []byte {
	s := make([]byte, 12)
	for i := range s {
		s[i] = 0xf0 + byte(i)
	}
	return s
}
func saltB() []byte {
	s := make([]byte, 12)
	for i := range s {
		s[i] = 0xa0 + byte(i)
	}
	return s
}

var srWant = []byte{
	0x80, 0xc8, 0x00, 0x05, 0x11, 0x22, 0x33, 0x44,
	0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	0x00, 0x00, 0x06, 0x40, 0x00, 0x00, 0x00, 0x02,
	0x00, 0x00, 0x00, 0x38,
}

var rrWant = []byte{
	0x80, 0xc9, 0x00, 0x07, 0xfe, 0xdc, 0xba, 0xfe,
	0xde, 0xad, 0xc0, 0xde, 0x01,
	0x00, 0x00, 0x02,
	0x00, 0x00, 0x00, 0x02,
	0x00, 0x00, 0x00, 0x0a,
	0x12, 0x34, 0x56, 0x78,
	0x00, 0x00, 0x00, 0x03,
}

func TestGoldenProtectA(t *testing.T) {
	g := loadGoldens(t)
	sess, err := NewSession(key128(), saltA())
	if err != nil {
		t.Fatal(err)
	}
	got1, err := sess.ProtectRTP(mkRTP(t, 0x11223344, 100, 1600, []byte("abcdefghijklmnopqrstuvwx")))
	if err != nil {
		t.Fatal(err)
	}
	assertHexEq(t, "A_rtp1", got1, g["A_rtp1"])

	got2, err := sess.ProtectRTP(mkRTP(t, 0x11223344, 101, 1680, []byte("ABCDEFGHIJKLMNOPQRSTUVWX")))
	if err != nil {
		t.Fatal(err)
	}
	assertHexEq(t, "A_rtp2", got2, g["A_rtp2"])

	gotSR, err := sess.ProtectRTCP(srWant)
	if err != nil {
		t.Fatal(err)
	}
	assertHexEq(t, "A_rtcp", gotSR, g["A_rtcp"])
}

func TestGoldenProtectB(t *testing.T) {
	g := loadGoldens(t)
	sess, err := NewSession(key256(), saltB())
	if err != nil {
		t.Fatal(err)
	}
	got1, err := sess.ProtectRTP(mkRTP(t, 0xdeadc0de, 65535, 200, []byte("iLBC-payload-123456789")))
	if err != nil {
		t.Fatal(err)
	}
	assertHexEq(t, "B_rtp1_wrap_at_65535", got1, g["B_rtp1_wrap_at_65535"])

	got2, err := sess.ProtectRTP(mkRTP(t, 0xdeadc0de, 0, 360, []byte("iLBC-payload-after-wrap")))
	if err != nil {
		t.Fatal(err)
	}
	assertHexEq(t, "B_rtp2_after_wrap", got2, g["B_rtp2_after_wrap"])

	got3, err := sess.ProtectRTP(mkRTP(t, 0xdeadc0de, 1, 520, []byte("iLBC-payload-final-xyz!")))
	if err != nil {
		t.Fatal(err)
	}
	assertHexEq(t, "B_rtp3", got3, g["B_rtp3"])

	gotRR, err := sess.ProtectRTCP(rrWant)
	if err != nil {
		t.Fatal(err)
	}
	assertHexEq(t, "B_rtcp_rr", gotRR, g["B_rtcp_rr"])
}

func TestGoldenUnprotect(t *testing.T) {
	g := loadGoldens(t)
	rx, _ := NewSession(key128(), saltA())

	want1 := mkRTP(t, 0x11223344, 100, 1600, []byte("abcdefghijklmnopqrstuvwx"))
	got1, err := rx.UnprotectRTP(g["A_rtp1"])
	if err != nil {
		t.Fatalf("A_rtp1: %v", err)
	}
	if !bytes.Equal(got1, want1) {
		t.Errorf("A_rtp1 mismatch")
	}
	want2 := mkRTP(t, 0x11223344, 101, 1680, []byte("ABCDEFGHIJKLMNOPQRSTUVWX"))
	got2, err := rx.UnprotectRTP(g["A_rtp2"])
	if err != nil {
		t.Fatalf("A_rtp2: %v", err)
	}
	if !bytes.Equal(got2, want2) {
		t.Errorf("A_rtp2 mismatch")
	}

	// The golden SRTCP was sealed with index 1; a fresh receiver context
	// accepts it and must restore the SR verbatim.
	gotSR, err := rx.UnprotectRTCP(g["A_rtcp"])
	if err != nil {
		t.Fatalf("A_rtcp: %v", err)
	}
	if !bytes.Equal(gotSR, srWant) {
		t.Errorf("A_rtcp mismatch: got %x", gotSR)
	}

	// Fresh receiver for the AES-256 golden chain across the ROC wrap
	// (65535 -> 0 -> 1).
	rxB2, _ := NewSession(key256(), saltB())
	for _, tag := range []string{"B_rtp1_wrap_at_65535", "B_rtp2_after_wrap", "B_rtp3"} {
		if _, err := rxB2.UnprotectRTP(g[tag]); err != nil {
			t.Errorf("%s: golden unprotect: %v", tag, err)
		}
	}

	// Live wrap round-trip through fresh tx/rx sessions.
	txB, _ := NewSession(key256(), saltB())
	rxB, _ := NewSession(key256(), saltB())
	bPlain := []struct {
		seq  uint16
		ts   uint32
		body string
	}{
		{65535, 200, "iLBC-payload-123456789"},
		{0, 360, "iLBC-payload-after-wrap"},
		{1, 520, "iLBC-payload-final-xyz!"},
	}
	for i, b := range bPlain {
		p, err := txB.ProtectRTP(mkRTP(t, 0xdeadc0de, b.seq, b.ts, []byte(b.body)))
		if err != nil {
			t.Fatalf("B resend %d: %v", i, err)
		}
		rec, err := rxB.UnprotectRTP(p)
		if err != nil {
			t.Fatalf("B recv %d: %v", i, err)
		}
		if !bytes.Contains(rec, []byte(b.body)) {
			t.Errorf("B recv %d: payload lost", i)
		}
	}
}

func TestTamperRejected(t *testing.T) {
	g := loadGoldens(t)
	rx, _ := NewSession(key128(), saltA())
	bad := make([]byte, len(g["A_rtp1"]))
	copy(bad, g["A_rtp1"])
	bad[14] ^= 0x01 // flip a ciphertext byte
	err := func() (e error) { _, e = rx.UnprotectRTP(bad); return e }()
	if err == nil {
		t.Fatal("tampered RTP accepted")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("authentication")) {
		t.Errorf("unexpected error kind: %v", err)
	}

	rx2, _ := NewSession(key128(), saltA())
	badSR := make([]byte, len(g["A_rtcp"]))
	copy(badSR, g["A_rtcp"])
	badSR[10] ^= 0x01
	if _, err := rx2.UnprotectRTCP(badSR); err == nil {
		t.Error("tampered RTCP accepted")
	}
}

func TestRTCPReplay(t *testing.T) {
	g := loadGoldens(t)
	rx, _ := NewSession(key128(), saltA())
	if _, err := rx.UnprotectRTCP(g["A_rtcp"]); err != nil {
		t.Fatalf("first accept: %v", err)
	}
	if _, err := rx.UnprotectRTCP(g["A_rtcp"]); err != ErrRTCPReplay {
		t.Errorf("replay not rejected, got %v", err)
	}
}

func TestBadParams(t *testing.T) {
	if _, err := NewSession(make([]byte, 15), saltA()); err != ErrBadKeySize {
		t.Errorf("short key: %v", err)
	}
	if _, err := NewSession(key128(), saltA()[:11]); err != ErrBadSaltSize {
		t.Errorf("short salt: %v", err)
	}
	s, _ := NewSession(key128(), saltA())
	if _, err := s.ProtectRTP(make([]byte, 4)); err != ErrPacketTooShort {
		t.Errorf("short rtp: %v", err)
	}
	if _, err := s.ProtectRTCP(make([]byte, 4)); err != ErrBadRTCP {
		t.Errorf("short rtcp: %v", err)
	}
}
