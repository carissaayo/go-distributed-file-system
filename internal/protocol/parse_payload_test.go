package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func TestParsePayload_PutEmptyBody(t *testing.T) {
	p := []byte{1, KindPut}
	v, k, body, err := ParsePayload(p)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 || k != KindPut {
		t.Fatalf("v=%d k=%#x", v, k)
	}
	if body != nil && len(body) != 0 {
		t.Fatalf("want empty body, got len %d", len(body))
	}
}

func TestParsePayload_PutWithContent(t *testing.T) {
	p := append([]byte{1, KindPut}, []byte("ab")...)
	v, k, body, err := ParsePayload(p)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 || k != KindPut || string(body) != "ab" {
		t.Fatalf("got kind=%#x body=%q", k, body)
	}
}

func TestParsePayload_GetValid(t *testing.T) {
	keyHex := bytes.Repeat([]byte{'a'}, 64) // valid hex
	p := append([]byte{1, KindGet}, keyHex...)
	v, k, body, err := ParsePayload(p)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 || k != KindGet || len(body) != 64 {
		t.Fatalf("v=%d k=%#x len(body)=%d", v, k, len(body))
	}
	if !bytes.Equal(body, keyHex) {
		t.Fatal("body mismatch")
	}
}

func TestParsePayload_GetWrongLength(t *testing.T) {
	p := append([]byte{1, KindGet}, bytes.Repeat([]byte{'a'}, 63)...)
	_, _, _, err := ParsePayload(p)
	if !errors.Is(err, ErrInvalidGetLength) {
		t.Fatalf("want ErrInvalidGetLength, got %v", err)
	}
}

func TestParsePayload_GetInvalidHex(t *testing.T) {
	suffix := append(bytes.Repeat([]byte{'a'}, 63), 'g')
	p := append([]byte{1, KindGet}, suffix...)
	_, _, _, err := ParsePayload(p)
	if !errors.Is(err, ErrInvalidHexCharacters) {
		t.Fatalf("want ErrInvalidHexCharacters, got %v", err)
	}
}

func TestParsePayload_StoredValid(t *testing.T) {
	keyHex := bytes.Repeat([]byte{'b'}, 64)
	p := append([]byte{1, KindStored}, keyHex...)
	_, k, body, err := ParsePayload(p)
	if err != nil {
		t.Fatal(err)
	}
	if k != KindStored || len(body) != 64 {
		t.Fatal(k, len(body))
	}
}

func TestParsePayload_DataEmpty(t *testing.T) {
	p := []byte{1, KindData}
	_, k, body, err := ParsePayload(p)
	if err != nil {
		t.Fatal(err)
	}
	if k != KindData || len(body) != 0 {
		t.Fatalf("k=%#x len=%d", k, len(body))
	}
}

func TestParsePayload_UnknownKind(t *testing.T) {
	p := []byte{1, 0xff}
	_, _, _, err := ParsePayload(p)
	if !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("want ErrUnknownKind, got %v", err)
	}
}
