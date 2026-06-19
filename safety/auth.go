// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safety

import (
	"crypto/hmac"
	"crypto/sha256"
)

// MessageAuthenticator is a pluggable cryptographic message-authentication
// interface (RELAY shared requirement REQ-SEC-006). It provides authenticity
// and integrity for a CAN payload above the data-link layer — the defence the
// bus itself cannot offer (see THREAT_MODEL.md, threats T2 and T5).
//
// It is deliberately separate from the E2E Protector/Receiver, which protect
// against random corruption and sequencing faults; authentication protects
// against an active adversary forging or tampering with frames. An integrator
// supplies the shared key (from an HSM or secure key store).
//
//fusa:req REQ-SEC-006
type MessageAuthenticator interface {
	// Sign returns the authentication tag for data under key.
	Sign(key, data []byte) []byte
	// Verify reports whether tag is a valid authentication tag for data under
	// key. Implementations MUST use a constant-time comparison.
	Verify(key, data, tag []byte) bool
	// TagLen returns the length in bytes of a tag produced by Sign.
	TagLen() int
}

// HmacSha256Auth is an HMAC-SHA256 MessageAuthenticator (FIPS 198-1 / RFC 2104).
//
// Security properties:
//   - Key length: HMAC-SHA256 accepts any key length; use ≥ 32 bytes (256 bits)
//     from a cryptographically secure source or an HSM.
//   - Tag length: 32 bytes (256 bits), giving 128-bit forgery resistance.
//   - Timing: Verify uses crypto/hmac.Equal, a constant-time comparison that
//     prevents tag-comparison timing side-channels.
//
// It satisfies the IEC 62443 SL-2 and ISO/SAE 21434 integrity requirements
// expressed as REQ-SEC-006.
//
//fusa:req REQ-SEC-006
type HmacSha256Auth struct{}

// Sign returns the HMAC-SHA256 tag (32 bytes) of data under key.
//
//fusa:req REQ-SEC-006
func (HmacSha256Auth) Sign(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// Verify reports whether tag authenticates data under key, using a constant-time
// comparison.
//
//fusa:req REQ-SEC-006
func (HmacSha256Auth) Verify(key, data, tag []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return hmac.Equal(mac.Sum(nil), tag)
}

// TagLen returns the HMAC-SHA256 tag length in bytes (32).
//
//fusa:req REQ-SEC-006
func (HmacSha256Auth) TagLen() int { return sha256.Size }
