package store

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestPutGetRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	data := []byte("hello world")

	key, err := s.Put(data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if len(key) != 64 {
		t.Fatalf("key length: got %d want 64", len(key))
	}

	got, err := s.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %q want %q", got, data)
	}
}

func TestGetNotFound(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	missing := "0000000000000000000000000000000000000000000000000000000000000000"

	_, err := s.Get(missing)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want ErrNotExist, got %v", err)
	}
}

func TestPutGetEmpty(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	key, err := s.Put(nil)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := s.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty blob, got len %d", len(got))
	}
}
