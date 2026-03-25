package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
)

type Transport struct {
	listenAddr string
	Listener   net.Listener
}

func (tp *Transport) Listen() error {
	var err error

	tp.Listener, err = net.Listen("tcp", tp.listenAddr)

	if err != nil {
		return err
	}

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
	buf := make([]byte, 2000)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				fmt.Printf("TCP error: %s\n", err)
				break

			}
		}

		fmt.Printf("message: %+v\n", buf[:n])

	}
}
