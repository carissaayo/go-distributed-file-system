package main

import (
	"log"

	"github.com/carissaayo/go-tcp-scratch/internal/store"
	"github.com/carissaayo/go-tcp-scratch/transport"
)

func main() {
	listenAddr := ":3000"
	dataDir := "./data"

	st := store.NewStore(dataDir)
	tp := transport.NewTransport(listenAddr, st)

	if err := tp.Listen(); err != nil {
		log.Fatal(err)
	}

	select {}

}
