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

// sampleDBCWithValues extends sampleDBC with VAL_ entries.
const sampleDBCWithValues = `
VERSION ""

NS_ :

BS_:

BU_: ECU BCM

BO_ 256 EngineData: 8 ECU
 SG_ EngineSpeed : 0|16@1+ (0.25,0) [0|16383.75] "rpm" BCM
 SG_ EngineTemp : 16|8@1- (1,-40) [-40|215] "degC" BCM
 SG_ EngineMode : 24|4@1+ (1,0) [0|15] "" BCM

BO_ 512 BrakeStatus: 2 BCM
 SG_ BrakePressure : 0|12@1+ (0.1,0) [0|409.5] "bar" ECU
 SG_ BrakeActive : 12|1@1+ (1,0) [0|1] "" ECU

VAL_ 256 EngineMode 0 "Off" 1 "Idle" 2 "Running" 3 "Error" ;
`

// ---- Encode round-trip tests ------------------------------------------------

//fusa:test REQ-DBC-005

func TestEncodeDecodeRoundTripUnsigned(t *testing.T) {
	//fusa:test REQ-DBC-005
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := map[string]float64{
		"EngineSpeed": 1024.0,
		"EngineTemp":  -40.0,
	}

	encoded, err := db.Encode(256, want)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded) != 8 {
		t.Fatalf("Encode returned %d bytes, want 8", len(encoded))
	}

	got := db.Decode(256, encoded)
	for name, wantVal := range want {
		gotVal, ok := got[name]
		if !ok {
			t.Errorf("signal %q missing from decoded output", name)
			continue
		}
		if math.Abs(gotVal-wantVal) > 0.001 {
			t.Errorf("round-trip %s: got %f, want %f", name, gotVal, wantVal)
		}
	}
}

func TestEncodeDecodeRoundTripSigned(t *testing.T) {
	//fusa:test REQ-DBC-005
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// EngineTemp: signed 8-bit, factor=1, offset=-40.
	// raw range for signed 8-bit: -128..127
	// physical range: offset + raw*factor → -40+(-128)=-168 .. -40+127=87
	// So valid physical range is -168..87.
	// physical=-40 → raw=0
	// physical=87  → raw=127 (max positive for signed 8-bit)
	// physical=-128+(-40)=-168 → raw=-128 (most negative)
	tests := []struct {
		physical float64
	}{
		{-40.0},
		{0.0},
		{87.0},
		{-168.0},
	}

	for _, tc := range tests {
		want := map[string]float64{
			"EngineSpeed": 0,
			"EngineTemp":  tc.physical,
		}
		encoded, err := db.Encode(256, want)
		if err != nil {
			t.Fatalf("Encode(%f): %v", tc.physical, err)
		}
		got := db.Decode(256, encoded)
		if math.Abs(got["EngineTemp"]-tc.physical) > 0.001 {
			t.Errorf("EngineTemp round-trip: physical=%f got=%f", tc.physical, got["EngineTemp"])
		}
	}
}

func TestEncodePartialSignals(t *testing.T) {
	//fusa:test REQ-DBC-005
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Encode only EngineSpeed; EngineTemp should be zero-initialised (physical = offset = -40).
	signals := map[string]float64{"EngineSpeed": 512.0}
	encoded, err := db.Encode(256, signals)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	got := db.Decode(256, encoded)
	if math.Abs(got["EngineSpeed"]-512.0) > 0.001 {
		t.Errorf("EngineSpeed = %f, want 512.0", got["EngineSpeed"])
	}
	// raw=0 for EngineTemp → -40 degC
	if math.Abs(got["EngineTemp"]-(-40.0)) > 0.001 {
		t.Errorf("EngineTemp = %f, want -40.0 (zero raw)", got["EngineTemp"])
	}
}

// ---- Error path tests -------------------------------------------------------

func TestEncodeUnknownMsgID(t *testing.T) {
	//fusa:test REQ-DBC-005
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = db.Encode(0xDEAD, map[string]float64{"EngineSpeed": 0})
	if err == nil {
		t.Error("Encode unknown msgID should return error")
	}
}

func TestEncodeUnknownSignalName(t *testing.T) {
	//fusa:test REQ-DBC-005
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = db.Encode(256, map[string]float64{"NoSuchSignal": 0})
	if err == nil {
		t.Error("Encode unknown signal name should return error")
	}
}

// motorolaDBC defines big-endian (Motorola, @0) signals to exercise the
// big-endian packing/unpacking path.
const motorolaDBC = `
VERSION ""
BU_: ECU
BO_ 100 MotorolaMsg: 8 ECU
 SG_ BigA : 7|8@0+ (1,0) [0|255] "" ECU
 SG_ BigB : 23|16@0- (1,0) [-32768|32767] "" ECU
`

// wideDBC defines a full 64-bit unsigned signal to exercise the Length==64
// branch of physicalToRaw.
const wideDBC = `
VERSION ""
BU_: ECU
BO_ 200 WideMsg: 8 ECU
 SG_ Counter : 0|64@1+ (1,0) [0|0] "" ECU
`

