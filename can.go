// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package can provides the core types and Bus interface for CAN bus
// communication. Implementations are in sub-packages and are swappable
// without changing application code.
//
// Quickstart:
//
//	import (
//	    can "github.com/SoundMatt/go-CAN"
//	    "github.com/SoundMatt/go-CAN/virtual"
//	)
//
//	bus, _ := virtual.New()
//	defer bus.Close()
//
//	ch, _ := bus.Subscribe([]can.Filter{{ID: 0x100, Mask: 0x7FF}})
//	bus.Send(context.Background(), can.Frame{ID: 0x100, Data: []byte{0x01, 0x02}})
//	frame := <-ch
package can

import (
	"context"

	relay "github.com/SoundMatt/RELAY"
)

//fusa:req REQ-CAN-001
//fusa:req REQ-CAN-002
//fusa:req REQ-CAN-003
//fusa:req REQ-CAN-004
//fusa:req REQ-CAN-005

// SpecVersion is the RELAY spec version this package conforms to. It tracks the
// linked relay module automatically so a dependency bump cannot leave it stale.
const SpecVersion = relay.SpecVersion

// CAN data-length constants.
const (
	CANMaxDataLen   = 8
	CANFDMaxDataLen = 64
	CANMaxStdID     = 0x7FF
	CANMaxExtID     = 0x1FFFFFFF

	// CAN XL (ISO 11898-1:2024) limits.
	CANXLMinDataLen = 1      // XL frames carry at least 1 data byte
	CANXLMaxDataLen = 2048   // XL frames carry up to 2048 data bytes
	CANXLMaxPrioID  = 0x7FF  // XL uses an 11-bit Priority ID
)

// Error sentinels — aliases for relay errors so errors.Is(err, can.ErrClosed) works.
//
//fusa:req REQ-CAN-008
var (
	ErrClosed          = relay.ErrClosed
	ErrNotConnected    = relay.ErrNotConnected
	ErrTimeout         = relay.ErrTimeout
	ErrPayloadTooLarge = relay.ErrPayloadTooLarge
)

// Frame is a CAN, CAN FD, or CAN XL frame.
//
// Standard CAN frames carry 0–8 bytes of data with an 11-bit (Base) or
// 29-bit (Extended) arbitration ID. CAN FD frames extend the payload to
// 64 bytes and optionally switch to a higher data-phase bit rate (BRS).
// CAN XL frames (ISO 11898-1:2024) carry 1–2048 bytes with an 11-bit
// Priority ID and additional addressing fields (SDT, VCID, AF, SEC).
//
//fusa:req REQ-CAN-001
//fusa:req REQ-CANXL-001
type Frame struct {
	// ID is the arbitration identifier. Standard IDs are 11 bits (0–0x7FF);
	// extended IDs are 29 bits (0–0x1FFFFFFF). Set Ext=true for extended IDs.
	// For CAN XL this is the 11-bit Priority ID (0–0x7FF).
	ID uint32 `json:"id"`

	// Ext indicates a 29-bit extended-format frame. Must be false for CAN XL.
	Ext bool `json:"ext,omitempty"`

	// RTR indicates a Remote Transmission Request frame. RTR frames carry no
	// payload; Data must be nil or empty. Must be false for CAN FD and CAN XL.
	RTR bool `json:"rtr,omitempty"`

	// FD indicates a CAN FD frame. FD frames may carry up to 64 bytes.
	// Mutually exclusive with XL.
	FD bool `json:"fd,omitempty"`

	// BRS enables the bit-rate switch in a CAN FD frame (higher data-phase
	// speed). Ignored when FD is false. Must be false for CAN XL.
	BRS bool `json:"brs,omitempty"`

	// ESI is the Error State Indicator (CAN FD / CAN XL only). Must be false
	// when neither FD nor XL is set.
	ESI bool `json:"esi,omitempty"`

	// XL indicates a CAN XL frame (ISO 11898-1:2024; up to 2048 data bytes).
	// Mutually exclusive with FD.
	XL bool `json:"xl,omitempty"`

	// SDT is the CAN XL SDU Type identifying the payload content.
	SDT uint8 `json:"sdt,omitempty"`

	// VCID is the CAN XL Virtual CAN network ID.
	VCID uint8 `json:"vcid,omitempty"`

	// AF is the CAN XL Acceptance Field.
	AF uint32 `json:"af,omitempty"`

	// SEC is the CAN XL Simple Extended Content flag.
	SEC bool `json:"sec,omitempty"`

	// Data holds the frame payload. Length must not exceed 8 bytes for
	// standard CAN frames, 64 bytes for CAN FD frames, or 2048 bytes
	// (minimum 1) for CAN XL frames.
	Data []byte `json:"data"`
}

// MaxDataLen returns the maximum payload length for this frame's format.
//
//fusa:req REQ-CANXL-002
func (f Frame) MaxDataLen() int {
	switch {
	case f.XL:
		return CANXLMaxDataLen
	case f.FD:
		return CANFDMaxDataLen
	default:
		return CANMaxDataLen
	}
}

