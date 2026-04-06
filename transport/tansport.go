package transport

import (
	"errors"
	"io"
	"log"
	"net"
	"os"

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
			log.Printf("accept: %v", err)
			continue
		}

		log.Printf("connection from %s", conn.RemoteAddr())
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
			log.Printf("write ERROR frame: %v", err)
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
			log.Printf("read frame: %v", err)
			break
		}

		_, kind, body, err := protocol.ParsePayload(payload)
		if err != nil {
			log.Printf("parse payload: %v", err)
			return

		}

		if upload != nil {
			switch kind {
			case protocol.KindPutStreamChunk:
				if _, err := upload.pw.Write(body); err != nil {
					_ = upload.pw.CloseWithError(err)
					<-upload.done
					upload = nil
					if !writeError("chunk write failed") {
						return
					}
					continue
				}
			case protocol.KindPutStreamEnd:
				if err := upload.pw.Close(); err != nil {
					<-upload.done
					upload = nil
					if !writeError("end stream failed") {
						return
					}
					continue
				}

				res := <-upload.done
				upload = nil

				if res.err != nil {
					log.Printf("PutReader failed: %v", res.err)
					if !writeError("store failed") {
						return
					}
					continue
				}

				stored := append([]byte{1, protocol.KindStored}, []byte(res.keyHex)...)

				if err := protocol.WriteFrame(conn, stored); err != nil {
					log.Printf("write STORED: %v", err)
					return
				}

			default:
				abortUpload()
				if !writeError("unexpected frame during upload") {
					return
				}
			}
			continue
		}

		switch kind {
		case protocol.KindPING:
			err := protocol.WriteFrame(conn, pongbuf)
			if err != nil {
				log.Printf("write PONG: %v", err)
				return
			}
			continue

		case protocol.KindPut:
			keyHex, err := tp.store.Put(body)
			if err != nil {
				log.Printf("store Put: %v", err)
				return
			}
			storedBuf := []byte{1, protocol.KindStored}

			data := append(storedBuf, []byte(keyHex)...)

			err = protocol.WriteFrame(conn, data)
			if err != nil {
				log.Printf("write STORED: %v", err)
				return
			}

		case protocol.KindGet:
			r, err := tp.store.GetReader(string(body))
			if err != nil {
				if os.IsNotExist(err) {
					errPayload := append(errorBuf, []byte("data not found")...)
					if werr := protocol.WriteFrame(conn, errPayload); werr != nil {
						log.Printf("write ERROR (not found): %v", werr)
					}
				} else {
					log.Printf("open object: %v", err)
					if !writeError("internal error") {
						return
					}
				}
				return
			}

			fi, err := r.(*os.File).Stat()
			if err != nil {
				r.Close()

				log.Printf("stat object: %v", err)
				if !writeError("internal error") {
					return
				}
				return
			}

			size := fi.Size()
			if size+2 <= protocol.MaxPayload {
				buf := make([]byte, size)

				if _, err := io.ReadFull(r, buf); err != nil {
					r.Close()

					log.Printf("read object for DATA: %v", err)
					return
				}
				dataBuf := append([]byte{1, protocol.KindData}, buf...)

				if err := protocol.WriteFrame(conn, dataBuf); err != nil {
					r.Close()

					log.Printf("write DATA: %v", err)
					return
				}
				r.Close()

				continue
			} else {
				chunkBuf := make([]byte, maxBodyPerFrame)
				for {
					n, readErr := r.Read(chunkBuf)
					if n > 0 {
						writeBuf := append([]byte{1, protocol.KindDataChunk}, []byte(chunkBuf[:n])...)
						if werr := protocol.WriteFrame(conn, writeBuf); werr != nil {
							r.Close()
							log.Printf("write DATA_CHUNK: %v", werr)
							return
						}

					}
					if readErr == io.EOF {
						break
					}
					if readErr != nil {
						r.Close()

						log.Printf("read object chunk: %v", readErr)
						if !writeError("read failed") {
							return
						}
						return

					}

				}
				dataBuf := []byte{1, protocol.KindDataEnd}
				if err := protocol.WriteFrame(conn, dataBuf); err != nil {
					r.Close()

					log.Printf("write DATA_END: %v", err)
					return
				}
				r.Close()
				continue
			}

		case protocol.KindPutStreamBegin:
			pr, pw := io.Pipe()
			done := make(chan putResult, 1)
			go func() {
				k, e := tp.store.PutReader(pr)
				done <- putResult{k, e}
			}()
			upload = &uploadSession{pw: pw, done: done}

		case protocol.KindPutStreamChunk, protocol.KindPutStreamEnd:
			if !writeError("PUT_STREAM_BEGIN required") {
				return
			}

		default:
			if !writeError("unexpected or unsupported kind") {
				return
			}
		}
	}
}
