# Writing a T3 Code plugin

This guide is for coding agents asked to build a plugin for the
[plugin build of T3 Code](https://github.com/marcelocantos/t3code) (local checkout:
`~/work/github.com/marcelocantos/t3code`). It orients and routes. The protocol itself is specified
in [panel plugins](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md)
(abbreviated *the spec* below), with its schemas in `packages/contracts/src/panelPlugins.ts` and the
server side in `apps/server/src/plugins/`. Read the spec in full before writing code; when this guide
and the spec disagree, the spec wins and this guide should be fixed.

## The model

A plugin is a separate process that speaks protocol version 1 over a WebSocket. T3 never loads plugin
code. The T3 server connects to the plugin from the machine it runs on and relays everything to its
clients over their own authenticated connection, so a phone or remote browser never reaches the
plugin directly. A plugin therefore runs in the T3 server's environment: its filesystem, its
processes, its screen. A loopback address in a plugin's configuration means the server's machine,
not the user's.

What a plugin can contribute:

- **Panels.** Live sections that the plugin pushes as complete replacement snapshots per
  subscription. Thread panels sit in thread details; environment panels sit in the left sidebar
  and see every project of the environment. See
  [protocol version 1](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#protocol-version-1)
  and [environment panels](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#row-actions-surfaces-and-environment-panels).
- **Structured surfaces.** Rows, badges, actions, hover cards and dialogs that T3 renders from
  data. The plugin never supplies executable UI to T3's page. Prefer these; they work everywhere
  and need only `orchestration:read` to view.
- **Web surfaces.** A `kind: "web"` panel, a web link card or a web dialog is a page the plugin
  serves, shown in an opaque-origin sandbox through T3's gateway, with a narrow host bridge for
  popovers, dialogs and agent prompts. Use them only when structured surfaces cannot express the
  UI. See [embedded web applications](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#embedded-web-applications)
  and [host bridge](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#host-bridge).
- **Link cards.** Up to 16 `prefix` or `glob` patterns that claim chat links, web or file. A
  matching link gets a hover card unless the pattern says `"card": false`. See
  [link cards](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#link-cards).
- **Link takeover.** A pattern can rewrite where a link leads (`href` template), ask the plugin
  for each link's target and text once (`resolve`), show the card on click (`click: "card"`), or
  hand plain clicks to the plugin (`click: "activate"`), which answers whether it handled the click.
  See [taking a link over](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#taking-a-link-over).

A plugin declares what it serves with `scopes`: `thread` (the default), `environment` and `links`.
Each scope widens what the plugin learns about the user's machine, so discovered plugins need the
user's consent per scope (see [Discovery](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#discovery)).
Individual capabilities are further gated by the client's permissions: actions, activation, web
panels and web cards need `orchestration:operate`, and a client without it never sends those
requests.

## Choosing the shape

There are two ways for T3 to reach a plugin.

**Sidecar (preferred).** The user adds an entry to `panel-plugins.json` beside the environment's
`settings.json`. T3 starts the command, reads a readiness record from its stdout, connects, shares
that one process across every subscribing thread, restarts it if it dies, and sends SIGTERM when
nothing needs it any more. See
[configure other installations](https://github.com/marcelocantos/t3code/blob/main/docs/operations/panel-plugins.md#configure-other-installations).
The entry is the plugin's whole declaration: `scopes` and `links` come from it, and T3 never probes a
sidecar (`apps/server/src/plugins/PluginDiscovery.ts` passes hand-written entries through as written).
Choose this for small, single-purpose plugins. There is nothing for the user to keep running.

**Independent server with a discovery advertisement.** The plugin runs on its own, serves the
protocol at a loopback HTTP URL, and writes a small manifest into `$XDG_DATA_HOME/t3code/plugins/`.
T3 probes the URL, offers the plugin in **Settings → Plugins**, and connects only after the user
enables it. Choose this only when the plugin must outlive T3, or is one face of a larger daemon, as
Bullseye and Jevons are. Discovery files can never name a command, by design: T3 does not start
discovered programs, so anything T3 launches must come from a file the user wrote. An existing
server can also be configured by hand with `"kind": "server"` in `panel-plugins.json`.

## Walkthrough: open-with

[open-with](../open-with/) is the smallest real plugin here: a Go sidecar with one link pattern and
no panels. A plain click on a Markdown file link asks it to open the file with the machine's default
application; when it declines, T3 opens the file itself. Read it in this order:

- `README.md` shows the `panel-plugins.json` entry: `"kind": "sidecar"`, `"scopes": ["links"]` and
  one pattern `{ "glob": "/**/*.{md,markdown}", "click": "activate", "card": false }`.
- `main.go` is the sidecar lifecycle: listen on `127.0.0.1:0`, print the readiness record
  `{"type":"ready","protocol":1,"url":"ws://127.0.0.1:<port>/t3"}` as one stdout line, log to
  stderr, and close on SIGTERM.
- `plugin.go` is the protocol: refuse any request with an `Origin` header, accept the WebSocket,
  read frames, ignore frames without a `requestId` (a links-only plugin gets no subscriptions),
  answer each request on its own goroutine with a `result` carrying the same `requestId`, and
  answer unsupported request types with `ok: false` and an `error`. It also answers a plain `GET`
  with a probe body, which a sidecar does not need; a server that advertises itself does.
- `activate.go` is the decision, written as a pure function over an `Opener` interface so tests can
  substitute a fake.
- `opener_<os>.go` holds the platform code behind build constraints, keeping it out of the
  decision logic.
- `plugin_test.go` tests `Decide` with a table and drives the real handler over a WebSocket.

Copy this skeleton for a new links plugin. A panels plugin keeps the same lifecycle and transport
and adds `subscribe` and `unsubscribe` handling that pushes `panels` snapshots.

## Pitfalls

Each of these is a mistake an agent is likely to make. The source is in parentheses.

- **Activation has two seconds, and you must decide before acting.** T3 waits two seconds for an
  `activate` answer, then the client follows the link or opens the file itself
  (`ACTIVATE_TIMEOUT` in `apps/server/src/plugins/PanelPlugins.ts`; spec, *Taking a link over*).
  If you act and then answer late, the user gets both. Bound your own decision well inside the
  deadline (open-with uses 1.5 s), and do nothing visible unless you will answer `handled: true`.
- **`client.local` says where the user is, not who they are.** It is true only when the client
  claims to run on this machine and its connection arrives from loopback; mobile is never local
  (`linkActivate` in `PanelPlugins.ts`). A tunnel or SSH forward also arrives on loopback, so never
  treat it as authentication. Use it to avoid opening windows on a screen nobody is looking at.
- **Refuse requests that carry an `Origin` header.** T3's WebSocket client sends none; a browser
  always does. Rejecting them stops any web page from driving a loopback plugin (spec, end of
  *Protocol version 1*; `plugin.go` in open-with). A remotely deployed plugin needs real access
  control instead.
- **Reconnects replay subscriptions.** After any disconnect, T3 restarts the sidecar or reconnects
  every second and resends `subscribe` for every live subscription; answer each with a fresh
  complete snapshot (spec, *Protocol version 1*). Environment subscriptions are also resent with
  the same `subscriptionId` whenever the project set changes; treat that as a replacement. Keep
  per-connection state rebuildable from the `subscribe` frame alone.
- **Sidecars must be ready within ten seconds.** T3 kills and retries a sidecar that has not printed
  a valid readiness record by then, and only accepts a record whose URL host is `127.0.0.1`,
  `localhost` or `[::1]` (`START_TIMEOUT_MS` and `start()` in
  `apps/server/src/plugins/PanelPluginConnection.ts`). Print it after the listener is bound and
  before any slow initialization. Other stdout lines are ignored as diagnostics.
- **Malformed frames disconnect you.** T3 validates every frame against the contracts; one that
  fails, or a snapshot with duplicate panel or row ids, closes the connection and shows an error
  (spec, *Protocol version 1*). Respect the limits there: 1 MiB frames, 20 panels, 2,000 rows.
- **Hover cards and resolution are reads.** T3 asks for a link card when the user hovers or focuses
  the link, and asks `resolve` the first time a client shows it (spec, *Link cards* and *Taking a
  link over*). Neither may have side effects, and clients cache both, so do not rely on being asked
  again. Only `action` and `activate` mean the user asked for something to happen.
- **Mobile differs.** Panels appear below the thread list rather than in thread details; a tap on a
  claimed link shows the card unless `click` is `"open"` or `"activate"` or the pattern has no card;
  resolved `text` replaces the link's text even when the message gave it its own; every surface is a
  sheet and anchors are ignored; the `surface.hover` bridge feature is not offered; and `client.local`
  is always false (spec, *Configure other installations*, *Taking a link over* and *Host bridge*).
  Design so that declining on mobile still gives the user T3's ordinary behaviour.
- **Widening scopes asks the user again.** For a discovered plugin, T3 keeps the scopes the user
  approved; declaring a new one, such as `environment` or `links`, waits for **Allow**, while
  dropping a scope or changing link patterns needs no consent (spec, *Discovery* and *Link cards*;
  `pendingScopes` in `PluginDiscovery.ts`). A hand-written `panel-plugins.json` entry is trusted as
  written. A new version that adds a scope will appear broken until the user approves it.

## Testing

Unit-test the decision logic directly, as open-with's `TestDecide` does: the conditions under which
the plugin acts, declines or errors are where bugs live. Add one protocol test that drives the real
handler over a WebSocket (request in, `result` with the same `requestId` out, `Origin` refused).
Do not mock T3 beyond that.

Then do one integrated pass in a real client, following the fork's
[`test-t3-app`](https://github.com/marcelocantos/t3code/blob/main/.agents/skills/test-t3-app/SKILL.md)
skill (`.agents/skills/test-t3-app/SKILL.md` in the checkout):

1. Work in a git worktree of the fork, not the main checkout, and run `vp run dev` there. Its state
   lives in the worktree's ignored `.t3`, and the first run seeds it with a copy of real projects
   and threads when the source database is small enough (otherwise run `vp run migrate-dev-db`
   with the server stopped). Never point a dev server at `~/.t3/userdata`; that is the developer's live install.
2. Register the plugin in the dev home: write the entry into `<worktree>/.t3/userdata/panel-plugins.json`,
   with `command` pointing at a binary you built (an absolute path). T3 picks the file up within ten
   seconds while a client is connected. A discovered server plugin instead writes its manifest and is enabled in
   **Settings → Plugins**; note that the manifest directory is shared by every T3 on the machine.
3. Put a link or row in front of the plugin without paying for a model: the free `Replay (dev)`
   provider that `vp run dev` registers replays recorded turns, and an existing seeded thread may
   already contain suitable links.
4. Exercise every path you changed, including the decline path and a client without
   `orchestration:operate` where it matters, and capture the result.

Stop only the processes you started, by the PID you captured.

## Adding a plugin to this repo

- One directory per plugin, named for what it does (`open-with`), self-contained: code, tests,
  `README.md`, build.
- Go plugins are separate modules: `<dir>/go.mod` with module path
  `github.com/marcelocantos/t3-plugins/<dir>`. Build and test from the plugin's directory with
  `GOWORK=off`, for example `GOWORK=off go test -race ./...`, so a parent `go.work` cannot change
  the build.
- Give the plugin a stable `id`. A sidecar entry may use any id, but use
  `github.com/marcelocantos/t3-plugins/<dir>`, which is also the form discovery requires.
- Build output goes outside the repo or under `<dir>/bin/`, which `.gitignore` excludes.
- The plugin's `README.md` says what it does from the user's side, how it decides when that is not
  obvious, how to build it, and the exact `panel-plugins.json` entry (or, for a server, how it
  advertises itself). Name the fork's userdata folder for that file: `~/.t3-plugins/userdata` for
  the parallel **T3 Code+** build, `~/.t3/userdata` for the upgrade build (spec, *Build modes*).
- Add a row to the table in the root `README.md`.
