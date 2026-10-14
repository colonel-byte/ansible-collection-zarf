# CodeQL

GitHub's static analysis, feeding the repository's security tab. Runs on pushes to `main`, on pull requests, on the merge queue, and weekly on a schedule -- the scheduled run is what keeps the tab current for advisories published after a commit landed.

Three languages are analysed in a matrix:

| Language  | Build mode | Why it is scanned                                                                               |
| --------- | ---------- | ----------------------------------------------------------------------------------------------- |
| `go`      | `manual`   | The wrapper binaries and the module logic. The build step decides which packages get extracted. |
| `python`  | `none`     | The action plugins, where parameters are delivered and `zarf_state_info`'s redaction lives.     |
| `actions` | `none`     | The workflows in this directory.                                                                |

The Go build names its packages explicitly -- `go build -mod=vendor ./cmd/... ./internal/...` -- rather than using `./...`. `magefiles/` is `package main` behind `//go:build mage`, and mage supplies the main function at runtime, so `go build` cannot compile it at all, with or without `-tags mage`. Those targets are dev tooling and ship in no binary, so they are left unanalysed. CodeQL's `paths-ignore` cannot express this: it does not apply to Go, which is analysed by being built, so the exclusion has to live in the build command.

The build runs with `GOPROXY=off` and `GOFLAGS=-mod=vendor`, matching [`unit-tests.yaml`](unit-tests.yaml.md) and [`e2e.yaml`](e2e.yaml.md), so accidental network access fails fast.
