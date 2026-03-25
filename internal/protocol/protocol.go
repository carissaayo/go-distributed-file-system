package protocol

import "io"

const (
	maxPayload      = 1_048_575
	minPayload      = 2
	fieldSizeLength = 1_048_576
)

func WriteFrame(w io.Writer, payload []byte) error {

}

func ReadFrame(r io.Reader) ([]byte, error) {

}
