// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
)

// Opener asks the operating system about default applications. Each platform has its own.
type Opener interface {
	// DefaultApp names the application registered to open path, or "" when none is.
	DefaultApp(ctx context.Context, path string) (string, error)
	// Open opens path with app, which DefaultApp returned.
	Open(ctx context.Context, app, path string) error
}

// ActivateRequest is the part of T3's `activate` frame the decision needs.
type ActivateRequest struct {
	URL   string
	Local bool
}

// Decide opens the file with its default application when that makes sense, and reports whether
// it did. Every "no" leaves the click to T3, which then opens the file its own way.
//
// The lookup happens on every click, so installing or removing a viewer takes effect at once.
func Decide(ctx context.Context, opener Opener, request ActivateRequest) bool {
	// An app opened here would appear on a screen the user is not looking at.
	if !request.Local {
		return false
	}
	path := request.URL
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	// A missing file is T3's to report; LaunchServices would still name an app by extension.
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	app, err := opener.DefaultApp(ctx, path)
	if err != nil || app == "" {
		return false
	}
	return opener.Open(ctx, app, path) == nil
}
