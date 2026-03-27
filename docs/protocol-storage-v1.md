# Wire protocol — storage extension (v1)

This document **extends** [protocol-v1.md](./protocol-v1.md). Unless stated otherwise, **framing**, **payload layout** (version + kind + body), **limits** (min/max **L**), and **ERROR** (`0x03`) behave exactly as in v1.

---

## Scope of this extension

- **Content addressing:** object identity is **SHA-256** over raw bytes.
- **Key format on the wire:** **64 ASCII hexadecimal characters** (lowercase preferred), representing **32 bytes** of hash. Example: `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty SHA-256).
- **Single-frame messages only** in this extension: one **PUT** or **GET** request or response must fit in **one** frame (**L ≤ 1_048_576**). Larger objects require a **future** chunked protocol.

---

## New kinds (byte at offset 1)


| Kind (hex) | Name       | Direction (typical) | Body (bytes at offset 2 … L−1)                                                                           |
| ---------- | ---------- | ------------------- | -------------------------------------------------------------------------------------------------------- |
| **0x10**   | **PUT**    | Client → Server     | **Raw file/object bytes** (may be **empty**: then **L = 2**).                                            |
| **0x11**   | **GET**    | Client → Server     | **Exactly 64 bytes**: object key as **ASCII hex** SHA-256 (see below).                                   |
| **0x12**   | **STORED** | Server → Client     | **Exactly 64 bytes**: ASCII hex of the **stored** object hash (same as key). Confirms **PUT** completed. |
| **0x13**   | **DATA**   | Server → Client     | **Raw bytes** of the object returned for **GET** (may be empty).                                         |


**Note:** Use **ERROR** (`0x03`) for failures (e.g. unknown key, invalid GET body), with UTF-8 text in the body per v1.

---

## Validation rules

### PUT (`0x10`)

- **L ≥ 2**. Body length **N = L − 2** may be **0** (empty object).
- Server computes **SHA-256(body)**, stores bytes addressed by that hash (idempotent: same bytes → same path), then responds with **STORED** or **ERROR**.

### GET (`0x11`)

- **L** must be exactly **66** (**2 + 64**): version + kind + **64-byte** key.
- Key **must** match **^[0-9a-f]{64}$** (implementations may accept **A–F** and normalize; normative is **lowercase hex**).
- If the object **does not exist**, server sends **ERROR** (body e.g. `not found`) and **must not** send **DATA**.

### STORED (`0x12`)

- **L** must be exactly **66**: version + kind + **64** hex chars of the **content hash** after a successful **PUT**.

### DATA (`0x13`)

- **L ≥ 2**. Body is the object bytes. **L − 2** may be **0**.

---

## Session behavior (recommended)

1. **Optional:** v1 **PING** / **PONG** handshake (same as [protocol-v1.md](./protocol-v1.md)).
2. **Client** sends **PUT** or **GET** frames as needed (each is one frame).
3. **Server** responds with **STORED** or **ERROR** after **PUT**; **DATA** or **ERROR** after **GET**.
4. Connection may stay open for multiple requests or close after one; both are allowed until a future spec narrows this.

---

## Size limits (reminder)

- **Max frame payload L:** **1_048_576** (1 MiB) — same as v1.
- Therefore **max object size** in **one PUT** or **one DATA** response is **1_048_576 − 2** bytes.
- **GET** request payload is fixed **66** bytes.

---

## Examples (payload only; prepend 4-byte length + use v1 framing on the wire)

### PUT two bytes of content `hi`

- **L** = 2 + 2 = **4**
- Payload: `01 10 68 69` → version **1**, **PUT**, body `**h` `i`**

### GET (key)

- **L** = **66**
- Payload: `01 11` + **64** ASCII hex digits (full SHA-256 key)

### STORED (acknowledgement)

- **L** = **66**
- Payload: `01 12` + **64** ASCII hex digits (the stored hash)

### DATA (empty object)

- **L** = **2**
- Payload: `01 13` (version **1**, **DATA**, no body)

---

## Document history

- **storage-v1:** PUT/GET/STORED/DATA kinds, SHA-256 keys as 64 hex ASCII chars, single-frame size limits.

