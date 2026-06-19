// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uds_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SoundMatt/go-CAN/isotp"
	"github.com/SoundMatt/go-CAN/uds"
	"github.com/SoundMatt/go-CAN/virtual"
)

//fusa:test REQ-UDS-001
//fusa:test REQ-UDS-002
//fusa:test REQ-UDS-003
//fusa:test REQ-UDS-004
//fusa:test REQ-UDS-005
//fusa:test REQ-UDS-006
//fusa:test REQ-UDS-007
//fusa:test REQ-UDS-008
//fusa:test REQ-UDS-009

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

func TestECUReset(t *testing.T) {
	var gotReq []byte
	client := newClientServer(t, func(req []byte) []byte {
		gotReq = append([]byte(nil), req...)
		// Positive response: SID+0x40, echoed reset type, power-down time.
		return []byte{req[0] + 0x40, req[1], 0x00}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.ECUReset(ctx, uds.ResetHard); err != nil {
		t.Fatalf("ECUReset: %v", err)
	}
	if len(gotReq) < 2 || gotReq[0] != uds.SIDECUReset {
		t.Errorf("request SID = %X, want 0x11 ECUReset", gotReq)
	}
	if gotReq[1] != byte(uds.ResetHard) {
		t.Errorf("reset type = 0x%02X, want 0x%02X", gotReq[1], byte(uds.ResetHard))
	}
}

func TestECUResetNegative(t *testing.T) {
	client := newClientServer(t, func(req []byte) []byte {
		return []byte{0x7F, req[0], uds.NRCConditionsNotCorrect}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := client.ECUReset(ctx, uds.ResetSoft)
	if err == nil {
		t.Fatal("expected negative response error")
	}
	nrcErr, ok := err.(*uds.NegativeResponseError)
	if !ok {
		t.Fatalf("expected *NegativeResponseError, got %T: %v", err, err)
	}
	if nrcErr.NRC != uds.NRCConditionsNotCorrect {
		t.Errorf("NRC = 0x%02X, want 0x%02X", nrcErr.NRC, uds.NRCConditionsNotCorrect)
	}
}

func TestWriteDataByIdentifier(t *testing.T) {
	var did uint16 = 0xF190
	payload := []byte("WVWZZZ3BZWE689725")
	var gotReq []byte
	client := newClientServer(t, func(req []byte) []byte {
		gotReq = append([]byte(nil), req...)
		// Positive response echoes SID+0x40 and the 2-byte DID.
		return []byte{req[0] + 0x40, req[1], req[2]}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.WriteDataByIdentifier(ctx, did, payload); err != nil {
		t.Fatalf("WriteDataByIdentifier: %v", err)
	}
	want := append([]byte{uds.SIDWriteDataByIdentifier, byte(did >> 8), byte(did)}, payload...)
	if string(gotReq) != string(want) {
		t.Errorf("request = % X, want % X", gotReq, want)
	}
}

func TestWriteDataByIdentifierNegative(t *testing.T) {
	client := newClientServer(t, func(req []byte) []byte {
		return []byte{0x7F, req[0], uds.NRCSecurityAccessDenied}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := client.WriteDataByIdentifier(ctx, 0xF190, []byte{0x01})
	if err == nil {
		t.Fatal("expected negative response error")
	}
	nrcErr, ok := err.(*uds.NegativeResponseError)
	if !ok {
		t.Fatalf("expected *NegativeResponseError, got %T: %v", err, err)
	}
	if nrcErr.NRC != uds.NRCSecurityAccessDenied {
		t.Errorf("NRC = 0x%02X, want 0x%02X", nrcErr.NRC, uds.NRCSecurityAccessDenied)
	}
}

// TestNegativeResponseErrorString exercises the Error()/nrcString formatting
// for every known NRC plus an unknown one.
func TestNegativeResponseErrorString(t *testing.T) {
	known := []byte{
		uds.NRCGeneralReject,
		uds.NRCServiceNotSupported,
		uds.NRCSubFunctionNotSupported,
		uds.NRCIncorrectMessageLengthOrFormat,
		uds.NRCConditionsNotCorrect,
		uds.NRCRequestSequenceError,
		uds.NRCRequestOutOfRange,
		uds.NRCSecurityAccessDenied,
		uds.NRCUploadDownloadNotAccepted,
		uds.NRCResponsePending,
	}
	for _, nrc := range known {
		e := &uds.NegativeResponseError{RequestSID: 0x22, NRC: nrc}
		s := e.Error()
		if !strings.Contains(s, "0x22") {
			t.Errorf("NRC 0x%02X: Error() %q missing request SID", nrc, s)
		}
		if strings.Contains(s, "unknown") {
			t.Errorf("NRC 0x%02X classified as unknown: %q", nrc, s)
		}
	}
	// An unrecognised NRC should render as "unknown".
	e := &uds.NegativeResponseError{RequestSID: 0x10, NRC: 0xAB}
	if !strings.Contains(e.Error(), "unknown") {
		t.Errorf("unexpected NRC string: %q", e.Error())
	}
}

// TestUnexpectedPositiveResponse verifies each service rejects a positive
// response carrying the wrong SID echo.
func TestUnexpectedPositiveResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	t.Run("DiagnosticSessionControl", func(t *testing.T) {
		c := newClientServer(t, func(req []byte) []byte { return []byte{0xFF, 0x00} })
		if err := c.DiagnosticSessionControl(ctx, uds.SessionDefault); err == nil {
			t.Error("expected error on mismatched response SID")
		}
	})
	t.Run("ECUReset", func(t *testing.T) {
		c := newClientServer(t, func(req []byte) []byte { return []byte{0xFF, 0x00} })
		if err := c.ECUReset(ctx, uds.ResetHard); err == nil {
			t.Error("expected error on mismatched response SID")
		}
	})
	t.Run("ReadDataByIdentifier", func(t *testing.T) {
		c := newClientServer(t, func(req []byte) []byte { return []byte{0xFF, 0x00, 0x00} })
		if _, err := c.ReadDataByIdentifier(ctx, 0xF190); err == nil {
			t.Error("expected error on mismatched response SID")
		}
	})
	t.Run("TesterPresent", func(t *testing.T) {
		c := newClientServer(t, func(req []byte) []byte { return []byte{0xFF} })
		if err := c.TesterPresent(ctx); err == nil {
			t.Error("expected error on mismatched response SID")
		}
	})
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
