// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command quickstart demonstrates go-CAN's virtual in-process bus.
// A sender goroutine publishes synthetic engine-speed frames every 500 ms;
// a receiver goroutine decodes and prints each frame.
//
// This is the Docker quickstart entrypoint.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/virtual"
)

func main() {
	bus, err := virtual.New()
	if err != nil {
		log.Fatalf("virtual.New: %v", err)
	}
	defer bus.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ch, err := bus.Subscribe(can.Filter{ID: 0x100, Mask: 0x7FF})
	if err != nil {
		log.Fatalf("Subscribe: %v", err)
	}

	go receiver(ctx, ch)
	sender(ctx, bus)
}

func sender(ctx context.Context, bus *virtual.Bus) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	hostname, _ := os.Hostname()
	log.Printf("[sender %s] publishing engine_speed on 0x100", hostname)

	var tick uint64
	for {
		select {
		case <-ticker.C:
			rpm := 2000.0 + 1200.0*math.Sin(float64(tick)/20.0)
			raw := uint16(rpm / 0.25)
			data := make([]byte, 2)
			binary.LittleEndian.PutUint16(data, raw)

			f := can.Frame{ID: 0x100, Data: data}
			if err := bus.Send(ctx, f); err != nil {
				log.Printf("Send: %v", err)
			} else {
				fmt.Printf("TX  100#%04X  (%.1f rpm)\n", raw, rpm)
			}
			tick++
		case <-ctx.Done():
			log.Println("sender: done")
			return
		}
	}
}

func receiver(ctx context.Context, ch <-chan can.Frame) {
	hostname, _ := os.Hostname()
	log.Printf("[receiver %s] waiting for frames on 0x100", hostname)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return
			}
			if len(f.Data) < 2 {
				continue
			}
			raw := binary.LittleEndian.Uint16(f.Data[:2])
			rpm := float64(raw) * 0.25
			fmt.Printf("RX  %03X#%04X  engine_speed=%.1f rpm\n", f.ID, raw, rpm)
		case <-ctx.Done():
			log.Println("receiver: done")
			return
		}
	}
}
