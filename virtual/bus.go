// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package virtual provides an in-process CAN bus for development and testing.
// It has zero dependencies and works on all platforms.
//
// All frames sent on a virtual Bus are broadcast to every subscriber whose
// filters match, including the sender. This mirrors real CAN bus behaviour
// where every node on the bus sees every frame.
//
//fusa:req REQ-VIRT-001
//fusa:req REQ-VIRT-002
//fusa:req REQ-VIRT-003
package virtual

import (
	"context"
	"sync"

	can "github.com/SoundMatt/go-CAN"
)

const defaultChanSize = 64

// Bus is an in-process CAN bus. Multiple goroutines may call Send and
// Subscribe concurrently. The zero value is not usable; call New.
type Bus struct {
	mu     sync.RWMutex
	subs   []*subscription
	closed bool
}

type subscription struct {
	filters []can.Filter
	ch      chan can.Frame
}

// New creates an in-process virtual CAN bus.
//
//fusa:req REQ-VIRT-001
func New() (*Bus, error) {
	return &Bus{}, nil
}

// Send broadcasts f to all matching subscribers.
// It returns an error if f is invalid or the bus is closed.
//
//fusa:req REQ-VIRT-002
func (b *Bus) Send(_ context.Context, f can.Frame) error {
	if err := can.ValidateFrame(f); err != nil {
		return err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errClosed
	}
	for _, s := range b.subs {
		if matchesAny(s.filters, f) {
			select {
			case s.ch <- f:
			default:
				// drop on full channel — same semantics as real CAN hardware
			}
		}
	}
	return nil
}

// Subscribe returns a channel that delivers frames matching any of the
// supplied filters. With no filters, all frames are delivered.
//
//fusa:req REQ-VIRT-003
func (b *Bus) Subscribe(filters ...can.Filter) (<-chan can.Frame, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errClosed
	}
	s := &subscription{
		filters: filters,
		ch:      make(chan can.Frame, defaultChanSize),
	}
	b.subs = append(b.subs, s)
	return s.ch, nil
}

// Close releases all resources and closes all subscriber channels.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, s := range b.subs {
		close(s.ch)
	}
	b.subs = nil
	return nil
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

var errClosed = &closedError{}

type closedError struct{}

func (*closedError) Error() string { return "can/virtual: bus is closed" }
