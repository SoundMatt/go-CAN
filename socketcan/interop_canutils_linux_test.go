// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Third-party-peer SocketCAN interop: CAN has no equivalent of a second
// independent network stack to test against (unlike, say, DDS's
// CycloneDDS) — the OS kernel's own SocketCAN subsystem plus can-utils
// (cangen/candump) *is* the real wire and a genuine, independent-of-go-CAN
// validator of both directions:
//
//   - cangen injects real CAN frames onto vcan0; go-CAN's socketcan.Bus
//     receives and decodes them. cangen's frame-generation flags (-I id,
//     -L length, -D data, -e extended, -R RTR, -f FD, -b BRS) are used in
//     their fixed/deterministic modes (never 'r' random), so each
//     invocation's ground truth is exactly the flags this test passed it —
//     no need to parse cangen's own -v log text to find out what it did.
//   - go-CAN's own socketcan.Bus (via cmd/can-interop-peer) sends frames;
//     `candump -L` (log-file format on stdout — real kernel receive path,
//     independent decoder from go-CAN's) captures them, and this test
//     parses candump's documented log-line format
//     ("id#data" / "id#R" / "id##<flags><data>", see candump.c's
//     snprintf_canframe) and asserts it matches the wire encoding go-CAN
//     was supposed to produce.
//
// Gated behind CAN_INTEROP_TESTS=1 (requireCANInterop) plus a can-utils
// presence check (requireCANUtils) — absent from the fast default
// `go test ./...` sweep and the existing test-socketcan CI job. Runs in the
// new can-interop CI job (.github/workflows/ci.yml), which installs
// can-utils before running these tests and skips the job cleanly (not a
// hard failure) if vcan/can-utils setup does not succeed.
package socketcan_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// canGenVector is one deterministic cangen invocation and the frame go-CAN's
// SocketCAN receiver is expected to decode from it.
type canGenVector struct {
	name string
	args []string // cangen flags, excluding -g/-n/the interface (added by the test)
	want peerFrame
}

func canGenVectors() []canGenVector {
	return []canGenVector{
		{
			name: "standard ID, 4-byte data",
			args: []string{"-I", "123", "-L", "4", "-D", "DEADBEEF"},
			want: peerFrame{ID: 0x123, DataHex: "deadbeef"},
		},
		{
			name: "standard ID, zero-length data",
			args: []string{"-I", "0", "-L", "0"},
			want: peerFrame{ID: 0, DataHex: ""},
		},
		{
			name: "extended ID, max classic 8-byte data",
			args: []string{"-e", "-I", "1FFFFFFF", "-L", "8", "-D", "0102030405060708"},
			want: peerFrame{ID: 0x1FFFFFFF, Ext: true, DataHex: "0102030405060708"},
		},
		{
			name: "extended ID, 1-byte data",
			args: []string{"-e", "-I", "ABCDE", "-L", "1", "-D", "AA"},
			want: peerFrame{ID: 0xABCDE, Ext: true, DataHex: "aa"},
		},
		{
			name: "standard ID, RTR",
			args: []string{"-R", "-I", "321", "-L", "0"},
			want: peerFrame{ID: 0x321, RTR: true, DataHex: ""},
		},
		{
			name: "standard ID, CAN FD with BRS",
			args: []string{"-f", "-b", "-I", "400", "-L", "20", "-D", strings.Repeat("AB", 20)},
			want: peerFrame{ID: 0x400, FD: true, BRS: true, DataHex: strings.Repeat("ab", 20)},
		},
		{
			name: "extended ID, CAN FD without BRS, 64-byte data",
			args: []string{"-f", "-e", "-I", "1234567", "-L", "64", "-D", strings.Repeat("CD", 64)},
			want: peerFrame{ID: 0x1234567, Ext: true, FD: true, DataHex: strings.Repeat("cd", 64)},
		},
	}
}

