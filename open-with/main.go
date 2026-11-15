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
	"strconv"
	"syscall"
	"time"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

const usage = `t3-open-with: open Markdown file links from T3 Code with the default application.

Usage:
  t3-open-with [--port N] [--no-advertise]

Flags:
  --port N         Listen on 127.0.0.1:N (default 0, a free port).
  --no-advertise   Do not write a discovery manifest; register the endpoint by hand instead.
  --version        Print the version and exit.
  --help           Print this help and exit.
  --help-agent     Print this help and a guide for coding agents, then exit.
`

const agentGuide = `
Agent guide:
  The plugin listens on loopback and serves http://127.0.0.1:<port>/t3: a plain GET is T3's
  discovery probe, a WebSocket upgrade is T3's plugin protocol (version 1). It declares one link
  pattern, {"glob": "/**/*.{md,markdown}", "click": "activate", "card": false}, so T3 sends an
  "activate" frame on a plain click of a matching file link and shows no hover card.

  It answers {"activate": {"handled": true}} only when client.local is true, the path is an
  existing regular file, and the OS names a default application at that moment (macOS:
  LaunchServices via osascript; Linux: xdg-mime). It then opens the file with that application.
  Every other case answers handled:false and T3 opens the file itself.

  Unless --no-advertise is given, it writes
  $XDG_DATA_HOME/t3code/plugins/github.com+marcelocantos+t3-open-with+<port>.json
  (~/.local/share/... when XDG_DATA_HOME is unset) and removes it on SIGINT/SIGTERM. T3 offers the
  plugin in Settings -> Plugins; nothing connects until the user enables it.

  Logs go to stderr, one line per activation.
`

func main() {
	port := flag.Int("port", 0, "")
	noAdvertise := flag.Bool("no-advertise", false, "")
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
	if err := run(*port, !*noAdvertise); err != nil {
		log.Fatal(err)
	}
}

func run(port int, advertise bool) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	actual := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d%s", actual, endpointPath)
	server := &http.Server{Handler: &Handler{Opener: platformOpener{}}, ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	log.Printf("t3-open-with %s listening on %s", version, baseURL)

	if advertise {
		ad, err := Advertise(baseURL, strconv.Itoa(actual))
		if err != nil {
			server.Close()
			return fmt.Errorf("advertise: %w", err)
		}
		defer ad.Withdraw()
		log.Printf("advertised in %s", ad.Path())
	}

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
