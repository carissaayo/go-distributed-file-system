# Wire protocol — storage (single-frame and streaming)

This document **extends** [protocol.md](./protocol.md). Unless stated otherwise, **framing**, **payload layout** (version + kind + body), **limits** (min/max **L**), and **ERROR** (`0x03`) behave exactly as in that document.

---

## Scope

- **Content addressing:** object identity is **SHA-256** over raw bytes.
- **Key format on the wire:** **64 ASCII hexadecimal characters** (lowercase preferred), representing **32 bytes** of hash. Example: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty SHA-256).
- **Single-frame operations:** when the whole object fits in **one** frame (**L ≤ 1_048_576**), use **PUT** (`0x10`) and **DATA** (`0x13`) as below.
- **Streaming:** when the object body does **not** fit in one frame on upload or download, use the **PUT_STREAM_**\* and **DATA_CHUNK** / **DATA_END** kinds (`0x14`–`0x18`). Peers negotiate streaming by **sending** those kinds.

---

## All storage-related kinds (byte at offset 1)

| Kind (hex) | Name                 | Direction (typical) | Body (bytes at offset 2 … L−1) |
| ---------- | -------------------- | ------------------- | ------------------------------ |
| **0x10**   | **PUT**              | Client → Server     | **Raw object bytes** (may be **empty**: **L = 2**). |
| **0x11**   | **GET**              | Client → Server     | **Exactly 64 bytes**: ASCII hex SHA-256 key. |
| **0x12**   | **STORED**           | Server → Client     | **Exactly 64 bytes**: ASCII hex of stored hash. |
| **0x13**   | **DATA**             | Server → Client     | **Raw bytes** of object for **GET** (may be empty). **Single-frame** response when object fits one frame. |
| **0x14**   | **PUT_STREAM_BEGIN** | Client → Server     | **None** — **L = 2** only. |
| **0x15**   | **PUT_STREAM_CHUNK** | Client → Server     | **Raw bytes** — next segment (may be **empty**: **L = 2**). |
| **0x16**   | **PUT_STREAM_END**   | Client → Server     | **None** — **L = 2** only. Finalizes upload. |
| **0x17**   | **DATA_CHUNK**       | Server → Client     | **Raw bytes** — next segment of **GET** response (may be **empty**: **L = 2**). |
| **0x18**   | **DATA_END**         | Server → Client     | **None** — **L = 2** only. Ends **GET** stream. |

Use **ERROR** (`0x03`) for failures, with UTF-8 text in the body per [protocol.md](./protocol.md).

---

## Single-frame PUT / GET / STORED / DATA

### PUT (`0x10`)

- **L ≥ 2**. Body length **N = L − 2** may be **0** (empty object).
- Server computes **SHA-256(body)**, stores bytes addressed by that hash (idempotent: same bytes → same path), then responds with **STORED** or **ERROR**.

### GET (`0x11`)

- **L** must be exactly **66** (**2 + 64**).
- Key **must** match **^[0-9a-f]{64}$** (implementations may accept **A–F** and normalize).
- If the object **does not exist**, server sends **ERROR** and **must not** send **DATA** / streaming data.

### STORED (`0x12`)

- **L** must be exactly **66**: version + kind + **64** hex chars of the content hash after **PUT** or streaming PUT.

### DATA (`0x13`)

- **L ≥ 2**. Body is the full object in **one** frame. **L − 2** may be **0**.

**Max object size** in one **PUT** or one **DATA** frame: **1_048_576 − 2** bytes (body only).

---

## Streaming PUT (unknown length)

For objects larger than one **PUT** frame allows, the client sends:

1. **PUT_STREAM_BEGIN** (`L = 2`)
2. Zero or more **PUT_STREAM_CHUNK** frames, each with **L ≤ 1_048_576** (**body ≤ 1_048_574** bytes)
3. **PUT_STREAM_END** (`L = 2`)

Server computes **SHA-256** over the concatenation of all chunk bytes, stores, responds with **STORED** or **ERROR**.

### Ordering (upload)

- After **PUT_STREAM_BEGIN**, the connection is in **upload mode** until the server sends **STORED** or **ERROR** for that upload.
- In upload mode the client sends only **PUT_STREAM_CHUNK** (any number, including zero) then **exactly one** **PUT_STREAM_END**.
- The client **must not** send **GET**, **PUT** (`0x10`), **PUT_STREAM_BEGIN**, **PING**, or **PONG** until that upload completes (see strict ordering in implementations).

### Mid-session ERROR (upload)

- On **ERROR**, the upload is aborted; the server **must not** send **STORED** for that session. The next upload **must** start with a new **PUT_STREAM_BEGIN**.

---

## Streaming GET

Client sends **GET** as usual (**L = 66**).

Server responds with either:

- **ERROR**, or
- **exactly one** **DATA** frame if the object fits (**L ≤ 1_048_576**), or
- **one or more** **DATA_CHUNK** frames (in order), then **exactly one** **DATA_END** (`L = 2`).

The concatenation of all **DATA_CHUNK** bodies equals the object bytes. Clients **must** accept both **DATA**-only and **DATA_CHUNK** + **DATA_END** responses.

### Mid-session ERROR (GET)

- Same as single-frame: if **ERROR**, no **DATA** / **DATA_CHUNK** / **DATA_END** for that **GET**.

---

## Size limits (reminder)

- **Max frame payload L:** **1_048_576** (1 MiB).
- **Max chunk body** per **PUT_STREAM_CHUNK** / **DATA_CHUNK:** **1_048_574** bytes.
- **GET** request payload is fixed **66** bytes.

---

## Examples (payload only; prepend 4-byte length per [protocol.md](./protocol.md))

### Small PUT (`hi`)

- **L** = 4 — Payload: `01 10 68 69`

### GET (key)

- **L** = **66** — Payload: `01 11` + **64** ASCII hex digits

### STORED

- **L** = **66** — Payload: `01 12` + **64** hex digits

### Streaming PUT (empty object)

1. `01 14` (**BEGIN**)
2. `01 16` (**END**)
3. Server: **STORED** with empty SHA-256 hash hex

### Streaming GET (two chunks)

1. Client: **GET** (66 bytes)
2. Server: **DATA_CHUNK** — `01 17` + bytes
3. Server: **DATA_CHUNK** — `01 17` + bytes
4. Server: **DATA_END** — `01 18` (**L = 2**)

---

## Document history

- **Single doc:** Merged former storage v1 (single-frame **PUT**/**GET**/**DATA**/**STORED**) and storage v2 (streaming **PUT_STREAM_**\* / **DATA_CHUNK**/**DATA_END**), ordering, and error rules.
