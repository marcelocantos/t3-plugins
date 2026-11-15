// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os/exec"
	"strings"
)

// LaunchServices answers directly. `open <path>` would also fail without a handler (exit 1,
// kLSApplicationNotFoundErr), but asking first means a "no" never touches the screen.
const lookupScript = `ObjC.import('AppKit');
function run(argv) {
  const url = $.NSURL.fileURLWithPath(argv[0]);
  const app = $.NSWorkspace.sharedWorkspace.URLForApplicationToOpenURL(url);
  return app.isNil() ? "" : app.path.js;
}`

type platformOpener struct{}

func (platformOpener) DefaultApp(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", lookupScript, path).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (platformOpener) Open(ctx context.Context, app, path string) error {
	// `--` keeps a path that starts with "-" from reading as an option.
	return exec.CommandContext(ctx, "/usr/bin/open", "-a", app, "--", path).Run()
}
