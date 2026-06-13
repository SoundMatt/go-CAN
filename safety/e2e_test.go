// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safety_test

import (
	"testing"

	"github.com/SoundMatt/go-CAN/safety"
)

//fusa:test REQ-SAFETY-003
//fusa:test REQ-SAFETY-004
//fusa:test REQ-SAFETY-005

func TestRoundTrip(t *testing.T) {
	cfg := safety.Config{DataID: 0x0001, SourceID: 0x0010}
	p := safety.NewProtector(cfg)
	r := safety.NewReceiver(cfg)

	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	protected := p.Protect(payload)

	got, err := r.Unwrap(protected)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("payload mismatch: got %v, want %v", got, payload)
	}
}

func TestMultipleFramesInOrder(t *testing.T) {
	cfg := safety.Config{DataID: 0x0001, SourceID: 0x0010}
	p := safety.NewProtector(cfg)
	r := safety.NewReceiver(cfg)

	for i := 0; i < 10; i++ {
		payload := []byte{byte(i), byte(i + 1)}
		protected := p.Protect(payload)
		got, err := r.Unwrap(protected)
		if err != nil {
			t.Fatalf("frame %d Unwrap: %v", i, err)
		}
		if string(got) != string(payload) {
			t.Errorf("frame %d: got %v, want %v", i, got, payload)
		}
	}
}

func TestCRCDetection(t *testing.T) {
	cfg := safety.Config{DataID: 0x0001, SourceID: 0x0010}
	p := safety.NewProtector(cfg)
	r := safety.NewReceiver(cfg)

	protected := p.Protect([]byte{0x01, 0x02})
	// Corrupt the CRC bytes (bytes 8–9)
	protected[8] ^= 0xFF
	protected[9] ^= 0xFF

	_, err := r.Unwrap(protected)
	if err == nil {
		t.Error("expected CRC error, got nil")
	}
	e2eErr, ok := err.(*safety.E2EError)
	if !ok || e2eErr.Kind != safety.ErrCRCMismatch {
		t.Errorf("expected ErrCRCMismatch, got %v", err)
	}
}

func TestSequenceGapDetection(t *testing.T) {
	cfg := safety.Config{DataID: 0x0001, SourceID: 0x0010}
	p := safety.NewProtector(cfg)
	r := safety.NewReceiver(cfg)

	// Unwrap seq=0
	if _, err := r.Unwrap(p.Protect([]byte{0x01})); err != nil {
		t.Fatalf("frame 0 Unwrap: %v", err)
	}
	// Skip seq=1 by protecting but not unwrapping
	_ = p.Protect([]byte{0x02})
	// Present seq=2 — receiver expects seq=1, so this is a gap
	_, err := r.Unwrap(p.Protect([]byte{0x03}))
	if err == nil {
		t.Error("expected sequence gap error, got nil")
	}
	e2eErr, ok := err.(*safety.E2EError)
	if !ok || e2eErr.Kind != safety.ErrSequenceGap {
		t.Errorf("expected ErrSequenceGap, got %v", err)
	}
}

func TestHeaderTooShort(t *testing.T) {
	cfg := safety.Config{DataID: 0x01}
	r := safety.NewReceiver(cfg)
	_, err := r.Unwrap([]byte{0x01, 0x02})
	if err == nil {
		t.Error("expected ErrHeaderTooShort")
	}
	e2eErr, ok := err.(*safety.E2EError)
	if !ok || e2eErr.Kind != safety.ErrHeaderTooShort {
		t.Errorf("expected ErrHeaderTooShort, got %v", err)
	}
}

func TestConcurrentProtect(t *testing.T) {
	cfg := safety.Config{DataID: 0x01, SourceID: 0x01}
	p := safety.NewProtector(cfg)
	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			p.Protect([]byte{byte(i)})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
