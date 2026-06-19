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

//fusa:test REQ-OBD-001
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

// TestDecodePIDs exercises every Mode 01 PID decoder branch (REQ-OBD-002).
func TestDecodePIDs(t *testing.T) {
	tests := []struct {
		name string
		pid  byte
		raw  []byte
		want float64
		unit string
	}{
		{"intake air temp", obdii.PIDIntakeAirTemp, []byte{60}, 20, "°C"},
		{"ambient air temp", obdii.PIDAmbientAirTemp, []byte{55}, 15, "°C"},
		{"engine load", obdii.PIDEngineLoad, []byte{255}, 100, "%"},
		{"throttle position", obdii.PIDThrottlePosition, []byte{128}, 128 * 100.0 / 255.0, "%"},
		{"MAF", obdii.PIDMAF, []byte{0x10, 0x00}, 40.96, "g/s"},
		{"intake manifold pressure", obdii.PIDIntakeManifoldPressure, []byte{101}, 101, "kPa"},
		{"barometric pressure", obdii.PIDBarometricPressure, []byte{99}, 99, "kPa"},
		{"fuel tank level", obdii.PIDFuelTankLevel, []byte{128}, 128 * 100.0 / 255.0, "%"},
		{"short fuel trim", obdii.PIDBankFuelTrim1Short, []byte{128}, 0, "%"},
		{"long fuel trim", obdii.PIDBankFuelTrim1Long, []byte{192}, 192*100.0/128.0 - 100.0, "%"},
		{"timing advance", obdii.PIDTimingAdvance, []byte{128}, 0, "°"},
		{"control module voltage", obdii.PIDControlModuleVoltage, []byte{0x2E, 0xE0}, 12.0, "V"},
		{"runtime since start", obdii.PIDRuntimeSinceStart, []byte{0x01, 0x00}, 256, "s"},
		{"absolute load", obdii.PIDAbsoluteLoad, []byte{0x00, 0xFF}, 100, "%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid, raw := tt.pid, tt.raw
			client := newClient(t, func(req []byte) []byte {
				if req[0] == obdii.ModeCurrentData && req[1] == pid {
					return append([]byte{obdii.ModeCurrentData + 0x40, pid}, raw...)
				}
				return []byte{0x7F, req[0], 0x31}
			})

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			v, err := client.ReadPID(ctx, pid)
			if err != nil {
				t.Fatalf("ReadPID(0x%02X): %v", pid, err)
			}
			if math.Abs(v.Float-tt.want) > 0.01 {
				t.Errorf("PID 0x%02X value = %.4f, want %.4f", pid, v.Float, tt.want)
			}
			if v.Unit != tt.unit {
				t.Errorf("PID 0x%02X unit = %q, want %q", pid, v.Unit, tt.unit)
			}
		})
	}
}

// TestReadPIDShortResponse verifies a response too short for a valid PID echo
// is rejected.
func TestReadPIDShortResponse(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		return []byte{obdii.ModeCurrentData + 0x40} // missing echoed PID + data
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ReadPID(ctx, obdii.PIDEngineRPM); err == nil {
		t.Error("expected error for short response")
	}
}

// TestReadVINShortResponse verifies a truncated VIN response is rejected.
func TestReadVINShortResponse(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		return []byte{obdii.ModeVehicleInfo + 0x40, obdii.PIDVIN} // no count/data
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ReadVIN(ctx); err == nil {
		t.Error("expected error for short VIN response")
	}
}

// TestReadVINNullPadding verifies null-padding is trimmed from the VIN.
func TestReadVINNullPadding(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		resp := []byte{obdii.ModeVehicleInfo + 0x40, obdii.PIDVIN, 0x01}
		resp = append(resp, []byte("ABC123")...)
		resp = append(resp, 0x00, 0x00) // trailing null padding
		return resp
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	vin, err := client.ReadVIN(ctx)
	if err != nil {
		t.Fatalf("ReadVIN: %v", err)
	}
	if vin != "ABC123" {
		t.Errorf("VIN = %q, want ABC123 (null padding trimmed)", vin)
	}
}

// TestReadVINNegative verifies a negative response on a VIN request errors.
func TestReadVINNegative(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		return []byte{0x7F, req[0], 0x31}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.ReadVIN(ctx); err == nil {
		t.Error("expected error for negative VIN response")
	}
}

// TestSupportedPIDsNegative verifies SupportedPIDs surfaces a negative response.
func TestSupportedPIDsNegative(t *testing.T) {
	client := newClient(t, func(req []byte) []byte {
		return []byte{0x7F, req[0], 0x31}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := client.SupportedPIDs(ctx, 0x00); err == nil {
		t.Error("expected error for negative SupportedPIDs response")
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
