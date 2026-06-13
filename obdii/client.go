// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package obdii implements OBD-II (ISO 15031 / SAE J1979) on-board
// diagnostics over ISO-TP. OBD-II is the standardised diagnostic interface
// mandated for all passenger cars sold in the USA since 1996.
//
// OBD-II uses fixed CAN IDs:
//   - 0x7DF  functional broadcast request (all ECUs)
//   - 0x7E8  ECU #1 response (engine control unit)
//   - 0x7E9–0x7EF additional ECU responses
//
// Typical ISO-TP config: TxID=0x7DF, RxID=0x7E8.
//
//fusa:req REQ-OBD-001
//fusa:req REQ-OBD-002
//fusa:req REQ-OBD-003
package obdii

import (
	"context"
	"errors"
	"fmt"

	"github.com/SoundMatt/go-CAN/isotp"
)

// Service identifiers (OBD-II modes).
const (
	ModeCurrentData       = 0x01 // show current data
	ModeFreezeDTC         = 0x02 // show freeze frame data
	ModeStoredDTC         = 0x03 // show stored diagnostic trouble codes
	ModeClearDTC          = 0x04 // clear DTCs and stored values
	ModeTestResults       = 0x05 // oxygen sensor monitor test results
	ModeOnboardMonitor    = 0x06 // on-board monitoring test results
	ModePendingDTC        = 0x07 // show pending diagnostic trouble codes
	ModeControlOperation  = 0x08 // control operation of on-board system
	ModeVehicleInfo       = 0x09 // request vehicle information
	ModePermDTC           = 0x0A // permanent / confirmed DTCs
)

// Standard Mode 01 PIDs (partial list).
const (
	PIDMonitorStatus         = 0x01 // monitor status since DTCs cleared
	PIDFuelSystemStatus      = 0x03 // fuel system status
	PIDEngineLoad            = 0x04 // calculated engine load (%)
	PIDCoolantTemp           = 0x05 // engine coolant temperature (°C)
	PIDBankFuelTrim1Short    = 0x06 // short-term fuel trim bank 1 (%)
	PIDBankFuelTrim1Long     = 0x07 // long-term fuel trim bank 1 (%)
	PIDIntakeManifoldPressure = 0x0B // intake manifold absolute pressure (kPa)
	PIDEngineRPM             = 0x0C // engine speed (rpm)
	PIDVehicleSpeed          = 0x0D // vehicle speed (km/h)
	PIDTimingAdvance         = 0x0E // timing advance (°)
	PIDIntakeAirTemp         = 0x0F // intake air temperature (°C)
	PIDMAF                   = 0x10 // MAF air flow rate (g/s)
	PIDThrottlePosition      = 0x11 // throttle position (%)
	PIDOBDStandards          = 0x1C // OBD standards this vehicle conforms to
	PIDRuntimeSinceStart     = 0x1F // run time since engine start (s)
	PIDFuelTankLevel         = 0x2F // fuel tank input level (%)
	PIDBarometricPressure    = 0x33 // barometric pressure (kPa)
	PIDControlModuleVoltage  = 0x42 // control module voltage (V)
	PIDAbsoluteLoad          = 0x43 // absolute load value (%)
	PIDOBDFuelType           = 0x51 // fuel type
	PIDAmbientAirTemp        = 0x46 // ambient air temperature (°C)

	// Mode 09 PIDs
	PIDVIN                   = 0x02 // vehicle identification number (17 chars)
	PIDECUName               = 0x0A // ECU name (20 bytes)
)

// Value holds a decoded OBD-II signal with its physical value and unit.
type Value struct {
	PID   byte
	Raw   []byte
	Float float64
	Unit  string
}

// Client is an OBD-II client communicating over ISO-TP.
//
//fusa:req REQ-OBD-001
type Client struct {
	conn *isotp.Conn
}

// NewClient creates an OBD-II client.
//
//fusa:req REQ-OBD-001
func NewClient(conn *isotp.Conn) *Client {
	return &Client{conn: conn}
}

// ReadPID requests a single Mode 01 (current data) PID and returns the decoded value.
//
//fusa:req REQ-OBD-002
func (c *Client) ReadPID(ctx context.Context, pid byte) (*Value, error) {
	resp, err := c.request(ctx, []byte{ModeCurrentData, pid})
	if err != nil {
		return nil, err
	}
	if len(resp) < 3 || resp[0] != ModeCurrentData+0x40 || resp[1] != pid {
		return nil, fmt.Errorf("obdii: unexpected response for PID 0x%02X: %X", pid, resp)
	}
	data := resp[2:]
	return decode01(pid, data), nil
}

