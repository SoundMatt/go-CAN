// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package socketcan provides a CAN bus implementation using Linux SocketCAN.
// It works with hardware CAN interfaces (can0, can1, …) and the Linux
// virtual CAN driver (vcan0, …).
//
// Requires Linux kernel ≥ 3.6 with CONFIG_CAN_RAW=y or =m.
//
//fusa:req REQ-SCAN-001
//fusa:req REQ-SCAN-002
//fusa:req REQ-SCAN-003
package socketcan

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	can "github.com/SoundMatt/go-CAN"
	"golang.org/x/sys/unix"
)

const (
	canEFFFlag = 0x80000000 // extended frame format
	canRTRFlag = 0x40000000 // remote transmission request
	canEFFMask = 0x1FFFFFFF // extended ID mask
	canSFFMask = 0x000007FF // standard ID mask
)

// canFrame is the kernel struct layout for a CAN frame (16 bytes).
type canFrame struct {
	ID  uint32
	DLC uint8
	_   [3]byte
	// Data is 8 bytes; kernel pads to 16 total.
}

// Bus is a Linux SocketCAN bus implementation.
//
//fusa:req REQ-SCAN-001
type Bus struct {
	fd   int
	mu   sync.RWMutex
	subs []*subscription
	done chan struct{}
	once sync.Once
}

type subscription struct {
	filters []can.Filter
	ch      chan can.Frame
}

// New opens a raw CAN socket on the named network interface (e.g. "can0", "vcan0").
//
//fusa:req REQ-SCAN-001
func New(iface string) (*Bus, error) {
	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_RAW, unix.CAN_RAW)
	if err != nil {
		return nil, fmt.Errorf("socketcan: socket: %w", err)
	}

	ifIdx, err := ifaceIndex(iface)
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}

	addr := &unix.SockaddrCAN{Ifindex: ifIdx}
	if err := unix.Bind(fd, addr); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("socketcan: bind %q: %w", iface, err)
	}

	b := &Bus{fd: fd, done: make(chan struct{})}
	go b.readLoop()
	return b, nil
}

// Send transmits a CAN frame. Blocks until the kernel accepts the frame.
//
//fusa:req REQ-SCAN-002
func (b *Bus) Send(ctx context.Context, f can.Frame) error {
	if err := can.ValidateFrame(f); err != nil {
		return err
	}
	raw := encodeFrame(f)
	_, err := unix.Write(b.fd, raw)
	if err != nil {
		return fmt.Errorf("socketcan: write: %w", err)
	}
	return nil
}

// Subscribe returns a channel that delivers frames matching any of the filters.
//
//fusa:req REQ-SCAN-003
func (b *Bus) Subscribe(filters ...can.Filter) (<-chan can.Frame, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &subscription{
		filters: filters,
		ch:      make(chan can.Frame, 64),
	}
	b.subs = append(b.subs, s)
	return s.ch, nil
}

// Close releases the socket and closes all subscriber channels.
func (b *Bus) Close() error {
	var closeErr error
	b.once.Do(func() {
		close(b.done)
		closeErr = unix.Close(b.fd)
		b.mu.Lock()
		for _, s := range b.subs {
			close(s.ch)
		}
		b.subs = nil
		b.mu.Unlock()
	})
	return closeErr
}

func (b *Bus) readLoop() {
	buf := make([]byte, 16)
	for {
		select {
		case <-b.done:
			return
		default:
		}
		n, err := unix.Read(b.fd, buf)
		if err != nil || n < 8 {
			return
		}
		f := decodeFrame(buf[:n])
		b.mu.RLock()
		for _, s := range b.subs {
			if matchesAny(s.filters, f) {
				select {
				case s.ch <- f:
				default:
				}
			}
		}
		b.mu.RUnlock()
	}
}

func encodeFrame(f can.Frame) []byte {
	raw := make([]byte, 16)
	id := f.ID
	if f.Ext {
		id |= canEFFFlag
	}
	if f.RTR {
		id |= canRTRFlag
	}
	binary.NativeEndian.PutUint32(raw[0:4], id)
	raw[4] = byte(len(f.Data))
	copy(raw[8:], f.Data)
	return raw
}

func decodeFrame(raw []byte) can.Frame {
	id := binary.NativeEndian.Uint32(raw[0:4])
	ext := id&canEFFFlag != 0
	rtr := id&canRTRFlag != 0
	if ext {
		id &= canEFFMask
	} else {
		id &= canSFFMask
	}
	dlc := int(raw[4])
	if dlc > 8 {
		dlc = 8
	}
	data := make([]byte, dlc)
	copy(data, raw[8:8+dlc])
	return can.Frame{ID: id, Ext: ext, RTR: rtr, Data: data}
}

func ifaceIndex(name string) (int, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return 0, fmt.Errorf("socketcan: interface %q: %w", name, err)
	}
	return iface.Index, nil
}

func matchesAny(filters []can.Filter, f can.Frame) bool {
	if len(filters) == 0 {
		return true
	}
	for _, fl := range filters {
		if fl.Matches(f) {
			return true
		}
	}
	return false
}

