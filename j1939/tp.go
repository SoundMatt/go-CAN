// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// J1939 Transport Protocol (SAE J1939-21) — BAM implementation.
//
// BAM (Broadcast Announce Message) allows multi-packet transmission of
// J1939 messages up to 1785 bytes using PGN 0xEC00 (TP.CM) and 0xEB00
// (TP.DT) as transport wrappers.
//
// CMDT (Connection Mode Data Transfer) for peer-to-peer PGNs is a future
// enhancement and is not implemented here.

package j1939

import (
	"context"
	"fmt"
	"time"

	can "github.com/SoundMatt/go-CAN"
)

// PGN assignments for J1939-21 Transport Protocol.
const (
	pgnTPCM PGN = 0xEC00 // TP Connection Management (BAM / CMDT control)
	pgnTPDT PGN = 0xEB00 // TP Data Transfer
)

// BAM control byte.
const bamControlByte byte = 0x20

// J1939-21 limits.
const (
	tpMaxDataBytes   = 1785 // maximum payload per SAE J1939-21
	tpBytesPerPacket = 7    // usable payload bytes per TP.DT packet
)

// DefaultTPPacketDelay is the inter-packet gap between BAM TP.DT frames.
// SAE J1939-21 requires 50–200 ms between packets for BAM transmissions.
const DefaultTPPacketDelay = 50 * time.Millisecond

// TPConfig configures Transport Protocol behaviour.
type TPConfig struct {
	// PacketDelay is the delay between consecutive TP.DT packets.
	// A zero value selects DefaultTPPacketDelay.
	PacketDelay time.Duration
}

// packetDelay returns the effective inter-packet delay.
func (c TPConfig) packetDelay() time.Duration {
	if c.PacketDelay == 0 {
		return DefaultTPPacketDelay
	}
	return c.PacketDelay
}

// SendTP sends a multi-packet J1939 frame using the BAM mechanism.
//
// f.Data must be between 9 and 1785 bytes inclusive. For single frames
// (≤8 bytes) use Bus.Send instead.
func (b *Bus) SendTP(ctx context.Context, f Frame, cfg TPConfig) error {
	n := len(f.Data)
	if n < 9 {
		return fmt.Errorf("j1939/tp: SendTP requires at least 9 data bytes (got %d); use Send for single frames", n)
	}
	if n > tpMaxDataBytes {
		return fmt.Errorf("j1939/tp: data length %d exceeds J1939-21 maximum of %d bytes", n, tpMaxDataBytes)
	}

	numPackets := (n + tpBytesPerPacket - 1) / tpBytesPerPacket
	delay := cfg.packetDelay()

	// --- 1. Send TP.CM_BAM ---
	pgnBytes := pgnToBytes(f.PGN)
	bam := [8]byte{
		bamControlByte,
		byte(n),      // total size low byte
		byte(n >> 8), // total size high byte
		byte(numPackets),
		0xFF, // reserved
		pgnBytes[0],
		pgnBytes[1],
		pgnBytes[2],
	}

	bamID := EncodeID(f.Priority, pgnTPCM, b.src)
	if err := b.can.Send(ctx, can.Frame{
		ID:   bamID,
		Ext:  true,
		Data: bam[:],
	}); err != nil {
		return fmt.Errorf("j1939/tp: send TP.CM_BAM: %w", err)
	}

	// --- 2. Send TP.DT packets ---
	dtBaseID := EncodeID(f.Priority, pgnTPDT, b.src)
	for seq := 1; seq <= numPackets; seq++ {
		if seq > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		var pkt [8]byte
		pkt[0] = byte(seq)
		offset := (seq - 1) * tpBytesPerPacket
		// fill payload bytes; pad remainder with 0xFF
		for i := 1; i <= tpBytesPerPacket; i++ {
			src := offset + i - 1
			if src < n {
				pkt[i] = f.Data[src]
			} else {
				pkt[i] = 0xFF
			}
		}

		if err := b.can.Send(ctx, can.Frame{
			ID:   dtBaseID,
			Ext:  true,
			Data: pkt[:],
		}); err != nil {
			return fmt.Errorf("j1939/tp: send TP.DT seq %d: %w", seq, err)
		}
	}

	return nil
}

// SubscribeTP returns a channel delivering reassembled multi-packet J1939
// frames that match any of the given PGNs. With no PGNs, all assembled TP
// frames are delivered.
//
// A background goroutine listens for TP.CM_BAM (0xEC00) and TP.DT (0xEB00)
// frames and reassembles them. The goroutine exits when ctx is done or the
// underlying bus is closed.
func (b *Bus) SubscribeTP(ctx context.Context, pgns ...PGN) (<-chan Frame, error) {
	// Subscribe to all frames; we filter for TP PGNs in the goroutine.
	raw, err := b.can.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("j1939/tp: subscribe: %w", err)
	}

	out := make(chan Frame, 64)

	go func() {
		defer close(out)

		// bamSession tracks an in-progress BAM reassembly keyed by source address.
		type bamSession struct {
			totalSize  int
			numPackets int
			pgn        PGN
			priority   Priority
			src        byte
			buf        []byte
			received   int // packets received so far
		}

		sessions := make(map[byte]*bamSession)

		for {
			select {
			case <-ctx.Done():
				return
			case f, ok := <-raw:
				if !ok {
					return
				}
				if !f.Ext {
					continue
				}

				priority, pgn, src := DecodeID(f.ID)

				switch pgn {
				case pgnTPCM:
					// TP.CM — check for BAM control byte.
					if len(f.Data) < 8 || f.Data[0] != bamControlByte {
						continue
					}
					totalSize := int(f.Data[1]) | int(f.Data[2])<<8
					numPackets := int(f.Data[3])
					targetPGN := pgnFromBytes(f.Data[5], f.Data[6], f.Data[7])

					// A new BAM for the same source replaces any stale session.
					sessions[src] = &bamSession{
						totalSize:  totalSize,
						numPackets: numPackets,
						pgn:        targetPGN,
						priority:   priority,
						src:        src,
						buf:        make([]byte, totalSize),
						received:   0,
					}

				case pgnTPDT:
					// TP.DT — copy packet payload into the reassembly buffer.
					sess, ok := sessions[src]
					if !ok || len(f.Data) < 8 {
						continue
					}
					seq := int(f.Data[0])
					if seq < 1 || seq > sess.numPackets {
						continue
					}
					offset := (seq - 1) * tpBytesPerPacket
					for i := 1; i <= tpBytesPerPacket; i++ {
						dst := offset + i - 1
						if dst < sess.totalSize {
							sess.buf[dst] = f.Data[i]
						}
					}
					sess.received++

					if sess.received == sess.numPackets {
						// All packets received — emit the reassembled frame.
						delete(sessions, src)
						if !matchesPGNs(sess.pgn, pgns) {
							continue
						}
						assembled := Frame{
							Priority: sess.priority,
							PGN:      sess.pgn,
							Src:      sess.src,
							Dst:      BroadcastAddr,
							Data:     sess.buf,
						}
						select {
						case out <- assembled:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	return out, nil
}

// pgnToBytes returns the PGN as a 3-byte little-endian array.
func pgnToBytes(p PGN) [3]byte {
	return [3]byte{byte(p), byte(p >> 8), byte(p >> 16)}
}

// pgnFromBytes reconstructs a PGN from 3 little-endian bytes.
func pgnFromBytes(b0, b1, b2 byte) PGN {
	return PGN(uint32(b0) | uint32(b1)<<8 | uint32(b2)<<16)
}
