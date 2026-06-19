// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safety_test

import (
	"testing"

	"github.com/SoundMatt/go-CAN/safety"
)

// These tests mirror the rust-CAN safety::hmac_auth conformance tests so the
// two implementations are behaviourally equivalent (REQ-SEC-006).

//fusa:test REQ-SEC-006
func TestHmacSignVerifyRoundtrip(t *testing.T) {
	var auth safety.HmacSha256Auth
	key := []byte("test-key-32-bytes-padding-padpad")
	data := []byte("safety-critical-payload")

	tag := auth.Sign(key, data)
	if len(tag) != 32 {
		t.Fatalf("tag length = %d, want 32", len(tag))
	}
	if !auth.Verify(key, data, tag) {
		t.Error("Verify rejected a valid tag")
	}
}

//fusa:test REQ-SEC-006
func TestHmacWrongKeyRejected(t *testing.T) {
	var auth safety.HmacSha256Auth
	key := []byte("correct-key-32-bytes-padding-pad")
	badKey := []byte("wrong-key-32-bytes-padding-paddd")
	data := []byte("payload")

	tag := auth.Sign(key, data)
	if auth.Verify(badKey, data, tag) {
		t.Error("Verify accepted a tag under the wrong key")
	}
}

//fusa:test REQ-SEC-006
func TestHmacTamperedDataRejected(t *testing.T) {
	var auth safety.HmacSha256Auth
	key := []byte("key-32-bytes-padding-paddingpadd")
	data := []byte("original")

	tag := auth.Sign(key, data)
	if auth.Verify(key, []byte("tampered"), tag) {
		t.Error("Verify accepted a tag for tampered data")
	}
}

//fusa:test REQ-SEC-006
func TestHmacTruncatedTagRejected(t *testing.T) {
	var auth safety.HmacSha256Auth
	key := []byte("key-32-bytes-padding-paddingpadd")
	data := []byte("payload")

	tag := auth.Sign(key, data)
	if auth.Verify(key, data, tag[:16]) {
		t.Error("Verify accepted a truncated tag")
	}
}

//fusa:test REQ-SEC-006
func TestHmacTagLen(t *testing.T) {
	if got := (safety.HmacSha256Auth{}).TagLen(); got != 32 {
		t.Errorf("TagLen() = %d, want 32", got)
	}
}

// TestHmacAuthenticatorInterface verifies HmacSha256Auth satisfies the
// MessageAuthenticator interface (pluggability, REQ-SEC-006).
//
//fusa:test REQ-SEC-006
func TestHmacAuthenticatorInterface(t *testing.T) {
	var auth safety.MessageAuthenticator = safety.HmacSha256Auth{}
	key := []byte("key-32-bytes-padding-paddingpadd")
	data := []byte("brake-command")
	if !auth.Verify(key, data, auth.Sign(key, data)) {
		t.Error("interface round-trip failed")
	}
}
