// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// The live two-process SocketCAN self-interop test harness: two real,
// independent OS processes of cmd/can-interop-peer (real production
// socketcan.Bus — no test-only shortcuts) bound to the same real Linux
// vcan0 interface, one sending real CAN frames via the RELAY-conformant
// Bus.Send API, the other receiving via Bus.Subscribe and reporting exactly
// what arrived. This is genuine kernel-level CAN traffic between two
// processes, not two Bus values sharing memory in one process's test
// binary — TestSendReceive in bus_linux_test.go already covers that case
// and cannot, by construction, prove that a "go-CAN process" and another
// independent "go-CAN process" actually interoperate over the wire.
//
// Gated behind CAN_INTEROP_TESTS=1 (requireCANInterop) — real subprocess
// spawning plus real SocketCAN I/O is unsuited to the fast default
// `go test ./...` sweep and the existing test-socketcan CI job. Runs in the
// new can-interop CI job (.github/workflows/ci.yml), ubuntu-only.
package socketcan_test

import (
	"testing"
	"time"
)

//fusa:test REQ-SCAN-001
//fusa:test REQ-SCAN-003
//fusa:test REQ-SCAN-004
//fusa:test REQ-SCAN-005
//fusa:test REQ-SCAN-006
//fusa:test REQ-SCAN-007
//fusa:test REQ-SCAN-008
func TestTwoProcessSelfInterop(t *testing.T) {
	requireCANInterop(t)
	const iface = "vcan0"

	reader := startPeer(t, "--role", "reader", "--iface", iface, "--timeout-secs", "10")
	reader.waitReady(t, 5*time.Second)

	writer := startPeer(t, "--role", "writer", "--iface", iface)
	writerReport := writer.waitDone(t, 10*time.Second)
	if !writerReport.OK {
		t.Fatalf("writer process reported failure: %+v", writerReport)
	}

	readerReport := reader.waitDone(t, 10*time.Second)
	if !readerReport.OK {
		t.Fatalf("reader process reported failure: %+v", readerReport)
	}

	if len(writerReport.Sent) == 0 {
		t.Fatal("writer reported sending zero frames — the test proved nothing")
	}
	if len(readerReport.Received) != len(writerReport.Sent) {
		t.Fatalf("frame count mismatch: writer process sent %d, reader process received %d\nsent:     %+v\nreceived: %+v",
			len(writerReport.Sent), len(readerReport.Received), writerReport.Sent, readerReport.Received)
	}
	for i, want := range writerReport.Sent {
		got := readerReport.Received[i]
		if got != want {
			t.Errorf("frame %d field mismatch across the two live processes:\n  sent (writer process):     %+v\n  received (reader process): %+v",
				i, want, got)
		}
	}
}

// TestReaderReportsTimeoutWhenNothingArrives is a sanity check on the
// harness itself: a lone reader process with nobody to talk to must report
// ok=false rather than hang or false-positive.
func TestReaderReportsTimeoutWhenNothingArrives(t *testing.T) {
	requireCANInterop(t)

	reader := startPeer(t, "--role", "reader", "--iface", "vcan0", "--count", "1", "--timeout-secs", "1")
	reader.waitReady(t, 5*time.Second)
	rep := reader.waitDone(t, 5*time.Second)

	if rep.OK {
		t.Fatalf("expected the reader process to report failure when nothing ever arrives, got: %+v", rep)
	}
	if len(rep.Received) != 0 {
		t.Fatalf("expected zero frames received, got %d: %+v", len(rep.Received), rep.Received)
	}
	if rep.Error == "" {
		t.Error("expected a non-empty error message")
	}
}
