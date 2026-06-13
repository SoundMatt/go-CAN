// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command sender publishes synthetic CAN frames on the virtual bus.
// Part of the Docker Quickstart.
//
// Environment variables:
//
//	CAN_INTERVAL_MS  Milliseconds between frames (default: 500)
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/virtual"
)

func main() {
	intervalMs := 500
	if v := os.Getenv("CAN_INTERVAL_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			intervalMs = n
		}
	}

	bus, err := virtual.New()
	if err != nil {
		log.Fatalf("virtual.New: %v", err)
	}
	defer bus.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	log.Printf("sender: publishing engine_speed on ID 0x100 every %d ms", intervalMs)

	var tick uint64
	for {
		select {
		case <-ticker.C:
			// Synthetic engine speed: 800–3200 rpm (sinusoidal)
			rpm := 2000.0 + 1200.0*math.Sin(float64(tick)/20.0)
			raw := uint16(rpm / 0.25) // factor 0.25 rpm/bit
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
			log.Println("sender: shutting down")
			return
		}
	}
}
