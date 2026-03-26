package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// TestWriteReadRoundTrip checks that WriteFrame + ReadFrame recover the same payload.
func TestWriteReadRoundTrip(t *testing.T) {
	payload := []byte{1, 1} // version 1, kind PING per your spec

	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %v want %v", got, payload)
	}
}

// TestReadFrame_InvalidLength rejects L > maxPayload before allocating a huge buffer.
func TestReadFrame_InvalidLength(t *testing.T) {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(maxPayload+1))
	_, _ = buf.Write(lenBuf[:])

	_, err := ReadFrame(&buf)
	if !errors.Is(err, ErrInvalidPayloadLimit) {
		t.Fatalf("want ErrInvalidPayloadLimit, got %v", err)
	}
}

// TestReadFrame_TruncatedPayload: length says 100 bytes but stream ends early.
func TestReadFrame_TruncatedPayload(t *testing.T) {
	var buf bytes.Buffer
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], 100)
	_, _ = buf.Write(lenBuf[:])
	_, _ = buf.Write(bytes.Repeat([]byte{'x'}, 10)) // only 10 of 100

	_, err := ReadFrame(&buf)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("want ErrUnexpectedEOF, got %v", err)
	}
}

// TestWriteFrame_InvalidPayload checks WriteFrame rejects too-short payloads.
func TestWriteFrame_InvalidPayload(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{1} // len 1 < minPayload 2
	err := WriteFrame(&buf, payload)
	if !errors.Is(err, ErrInvalidPayloadLimit) {
		t.Fatalf("want ErrInvalidPayloadLimit, got %v", err)
	}
}

// oneByteReader returns at most one byte per Read, like a slow TCP stream.
type oneByteReader struct {
	r io.Reader
}

func (o *oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return o.r.Read(p[:1])
}

// TestReadFrame_SplitReads ensures ReadFrame works when Read returns 1 byte at a time.
func TestReadFrame_SplitReads(t *testing.T) {
	payload := []byte{1, 2}
	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	slow := &oneByteReader{r: bytes.NewReader(buf.Bytes())}
	got, err := ReadFrame(slow)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %v want %v", got, payload)
	}
}
