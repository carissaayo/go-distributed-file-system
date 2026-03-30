# Wire protocol — storage extension (v2): streaming

This document **extends** [protocol-v1.md](./protocol-v1.md) and [protocol-storage-v1.md](./protocol-storage-v1.md). Unless stated otherwise, **framing**, **payload layout** (version + kind + body), **limits** (min/max **L**), and **ERROR** (`0x03`) behave as in v1.

**v2 adds** multi-frame **streaming PUT** (unknown total length) and multi-frame **streaming GET**, with **strict frame ordering**, **per-chunk size caps**, and **mid-session error** semantics.

---

## Relationship to storage v1

- **storage-v1** remains valid for **single-frame** **PUT** (`0x10`) and **GET** / **DATA** (`0x11` / `0x13`) when the whole object fits in **one** frame (see [protocol-storage-v1.md](./protocol-storage-v1.md)).
- **storage-v2** is used when either side needs **more than one frame** to carry an object body on **upload** or **download**.
- Peers negotiate use of v2 by **sending v2 kinds** (`0x14`–`0x18`). If a peer receives an unknown **Kind**, behavior follows v1: implementations **may** close the connection (see [protocol-v1.md](./protocol-v1.md) implementation notes).

---

## Scope of this extension

- **Content addressing** and **64-character ASCII hex keys** are unchanged from storage-v1.
- **Upload:** **Unknown-length** stream: client sends **PUT_STREAM_BEGIN** → zero or more **PUT_STREAM_CHUNK** → **PUT_STREAM_END**; server responds with **STORED** (`0x12`) or **ERROR** (`0x03`).
- **Download:** Client sends **GET** (`0x11`) as in v1. Server responds with either:
  - **ERROR**, or
  - a **single-frame** **DATA** (`0x13`) (v1-compatible, object fits in one frame), or
  - **one or more** **DATA_CHUNK** (`0x17`) frames followed by **exactly one** **DATA_END** (`0x18`) (streaming).
- **Chunk body cap:** In every **PUT_STREAM_CHUNK** and **DATA_CHUNK** frame, **body length** **B = L − 2** satisfies **0 ≤ B ≤ 1_048_574** (so **L ≤ 1_048_576**, same max as v1). This is the **maximum chunk payload** (~1 MiB including the 2-byte version/kind prefix in **L**).

---

## New kinds (byte at offset 1)


| Kind (hex) | Name                 | Direction (typical) | Body (bytes at offset 2 … L−1)                                                        |
| ---------- | -------------------- | ------------------- | ------------------------------------------------------------------------------------- |
| **0x14**   | **PUT_STREAM_BEGIN** | Client → Server     | **None** — **L = 2** only.                                                            |
| **0x15**   | **PUT_STREAM_CHUNK** | Client → Server     | **Raw bytes** — next segment of the object (may be **empty**: **L = 2**).             |
| **0x16**   | **PUT_STREAM_END**   | Client → Server     | **None** — **L = 2** only. Ends the stream; server finalizes **SHA-256** and stores.  |
| **0x17**   | **DATA_CHUNK**       | Server → Client     | **Raw bytes** — next segment of the object for **GET** (may be **empty**: **L = 2**). |
| **0x18**   | **DATA_END**         | Server → Client     | **None** — **L = 2** only. Marks end of the **GET** response stream.                  |


**Kinds from storage-v1** used without change in v2: **GET** (`0x11`), **STORED** (`0x12`), **ERROR** (`0x03`). **DATA** (`0x13`) is used only for **single-frame** responses.

---

## Validation rules

### PUT_STREAM_BEGIN (`0x14`)

- **L** must be exactly **2** (no body).

### PUT_STREAM_CHUNK (`0x15`)

- **L ≥ 2**. **B = L − 2** is the number of chunk bytes (**0 ≤ B ≤ 1_048_574**).
- **Zero-length chunks** (**L = 2**) are valid and contribute **no bytes** to the object (and do not change the rolling hash).

### PUT_STREAM_END (`0x16`)

- **L** must be exactly **2** (no body).
- Server computes **SHA-256** over the **concatenation** of all chunk bytes in order, stores the object, responds with **STORED** (same as v1: **L = 66**, 64 hex chars) or **ERROR**.

### DATA_CHUNK (`0x17`) — server → client

- **L ≥ 2**, same **B** bounds as **PUT_STREAM_CHUNK**.

### DATA_END (`0x18`) — server → client

- **L** must be exactly **2** (no body).

### GET (`0x11`)

- Unchanged from storage-v1 (**L = 66**, 64-char hex key).

---

## Ordering rules (normative)

These rules avoid ambiguous state on the connection.

### One streaming upload at a time

