// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package can_test

import (
	"testing"

	can "github.com/SoundMatt/go-CAN"
)

//fusa:test REQ-CAN-003
//fusa:test REQ-CAN-004
//fusa:test REQ-CAN-007
//fusa:test REQ-CAN-009
//fusa:test REQ-CAN-010
//fusa:test REQ-CAN-011
//fusa:test REQ-CAN-012
//fusa:test REQ-CAN-013
//fusa:test REQ-CAN-014

func TestValidateFrame(t *testing.T) {
	tests := []struct {
		name    string
		frame   can.Frame
		wantErr bool
	}{
		{name: "valid standard", frame: can.Frame{ID: 0x100, Data: []byte{1, 2, 3}}, wantErr: false},
		{name: "valid extended", frame: can.Frame{ID: 0x1FFFFFFF, Ext: true, Data: []byte{0xFF}}, wantErr: false},
		{name: "valid CAN FD", frame: can.Frame{ID: 0x100, FD: true, Data: make([]byte, 64)}, wantErr: false},
		{name: "valid RTR", frame: can.Frame{ID: 0x200, RTR: true}, wantErr: false},
		{name: "RTR with FD", frame: can.Frame{ID: 0x200, RTR: true, FD: true}, wantErr: true},
		{name: "standard ID too large", frame: can.Frame{ID: 0x800}, wantErr: true},
		{name: "extended ID too large", frame: can.Frame{ID: 0x20000000, Ext: true}, wantErr: true},
		{name: "RTR with data", frame: can.Frame{ID: 0x100, RTR: true, Data: []byte{1}}, wantErr: true},
		{name: "CAN data too long", frame: can.Frame{ID: 0x100, Data: make([]byte, 9)}, wantErr: true},
		{name: "FD data too long", frame: can.Frame{ID: 0x100, FD: true, Data: make([]byte, 65)}, wantErr: true},
		{name: "BRS without FD", frame: can.Frame{ID: 0x100, BRS: true, Data: []byte{1}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := can.ValidateFrame(tt.frame)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFrame() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilterMatches(t *testing.T) {
	tests := []struct {
		filter can.Filter
		frame  can.Frame
		want   bool
	}{
		{can.Filter{}, can.Frame{ID: 0x123}, true},
		{can.Filter{ID: 0x100, Mask: 0x7FF}, can.Frame{ID: 0x100}, true},
		{can.Filter{ID: 0x100, Mask: 0x7FF}, can.Frame{ID: 0x101}, false},
		{can.Filter{ID: 0x100, Mask: 0x700}, can.Frame{ID: 0x1FF}, true},
	}
	for _, tt := range tests {
		got := tt.filter.Matches(tt.frame)
		if got != tt.want {
			t.Errorf("Filter{%#x,%#x}.Matches(%#x) = %v, want %v",
				tt.filter.ID, tt.filter.Mask, tt.frame.ID, got, tt.want)
		}
	}
}

func TestMaxDataLen(t *testing.T) {
	if can.MaxDataLen(false) != 8 {
		t.Error("standard CAN max data len should be 8")
	}
	if can.MaxDataLen(true) != 64 {
		t.Error("CAN FD max data len should be 64")
	}
}
