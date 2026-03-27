package main

import (
	"log"
	"net"
	"os"

	"github.com/carissaayo/go-tcp-scratch/internal/protocol"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:3000")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err = protocol.WriteFrame(conn, []byte{1, protocol.KindPING}); err != nil {
		log.Fatal(err)
	}

	payload, err := protocol.ReadFrame(conn)
	if err != nil {
		log.Fatal(err)
	}

	_, kind, _, err := protocol.ParsePayload(payload)
	if err != nil {
		log.Fatal(err)
	}

	if kind != protocol.KindPONG {
		log.Println("invalid kind")
		os.Exit(1)
	}

	log.Println("ok: received PONG")

}
