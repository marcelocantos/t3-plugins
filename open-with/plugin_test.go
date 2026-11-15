// Copyright 2026 Marcelo Cantos
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/coder/websocket"
)

// fakeOpener registers apps by extension and records what it opened.
type fakeOpener struct {
	mu      sync.Mutex
	apps    map[string]string
	openErr error
	opened  []string
}

func (f *fakeOpener) DefaultApp(_ context.Context, path string) (string, error) {
	return f.apps[filepath.Ext(path)], nil
}

func (f *fakeOpener) Open(_ context.Context, app, path string) error {
	if f.openErr != nil {
		return f.openErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, app+" "+path)
	return nil
}

func writeFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("# note\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDecide(t *testing.T) {
	dir := t.TempDir()
	note := writeFile(t, dir, "note.md")
	odd := writeFile(t, dir, "note.markdown")
	for _, tc := range []struct {
		name    string
		request ActivateRequest
		openErr error
		want    bool
	}{
		{"local file with a default app opens", ActivateRequest{URL: note, Local: true}, nil, true},
		{"a remote user is never sent to this screen", ActivateRequest{URL: note, Local: false}, nil, false},
		{"no registered app leaves it to T3", ActivateRequest{URL: odd, Local: true}, nil, false},
		{"a missing file leaves it to T3", ActivateRequest{URL: filepath.Join(dir, "gone.md"), Local: true}, nil, false},
		{"a directory is not a file", ActivateRequest{URL: dir, Local: true}, nil, false},
		{"a relative path is refused", ActivateRequest{URL: "note.md", Local: true}, nil, false},
		{"an unclean path is refused", ActivateRequest{URL: dir + "/../" + filepath.Base(dir) + "/note.md", Local: true}, nil, false},
		{"a failed launch leaves it to T3", ActivateRequest{URL: note, Local: true}, errors.New("boom"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opener := &fakeOpener{apps: map[string]string{".md": "/Applications/Viewer.app"}, openErr: tc.openErr}
			if got := Decide(context.Background(), opener, tc.request); got != tc.want {
				t.Fatalf("Decide = %t, want %t", got, tc.want)
			}
			if tc.want && len(opener.opened) != 1 {
				t.Fatalf("opened %v, want one launch", opener.opened)
			}
			if !tc.want && len(opener.opened) != 0 {
				t.Fatalf("opened %v on a decline", opener.opened)
			}
		})
	}
}

func TestProtocol(t *testing.T) {
	opener := &fakeOpener{apps: map[string]string{".md": "/Applications/Viewer.app"}}
	server := httptest.NewServer(&Handler{Opener: opener})
	defer server.Close()
	endpoint := server.URL + endpointPath

	// The probe names the plugin and its one activate pattern, which serves no cards.
	response, err := http.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	var answer probeAnswer
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.Header.Get("Content-Type") != "application/json" || answer.ID != pluginID ||
		len(answer.Links) != 1 || answer.Links[0].Click != "activate" || answer.Links[0].Card {
		t.Fatalf("probe = %+v (%s)", answer, response.Header.Get("Content-Type"))
	}

	// A browser page cannot reach the plugin.
	request, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	request.Header.Set("Origin", "https://evil.example")
	if response, err := http.DefaultClient.Do(request); err != nil || response.StatusCode != http.StatusNotFound {
		t.Fatalf("browser request = %v, %v", response, err)
	}

	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(endpoint, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	note := writeFile(t, t.TempDir(), "note.md")
	send := func(frame map[string]any) map[string]any {
		t.Helper()
		data, _ := json.Marshal(frame)
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			t.Fatal(err)
		}
		_, reply, err := conn.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(reply, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["requestId"] != frame["requestId"] || decoded["type"] != "result" {
			t.Fatalf("reply %v does not answer %v", decoded, frame)
		}
		return decoded
	}
	activate := func(id string, local bool) map[string]any {
		return send(map[string]any{
			"type": "activate", "requestId": id, "scope": "link", "linkId": "markdown",
			"url": note, "client": map[string]any{"local": local},
		})
	}

	if got := activate("r1", true); got["ok"] != true || got["activate"].(map[string]any)["handled"] != true {
		t.Fatalf("local activate = %v", got)
	}
	if got := activate("r2", false); got["ok"] != true || got["activate"].(map[string]any)["handled"] != false {
		t.Fatalf("remote activate = %v", got)
	}
	if len(opener.opened) != 1 {
		t.Fatalf("opened %v, want only the local click", opener.opened)
	}
	// Cards and resolution are refused outright, so T3 never waits on them.
	if got := send(map[string]any{"type": "surface", "requestId": "r3", "scope": "link", "url": note}); got["ok"] != false {
		t.Fatalf("surface = %v", got)
	}
}