- After **PUT_STREAM_BEGIN**, the connection is in **upload mode** until the server has sent **exactly one** terminal response for that upload: **STORED** or **ERROR**.
- In **upload mode**, the client **must** send only:
  - **PUT_STREAM_CHUNK** (any number, including zero), then
  - **exactly one** **PUT_STREAM_END**.
- The client **must not** send **PUT_STREAM_BEGIN** again until **upload mode** has ended (after **STORED** or **ERROR**).
- In **upload mode**, the client **must not** send **GET**, **PUT** (v1 `0x10`), **PUT_STREAM_BEGIN**, **PING**, or **PONG**. (Optional **PING** during long uploads is **not** allowed in v2; keep the connection alive by other means or close and reconnect.)

### Upload frame sequence (client)

Exact sequence:

1. **PUT_STREAM_BEGIN** (`L = 2`)
2. Zero or more **PUT_STREAM_CHUNK** (each `L ≤ 1_048_576`)
3. **PUT_STREAM_END** (`L = 2`)

Then the server sends **STORED** or **ERROR**.

### Download frame sequence (server)

After a valid **GET**:

- If the implementation sends **ERROR**, it **must not** send **DATA**, **DATA_CHUNK**, or **DATA_END** for that **GET**.
- If the object fits in a **single** **DATA** frame (**L ≤ 1_048_576**), the server **may** send **exactly one** **DATA** (`0x13`) and **must not** send **DATA_CHUNK** / **DATA_END** for that **GET**.
- If the object does **not** fit in one **DATA** frame, the server **must** send **one or more** **DATA_CHUNK** frames (in order), then **exactly one** **DATA_END** (`L = 2`). The **concatenation** of all **DATA_CHUNK** bodies equals the object bytes.

Clients implementing v2 **must** accept:

- **DATA** only (small object), or  
- **DATA_CHUNK** + **DATA_END** (streaming).

---

## Mid-session errors (normative)

### Server sends ERROR during upload

- **When:** e.g. invalid frame while in **upload mode**, I/O failure, malformed **PUT_STREAM_** sequence, policy violation.
- **Effect:** The **upload session is aborted**. The server **must not** emit **STORED** for that session.
- **Client obligation:** After receiving **ERROR** that refers to the failed upload (or after any **ERROR** while in **upload mode**), the client **must** treat **upload mode** as **cleared**. The **next** upload on this connection **must** start with a new **PUT_STREAM_BEGIN**.
- **Forbidden:** Sending **PUT_STREAM_CHUNK** or **PUT_STREAM_END** after the server has already responded with **ERROR** for that upload. If the client does this, the server **should** send **ERROR** and **may** **close the connection** without reading further frames.

### Client violates ordering

- If the server receives an unexpected **Kind** while in **upload mode** (see ordering rules), the server **must** send **ERROR** (UTF-8 body explaining the violation), **abort** the upload, and **may** **close the connection**.

### Server sends ERROR during GET

- Same as v1: **no** **DATA** / **DATA_CHUNK** / **DATA_END** for that **GET**.

### Connection close

- Either side **may** close the TCP connection at any time; partial uploads/downloads have no guaranteed durability until **STORED** (upload) or full response (download) is received.

---

## Recommended session flow (informative)

1. Optional: **PING** / **PONG** ([protocol-v1.md](./protocol-v1.md)).
2. **Streaming PUT:** **PUT_STREAM_BEGIN** → chunks → **PUT_STREAM_END** → **STORED** or **ERROR**.
3. **GET:** **GET** → **ERROR**, or **DATA**, or **DATA_CHUNK** → **DATA_END**.
4. v1 **single-frame PUT** remains available for small objects.

---

## Size limits (reminder)

- **Max frame payload L:** **1_048_576** (1 MiB) — unchanged.
- **Max chunk body** per **PUT_STREAM_CHUNK** / **DATA_CHUNK:** **1_048_574** bytes.
- **Object size** is unbounded by framing (bounded only by disk, memory strategy, and implementation limits).

---

## Examples (payload only; prepend 4-byte length per v1)

### Streaming PUT: empty object (hash of empty blob)

1. Payload **PUT_STREAM_BEGIN:** `01 14` (**L = 2**)
2. Payload **PUT_STREAM_END:** `01 16` (**L = 2**)
3. Server responds **STORED** with **64** hex chars of **SHA-256** of empty input (same as v1 empty object).

### Streaming GET: two chunks + end

Assume object is split for illustration.

1. Client: **GET** (66 bytes) as in v1.
2. Server: **DATA_CHUNK** #1 — `01 17` + first bytes
3. Server: **DATA_CHUNK** #2 — `01 17` + remaining bytes
4. Server: **DATA_END** — `01 18` (**L = 2**)

---

## Document history

- **storage-v2:** Streaming PUT (**BEGIN** / **CHUNK** / **END**), streaming GET (**DATA_CHUNK** / **DATA_END**), unknown-length uploads, strict ordering, mid-session **ERROR** rules.

