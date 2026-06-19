// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	relay "github.com/SoundMatt/RELAY"
	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/virtual"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what
// was written along with fn's error.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

func TestCmdVersionText(t *testing.T) {
	out, err := captureStdout(t, func() error { return cmdVersion(nil) })
	if err != nil {
		t.Fatalf("cmdVersion: %v", err)
	}
	if !strings.Contains(out, toolVersion) || !strings.Contains(out, "go-can") {
		t.Errorf("version text missing fields: %q", out)
	}
}

func TestCmdVersionJSON(t *testing.T) {
	out, err := captureStdout(t, func() error { return cmdVersion([]string{"--format", "json"}) })
	if err != nil {
		t.Fatalf("cmdVersion json: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("version json invalid: %v\n%s", err, out)
	}
	if m["tool"] != "go-can" || m["version"] != toolVersion {
		t.Errorf("version json fields wrong: %v", m)
	}
}

func TestCmdCapabilities(t *testing.T) {
	out, err := captureStdout(t, cmdCapabilities)
	if err != nil {
		t.Fatalf("cmdCapabilities: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("capabilities json invalid: %v\n%s", err, out)
	}
	if m["kind"] != "capabilities" {
		t.Errorf("capabilities kind wrong: %v", m["kind"])
	}
	cmds, ok := m["commands"].([]interface{})
	if !ok || len(cmds) == 0 {
		t.Errorf("capabilities commands missing: %v", m["commands"])
	}
}

func TestCmdStatus(t *testing.T) {
	textOut, err := captureStdout(t, func() error { return cmdStatus(nil) })
	if err != nil {
		t.Fatalf("cmdStatus text: %v", err)
	}
	if !strings.Contains(textOut, "go-can") {
		t.Errorf("status text missing tool name: %q", textOut)
	}

	jsonOut, err := captureStdout(t, func() error { return cmdStatus([]string{"--format", "json"}) })
	if err != nil {
		t.Fatalf("cmdStatus json: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &m); err != nil {
		t.Fatalf("status json invalid: %v\n%s", err, jsonOut)
	}
	if m["healthy"] != true {
		t.Errorf("status healthy = %v, want true", m["healthy"])
	}
}

func TestCmdSendVirtual(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return cmdSend(context.Background(), []string{"virtual", "123#DEADBEEF"})
	})
	if err != nil {
		t.Fatalf("cmdSend: %v", err)
	}
	if !strings.Contains(strings.ToUpper(out), "123#DEADBEEF") {
		t.Errorf("send output missing frame: %q", out)
	}
}

func TestCmdSendErrors(t *testing.T) {
	ctx := context.Background()
	cases := [][]string{
		{},                        // too few args
		{"virtual"},               // missing frame
		{"virtual", "ZZZ#AA"},     // invalid ID
		{"virtual", "123#NOTHEX"}, // invalid data
	}
	for _, args := range cases {
		if err := cmdSend(ctx, args); err == nil {
			t.Errorf("cmdSend(%v) = nil, want error", args)
		}
	}
}

func TestCmdReplayVirtual(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "log.candump")
	logData := "(1609459200.000000) virtual 123#AABB\n" +
		"(1609459200.000100) virtual 456#CCDD\n"
	if err := os.WriteFile(logPath, []byte(logData), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	err := cmdReplay(context.Background(), []string{"virtual", logPath, "--speed", "100"})
	if err != nil {
		t.Fatalf("cmdReplay: %v", err)
	}
}

func TestCmdReplayErrors(t *testing.T) {
	ctx := context.Background()
	if err := cmdReplay(ctx, []string{"virtual"}); err == nil {
		t.Error("cmdReplay with too few args should error")
	}
	if err := cmdReplay(ctx, []string{"virtual", "/no/such/file"}); err == nil {
		t.Error("cmdReplay with missing file should error")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "log.candump")
	_ = os.WriteFile(logPath, []byte("(1.0) virtual 1#FF\n"), 0o644)
	if err := cmdReplay(ctx, []string{"virtual", logPath, "--speed", "notanumber"}); err == nil {
		t.Error("cmdReplay with invalid speed should error")
	}
}

func TestCmdRecordToFile(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.candump")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	// Records from an (empty) virtual bus until the context is cancelled,
	// mirroring how the CLI stops on Ctrl+C (context.Canceled).
	if err := cmdRecord(ctx, []string{"virtual", outPath}); err != nil {
		t.Fatalf("cmdRecord: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("record output file not created: %v", err)
	}
}

func TestCmdRecordError(t *testing.T) {
	if err := cmdRecord(context.Background(), nil); err == nil {
		t.Error("cmdRecord with no args should error")
	}
}

func TestRunConvertStandardFrame(t *testing.T) {
	var out, errb strings.Builder
	in := strings.NewReader(`{"id":291,"data":"3q2+7w=="}`)
	code := runConvert([]string{"--protocol", "CAN"}, in, &out, &errb)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errb.String())
	}
	want := `{"protocol":1,"version":{"major":0,"minor":0,"patch":0},"id":"291","payload":"3q2+7w==","timestamp":"0001-01-01T00:00:00Z","meta":{"can.brs":"false","can.ext":"false","can.fd":"false","can.rtr":"false"}}`
	if strings.TrimSpace(out.String()) != want {
		t.Errorf("convert output mismatch:\n got  %s\n want %s", strings.TrimSpace(out.String()), want)
	}
}