//fusa:test REQ-SCAN-001
//fusa:test REQ-SCAN-005
//fusa:test REQ-SCAN-006
//fusa:test REQ-SCAN-007
//fusa:test REQ-SCAN-008
func TestCanUtilsCangenIntoGoCAN(t *testing.T) {
	requireCANInterop(t)
	requireCANUtils(t, "cangen")
	const iface = "vcan0"

	vectors := canGenVectors()

	reader := startPeer(t, "--role", "reader", "--iface", iface,
		"--count", strconv.Itoa(len(vectors)), "--timeout-secs", "10")
	reader.waitReady(t, 5*time.Second)

	for _, v := range vectors {
		args := append([]string{"-g", "0", "-n", "1"}, v.args...)
		args = append(args, iface)
		cmd := exec.Command("cangen", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("cangen %v (%s): %v\nstderr: %s", args, v.name, err, stderr.String())
		}
	}

	rep := reader.waitDone(t, 10*time.Second)
	if !rep.OK {
		t.Fatalf("go-CAN reader process reported failure: %+v", rep)
	}
	if len(rep.Received) != len(vectors) {
		t.Fatalf("frame count mismatch: cangen sent %d frames, go-CAN received %d\nreceived: %+v",
			len(vectors), len(rep.Received), rep.Received)
	}
	for i, v := range vectors {
		got := rep.Received[i]
		if got != v.want {
			t.Errorf("vector %d (%s): go-CAN decoded cangen's frame incorrectly:\n  cangen sent (ground truth): %+v\n  go-CAN decoded:             %+v",
				i, v.name, v.want, got)
		}
	}
}

//fusa:test REQ-SCAN-001
//fusa:test REQ-SCAN-003
//fusa:test REQ-SCAN-004
//fusa:test REQ-SCAN-007
//fusa:test REQ-SCAN-008
func TestGoCANIntoCanUtilsCandump(t *testing.T) {
	requireCANInterop(t)
	requireCANUtils(t, "candump")
	const iface = "vcan0"

	// candump -L: log-file format on stdout, real independent kernel-facing
	// decoder. -n <count> makes it exit on its own once it has received
	// exactly as many frames as the writer process will send; -T is a
	// safety net so the test cannot hang if fewer frames ever arrive. The
	// count comes from the peer binary itself (--print-vector-count) rather
	// than a second, drift-prone hardcoded copy of defaultVectors()'s length.
	writerVectorCount := queryWriterVectorCount(t)
	candumpCmd := exec.Command("candump", "-L",
		"-n", strconv.Itoa(writerVectorCount),
		"-T", "8000",
		iface,
	)
	var candumpOut bytes.Buffer
	candumpCmd.Stdout = &candumpOut
	var candumpErr bytes.Buffer
	candumpCmd.Stderr = &candumpErr
	if err := candumpCmd.Start(); err != nil {
		t.Fatalf("start candump: %v", err)
	}
	// candump binds its raw socket during startup; give it a moment before
	// the writer process starts sending, same "must be listening first"
	// requirement as the go-CAN reader peer (see interop_helpers's READY
	// handshake — candump has no equivalent stdout signal, so this uses a
	// short fixed delay instead).
	time.Sleep(300 * time.Millisecond)

	writer := startPeer(t, "--role", "writer", "--iface", iface)
	writerReport := writer.waitDone(t, 10*time.Second)
	if !writerReport.OK {
		t.Fatalf("writer process reported failure: %+v", writerReport)
	}

	if err := candumpCmd.Wait(); err != nil {
		t.Fatalf("candump did not exit cleanly: %v\nstderr: %s\nstdout so far:\n%s",
			err, candumpErr.String(), candumpOut.String())
	}

	got, err := parseCandumpLog(candumpOut.String())
	if err != nil {
		t.Fatalf("failed to parse candump -L output: %v\nraw output:\n%s", err, candumpOut.String())
	}

	want := writerReport.Sent
	if len(got) != len(want) {
		t.Fatalf("frame count mismatch: go-CAN sent %d frames, candump captured %d\nsent:     %+v\ncaptured: %+v\nraw candump output:\n%s",
			len(want), len(got), want, got, candumpOut.String())
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d wire-encoding mismatch:\n  go-CAN sent (ground truth): %+v\n  candump captured on wire:  %+v",
				i, want[i], got[i])
		}
	}
}