func TestEncodeDecodeRoundTripBigEndian(t *testing.T) {
	//fusa:test REQ-DBC-005
	db, err := dbc.Parse(strings.NewReader(motorolaDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := map[string]float64{
		"BigA": 200,
		"BigB": -1000,
	}
	encoded, err := db.Encode(100, want)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := db.Decode(100, encoded)
	for name, wantVal := range want {
		if math.Abs(got[name]-wantVal) > 0.001 {
			t.Errorf("big-endian round-trip %s: got %f, want %f", name, got[name], wantVal)
		}
	}
}

func TestEncodeClampsUnsigned(t *testing.T) {
	//fusa:test REQ-DBC-005
	//fusa:test REQ-SEC-004
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// EngineSpeed: 16-bit unsigned, factor 0.25 → physical max 16383.75.
	// Over-range clamps to max; negative clamps to 0.
	over := db.Decode(256, mustEncode(t, db, map[string]float64{"EngineSpeed": 1e9}))
	if math.Abs(over["EngineSpeed"]-16383.75) > 0.001 {
		t.Errorf("over-range clamp: got %f, want 16383.75", over["EngineSpeed"])
	}
	under := db.Decode(256, mustEncode(t, db, map[string]float64{"EngineSpeed": -500}))
	if under["EngineSpeed"] != 0 {
		t.Errorf("under-range clamp: got %f, want 0", under["EngineSpeed"])
	}
}

func TestEncodeClampsSigned(t *testing.T) {
	//fusa:test REQ-DBC-005
	//fusa:test REQ-SEC-004
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// EngineTemp: signed 8-bit, factor 1, offset -40 → physical range -168..87.
	over := db.Decode(256, mustEncode(t, db, map[string]float64{"EngineTemp": 1000}))
	if math.Abs(over["EngineTemp"]-87) > 0.001 {
		t.Errorf("signed over-range clamp: got %f, want 87", over["EngineTemp"])
	}
	under := db.Decode(256, mustEncode(t, db, map[string]float64{"EngineTemp": -1000}))
	if math.Abs(under["EngineTemp"]-(-168)) > 0.001 {
		t.Errorf("signed under-range clamp: got %f, want -168", under["EngineTemp"])
	}
}

func TestEncode64BitSignal(t *testing.T) {
	//fusa:test REQ-DBC-005
	db, err := dbc.Parse(strings.NewReader(wideDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	const v = 1 << 40
	got := db.Decode(200, mustEncode(t, db, map[string]float64{"Counter": v}))
	if math.Abs(got["Counter"]-v) > 1 {
		t.Errorf("64-bit round-trip: got %f, want %d", got["Counter"], v)
	}
}

func mustEncode(t *testing.T, db *dbc.DB, signals map[string]float64) []byte {
	t.Helper()
	b, err := db.Encode(forMsgID(signals), signals)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return b
}

// forMsgID resolves the message ID a signal set belongs to for the test DBCs.
func forMsgID(signals map[string]float64) uint32 {
	for name := range signals {
		switch name {
		case "Counter":
			return 200
		case "BigA", "BigB":
			return 100
		default:
			return 256
		}
	}
	return 256
}

// ---- VAL_ parsing tests -----------------------------------------------------

func TestParseVAL(t *testing.T) {
	//fusa:test REQ-DBC-006
	db, err := dbc.Parse(strings.NewReader(sampleDBCWithValues))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	msg, ok := db.Messages[256]
	if !ok {
		t.Fatal("message 256 not found")
	}
	sig, ok := msg.Signals["EngineMode"]
	if !ok {
		t.Fatal("signal EngineMode not found")
	}
	if sig.Values == nil {
		t.Fatal("signal.Values is nil, expected VAL_ entries to be populated")
	}

	expected := map[int64]string{
		0: "Off",
		1: "Idle",
		2: "Running",
		3: "Error",
	}
	for raw, label := range expected {
		got, ok := sig.Values[raw]
		if !ok {
			t.Errorf("Values[%d] missing", raw)
			continue
		}
		if got != label {
			t.Errorf("Values[%d] = %q, want %q", raw, got, label)
		}
	}
	if len(sig.Values) != len(expected) {
		t.Errorf("Values has %d entries, want %d", len(sig.Values), len(expected))
	}
}

func TestParseVALOtherSignalsUnaffected(t *testing.T) {
	//fusa:test REQ-DBC-006
	db, err := dbc.Parse(strings.NewReader(sampleDBCWithValues))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	sig := db.Messages[256].Signals["EngineSpeed"]
	if sig.Values != nil {
		t.Errorf("EngineSpeed.Values should be nil, got %v", sig.Values)
	}
}

// ---- PhysicalToLabel tests --------------------------------------------------

func TestPhysicalToLabel(t *testing.T) {
	//fusa:test REQ-DBC-006
	//fusa:test REQ-DBC-007
	db, err := dbc.Parse(strings.NewReader(sampleDBCWithValues))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	sig := db.Messages[256].Signals["EngineMode"]

	tests := []struct {
		physical float64
		want     string
		wantOK   bool
	}{
		{0.0, "Off", true},
		{1.0, "Idle", true},
		{2.0, "Running", true},
		{3.0, "Error", true},
		{4.0, "", false},
		{99.0, "", false},
	}
	for _, tc := range tests {
		got, ok := sig.PhysicalToLabel(tc.physical)
		if ok != tc.wantOK {
			t.Errorf("PhysicalToLabel(%f): ok=%v, want %v", tc.physical, ok, tc.wantOK)
		}
		if got != tc.want {
			t.Errorf("PhysicalToLabel(%f): label=%q, want %q", tc.physical, got, tc.want)
		}
	}
}

func TestPhysicalToLabelNoValues(t *testing.T) {
	//fusa:test REQ-DBC-007
	db, err := dbc.Parse(strings.NewReader(sampleDBC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// EngineSpeed has no VAL_ entries
	sig := db.Messages[256].Signals["EngineSpeed"]
	label, ok := sig.PhysicalToLabel(1024.0)
	if ok || label != "" {
		t.Errorf("expected ('', false), got (%q, %v)", label, ok)
	}
}
