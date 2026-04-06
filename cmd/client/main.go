package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/carissaayo/go-distributed-file-system/internal/protocol"
)

// maxBodyPerFrame is max object bytes per PUT/DATA(_CHUNK) frame body.
const maxBodyPerFrame = protocol.MaxPayload - 2

func main() {
	log.SetPrefix("client: ")
	log.SetFlags(0)

	addr := flag.String("addr", "localhost:3000", "server TCP address (host:port)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "ping":
		runPing(*addr)
	case "put":
		fs := flag.NewFlagSet("put", flag.ExitOnError)
		file := fs.String("file", "", "path to file to upload (required)")
		fs.Usage = func() {
			fmt.Fprintln(fs.Output(), "usage: client put -file <path>")
			fs.PrintDefaults()
		}
		_ = fs.Parse(args[1:])
		if *file == "" {
			log.Fatal("put: -file is required (path to the file to upload)")
		}
		runPut(*addr, *file)
	case "get":
		fs := flag.NewFlagSet("get", flag.ExitOnError)
		key := fs.String("key", "", "64-character hex SHA-256 content key (required)")
		outPath := fs.String("out", "", "write downloaded object to this file (default: stdout)")
		fs.Usage = func() {
			fmt.Fprintln(fs.Output(), "usage: client get -key <64-hex> [-out <path>]")
			fs.PrintDefaults()
		}
		_ = fs.Parse(args[1:])
		if *key == "" {
			log.Fatal("get: -key is required (64 hex characters, the object hash from put)")
		}
		runGet(*addr, *key, *outPath)
	default:
		log.Fatalf("unknown command %q (use ping, put, or get)", args[0])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: client [-addr host:port] <command> [options]

Content-addressed file storage client. Server must be running (see README).

Commands:
  ping              Send PING, expect PONG (health check)
  put  -file <path> Upload a file; prints stored: <64-hex-key>
  get  -key <hex>   Download object by key (stdout, or -out file)

Examples:
  client ping
  client put -file ./data.bin
  client get -key e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
  client get -key <key> -out ./restored.bin

`)
	flag.PrintDefaults()
}

func dial(addr string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("connect to %s: %v", addr, err)
	}
	return conn
}

func handshake(conn net.Conn) error {
	if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPING}); err != nil {
		return fmt.Errorf("send PING: %w", err)
	}
	payload, err := protocol.ReadFrame(conn)
	if err != nil {
		return fmt.Errorf("read handshake response: %w", err)
	}
	_, kind, _, err := protocol.ParsePayload(payload)
	if err != nil {
		return fmt.Errorf("parse handshake: %w", err)
	}
	if kind != protocol.KindPONG {
		return fmt.Errorf("expected PONG from server, got message kind 0x%02x", kind)
	}
	return nil
}

func runPing(addr string) {
	conn := dial(addr)
	defer conn.Close()

	if err := handshake(conn); err != nil {
		log.Fatal(err)
	}
	log.Println("ok: server replied with PONG")
}

func runPut(addr, file string) {
	fi, err := os.Stat(file)
	if err != nil {
		log.Fatalf("stat %q: %v", file, err)
	}

	conn := dial(addr)
	defer conn.Close()

	if err := handshake(conn); err != nil {
		log.Fatal(err)
	}

	if fi.Size()+2 <= protocol.MaxPayload {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("read %q: %v", file, err)
		}
		putPayload := append([]byte{1, protocol.KindPut}, data...)
		if err := protocol.WriteFrame(conn, putPayload); err != nil {
			log.Fatalf("send PUT: %v", err)
		}
	} else {
		f, err := os.Open(file)
		if err != nil {
			log.Fatalf("open %q: %v", file, err)
		}
		defer f.Close()

		if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamBegin}); err != nil {
			log.Fatalf("send PUT_STREAM_BEGIN: %v", err)
		}
		buf := make([]byte, maxBodyPerFrame)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				chunk := append([]byte{1, protocol.KindPutStreamChunk}, buf[:n]...)
				if err := protocol.WriteFrame(conn, chunk); err != nil {
					log.Fatalf("send PUT_STREAM_CHUNK: %v", err)
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("read file: %v", err)
			}
		}
		if err := protocol.WriteFrame(conn, []byte{1, protocol.KindPutStreamEnd}); err != nil {
			log.Fatalf("send PUT_STREAM_END: %v", err)
		}
	}

	resp, err := protocol.ReadFrame(conn)
	if err != nil {
		log.Fatalf("read server response: %v", err)
	}
	_, kind, body, err := protocol.ParsePayload(resp)
	if err != nil {
		log.Fatalf("parse response: %v", err)
	}
	if kind == protocol.KindError {
		log.Fatalf("server error: %s", string(body))
	}
	if kind != protocol.KindStored {
		log.Fatalf("expected STORED from server, got kind 0x%02x", kind)
	}
	log.Printf("stored: %s", string(body))
}

func runGet(addr, keyHex, outPath string) {
	keyHex = strings.ToLower(strings.TrimSpace(keyHex))
	if len(keyHex) != 64 {
		log.Fatal("get: -key must be exactly 64 hexadecimal characters (SHA-256 hex)")
	}
	for _, c := range keyHex {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		log.Fatal("get: -key must contain only 0-9 and a-f")
	}

	out := io.Writer(os.Stdout)
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			log.Fatalf("create output %q: %v", outPath, err)
		}
		defer f.Close()
		out = f
	}

	conn := dial(addr)
	defer conn.Close()

	if err := handshake(conn); err != nil {
		log.Fatal(err)
	}

	getPayload := append([]byte{1, protocol.KindGet}, []byte(keyHex)...)
	if err := protocol.WriteFrame(conn, getPayload); err != nil {
		log.Fatalf("send GET: %v", err)
	}

	for {
		resp, err := protocol.ReadFrame(conn)
		if err != nil {
			log.Fatalf("read response: %v", err)
		}
		_, kind, body, err := protocol.ParsePayload(resp)
		if err != nil {
			log.Fatalf("parse response: %v", err)
		}
		switch kind {
		case protocol.KindError:
			log.Fatalf("server error: %s", string(body))
		case protocol.KindData:
			if _, err := out.Write(body); err != nil {
				log.Fatalf("write output: %v", err)
			}
			return
		case protocol.KindDataChunk:
			if _, err := out.Write(body); err != nil {
				log.Fatalf("write output: %v", err)
			}
		case protocol.KindDataEnd:
			return
		default:
			log.Fatalf("unexpected response from server: kind 0x%02x", kind)
		}
	}
}
