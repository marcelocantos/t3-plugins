# t3-plugins

A collection of plugins for the T3 Code plugin build (https://github.com/marcelocantos/t3code).

- Read [docs/authoring.md](docs/authoring.md) before building or changing a plugin. The protocol's
  source of truth is `docs/operations/panel-plugins.md` in the fork
  (`~/work/github.com/marcelocantos/t3code` locally); when the two disagree, the fork wins and
  the guide should be fixed.
- One directory per plugin, each self-contained with its own README, build and tests. Go plugins
  are separate modules (`<dir>/go.mod`, path `github.com/marcelocantos/t3-plugins/<dir>`); build
  and test with `GOWORK=off` from the plugin's directory.
- Prefer a sidecar (T3 starts the binary) over a standalone server unless the plugin genuinely
  needs to outlive T3 or serve several environments.
- Add each new plugin to the table in README.md.
