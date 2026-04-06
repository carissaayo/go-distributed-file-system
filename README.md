# Distributed file system (content-addressed storage over TCP)

A small **Go** service that stores opaque blobs on disk keyed by **SHA-256** (hex). Clients speak a **length-prefixed TCP protocol**: optional **PING/PONG**, **PUT** (single-frame or streaming for large files), and **GET** by hash. See the specs in **`docs/protocol.md`** (framing, handshake) and **`docs/protocol-storage.md`** (storage kinds and streaming).

## Requirements

- [Go](https://go.dev/dl/) 1.21+ (module uses modern `go` toolchain)

## Run the server

From the repository root:

```bash
go run ./cmd/server -addr :3000 -data ./data
```

- **`-addr`** — TCP listen address (default `:3000`).
- **`-data`** — root directory for stored blobs (created if missing; default `./data`).

The server logs its listen address and accepts connections until you stop the process (**Ctrl+C**).

## Run the client

All examples use **`localhost:3000`**; override with **`-addr host:port`**.

```bash
# Optional handshake (server must respond with PONG)
go run ./cmd/client -addr localhost:3000 ping

# Upload a file (small files use one PUT frame; larger files use streaming PUT)
go run ./cmd/client -addr localhost:3000 put -file ./myfile.bin

# Download by 64-character hex content hash (prints to stdout)
go run ./cmd/client -addr localhost:3000 get -key <64-hex-chars>

# Save downloaded bytes to a file instead of stdout
go run ./cmd/client -addr localhost:3000 get -key <64-hex-chars> -out ./downloaded.bin
```

After **`put`**, the client prints **`stored: <64-hex-key>`** — use that string as **`-key`** for **`get`**.

## Project layout

| Path | Role |
|------|------|
| `cmd/server` | TCP listener, one goroutine per connection |
| `cmd/client` | CLI: `ping`, `put`, `get` |
| `internal/protocol` | Framing, kinds, `ReadFrame` / `WriteFrame` / `ParsePayload` |
| `internal/store` | Content-addressed files under `./data/<aa>/<bb>/<hash>` |
| `transport` | Connection handler: upload FSM, GET streaming |
| `docs/` | Wire protocol documentation |

## Tests

```bash
go test ./... -timeout 120s
```

Large-object tests allocate ~1 MiB payloads and may take several seconds.
