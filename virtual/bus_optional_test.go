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

func TestLoan(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	lf, err := b.Loan()
	if err != nil {
		t.Fatalf("Loan() error: %v", err)
	}
	if lf == nil {
		t.Fatal("Loan() returned nil")
	}

	// Assign a frame and verify the field is accessible.
	lf.Frame = can.Frame{ID: 0x100, Data: []byte{0x01}}
	if lf.Frame.ID != 0x100 {
		t.Errorf("frame ID not preserved: got 0x%X", lf.Frame.ID)
	}

	// Return() must not panic.
	lf.Return()

	// Double Return() must not panic.
	lf.Return()
}

func TestLoanOnClosedBus(t *testing.T) {
	b, _ := virtual.New()
	b.Close()

	_, err := b.Loan()
	if err == nil {
		t.Error("Loan() on closed bus should return error")
	}
}

func TestSendLoaned(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	ch, err := b.Subscribe()
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	lf, err := b.Loan()
	if err != nil {
		t.Fatalf("Loan: %v", err)
	}
	lf.Frame = can.Frame{ID: 0x200, Data: []byte{0xAB, 0xCD}}

	if err := b.SendLoaned(context.Background(), lf); err != nil {
		t.Fatalf("SendLoaned: %v", err)
	}

	select {
	case got := <-ch:
		if got.ID != 0x200 {
			t.Errorf("got ID 0x%X, want 0x200", got.ID)
		}
		if len(got.Data) != 2 || got.Data[0] != 0xAB || got.Data[1] != 0xCD {
			t.Errorf("got data %v, want [0xAB 0xCD]", got.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for loaned frame")
	}
}

func TestHealth(t *testing.T) {
	b, _ := virtual.New()

	h := b.Health()
	if h.Status != can.HealthOK {
		t.Errorf("expected HealthOK on open bus, got %v", h.Status)
	}

	b.Close()

	h = b.Health()
	if h.Status != can.HealthDown {
		t.Errorf("expected HealthDown after Close, got %v", h.Status)
	}
	if h.Details == "" {
		t.Error("expected non-empty Details on HealthDown")
	}
}

func TestMetrics(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	ch, _ := b.Subscribe()

	f := can.Frame{ID: 0x300, Data: []byte{0x01, 0x02, 0x03}}
	if err := b.Send(context.Background(), f); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Drain the subscriber so DeliverCount increments.
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for frame")
	}

	m := b.Metrics()
	if m.WriteCount != 1 {
		t.Errorf("WriteCount: got %d, want 1", m.WriteCount)
	}
	if m.DeliverCount != 1 {
		t.Errorf("DeliverCount: got %d, want 1", m.DeliverCount)
	}
	if m.BytesWritten != 3 {
		t.Errorf("BytesWritten: got %d, want 3", m.BytesWritten)
	}
	if m.BytesDelivered != 3 {
		t.Errorf("BytesDelivered: got %d, want 3", m.BytesDelivered)
	}
}

func TestMetricsDropCount(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	// Subscribe but never read — channel will fill and frames will be dropped.
	ch, _ := b.Subscribe()
	_ = ch

	// Send enough frames to overflow the channel buffer (defaultChanSize = 64).
	f := can.Frame{ID: 0x400, Data: []byte{0xFF}}
	drops := 0
	for i := 0; i < 128; i++ {
		_ = b.Send(context.Background(), f)
	}

	m := b.Metrics()
	if m.WriteCount != 128 {
		t.Errorf("WriteCount: got %d, want 128", m.WriteCount)
	}
	drops = int(m.DropCount)
	if drops == 0 {
		t.Error("expected some drops when channel buffer is full")
	}
	if m.DeliverCount+m.DropCount != 128 {
		t.Errorf("DeliverCount(%d) + DropCount(%d) != 128", m.DeliverCount, m.DropCount)
	}
}

func TestCloseWithDrain(t *testing.T) {
	b, _ := virtual.New()

	ch, _ := b.Subscribe()

	// Send a frame, then drain it in a goroutine slightly after.
	f := can.Frame{ID: 0x500, Data: []byte{0x42}}
	_ = b.Send(context.Background(), f)

	// Drain in a goroutine.
	go func() {
		<-ch
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := b.CloseWithDrain(ctx); err != nil {
		t.Fatalf("CloseWithDrain: %v", err)
	}

	// Verify bus is actually closed.
	if err := b.Send(context.Background(), f); err == nil {
		t.Error("Send after CloseWithDrain should error")
	}
}

func TestCloseWithDrainContextCancel(t *testing.T) {
	b, _ := virtual.New()

	// Subscribe but never drain — CloseWithDrain should respect ctx cancellation.
	ch, _ := b.Subscribe()
	_ = ch // intentionally never read

	// Send a frame so the channel is non-empty.
	_ = b.Send(context.Background(), can.Frame{ID: 0x600})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should return after ctx expires, not hang forever.
	start := time.Now()
	_ = b.CloseWithDrain(ctx)
	if time.Since(start) > time.Second {
		t.Error("CloseWithDrain did not respect context deadline")
	}
}
