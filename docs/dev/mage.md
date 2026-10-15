# Mage targets

Build and test targets run through [mage](https://magefile.org), not a Makefile. `magefiles/` does not build with a plain `go build` -- it needs mage's own build tag -- so drive it through the `mage` CLI, or through `go run ./magefiles/core <namespace>:<target>`. The second form needs only the Go toolchain and works on a host with no `mage` binary installed.

```sh
go run ./magefiles/core -l    # list every target
```

The dependency tree is vendored, so every target builds from `vendor/` and needs no module proxy. CI passes `-mod=vendor` with `GOPROXY=off` explicitly, so that an accidental fetch fails fast instead of quietly succeeding on a machine that happens to be online. After changing `go.mod`, run `go mod vendor` and commit the result -- `check-go-mod.yaml` fails the pull request otherwise.

## `Build`

| Target          | What it does                                                                                                                 |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `build:modules` | Compiles both Ansible module files -- `zarf_init` from `cmd/init`, `zarf_package_deploy` from `cmd/deploy` -- into `build/`. |

They are built for the host rather than for a release target list, because what drives them is the end-to-end suite and an operator reproducing a module run. The release builds come from `.goreleaser.yaml`. No version is stamped in: a module file reports what zarf did, and has no version of its own to report.

## `Test`

| Target                    | What it needs                                                            |
| ------------------------- | ------------------------------------------------------------------------ |
| `test:unit`               | Nothing. Runs every package except the suites under `test/`.             |
| `test:endToEndNonCluster` | `ansible-playbook`.                                                      |
| `test:endToEndCluster`    | `k3d`, `zarf`, `ansible-playbook`, Docker, and the network on first run. |
| `test:cleanClusters`      | `k3d`. Removes clusters an interrupted run left behind.                  |

See [e2e-tests](e2e-tests.md) for what each suite covers.

## `Generate`

| Target               | What it writes                                                                                                      |
| -------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `generate:document`  | `docs/ansible/module_*.md` and `role_*.md`, `docs/golang/`, `docs/index.md`, `docs/security.md`, `docs/SUMMARY.md`. |
| `generate:zarfFlags` | `internal/zarfmod/testdata/zarf-init-flags.txt` and `zarf-package-deploy-flags.txt`, read from the installed zarf.  |
| `generate:dnfPins`   | The `ansible-core` and `bash-completion` pins in `containers/ansible/Dockerfile` and `.goreleaser.yaml`.            |

`generate:document` rewrites generated pages in place. The hand-written overviews beside them -- `docs/ansible/collection.md`, `modules.md`, `roles.md` -- are left alone, which is why only the `module_*` and `role_*` patterns are cleared before a run.

`generate:zarfFlags` is the refresh for the recorded flag lists `TestInitArgsAreKnownFlags` and `TestDeployArgsAreKnownFlags` check against. Run it when the zarf the modules run moves, and review the diff: a flag that disappeared is a module parameter that no longer works. Set `ZARF_ANSIBLE_FLAGS_BINARY` to name a zarf that is not on `PATH`. It deliberately does not read the flags at test time -- the unit tests run with no zarf on the machine, in CI and on a contributor's laptop, and a test that needs a binary to say anything is a test that gets skipped.

`generate:dnfPins` moves the two dnf package pins the management node image installs. It fetches `repomd.xml` and `primary.xml.gz` from the AlmaLinux 10 AppStream and BaseOS repodata over HTTP, picks the newest `ansible-core` and `bash-completion` by RPM epoch-version-release ordering, and rewrites the `ARG` lines in `containers/ansible/Dockerfile` along with the `zarf-ansible` `build_args` in `.goreleaser.yaml`. Both files carry the same two versions on purpose -- the `ARG` defaults are what a local build gets and the `build_args` are what the release build passes -- so one run writes both, and a rewriter that no longer finds what it edits fails the target rather than leaving one of them behind. Dependabot's `docker` ecosystem moves the two `FROM` digests and nothing else, which is why these pins need a target at all.

Read the diff rather than committing it unseen: a pin that jumped a minor line is a change to what the image installs, and the image's own build is what proves it still works.
