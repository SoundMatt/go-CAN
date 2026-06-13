// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// CAN FD constants and socket option helper.
//
//fusa:req REQ-SCAN-004
package socketcan

import (
	"golang.org/x/sys/unix"
)

const (
	canFDBRSFlag  = 0x01 // CAN FD bit-rate switch flag (in canfd_frame.flags)
	canFDESIFlag  = 0x02 // CAN FD error state indicator flag
	canFDFrameLen = 72   // sizeof(struct canfd_frame): 4+1+1+2+64 bytes
)

// enableFD enables CAN FD frames on the given socket file descriptor.
// After calling this, the socket can send and receive both classic and FD frames.
//
//fusa:req REQ-SCAN-004
func enableFD(fd int) error {
	return unix.SetsockoptInt(fd, unix.SOL_CAN_RAW, unix.CAN_RAW_FD_FRAMES, 1)
}