func TestRunConvertXLFrame(t *testing.T) {
	var out, errb strings.Builder
	in := strings.NewReader(`{"id":291,"esi":true,"xl":true,"sdt":5,"vcid":2,"af":51966,"sec":true,"data":"3q2+7w=="}`)
	code := runConvert([]string{"--protocol", "CAN", "--format", "json"}, in, &out, &errb)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errb.String())
	}
	for _, key := range []string{`"can.xl":"true"`, `"can.sdt":"5"`, `"can.vcid":"2"`, `"can.af":"51966"`, `"can.sec":"true"`, `"can.esi":"true"`} {
		if !strings.Contains(out.String(), key) {
			t.Errorf("XL convert output missing %s:\n%s", key, out.String())
		}
	}
}

func TestRunConvertInvalidInput(t *testing.T) {
	var out, errb strings.Builder
	// Standard ID overflow → ValidateFrame fails.
	in := strings.NewReader(`{"id":2048,"data":"AA=="}`)
	code := runConvert([]string{"--protocol", "CAN"}, in, &out, &errb)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if strings.TrimSpace(errb.String()) != "ErrInvalidFrame" {
		t.Errorf("stderr = %q, want ErrInvalidFrame", strings.TrimSpace(errb.String()))
	}
	if out.String() != "" {
		t.Errorf("stdout should be empty on invalid input, got %q", out.String())
	}
}

func TestRunConvertMalformedJSON(t *testing.T) {
	var out, errb strings.Builder
	code := runConvert([]string{"--protocol", "CAN"}, strings.NewReader("not json"), &out, &errb)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 for malformed input", code)
	}
}

func TestRunConvertArgErrors(t *testing.T) {
	cases := [][]string{
		{"--protocol", "DDS"},   // wrong protocol
		{"--protocol"},          // missing value
		{"--format", "yaml", "--protocol", "CAN"}, // unsupported format
		{"--bogus"},             // unknown arg
		{},                      // missing protocol
	}
	for _, args := range cases {
		var out, errb strings.Builder
		code := runConvert(args, strings.NewReader("{}"), &out, &errb)
		if code != 2 {
			t.Errorf("runConvert(%v) exit = %d, want 2", args, code)
		}
	}
}

func TestOpenBusVirtual(t *testing.T) {
	bus, err := openBus("virtual")
	if err != nil {
		t.Fatalf("openBus: %v", err)
	}
	defer bus.Close()
}

func TestUsage(t *testing.T) {
	// usage() writes to stderr; just ensure it runs without panicking.
	old := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w
	usage()
	_ = w.Close()
	os.Stderr = old
}

func TestCmdDumpCancellation(t *testing.T) {
	// cmdDump opens its own virtual bus, subscribes, and blocks until the
	// context is cancelled. Verify it returns nil on cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	if err := cmdDump(ctx, []string{"virtual"}); err != nil {
		t.Errorf("cmdDump returned %v, want nil on cancellation", err)
	}
}

func TestCmdDumpMissingArg(t *testing.T) {
	if err := cmdDump(context.Background(), nil); err == nil {
		t.Error("cmdDump with no iface should error")
	}
}

func TestOpenBusNonVirtualOnNonLinux(t *testing.T) {
	// On this (non-Linux) platform, a SocketCAN iface must return an error
	// rather than silently succeeding.
	if _, err := openBus("can0"); err == nil {
		t.Skip("SocketCAN available on this platform; skipping non-Linux assertion")
	}
}

func TestCmdSendDefaultIface(t *testing.T) {
	// Empty iface falls through openBus to a virtual bus.
	_, err := captureStdout(t, func() error {
		return cmdSend(context.Background(), []string{"", "100#AA"})
	})
	if err != nil {
		t.Fatalf("cmdSend with empty iface: %v", err)
	}
}

