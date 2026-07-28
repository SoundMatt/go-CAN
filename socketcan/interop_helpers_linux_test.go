// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socketcan_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// peerBinPath is the built cmd/can-interop-peer executable, populated once
// by TestMain when CAN_INTEROP_TESTS=1 (see requireCANInterop). It is empty
// (and unused) for every other test run, matching this package's existing
// "skip gracefully when the environment can't support it" posture
// (requireVCAN in bus_linux_test.go).
var peerBinPath string

// TestMain builds cmd/can-interop-peer exactly once per test binary run,
// before any interop test needs it, rather than each test shelling out to
// `go build` independently.
func TestMain(m *testing.M) {
	var buildDir string
	if os.Getenv("CAN_INTEROP_TESTS") == "1" {
		dir, err := os.MkdirTemp("", "can-interop-peer-")
		if err != nil {
			fmt.Fprintln(os.Stderr, "can-interop: MkdirTemp:", err)
		} else {
			buildDir = dir
			bin := filepath.Join(dir, "can-interop-peer")
			cmd := exec.Command("go", "build", "-o", bin, "github.com/SoundMatt/go-CAN/cmd/can-interop-peer")
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "can-interop: failed to build cmd/can-interop-peer:", err)
			} else {
				peerBinPath = bin
			}
		}
	}

	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

// requireCANInterop skips the test unless CAN_INTEROP_TESTS=1 is set (the
// live two-process and can-utils third-party interop tests are deliberately
// absent from the fast default `go test ./...` sweep and the existing
// test-socketcan CI job — they run only in the new can-interop CI job, see
// .github/workflows/ci.yml) and vcan0 is actually available.
func requireCANInterop(t *testing.T) {
	t.Helper()
	if os.Getenv("CAN_INTEROP_TESTS") != "1" {
		t.Skip("CAN_INTEROP_TESTS != 1 — live interop tests only run in the can-interop CI job")
	}
	requireVCAN(t)
	if peerBinPath == "" {
		t.Fatal("can-interop-peer was not built successfully (see TestMain output above)")
	}
}

// requireCANUtils skips the test if the can-utils tools this test drives as
// a genuine third-party validator are not installed.
func requireCANUtils(t *testing.T, bins ...string) {
	t.Helper()
	for _, bin := range bins {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not found in PATH — install can-utils to run this test", bin)
		}
	}
}

// peerFrame mirrors cmd/can-interop-peer's frameJSON. All fields are
// comparable (no slices/maps) so peerFrame values can be compared with ==.
type peerFrame struct {
	ID      uint32 `json:"id"`
	Ext     bool   `json:"ext"`
	RTR     bool   `json:"rtr"`
	FD      bool   `json:"fd"`
	BRS     bool   `json:"brs"`
	DataHex string `json:"data_hex"`
}

// peerReport mirrors cmd/can-interop-peer's report.
type peerReport struct {
	Role     string      `json:"role"`
	OK       bool        `json:"ok"`
	Sent     []peerFrame `json:"sent"`
	Received []peerFrame `json:"received"`
	Error    string      `json:"error"`
}

func parsePeerReport(stdout []byte) (peerReport, error) {
	var rep peerReport
	trimmed := strings.TrimSpace(string(stdout))
	if trimmed == "" {
		return rep, fmt.Errorf("peer produced no stdout")
	}
	lines := strings.Split(trimmed, "\n")
	last := lines[len(lines)-1]
	if err := json.Unmarshal([]byte(last), &rep); err != nil {
		return rep, fmt.Errorf("peer stdout not a valid JSON report: %w (line: %q)", err, last)
	}
	return rep, nil
}

type peerResultMsg struct {
	report peerReport
	err    error
}

// runningPeer is a can-interop-peer process started with its stdout/stderr
// piped and drained concurrently, so the caller can watch stderr for the
// "READY" line while the process is still running and safely retrieve its
// final JSON report once it exits.
type runningPeer struct {
	cmd    *exec.Cmd
	ready  chan struct{}
	result chan peerResultMsg
}

// startPeer starts cmd/can-interop-peer with the given args.
func startPeer(t *testing.T, args ...string) *runningPeer {
	t.Helper()
	cmd := exec.Command(peerBinPath, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s %v: %v", peerBinPath, args, err)
	}

	p := &runningPeer{cmd: cmd, ready: make(chan struct{}), result: make(chan peerResultMsg, 1)}

	var stdout []byte
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		data, _ := io.ReadAll(stdoutPipe)
		stdout = data
	}()
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderrPipe)
		signalled := false
		for sc.Scan() {
			if !signalled && sc.Text() == "READY" {
				signalled = true
				close(p.ready)
			}
		}
	}()

	go func() {
		// Per os/exec's docs, Wait must not be called until all reads from
		// the pipes have completed.
		wg.Wait()
		waitErr := cmd.Wait()
		rep, parseErr := parsePeerReport(stdout)
		if waitErr == nil {
			waitErr = parseErr
		}
		p.result <- peerResultMsg{report: rep, err: waitErr}
	}()

	return p
}

// waitReady blocks until the peer signals READY on stderr, exits early, or
// timeout elapses.
func (p *runningPeer) waitReady(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-p.ready:
	case res := <-p.result:
		t.Fatalf("peer exited before signalling READY: %+v (err=%v)", res.report, res.err)
	case <-time.After(timeout):
		_ = p.cmd.Process.Kill()
		t.Fatalf("peer did not signal READY within %s", timeout)
	}
}

// waitDone blocks until the peer process exits and returns its parsed
// report.
func (p *runningPeer) waitDone(t *testing.T, timeout time.Duration) peerReport {
	t.Helper()
	select {
	case res := <-p.result:
		if res.err != nil && res.report.Role == "" {
			t.Fatalf("peer failed: %v", res.err)
		}
		return res.report
	case <-time.After(timeout):
		_ = p.cmd.Process.Kill()
		t.Fatalf("peer did not exit within %s", timeout)
		return peerReport{}
	}
}
