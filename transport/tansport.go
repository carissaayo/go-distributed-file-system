package transport

import (
	"fmt"
	"net"

	"github.com/carissaayo/go-tcp-scratch/internal/protocol"
)

type Transport struct {
	listenAddr string
	Listener   net.Listener
}

func NewTransport(listenAddr string) *Transport {
	return &Transport{
		listenAddr: listenAddr,
	}
}

func (tp *Transport) Listen() error {
	var err error

	tp.Listener, err = net.Listen("tcp", tp.listenAddr)

	if err != nil {
		return err
	}

	go tp.Accept()

	return nil

}

func (tp *Transport) Accept() {
	for {
		conn, err := tp.Listener.Accept()

		if err != nil {
			fmt.Printf("TCP accept error: %s\n", err)
			continue
		}

		fmt.Printf("new incoming connection %+v\n", conn)
		go tp.handleConn(conn)
	}
}

func (tp *Transport) handleConn(conn net.Conn) {
	defer conn.Close()
	pongbuf := []byte{1, protocol.KindPONG}

	for {
		payload, err := protocol.ReadFrame(conn)
		if err != nil {
			fmt.Printf("TCP error: %s\n", err)
			break
		}

		_, kind, _, err := protocol.ParsePayload(payload)
		if err != nil {
			fmt.Printf("Error parsing the payload: %s\n", err)
			return

		}

		if kind == protocol.KindPING {
			err := protocol.WriteFrame(conn, pongbuf)
			if err != nil {
				fmt.Printf("Error writing the payload: %s\n", err)

			}
		}

	}
}
