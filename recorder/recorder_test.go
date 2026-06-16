// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package recorder_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/recorder"
	"github.com/SoundMatt/go-CAN/virtual"
)

// TestRecord sends 3 known frames to a virtual bus, records them, and
// verifies the output lines contain the expected IDs and data.
func TestRecord(t *testing.T) {
	bus, err := virtual.New()
	if err != nil {
		t.Fatalf("virtual.New: %v", err)
	}

	frames := []can.Frame{
		{ID: 0x123, Data: []byte{0x01, 0x02, 0x03}},
		{ID: 0x456, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{ID: 0x1FFFFFFF, Ext: true, Data: []byte{0xCA, 0xFE}},
	}

	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())

	// Start recording in background.
	done := make(chan error, 1)
	go func() {
		done <- recorder.Record(ctx, bus, &buf, "vcan0")
	}()

	// Give the subscription a moment to register, then send frames.
	time.Sleep(10 * time.Millisecond)
	for _, f := range frames {
		if err := bus.Send(context.Background(), f); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	// Give Record time to write all frames before cancelling.
	time.Sleep(20 * time.Millisecond)
	cancel()

	if err := <-done; err != nil && err != context.Canceled {
		t.Fatalf("Record returned unexpected error: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), output)
	}

	expects := []struct {
		iface string
		id    string
		data  string
	}{
		{"vcan0", "123", "010203"},
		{"vcan0", "456", "DEADBEEF"},
		{"vcan0", "1FFFFFFF", "CAFE"},
	}

	for i, ex := range expects {
		line := lines[i]
		if !strings.Contains(line, ex.iface) {
			t.Errorf("line %d: expected iface %q in %q", i, ex.iface, line)
		}
		if !strings.Contains(strings.ToUpper(line), strings.ToUpper(ex.id)+"#") {
			t.Errorf("line %d: expected ID %q in %q", i, ex.id, line)
		}
		if !strings.Contains(strings.ToUpper(line), strings.ToUpper(ex.data)) {
			t.Errorf("line %d: expected data %q in %q", i, ex.data, line)
		}
	}
}

// TestReplay writes candump lines to a reader, replays to a virtual bus,
// and verifies the received frames match.
func TestReplay(t *testing.T) {
	log := "(1609459200.000000) vcan0 123#0102030405060708\n" +
		"(1609459200.001000) vcan0 456#DEADBEEF\n" +
		"(1609459200.002000) vcan0 1FFFFFFF#CAFE\n"

	bus, err := virtual.New()
	if err != nil {
		t.Fatalf("virtual.New: %v", err)
	}

	ch, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx := context.Background()
	if err := recorder.Replay(ctx, bus, strings.NewReader(log), 100.0); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	want := []can.Frame{
		{ID: 0x123, Data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}},
		{ID: 0x456, Ext: true, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		{ID: 0x1FFFFFFF, Ext: true, Data: []byte{0xCA, 0xFE}},
	}

	for i, wf := range want {
		select {
		case got := <-ch:
			if got.ID != wf.ID {
				t.Errorf("frame %d: ID got %X want %X", i, got.ID, wf.ID)
			}
			if !bytes.Equal(got.Data, wf.Data) {
				t.Errorf("frame %d: Data got %X want %X", i, got.Data, wf.Data)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("frame %d: timed out waiting for frame", i)
		}
	}
}

// TestParseLineRoundtrip checks that FormatLine → ParseLine round-trips correctly.
func TestParseLineRoundtrip(t *testing.T) {
	cases := []struct {
		name  string
		iface string
		ts    time.Time
		frame can.Frame
	}{
		{
			name:  "standard frame",
			iface: "vcan0",
			ts:    time.Unix(1609459200, 123456000).UTC(),
			frame: can.Frame{ID: 0x123, Data: []byte{0x01, 0x02, 0x03}},
		},
		{
			name:  "extended frame",
			iface: "can0",
			ts:    time.Unix(1609459200, 50000000).UTC(),
			frame: can.Frame{ID: 0x1FFFFFFF, Ext: true, Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}},
		},
		{
			name:  "CAN FD frame with BRS",
			iface: "vcan0",
			ts:    time.Unix(1000000000, 0).UTC(),
			frame: can.Frame{ID: 0x100, FD: true, BRS: true, Data: []byte{0x11, 0x22, 0x33}},
		},
		{
			name:  "empty data frame",
			iface: "vcan0",
			ts:    time.Unix(1609459200, 0).UTC(),
			frame: can.Frame{ID: 0x7FF},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := recorder.FormatLine(tc.iface, tc.ts, tc.frame)
			gotIface, gotTs, gotFrame, err := recorder.ParseLine(line)
			if err != nil {
				t.Fatalf("ParseLine(%q): %v", line, err)
			}
			if gotIface != tc.iface {
				t.Errorf("iface: got %q want %q", gotIface, tc.iface)
			}
			// Timestamp round-trips to microsecond precision.
			wantUs := tc.ts.UnixMicro()
			gotUs := gotTs.UnixMicro()
			if gotUs != wantUs {
				t.Errorf("ts: got %d want %d (diff %d µs)", gotUs, wantUs, gotUs-wantUs)
			}
			if gotFrame.ID != tc.frame.ID {
				t.Errorf("ID: got %X want %X", gotFrame.ID, tc.frame.ID)
			}
			if gotFrame.FD != tc.frame.FD {
				t.Errorf("FD: got %v want %v", gotFrame.FD, tc.frame.FD)
			}
			if gotFrame.BRS != tc.frame.BRS {
				t.Errorf("BRS: got %v want %v", gotFrame.BRS, tc.frame.BRS)
			}
			if !bytes.Equal(gotFrame.Data, tc.frame.Data) {
				t.Errorf("Data: got %X want %X", gotFrame.Data, tc.frame.Data)
			}
		})
	}
}

// TestReplayTiming verifies that speedFactor=10 makes a 50ms gap play back
// in well under 100ms.
func TestReplayTiming(t *testing.T) {
	log := "(1609459200.000000) vcan0 123#01\n" +
		"(1609459200.050000) vcan0 456#02\n"

	bus, err := virtual.New()
	if err != nil {
		t.Fatalf("virtual.New: %v", err)
	}

	ch, err := bus.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	start := time.Now()
	if err := recorder.Replay(context.Background(), bus, strings.NewReader(log), 10.0); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed >= 100*time.Millisecond {
		t.Errorf("speedFactor=10 should replay 50ms gap in <10ms wall time, got %v", elapsed)
	}

	// Verify both frames arrived.
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("frame %d: timed out", i)
		}
	}
}
