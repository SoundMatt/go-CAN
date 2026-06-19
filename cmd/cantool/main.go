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
//	send         [--iface X] --format json     Streaming sink: publish relay.Message NDJSON from stdin
//	subscribe    [--iface X] [--count N]        Streaming source: print relay.Message NDJSON to stdout
//	dump         <iface>                        Dump all received frames to stdout
//	record       <iface> [output-file]          Record frames in candump format
//	replay       <iface> <log-file> [--speed N] Replay a candump log file
//	convert      --protocol CAN [--format json] RELAY interop driver (stdin->stdout)
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	relay "github.com/SoundMatt/RELAY"
	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/recorder"
	"github.com/SoundMatt/go-CAN/virtual"
)

const toolVersion = "0.9.0"

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
	case "subscribe":
		err = cmdSubscribe(ctx, os.Args[2:])
	case "record":
		err = cmdRecord(ctx, os.Args[2:])
	case "replay":
		err = cmdReplay(ctx, os.Args[2:])
	case "convert":
		// convert has its own exit-code contract (RELAY spec §11.2):
		// 0 = converted, 1 = invalid input, 2 = invalid args.
		os.Exit(runConvert(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
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
		"commands":            []string{"version", "capabilities", "status", "send", "subscribe", "dump", "record", "replay", "convert"},
		"transports":          []string{"socketcan", "virtual"},
		"features":            []string{"fd", "xl", "isotp", "j1939"},
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
	// Streaming JSON sink (RELAY spec §11.2 crossbar spoke): `send --format json`
	// with no positional frame reads relay.Message NDJSON from stdin and
	// publishes each until EOF. This is the egress dual of `subscribe`.
	if iface, ok := jsonSinkIface(args); ok {
		bus, err := openBus(iface)
		if err != nil {
			return err
		}
		defer bus.Close()
		return runSendJSON(ctx, bus, os.Stdin)
	}

	if len(args) < 2 {
		return fmt.Errorf("usage: go-can send <iface> <hex-id>[#<hex-data>]  |  go-can send [--iface X] --format json")
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

// jsonSinkIface reports whether args select the streaming JSON sink mode
// (`--format json` with no positional frame) and returns the chosen iface
// (default "virtual"). It accepts an optional `--iface <name>` flag.
func jsonSinkIface(args []string) (string, bool) {
	iface := "virtual"
	jsonMode := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) && args[i+1] == "json" {
				jsonMode = true
				i++
			}
		case "--iface":
			if i+1 < len(args) {
				iface = args[i+1]
				i++
			}
		}
	}
	return iface, jsonMode
}

// runSendJSON reads relay.Message values as NDJSON from r and publishes each as
// a CAN frame to bus until EOF (RELAY spec §11.2 streaming JSON sink).
func runSendJSON(ctx context.Context, bus can.Bus, r io.Reader) error {
	dec := json.NewDecoder(r)
	for {
		var msg relay.Message
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("send: decode message: %w", err)
		}
		f, err := can.FromMessage(msg)
		if err != nil {
			return fmt.Errorf("send: %w", err)
		}
		if err := bus.Send(ctx, f); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
}

// cmdSubscribe implements `subscribe [--iface X] [--format json] [--count N]`
// (RELAY spec §11.2): it prints every received frame as a relay.Message NDJSON
// line on stdout — the crossbar source dual of the streaming `send` sink.
func cmdSubscribe(ctx context.Context, args []string) error {
	iface := "virtual"
	format := "json"
	count := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--iface":
			if i+1 >= len(args) {
				return fmt.Errorf("subscribe: --iface requires a value")
			}
			iface = args[i+1]
			i++
		case "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("subscribe: --format requires a value")
			}
			format = args[i+1]
			i++
		case "--count":
			if i+1 >= len(args) {
				return fmt.Errorf("subscribe: --count requires a value")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("subscribe: invalid --count %q: %w", args[i+1], err)
			}
			count = n
			i++
		default:
			return fmt.Errorf("subscribe: unknown argument %q", args[i])
		}
	}
	if format != "json" {
		return fmt.Errorf("subscribe: only --format json is supported (got %q)", format)
	}

	bus, err := openBus(iface)
	if err != nil {
		return err
	}
	defer bus.Close()
	return runSubscribeJSON(ctx, bus, os.Stdout, count)
}

