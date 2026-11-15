# open-with

A [T3 Code](https://github.com/marcelocantos/t3code) link plugin. A plain click on a Markdown file
link in a chat opens the file with the application your machine registers for it, such as Vellum
Viewer. When there is no such application, or you are not at the machine running T3, T3 opens the
file exactly as it does without the plugin.

Nothing is configured: the plugin asks the operating system on every click, so installing or
removing a viewer takes effect immediately. A modified click (⌘, Ctrl, Shift or Alt) always gets
T3's own behaviour.

It needs the plugin build of T3 Code with link activation (`"click": "activate"`, see
[panel plugins](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#taking-a-link-over)).

## Install

T3 starts and stops the plugin itself; there is no server to keep running. Build it, put the
binary somewhere stable, and add this entry to `panel-plugins.json` beside T3's `settings.json`
(`~/.t3/userdata/` for the desktop app):

```sh
go build -o ~/.local/bin/t3-open-with .   # GOWORK=off if a parent go.work does not list this module
```

```json
[
  {
    "id": "github.com/marcelocantos/t3-plugins/open-with",
    "connection": { "kind": "sidecar", "command": "/Users/you/.local/bin/t3-open-with", "args": [] },
    "scopes": ["links"],
    "links": [
      { "id": "markdown", "glob": "/**/*.{md,markdown}", "click": "activate", "card": false }
    ]
  }
]
```

T3 runs the binary, which listens on a free loopback port and prints T3's readiness record on
stdout. T3 stops it with SIGTERM when nothing needs it.

## How it decides

On each click T3 sends an `activate` request with the file's absolute path and whether the
clicking client is on this machine. The plugin opens the file and answers `handled: true` only
when all of these hold:

- the client is local (desktop, or a browser on this machine's loopback address; never mobile or
  a remote browser);
- the path is an existing regular file;
- the OS names a default application for it right now:
  - **macOS**: LaunchServices (`NSWorkspace URLForApplicationToOpenURL:` via `osascript`), then
    `open -a <app> <path>`. A plain `open` of a file nothing claims exits 1 without a dialog, but
    asking first means a decline never touches the screen.
  - **Linux**: `xdg-mime query filetype` and `xdg-mime query default`, then `xdg-open`.
  - Other platforms always decline.

Otherwise it answers `handled: false` and T3 opens the file in its own viewer. The decision has
1.5 seconds; T3 gives up after two and opens the file itself.

The plugin refuses any request carrying an `Origin` header, so web pages cannot drive it.

## Development

```sh
GOWORK=off go test -race ./...
```

`--help-agent` prints a guide for coding agents.

## Licence

Apache 2.0. See [LICENSE](LICENSE).
