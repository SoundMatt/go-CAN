// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package obdii_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/SoundMatt/go-CAN/isotp"
	"github.com/SoundMatt/go-CAN/obdii"
	"github.com/SoundMatt/go-CAN/virtual"
)

//fusa:test REQ-OBD-002
//fusa:test REQ-OBD-003

func ecuServer(ctx context.Context, b *virtual.Bus, handler func(req []byte) []byte) {
	conn, err := isotp.New(b, isotp.Config{TxID: 0x7E8, RxID: 0x7DF})
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

func newClient(t *testing.T, handler func(req []byte) []byte) *obdii.Client {
	t.Helper()
	b, _ := virtual.New()
	t.Cleanup(func() { b.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	ecuServer(ctx, b, handler)

	conn, err := isotp.New(b, isotp.Config{TxID: 0x7DF, RxID: 0x7E8})
	if err != nil {
		t.Fatalf("isotp.New: %v", err)
	}
	return obdii.NewClient(conn)
}

func TestReadEngineRPM(t *testing.T) {
	// 2000 rpm: raw = 2000 * 4 = 8000 = 0x1F40
	client := newClient(t, func(req []byte) []byte {
		if req[0] == obdii.ModeCurrentData && req[1] == obdii.PIDEngineRPM {
			return []byte{obdii.ModeCurrentData + 0x40, obdii.PIDEngineRPM, 0x1F, 0x40}
		}
		return []byte{0x7F, req[0], 0x31}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	v, err := client.ReadPID(ctx, obdii.PIDEngineRPM)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}
	if math.Abs(v.Float-2000.0) > 0.1 {
		t.Errorf("RPM = %.1f, want 2000.0", v.Float)
	}
	if v.Unit != "rpm" {
		t.Errorf("unit = %q, want rpm", v.Unit)
	}
}

func TestReadVehicleSpeed(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		if req[0] == obdii.ModeCurrentData && req[1] == obdii.PIDVehicleSpeed {
			return []byte{obdii.ModeCurrentData + 0x40, obdii.PIDVehicleSpeed, 120}
		}
		return []byte{0x7F, req[0], 0x31}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	v, err := client.ReadPID(ctx, obdii.PIDVehicleSpeed)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}
	if v.Float != 120 {
		t.Errorf("speed = %.0f, want 120", v.Float)
	}
	if v.Unit != "km/h" {
		t.Errorf("unit = %q, want km/h", v.Unit)
	}
}

func TestReadCoolantTemp(t *testing.T) {
	// 80°C: raw = 80 + 40 = 120
	client := newClient(t, func(req []byte) []byte {
		if req[0] == obdii.ModeCurrentData && req[1] == obdii.PIDCoolantTemp {
			return []byte{obdii.ModeCurrentData + 0x40, obdii.PIDCoolantTemp, 120}
		}
		return []byte{0x7F, req[0], 0x31}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	v, err := client.ReadPID(ctx, obdii.PIDCoolantTemp)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}
	if v.Float != 80.0 {
		t.Errorf("coolant temp = %.1f, want 80.0", v.Float)
	}
}

func TestReadVIN(t *testing.T) {
	vin := "WVWZZZ3BZWE689725"
	client := newClient(t, func(req []byte) []byte {
		if req[0] == obdii.ModeVehicleInfo && req[1] == obdii.PIDVIN {
			resp := []byte{obdii.ModeVehicleInfo + 0x40, obdii.PIDVIN, 0x01}
			return append(resp, []byte(vin)...)
		}
		return []byte{0x7F, req[0], 0x31}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err := client.ReadVIN(ctx)
	if err != nil {
		t.Fatalf("ReadVIN: %v", err)
	}
	if got != vin {
		t.Errorf("VIN = %q, want %q", got, vin)
	}
}

func TestNegativeResponse(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		return []byte{0x7F, req[0], 0x31} // requestOutOfRange
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.ReadPID(ctx, 0xFF)
	if err == nil {
		t.Error("expected error for negative response")
	}
}

func TestSupportedPIDs(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		if req[0] == obdii.ModeCurrentData && req[1] == 0x00 {
			// Bitmap: PIDs 1-32 support mask
			return []byte{obdii.ModeCurrentData + 0x40, 0x00, 0xBE, 0x1F, 0xA8, 0x13}
		}
		return []byte{0x7F, req[0], 0x31}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	mask, err := client.SupportedPIDs(ctx, 0x00)
	if err != nil {
		t.Fatalf("SupportedPIDs: %v", err)
	}
	if mask == 0 {
		t.Error("expected non-zero PID support mask")
	}
}
