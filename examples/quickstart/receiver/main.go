// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command receiver subscribes to CAN frames on the virtual bus and decodes
// engine speed using a hardcoded signal definition.
// Part of the Docker Quickstart.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/virtual"
)

func main() {
	bus, err := virtual.New()
	if err != nil {
		log.Fatalf("virtual.New: %v", err)
	}
	defer bus.Close()

	ch, err := bus.Subscribe([]can.Filter{{ID: 0x100, Mask: 0x7FF}})
	if err != nil {
		log.Fatalf("Subscribe: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	hostname, _ := os.Hostname()
	log.Printf("receiver [%s]: waiting for frames on ID 0x100", hostname)

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
			log.Println("receiver: shutting down")
			return
		}
	}
}
