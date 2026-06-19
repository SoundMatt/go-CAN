// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package isotp_test

import (
	"context"
	"strings"
	"testing"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/isotp"
	"github.com/SoundMatt/go-CAN/virtual"
)

//fusa:test REQ-ISOTP-001
//fusa:test REQ-ISOTP-002
//fusa:test REQ-ISOTP-003
//fusa:test REQ-ISOTP-004
//fusa:test REQ-ISOTP-005
//fusa:test REQ-ISOTP-006
//fusa:test REQ-ISOTP-007
//fusa:test REQ-ISOTP-008
//fusa:test REQ-ISOTP-009
//fusa:test REQ-ISOTP-010
//fusa:test REQ-ISOTP-011
//fusa:test REQ-ISOTP-012
//fusa:test REQ-ISOTP-013
//fusa:test REQ-SEC-002
//fusa:test REQ-SEC-003

func newPair(t *testing.T) (sender, receiver *isotp.Conn) {
	t.Helper()
	b, _ := virtual.New()
	t.Cleanup(func() { b.Close() })

	cfg := isotp.Config{TxID: 0x7E0, RxID: 0x7E8}
	cfgResp := isotp.Config{TxID: 0x7E8, RxID: 0x7E0}

	var err error
	sender, err = isotp.New(b, cfg)
	if err != nil {
		t.Fatalf("New sender: %v", err)
	}
	receiver, err = isotp.New(b, cfgResp)
	if err != nil {
		t.Fatalf("New receiver: %v", err)
	}
	return sender, receiver
}

func TestSingleFrame(t *testing.T) {
	sender, receiver := newPair(t)
	payload := []byte{0x01, 0x02, 0x03, 0x04}

	done := make(chan []byte, 1)
	go func() {
		got, err := receiver.Recv(context.Background())
		if err != nil {
			t.Errorf("Recv: %v", err)
			done <- nil
			return
		}
		done <- got
	}()

	if err := sender.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := <-done
	if string(got) != string(payload) {
		t.Errorf("got %v, want %v", got, payload)
	}
}

func TestMultiFrame(t *testing.T) {
	sender, receiver := newPair(t)
	payload := make([]byte, 100)
	for i := range payload {
		payload[i] = byte(i)
	}

	done := make(chan []byte, 1)
	go func() {
		got, err := receiver.Recv(context.Background())
		if err != nil {
			t.Errorf("Recv: %v", err)
			done <- nil
			return
		}
		done <- got
	}()

	if err := sender.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := <-done
	if string(got) != string(payload) {
		t.Errorf("multiframe round-trip mismatch (len %d vs %d)", len(got), len(payload))
	}
}

func TestSendEmptyError(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()
	conn, _ := isotp.New(b, isotp.Config{TxID: 0x7E0, RxID: 0x7E8})
	if err := conn.Send(context.Background(), nil); err == nil {
		t.Error("Send(nil) should error")
	}
}

func TestSendTooLarge(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()
	conn, _ := isotp.New(b, isotp.Config{TxID: 0x7E0, RxID: 0x7E8})
	if err := conn.Send(context.Background(), make([]byte, 4096)); err == nil {
		t.Error("Send(4096 bytes) should error")
	}
}

