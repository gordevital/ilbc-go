package ilbc

import "errors"

var (
	ErrInvalidFrameSize = errors.New("ilbc: invalid frame size")
	ErrInvalidPayload   = errors.New("ilbc: invalid payload length")
	ErrIncompleteTables = errors.New("ilbc: constant tables incomplete, output is not bit-exact")
	ErrMode             = errors.New("ilbc: unsupported mode (use Mode20ms or Mode30ms)")
)
