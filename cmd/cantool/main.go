// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command cantool is a CLI for interacting with CAN buses.
//
// Subcommands:
//
//	send   <iface> <id> <hex-data>   Send a CAN frame
//	dump   <iface>                   Dump all received frames to stdout
//	decode <dbc-file> <iface>        Decode frames using a DBC file
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/virtual"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "send":
		err = cmdSend(ctx, os.Args[2:])
	case "dump":
		err = cmdDump(ctx, os.Args[2:])
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "cantool: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "cantool:", err)
		os.Exit(1)
	}
}

func cmdSend(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantool send <iface> <hex-id>[#<hex-data>]")
	}
	iface := args[0]
	frameStr := args[1]

	idStr, dataStr, _ := strings.Cut(frameStr, "#")
	id64, err := strconv.ParseUint(strings.TrimPrefix(idStr, "0x"), 16, 32)
	if err != nil {
		return fmt.Errorf("invalid CAN ID %q: %w", idStr, err)
	}

	var data []byte
	if dataStr != "" {
		data, err = hex.DecodeString(strings.ReplaceAll(dataStr, " ", ""))
		if err != nil {
			return fmt.Errorf("invalid data %q: %w", dataStr, err)
		}
	}

	bus, err := openBus(iface)
	if err != nil {
		return err
	}
	defer bus.Close()

	f := can.Frame{ID: uint32(id64), Data: data}
	if err := bus.Send(ctx, f); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	fmt.Printf("sent  %03X#%X\n", f.ID, f.Data)
	return nil
}

func cmdDump(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantool dump <iface>")
	}
	iface := args[0]

	bus, err := openBus(iface)
	if err != nil {
		return err
	}
	defer bus.Close()

	ch, err := bus.Subscribe()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "dumping %s — press Ctrl+C to stop\n", iface)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return nil
			}
			fmt.Printf("%03X#%X\n", f.ID, f.Data)
		case <-ctx.Done():
			return nil
		}
	}
}

// openBus returns a virtual bus when iface == "virtual", otherwise tries
// to open a SocketCAN interface. On non-Linux systems virtual is always used.
func openBus(iface string) (can.Bus, error) {
	if iface == "virtual" || iface == "" {
		return virtual.New()
	}
	return openPlatformBus(iface)
}

func usage() {
	fmt.Fprintln(os.Stderr, `cantool — CAN bus command-line tool

Usage:
  cantool send  <iface> <id>[#<data>]   Send a CAN frame (e.g. cantool send vcan0 123#DEADBEEF)
  cantool dump  <iface>                  Dump all received frames

iface: "virtual" for in-process test bus; "vcan0", "can0", etc. for SocketCAN (Linux only).`)
}
