package transport

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/carissaayo/go-tcp-scratch/internal/protocol"
	"github.com/carissaayo/go-tcp-scratch/internal/store"
)

// maxBodyPerFrame is the max object bytes per DATA / DATA_CHUNK frame (L = 2 + body ≤ MaxPayload).
const maxBodyPerFrame = protocol.MaxPayload - 2

type putResult struct {
	keyHex string
	err    error
}

type uploadSession struct {
	pw   *io.PipeWriter
	done chan putResult
}
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
	var upload *uploadSession

	writeError := func(msg string) bool {
		if msg == "" {
			msg = "error"
		}
		if len(msg) > 1024 {
			msg = msg[:1024]
		}
		p := append(append([]byte{}, errorBuf...), []byte(msg)...)
		if err := protocol.WriteFrame(conn, p); err != nil {
			fmt.Printf("Error writing ERROR frame: %s\n", err)
			return false
		}
		return true
	}

	abortUpload := func() {
		if upload == nil {
			return
		}
		_ = upload.pw.CloseWithError(errors.New("upload aborted"))
		<-upload.done
		upload = nil
	}

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

		if upload != nil {

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
				if werr := protocol.WriteFrame(conn, errPayload); werr != nil {
					fmt.Printf("Error writing ERROR frame: %s\n", werr)
				}
				return
			}

			buf := append([]byte{1, protocol.KindData}, data...)
			if err = protocol.WriteFrame(conn, buf); err != nil {
				fmt.Printf("Error writing DATA frame: %s\n", err)
				return
			}

		}

	}

}
