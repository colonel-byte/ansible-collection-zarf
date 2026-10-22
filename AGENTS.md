# Working in this repository

This repository is the `colonel_byte.zarf` Ansible collection: a working proof of [ZEP-0072](https://github.com/zarf-dev/proposals/pull/73), which proposes native Ansible module support for [zarf](https://zarf.dev). Two modules converge a cluster -- `zarf_init` and `zarf_package_deploy` -- `zarf_package_pull` stages a package on disk, `zarf_package_mirror_resources` seeds a registry and git server from one, `zarf_package_remove` takes a deployed package off a cluster, `zarf_update_creds` rotates the credentials the cluster holds, `zarf_kubectl` passes a command to the kubectl zarf vendors, and five read a cluster. The seven that act are one binary under seven names, dispatched on `argv[0]`. See [`README.md`](README.md) for the pitch, [`docs/guides/zarf-ansible-module.md`](docs/guides/zarf-ansible-module.md) for the user guide, and [`docs/agent/choice-zarf-ansible-module.md`](docs/agent/choice-zarf-ansible-module.md) for the one place the proof deliberately diverges from the proposal.

The proof began in [colonel-byte/cargoship](https://github.com/colonel-byte/cargoship), beside the `colonel_byte.cargoship` collection whose shape it copies, and moved here once it had outgrown being a guest there. Several conventions in this tree -- the mage layout, the doc generators, the `AGENTS.md` habit itself -- arrived with it.

Nothing here runs on the cluster's nodes. The modules run the installed `zarf` on the node the task is delegated to, and zarf talks to one cluster through a kubeconfig.

## Layout

| Path                          | What lives there                                                                                                                                                                                |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `api/colonel-byte.dev/`       | The `colonel-byte.dev/v1alpha1` `PackagePrerequisites` Go types, which the published JSON Schema is reflected from                                                                              |
| `cmd/zarfmod/`                | The one `main` package behind every module file. A thirty-line entry point; everything it does is in `internal/zarfmod`                                                                         |
| `internal/zarfmod/`           | The module logic: `argv[0]` dispatch, the WANT_JSON intake, the command-line rendering, the progress reader, the stdout guard, the heartbeat, the response                                      |
| `internal/testenv/`           | Scaffolding the end-to-end suites share: the repo-root chdir, the per-run `TMPDIR`, and waiting on a cluster                                                                                    |
| `ansible/colonel_byte/zarf/`  | The collection: action plugins, the shared Python, roles. See [`ansible/colonel_byte/zarf/AGENTS.md`](ansible/colonel_byte/zarf/AGENTS.md)                                                      |
| `ansible/examples/playbooks/` | Runnable example playbooks                                                                                                                                                                      |
| `test/e2e/noncluster/`        | The suite that needs no cluster: the read-only modules and the state redaction against a stub zarf, and every acting module's refusals and check-mode answers                                   |
| `test/e2e/cluster/`           | The suite that needs k3d, zarf and Docker: the converging modules walked across two real clusters, then the other five acting modules against what the walk left                                |
| `magefiles/`                  | The task runner's source: build, test and generate targets. See [`magefiles/AGENTS.md`](magefiles/AGENTS.md)                                                                                    |
| `docs/`                       | The mdBook source. Several subpaths are generated and must not be hand-edited -- see [`docs/AGENTS.md`](docs/AGENTS.md)                                                                         |
| `docs/workflows/`             | One page per file under `.github/workflows/`, named after it with a `.md` extension -- required by [`.github/AGENTS.md`](.github/AGENTS.md)                                                     |
| `docs/agent/choice-*.md`      | Records of non-obvious decisions and the tradeoffs behind them -- read before reversing one                                                                                                     |
| `containers/ansible/`         | The management node image: `ansible-core`, the collection, both module files and a `zarf`. Its build asserts the dispatch contract                                                              |
| `schema/`                     | The generated JSON Schemas, written by `generate:schema` and committed because other repositories reference them                                                                                |
| `examples/packages/`          | Reference Zarf packages declaring their cluster prerequisites. Read, linted and inspected; nothing builds one                                                                                   |
| `skills/`                     | Skills for the two authoring tasks this repository is about: writing a Zarf package, and declaring its prerequisites. Connected-side authoring only. See [`skills/AGENTS.md`](skills/AGENTS.md) |

Several directories carry their own `AGENTS.md` with rules specific to that directory. Check the closest one before editing. Keep this table in sync when adding or removing one.

| `AGENTS.md`                                                                  | Covers                                                                                                                                   |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| [`ansible/AGENTS.md`](ansible/AGENTS.md)                                     | Markdown line formatting rules that feed generated docs                                                                                  |
| [`ansible/colonel_byte/zarf/AGENTS.md`](ansible/colonel_byte/zarf/AGENTS.md) | The collection's contract: the two kinds of module, the `cli_flag` key, quoting every description, the test lists a new plugin must join |
| [`docs/AGENTS.md`](docs/AGENTS.md)                                           | Markdown formatting, generated pages, redirects for a moved page                                                                         |
| [`skills/AGENTS.md`](skills/AGENTS.md)                                       | The authoring-only scope of the two skills, and the tests that hold a page to the format                                                 |
| [`magefiles/AGENTS.md`](magefiles/AGENTS.md)                                 | Mage target layout and structure rules                                                                                                   |
| [`test/e2e/AGENTS.md`](test/e2e/AGENTS.md)                                   | Fixtures belong in files, not Go string constants                                                                                        |
| [`.github/AGENTS.md`](.github/AGENTS.md)                                     | Scorecard-safe workflow changes, the workflow docs under `docs/workflows/`, the issue forms, PR description format                       |

Three things in the root are symlinks, and they exist so that the real file sits where a reader would look for it rather than inside a tool's directory:

| Symlink          | Points at           | So that                                                                      |
| ---------------- | ------------------- | ---------------------------------------------------------------------------- |
| `CLAUDE.md`      | this file           | a Claude Code session started here loads these rules without being asked     |
| `.claude/skills` | [`skills/`](skills) | the same session loads those skills                                          |
| `.agents/skills` | [`skills/`](skills) | an agent runner that is not Claude Code finds them in the tool-neutral place |

`.agents/` is the same convention as the `AGENTS.md` filename this repository already uses everywhere: a location agreed between tools rather than owned by one. Both skill symlinks point at the same `skills/` tree, so there is exactly one copy of every page and adding a skill means touching neither -- which is the whole reason they are symlinks and not a directory per tool. A copy would be a second thing to keep current, and nothing would report it going stale.

That covers the root and nothing else: the directory-specific files above are not symlinked. Do not treat "nothing was loaded for me" as "there are no rules here", and do not grep a rules file for the one thing you thought to ask about. Open the closest `AGENTS.md` and read it.

## Building and testing

Targets run through mage, not a Makefile. `magefiles/` does not build with a plain `go build` -- it needs mage's own build tag -- so drive it through the `mage` CLI or through `go run ./magefiles/core <namespace>:<target>`. The latter needs only the Go toolchain.

```sh
go run ./magefiles/core build:modules            # both module files into build/
go run ./magefiles/core test:unit                # everything but the e2e suites
go run ./magefiles/core test:endToEndNonCluster  # needs ansible-playbook
go run ./magefiles/core test:endToEndCluster     # needs k3d, zarf, Docker, network
go run ./magefiles/core generate:document        # regenerate the book's generated pages
go run ./magefiles/core generate:schema          # regenerate schema/ from the Go types in api/
go run ./magefiles/core examples:lint            # zarf dev lint over every reference package
```

See [`docs/dev/mage.md`](docs/dev/mage.md) for the full reference and [`docs/dev/e2e-tests.md`](docs/dev/e2e-tests.md) for what each suite covers.

Plain Go commands work for anything mage does not wrap -- `go build ./...`, `go vet ./...` -- but exclude `./magefiles/...` from a bare build, for the reason above.

## The dependency tree is vendored

`vendor/` is committed, and every build and CI job reads from it. The management node this collection runs on is typically air-gapped, so a build has to work with no module proxy reachable -- which means the tree is not a cache, it is the dependency list.

Two things follow:

- **A `go.mod` change is not finished until `go mod vendor` has run and the result is committed.** `go build` happily uses a `vendor/` that no longer matches `go.mod`, so nothing fails locally. [`check-go-mod.yaml`](.github/workflows/check-go-mod.yaml) re-vendors and fails on any diff, which is the only thing that catches it.
- **Nothing edits `vendor/` by hand.** The pre-commit hooks exclude it for that reason: `addlicense` would stamp a header onto other people's code, and the whitespace fixers would rewrite it, in both cases making the tree differ from what `go mod vendor` produces. If a hook starts touching `vendor/`, the fix is an exclude, not a reformatted dependency.

The `tool` directives in `go.mod` are vendored too, so `generate:document` runs gomarkdoc from `vendor/` and needs nothing fetched.

**Never run `go test ./...` without scoping it.** The e2e suites need Ansible, k3d and Docker. Use `test:unit`, or name packages explicitly.

## The tests that hold the two halves together

Most of what is worth knowing about this repository is enforced by five tests in `internal/zarfmod`, and it is worth knowing which one catches what. A module that acts on a cluster has one `*ArgsAreKnownFlags` test apiece, each parsing what it renders against zarf's own command for it:

A test file in `internal/zarfmod` is named for the source file it covers -- `init_test.go` beside `init.go`, `progress_test.go` beside `progress.go` -- with three exceptions, each named for what it holds rather than what it is beside: `helpers_test.go` is the scaffolding the package shares, and `plugindocs_test.go` and `infoplugins_test.go` are the two contract tests that reach into `ansible/`. Put a new test beside its subject; a file that collects whatever was written most recently is how the tests below stop being findable.

| Test                                                           | What it would catch                                                                                             |
| -------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `TestActionPluginDocsMatchModuleParams`                        | A parameter added to a wrapper module's Go struct and not to its Python `DOCUMENTATION`, or the reverse         |
| `TestInfoPluginDocsMatchRenderedFlags`                         | A read-only module's documented `cli_flag` values disagreeing with the flags its `command_parts` function names |
| `TestEveryActionPluginIsCovered`                               | A new action plugin that no test looks at, and a test list naming a plugin that no longer exists                |
| `TestInitArgsAreKnownFlags` and the other `*ArgsAreKnownFlags` | A flag the module renders that the linked-in zarf does not accept, including one zarf renamed                   |
| `TestModulesAnswersEveryAction`                                | A module added to the dispatch table without the symlink names, packaging and docs that go with it              |

Three more, in `magefiles/pkg/gen/schema`, hold the `PackagePrerequisites` format together the same way:

| Test                                   | What it would catch                                                                                                                                 |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestCommittedSchemasAreCurrent`       | A field added to a type in `api/` without `generate:schema` being run, leaving a schema that validates the wrong document                           |
| `TestExampleDocumentsMatchTheTypes`    | A reference package's `docs/prerequisites.yaml` written against an older shape of the format                                                        |
| `TestEveryRequirementIsNamedByTheRole` | A field added under `requires` that no task in the `prerequisites` role mentions -- a prerequisite a package declares and the role silently ignores |
| `TestRenderedSchemaRefusesBothEngines` | The mutual exclusivity of `requires.fluxcd` and `requires.argocd` going missing from the generated schema                                           |

`TestInitArgsAreKnownFlags` and `TestDeployArgsAreKnownFlags` parse what the modules render against zarf's own command tree, which is linked in, so pflag decides whether a flag exists and a zarf release that renamed one fails them. There is nothing to refresh by hand: the zarf they check against is the `github.com/zarf-dev/zarf` version in `go.mod`.

Those are Go tests over the Go types. The generated schema itself is held by a pre-commit hook: `check-jsonschema` validates every reference package's `docs/prerequisites.yaml` and `resources.json` against the schema generated from `api/`. That is a different check from the Go ones, and the difference is the point -- they hold a document against the *types*, and the hook holds it against the *schema a consumer actually validates with*. A `jsonschema` tag that rendered wrong would leave the two disagreeing, and nothing reflecting over the types could see it.

## Keeping `.gitattributes` in sync with the generators

Everything a generator writes is marked `linguist-generated` in [`.gitattributes`](.gitattributes), which collapses it in a GitHub diff and keeps it out of the repository's language statistics. A generated file that is not marked shows up as hand-written work in every review.

So a change to what a generator produces is a change to `.gitattributes` as well: a new output path gets an entry, a moved or removed one loses its entry. Hand-written files inside a generated tree stay unmarked: `docs/ansible/collection.md`, `modules.md` and `roles.md` sit beside generated module and role pages, each reference package's `zarf.yaml` and `docs/prerequisites.yaml` sit beside its vendored `upstream/` tree, and `skills/zarf-package-prerequisites/SKILL.md` sits beside its generated copy of the schema. Mark the pattern the generator writes, not the directory it writes into.

Check a specific file rather than reasoning about the patterns:

```sh
git check-attr linguist-generated -- docs/ansible/module_init.md
```

## Keeping `README.md` current

[`README.md`](README.md) is the repository's front page, and it is also the source for the book's landing page -- `magefiles/pkg/gen/docs/book.go` copies it to `docs/index.md` -- so a stale README is stale in two places. Treat it as part of the change: a new module belongs in the module table with a runnable snippet, a removed one is deleted everywhere it is mentioned, and a behaviour change fixes the prose and the snippet together. After editing it, run `generate:document` so `docs/index.md` matches.

A relative link in `README.md` must point at something under `docs/`, or the generator fails naming it: the book is built from `docs/`, so no other relative path resolves there. Link to anything outside `docs/` with an absolute URL.

## Go test formatting

Do not condense struct literals in table-driven test cases onto single lines. Format each field on its own line.

```go
// Do not condense:
{name: "unconstrained release", release: "", want: true},

// Prefer multi-line:
{
	name:    "unconstrained release",
	release: "",
	want:    true,
},
```

## Writing style

Use `-` (hyphen), not `—` (em dash), wherever a hyphen reads fine.

Markdown is not hard-wrapped: write a paragraph or a list item on one continuous line and let the renderer wrap it. That rule, with examples, is in [`docs/AGENTS.md`](docs/AGENTS.md) and [`ansible/AGENTS.md`](ansible/AGENTS.md). The table rule is here, because it applies to every Markdown file in the tree, including this one.

### Pad table cells so the columns line up

Tables are written in their aligned form, so that the source reads as a table and a reviewer can see the columns without rendering the file. Pad every cell with spaces to the width of the widest cell in its column, keep one space of padding inside each pipe, and run the dashes of the delimiter row out to that same width. Every line of the table then has the same length.

Write this:

```markdown
| Path                                   | What it holds     |
| -------------------------------------- | ----------------- |
| `.spec.config.registries[N].auth.user` | Registry username |
```

Not this:

```markdown
| Path | What it holds |
| --- | --- |
| `.spec.config.registries[N].auth.user` | Registry username |
```

Adding a row that is wider than the column repads the whole table, which is expected - the alignment is part of the content, not a one-time cleanup.

The `pad-markdown-tables` pre-commit hook does this for you: [`hack/markdown-tables.py`](hack/markdown-tables.py) repads every table in the files being committed and touches nothing else -- no rewrapping, no normalised bullets or emphasis. It skips tables inside fenced code blocks, which is what keeps the unaligned example above an example rather than something the hook quietly aligns. A second hook, `markdown-tables-self-check`, asserts that: `markdown-tables.py --self-check` runs a fenced example through the formatter and fails if it came out changed.

## Commit messages

Keep commit messages to a short title only - no body paragraph explaining the change. End with a `Co-Authored-By` trailer.

```
test(e2e): cover the state role's set_fact censoring

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
```

## Committing changes

Run the hooks over the whole tree before every commit and fix anything they flag. Prefer [`prek`](https://github.com/j178/prek) if it is on `PATH`; it reads the same [`.pre-commit-config.yaml`](.pre-commit-config.yaml) and runs the same hooks, just faster. Fall back to `pre-commit run --all-files` when it isn't. Several hooks rewrite files - re-stage after the run. Don't skip hooks with `--no-verify` or `SKIP=`.

```sh
prek run --all-files -c .pre-commit-config.yaml
```

**The `-c` is load-bearing, and only for prek.** Unlike pre-commit, prek discovers nested `.pre-commit-config.yaml` files and runs each as its own project - and two vendored dependencies ship one, so a bare `prek run --all-files` runs *their* hooks over *their* code. It fails on things that are none of this repository's business: a `shellcheck` it does not have installed, a `golangci-lint` whose Go version does not match, typecheck errors inside a dependency. Naming the config explicitly turns workspace discovery off, so only this repository's hooks run. The per-hook `exclude: vendor/.*` entries do not help here - they filter the files a hook walks, not which configs prek finds - and neither does a top-level `exclude`.

## Pull requests

Before creating or updating a pull request description, read [`.github/pull_request_template.md`](.github/pull_request_template.md) and follow the format rules in [`.github/AGENTS.md`](.github/AGENTS.md).

## Quirks worth knowing

### `docs/agent/choice-*.md` are constraints, not history

When a decision looks reversible from the code alone -- a read-only module built as a plain action plugin rather than a wrapper, a redaction that removes keys instead of blanking them, two near-identical k3d configs -- check `docs/agent/` first. These are the record of a tradeoff already made on purpose, not an oversight waiting to be cleaned up.

[`choice-zarf-collection-removal.md`](docs/agent/choice-zarf-collection-removal.md) is the odd one: it was written as a removal plan for cargoship, and reads that way. What survives the move is its account of what this proof is for and when it stops being worth maintaining.

### Relative-path constants don't move themselves

`collectionRoot` in `internal/zarfmod/plugindocs_test.go`, `repoRoot` in `magefiles/pkg/devtools/dnfpins/dnfpins_test.go`, and the link out of `ansible/colonel_byte/zarf/plugins/modules/README.md` all hand-encode `../` depth to reach the repository root. Moving one of those endpoints changes the required count, and nothing catches a wrong one except running the test -- `go vet` only compiles.

### The e2e suites skip rather than fail

Every test in both suites checks its prerequisites and calls `t.Skip` with a reason when one is missing. A green run is not proof that anything ran; read the output. This is deliberate -- the alternative is a suite that fails on every contributor's laptop -- but it means a suite that silently stopped running looks exactly like a suite that passed.

### `build/tmp` breaks every `./...` command while an e2e run is in flight

The e2e targets point `TMPDIR` at `build/tmp` inside the repository, so the `go test` process that runs a suite puts its own build scratch there too -- a `go-build<n>/` tree of generated cgo. `go list ./...` walks the filesystem rather than the index, so `build/.gitignore` does not keep it out, and loading that package fails:

```
package github.com/colonel-byte/ansible-collection-zarf/build/tmp/go-build.../b170
    .../src/runtime/cgo/cgo.go:39:8: use of internal package internal/runtime/sys not allowed
```

It names the Go toolchain's own source, so it reads as a broken checkout or a bad toolchain. It is neither: it is leftovers from a suite running in another terminal, or from one that was interrupted before it cleaned up.

While that directory exists, **`go build ./...`, `go vet ./...` and `go list ./...` all fail**, and so does the `golangci-lint` hook -- `.golangci.yaml` excludes `^build/`, but that suppresses *issues* in a path, not a package that fails to load. `test:unit` is immune: it lists with `go list -e` so one unbuildable directory cannot take the command down, then drops `build/` in `unitPackages`. Nothing else is.

So: scope the command (`go build ./internal/... ./cmd/...`), or wait for the suite to finish, or remove the `go-build*` tree if no suite is running. A run that exits normally takes its own `go-build` tree with it, which is why this is only ever hit mid-run or after an interrupt.

Do not `rm -rf build/tmp` wholesale. `build/tmp/zarf-e2e` is the e2e package cache, kept between runs on purpose -- it is around 800MB of pinned init and provider packages, and removing it means every suite re-downloads all three.
