// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// This file contains the RELAY spec bridge: ToMessage, FromMessage, and Adapt.

package can

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	relay "github.com/SoundMatt/RELAY"
)

// ToMessage converts a CAN Frame to a relay.Message for cross-protocol routing.
//
//fusa:req REQ-CAN-007
func (f Frame) ToMessage() relay.Message {
	return relay.Message{
		Protocol:  relay.CAN,
		ID:        strconv.FormatUint(uint64(f.ID), 10),
		Payload:   f.Data,
		Timestamp: time.Now(),
		Meta: map[string]string{
			"can.ext": strconv.FormatBool(f.Ext),
			"can.fd":  strconv.FormatBool(f.FD),
			"can.rtr": strconv.FormatBool(f.RTR),
			"can.brs": strconv.FormatBool(f.BRS),
		},
	}
}

// FromMessage converts a relay.Message back to a CAN Frame.
// Returns ErrInvalidFrame if the message ID is not a valid uint32.
//
//fusa:req REQ-CAN-007
func FromMessage(m relay.Message) (Frame, error) {
	id64, err := strconv.ParseUint(m.ID, 10, 32)
	if err != nil {
		return Frame{}, &ErrInvalidFrame{Reason: "invalid CAN ID: " + m.ID}
	}
	f := Frame{ID: uint32(id64), Data: m.Payload}
	if m.Meta["can.ext"] == "true" {
		f.Ext = true
	}
	if m.Meta["can.fd"] == "true" {
		f.FD = true
	}
	if m.Meta["can.rtr"] == "true" {
		f.RTR = true
	}
	if m.Meta["can.brs"] == "true" {
		f.BRS = true
	}
	return f, nil
}

// Adapt wraps a Bus as a relay.Node for use in cross-protocol relay pipelines.
//
//fusa:req REQ-CAN-007
func Adapt(bus Bus) relay.Node {
	return &canAdapter{bus: bus}
}

type canAdapter struct {
	bus Bus
	seq uint64
}

func (a *canAdapter) Protocol() relay.Protocol { return relay.CAN }

func (a *canAdapter) Send(ctx context.Context, msg relay.Message) error {
	f, err := FromMessage(msg)
	if err != nil {
		return err
	}
	return a.bus.Send(ctx, f)
}

func (a *canAdapter) Subscribe(opts ...relay.SubscriberOption) (<-chan relay.Message, error) {
	cfg := relay.ApplySubscriberOpts(opts)
	out := make(chan relay.Message, cfg.ChanDepth(64))
	frames, err := a.bus.Subscribe(nil)
	if err != nil {
		return nil, err
	}
	go func() {
		defer close(out)
		for f := range frames {
			msg := f.ToMessage()
			msg.Seq = atomic.AddUint64(&a.seq, 1)
			switch cfg.BackPressure {
			case relay.DropNewest:
				select {
				case out <- msg:
				default:
				}
			case relay.DropOldest:
				select {
				case out <- msg:
				default:
					<-out
					out <- msg
				}
			case relay.Block:
				out <- msg
			}
		}
	}()
	return out, nil
}

func (a *canAdapter) Close() error { return a.bus.Close() }