// TestRunSendJSON verifies the streaming NDJSON sink publishes each message as
// a CAN frame to the bus (RELAY §11.2 crossbar spoke).
func TestRunSendJSON(t *testing.T) {
	bus, _ := virtual.New()
	defer bus.Close()
	ch, _ := bus.Subscribe(nil)

	m1, _ := json.Marshal(can.Frame{ID: 0x100, Data: []byte{0xAA}}.ToMessage())
	m2, _ := json.Marshal(can.Frame{ID: 0x123, Data: []byte{0xDE, 0xAD}}.ToMessage())
	nd := string(m1) + "\n" + string(m2) + "\n"

	if err := runSendJSON(context.Background(), bus, strings.NewReader(nd)); err != nil {
		t.Fatalf("runSendJSON: %v", err)
	}

	want := []uint32{0x100, 0x123}
	for i, id := range want {
		select {
		case f := <-ch:
			if f.ID != id {
				t.Errorf("frame %d ID = %X, want %X", i, f.ID, id)
			}
		case <-time.After(time.Second):
			t.Fatalf("frame %d not published", i)
		}
	}
}

// TestRunSendJSONInvalid verifies a message with an invalid ID surfaces an error.
func TestRunSendJSONInvalid(t *testing.T) {
	bus, _ := virtual.New()
	defer bus.Close()
	err := runSendJSON(context.Background(), bus, strings.NewReader(`{"protocol":1,"id":"not-a-number"}`+"\n"))
	if err == nil {
		t.Error("expected error for message with invalid ID")
	}
}

// TestRunSubscribeJSON verifies the streaming source writes each received frame
// as one relay.Message NDJSON line, and that --count bounds the stream.
func TestRunSubscribeJSON(t *testing.T) {
	bus, _ := virtual.New()
	defer bus.Close()

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- runSubscribeJSON(context.Background(), bus, &buf, 2) }()

	time.Sleep(30 * time.Millisecond) // let the subscription register
	bus.Send(context.Background(), can.Frame{ID: 0x100, Data: []byte{0xAA}})
	bus.Send(context.Background(), can.Frame{ID: 0x123, Data: []byte{0xDE, 0xAD}})

	if err := <-done; err != nil {
		t.Fatalf("runSubscribeJSON: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d:\n%s", len(lines), buf.String())
	}
	var m relay.Message
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatalf("line 0 not valid JSON: %v", err)
	}
	if m.Protocol != relay.CAN || m.ID != "256" {
		t.Errorf("first message = protocol %v id %q, want CAN/256", m.Protocol, m.ID)
	}
}

// TestRunSubscribeJSONCancel verifies cancellation ends the stream cleanly.
func TestRunSubscribeJSONCancel(t *testing.T) {
	bus, _ := virtual.New()
	defer bus.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runSubscribeJSON(ctx, bus, io.Discard, 0) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runSubscribeJSON returned %v, want nil on cancel", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runSubscribeJSON did not return on cancel")
	}
}

// TestSendJSONSinkMode verifies cmdSend routes to the streaming sink when
// --format json is present.
func TestSendJSONSinkMode(t *testing.T) {
	m, _ := json.Marshal(can.Frame{ID: 0x1, Data: []byte{0xFF}}.ToMessage())
	old := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() { w.Write(append(m, '\n')); w.Close() }()
	err := cmdSend(context.Background(), []string{"--format", "json"})
	os.Stdin = old
	if err != nil {
		t.Fatalf("cmdSend streaming sink: %v", err)
	}
}

func TestJSONSinkIfaceDetection(t *testing.T) {
	if iface, ok := jsonSinkIface([]string{"--format", "json"}); !ok || iface != "virtual" {
		t.Errorf("default: ok=%v iface=%q", ok, iface)
	}
	if iface, ok := jsonSinkIface([]string{"--iface", "vcan0", "--format", "json"}); !ok || iface != "vcan0" {
		t.Errorf("with iface: ok=%v iface=%q", ok, iface)
	}
	if _, ok := jsonSinkIface([]string{"virtual", "100#AA"}); ok {
		t.Error("positional form should not be detected as JSON sink")
	}
}

func TestCmdSubscribeArgErrors(t *testing.T) {
	ctx := context.Background()
	cases := [][]string{
		{"--iface"},            // missing value
		{"--count", "notanum"}, // bad count
		{"--format", "yaml"},   // unsupported format
		{"--bogus"},            // unknown arg
	}
	for _, args := range cases {
		if err := cmdSubscribe(ctx, args); err == nil {
			t.Errorf("cmdSubscribe(%v) = nil, want error", args)
		}
	}
}
