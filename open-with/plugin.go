// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	pluginID   = "github.com/marcelocantos/t3-plugins/open-with"
	pluginName = "Open with default app"
	// endpointPath serves both the discovery probe (plain GET) and the protocol (WebSocket).
	endpointPath = "/t3"
	// T3 waits two seconds for an answer before opening the file itself.
	decideTimeout = 1500 * time.Millisecond
	maxFrameBytes = 1 << 20
)

type linkPattern struct {
	ID    string `json:"id"`
	Glob  string `json:"glob"`
	Click string `json:"click"`
	Card  bool   `json:"card"`
}

type probeAnswer struct {
	Protocol int           `json:"protocol"`
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Scopes   []string      `json:"scopes"`
	Links    []linkPattern `json:"links"`
}

var probe = probeAnswer{
	Protocol: 1,
	ID:       pluginID,
	Name:     pluginName,
	Scopes:   []string{"links"},
	Links:    []linkPattern{{ID: "markdown", Glob: "/**/*.{md,markdown}", Click: "activate", Card: false}},
}

// inbound is every T3 frame this plugin reads; fields a frame lacks stay zero.
type inbound struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	Scope     string `json:"scope"`
	URL       string `json:"url"`
	Client    struct {
		Local bool `json:"local"`
	} `json:"client"`
}

type activation struct {
	Handled bool `json:"handled"`
}

type result struct {
	Type      string      `json:"type"`
	RequestID string      `json:"requestId"`
	OK        bool        `json:"ok"`
	Activate  *activation `json:"activate,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// Handler serves the plugin endpoint. Opener decides how files open on this machine.
type Handler struct {
	Opener Opener
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// T3 never sends an Origin; a browser always does, so no web page can drive this plugin.
	if r.URL.Path != endpointPath || r.Header.Get("Origin") != "" {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Upgrade") == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(probe)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxFrameBytes)
	h.serve(r.Context(), conn)
}

func (h *Handler) serve(ctx context.Context, conn *websocket.Conn) {
	defer conn.CloseNow()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var frame inbound
		if json.Unmarshal(data, &frame) != nil || frame.RequestID == "" {
			// Subscriptions and unknown frames need no answer.
			continue
		}
		// A slow lookup must not hold up the next click.
		go func() {
			reply := h.answer(ctx, frame)
			out, _ := json.Marshal(reply)
			_ = conn.Write(ctx, websocket.MessageText, out)
		}()
	}
}

func (h *Handler) answer(ctx context.Context, frame inbound) result {
	reply := result{Type: "result", RequestID: frame.RequestID}
	if frame.Type != "activate" || frame.Scope != "link" {
		reply.Error = "This plugin only opens links."
		return reply
	}
	decideCtx, cancel := context.WithTimeout(ctx, decideTimeout)
	defer cancel()
	handled := Decide(decideCtx, h.Opener, ActivateRequest{URL: frame.URL, Local: frame.Client.Local})
	log.Printf("activate %q local=%t handled=%t", frame.URL, frame.Client.Local, handled)
	reply.OK = true
	reply.Activate = &activation{Handled: handled}
	return reply
}
