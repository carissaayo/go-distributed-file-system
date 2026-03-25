# Wire protocol v1 (DFS TCP)

## Scope

- **Transport:** TCP only.
- **Version:** **1** (this document).
- **Byte order:** Multi-byte integers are **big-endian** (network byte order).

---

## Frame format (framing layer)

Every message on the wire is **one frame**:

| Field     | Size   | Description |
|-----------|--------|-------------|
| **Length**| 4 bytes| Unsigned **uint32**, big-endian. Denoted **L**. Number of bytes in **Payload** only (does **not** include these 4 bytes). |
| **Payload** | **L** bytes | Interpreted by the message layer below. |

### Limits

- **Minimum L:** **2** (payload always includes **Version** + **Kind**).
- **Maximum L:** **1_048_576** (1 MiB).  
  If **L > 1 MiB**, the implementation **must not** allocate that much; **close the connection** (or reject in a defined way).

### Reading rule

The reader **must**:

1. Read exactly **4** bytes, decode as **uint32** → **L**.
2. If **L** is outside the allowed range, **close the connection**.
3. Read exactly **L** bytes (possibly over multiple `Read` calls) → **Payload**.
4. Parse **Payload** according to “Payload layout”.

Multiple frames may appear back-to-back on one TCP connection. After handling one frame, repeat from step 1.

---

## Payload layout (message layer)

**Payload** bytes:

| Offset | Size | Field |
|--------|------|--------|
| **0**  | 1    | **Version** — must be **1** for this spec. |
| **1**  | 1    | **Kind** — message type (see table). |
| **2**  | **L − 2** | **Body** — optional; meaning depends on **Kind**. If **L = 2**, there is no body. |

If **Version ≠ 1**, **close the connection**.

---

## Kinds (byte at offset 1)

| Kind (hex) | Name  | Required **L** | Body (bytes at offset 2 … L−1) |
|------------|-------|----------------|----------------------------------|
| **0x01**   | PING  | **2** only     | None. |
| **0x02**   | PONG  | **2** only     | None. |
| **0x03**   | ERROR | **3 + N**, **N ≤ 1024** | **N** bytes of **UTF-8** error text. So **L = 2 + N**, **1 ≤ N ≤ 1024**. |

### Validation

- **PING:** If **Kind** is PING and **L ≠ 2**, **close the connection**.
- **PONG:** If **Kind** is PONG and **L ≠ 2**, **close the connection**.
- **ERROR:** If **Kind** is ERROR and **L < 3** (no message) or **N > 1024**, **close the connection**.

---

## Session behavior (v1)

1. TCP connection is established.
2. **Client** sends **one** frame: **PING** (Version **1**, Kind **0x01**, **L = 2**).
3. **Server** sends **one** frame: **PONG** (Version **1**, Kind **0x02**, **L = 2**).
4. v1 does not define further messages; implementations may **close** the connection or extend the protocol in a later document.

---

## Example: PING frame (total 6 bytes on the wire)

- **Length (4 bytes, uint32 BE):** `00 00 00 02` (L = 2)
- **Payload (2 bytes):** `01 01`  
  - Byte 0: **Version = 1**  
  - Byte 1: **Kind = 0x01 (PING)**

---

## Example: PONG frame (total 6 bytes on the wire)

- **Length:** `00 00 00 02`
- **Payload:** `01 02`  
  - Version **1**, Kind **0x02 (PONG)**

---

## Implementation notes (non-normative)

- A single `Read` may return fewer than **L** bytes; implementations must loop until **L** bytes are received or an error occurs.
- **EOF** during a read means the peer closed the connection; treat as end of session.
- Future versions may add new **Kind** values or increase **Version**; v1 peers reject unknown **Version** by closing the connection.

---

## Document history

- **v1:** Length-prefixed frames, PING/PONG handshake, ERROR kind with UTF-8 body (max 1024 bytes).