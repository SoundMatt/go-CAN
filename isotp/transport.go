// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package isotp implements the ISO 15765-2 (ISO-TP) transport protocol
// for CAN. ISO-TP enables multi-frame message transfer over CAN, supporting
// payloads of up to 4095 bytes (classic) or larger with extended addressing.
//
// ISO-TP is widely used for UDS (ISO 14229) diagnostic communication and
// OBD-II (ISO 15031).
//
// Frame types:
//   - Single Frame (SF):   payload fits in one CAN frame (≤7 bytes)
//   - First Frame (FF):    first segment of a multi-frame message
//   - Consecutive Frame (CF): subsequent segments
//   - Flow Control (FC):   receiver pacing signal
//
//fusa:req REQ-ISOTP-001
//fusa:req REQ-ISOTP-002
//fusa:req REQ-ISOTP-003
//fusa:req REQ-ISOTP-004
//fusa:req REQ-ISOTP-005
//fusa:req REQ-ISOTP-006
//fusa:req REQ-ISOTP-007
//fusa:req REQ-ISOTP-008
//fusa:req REQ-ISOTP-009
//fusa:req REQ-ISOTP-010
//fusa:req REQ-ISOTP-011
//fusa:req REQ-ISOTP-012
//fusa:req REQ-ISOTP-013
package isotp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	can "github.com/SoundMatt/go-CAN"
)

// frame type nibbles (upper nibble of first byte)
const (
	typeSF byte = 0x00
	typeFF byte = 0x10
	typeCF byte = 0x20
	typeFC byte = 0x30
)

// flow control status
const (
	fcContinueToSend byte = 0x00
	fcWait           byte = 0x01
	fcOverflow       byte = 0x02
)

// Config holds ISO-TP addressing and timing parameters.
//
//fusa:req REQ-ISOTP-001
//fusa:req REQ-ISOTP-002
//fusa:req REQ-ISOTP-003
type Config struct {
	// TxID is the CAN arbitration ID used for outgoing frames.
	TxID uint32
	// RxID is the CAN arbitration ID expected for incoming frames.
	RxID uint32
	// ExtIDs indicates use of 29-bit extended CAN IDs.
	ExtIDs bool
	// BlockSize is the maximum number of consecutive frames per block (0 = unlimited).
	BlockSize byte
	// STmin is the minimum separation time between consecutive frames (0–127 ms, or 0xF1–0xF9 for 100–900 µs).
	STmin byte
	// Timeout is the maximum wait for a flow control or consecutive frame (default 250 ms).
	Timeout time.Duration
}

func (c *Config) timeout() time.Duration {
	if c.Timeout == 0 {
		return 250 * time.Millisecond
	}
	return c.Timeout
}

// Conn is an ISO-TP connection over a CAN bus.
//
//fusa:req REQ-ISOTP-004
type Conn struct {
	bus  can.Bus
	cfg  Config
	rxCh <-chan can.Frame
}

// New creates a new ISO-TP connection over the given CAN bus.
//
//fusa:req REQ-ISOTP-001
//fusa:req REQ-ISOTP-002
//fusa:req REQ-ISOTP-003
//fusa:req REQ-ISOTP-004
func New(bus can.Bus, cfg Config) (*Conn, error) {
	rxCh, err := bus.Subscribe([]can.Filter{{ID: cfg.RxID, Mask: 0x1FFFFFFF}})
	if err != nil {
		return nil, fmt.Errorf("isotp: subscribe: %w", err)
	}
	return &Conn{bus: bus, cfg: cfg, rxCh: rxCh}, nil
}

// Send transmits payload using ISO-TP segmentation.
//
//fusa:req REQ-ISOTP-005
//fusa:req REQ-ISOTP-006
//fusa:req REQ-ISOTP-007
//fusa:req REQ-ISOTP-008
func (c *Conn) Send(ctx context.Context, payload []byte) error {
	if len(payload) == 0 {
		return errors.New("isotp: empty payload")
	}
	if len(payload) > 4095 {
		return fmt.Errorf("isotp: payload too large (%d bytes, max 4095)", len(payload))
	}

	if len(payload) <= 7 {
		return c.sendSingleFrame(ctx, payload)
	}
	return c.sendMultiFrame(ctx, payload)
}

func (c *Conn) sendSingleFrame(ctx context.Context, payload []byte) error {
	data := make([]byte, len(payload)+1)
	data[0] = typeSF | byte(len(payload))
	copy(data[1:], payload)
	return c.bus.Send(ctx, c.frame(data))
}