func TestRecvTimeout(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	conn, _ := isotp.New(b, isotp.Config{
		TxID:    0x7E0,
		RxID:    0x7E8,
		Timeout: 50 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := conn.Recv(ctx)
	if err == nil {
		t.Error("Recv should error on timeout")
	}
}

// TestRecvOutOfOrderCF verifies that Recv rejects a Consecutive Frame whose
// sequence number does not match the expected value (REQ-ISOTP-012).
func TestRecvOutOfOrderCF(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	// Receiver expects frames on RxID 0x7E0 and sends FC on 0x7E8.
	receiver, err := isotp.New(b, isotp.Config{TxID: 0x7E8, RxID: 0x7E0, Timeout: time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := receiver.Recv(context.Background())
		done <- err
	}()

	ctx := context.Background()
	// First Frame: total length 20, carrying the first 6 payload bytes.
	ff := can.Frame{ID: 0x7E0, Data: []byte{0x10, 20, 1, 2, 3, 4, 5, 6}}
	if err := b.Send(ctx, ff); err != nil {
		t.Fatalf("send FF: %v", err)
	}
	// Consecutive Frame with the WRONG sequence number (2 instead of 1).
	badCF := can.Frame{ID: 0x7E0, Data: []byte{0x22, 7, 8, 9, 10, 11, 12, 13}}
	if err := b.Send(ctx, badCF); err != nil {
		t.Fatalf("send CF: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected out-of-order sequence-number error")
		}
		if !strings.Contains(err.Error(), "unexpected SN") {
			t.Errorf("error = %v, want 'unexpected SN'", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not return")
	}
}

// TestRecvInvalidSingleFrameLength verifies that an SF declaring a length
// larger than the frame payload is rejected.
func TestRecvInvalidSingleFrameLength(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	receiver, _ := isotp.New(b, isotp.Config{TxID: 0x7E8, RxID: 0x7E0, Timeout: time.Second})

	done := make(chan error, 1)
	go func() {
		_, err := receiver.Recv(context.Background())
		done <- err
	}()

	// SF claims 7 bytes but only carries 2.
	bad := can.Frame{ID: 0x7E0, Data: []byte{0x07, 0xAA, 0xBB}}
	if err := b.Send(context.Background(), bad); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected invalid SF length error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not return")
	}
}

// TestRecvUnexpectedFrameType verifies that a Flow Control frame arriving
// where a SF/FF is expected is rejected.
func TestRecvUnexpectedFrameType(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	receiver, _ := isotp.New(b, isotp.Config{TxID: 0x7E8, RxID: 0x7E0, Timeout: time.Second})

	done := make(chan error, 1)
	go func() {
		_, err := receiver.Recv(context.Background())
		done <- err
	}()

	// Flow Control (type 0x30) is not a valid message-initiating frame.
	fc := can.Frame{ID: 0x7E0, Data: []byte{0x30, 0x00, 0x00}}
	if err := b.Send(context.Background(), fc); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected unexpected-frame-type error")
		}
		if !strings.Contains(err.Error(), "unexpected frame type") {
			t.Errorf("error = %v, want 'unexpected frame type'", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not return")
	}
}

// TestRecvEmptyFrame verifies that a zero-length CAN payload is rejected.
func TestRecvEmptyFrame(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()

	receiver, _ := isotp.New(b, isotp.Config{TxID: 0x7E8, RxID: 0x7E0, Timeout: time.Second})

	done := make(chan error, 1)
	go func() {
		_, err := receiver.Recv(context.Background())
		done <- err
	}()

	if err := b.Send(context.Background(), can.Frame{ID: 0x7E0, Data: []byte{}}); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected empty-frame error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not return")
	}
}

// waitTxType reads the next frame on txCh whose type nibble matches want,
// failing on timeout. Returns the matched frame.
func waitTxType(t *testing.T, txCh <-chan can.Frame, want byte) can.Frame {
	t.Helper()
	for {
		select {
		case f := <-txCh:
			if len(f.Data) > 0 && f.Data[0]&0xF0 == want {
				return f
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for frame type 0x%02X", want)
		}
	}
}

// newSenderWithTap creates an ISO-TP sender (tx=0x7E0, rx=0x7E8) and a channel
// that observes every frame the sender transmits.
func newSenderWithTap(t *testing.T) (*isotp.Conn, *virtual.Bus, <-chan can.Frame) {
	t.Helper()
	b, _ := virtual.New()
	t.Cleanup(func() { b.Close() })
	sender, err := isotp.New(b, isotp.Config{TxID: 0x7E0, RxID: 0x7E8, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("New sender: %v", err)
	}
	tx, err := b.Subscribe([]can.Filter{{ID: 0x7E0, Mask: 0x7FF}})
	if err != nil {
		t.Fatalf("Subscribe tap: %v", err)
	}
	return sender, b, tx
}

// TestSendMultiFrameWithSTmin exercises the STmin separation-time path
// (stminToDuration ≤0x7F branch) via a flow control that requests 1 ms spacing.
func TestSendMultiFrameWithSTmin(t *testing.T) {
	sender, b, tx := newSenderWithTap(t)

	payload := make([]byte, 30)
	for i := range payload {
		payload[i] = byte(i)
	}
	done := make(chan error, 1)
	go func() { done <- sender.Send(context.Background(), payload) }()

	waitTxType(t, tx, 0x10) // First Frame
	// FC: continue to send, block size 0 (unlimited), STmin = 1 ms.
	if err := b.Send(context.Background(), can.Frame{ID: 0x7E8, Data: []byte{0x30, 0x00, 0x01}}); err != nil {
		t.Fatalf("send FC: %v", err)
	}
	go func() {
		for range tx {
		}
	}()
	if err := <-done; err != nil {
		t.Fatalf("Send: %v", err)
	}
}

// TestSendMultiFrameMicroSTmin exercises the stminToDuration 0xF1–0xF9
// (100–900 µs) branch and the default (invalid) branch.
func TestSendMultiFrameMicroSTmin(t *testing.T) {
	for _, stmin := range []byte{0xF1, 0x80} { // 0xF1 = 100 µs; 0x80 = reserved → 0
		sender, b, tx := newSenderWithTap(t)
		payload := make([]byte, 20)
		done := make(chan error, 1)
		go func() { done <- sender.Send(context.Background(), payload) }()

		waitTxType(t, tx, 0x10)
		if err := b.Send(context.Background(), can.Frame{ID: 0x7E8, Data: []byte{0x30, 0x00, stmin}}); err != nil {
			t.Fatalf("send FC: %v", err)
		}
		go func() {
			for range tx {
			}
		}()
		if err := <-done; err != nil {
			t.Fatalf("Send (stmin 0x%02X): %v", stmin, err)
		}
	}
}

// TestSendMultiFrameBlockSize exercises the block-size path: the sender must
// wait for a fresh flow control after every BlockSize consecutive frames.
func TestSendMultiFrameBlockSize(t *testing.T) {
	sender, b, tx := newSenderWithTap(t)

	// 24 bytes → FF(6) + CFs of 7,7,4. With block size 2 the third CF lands in
	// a new block, so the transfer ends mid-block (no dangling FC wait).
	payload := make([]byte, 24)
	done := make(chan error, 1)
	go func() { done <- sender.Send(context.Background(), payload) }()

	waitTxType(t, tx, 0x10) // FF
	// First FC: block size 2.
	b.Send(context.Background(), can.Frame{ID: 0x7E8, Data: []byte{0x30, 0x02, 0x00}})
	waitTxType(t, tx, 0x20) // CF 1
	waitTxType(t, tx, 0x20) // CF 2 — sender now waits for a new FC
	// Second FC: another block of 2 (only one CF remains).
	b.Send(context.Background(), can.Frame{ID: 0x7E8, Data: []byte{0x30, 0x02, 0x00}})
	go func() {
		for range tx {
		}
	}()
	if err := <-done; err != nil {
		t.Fatalf("Send: %v", err)
	}
}

// TestSendMultiFrameOverflow verifies a flow control with overflow status
// aborts the transfer with an error.
func TestSendMultiFrameOverflow(t *testing.T) {
	sender, b, tx := newSenderWithTap(t)

	payload := make([]byte, 20)
	done := make(chan error, 1)
	go func() { done <- sender.Send(context.Background(), payload) }()

	waitTxType(t, tx, 0x10) // FF
	// FC with overflow status (0x02).
	b.Send(context.Background(), can.Frame{ID: 0x7E8, Data: []byte{0x32, 0x00, 0x00}})

	if err := <-done; err == nil {
		t.Fatal("expected overflow error")
	}
}

// TestSendMultiFrameFlowControlWait exercises the wait (0x01) flow-control
// status, which makes the sender wait for a follow-up FC before continuing.
func TestSendMultiFrameFlowControlWait(t *testing.T) {
	sender, b, tx := newSenderWithTap(t)

	payload := make([]byte, 20)
	done := make(chan error, 1)
	go func() { done <- sender.Send(context.Background(), payload) }()

	waitTxType(t, tx, 0x10) // FF
	// FC wait, then FC continue.
	b.Send(context.Background(), can.Frame{ID: 0x7E8, Data: []byte{0x31, 0x00, 0x00}})
	b.Send(context.Background(), can.Frame{ID: 0x7E8, Data: []byte{0x30, 0x00, 0x00}})
	go func() {
		for range tx {
		}
	}()
	if err := <-done; err != nil {
		t.Fatalf("Send: %v", err)
	}
}

// TestSendFlowControlTimeout verifies the sender times out when no flow control
// arrives after the First Frame.
func TestSendFlowControlTimeout(t *testing.T) {
	b, _ := virtual.New()
	defer b.Close()
	sender, _ := isotp.New(b, isotp.Config{TxID: 0x7E0, RxID: 0x7E8, Timeout: 50 * time.Millisecond})

	if err := sender.Send(context.Background(), make([]byte, 20)); err == nil {
		t.Fatal("expected flow-control timeout error")
	}
}

// TestNewSubscribeError verifies New surfaces a Subscribe failure (closed bus).
func TestNewSubscribeError(t *testing.T) {
	b, _ := virtual.New()
	b.Close()
	if _, err := isotp.New(b, isotp.Config{TxID: 0x7E0, RxID: 0x7E8}); err == nil {
		t.Error("expected New to fail on a closed bus")
	}
}
