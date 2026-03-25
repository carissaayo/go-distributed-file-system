package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	maxPayload      = 1_048_576
	minPayload      = 2
	fieldSizeLength = 4
)

var ErrInvalidPayloadLimit = errors.New("invalid payload length")

func WriteFrame(w io.Writer, payload []byte) error {
	n := len(payload)
	if n < minPayload || n > maxPayload {
		return ErrInvalidPayloadLimit
	}

	var hdr = make([]byte, 4)
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))

	p := hdr[:]

	// first loop: drain the 4-byte length
	for len(p) > 0 {
		written, err := w.Write(p)
		if err != nil {
			return err
		}
		p = p[written:]
	}

	p = payload

	// second loop: drain the payload
	for len(p) > 0 {
		written, err := w.Write(p)
		if err != nil {
			return err
		}
		p = p[written:]

	}

	return nil

}

func ReadFrame(r io.Reader) ([]byte, error) {

}