// ReadVIN requests the Vehicle Identification Number (Mode 09 PID 0x02).
//
//fusa:req REQ-OBD-003
func (c *Client) ReadVIN(ctx context.Context) (string, error) {
	resp, err := c.request(ctx, []byte{ModeVehicleInfo, PIDVIN})
	if err != nil {
		return "", err
	}
	if len(resp) < 3 || resp[0] != ModeVehicleInfo+0x40 || resp[1] != PIDVIN {
		return "", fmt.Errorf("obdii: unexpected VIN response: %X", resp)
	}
	// resp[2] = number of data items (usually 1); resp[3:] = VIN bytes
	if len(resp) < 4 {
		return "", errors.New("obdii: VIN response too short")
	}
	vin := resp[3:]
	// trim null padding
	for i, b := range vin {
		if b == 0 {
			vin = vin[:i]
			break
		}
	}
	return string(vin), nil
}

// SupportedPIDs returns the bitmask of supported PIDs in the given group.
// group is the PID group base (0x00, 0x20, 0x40, 0x60, 0x80, 0xA0, 0xC0, 0xE0).
func (c *Client) SupportedPIDs(ctx context.Context, group byte) (uint32, error) {
	resp, err := c.request(ctx, []byte{ModeCurrentData, group})
	if err != nil {
		return 0, err
	}
	if len(resp) < 6 || resp[0] != ModeCurrentData+0x40 || resp[1] != group {
		return 0, fmt.Errorf("obdii: unexpected supported PIDs response: %X", resp)
	}
	mask := uint32(resp[2])<<24 | uint32(resp[3])<<16 | uint32(resp[4])<<8 | uint32(resp[5])
	return mask, nil
}

func (c *Client) request(ctx context.Context, req []byte) ([]byte, error) {
	if err := c.conn.Send(ctx, req); err != nil {
		return nil, fmt.Errorf("obdii: send: %w", err)
	}
	resp, err := c.conn.Recv(ctx)
	if err != nil {
		return nil, fmt.Errorf("obdii: recv: %w", err)
	}
	if len(resp) == 0 {
		return nil, errors.New("obdii: empty response")
	}
	if resp[0] == 0x7F {
		nrc := byte(0)
		if len(resp) >= 3 {
			nrc = resp[2]
		}
		return nil, fmt.Errorf("obdii: negative response for mode 0x%02X: NRC 0x%02X", req[0], nrc)
	}
	return resp, nil
}

// decode01 decodes a Mode 01 PID data bytes into a Value.
func decode01(pid byte, data []byte) *Value {
	v := &Value{PID: pid, Raw: data}
	switch pid {
	case PIDEngineRPM:
		if len(data) >= 2 {
			v.Float = float64(uint16(data[0])<<8|uint16(data[1])) / 4.0
			v.Unit = "rpm"
		}
	case PIDVehicleSpeed:
		if len(data) >= 1 {
			v.Float = float64(data[0])
			v.Unit = "km/h"
		}
	case PIDCoolantTemp:
		if len(data) >= 1 {
			v.Float = float64(data[0]) - 40
			v.Unit = "°C"
		}
	case PIDIntakeAirTemp:
		if len(data) >= 1 {
			v.Float = float64(data[0]) - 40
			v.Unit = "°C"
		}
	case PIDAmbientAirTemp:
		if len(data) >= 1 {
			v.Float = float64(data[0]) - 40
			v.Unit = "°C"
		}
	case PIDEngineLoad:
		if len(data) >= 1 {
			v.Float = float64(data[0]) * 100.0 / 255.0
			v.Unit = "%"
		}
	case PIDThrottlePosition:
		if len(data) >= 1 {
			v.Float = float64(data[0]) * 100.0 / 255.0
			v.Unit = "%"
		}
	case PIDMAF:
		if len(data) >= 2 {
			v.Float = float64(uint16(data[0])<<8|uint16(data[1])) / 100.0
			v.Unit = "g/s"
		}
	case PIDIntakeManifoldPressure, PIDBarometricPressure:
		if len(data) >= 1 {
			v.Float = float64(data[0])
			v.Unit = "kPa"
		}
	case PIDFuelTankLevel:
		if len(data) >= 1 {
			v.Float = float64(data[0]) * 100.0 / 255.0
			v.Unit = "%"
		}
	case PIDBankFuelTrim1Short, PIDBankFuelTrim1Long:
		if len(data) >= 1 {
			v.Float = float64(data[0])*100.0/128.0 - 100.0
			v.Unit = "%"
		}
	case PIDTimingAdvance:
		if len(data) >= 1 {
			v.Float = float64(data[0])/2.0 - 64.0
			v.Unit = "°"
		}
	case PIDControlModuleVoltage:
		if len(data) >= 2 {
			v.Float = float64(uint16(data[0])<<8|uint16(data[1])) / 1000.0
			v.Unit = "V"
		}
	case PIDRuntimeSinceStart:
		if len(data) >= 2 {
			v.Float = float64(uint16(data[0])<<8 | uint16(data[1]))
			v.Unit = "s"
		}
	case PIDAbsoluteLoad:
		if len(data) >= 2 {
			v.Float = float64(uint16(data[0])<<8|uint16(data[1])) * 100.0 / 255.0
			v.Unit = "%"
		}
	}
	return v
}
