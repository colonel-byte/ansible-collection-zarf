# Mage targets

Build and test targets run through [mage](https://magefile.org), not a Makefile. `magefiles/` does not build with a plain `go build` -- it needs mage's own build tag -- so drive it through the `mage` CLI, or through `go run ./magefiles/core <namespace>:<target>`. The second form needs only the Go toolchain and works on a host with no `mage` binary installed.

```sh
go run ./magefiles/core -l    # list every target
```

The dependency tree is vendored, so every target builds from `vendor/` and needs no module proxy. CI passes `-mod=vendor` with `GOPROXY=off` explicitly, so that an accidental fetch fails fast instead of quietly succeeding on a machine that happens to be online. After changing `go.mod`, run `go mod vendor` and commit the result -- `check-go-mod.yaml` fails the pull request otherwise.

## `Build`

| Target          | What it does                                                                                                                                                |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `build:modules` | Compiles the Ansible module wrapper from `cmd/zarfmod` into `build/zarfmod`, and links a `zarf_<module>` name to it for every module in the dispatch table. |

One binary under several names is what ZEP-0072 describes, and the names come from `zarfmod.ModuleFiles()` rather than a list in the target, so a module added to the dispatch table gets its symlink with nothing to keep in step. It is built for the host rather than for a release target list, because what drives it is the end-to-end suites and an operator reproducing a module run. The release builds come from `.goreleaser.yaml`. No version is stamped in: a module file reports what zarf did, and has no version of its own to report.

## `Test`

| Target                    | What it needs                                                                            |
| ------------------------- | ---------------------------------------------------------------------------------------- |
| `test:unit`               | Nothing. Runs every package except the suites under `test/`.                             |
| `test:endToEndNonCluster` | `ansible-playbook`, and a `zarf` for the cases that build a package. Builds the wrapper. |
| `test:endToEndCluster`    | `k3d`, `zarf`, `ansible-playbook`, Docker, and the network on first run.                 |
| `test:cleanClusters`      | `k3d`. Removes clusters an interrupted run left behind.                                  |

See [e2e-tests](e2e-tests.md) for what each suite covers.

## `Generate`

| Target              | What it writes                                                                                                      |
| ------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `generate:document` | `docs/ansible/module_*.md` and `role_*.md`, `docs/golang/`, `docs/index.md`, `docs/security.md`, `docs/SUMMARY.md`. |
| `generate:schema`   | `schema/*.json`, reflected from the Go types in `api/` and `magefiles/pkg/pkgvendor`.                               |
| `generate:dnfPins`  | The `ansible-core` and `bash-completion` pins in `containers/ansible/Dockerfile` and `.goreleaser.yaml`.            |

`generate:document` rewrites generated pages in place. The hand-written overviews beside them -- `docs/ansible/collection.md`, `modules.md`, `roles.md` -- are left alone, which is why only the `module_*` and `role_*` patterns are cleared before a run.

`generate:schema` writes the two published JSON Schemas: the `colonel-byte.dev/v1alpha1` `PackagePrerequisites` schema, from `api/colonel-byte.dev/v1alpha1/prerequisites`, and the schema of a reference package's `resources.json`, from `magefiles/pkg/pkgvendor`. The descriptions in both are the doc comments on those types, so a reworded comment is a schema change. `TestCommittedSchemasAreCurrent` fails when a committed schema stops matching its type -- a stale schema does not fail at build time, it validates the wrong document.

There is no `generate:zarfFlags` any more. It used to record the flag surface of the zarf commands the modules wrap into `internal/zarfmod/testdata/`, so their tests could check the rendered flags against a list. Both modules now link zarf's command tree in, so `TestInitArgsAreKnownFlags` and `TestDeployArgsAreKnownFlags` parse what they render against the real commands and pflag decides whether a flag exists -- which also catches a flag zarf renamed, the one thing a recorded list could not. The zarf they check against is the `github.com/zarf-dev/zarf` version in `go.mod`, which Dependabot watches. See [choice-zarf-ansible-module](../agent/choice-zarf-ansible-module.md).

`generate:dnfPins` moves the two dnf package pins the management node image installs. It fetches `repomd.xml` and `primary.xml.gz` from the AlmaLinux 10 AppStream and BaseOS repodata over HTTP, picks the newest `ansible-core` and `bash-completion` by RPM epoch-version-release ordering, and rewrites the `ARG` lines in `containers/ansible/Dockerfile` along with the `zarf-ansible` `build_args` in `.goreleaser.yaml`. Both files carry the same two versions on purpose -- the `ARG` defaults are what a local build gets and the `build_args` are what the release build passes -- so one run writes both, and a rewriter that no longer finds what it edits fails the target rather than leaving one of them behind. Dependabot's `docker` ecosystem moves the two `FROM` digests and nothing else, which is why these pins need a target at all.

Read the diff rather than committing it unseen: a pin that jumped a minor line is a change to what the image installs, and the image's own build is what proves it still works.

## `Examples`

The reference packages under `examples/packages` are illustrative, and the parts of them that track an upstream release are machine-maintained. These are the targets that maintain them. Nothing in CI runs them: `examples:vendor` reaches the network, and `examples:images` and `examples:lint` need a zarf.

| Target            | What it needs | What it does                                                                                                |
| ----------------- | ------------- | ----------------------------------------------------------------------------------------------------------- |
| `examples:vendor` | The network   | Fetches each package's manifests at its pinned tag and rewrites its `upstream/` tree, kustomizations and version |
| `examples:images` | `zarf`        | Rewrites each component's `images:` list from what `zarf dev find-images` discovers                         |
| `examples:lint`   | `zarf`        | Runs `zarf dev lint` over every example package                                                             |

A package opts in by holding a `resources.json`, which declares the upstream repository, the pinned tag, where to read the manifests from, and which files go into which subdirectory and zarf component. `app-fluxcd` and `app-argocd` hold one; `operator-external-secrets` ships its own manifests and vendors nothing.

The two differ in where the manifests live, which is why both are here. Flux attaches `manifests.tar.gz` to every release, so `app-fluxcd/resources.json` names an `asset`. Argo CD attaches only CLI binaries and SBOMs and documents applying `raw.githubusercontent.com/argoproj/argo-cd/<tag>/manifests/install.yaml` instead, so `app-argocd/resources.json` names a `tree` -- a directory in the tagged source tree, whose `match` entries are paths relative to it. Exactly one of the two is set, and a declaration naming both or neither is refused.

Moving a pin is a manual edit, which is the third kind of owner [`magefiles/AGENTS.md`](../../magefiles/AGENTS.md) allows and the one these pins have: write the new tag into `resources.json`, then run the two targets below in that order and read the diff. Nothing here discovers the upstream's latest release on its own -- the pin is on a reference package nothing deploys, and moving it is a reviewed change rather than a thing that happens to a tree.

The order matters when a pin moves: `examples:vendor` writes the manifests, and `examples:images` discovers images from the manifests that are now there.

```sh
go run ./magefiles/core examples:vendor    # after editing tag in resources.json, or to re-apply it as it is
go run ./magefiles/core examples:images
go run ./magefiles/core examples:lint
```

`magefiles/pkg/pkgvendor` is where that lives, and it is a Go port of the shell in [colonel-byte/zarf-pkg-fluxcd](https://github.com/colonel-byte/zarf-pkg-fluxcd)'s `.github/hack`. `ZARF_ANSIBLE_FLAGS_BINARY` names a zarf that is not on `PATH`. The name is the one the retired `generate:zarfFlags` read; these targets kept it rather than asking an operator to learn a second variable.