// Filter selects frames by masked ID comparison. A frame passes when:
//
//	(frame.ID & Mask) == (ID & Mask)
//
// Filter{} (zero value) passes all frames.
type Filter struct {
	ID   uint32 `json:"id"`
	Mask uint32 `json:"mask"`
}

// Matches reports whether f passes the filter.
func (fl Filter) Matches(f Frame) bool {
	return (f.ID & fl.Mask) == (fl.ID & fl.Mask)
}

// Bus is the interface implemented by all go-CAN transports.
//
// The three implementations are:
//   - virtual.Bus — in-process broadcast bus; zero dependencies; default for
//     development and testing.
//   - socketcan.Bus — Linux SocketCAN; real CAN frames over hardware or
//     virtual CAN interfaces (vcan0, can0, …).
//   - (future) canfd.Bus — CAN FD with bit-rate switching.
//
//fusa:req REQ-CAN-006
//fusa:req REQ-CAN-007
//fusa:req REQ-CAN-008
type Bus interface {
	// Send transmits a single CAN frame. It blocks until the frame is
	// accepted by the transport or ctx is cancelled.
	Send(ctx context.Context, f Frame) error

	// Subscribe returns a channel that delivers frames matching any of the
	// supplied filters. With no filters (nil or empty slice), all frames are
	// delivered. The channel is closed when the Bus is closed.
	Subscribe(filters []Filter, opts ...relay.SubscriberOption) (<-chan Frame, error)

	// Close releases all resources and closes subscriber channels.
	Close() error
}

// ErrInvalidFrame is returned when a Frame violates CAN constraints.
type ErrInvalidFrame struct {
	Reason string
}

func (e *ErrInvalidFrame) Error() string { return "can: invalid frame: " + e.Reason }

// MaxDataLen returns the maximum payload length for the given frame type.
func MaxDataLen(fd bool) int {
	if fd {
		return CANFDMaxDataLen
	}
	return CANMaxDataLen
}

// ValidateFrame checks that f satisfies CAN protocol constraints.
//
//fusa:req REQ-CAN-009
//fusa:req REQ-CAN-010
//fusa:req REQ-CAN-011
//fusa:req REQ-CAN-012
//fusa:req REQ-CAN-013
//fusa:req REQ-CAN-014
//fusa:req REQ-CANXL-003
//fusa:req REQ-SEC-001
func ValidateFrame(f Frame) error {
	// FD and XL are distinct, mutually exclusive frame formats.
	if f.FD && f.XL {
		return &ErrInvalidFrame{Reason: "FD and XL are mutually exclusive"}
	}
	// ESI is only meaningful on FD or XL frames.
	if f.ESI && !f.FD && !f.XL {
		return &ErrInvalidFrame{Reason: "ESI requires FD or XL"}
	}

	if f.XL {
		// CAN XL: 11-bit Priority ID; no Ext/RTR/BRS; 1..2048 data bytes.
		if f.Ext {
			return &ErrInvalidFrame{Reason: "CAN XL frame must not set Ext"}
		}
		if f.RTR {
			return &ErrInvalidFrame{Reason: "CAN XL frame must not set RTR"}
		}
		if f.BRS {
			return &ErrInvalidFrame{Reason: "CAN XL frame must not set BRS"}
		}
		if f.ID > CANXLMaxPrioID {
			return &ErrInvalidFrame{Reason: "CAN XL Priority ID exceeds 11 bits"}
		}
		if len(f.Data) < CANXLMinDataLen {
			return &ErrInvalidFrame{Reason: "CAN XL frame must carry at least 1 data byte"}
		}
		if len(f.Data) > CANXLMaxDataLen {
			return &ErrInvalidFrame{Reason: "CAN XL frame data exceeds 2048 bytes"}
		}
		return nil
	}

	if f.Ext && f.ID > CANMaxExtID {
		return &ErrInvalidFrame{Reason: "extended ID exceeds 29 bits"}
	}
	if !f.Ext && f.ID > CANMaxStdID {
		return &ErrInvalidFrame{Reason: "standard ID exceeds 11 bits"}
	}
	if f.RTR && f.FD {
		return &ErrInvalidFrame{Reason: "RTR frame cannot be CAN FD"}
	}
	if f.RTR && len(f.Data) > 0 {
		return &ErrInvalidFrame{Reason: "RTR frame must not carry data"}
	}
	if !f.FD && len(f.Data) > CANMaxDataLen {
		return &ErrInvalidFrame{Reason: "standard CAN frame data exceeds 8 bytes"}
	}
	if f.FD && len(f.Data) > CANFDMaxDataLen {
		return &ErrInvalidFrame{Reason: "CAN FD frame data exceeds 64 bytes"}
	}
	if f.BRS && !f.FD {
		return &ErrInvalidFrame{Reason: "BRS requires FD=true"}
	}
	return nil
}
