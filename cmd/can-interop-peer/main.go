//go:build linux

// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command can-interop-peer is a standalone SocketCAN participant process used
// only by the live interop test harness in socketcan/*_test.go. It is not
// part of go-CAN's public API and not the RELAY-conformant cmd/cantool CLI —
// it exists purely so a Go test can spawn two (or more) independent OS
// processes that talk to each other over a real Linux vcan/can interface,
// which an in-process test using two *socketcan.Bus values in the same
// goroutine tree cannot prove (that only proves the Bus type is safe to use
// twice in one process, not that two independent implementations of "a
// go-CAN process" actually interoperate over the wire).
//
// It drives the real, production socketcan.Bus
// (github.com/SoundMatt/go-CAN/socketcan) — genuine kernel CAN traffic, no
// mock — and reports what it sent or received as a single line of JSON on
// stdout, so the spawning test can assert field-exact correctness (ID, Ext,
// RTR, FD, BRS, data) without sharing any in-process state with the process
// under test.
//
// Usage:
//
//	can-interop-peer --role writer --iface vcan0
//	can-interop-peer --role reader --iface vcan0 [--count N] [--timeout-secs N]
//
// role=writer sends the fixed vector set from defaultVectors(), once, in
// order, and reports what it sent.
//
// role=reader subscribes, prints the single line "READY" on stderr the
// instant it is actually listening (Subscribe has returned — the earliest
// point at which a frame sent by another process is guaranteed not to be
// silently dropped by a not-yet-subscribed reader; see socketcan/bus_linux.go's
// readLoop, which only dispatches to subscriptions already registered in
// Bus.subs), then collects up to --count frames (default: len(defaultVectors()))
// or until --timeout-secs elapses, and reports what it received.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/socketcan"
)

// frameJSON is the wire (JSON) representation of a can.Frame in this peer's
// report. It is a report format owned by this test-support binary, not the
// RELAY relay.Message/can.Frame JSON contract, so it stays simple and stable
// independent of how those types evolve.
type frameJSON struct {
	ID      uint32 `json:"id"`
	Ext     bool   `json:"ext"`
	RTR     bool   `json:"rtr"`
	FD      bool   `json:"fd"`
	BRS     bool   `json:"brs"`
	DataHex string `json:"data_hex"`
}

func toFrameJSON(f can.Frame) frameJSON {
	return frameJSON{
		ID:      f.ID,
		Ext:     f.Ext,
		RTR:     f.RTR,
		FD:      f.FD,
		BRS:     f.BRS,
		DataHex: hex.EncodeToString(f.Data),
	}
}

// report is the single JSON line printed to stdout on exit.
type report struct {
	Role     string      `json:"role"`
	OK       bool        `json:"ok"`
	Sent     []frameJSON `json:"sent,omitempty"`
	Received []frameJSON `json:"received,omitempty"`
	Error    string      `json:"error,omitempty"`
}

func main() {
	role := flag.String("role", "", "writer | reader")
	iface := flag.String("iface", "vcan0", "SocketCAN interface")
	count := flag.Int("count", -1, "reader: number of frames to collect (default: len(defaultVectors()))")
	timeoutSecs := flag.Int("timeout-secs", 5, "reader: give up waiting after this many seconds")
	printVectorCount := flag.Bool("print-vector-count", false, "print len(defaultVectors()) and exit 0 (used by tests that need to know the writer's frame count up front, e.g. to size a candump -n bound, without hardcoding a second copy of that count)")
	flag.Parse()

	if *printVectorCount {
		fmt.Println(len(defaultVectors()))
		return
	}

	var rep report
	switch *role {
	case "writer":
		rep = runWriter(*iface)
	case "reader":
		n := *count
		if n < 0 {
			n = len(defaultVectors())
		}
		rep = runReader(*iface, n, time.Duration(*timeoutSecs)*time.Second)
	default:
		fmt.Fprintln(os.Stderr, "can-interop-peer: --role must be writer or reader")
		os.Exit(2)
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(rep); err != nil {
		fmt.Fprintln(os.Stderr, "can-interop-peer: encode report:", err)
		os.Exit(1)
	}
	if !rep.OK {
		os.Exit(1)
	}
}

// defaultVectors is the fixed, deterministic frame set used by the
// two-process self-interop test and the go-CAN-to-candump direction of the
// can-utils third-party interop test. It covers every field the socketcan
// wire codec (socketcan/bus_linux.go's encodeFrame/decodeFrame) round trips:
// standard and extended IDs, RTR, a zero-length payload, CAN FD with and
// without BRS, and a maximum-length (64-byte) FD payload.
//
// CAN XL is deliberately not exercised here: socketcan/bus_linux.go's
// encodeFrame/decodeFrame has no XL branch (only classic and FD kernel frame
// layouts), so there is nothing to interop-test at the wire level yet — see
// can.go's CANXL* constants and Frame.XL field, which are structural
// (RELAY-conformance) support only at this point.
func defaultVectors() []can.Frame {
	return []can.Frame{
		{ID: 0x123, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{ID: 0x000, Data: nil},
		{ID: 0x1FFFFFFF, Ext: true, Data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}},
		{ID: 0x0ABCDE, Ext: true, Data: []byte{0xAA}},
		{ID: 0x321, RTR: true},
		{ID: 0x1ABCDEF, Ext: true, RTR: true},
		{ID: 0x400, FD: true, BRS: true, Data: fdPayload(20)},
		{ID: 0x1234567, Ext: true, FD: true, Data: fdPayload(64)},
		{ID: 0x500, FD: true, BRS: true, Data: fdPayload(5)},
	}
}

func fdPayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + 1)
	}
	return b
}

func runWriter(iface string) report {
	bus, err := socketcan.New(iface)
	if err != nil {
		return report{Role: "writer", OK: false, Error: err.Error()}
	}
	defer bus.Close()

	vectors := defaultVectors()
	sent := make([]frameJSON, 0, len(vectors))
	for _, f := range vectors {
		if err := bus.Send(context.Background(), f); err != nil {
			return report{
				Role: "writer", OK: false, Sent: sent,
				Error: fmt.Sprintf("send id=0x%X: %v", f.ID, err),
			}
		}
		sent = append(sent, toFrameJSON(f))
	}
	return report{Role: "writer", OK: true, Sent: sent}
}

func runReader(iface string, count int, timeout time.Duration) report {
	bus, err := socketcan.New(iface)
	if err != nil {
		return report{Role: "reader", OK: false, Error: err.Error()}
	}
	defer bus.Close()

	ch, err := bus.Subscribe(nil)
	if err != nil {
		return report{Role: "reader", OK: false, Error: err.Error()}
	}

	// See the package doc: this is the earliest point at which a frame sent
	// by another process is guaranteed not to be silently dropped.
	fmt.Fprintln(os.Stderr, "READY")

	received := make([]frameJSON, 0, count)
	deadline := time.After(timeout)
	for len(received) < count {
		select {
		case f, ok := <-ch:
			if !ok {
				return report{
					Role: "reader", OK: false, Received: received,
					Error: "bus closed before all frames received",
				}
			}
			received = append(received, toFrameJSON(f))
		case <-deadline:
			return report{
				Role: "reader", OK: false, Received: received,
				Error: fmt.Sprintf("timed out after %s: got %d/%d frames", timeout, len(received), count),
			}
		}
	}
	return report{Role: "reader", OK: true, Received: received}
}
