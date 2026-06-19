// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package can_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	can "github.com/SoundMatt/go-CAN"
	relay "github.com/SoundMatt/RELAY"
)

// These tests verify go-CAN against the RELAY canonical golden reference
// vectors (spec §15.1), vendored under testdata/relay-vectors/. They are the
// cross-language equality oracle relied on by `relay interop`: a conforming
// go-CAN MUST produce the canonical relay.Message in each vector, and MUST
// reject every error vector with ErrInvalidFrame.

//fusa:test REQ-CANXL-001
//fusa:test REQ-CANXL-003
//fusa:test REQ-CANXL-004

// goldenVector is the on-disk golden reference vector format.
type goldenVector struct {
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	Value   can.Frame     `json:"value"`
	Message relay.Message `json:"message"`
}

// errorVector is an error-condition golden vector.
type errorVector struct {
	Name  string    `json:"name"`
	Kind  string    `json:"kind"`
	Value can.Frame `json:"value"`
	Error string    `json:"error"`
}

func TestRelayGoldenVectors(t *testing.T) {
	paths, err := filepath.Glob("testdata/relay-vectors/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no golden vectors found")
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var v goldenVector
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: %v", p, err)
		}

		t.Run(v.Name, func(t *testing.T) {
			// Frame -> Message must match the canonical message (ignoring the
			// runtime Timestamp, which the vector zeroes).
			got := v.Value.ToMessage()
			got.Timestamp = v.Message.Timestamp

			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(v.Message)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("ToMessage mismatch:\n got  %s\n want %s", gotJSON, wantJSON)
			}

			// Message -> Frame must round-trip back to the original value.
			back, err := can.FromMessage(v.Message)
			if err != nil {
				t.Fatalf("FromMessage: %v", err)
			}
			backJSON, _ := json.Marshal(back)
			valueJSON, _ := json.Marshal(v.Value)
			if string(backJSON) != string(valueJSON) {
				t.Errorf("round-trip mismatch:\n got  %s\n want %s", backJSON, valueJSON)
			}

			// The vector's frame must pass validation.
			if err := can.ValidateFrame(v.Value); err != nil {
				t.Errorf("ValidateFrame rejected a valid golden frame: %v", err)
			}
		})
	}
}

func TestRelayErrorVectors(t *testing.T) {
	paths, err := filepath.Glob("testdata/relay-vectors/errors/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no error vectors found")
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var v errorVector
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: %v", p, err)
		}

		t.Run(v.Name, func(t *testing.T) {
			err := can.ValidateFrame(v.Value)
			if err == nil {
				t.Fatalf("ValidateFrame accepted an invalid frame (expected %s)", v.Error)
			}
			if _, ok := err.(*can.ErrInvalidFrame); !ok {
				t.Errorf("expected *ErrInvalidFrame, got %T: %v", err, err)
			}
		})
	}
}
