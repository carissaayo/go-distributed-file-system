package transport

import (
	"bytes"
	"net"
	"testing"

	"github.com/carissaayo/go-tcp-scratch/internal/protocol"
	"github.com/carissaayo/go-tcp-scratch/internal/store"
)

// maxObjectBodyPerFrame is the max raw object bytes in one PUT/DATA(_CHUNK) frame (matches transport + client).
const maxObjectBodyPerFrame = protocol.MaxPayload - 2

// TestHandleConnPutGetRoundTrip runs handleConn over real TCP: PING/PONG, PUT→STORED, GET→DATA.
func TestHandleConnPutGetRoundTrip(t *testing.T) {
	root := t.TempDir()
	st := store.NewStore(root)
	tp := NewTransport("127.0.0.1:0", st)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		tp.handleConn(c)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPING}); err != nil {
		t.Fatal(err)
	}
	payload, err := protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, kind, _, err := protocol.ParsePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if kind != protocol.KindPONG {
		t.Fatalf("want PONG, got kind %#x", kind)
	}

	data := []byte("hello integration")
	put := append([]byte{1, protocol.KindPut}, data...)
	if err := protocol.WriteFrame(conn, put); err != nil {
		t.Fatal(err)
	}

	payload, err = protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, kind, body, err := protocol.ParsePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if kind != protocol.KindStored {
		t.Fatalf("want STORED, got kind %#x", kind)
	}
	if len(body) != 64 {
		t.Fatalf("stored key len %d want 64", len(body))
	}

	get := append([]byte{1, protocol.KindGet}, body...)
	if err := protocol.WriteFrame(conn, get); err != nil {
		t.Fatal(err)
	}

	payload, err = protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, kind, gotData, err := protocol.ParsePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if kind != protocol.KindData {
		t.Fatalf("want DATA, got kind %#x", kind)
	}
	if !bytes.Equal(gotData, data) {
		t.Fatalf("got %q want %q", gotData, data)
	}

	conn.Close()
	<-done
}

// readGetBody reads a full GET response: either one DATA frame or DATA_CHUNK* followed by DATA_END.
func readGetBody(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	var out bytes.Buffer
	for {
		payload, err := protocol.ReadFrame(conn)
		if err != nil {
			t.Fatal(err)
		}
		_, kind, body, err := protocol.ParsePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
		switch kind {
		case protocol.KindError:
			t.Fatalf("GET error: %s", string(body))
		case protocol.KindData:
			out.Write(body)
			return out.Bytes()
		case protocol.KindDataChunk:
			out.Write(body)
		case protocol.KindDataEnd:
			return out.Bytes()
		default:
			t.Fatalf("unexpected GET response kind %#x", kind)
		}
	}
}

// TestHandleConnStreamingPutGetRoundTrip exercises storage v2: PUT_STREAM_* upload and DATA_CHUNK+DATA_END download.
// Object size is chosen so one v1 PUT frame cannot hold it and the server must stream the GET response.
func TestHandleConnStreamingPutGetRoundTrip(t *testing.T) {
	// Body length > max single-frame PUT body ⇒ client would use streaming; server PutReader accepts the same stream.
	data := bytes.Repeat([]byte{'z'}, protocol.MaxPayload-1)

	root := t.TempDir()
	st := store.NewStore(root)
	tp := NewTransport("127.0.0.1:0", st)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		tp.handleConn(c)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPING}); err != nil {
		t.Fatal(err)
	}
	pong, err := protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, kind, _, err := protocol.ParsePayload(pong)
	if err != nil || kind != protocol.KindPONG {
		t.Fatalf("handshake: %v kind %#x", err, kind)
	}

	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamBegin}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(data); {
		end := i + maxObjectBodyPerFrame
		if end > len(data) {
			end = len(data)
		}
		chunk := append([]byte{1, protocol.KindPutStreamChunk}, data[i:end]...)
		if err := protocol.WriteFrame(conn, chunk); err != nil {
			t.Fatal(err)
		}
		i = end
	}
	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamEnd}); err != nil {
		t.Fatal(err)
	}

	storedFr, err := protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, skind, keyHex, err := protocol.ParsePayload(storedFr)
	if err != nil {
		t.Fatal(err)
	}
	if skind != protocol.KindStored {
		t.Fatalf("want STORED, got %#x", skind)
	}
	if len(keyHex) != 64 {
		t.Fatalf("key len %d", len(keyHex))
	}

	get := append([]byte{1, protocol.KindGet}, keyHex...)
	if err := protocol.WriteFrame(conn, get); err != nil {
		t.Fatal(err)
	}
	got := readGetBody(t, conn)
	if !bytes.Equal(got, data) {
		t.Fatalf("round-trip mismatch: got %d bytes want %d", len(got), len(data))
	}

	conn.Close()
	<-done
}

