// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mock provides an in-process mock CAN bus for testing.
// It satisfies the RELAY spec §7 requirement that every implementation
// ship a sub-package named "mock". The mock is identical to the virtual
// package and implements all mandatory interfaces.
package mock

import "github.com/SoundMatt/go-CAN/virtual"

// Bus is the mock CAN bus. It is identical to virtual.Bus.
type Bus = virtual.Bus

// New creates an in-process mock CAN bus.
//
//fusa:req REQ-MOCK-001
func New() (*Bus, error) { return virtual.New() }
