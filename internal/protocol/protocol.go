package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
)

const (
	KindPING           = 0x01
	KindPONG           = 0x02
	KindError          = 0x03
	KindPut            = 0x10
	KindGet            = 0x11
	KindStored         = 0x12
	KindData           = 0x13
	KindPutStreamBegin = 0x14
	KindPutStreamChunk = 0x15
	KindPutStreamEnd   = 0x16
	KindDataChunk      = 0x17
	KindDataEnd        = 0x18
)
const (
	maxPayload = 1_048_576
	minPayload = 2
)

var ErrInvalidPayloadLimit = errors.New("Invalid payload length")
var ErrInvalidIncomingPayload = errors.New("Invalid incoming payload")
var ErrInvalidVersion = errors.New("Invalid version")
var ErrPINGMustHaveNoBody = errors.New("PING must have no body")
var ErrInvalidPONGLength = errors.New("Invalid PONG length")
var ErrInvalidBody = errors.New("Invalid Body")
var ErrUnknownKind = errors.New("Unknown Kind")
var ErrInvalidGetLength = errors.New("Invalid Get Length")
var ErrInvalidHexCharacters = errors.New("Invalid Hex Characters")
var ErrInvalidStoredLength = errors.New("Invalid Stored Length")
var ErrInvalidDataLength = errors.New("Invalid Data Length")
var ErrStreamBeginMustHaveNoBody = errors.New("Stream Begin  must have no body")
var ErrStreamEndMustHaveNoBody = errors.New("Stream End  must have no body")

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
	case KindPING:
		if len(payload) != 2 {
			return 0, 0, nil, ErrPINGMustHaveNoBody
		}
	case KindPONG:
		if len(payload) != 2 {
			return 0, 0, nil, ErrInvalidPONGLength
		}
	case KindError:
		body = payload[2:]
		if len(body) < 1 || len(body) > 1024 {
			return 0, 0, nil, ErrInvalidBody

		}
	case KindPut:
		body = payload[2:]

	case KindData:
		body = payload[2:]

	case KindGet:
		if len(payload) != 66 {
			return 0, 0, nil, ErrInvalidGetLength
		}

		if !validKeyHexSuffix(payload[2:]) {
			return 0, 0, nil, ErrInvalidHexCharacters
		}
		body = payload[2:]

	case KindStored:
		if len(payload) != 66 {
			return 0, 0, nil, ErrInvalidStoredLength
		}
		if !validKeyHexSuffix(payload[2:]) {
			return 0, 0, nil, ErrInvalidHexCharacters
		}

		body = payload[2:]

	case KindPutStreamBegin:
		if len(payload) != 2 {
			return 0, 0, nil, ErrStreamBeginMustHaveNoBody
		}

	case KindPutStreamChunk:
		if len(payload) > 2 {
			body = payload[2:]

		}

	case KindPutStreamEnd:
		if len(payload) != 2 {
			return 0, 0, nil, ErrStreamEndMustHaveNoBody
		}

	case KindDataChunk:
		if len(payload) > 2 {
			body = payload[2:]

		}

	default:
		return 0, 0, nil, ErrUnknownKind

	}

	return version, kind, body, nil
}

func validKeyHexSuffix(b []byte) bool {
	if len(b) != 64 {
		return false
	}
	_, err := hex.DecodeString(string(b))
	return err == nil
}
