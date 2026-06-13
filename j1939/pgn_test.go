// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package j1939_test

import (
	"context"
	"testing"
	"time"

	"github.com/SoundMatt/go-CAN/j1939"
	"github.com/SoundMatt/go-CAN/virtual"
)

//fusa:test REQ-J1939-001
//fusa:test REQ-J1939-002
//fusa:test REQ-J1939-003
//fusa:test REQ-J1939-004
//fusa:test REQ-J1939-005
//fusa:test REQ-J1939-006

func TestDecodeEncodeRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		priority j1939.Priority
		pgn      j1939.PGN
		src      byte
	}{
		{"broadcast PGN 0xFECA (CCVS)", 6, 0x0FECA, 0x00},
		{"peer-to-peer PF=0xE8", 6, 0x00E800, 0x01},
		{"high priority", 0, 0x0FF00, 0xFE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := j1939.EncodeID(tt.priority, tt.pgn, tt.src)
			p2, pgn2, src2 := j1939.DecodeID(id)
			if p2 != tt.priority {
				t.Errorf("priority: got %d, want %d", p2, tt.priority)
			}
			if pgn2 != tt.pgn {
				t.Errorf("pgn: got 0x%X, want 0x%X", pgn2, tt.pgn)
			}
			if src2 != tt.src {
				t.Errorf("src: got 0x%X, want 0x%X", src2, tt.src)
			}
		})
	}
}

func TestIsPeerToPeer(t *testing.T) {
	// PF=0xE8 (<240) → peer-to-peer
	if !j1939.PGN(0x00E800).IsPeerToPeer() {
		t.Error("PGN with PF<240 should be peer-to-peer")
	}
	// PF=0xFE (≥240) → broadcast
	if j1939.PGN(0x0FECA).IsPeerToPeer() {
		t.Error("PGN with PF≥240 should be broadcast")
	}
}

func TestBusSendReceive(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	sender := j1939.NewBus(b, 0x00)
	receiver := j1939.NewBus(b, 0x01)

	const testPGN = j1939.PGN(0x0FECA)
	ch, err := receiver.Subscribe(testPGN)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	payload := []byte{0x11, 0x22, 0x33, 0x44}
	if err := sender.Send(context.Background(), j1939.Frame{
		Priority: 6,
		PGN:      testPGN,
		Data:     payload,
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case got := <-ch:
		if string(got.Data) != string(payload) {
			t.Errorf("data mismatch: got %v, want %v", got.Data, payload)
		}
		if got.PGN != testPGN {
			t.Errorf("PGN mismatch: got 0x%X, want 0x%X", got.PGN, testPGN)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for J1939 frame")
	}
}
