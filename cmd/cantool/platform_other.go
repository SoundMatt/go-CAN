//go:build !linux

// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"

	can "github.com/SoundMatt/go-CAN"
)

func openPlatformBus(iface string) (can.Bus, error) {
	return nil, fmt.Errorf("SocketCAN is only available on Linux — use iface=\"virtual\" on this platform")
}
