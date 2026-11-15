// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

// Command t3-open-with is a T3 Code link plugin: a plain click on a Markdown file link opens the
// file with the application this machine registers for it, when there is one and the user is at
// this machine. Otherwise T3 opens the file as it always does.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

const usage = `t3-open-with: open Markdown file links from T3 Code with the default application.

T3 Code starts it as a sidecar (see README.md); it is not meant to be run by hand.

Usage:
  t3-open-with

Flags:
  --version        Print the version and exit.
  --help           Print this help and exit.
  --help-agent     Print this help and a guide for coding agents, then exit.
`

const agentGuide = `
Agent guide:
  T3 starts the plugin from its panel-plugins.json entry. It listens on a free loopback port,
  prints T3's readiness record {"type":"ready","protocol":1,"url":"ws://127.0.0.1:<port>/t3"}
  on stdout, and speaks T3's plugin protocol (version 1) on that WebSocket until T3 sends
  SIGTERM. It declares one link pattern, {"glob": "/**/*.{md,markdown}", "click": "activate",
  "card": false}, so T3 sends an "activate" frame on a plain click of a matching file link and
  shows no hover card.

  It answers {"activate": {"handled": true}} only when client.local is true, the path is an
  existing regular file, and the OS names a default application at that moment (macOS:
  LaunchServices via osascript; Linux: xdg-mime). It then opens the file with that application.
  Every other case answers handled:false and T3 opens the file itself.

  Logs go to stderr, one line per activation.
`

func main() {
	showVersion := flag.Bool("version", false, "")
	helpAgent := flag.Bool("help-agent", false, "")
	flag.Usage = func() { fmt.Print(usage) }
	flag.Parse()
	switch {
	case *showVersion:
		fmt.Println(version)
		return
	case *helpAgent:
		fmt.Print(usage + agentGuide)
		return
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: &Handler{Opener: platformOpener{}}, ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	log.Printf("t3-open-with %s listening on 127.0.0.1:%d", version, port)
	// T3 reads this one line from stdout to learn the randomly allocated port.
	fmt.Printf("{\"type\":\"ready\",\"protocol\":1,\"url\":\"ws://127.0.0.1:%d%s\"}\n", port, endpointPath)

	select {
	case <-ctx.Done():
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	// Open WebSockets are hijacked and outlive Shutdown; Close ends them.
	return server.Close()
}
