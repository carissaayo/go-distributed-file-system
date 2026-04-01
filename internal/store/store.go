package store

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{
		root: root,
	}
}

// keyHex is 64 lowercase hex chars (no "0x" prefix).
func objectPath(root, keyHex string) string {
	a, b := keyHex[0:2], keyHex[2:4]
	name := keyHex
	return filepath.Join(root, a, b, name)
}

func (s *Store) GetReader(keyHex string) (getReader io.ReadCloser, err error) {
	path := objectPath(s.root, keyHex)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *Store) PutReader(r io.Reader) (keyHex string, err error) {
	buf := make([]byte, 32*1024)
	staging := filepath.Join(s.root, ".tmp")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(staging, "blob-*")
	if err != nil {
		return "", err
	}
	tmpPath := f.Name()

	h := sha256.New()
	mw := io.MultiWriter(f, h)
	if _, err = io.CopyBuffer(mw, r, buf); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	keyHex = hex.EncodeToString(h.Sum(nil))
	path := objectPath(s.root, keyHex)

	if _, err := os.Stat(path); err == nil {
		os.Remove(tmpPath)
		return keyHex, nil
	} else if !os.IsNotExist(err) {
		os.Remove(tmpPath)
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return keyHex, nil
}

// Put writes data to the path determined by its hash; returns the content key (hex).
func (s *Store) Put(data []byte) (keyHex string, err error) {
	sum := sha256.Sum256(data)          // [32]byte
	keyHex = hex.EncodeToString(sum[:]) // 64 lowercase hex chars

	path := objectPath(s.root, keyHex)

	if _, err := os.Stat(path); err == nil {
		return keyHex, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	// Atomic write: temp in same directory, then rename
	tmp, err := os.CreateTemp(filepath.Dir(path), "blob-*")
	if err != nil {
		return "", err
	}

	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	// Windows: Remove target first if replacing; for new key usually no file exists
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return keyHex, nil
}

// Get returns the full object bytes; it is implemented via GetReader and io.ReadAll.
func (s *Store) Get(keyHex string) ([]byte, error) {
	r, err := s.GetReader(keyHex)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
