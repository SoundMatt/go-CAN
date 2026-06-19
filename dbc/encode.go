// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dbc

import (
	"fmt"
	"math"
)

// Encode encodes a map of signal physical values into a CAN frame payload.
// signals maps signal name to physical value. Signals absent from the map
// are left as zero. Returns a byte slice of length msg.DLC.
// Returns an error if msgID is unknown or a signal name is not in the message.
//
//fusa:req REQ-DBC-005
//fusa:req REQ-DBC-007
func (db *DB) Encode(msgID uint32, signals map[string]float64) ([]byte, error) {
	msg, ok := db.Messages[msgID]
	if !ok {
		return nil, fmt.Errorf("dbc: unknown message ID %d", msgID)
	}

	// Validate all signal names before allocating output.
	for name := range signals {
		if _, ok := msg.Signals[name]; !ok {
			return nil, fmt.Errorf("dbc: signal %q not found in message %d", name, msgID)
		}
	}

	data := make([]byte, msg.DLC)

	for name, physical := range signals {
		sig := msg.Signals[name]
		raw := physicalToRaw(physical, sig)
		packRaw(data, raw, sig.StartBit, sig.Length, sig.ByteOrder)
	}

	return data, nil
}

// physicalToRaw converts a physical value to a raw unsigned bit pattern
// (two's complement for signed signals), clamped to the signal's bit range.
//
//fusa:req REQ-SEC-004
func physicalToRaw(physical float64, sig *Signal) uint64 {
	rawF := math.Round((physical - sig.Offset) / sig.Factor)

	if sig.Signed {
		maxRaw := int64(1)<<(sig.Length-1) - 1
		minRaw := -(int64(1) << (sig.Length - 1))
		ri := int64(rawF)
		if ri > maxRaw {
			ri = maxRaw
		} else if ri < minRaw {
			ri = minRaw
		}
		// Convert to two's complement bit pattern of sig.Length bits.
		mask := uint64((1 << sig.Length) - 1)
		return uint64(ri) & mask
	}

	// Unsigned
	maxRaw := uint64((1 << sig.Length) - 1)
	if sig.Length == 64 {
		maxRaw = math.MaxUint64
	}
	ru := uint64(math.Max(0, rawF))
	if ru > maxRaw {
		ru = maxRaw
	}
	return ru
}

// packRaw packs a raw value (of length bits) into data starting at startBit,
// using the given byte order.
func packRaw(data []byte, raw uint64, startBit, length int, order ByteOrder) {
	if order == LittleEndian {
		for i := 0; i < length; i++ {
			bit := startBit + i
			byteIdx := bit / 8
			bitIdx := uint(bit % 8)
			if byteIdx >= len(data) {
				break
			}
			if raw&(1<<i) != 0 {
				data[byteIdx] |= 1 << bitIdx
			}
		}
	} else {
		// Big endian (Motorola): startBit is MSB position.
		// Walk the same bit path as extractRaw (MSB first).
		byteIdx := startBit / 8
		bitIdx := startBit % 8
		for i := 0; i < length; i++ {
			if byteIdx < len(data) {
				if raw&(1<<(length-1-i)) != 0 {
					data[byteIdx] |= 1 << uint(bitIdx)
				}
			}
			if bitIdx == 0 {
				bitIdx = 7
				byteIdx++
			} else {
				bitIdx--
			}
		}
	}
}
