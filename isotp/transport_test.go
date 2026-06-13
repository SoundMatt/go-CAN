// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package isotp_test

import (
	"context"
	"testing"
	"time"

	"github.com/SoundMatt/go-CAN/isotp"
	"github.com/SoundMatt/go-CAN/virtual"
)

//fusa:test REQ-ISOTP-003
//fusa:test REQ-ISOTP-004

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