// readGetBodyAfterFirstDataChunk consumes frames after the first DATA_CHUNK (which is already read).
func readGetBodyAfterFirstDataChunk(t *testing.T, conn net.Conn, firstBody []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	out.Write(firstBody)
	for {
		payload, err := protocol.ReadFrame(conn)
		if err != nil {
			t.Fatal(err)
		}
		_, kind, body, err := protocol.ParsePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
		switch kind {
		case protocol.KindDataChunk:
			out.Write(body)
		case protocol.KindDataEnd:
			return out.Bytes()
		case protocol.KindError:
			t.Fatalf("GET error: %s", string(body))
		default:
			t.Fatalf("unexpected kind after DATA_CHUNK: %#x", kind)
		}
	}
}

// TestHandleConnGetNotFound returns ERROR when the object key does not exist.
func TestHandleConnGetNotFound(t *testing.T) {
	root := t.TempDir()
	st := store.NewStore(root)
	tp := NewTransport("127.0.0.1:0", st)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		tp.handleConn(c)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPING}); err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.ReadFrame(conn); err != nil {
		t.Fatal(err)
	}

	missingKey := []byte("0000000000000000000000000000000000000000000000000000000000000000")
	get := append([]byte{1, protocol.KindGet}, missingKey...)
	if err := protocol.WriteFrame(conn, get); err != nil {
		t.Fatal(err)
	}

	payload, err := protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, kind, body, err := protocol.ParsePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if kind != protocol.KindError {
		t.Fatalf("want ERROR, got kind %#x", kind)
	}
	if len(body) < 1 {
		t.Fatal("ERROR body empty")
	}

	conn.Close()
	<-done
}

// TestHandleConnLargeGetUsesDataChunk validates GetReader path: first response frame is DATA_CHUNK, not DATA.
func TestHandleConnLargeGetUsesDataChunk(t *testing.T) {
	data := bytes.Repeat([]byte{'z'}, protocol.MaxPayload-1)

	root := t.TempDir()
	st := store.NewStore(root)
	tp := NewTransport("127.0.0.1:0", st)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		tp.handleConn(c)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPING}); err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.ReadFrame(conn); err != nil {
		t.Fatal(err)
	}

	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamBegin}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(data); {
		end := i + maxObjectBodyPerFrame
		if end > len(data) {
			end = len(data)
		}
		chunk := append([]byte{1, protocol.KindPutStreamChunk}, data[i:end]...)
		if err := protocol.WriteFrame(conn, chunk); err != nil {
			t.Fatal(err)
		}
		i = end
	}
	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamEnd}); err != nil {
		t.Fatal(err)
	}

	storedFr, err := protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, skind, keyHex, err := protocol.ParsePayload(storedFr)
	if err != nil || skind != protocol.KindStored {
		t.Fatalf("STORED: %v kind %#x", err, skind)
	}

	get := append([]byte{1, protocol.KindGet}, keyHex...)
	if err := protocol.WriteFrame(conn, get); err != nil {
		t.Fatal(err)
	}

	first, err := protocol.ReadFrame(conn)
	if err != nil {
		t.Fatal(err)
	}
	_, k1, firstBody, err := protocol.ParsePayload(first)
	if err != nil {
		t.Fatal(err)
	}
	if k1 != protocol.KindDataChunk {
		t.Fatalf("large object GET: want first frame DATA_CHUNK, got %#x", k1)
	}

	got := readGetBodyAfterFirstDataChunk(t, conn, firstBody)
	if !bytes.Equal(got, data) {
		t.Fatalf("body mismatch: got %d bytes want %d", len(got), len(data))
	}

	conn.Close()
	<-done
}
