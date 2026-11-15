// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"
)

// Asks the real LaunchServices about an extension nothing claims; nothing opens.
func TestDarwinLookupFindsNoAppForAnUnclaimedExtension(t *testing.T) {
	path := writeFile(t, t.TempDir(), "note.t3-open-with-unclaimed")
	app, err := platformOpener{}.DefaultApp(context.Background(), path)
	if err != nil || app != "" {
		t.Fatalf("DefaultApp = %q, %v; want no app", app, err)
	}
	// A plain text file is claimed by something on every Mac (TextEdit at least).
	text := writeFile(t, t.TempDir(), "note.txt")
	if app, err := (platformOpener{}).DefaultApp(context.Background(), text); err != nil || app == "" {
		t.Fatalf("DefaultApp(.txt) = %q, %v; want an app", app, err)
	}
}
