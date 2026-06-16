// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package can

import "context"

// LoanedFrame is a pre-allocated frame obtained from LoaningBus.Loan().
// Call Return() when done to release the buffer back to the pool.
type LoanedFrame struct {
	Frame
	release func()
}

// NewLoanedFrame creates a LoanedFrame backed by the given release function.
// This is intended for use by LoaningBus implementations.
func NewLoanedFrame(f Frame, release func()) *LoanedFrame {
	return &LoanedFrame{Frame: f, release: release}
}

// Return releases the frame buffer back to the owning pool.
// It is safe to call Return more than once.
func (f *LoanedFrame) Return() {
	if f.release != nil {
		f.release()
		f.release = nil
	}
}

// LoaningBus is an optional zero-copy extension to Bus.
// Implementations that support pre-allocated frame buffers implement this.
type LoaningBus interface {
	Bus
	// Loan returns a pre-allocated LoanedFrame from a pool.
	// The caller must call Return() on the frame when done.
	Loan() (*LoanedFrame, error)
	// SendLoaned transmits the loaned frame and calls Return() on it.
	SendLoaned(ctx context.Context, f *LoanedFrame) error
}

// HealthStatus represents the operational health of a node.
type HealthStatus int

const (
	HealthOK       HealthStatus = 0
	HealthDegraded HealthStatus = 1
	HealthDown     HealthStatus = 2
)

// Health reports the operational health of a node.
type Health struct {
	Status  HealthStatus `json:"status"`
	Details string       `json:"details,omitempty"`
}

// HealthProvider is an optional interface exposing node health.
type HealthProvider interface {
	Health() Health
}

// Metrics holds runtime counters for a bus implementation.
type Metrics struct {
	WriteCount     uint64 `json:"write_count"`
	DeliverCount   uint64 `json:"deliver_count"`
	DropCount      uint64 `json:"drop_count"`
	BytesWritten   uint64 `json:"bytes_written"`
	BytesDelivered uint64 `json:"bytes_delivered"`
	ErrorCount     uint64 `json:"error_count"`
}

// MetricsProvider is an optional interface exposing runtime counters.
type MetricsProvider interface {
	Metrics() Metrics
}

// Drainer extends a bus with graceful shutdown.
// CloseWithDrain waits until all subscriber channels have been drained
// or ctx is cancelled before closing the bus.
type Drainer interface {
	CloseWithDrain(ctx context.Context) error
}
