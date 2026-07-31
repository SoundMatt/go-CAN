// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package isotp

import (
	"testing"
	"time"
)

// TestSTminToDurationReservedValuesMapToFailSafeMax verifies that STmin
// values ISO 15765-2 marks reserved (0x80-0xF0, 0xFA-0xFF) map to the
// conventional 127 ms fail-safe maximum rather than 0 (no delay). Mapping
// an undefined value to "no minimum separation" risks flooding a receiver
// that intended some (unknown) pacing requirement.
func TestSTminToDurationReservedValuesMapToFailSafeMax(t *testing.T) {
	const want = 127 * time.Millisecond
	reserved := []byte{0x80, 0x95, 0xF0, 0xFA, 0xFF}
	for _, stmin := range reserved {
		if got := stminToDuration(stmin); got != want {
			t.Errorf("stminToDuration(0x%02X) = %v, want %v (fail-safe max)", stmin, got, want)
		}
	}
}

// TestSTminToDurationStandardRanges verifies the two defined STmin ranges
// are unaffected by the reserved-value fail-safe change.
func TestSTminToDurationStandardRanges(t *testing.T) {
	tests := []struct {
		stmin byte
		want  time.Duration
	}{
		{0x00, 0},
		{0x01, 1 * time.Millisecond},
		{0x7F, 127 * time.Millisecond},
		{0xF1, 100 * time.Microsecond},
		{0xF9, 900 * time.Microsecond},
	}
	for _, tt := range tests {
		if got := stminToDuration(tt.stmin); got != tt.want {
			t.Errorf("stminToDuration(0x%02X) = %v, want %v", tt.stmin, got, tt.want)
		}
	}
}
