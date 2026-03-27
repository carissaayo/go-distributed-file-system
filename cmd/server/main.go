package main

import (
	"log"

	"github.com/carissaayo/go-tcp-scratch/transport"
)

var listenAddr string

func main() {
	listenAddr = ":3000"
	tp := transport.NewTransport(listenAddr)

	if err := tp.Listen(); err != nil {
		log.Fatal(err)
	}

	select {}

}
