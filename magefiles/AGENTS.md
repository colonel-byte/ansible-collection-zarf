# Working in `magefiles/`

This directory houses the repository's task automation layer using Mage.

## Layout and organization

The top level of `magefiles/` is strictly for Mage namespace declarations and targets, in `package main`. Do not place heavy domain logic, parsers, code generators, or template walkers directly in those files.

All implementation logic belongs in modular Go packages under `magefiles/pkg/`:

| Path | Purpose |
| ---- | ------- |
| `magefiles/build.go` | Mage entrypoint for the `Build` namespace (`build:modules`) |
| `magefiles/generate.go` | Mage entrypoint for the `Generate` namespace (`generate:document`, `generate:zarfFlags`) |
| `magefiles/test.go` | Mage entrypoint for the `Test` namespace (`test:unit`, `test:endToEnd*`, `test:cleanClusters`) |
| `magefiles/core/core.go` | Bootstrapping main package for `go run ./magefiles/core` |
| `magefiles/pkg/build/` | Compiling the two Ansible module files |
| `magefiles/pkg/gen/docs/` | The book's generators: the collection reference, the Go reference, the two copied-in pages, and `SUMMARY.md` |
| `magefiles/pkg/gen/zarfflags/` | Recording the flag surface of the zarf commands the modules wrap |
| `magefiles/pkg/testrunner/` | Unit and end-to-end test runners |
| `magefiles/pkg/util/` | Shared helper functions |

## Target structure rules

1. **Keep root namespace targets thin.** Root Mage methods should only perform argument defaults, environmental setup, or high-level status output, delegating immediately to the appropriate package under `magefiles/pkg/`.
2. **Encapsulate and test.** Logic under `magefiles/pkg/` should be written as standard Go packages with unit test files, following the table-driven test formatting rules in the root [`AGENTS.md`](../AGENTS.md).

## `magefiles/` does not build with a bare `go build`

The namespace files carry mage's `//go:build mage` tag, so `go build ./...` and `go vet ./...` skip them and a bare build of the directory fails. Drive them through the `mage` CLI, or `go run ./magefiles/core`. The packages under `magefiles/pkg/` carry no such tag and do build normally, which is why the unit tests can reach them.

## Every pin needs an owner

A pinned value is a version, tag, digest, or checksum written into the tree that names something living upstream. A pin with nothing watching it goes stale silently: nothing fails, the tree just keeps naming a version that stopped being current.

So a new pin is not done when it is written. It is done when something can move it. Pick one of two owners and wire it up in the same change:

- **A mage target that rewrites the pin in place**, when the current value has to be discovered and written back. `generate:zarfFlags` is the model: it reads the installed zarf's command tree and rewrites the recorded flag lists under `internal/zarfmod/testdata/`.
- **A [`.github/dependabot.yaml`](../.github/dependabot.yaml) entry**, when the pin sits in a file format Dependabot already parses. Prefer this when it fits: it is a few lines, it opens the PR unprompted, and it needs nobody to remember.

If neither fits, say so where the pin is. A documented manual bump is an acceptable third answer; an undocumented one is not, because a reader who finds a bare hash and no way to move it cannot tell a deliberate pin from an abandoned one.

The pinned storage-provider package tag in `test/e2e/cluster/main_test.go` is a deliberate third case: it is pinned so the suite does not test whatever was published this morning, and the comment beside it says so.
