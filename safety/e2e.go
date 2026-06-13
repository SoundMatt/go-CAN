// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package safety provides end-to-end data protection for CAN payloads.
//
// Protector prepends a 10-byte E2E header to every payload before transmission.
// Receiver strips the header and validates CRC, sequence counter, and freshness
// on every received payload.
//
// Wire format (little-endian, 10 bytes followed by original payload):
//
//	Bytes  0–1   DataID (uint16)
//	Bytes  2–3   SourceID (uint16)
//	Bytes  4–7   SequenceCounter (uint32, monotonically increasing per protector)
//	Bytes  8–9   CRC-16/CCITT-FALSE over bytes 0–7 plus the original payload
//	Bytes 10+    Original payload
//
// The 10-byte header does not fit within a standard CAN frame (8-byte limit).
// Use safety with:
//   - ISO-TP (isotp package): wrap the reassembled multi-byte payload
//   - CAN FD frames (FD=true): up to 54 bytes of protected payload
//   - J1939 multi-packet TP: wrap the assembled data
//
// The CRC slot is treated as zero when computing the CRC.
//
//fusa:req REQ-SAFETY-001
//fusa:req REQ-SAFETY-002
//fusa:req REQ-SAFETY-003
//fusa:req REQ-SAFETY-004
//fusa:req REQ-SAFETY-005
//fusa:req REQ-SEOOC-001
package safety

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
)

const headerSize = 10

// Config configures end-to-end protection parameters.
//
//fusa:req REQ-SAFETY-001
type Config struct {
	// DataID identifies the logical data element (0–65535).
	DataID uint16
	// SourceID identifies the sender (0–65535).
	SourceID uint16
}

// ErrorKind categorises E2E check failures.
type ErrorKind int

const (
	// ErrCRCMismatch means the CRC in the header did not match.
	ErrCRCMismatch ErrorKind = iota
	// ErrSequenceGap means one or more sequence numbers were skipped.
	ErrSequenceGap
	// ErrHeaderTooShort means the payload is shorter than the 10-byte header.
	ErrHeaderTooShort
)

// E2EError is returned when an E2E safety check fails.
type E2EError struct {
	Kind    ErrorKind
	Counter uint32
	Message string
}

func (e *E2EError) Error() string {
	return fmt.Sprintf("can/safety: E2E error (kind=%d, counter=%d): %s", e.Kind, e.Counter, e.Message)
}

// Protector adds an E2E header to payloads before transmission.
// Use Protect to wrap a payload, then send the result via any transport
// (ISO-TP, CAN FD, J1939 TP, etc.).
//
//fusa:req REQ-SAFETY-002
type Protector struct {
	cfg Config
	seq atomic.Uint32
}

// NewProtector creates an E2E protector.
//
//fusa:req REQ-SAFETY-002
func NewProtector(cfg Config) *Protector {
	return &Protector{cfg: cfg}
}

// Protect prepends the E2E header and returns the protected payload.
//
//fusa:req REQ-SAFETY-003
func (p *Protector) Protect(payload []byte) []byte {
	seq := p.seq.Add(1) - 1
	hdr := buildHeader(p.cfg.DataID, p.cfg.SourceID, seq, payload)
	out := make([]byte, headerSize+len(payload))
	copy(out[:headerSize], hdr)
	copy(out[headerSize:], payload)
	return out
}

// Receiver validates E2E headers on received payloads.
//
//fusa:req REQ-SAFETY-004
type Receiver struct {
	mu      sync.Mutex
	cfg     Config
	lastSeq uint32
	first   bool
}

// NewReceiver creates an E2E receiver.
//
//fusa:req REQ-SAFETY-004
func NewReceiver(cfg Config) *Receiver {
	return &Receiver{cfg: cfg, first: true}
}

// Unwrap validates the E2E header in data and returns the original payload.
//
//fusa:req REQ-SAFETY-005
func (r *Receiver) Unwrap(data []byte) ([]byte, error) {
	if len(data) < headerSize {
		return nil, &E2EError{Kind: ErrHeaderTooShort, Message: fmt.Sprintf("need %d bytes, got %d", headerSize, len(data))}
	}

	seq := binary.LittleEndian.Uint32(data[4:8])
	receivedCRC := binary.LittleEndian.Uint16(data[8:10])
	payload := data[headerSize:]

	// Verify CRC
	hdr := buildHeader(r.cfg.DataID, r.cfg.SourceID, seq, payload)
	expectedCRC := binary.LittleEndian.Uint16(hdr[8:10])
	if receivedCRC != expectedCRC {
		return nil, &E2EError{Kind: ErrCRCMismatch, Counter: seq, Message: "CRC mismatch"}
	}

	// Check sequence counter
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.first && seq != r.lastSeq+1 {
		expected := r.lastSeq + 1
		r.lastSeq = seq
		return nil, &E2EError{Kind: ErrSequenceGap, Counter: seq,
			Message: fmt.Sprintf("expected %d, got %d", expected, seq)}
	}
	r.first = false
	r.lastSeq = seq

	return payload, nil
}

// buildHeader constructs the 10-byte E2E header with CRC filled in.
func buildHeader(dataID, sourceID uint16, seq uint32, payload []byte) []byte {
	hdr := make([]byte, headerSize)
	binary.LittleEndian.PutUint16(hdr[0:2], dataID)
	binary.LittleEndian.PutUint16(hdr[2:4], sourceID)
	binary.LittleEndian.PutUint32(hdr[4:8], seq)
	// hdr[8:10] = 0 during CRC computation
	crc := crc16(hdr[:8])
	crc = crc16Cont(crc, payload)
	binary.LittleEndian.PutUint16(hdr[8:10], crc)
	return hdr
}

// crc16 computes CRC-16/CCITT-FALSE (poly=0x1021, init=0xFFFF).
func crc16(data []byte) uint16 {
	const poly = uint16(0x1021)
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func crc16Cont(crc uint16, data []byte) uint16 {
	const poly = uint16(0x1021)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
