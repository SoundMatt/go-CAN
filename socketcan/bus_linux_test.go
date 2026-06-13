// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socketcan_test

import (
	"context"
	"os"
	"testing"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/socketcan"
)

//fusa:test REQ-SCAN-001
//fusa:test REQ-SCAN-002
//fusa:test REQ-SCAN-003

// requireVCAN skips the test if vcan0 is not available.
// Run: modprobe vcan && ip link add dev vcan0 type vcan && ip link set up vcan0
func requireVCAN(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/sys/class/net/vcan0"); os.IsNotExist(err) {
		t.Skip("vcan0 not available — load vcan module and create vcan0 to run this test")
	}
}

func TestNew(t *testing.T) {
	requireVCAN(t)
	b, err := socketcan.New("vcan0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()
}

func TestSendReceive(t *testing.T) {
	requireVCAN(t)

	b, err := socketcan.New("vcan0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	b2, err := socketcan.New("vcan0")
	if err != nil {
		t.Fatalf("New (receiver): %v", err)
	}
	defer b2.Close()

	ch, err := b2.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	f := can.Frame{ID: 0x123, Data: []byte{0x01, 0x02, 0x03}}
	if err := b.Send(context.Background(), f); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case got := <-ch:
		if got.ID != f.ID {
			t.Errorf("ID: got 0x%X, want 0x%X", got.ID, f.ID)
		}
		if string(got.Data) != string(f.Data) {
			t.Errorf("Data: got %v, want %v", got.Data, f.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for frame")
	}
}

func TestBadInterface(t *testing.T) {
	_, err := socketcan.New("nosuchiface99")
	if err == nil {
		t.Error("expected error for non-existent interface")
	}
}
