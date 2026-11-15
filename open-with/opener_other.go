// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux

package main

import "context"

// Other platforms decline every click, so T3 opens the file as usual.
type platformOpener struct{}

func (platformOpener) DefaultApp(context.Context, string) (string, error) { return "", nil }

func (platformOpener) Open(context.Context, string, string) error { return nil }
