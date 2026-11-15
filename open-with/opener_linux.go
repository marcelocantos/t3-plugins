// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// xdg-open can stay attached to the application it starts; once it has run this long the
// application is launching, and the click counts as handled.
const xdgOpenSettle = time.Second

type platformOpener struct{}

func (platformOpener) DefaultApp(ctx context.Context, path string) (string, error) {
	mime, err := exec.CommandContext(ctx, "xdg-mime", "query", "filetype", path).Output()
	if err != nil {
		return "", err
	}
	desktop, err := exec.CommandContext(ctx, "xdg-mime", "query", "default", strings.TrimSpace(string(mime))).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(desktop)), nil
}

func (platformOpener) Open(_ context.Context, _ string, path string) error {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(xdgOpenSettle):
		return nil
	}
}
