// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual_test

import (
	"context"
	"testing"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/virtual"
)

//fusa:test REQ-VIRT-001
//fusa:test REQ-VIRT-002
//fusa:test REQ-VIRT-003
//fusa:test REQ-VIRT-004
//fusa:test REQ-VIRT-005

func TestNew(t *testing.T) {
	b, err := virtual.New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer b.Close()
}

func TestSendReceive(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	ch, err := b.Subscribe(nil)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	f := can.Frame{ID: 0x100, Data: []byte{0xDE, 0xAD}}
	if err := b.Send(context.Background(), f); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case got := <-ch:
		if got.ID != f.ID || string(got.Data) != string(f.Data) {
			t.Errorf("got %+v, want %+v", got, f)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for frame")
	}
}

func TestFilterAccepts(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	ch, _ := b.Subscribe([]can.Filter{{ID: 0x200, Mask: 0x7FF}})

	_ = b.Send(context.Background(), can.Frame{ID: 0x100, Data: []byte{1}})
	_ = b.Send(context.Background(), can.Frame{ID: 0x200, Data: []byte{2}})

	select {
	case got := <-ch:
		if got.ID != 0x200 {
			t.Errorf("expected ID 0x200, got 0x%X", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for frame")
	}

	// 0x100 should not arrive
	select {
	case got := <-ch:
		t.Errorf("unexpected frame: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	ch1, _ := b.Subscribe(nil)
	ch2, _ := b.Subscribe(nil)

	f := can.Frame{ID: 0x300, Data: []byte{0x42}}
	_ = b.Send(context.Background(), f)

	for i, ch := range []<-chan can.Frame{ch1, ch2} {
		select {
		case got := <-ch:
			if got.ID != f.ID {
				t.Errorf("subscriber %d: got ID 0x%X, want 0x%X", i, got.ID, f.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timeout", i)
		}
	}
}

func TestClosedBus(t *testing.T) {
	b, _ := virtual.New()
	b.Close()

	if err := b.Send(context.Background(), can.Frame{ID: 0x100}); err == nil {
		t.Error("Send on closed bus should error")
	}
	if _, err := b.Subscribe(nil); err == nil {
		t.Error("Subscribe on closed bus should error")
	}
}

func TestDoubleClose(t *testing.T) {
	b, _ := virtual.New()
	b.Close()
	if err := b.Close(); err != nil {
		t.Errorf("second Close should not error: %v", err)
	}
}

func TestInvalidFrame(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	// Standard ID too large
	err := b.Send(context.Background(), can.Frame{ID: 0x800})
	if err == nil {
		t.Error("expected error for invalid frame")
	}
}

func FuzzSend(f *testing.F) {
	f.Add(uint32(0x100), false, false, []byte{1, 2, 3})
	f.Add(uint32(0x1FFFFFFF), true, false, []byte{0xFF})
	f.Fuzz(func(t *testing.T, id uint32, ext, rtr bool, data []byte) {
		b, _ := virtual.New()
		defer b.Close()

		ch, _ := b.Subscribe(nil)
		fr := can.Frame{ID: id, Ext: ext, RTR: rtr}
		if !rtr && len(data) <= 8 {
			fr.Data = data
		}

		_ = b.Send(context.Background(), fr)
		select {
		case <-ch:
		default:
		}
	})
}
