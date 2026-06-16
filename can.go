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

// SpecVersion is the RELAY spec version this package conforms to.
const SpecVersion = "0.2"

// CAN data-length constants.
const (
	CANMaxDataLen   = 8
	CANFDMaxDataLen = 64
	CANMaxStdID     = 0x7FF
	CANMaxExtID     = 0x1FFFFFFF
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

// Frame is a CAN or CAN FD frame.
//
// Standard CAN frames carry 0–8 bytes of data with an 11-bit (Base) or
// 29-bit (Extended) arbitration ID. CAN FD frames extend the payload to
// 64 bytes and optionally switch to a higher data-phase bit rate (BRS).
//
//fusa:req REQ-CAN-001
type Frame struct {
	// ID is the arbitration identifier. Standard IDs are 11 bits (0–0x7FF);
	// extended IDs are 29 bits (0–0x1FFFFFFF). Set Ext=true for extended IDs.
	ID uint32 `json:"id"`

	// Ext indicates a 29-bit extended-format frame.
	Ext bool `json:"ext,omitempty"`

	// RTR indicates a Remote Transmission Request frame. RTR frames carry no
	// payload; Data must be nil or empty.
	RTR bool `json:"rtr,omitempty"`

	// FD indicates a CAN FD frame. FD frames may carry up to 64 bytes.
	FD bool `json:"fd,omitempty"`

	// BRS enables the bit-rate switch in a CAN FD frame (higher data-phase
	// speed). Ignored when FD is false.
	BRS bool `json:"brs,omitempty"`

	// Data holds the frame payload. Length must not exceed 8 bytes for
	// standard CAN frames or 64 bytes for CAN FD frames.
	Data []byte `json:"data"`
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
func ValidateFrame(f Frame) error {
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
