// Copyright (c) 2026 Matt Jones. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mock_test

import (
	"context"
	"testing"

	can "github.com/SoundMatt/go-CAN"
	"github.com/SoundMatt/go-CAN/mock"
)

func TestMockImplementsBus(t *testing.T) {
	b, err := mock.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	// Verify mock.Bus satisfies can.Bus interface.
	var _ can.Bus = b

	ch, err := b.Subscribe(nil)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := b.Send(context.Background(), can.Frame{ID: 0x100, Data: []byte{0x01}}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	f := <-ch
	if f.ID != 0x100 {
		t.Errorf("got ID 0x%X, want 0x100", f.ID)
	}
}
