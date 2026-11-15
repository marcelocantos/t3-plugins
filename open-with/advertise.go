// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// advertisementDir is where T3 looks for plugins running on this machine.
func advertisementDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(xdg) {
		return filepath.Join(xdg, "t3code", "plugins"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "t3code", "plugins"), nil
}

// Advertisement is one manifest file this process wrote and will remove.
type Advertisement struct {
	path     string
	contents []byte
}

// Advertise writes the manifest for baseURL atomically, named for the plugin id and instance.
func Advertise(baseURL, instance string) (*Advertisement, error) {
	dir, err := advertisementDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(map[string]any{
		"protocol":   1,
		"id":         pluginID,
		"name":       pluginName,
		"connection": map[string]string{"kind": "server", "baseUrl": baseURL},
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	name := strings.ReplaceAll(pluginID, "/", "+") + "+" + instance + ".json"
	tmp, err := os.CreateTemp(dir, ".t3-open-with-*")
	if err != nil {
		return nil, err
	}
	_, writeErr := tmp.Write(contents)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(tmp.Name())
		return nil, fmt.Errorf("write advertisement: %w", errors.Join(writeErr, closeErr))
	}
	path := filepath.Join(dir, name)
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	return &Advertisement{path: path, contents: contents}, nil
}

// Path is the manifest's location.
func (a *Advertisement) Path() string { return a.path }

// Withdraw removes the manifest, unless another process has since replaced it.
func (a *Advertisement) Withdraw() {
	current, err := os.ReadFile(a.path)
	if err == nil && bytes.Equal(current, a.contents) {
		os.Remove(a.path)
	}
}
