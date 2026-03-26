package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	kindPING  = 0x01
	kindPONG  = 0x02
	kindError = 0x03
)
const (
	maxPayload = 1_048_576
	minPayload = 2
	// fieldSizeLength = 4
)

var ErrInvalidPayloadLimit = errors.New("Invalid payload length")
var ErrInvalidIncomingPayload = errors.New("Invalid incoming payload")
var ErrInvalidVersion = errors.New("Invalid version")
var ErrPINGMustHaveNoBody = errors.New("PING must have no body")
var ErrInvalidPONGLength = errors.New("Invalid PONG length")
var ErrInvalidBody = errors.New("Invalid Body")
var ErrUnknownKind = errors.New("Unknown Kind")

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
	buf := make([]byte, 4)

	// Read the 4-byte length
	_, err := io.ReadFull(r, buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}

	// Decode
	L := binary.BigEndian.Uint32(buf[:])

	if L < minPayload || L > maxPayload {
		return nil, ErrInvalidPayloadLimit
	}

	// create a slice that can hold all the payload bytes
	payload := make([]byte, L)

	// read payload
	_, err = io.ReadFull(r, payload)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}

	return payload, nil
}

func ParsePayload(payload []byte) (version byte, kind byte, body []byte, err error) {

	if len(payload) < 2 {
		return 0, 0, nil, ErrInvalidIncomingPayload
	}

	version = payload[0]
	if version != 1 {
		return 0, 0, nil, ErrInvalidVersion
	}
	kind = payload[1]

	switch kind {
	case kindPING:
		if len(payload) != 2 {
			return 0, 0, nil, ErrPINGMustHaveNoBody
		}
	case kindPONG:
		if len(payload) != 2 {
			return 0, 0, nil, ErrInvalidPONGLength
		}
	case kindError:
		body = payload[2:]
		if len(body) < 1 || len(body) > 1024 {
			return 0, 0, nil, ErrInvalidBody

		}
	default:
		return 0, 0, nil, ErrUnknownKind

	}

	return version, kind, body, nil
}
