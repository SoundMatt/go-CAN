// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dbc_test

import (
	"math"
	"strings"
	"testing"

	"github.com/SoundMatt/go-CAN/dbc"
)

//fusa:test REQ-DBC-001
//fusa:test REQ-DBC-002
//fusa:test REQ-DBC-003

const sampleDBC = `
VERSION ""

NS_ :

BS_:

BU_: ECU BCM

BO_ 256 EngineData: 8 ECU
 SG_ EngineSpeed : 0|16@1+ (0.25,0) [0|16383.75] "rpm" BCM
 SG_ EngineTemp : 16|8@1- (1,-40) [-40|215] "degC" BCM

BO_ 512 BrakeStatus: 2 BCM
 SG_ BrakePressure : 0|12@1+ (0.1,0) [0|409.5] "bar" ECU
 SG_ BrakeActive : 12|1@1+ (1,0) [0|1] "" ECU
`

func TestParse(t *testing.T) {
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(db.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(db.Messages))
	}
	msg, ok := db.Messages[256]
	if !ok {
		t.Fatal("message 256 not found")
	}
	if msg.Name != "EngineData" {
		t.Errorf("name = %q, want EngineData", msg.Name)
	}
	if len(msg.Signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(msg.Signals))
	}
}

func TestDecodeUnsigned(t *testing.T) {
	db, _ := dbc.Parse(strings.NewReader(sampleDBC))

	// EngineSpeed: bits 0-15, LE, unsigned, factor=0.25, offset=0
	// raw = 0x1000 = 4096 → 4096 * 0.25 = 1024.0 rpm
	data := []byte{0x00, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	vals := db.Decode(256, data)
	if vals == nil {
		t.Fatal("Decode returned nil")
	}
	got := vals["EngineSpeed"]
	want := 1024.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("EngineSpeed = %f, want %f", got, want)
	}
}

func TestDecodeSigned(t *testing.T) {
	db, _ := dbc.Parse(strings.NewReader(sampleDBC))

	// EngineTemp: bits 16-23, LE, signed, factor=1, offset=-40
	// raw = 0x00 → -40 degC
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	vals := db.Decode(256, data)
	got := vals["EngineTemp"]
	want := -40.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("EngineTemp = %f, want %f", got, want)
	}
}

func TestDecodeUnknownID(t *testing.T) {
	db, _ := dbc.Parse(strings.NewReader(sampleDBC))
	if db.Decode(0xFFFF, []byte{}) != nil {
		t.Error("Decode for unknown ID should return nil")
	}
}

func TestParseMalformed(t *testing.T) {
	tests := []string{
		"BO_ bad_id Name: 8 ECU",
		"BO_ 1 Name 8 ECU",
	}
	for _, input := range tests {
		_, err := dbc.Parse(strings.NewReader(input))
		if err == nil {
			t.Errorf("Parse(%q) expected error, got nil", input)
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add(sampleDBC)
	f.Fuzz(func(t *testing.T, s string) {
		// Must not panic
		_, _ = dbc.Parse(strings.NewReader(s))
	})
}
