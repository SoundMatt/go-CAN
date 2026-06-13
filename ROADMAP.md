# go-CAN Roadmap

## Vision

go-CAN is a modern, Go-native CAN bus library for automotive, industrial, and
heavy-vehicle domains.

The project focuses on:

- A clean, stable `can.Bus` interface with swappable transports
- Pure Go where possible — no CGo, no native dependencies beyond SocketCAN
- Safety-oriented design with go-FuSa annotations and E2E protection
- Standards compliance: ISO 15765-2 (ISO-TP), SAE J1939, DBC signal decoding
- Testability by default via the in-process virtual bus

go-CAN does not bridge to other protocols (DDS, MQTT, SOME/IP, etc.).
Other projects adapt to go-CAN.

---

## Guiding Principles

1. Pure Go first
2. Standards where they provide value (ISO-TP, J1939, DBC, CAN FD)
3. Simplicity over completeness
4. Testability by default — virtual bus works everywhere
5. Safety as a first-class concern
6. Interface-first API — transports are always swappable

---

## Release Plan

| Version | Theme | Status |
|---|---|---|
| v0.1.0 | Core `can.Bus` interface, virtual bus, DBC parser, ISO-TP, J1939, safety E2E, SocketCAN, Docker quickstart | **next** |
| v0.2.0 | CAN FD support (FD frames, BRS flag, 64-byte payloads via socketcan) | planned |
| v0.3.0 | UDS (ISO 14229) — request/response over ISO-TP; common service IDs | planned |
| v0.4.0 | OBD-II (ISO 15031) — Mode 01/02/03/09 PID decoding | planned |
| v0.5.0 | J1939 transport layer — Transport Protocol (TP) for multi-packet PGNs (>8 bytes) | planned |
| v0.6.0 | DBC signal encoding (write direction) and value table support | planned |
| v0.7.0 | Logging / trace — candump-compatible frame recording and replay | planned |
| v0.8.0 | go-FuSa v0.30.0 → latest; coverage 80% across all packages | planned |
| v0.9.0 | Statistics and metrics — bus load, error frames, frame counters per ID | planned |
| v1.0.0 | API stability, full SocketCAN feature set, documentation complete | planned |

---

## Milestones

### 1 — Core Transport Abstraction
- `can.Bus` interface (Send, Subscribe, Close)
- `can.Frame` with standard and extended IDs, CAN FD, RTR
- `can.Filter` with masked ID matching
- `can.ValidateFrame`

### 2 — Virtual In-Process Bus
- Zero-dependency broadcast bus
- Multiple subscribers with independent filter sets
- Drop-on-full-channel semantics (mirrors real CAN behaviour)
- Fuzz target for `Send`

### 3 — SocketCAN
- Linux `AF_CAN` raw socket
- vcan0 integration tests (skips gracefully when unavailable)
- Standard + extended frame format
- Non-Linux stub (error + redirect to virtual)

### 4 — DBC Parser
- Messages, signals, byte-order (Intel/Motorola), signed/unsigned
- Scale, offset, min, max, unit, receivers
- Signal decoder: `db.Decode(id, data) map[string]float64`
- Fuzz target for `Parse`

### 5 — ISO-TP (ISO 15765-2)
- Single Frame, First Frame, Consecutive Frame, Flow Control
- BlockSize and STmin parameters
- Up to 4095-byte payloads

### 6 — J1939
- PGN encode/decode (29-bit extended ID layout)
- Peer-to-peer vs broadcast addressing
- `j1939.Bus` wrapping any `can.Bus`

### 7 — Safety E2E
- 10-byte protection header: DataID, SourceID, SequenceCounter, CRC-16/CCITT-FALSE
- `Sender` and `Receiver` wrappers
- Detects CRC mismatch, sequence gaps, and short headers

### 8 — CLI (cantool)
- `send <iface> <id>#<data>` — transmit a frame
- `dump <iface>` — print all received frames
- `virtual` pseudo-interface for platform-independent testing

### 9 — Docker Quickstart
- Single-container demo with virtual bus sender + receiver goroutines
- Multi-arch image (linux/amd64, linux/arm64) published to GHCR

### 10 — CAN FD
- Extended `can.Frame` flags: `FD`, `BRS`
- SocketCAN CAN FD socket (`SOCK_RAW` with `CAN_RAW_FD_FRAMES`)
- Up to 64-byte payloads

### 11 — UDS (ISO 14229)
- Request/response session over ISO-TP
- Common service IDs: ReadDataByIdentifier (0x22), WriteDataByIdentifier (0x2E),
  DiagnosticSessionControl (0x10), ECUReset (0x11)

### 12 — J1939 Transport Protocol
- Multi-packet PGNs (>8 bytes) via J1939 TP (BAM and CMDT)
- RTS/CTS handshake for peer-to-peer TP

### 13 — Frame Recorder and Replay
- Record frames to JSONL file (with timestamps)
- Replay in real-time or at scaled rate
- candump-compatible text format option
