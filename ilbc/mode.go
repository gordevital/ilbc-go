package ilbc

// Mode selects the iLBC frame length.
type Mode int

const (
	// Mode20ms : BLOCKL=160 samples, 304 bits (38 bytes) per frame,
	// 15.2 kbit/s.
	Mode20ms Mode = 20

	// Mode30ms : BLOCKL=240 samples, 400 bits (50 bytes) per frame,
	// 13.33 kbit/s.
	Mode30ms Mode = 30
)

func (m Mode) valid() bool { return m == Mode20ms || m == Mode30ms }

// BlockLen returns the number of PCM samples per frame.
func (m Mode) BlockLen() int {
	switch m {
	case Mode20ms:
		return 160
	case Mode30ms:
		return 240
	default:
		return 0
	}
}

// PayloadLen returns the number of encoded bytes per frame.
func (m Mode) PayloadLen() int {
	switch m {
	case Mode20ms:
		return 38 // 304 bits
	case Mode30ms:
		return 50 // 400 bits
	default:
		return 0
	}
}
