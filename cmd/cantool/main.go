// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command cantool is a CLI for interacting with CAN buses.
//
// Subcommands:
//
//	send   <iface> <id>[#<data>]              Send a CAN frame
//	dump   <iface>                             Dump all received frames to stdout
//	record <iface> [output-file]              Record frames to stdout or file (candump format)
//	replay <iface> <log-file> [--speed N]     Replay a candump log file
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
	"github.com/SoundMatt/go-CAN/recorder"
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
	case "record":
		err = cmdRecord(ctx, os.Args[2:])
	case "replay":
		err = cmdReplay(ctx, os.Args[2:])
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

// cmdRecord records all frames from iface to an output file (or stdout) in
// candump format. Press Ctrl+C to stop.
//
//	cantool record <iface> [output-file]
func cmdRecord(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cantool record <iface> [output-file]")
	}
	iface := args[0]

	var out *os.File
	if len(args) >= 2 {
		var err error
		out, err = os.Create(args[1])
		if err != nil {
			return fmt.Errorf("record: open output: %w", err)
		}
		defer out.Close()
		fmt.Fprintf(os.Stderr, "recording %s → %s (Ctrl+C to stop)\n", iface, args[1])
	} else {
		out = os.Stdout
		fmt.Fprintf(os.Stderr, "recording %s → stdout (Ctrl+C to stop)\n", iface)
	}

	bus, err := openBus(iface)
	if err != nil {
		return err
	}
	defer bus.Close()

	if err := recorder.Record(ctx, bus, out, iface); err != nil && err != context.Canceled {
		return fmt.Errorf("record: %w", err)
	}
	return nil
}

// cmdReplay replays a candump log file to iface, preserving frame timing.
//
//	cantool replay <iface> <log-file> [--speed N]
func cmdReplay(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cantool replay <iface> <log-file> [--speed N]")
	}
	iface := args[0]
	logFile := args[1]

	speedFactor := 1.0
	for i := 2; i < len(args)-1; i++ {
		if args[i] == "--speed" {
			v, err := strconv.ParseFloat(args[i+1], 64)
			if err != nil {
				return fmt.Errorf("replay: invalid speed %q: %w", args[i+1], err)
			}
			speedFactor = v
		}
	}

	f, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("replay: open log: %w", err)
	}
	defer f.Close()

	bus, err := openBus(iface)
	if err != nil {
		return err
	}
	defer bus.Close()

	fmt.Fprintf(os.Stderr, "replaying %s → %s at %.1f× speed (Ctrl+C to stop)\n",
		logFile, iface, speedFactor)

	if err := recorder.Replay(ctx, bus, f, speedFactor); err != nil && err != context.Canceled {
		return fmt.Errorf("replay: %w", err)
	}
	return nil
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
  cantool send   <iface> <id>[#<data>]              Send a CAN frame
  cantool dump   <iface>                             Dump all received frames
  cantool record <iface> [output-file]              Record frames in candump format (Ctrl+C to stop)
  cantool replay <iface> <log-file> [--speed N]     Replay a candump log file (default speed: 1.0)

iface: "virtual" for in-process test bus; "vcan0", "can0", etc. for SocketCAN (Linux only).`)
}
