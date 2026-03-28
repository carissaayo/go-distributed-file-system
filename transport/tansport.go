package transport

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/carissaayo/go-tcp-scratch/internal/protocol"
	"github.com/carissaayo/go-tcp-scratch/internal/store"
)

type Transport struct {
	listenAddr string
	Listener   net.Listener
	store      *store.Store
}

func NewTransport(listenAddr string, store *store.Store) *Transport {
	return &Transport{
		listenAddr: listenAddr,
		store:      store,
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
	errorBuf := []byte{1, protocol.KindError}

	for {
		payload, err := protocol.ReadFrame(conn)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			fmt.Printf("TCP error: %s\n", err)
			break
		}

		_, kind, body, err := protocol.ParsePayload(payload)
		if err != nil {
			fmt.Printf("Error parsing the payload: %s\n", err)
			return

		}

		if kind == protocol.KindPING {
			err := protocol.WriteFrame(conn, pongbuf)
			if err != nil {
				fmt.Printf("Error writing the payload: %s\n", err)
				return
			}
			continue
		}

		if kind == protocol.KindPut {
			keyHex, err := tp.store.Put(body)
			if err != nil {
				fmt.Printf("Error storing the body: %s\n", err)
				return
			}
			storedBuf := []byte{1, protocol.KindStored}

			data := append(storedBuf, []byte(keyHex)...)

			err = protocol.WriteFrame(conn, data)
			if err != nil {
				fmt.Printf("Error writing the payload: %s\n", err)
				return
			}

		}

		if kind == protocol.KindGet {
			data, err := tp.store.Get(string(body))

			if err != nil {
				errPayload := append(errorBuf, []byte("data not found")...)
				err = protocol.WriteFrame(conn, errPayload)
				return
			}

			buf := append([]byte{1, protocol.KindData}, data...)
			if err = protocol.WriteFrame(conn, buf); err != nil {
				fmt.Printf("Error writing the get data")
				return
			}

		}
	}

}
