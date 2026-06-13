# go-CAN

A Go library for [CAN bus](https://en.wikipedia.org/wiki/CAN_bus) (Controller Area Network) communication.
Works in automotive, industrial, robotics, and heavy-vehicle domains.

The `can.Bus` interface is stable. Implementations are swappable without changing application code.

[![CI](https://github.com/SoundMatt/go-CAN/actions/workflows/ci.yml/badge.svg)](https://github.com/SoundMatt/go-CAN/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/SoundMatt/go-CAN.svg)](https://pkg.go.dev/github.com/SoundMatt/go-CAN)

## Packages

| Package | Description | Requires |
|---|---|---|
| `can` | Core `Bus` interface, `Frame`, `Filter`, and validation. | Nothing |
| `virtual` | In-process broadcast bus. Zero dependencies. Default for development and testing. | Nothing |
| `socketcan` | Linux SocketCAN — hardware and virtual CAN interfaces (can0, vcan0, …). | Linux kernel ≥ 3.6 |
| `dbc` | DBC file parser and signal decoder. | Nothing |
| `isotp` | ISO 15765-2 (ISO-TP) multi-frame transport. | Nothing |
| `j1939` | SAE J1939 — 29-bit extended ID, PGN addressing, J1939 Bus. | Nothing |
| `safety` | E2E protection header — DataID, SourceID, SequenceCounter, CRC-16. | Nothing |
| `cmd/cantool` | CLI tool: `send`, `dump` subcommands. | Nothing |

## Install

```bash
go get github.com/SoundMatt/go-CAN
```

## Quick start

```go
import (
    can "github.com/SoundMatt/go-CAN"
    "github.com/SoundMatt/go-CAN/virtual"
)

bus, _ := virtual.New()
defer bus.Close()

ch, _ := bus.Subscribe(can.Filter{ID: 0x100, Mask: 0x7FF})
bus.Send(context.Background(), can.Frame{ID: 0x100, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}})

frame := <-ch
fmt.Printf("%03X#%X\n", frame.ID, frame.Data) // 100#DEADBEEF
```

## Docker quickstart

```bash
docker compose -f docker/docker-compose.yml up --build
```

Runs a single container with sender and receiver goroutines on an in-process virtual bus.

## Switching transports

```go
// Development / testing — zero dependencies:
import "github.com/SoundMatt/go-CAN/virtual"
bus, err := virtual.New()

// Linux hardware or vcan — real SocketCAN:
import "github.com/SoundMatt/go-CAN/socketcan"
bus, err := socketcan.New("can0")   // or "vcan0" for virtual CAN
```

Application code only references the `can.Bus` interface — swap the transport at the call site.

## DBC signal decoding

```go
import (
    "strings"
    "github.com/SoundMatt/go-CAN/dbc"
)

db, _ := dbc.Parse(strings.NewReader(dbcContent))
values := db.Decode(0x100, frame.Data)
fmt.Println(values["EngineSpeed"], "rpm")
```

## ISO-TP (multi-frame messages)

```go
import "github.com/SoundMatt/go-CAN/isotp"

conn, _ := isotp.New(bus, isotp.Config{TxID: 0x7E0, RxID: 0x7E8})
conn.Send(ctx, payload)           // up to 4095 bytes
data, _ := conn.Recv(ctx)
```

## J1939

```go
import "github.com/SoundMatt/go-CAN/j1939"

jBus := j1939.NewBus(bus, 0x00)  // source address 0x00
ch, _ := jBus.Subscribe(0x0FECA) // CCVS PGN
jBus.Send(ctx, j1939.Frame{Priority: 6, PGN: 0x0FECA, Data: payload})
```

## Safety E2E protection

```go
import "github.com/SoundMatt/go-CAN/safety"

cfg := safety.Config{DataID: 0x0001, SourceID: 0x0010}
protector := safety.NewProtector(cfg)
receiver := safety.NewReceiver(cfg)

// wrap the payload, then send via ISO-TP or CAN FD
protected := protector.Protect([]byte{0x01, 0x02, 0x03})
conn.Send(ctx, protected)  // e.g. an isotp.Conn

// on receive (after ISO-TP reassembly):
payload, err := receiver.Unwrap(data)
```

The 10-byte header (DataID, SourceID, SequenceCounter, CRC-16) does not fit in a standard 8-byte CAN frame. Use with ISO-TP or CAN FD.

## Philosophy

- **Interface-first** — one stable `can.Bus` interface; transports are swappable.
- **Safety-oriented** — go-FuSa annotations throughout; E2E protection built-in.
- **Testable by default** — the virtual bus needs no OS support; tests run anywhere.
- **Extensible** — bridge packages to other protocols (DDS, MQTT, SOME/IP) belong here or in consuming projects.

## License

Mozilla Public License v2.0. Copyright (c) 2026 Matt Jones.
