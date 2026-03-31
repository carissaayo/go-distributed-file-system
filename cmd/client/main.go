package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/carissaayo/go-tcp-scratch/internal/protocol"
)

// maxBodyPerFrame is max object bytes per PUT/DATA(_CHUNK) frame body.
const maxBodyPerFrame = protocol.MaxPayload - 2

// Matches internal/protocol max frame payload (version+kind+body).
const maxFramePayload = 1_048_576

func main() {
	addr := flag.String("addr", "localhost:3000", "server TCP address (host:port)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("usage: client [-addr host:port] <ping|put|get> [flags]\n" +
			"  client ping\n" +
			"  client put -file <path>\n" +
			"  client get -key <64-char-hex>")
	}

	switch args[0] {
	case "ping":
		runPing(*addr)
	case "put":
		fs := flag.NewFlagSet("put", flag.ExitOnError)
		file := fs.String("file", "", "file to upload")
		_ = fs.Parse(args[1:])
		if *file == "" {
			log.Fatal("put: -file is required")
		}
		runPut(*addr, *file)
	case "get":
		fs := flag.NewFlagSet("get", flag.ExitOnError)
		key := fs.String("key", "", "64-character hex SHA-256 key")
		_ = fs.Parse(args[1:])
		if *key == "" {
			log.Fatal("get: -key is required")
		}
		runGet(*addr, *key)
	default:
		log.Fatalf("unknown command %q", args[0])
	}
}

func dial(addr string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	return conn
}

func handshake(conn net.Conn) error {
	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPING}); err != nil {
		return err
	}
	payload, err := protocol.ReadFrame(conn)
	if err != nil {
		return err
	}
	_, kind, _, err := protocol.ParsePayload(payload)
	if err != nil {
		return err
	}
	if kind != protocol.KindPONG {
		return fmt.Errorf("expected PONG, got kind 0x%02x", kind)
	}
	return nil
}

func runPing(addr string) {
	conn := dial(addr)
	defer conn.Close()

	if err := handshake(conn); err != nil {
		log.Fatal(err)
	}
	log.Println("ok: received PONG")
}

func runPut(addr, file string) {
	fi, err := os.Stat(file)
	if err != nil {
		log.Fatal(err)
	}

	conn := dial(addr)
	defer conn.Close()

	if err := handshake(conn); err != nil {
		log.Fatal(err)
	}

	// Single-frame PUT (storage v1) when the whole object fits in one frame.
	if fi.Size()+2 <= protocol.MaxPayload {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		putPayload := append([]byte{1, protocol.KindPut}, data...)
		if err := protocol.WriteFrame(conn, putPayload); err != nil {
			log.Fatal(err)
		}
	} else {
		f, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamBegin}); err != nil {
			log.Fatal(err)
		}
		buf := make([]byte, maxBodyPerFrame)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				chunk := append([]byte{1, protocol.KindPutStreamChunk}, buf[:n]...)
				if err := protocol.WriteFrame(conn, chunk); err != nil {
					log.Fatal(err)
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
		}
		if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamEnd}); err != nil {
			log.Fatal(err)
		}
	}

	resp, err := protocol.ReadFrame(conn)
	if err != nil {
		log.Fatal(err)
	}
	_, kind, body, err := protocol.ParsePayload(resp)
	if err != nil {
		log.Fatal(err)
	}
	if kind == protocol.KindError {
		log.Fatalf("server error: %s", string(body))
	}
	if kind != protocol.KindStored {
		log.Fatalf("expected STORED, got kind 0x%02x", kind)
	}
	log.Printf("stored: %s", string(body))
}

func runGet(addr, keyHex string) {
	keyHex = strings.ToLower(strings.TrimSpace(keyHex))
	if len(keyHex) != 64 {
		log.Fatal("get: -key must be exactly 64 hex characters")
	}
	for _, c := range keyHex {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		log.Fatal("get: -key must be hexadecimal")
	}

	conn := dial(addr)
	defer conn.Close()

	if err := handshake(conn); err != nil {
		log.Fatal(err)
	}

	getPayload := append([]byte{1, protocol.KindGet}, []byte(keyHex)...)
	if err := protocol.WriteFrame(conn, getPayload); err != nil {
		log.Fatal(err)
	}

	resp, err := protocol.ReadFrame(conn)
	if err != nil {
		log.Fatal(err)
	}
	_, kind, body, err := protocol.ParsePayload(resp)
	if err != nil {
		log.Fatal(err)
	}
	if kind == protocol.KindError {
		log.Fatalf("server error: %s", string(body))
	}
	if kind != protocol.KindData {
		log.Fatalf("expected DATA, got kind 0x%02x", kind)
	}
	if _, err := os.Stdout.Write(body); err != nil {
		log.Fatal(err)
	}
}