// runSubscribeJSON writes each frame received from bus as a relay.Message
// NDJSON line to w. It stops when ctx is done, the bus closes, or count
// messages have been written (count <= 0 means run until cancelled).
func runSubscribeJSON(ctx context.Context, bus can.Bus, w io.Writer, count int) error {
	ch, err := bus.Subscribe(nil)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w) // Encode writes one JSON value + "\n" → NDJSON
	n := 0
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return nil
			}
			if err := enc.Encode(f.ToMessage()); err != nil {
				return fmt.Errorf("subscribe: encode message: %w", err)
			}
			n++
			if count > 0 && n >= count {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// runConvert implements the RELAY interop driver command (spec §11.2):
//
//	convert --protocol CAN [--format json]
//
// It reads one canonical can.Frame as JSON on stdin, runs it through this
// implementation's own ToMessage() conversion, and writes the resulting
// relay.Message as JSON on stdout (timestamp zeroed for comparability). The
// output is a faithful witness of runtime behaviour, so `relay interop` can
// diff it against other CAN implementations for byte-identical equality.
//
// Exit codes: 0 converted, 1 invalid input (sentinel name on stderr),
// 2 invalid arguments.
func runConvert(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	protocol := ""
	format := "json"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--protocol":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "convert: --protocol requires a value")
				return 2
			}
			protocol = args[i+1]
			i++
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "convert: --format requires a value")
				return 2
			}
			format = args[i+1]
			i++
		default:
			fmt.Fprintf(stderr, "convert: unknown argument %q\n", args[i])
			return 2
		}
	}
	if protocol != "CAN" {
		fmt.Fprintf(stderr, "convert: --protocol must be CAN (got %q)\n", protocol)
		return 2
	}
	if format != "json" {
		fmt.Fprintf(stderr, "convert: unsupported --format %q (only json)\n", format)
		return 2
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "convert: read stdin: %v\n", err)
		return 2
	}

	var f can.Frame
	if err := json.Unmarshal(data, &f); err != nil {
		// Unparseable canonical input is treated as invalid input.
		fmt.Fprintln(stderr, "ErrInvalidFrame")
		return 1
	}
	if err := can.ValidateFrame(f); err != nil {
		// RELAY §5 sentinel name for a structurally invalid CAN frame.
		fmt.Fprintln(stderr, "ErrInvalidFrame")
		return 1
	}

	msg := f.ToMessage()
	msg.Timestamp = time.Time{} // zeroed to keep interop comparisons stable
	out, err := json.Marshal(msg)
	if err != nil {
		fmt.Fprintf(stderr, "convert: marshal: %v\n", err)
		return 2
	}
	fmt.Fprintln(stdout, string(out))
	return 0
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
  go-can send          [--iface X] --format json       Streaming sink: publish relay.Message NDJSON read from stdin
  go-can subscribe     [--iface X] [--count N]          Streaming source: print received frames as relay.Message NDJSON
  go-can dump          <iface>                         Dump all received frames
  go-can record        <iface> [output-file]           Record frames in candump format (Ctrl+C to stop)
  go-can replay        <iface> <log-file> [--speed N]  Replay a candump log file (default speed: 1.0)
  go-can convert       --protocol CAN [--format json]  RELAY interop driver: can.Frame JSON (stdin) -> relay.Message JSON (stdout)

iface: "virtual" for in-process test bus; "vcan0", "can0", etc. for SocketCAN (Linux only).`)
}
