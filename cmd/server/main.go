package main

import (
	"flag"
	"log"

	"github.com/carissaayo/go-distributed-file-system/internal/store"
	"github.com/carissaayo/go-distributed-file-system/transport"
)

func main() {
	log.SetPrefix("server: ")
	log.SetFlags(log.LstdFlags)

	listenAddr := flag.String("addr", ":3000", "TCP listen address (e.g. :3000 or 127.0.0.1:3000)")
	dataDir := flag.String("data", "./data", "directory for stored blob files")
	flag.Parse()

	st := store.NewStore(*dataDir)
	tp := transport.NewTransport(*listenAddr, st)

	if err := tp.Listen(); err != nil {
		log.Fatalf("listen on %s: %v", *listenAddr, err)
	}

	log.Printf("listening on %s, data root %s", *listenAddr, *dataDir)

	select {}
}
