package transport

import (
	"bytes"
	"net"
	"testing"

	"github.com/carissaayo/go-tcp-scratch/internal/protocol"
	"github.com/carissaayo/go-tcp-scratch/internal/store"
)

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
