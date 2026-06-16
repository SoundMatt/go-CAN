// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package can_test

import (
	"context"
	"errors"
	"testing"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/virtual"
	relay "github.com/SoundMatt/RELAY"
)

func TestToMessage(t *testing.T) {
	f := can.Frame{
		ID:   0x7FF,
		Ext:  false,
		FD:   true,
		BRS:  true,
		Data: []byte{0x01, 0x02, 0x03},
	}
	msg := f.ToMessage()

	if msg.Protocol != relay.CAN {
		t.Errorf("Protocol: got %v, want %v", msg.Protocol, relay.CAN)
	}
	if msg.ID != "2047" { // 0x7FF == 2047
		t.Errorf("ID: got %q, want %q", msg.ID, "2047")
	}
	if string(msg.Payload) != string(f.Data) {
		t.Errorf("Payload mismatch")
	}
	if msg.Meta["can.fd"] != "true" {
		t.Errorf("meta can.fd: got %q, want %q", msg.Meta["can.fd"], "true")
	}
	if msg.Meta["can.brs"] != "true" {
		t.Errorf("meta can.brs: got %q, want %q", msg.Meta["can.brs"], "true")
	}
	if msg.Meta["can.ext"] != "false" {
		t.Errorf("meta can.ext: got %q, want %q", msg.Meta["can.ext"], "false")
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestFromMessage(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		orig := can.Frame{
			ID:   0x1FFFFFFF,
			Ext:  true,
			FD:   true,
			BRS:  true,
			Data: []byte{0xDE, 0xAD},
		}
		msg := orig.ToMessage()
		got, err := can.FromMessage(msg)
		if err != nil {
			t.Fatalf("FromMessage: %v", err)
		}
		if got.ID != orig.ID {
			t.Errorf("ID: got 0x%X, want 0x%X", got.ID, orig.ID)
		}
		if got.Ext != orig.Ext {
			t.Errorf("Ext: got %v, want %v", got.Ext, orig.Ext)
		}
		if got.FD != orig.FD {
			t.Errorf("FD: got %v, want %v", got.FD, orig.FD)
		}
		if got.BRS != orig.BRS {
			t.Errorf("BRS: got %v, want %v", got.BRS, orig.BRS)
		}
		if string(got.Data) != string(orig.Data) {
			t.Errorf("Data: got %v, want %v", got.Data, orig.Data)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		msg := relay.Message{Protocol: relay.CAN, ID: "not-a-number"}
		_, err := can.FromMessage(msg)
		if err == nil {
			t.Error("expected error for invalid ID")
		}
	})
}

func TestAdapt(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	node := can.Adapt(b)

	if node.Protocol() != relay.CAN {
		t.Errorf("Protocol: got %v, want %v", node.Protocol(), relay.CAN)
	}

	ch, err := node.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Send a frame via the adapter
	msg := can.Frame{ID: 0x100, Data: []byte{0xAB, 0xCD}}.ToMessage()
	if err := node.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case got, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if got.Protocol != relay.CAN {
			t.Errorf("Protocol: got %v, want %v", got.Protocol, relay.CAN)
		}
		if got.ID != "256" { // 0x100 == 256
			t.Errorf("ID: got %q, want %q", got.ID, "256")
		}
		if got.Seq != 1 {
			t.Errorf("Seq: got %d, want 1", got.Seq)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for relay.Message")
	}
}

func TestAdaptClose(t *testing.T) {
	b, _ := virtual.New()
	node := can.Adapt(b)

	if err := node.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestErrClosed(t *testing.T) {
	b, _ := virtual.New()
	b.Close()

	err := b.Send(context.Background(), can.Frame{ID: 0x100})
	if !errors.Is(err, can.ErrClosed) {
		t.Errorf("Send on closed bus: errors.Is(err, ErrClosed) = false, err = %v", err)
	}

	_, err = b.Subscribe(nil)
	if !errors.Is(err, can.ErrClosed) {
		t.Errorf("Subscribe on closed bus: errors.Is(err, ErrClosed) = false, err = %v", err)
	}
}
