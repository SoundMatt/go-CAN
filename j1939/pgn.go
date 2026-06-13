// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package j1939 implements the SAE J1939 protocol layer for heavy-duty
// vehicle networks. J1939 uses 29-bit extended CAN IDs and defines a
// Parameter Group Number (PGN) based addressing scheme.
//
// J1939 29-bit ID layout:
//
//	Bits 28–26  Priority (P)     — 3 bits (0 highest, 7 lowest)
//	Bit  25     Reserved (R)     — always 0
//	Bit  24     Data Page (DP)   — 0 or 1
//	Bits 23–16  PDU Format (PF)  — 8 bits
//	Bits 15–8   PDU Specific (PS)— destination (PF<240) or group ext (PF≥240)
//	Bits  7–0   Source Address   — 8 bits
//
// When PF < 240, the message is peer-to-peer (PS is destination address).
// When PF ≥ 240, the message is broadcast (PS is the group extension, part of PGN).
//
//fusa:req REQ-J1939-001
//fusa:req REQ-J1939-002
package j1939

import (
	"context"
	"fmt"

	can "github.com/SoundMatt/go-CAN"
)

// BroadcastAddr is the J1939 global destination address.
const BroadcastAddr byte = 0xFF

// NullAddr is the J1939 null address (not claimed).
const NullAddr byte = 0xFE

// Priority represents J1939 message priority (0 = highest, 7 = lowest).
type Priority byte

// PGN is a J1939 Parameter Group Number (18 bits).
type PGN uint32

// Frame is a decoded J1939 message.
type Frame struct {
	Priority Priority
	PGN      PGN
	Src      byte
	Dst      byte // valid only when PGN.IsPeerToPeer()
	Data     []byte
}

// IsPeerToPeer reports whether this PGN is a peer-to-peer message
// (PDU Format < 240).
func (p PGN) IsPeerToPeer() bool {
	pf := byte((p >> 8) & 0xFF)
	return pf < 240
}

// DecodeID extracts J1939 fields from a 29-bit CAN extended ID.
//
//fusa:req REQ-J1939-001
func DecodeID(id uint32) (priority Priority, pgn PGN, src byte) {
	priority = Priority((id >> 26) & 0x07)
	src = byte(id & 0xFF)
	pf := byte((id >> 16) & 0xFF)
	ps := byte((id >> 8) & 0xFF)
	dp := byte((id >> 24) & 0x01)
	if pf < 240 {
		// Peer-to-peer: PS is destination, PGN does not include PS
		pgn = PGN(uint32(dp)<<17 | uint32(pf)<<8)
	} else {
		// Broadcast: PS is group extension, part of PGN
		pgn = PGN(uint32(dp)<<17 | uint32(pf)<<8 | uint32(ps))
	}
	return priority, pgn, src
}

// EncodeID builds a 29-bit J1939 CAN extended ID.
//
//fusa:req REQ-J1939-002
func EncodeID(priority Priority, pgn PGN, src byte) uint32 {
	pf := byte((pgn >> 8) & 0xFF)
	ps := byte(pgn & 0xFF)
	dp := byte((pgn >> 17) & 0x01)
	var id uint32
	id |= uint32(priority&0x07) << 26
	id |= uint32(dp) << 24
	id |= uint32(pf) << 16
	if pf >= 240 {
		id |= uint32(ps) << 8
	}
	id |= uint32(src)
	return id
}

// Bus wraps a CAN bus with J1939-aware send/receive.
type Bus struct {
	can can.Bus
	src byte
}

// NewBus creates a J1939 bus with the given source address.
func NewBus(canBus can.Bus, srcAddr byte) *Bus {
	return &Bus{can: canBus, src: srcAddr}
}

// Send transmits a J1939 frame.
func (b *Bus) Send(ctx context.Context, f Frame) error {
	id := EncodeID(f.Priority, f.PGN, b.src)
	if f.PGN.IsPeerToPeer() {
		// embed destination in bits 15–8
		id |= uint32(f.Dst) << 8
	}
	if len(f.Data) > 8 {
		return fmt.Errorf("j1939: data too long (%d bytes, max 8 for single frame)", len(f.Data))
	}
	return b.can.Send(ctx, can.Frame{ID: id, Ext: true, Data: f.Data})
}

// Subscribe returns a channel delivering decoded J1939 frames that match
// the given PGNs. With no PGNs, all J1939 frames are delivered.
func (b *Bus) Subscribe(pgns ...PGN) (<-chan Frame, error) {
	raw, err := b.can.Subscribe()
	if err != nil {
		return nil, err
	}
	out := make(chan Frame, 64)
	go func() {
		defer close(out)
		for f := range raw {
			if !f.Ext {
				continue
			}
			priority, pgn, src := DecodeID(f.ID)
			if !matchesPGNs(pgn, pgns) {
				continue
			}
			dst := BroadcastAddr
			if pgn.IsPeerToPeer() {
				dst = byte((f.ID >> 8) & 0xFF)
			}
			out <- Frame{
				Priority: priority,
				PGN:      pgn,
				Src:      src,
				Dst:      dst,
				Data:     f.Data,
			}
		}
	}()
	return out, nil
}

func matchesPGNs(pgn PGN, filter []PGN) bool {
	if len(filter) == 0 {
		return true
	}
	for _, p := range filter {
		if p == pgn {
			return true
		}
	}
	return false
}
