package transport

import (
	"fmt"
	"net"
)

type Transport struct {
	remoteddr string
	Listener  net.Listener
}

func (tp *Transport) Listen() error {
	var err error

	tp.Listener, err = net.Listen("tcp", tp.remoteddr)

	if err != nil {
		fmt.Printf("conn error: %s\n", err)
		return err
	}
	fmt.Printf("conn gotten")

	return nil

}
