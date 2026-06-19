// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package recorder provides candump-compatible CAN frame recording and replay.
//
// Record captures frames from a Bus to an io.Writer in candump format.
// Replay reads a candump log and re-sends frames to a Bus, preserving timing.
//
// Candump format (one frame per line):
//
//	(timestamp) iface can_id#hexdata
//	(timestamp) iface can_id##flagshexdata   (CAN FD)
//
// Example:
//
//	(1609459200.000000) vcan0 123#0102030405060708
//	(1609459200.050000) vcan0 1FFFFFFF#DEADBEEF
package recorder

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	can "github.com/SoundMatt/go-CAN"
)

// Record records all frames from bus to w in candump format.
// iface is the interface name written to each line (e.g. "vcan0").
// Blocks until ctx is cancelled or the bus subscription channel closes.
//
//fusa:req REQ-REC-001
func Record(ctx context.Context, bus can.Bus, w io.Writer, iface string) error {
	ch, err := bus.Subscribe(nil)
	if err != nil {
		return fmt.Errorf("recorder: subscribe: %w", err)
	}

	bw := bufio.NewWriter(w)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return bw.Flush()
			}
			line := FormatLine(iface, time.Now(), f)
			if _, err := fmt.Fprintln(bw, line); err != nil {
				return fmt.Errorf("recorder: write: %w", err)
			}
		case <-ctx.Done():
			_ = bw.Flush()
			return ctx.Err()
		}
	}
}

