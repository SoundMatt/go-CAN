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
//fusa:req REQ-VIRT-004
//fusa:req REQ-VIRT-005
package virtual

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	can "github.com/SoundMatt/go-CAN"
	relay "github.com/SoundMatt/RELAY"
)

const defaultChanSize = 64

// Compile-time assertions that *Bus satisfies all optional can interfaces.
var (
	_ can.LoaningBus      = (*Bus)(nil)
	_ can.HealthProvider  = (*Bus)(nil)
	_ can.MetricsProvider = (*Bus)(nil)
	_ can.Drainer         = (*Bus)(nil)
)

// Bus is an in-process CAN bus. Multiple goroutines may call Send and
// Subscribe concurrently. The zero value is not usable; call New.
type Bus struct {
	mu   sync.RWMutex
	subs []*subscription
	pool sync.Pool

	closed bool

	writeCount     atomic.Uint64
	deliverCount   atomic.Uint64
	dropCount      atomic.Uint64
	bytesWritten   atomic.Uint64
	bytesDelivered atomic.Uint64
	errorCount     atomic.Uint64
}

type subscription struct {
	filters      []can.Filter
	ch           chan can.Frame
	backPressure relay.BackPressurePolicy
}

// New creates an in-process virtual CAN bus.
//
//fusa:req REQ-VIRT-001
//fusa:req REQ-VIRT-003
func New() (*Bus, error) {
	b := &Bus{}
	b.pool.New = func() any {
		return new(can.LoanedFrame)
	}
	return b, nil
}

// Send broadcasts f to all matching subscribers.
// It returns an error if f is invalid or the bus is closed.
//
//fusa:req REQ-VIRT-001
//fusa:req REQ-VIRT-002
//fusa:req REQ-VIRT-005
func (b *Bus) Send(_ context.Context, f can.Frame) error {
	if err := can.ValidateFrame(f); err != nil {
		b.errorCount.Add(1)
		return err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		b.errorCount.Add(1)
		return errClosed
	}
	b.writeCount.Add(1)
	b.bytesWritten.Add(uint64(len(f.Data)))
	for _, s := range b.subs {
		if matchesAny(s.filters, f) {
			switch s.backPressure {
			case relay.DropNewest:
				select {
				case s.ch <- f:
					b.deliverCount.Add(1)
					b.bytesDelivered.Add(uint64(len(f.Data)))
				default:
					b.dropCount.Add(1)
				}
			case relay.DropOldest:
				select {
				case s.ch <- f:
					b.deliverCount.Add(1)
					b.bytesDelivered.Add(uint64(len(f.Data)))
				default:
					<-s.ch
					s.ch <- f
					b.deliverCount.Add(1)
					b.bytesDelivered.Add(uint64(len(f.Data)))
				}
			case relay.Block:
				s.ch <- f
				b.deliverCount.Add(1)
				b.bytesDelivered.Add(uint64(len(f.Data)))
			}
		}
	}
	return nil
}

// Subscribe returns a channel that delivers frames matching any of the
// supplied filters. With no filters (nil or empty), all frames are delivered.
//
//fusa:req REQ-VIRT-003
//fusa:req REQ-VIRT-004
func (b *Bus) Subscribe(filters []can.Filter, opts ...relay.SubscriberOption) (<-chan can.Frame, error) {
	cfg := relay.ApplySubscriberOpts(opts)
	depth := cfg.ChanDepth(defaultChanSize)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errClosed
	}
	s := &subscription{
		filters:      filters,
		ch:           make(chan can.Frame, depth),
		backPressure: cfg.BackPressure,
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

// Loan returns a pre-allocated LoanedFrame from a pool.
// The caller must call Return() on the frame when done.
func (b *Bus) Loan() (*can.LoanedFrame, error) {
	b.mu.RLock()
	closed := b.closed
	b.mu.RUnlock()
	if closed {
		return nil, errClosed
	}
	lf := b.pool.Get().(*can.LoanedFrame)
	*lf = *can.NewLoanedFrame(can.Frame{}, func() {
		b.pool.Put(lf)
	})
	return lf, nil
}

// SendLoaned transmits the loaned frame and calls Return() on it.
func (b *Bus) SendLoaned(ctx context.Context, f *can.LoanedFrame) error {
	err := b.Send(ctx, f.Frame)
	f.Return()
	return err
}

// Health returns the current operational health of the bus.
func (b *Bus) Health() can.Health {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return can.Health{Status: can.HealthDown, Details: "bus closed"}
	}
	return can.Health{Status: can.HealthOK}
}

// Metrics returns a snapshot of runtime counters.
func (b *Bus) Metrics() can.Metrics {
	return can.Metrics{
		WriteCount:     b.writeCount.Load(),
		DeliverCount:   b.deliverCount.Load(),
		DropCount:      b.dropCount.Load(),
		BytesWritten:   b.bytesWritten.Load(),
		BytesDelivered: b.bytesDelivered.Load(),
		ErrorCount:     b.errorCount.Load(),
	}
}

// CloseWithDrain waits until all subscriber channels are empty or ctx is
// cancelled, then calls Close.
func (b *Bus) CloseWithDrain(ctx context.Context) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return b.Close()
		case <-ticker.C:
			if b.allDrained() {
				return b.Close()
			}
		}
	}
}

// allDrained reports whether every subscriber channel is empty.
func (b *Bus) allDrained() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.subs {
		if len(s.ch) > 0 {
			return false
		}
	}
	return true
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

var errClosed = fmt.Errorf("can/virtual: bus is closed: %w", relay.ErrClosed)
