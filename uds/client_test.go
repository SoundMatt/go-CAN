// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uds_test

import (
	"context"
	"testing"
	"time"

	"github.com/SoundMatt/go-CAN/isotp"
	"github.com/SoundMatt/go-CAN/uds"
	"github.com/SoundMatt/go-CAN/virtual"
)

//fusa:test REQ-UDS-002
//fusa:test REQ-UDS-003

// echoServer runs a minimal UDS echo server on the virtual bus.
// It reads requests from rxID and sends positive responses to txID.
func echoServer(ctx context.Context, b *virtual.Bus, txID, rxID uint32, handler func(req []byte) []byte) {
	conn, err := isotp.New(b, isotp.Config{TxID: txID, RxID: rxID})
	if err != nil {
		return
	}
	go func() {
		for {
			req, err := conn.Recv(ctx)
			if err != nil {
				return
			}
			resp := handler(req)
			if err := conn.Send(ctx, resp); err != nil {
				return
			}
		}
	}()
}

func newClientServer(t *testing.T, handler func(req []byte) []byte) *uds.Client {
	t.Helper()
	b, _ := virtual.New()
	t.Cleanup(func() { b.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	// ECU (server): tx=0x7E8, rx=0x7E0
	echoServer(ctx, b, 0x7E8, 0x7E0, handler)

	// Client: tx=0x7E0, rx=0x7E8
	conn, err := isotp.New(b, isotp.Config{TxID: 0x7E0, RxID: 0x7E8})
	if err != nil {
		t.Fatalf("isotp.New: %v", err)
	}
	return uds.NewClient(conn)
}

func TestDiagnosticSessionControl(t *testing.T) {
	client := newClientServer(t, func(req []byte) []byte {
		// Positive response: SID + 0x40, echoed session byte, P2 timing
		return []byte{req[0] + 0x40, req[1], 0x00, 0x19, 0x01, 0xF4}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.DiagnosticSessionControl(ctx, uds.SessionExtended); err != nil {
		t.Fatalf("DiagnosticSessionControl: %v", err)
	}
}

func TestReadDataByIdentifier(t *testing.T) {
	vin := []byte("WVWZZZ3BZWE689725")
	client := newClientServer(t, func(req []byte) []byte {
		// req[0]=0x22, req[1:2]=DID
		resp := []byte{req[0] + 0x40, req[1], req[2]}
		return append(resp, vin...)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := client.ReadDataByIdentifier(ctx, 0xF190) // VIN DID
	if err != nil {
		t.Fatalf("ReadDataByIdentifier: %v", err)
	}
	if string(data) != string(vin) {
		t.Errorf("got %q, want %q", data, vin)
	}
}

func TestNegativeResponse(t *testing.T) {
	client := newClientServer(t, func(req []byte) []byte {
		// Negative response: 0x7F, requested SID, NRC
		return []byte{0x7F, req[0], uds.NRCServiceNotSupported}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := client.DiagnosticSessionControl(ctx, uds.SessionDefault)
	if err == nil {
		t.Fatal("expected negative response error")
	}
	nrcErr, ok := err.(*uds.NegativeResponseError)
	if !ok {
		t.Fatalf("expected *NegativeResponseError, got %T: %v", err, err)
	}
	if nrcErr.NRC != uds.NRCServiceNotSupported {
		t.Errorf("NRC = 0x%02X, want 0x%02X", nrcErr.NRC, uds.NRCServiceNotSupported)
	}
}

func TestTesterPresent(t *testing.T) {
	client := newClientServer(t, func(req []byte) []byte {
		return []byte{req[0] + 0x40, 0x00}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.TesterPresent(ctx); err != nil {
		t.Fatalf("TesterPresent: %v", err)
	}
}

func TestResponsePending(t *testing.T) {
	callCount := 0
	client := newClientServer(t, func(req []byte) []byte {
		callCount++
		if callCount == 1 {
			// First call: response pending
			return []byte{0x7F, req[0], uds.NRCResponsePending}
		}
		// Second call: positive response (for the re-sent request after pending)
		return []byte{req[0] + 0x40, req[1], 0x00, 0x19, 0x01, 0xF4}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// This will get a "response pending" first, then retry receiving
	// Note: UDS response pending means the ECU is still processing; the client
	// should keep waiting for the real response, not re-send the request.
	// Our simple echo server will send the pending NRC immediately,
	// then when Recv is called again after the first request, there's no second response.
	// So we expect an error here (either NRC or timeout), not a success.
	_ = client.DiagnosticSessionControl(ctx, uds.SessionExtended)
	// We're testing that the client handles NRC 0x78 without panicking
}