// Replay reads candump lines from r and sends each frame to bus.
// Timing is preserved relative to the first frame: if frame 2 appears 50 ms
// after frame 1 in the log, Replay sleeps 50 ms between sends.
// speedFactor scales inter-frame delays: 1.0 = real-time, 2.0 = 2× speed.
// Returns nil on EOF or a non-nil error on context cancellation or send failure.
//
//fusa:req REQ-REC-002
func Replay(ctx context.Context, bus can.Bus, r io.Reader, speedFactor float64) error {
	if speedFactor <= 0 {
		speedFactor = 1.0
	}

	scanner := bufio.NewScanner(r)

	var logT0 time.Time // timestamp of first frame in log
	var wall0 time.Time // wall-clock time when first frame was processed
	first := true

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		_, ts, f, err := ParseLine(line)
		if err != nil {
			// skip malformed lines
			continue
		}

		if first {
			logT0 = ts
			wall0 = time.Now()
			first = false
		} else {
			// How long after logT0 should this frame be sent (scaled)?
			logDelay := ts.Sub(logT0)
			scaledDelay := time.Duration(float64(logDelay) / speedFactor)

			// How much wall time has already elapsed since wall0?
			elapsed := time.Since(wall0)
			sleep := scaledDelay - elapsed
			if sleep > 0 {
				select {
				case <-time.After(sleep):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		if err := bus.Send(ctx, f); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("recorder: replay send: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("recorder: replay read: %w", err)
	}
	return nil
}

// ParseLine parses a single candump line and returns the interface name,
// timestamp, and Frame. Returns an error for malformed lines.
//
// Supported formats:
//
//	(1609459200.000000) vcan0 123#DEADBEEF
//	(1609459200.000000) vcan0 123##0DEADBEEF   (CAN FD, flags byte after ##)
//
//fusa:req REQ-REC-003
func ParseLine(line string) (iface string, ts time.Time, f can.Frame, err error) {
	// Expected: (TIMESTAMP) IFACE ID#DATA  or  (TIMESTAMP) IFACE ID##FLAGSDATA
	parts := strings.Fields(line)
	if len(parts) != 3 {
		err = fmt.Errorf("recorder: expected 3 fields, got %d: %q", len(parts), line)
		return
	}

	// Parse timestamp: (SECONDS.MICROS)
	tsStr := parts[0]
	if !strings.HasPrefix(tsStr, "(") || !strings.HasSuffix(tsStr, ")") {
		err = fmt.Errorf("recorder: malformed timestamp %q", tsStr)
		return
	}
	tsStr = tsStr[1 : len(tsStr)-1]
	// Parse as integer seconds + fractional microseconds to avoid float64
	// precision loss. Format is always SECONDS.MICROS (6 decimal places).
	dotIdx := strings.IndexByte(tsStr, '.')
	var sec int64
	var usec int64
	if dotIdx < 0 {
		// No fractional part
		sec, err = parseInt64(tsStr)
		if err != nil {
			err = fmt.Errorf("recorder: invalid timestamp %q: %w", tsStr, err)
			return
		}
	} else {
		sec, err = parseInt64(tsStr[:dotIdx])
		if err != nil {
			err = fmt.Errorf("recorder: invalid timestamp %q: %w", tsStr, err)
			return
		}
		fracStr := tsStr[dotIdx+1:]
		// Pad or truncate to 6 digits (microseconds).
		for len(fracStr) < 6 {
			fracStr += "0"
		}
		fracStr = fracStr[:6]
		usec, err = parseInt64(fracStr)
		if err != nil {
			err = fmt.Errorf("recorder: invalid timestamp %q: %w", tsStr, err)
			return
		}
	}
	ts = time.Unix(sec, usec*1000).UTC()

	iface = parts[1]

	// Parse ID##FLAGSDATA (CAN FD) or ID#DATA (standard CAN)
	frameStr := parts[2]
	if idx := strings.Index(frameStr, "##"); idx != -1 {
		// CAN FD frame
		idStr := frameStr[:idx]
		rest := frameStr[idx+2:] // flags byte + data hex

		id64, parseErr := strconv.ParseUint(idStr, 16, 32)
		if parseErr != nil {
			err = fmt.Errorf("recorder: invalid CAN ID %q: %w", idStr, parseErr)
			return
		}
		f.ID = uint32(id64)
		f.FD = true

		if len(rest) < 2 {
			err = fmt.Errorf("recorder: FD frame missing flags byte: %q", frameStr)
			return
		}
		flagsBytes, decErr := hex.DecodeString(rest[:2])
		if decErr != nil {
			err = fmt.Errorf("recorder: invalid FD flags byte %q: %w", rest[:2], decErr)
			return
		}
		flags := flagsBytes[0]
		f.BRS = flags&0x01 != 0
		// ESI (bit 1) is informational; no Frame field for it currently

		if len(rest) > 2 {
			f.Data, err = hex.DecodeString(rest[2:])
			if err != nil {
				err = fmt.Errorf("recorder: invalid FD data %q: %w", rest[2:], err)
				return
			}
		}
	} else if idx := strings.Index(frameStr, "#"); idx != -1 {
		// Standard CAN frame
		idStr := frameStr[:idx]
		dataStr := frameStr[idx+1:]

		id64, parseErr := strconv.ParseUint(idStr, 16, 32)
		if parseErr != nil {
			err = fmt.Errorf("recorder: invalid CAN ID %q: %w", idStr, parseErr)
			return
		}
		f.ID = uint32(id64)

		// IDs > 0x7FF are extended (29-bit).
		if f.ID > 0x7FF {
			f.Ext = true
		}

		if dataStr != "" {
			f.Data, err = hex.DecodeString(dataStr)
			if err != nil {
				err = fmt.Errorf("recorder: invalid data %q: %w", dataStr, err)
				return
			}
		}
	} else {
		err = fmt.Errorf("recorder: missing '#' in frame field %q", frameStr)
		return
	}

	return
}

// parseInt64 parses a decimal integer string, returning an error on failure.
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// FormatLine formats a frame as a candump line.
//
// Standard CAN:  (TIMESTAMP) iface ID#HEXDATA
// CAN FD:        (TIMESTAMP) iface ID##FLAGSHEXDATA
//
//fusa:req REQ-REC-004
func FormatLine(iface string, ts time.Time, f can.Frame) string {
	tsStr := fmt.Sprintf("(%d.%06d)",
		ts.Unix(),
		ts.Nanosecond()/1000,
	)

	idStr := fmt.Sprintf("%X", f.ID)

	if f.FD {
		var flags byte
		if f.BRS {
			flags |= 0x01
		}
		return fmt.Sprintf("%s %s %s##%02X%s",
			tsStr, iface, idStr,
			flags,
			strings.ToUpper(hex.EncodeToString(f.Data)),
		)
	}

	return fmt.Sprintf("%s %s %s#%s",
		tsStr, iface, idStr,
		strings.ToUpper(hex.EncodeToString(f.Data)),
	)
}
