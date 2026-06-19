// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Command go-can is the go-CAN command-line tool (RELAY spec §13.2).
//
// Subcommands:
//
//	version      [--format text|json]          Print version information
//	capabilities                               Print capability declaration (JSON)
//	status       [--format text|json]          Print bus status
//	send         <iface> <id>[#<data>]         Send a CAN frame
//	dump         <iface>                        Dump all received frames to stdout
//	record       <iface> [output-file]          Record frames in candump format
//	replay       <iface> <log-file> [--speed N] Replay a candump log file
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/recorder"
	"github.com/SoundMatt/go-CAN/virtual"
)

const toolVersion = "0.6.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "version":
		err = cmdVersion(os.Args[2:])
	case "capabilities":
		err = cmdCapabilities()
	case "status":
		err = cmdStatus(os.Args[2:])
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
		fmt.Fprintf(os.Stderr, "go-can: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "go-can:", err)
		os.Exit(1)
	}
}

func cmdVersion(args []string) error {
	format := "text"
	for i, a := range args {
		if a == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}
	if format == "json" {
		v := map[string]interface{}{
			"tool":         "go-can",
			"protocol":     "CAN",
			"protocol_int": 1,
			"version":      toolVersion,
			"spec_version": can.SpecVersion,
			"language":     "go",
			"runtime":      runtime.Version(),
		}
		return printJSON(v)
	}
	fmt.Printf("go-can %s (RELAY spec %s, %s)\n", toolVersion, can.SpecVersion, runtime.Version())
	return nil
}

func cmdCapabilities() error {
	cap := map[string]interface{}{
		"kind":                "capabilities",
		"tool":                "go-can",
		"protocol":            "CAN",
		"protocol_int":        1,
		"version":             toolVersion,
		"spec_version":        can.SpecVersion,
		"commands":            []string{"version", "capabilities", "status", "send", "dump", "record", "replay"},
		"transports":          []string{"socketcan", "virtual"},
		"features":            []string{"fd", "isotp", "j1939"},
		"interfaces":          []string{"Bus"},
		"optional_interfaces": []string{},
		"adapt":               true,
	}
	return printJSON(cap)
}

func cmdStatus(args []string) error {
	format := "text"
	for i, a := range args {
		if a == "--format" && i+1 < len(args) {
			format = args[i+1]
		}
	}
	status := map[string]interface{}{
		"protocol":  "CAN",
		"tool":      "go-can",
		"version":   toolVersion,
		"healthy":   true,
		"connected": false,
		"endpoint":  "",
		"details":   map[string]interface{}{},
	}
	if format == "json" {
		return printJSON(status)
	}
	fmt.Printf("tool=%s version=%s protocol=CAN healthy=true connected=false\n", "go-can", toolVersion)
	return nil
}

func cmdSend(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: go-can send <iface> <hex-id>[#<hex-data>]")
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
		return fmt.Errorf("usage: go-can dump <iface>")
	}
	iface := args[0]

	bus, err := openBus(iface)
	if err != nil {
		return err
	}
	defer bus.Close()

	ch, err := bus.Subscribe(nil)
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

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func usage() {
	fmt.Fprintln(os.Stderr, `go-can — CAN bus command-line tool (RELAY spec v`+can.SpecVersion+`)

Usage:
  go-can version       [--format text|json]           Print version information
  go-can capabilities                                 Print capability declaration
  go-can status        [--format text|json]           Print bus status
  go-can send          <iface> <id>[#<data>]          Send a CAN frame
  go-can dump          <iface>                         Dump all received frames
  go-can record        <iface> [output-file]           Record frames in candump format (Ctrl+C to stop)
  go-can replay        <iface> <log-file> [--speed N]  Replay a candump log file (default speed: 1.0)

iface: "virtual" for in-process test bus; "vcan0", "can0", etc. for SocketCAN (Linux only).`)
}
