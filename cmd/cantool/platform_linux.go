// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/socketcan"
)

func openPlatformBus(iface string) (can.Bus, error) {
	return socketcan.New(iface)
}