// queryWriterVectorCount asks cmd/can-interop-peer how many frames its
// writer role will send (--print-vector-count), so candump's -n bound can
// be set exactly, before the writer process ever runs.
func queryWriterVectorCount(t *testing.T) int {
	t.Helper()
	out, err := exec.Command(peerBinPath, "--print-vector-count").Output()
	if err != nil {
		t.Fatalf("query --print-vector-count: %v", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("parse --print-vector-count output %q: %v", out, err)
	}
	return n
}

// parseCandumpLog parses candump -L's log-file-format-on-stdout output
// (see candump.c's snprintf_canframe / sprint_timestamp): each line is
// "(sec.usec) ifname frame", where frame is one of:
//
//	<3-or-8-hex-digit-id>#<hex-data>              classic data frame
//	<3-or-8-hex-digit-id>#R                       classic RTR frame (no data)
//	<3-or-8-hex-digit-id>##<flags-hex><hex-data>  CAN FD frame
//
// ID length determines Ext (3 hex digits = standard, 8 = extended). The FD
// flags nibble's bit 0 is BRS (CANFD_BRS = 0x01), bit 1 is ESI
// (CANFD_ESI = 0x02) — see socketcan/canfd_linux.go's matching constants.
func parseCandumpLog(raw string) ([]peerFrame, error) {
	var frames []peerFrame
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("unexpected candump line (want '(ts) iface frame'): %q", line)
		}
		frameField := fields[len(fields)-1]

		hashIdx := strings.IndexByte(frameField, '#')
		if hashIdx < 0 {
			return nil, fmt.Errorf("no '#' in candump frame field: %q (line: %q)", frameField, line)
		}
		idHex := frameField[:hashIdx]
		rest := frameField[hashIdx+1:]

		var f peerFrame
		switch len(idHex) {
		case 3:
			f.Ext = false
		case 8:
			f.Ext = true
		default:
			return nil, fmt.Errorf("unexpected CAN ID field width %d in %q (line: %q)", len(idHex), frameField, line)
		}
		id, err := strconv.ParseUint(idHex, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("parse CAN ID %q: %w (line: %q)", idHex, err, line)
		}
		f.ID = uint32(id)

		switch {
		case strings.HasPrefix(rest, "#"):
			// CAN FD: "#" + one flags hex digit + data hex.
			f.FD = true
			flagsAndData := rest[1:]
			if len(flagsAndData) < 1 {
				return nil, fmt.Errorf("CAN FD frame missing flags digit: %q (line: %q)", frameField, line)
			}
			flags, err := strconv.ParseUint(flagsAndData[:1], 16, 8)
			if err != nil {
				return nil, fmt.Errorf("parse CAN FD flags %q: %w (line: %q)", flagsAndData[:1], err, line)
			}
			f.BRS = flags&0x01 != 0
			data, err := hex.DecodeString(flagsAndData[1:])
			if err != nil {
				return nil, fmt.Errorf("parse CAN FD data %q: %w (line: %q)", flagsAndData[1:], err, line)
			}
			f.DataHex = hex.EncodeToString(data)
		case rest == "R":
			f.RTR = true
			f.DataHex = ""
		default:
			data, err := hex.DecodeString(rest)
			if err != nil {
				return nil, fmt.Errorf("parse classic data %q: %w (line: %q)", rest, err, line)
			}
			f.DataHex = hex.EncodeToString(data)
		}

		frames = append(frames, f)
	}
	return frames, nil
}