func (c *Conn) sendMultiFrame(ctx context.Context, payload []byte) error {
	// First Frame
	ff := make([]byte, 8)
	binary.BigEndian.PutUint16(ff[0:2], uint16(typeFF)<<8|uint16(len(payload)))
	copy(ff[2:], payload[:6])
	if err := c.bus.Send(ctx, c.frame(ff)); err != nil {
		return err
	}

	// Wait for Flow Control
	fc, err := c.waitFC(ctx)
	if err != nil {
		return err
	}
	if fc[0]&0x0F == fcOverflow {
		return errors.New("isotp: receiver overflow")
	}

	// Send Consecutive Frames
	payload = payload[6:]
	sn := byte(1)
	blockCount := 0
	for len(payload) > 0 {
		if fc[0]&0x0F == fcWait {
			fc, err = c.waitFC(ctx)
			if err != nil {
				return err
			}
			blockCount = 0
		}

		chunk := payload
		if len(chunk) > 7 {
			chunk = chunk[:7]
		}
		cf := make([]byte, len(chunk)+1)
		cf[0] = typeCF | (sn & 0x0F)
		copy(cf[1:], chunk)
		if err := c.bus.Send(ctx, c.frame(cf)); err != nil {
			return err
		}

		payload = payload[len(chunk):]
		sn++
		blockCount++

		if fc[1] > 0 && byte(blockCount) >= fc[1] {
			fc, err = c.waitFC(ctx)
			if err != nil {
				return err
			}
			blockCount = 0
		}

		if fc[2] > 0 {
			time.Sleep(stminToDuration(fc[2]))
		}
	}
	return nil
}

// Recv reassembles and returns the next ISO-TP message.
//
//fusa:req REQ-ISOTP-009
//fusa:req REQ-ISOTP-010
//fusa:req REQ-ISOTP-011
//fusa:req REQ-ISOTP-012
//fusa:req REQ-ISOTP-013
func (c *Conn) Recv(ctx context.Context) ([]byte, error) {
	timeout := time.After(c.cfg.timeout())
	var first can.Frame
	select {
	case f, ok := <-c.rxCh:
		if !ok {
			return nil, errors.New("isotp: bus closed")
		}
		first = f
	case <-timeout:
		return nil, errors.New("isotp: recv timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if len(first.Data) == 0 {
		return nil, errors.New("isotp: empty frame")
	}

	frameType := first.Data[0] & 0xF0
	switch frameType {
	case typeSF:
		length := int(first.Data[0] & 0x0F)
		if length == 0 || length > len(first.Data)-1 {
			return nil, fmt.Errorf("isotp: invalid SF length %d", length)
		}
		return first.Data[1 : 1+length], nil

	case typeFF:
		if len(first.Data) < 2 {
			return nil, errors.New("isotp: FF too short")
		}
		length := int(binary.BigEndian.Uint16(first.Data[0:2]) & 0x0FFF)
		buf := make([]byte, 0, length)
		buf = append(buf, first.Data[2:]...)

		// Send Flow Control
		fc := c.frame([]byte{typeFC | fcContinueToSend, c.cfg.BlockSize, c.cfg.STmin})
		if err := c.bus.Send(ctx, fc); err != nil {
			return nil, fmt.Errorf("isotp: send FC: %w", err)
		}

		sn := byte(1)
		for len(buf) < length {
			cf, err := c.recvCF(ctx)
			if err != nil {
				return nil, err
			}
			if cf.Data[0]&0x0F != sn&0x0F {
				return nil, fmt.Errorf("isotp: unexpected SN %d (want %d)", cf.Data[0]&0x0F, sn&0x0F)
			}
			remaining := length - len(buf)
			chunk := cf.Data[1:]
			if len(chunk) > remaining {
				chunk = chunk[:remaining]
			}
			buf = append(buf, chunk...)
			sn++
		}
		return buf, nil

	default:
		return nil, fmt.Errorf("isotp: unexpected frame type 0x%X", frameType)
	}
}

func (c *Conn) waitFC(ctx context.Context) ([]byte, error) {
	timeout := time.After(c.cfg.timeout())
	for {
		select {
		case f, ok := <-c.rxCh:
			if !ok {
				return nil, errors.New("isotp: bus closed")
			}
			if len(f.Data) >= 3 && f.Data[0]&0xF0 == typeFC {
				return f.Data, nil
			}
		case <-timeout:
			return nil, errors.New("isotp: flow control timeout")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Conn) recvCF(ctx context.Context) (can.Frame, error) {
	timeout := time.After(c.cfg.timeout())
	for {
		select {
		case f, ok := <-c.rxCh:
			if !ok {
				return can.Frame{}, errors.New("isotp: bus closed")
			}
			if len(f.Data) > 0 && f.Data[0]&0xF0 == typeCF {
				return f, nil
			}
		case <-timeout:
			return can.Frame{}, errors.New("isotp: consecutive frame timeout")
		case <-ctx.Done():
			return can.Frame{}, ctx.Err()
		}
	}
}

func (c *Conn) frame(data []byte) can.Frame {
	return can.Frame{ID: c.cfg.TxID, Ext: c.cfg.ExtIDs, Data: data}
}

// stminToDuration converts an STmin byte value to a time.Duration.
func stminToDuration(stmin byte) time.Duration {
	switch {
	case stmin <= 0x7F:
		return time.Duration(stmin) * time.Millisecond
	case stmin >= 0xF1 && stmin <= 0xF9:
		return time.Duration(stmin-0xF0) * 100 * time.Microsecond
	default:
		return 0
	}
}
