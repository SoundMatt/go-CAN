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

Protocol bridges live under `bridge/` as optional sub-packages — import only
what you need. Each bridge adapts `can.Bus` bidirectionally to a target
protocol, with zero required dependencies in the core library.

---

## Guiding Principles

1. Pure Go first
2. Standards where they provide value (ISO-TP, J1939, DBC, CAN FD)
3. Simplicity over completeness
4. Testability by default — virtual bus works everywhere
5. Safety as a first-class concern
6. Interface-first API — transports are always swappable
7. Optional bridges — protocol adapters live under `bridge/` and carry their own dependencies; core remains zero-dependency

---

## Release Plan

| Version | Theme | Status |
|---|---|---|
| v0.1.0 | Core `can.Bus` interface, virtual bus, DBC parser, ISO-TP, J1939, safety E2E, SocketCAN, Docker quickstart (Milestones 1–9) | shipped |
| v0.1.1 | Patch: `.fusa.json` project config + `.gitignore` fix for safety-artifact release | shipped |
| v0.2.0 | CAN FD support (FD frames, BRS flag, 64-byte payloads via socketcan) (Milestone 10) | shipped |
| v0.3.0 | UDS (ISO 14229) — request/response over ISO-TP; common service IDs (Milestone 11) | shipped |
| v0.4.0 | OBD-II (ISO 15031 / SAE J1979) — Mode 01/02/03/09 PID decoding; formal go-FuSa requirements registry (74 atomic requirements, full traceability) | shipped |
| v0.5.0 | DBC signal encoding (write direction) + value tables (Milestone 6); J1939 Transport Protocol / BAM (Milestone 12); frame recorder and replay (Milestone 13); `mock` package; optional interfaces (`LoaningBus`, `HealthProvider`, `MetricsProvider`, `Drainer`); RELAY spec v0.2 conformance | shipped |
| v0.6.0 | Close FuSa & cybersecurity gaps — 100% requirement traceability, security baseline | shipped |
| v0.7.0 | RELAY spec v1.6 adoption + CAN XL canonical frame support | shipped |
| v0.8.0 | HMAC-SHA256 message authenticator (REQ-SEC-006) + coverage | shipped |
| v0.9.0 | RELAY spec v1.8 — crossbar spoke (streaming NDJSON send/subscribe) | shipped |
| v0.10.0 | RELAY spec v1.10 — §13.7 cross-language library architecture + §20 continuous conformance | shipped |
| v0.10.1 | Patch: per-subscription Seq counters, capabilities accuracy, RELAY spec v1.11, README fix | shipped |
| v0.11.0 | Interop testing infrastructure — two-process self-interop + can-utils third-party interop over real `vcan0` (Milestone 14) | shipped |
| v0.12.1 | Patch: RELAY dependency bump v1.11.0 → `github.com/SoundMatt/RELAY/v2` v2.0.4 (module-path change per RELAY #70; spec v1.12–v1.14 + v2.0 reviewed for conformance impact, none applicable to CAN — verified live against the built `relay` v2.0.4 CLI) | shipped |
| v1.0.0 | API stability, full SocketCAN feature set, documentation complete | planned |
| v1.1.0 | **Bridge — MQTT** (`bridge/mqtt/`) — publish/subscribe CAN frames over MQTT topics | planned |
| v1.2.0 | **Bridge — SOME/IP** (`bridge/someip/`) — translate CAN frames to/from SOME/IP service events | planned |
| v1.3.0 | **Bridge — DDS** (`bridge/dds/`) — CAN frame distribution over DDS topics (works with go-DDS) | planned |
| v1.4.0 | **Bridge — gRPC** (`bridge/grpc/`) — stream CAN frames over gRPC (bidirectional streaming RPC) | planned |
| v1.5.0 | **Bridge — REST** (`bridge/rest/`) — HTTP/REST gateway: send frames via POST, subscribe via SSE | planned |

See the [Milestones](#milestones) section below for what each shipped release actually contains; version numbers above are informational (not derived from git tags automatically), so keep this table in sync by hand whenever a release ships.

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

### 14 — Interop Testing Infrastructure
Real wire-level interop testing beyond in-process unit tests and RELAY's
`relay conform`/`relay interop` (which only check the CLI's JSON output
shape and RELAY-adapter equivalence — neither puts a frame on a real bus).
CAN has no equivalent of a second independent network stack the way DDS has
CycloneDDS to test against; the Linux kernel's own SocketCAN subsystem plus
`can-utils` (`cangen`/`candump`) *is* the real wire and a genuinely
independent (of go-CAN) validator, so that's the third-party oracle here.

- **Two-process self-interop** — `cmd/can-interop-peer`, a standalone
  SocketCAN participant process (not part of the public API or the
  RELAY-conformant `cmd/cantool` CLI) driven entirely by the real,
  production `socketcan.Bus`. `socketcan/interop_two_process_linux_test.go`
  spawns two of these as genuinely separate OS processes bound to the same
  real `vcan0` interface — one sending via `Bus.Send`, the other receiving
  via `Bus.Subscribe` — and asserts field-exact correctness (ID, Ext, RTR,
  FD, BRS, data) frame-for-frame. This is real kernel CAN traffic between
  two processes, not two `Bus` values sharing memory in one process's test
  binary (which `socketcan/bus_linux_test.go`'s existing
  `TestSendReceive`/etc. already cover and cannot, by construction, prove
  process-to-process interop).
- **Third-party-peer interop (can-utils)** —
  `socketcan/interop_canutils_linux_test.go` covers both directions:
  `cangen` (deterministic fixed-value flags, never `-I r`/`-D r` random
  modes, so each invocation's ground truth is exactly what this test told
  it to generate) injects real frames onto `vcan0` that go-CAN's
  `socketcan.Bus` receives and decodes; and go-CAN's own `Bus.Send` output
  is captured independently by `candump -L` (log-file format on stdout,
  the kernel-facing reference decoder) and checked byte-for-byte against
  `candump.c`'s documented log-line format.
- Both suites are gated behind `CAN_INTEROP_TESTS=1` (absent from the fast
  default `go test ./...` sweep and from the existing `test-socketcan` CI
  job) and run in a new `can-interop` CI job
  (`.github/workflows/ci.yml`), ubuntu-only, which loads `vcan`, brings up
  `vcan0`, and installs `can-utils` — probing all three first and skipping
  the job cleanly (`::notice::`, exit 0) rather than hard-failing if any
  of that setup does not succeed, the same posture the existing
  `test-socketcan` job already uses for `vcan0` alone.

---

## Bridges

Each bridge lives under `bridge/<protocol>/` and imports only its own protocol
library — no bridge dependency bleeds into the core `can` package. All bridges
implement the same bidirectional pattern:

- **Subscribe** direction: `can.Bus.Subscribe` → protocol publish
- **Publish** direction: protocol receive → `can.Bus.Send`

### 15 — Bridge: MQTT (`bridge/mqtt/`)
- Adapts any `can.Bus` to an MQTT broker
- CAN frame → MQTT topic (configurable topic pattern, e.g. `can/{id}`)
- MQTT message → CAN frame (with configurable QoS and retain)
- Uses [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) or Eclipse Paho v5
- Bidirectional `Bridge` struct; `Run(ctx)` blocks until context cancelled

### 16 — Bridge: SOME/IP (`bridge/someip/`)
- Translates CAN frames to/from SOME/IP service events
- Compatible with go-SOMEIP
- Each CAN message ID maps to a SOME/IP service/instance/event
- Configurable via a mapping table (JSON or Go struct)

### 17 — Bridge: DDS (`bridge/dds/`)
- Distributes CAN frames as DDS topic samples
- Compatible with go-DDS
- Each CAN frame → typed DDS sample; configurable topic name and QoS profile
- Useful for automotive middleware stacks mixing CAN and DDS domains

### 18 — Bridge: gRPC (`bridge/grpc/`)
- Bidirectional streaming RPC: client streams frames to/from a CAN bus
- Protobuf message mirrors `can.Frame` (ID, Ext, FD, BRS, Data)
- Server-side: wraps any `can.Bus`; client-side: implements `can.Bus` interface
- TLS and mutual-TLS support via standard gRPC dial options

### 19 — Bridge: REST (`bridge/rest/`)
- HTTP/REST gateway for environments where persistent connections are unavailable
- `POST /frames` — send a CAN frame
- `GET  /frames` — Server-Sent Events (SSE) stream of received frames
- `GET  /frames/{id}` — last-known-value for a specific CAN ID
- JSON encoding of `can.Frame`; configurable listen address and filters
