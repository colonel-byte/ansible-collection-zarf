# Mage targets

Build and test targets run through [mage](https://magefile.org), not a Makefile. `magefiles/` does not build with a plain `go build` -- it needs mage's own build tag -- so drive it through the `mage` CLI, or through `go run ./magefiles/core <namespace>:<target>`. The second form needs only the Go toolchain and works on a host with no `mage` binary installed.

```sh
go run ./magefiles/core -l    # list every target
```

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

`generate:document` rewrites generated pages in place. The hand-written overviews beside them -- `docs/ansible/collection.md`, `modules.md`, `roles.md` -- are left alone, which is why only the `module_*` and `role_*` patterns are cleared before a run.

`generate:zarfFlags` is the refresh for the recorded flag lists `TestInitArgsAreKnownFlags` and `TestDeployArgsAreKnownFlags` check against. Run it when the zarf the modules run moves, and review the diff: a flag that disappeared is a module parameter that no longer works. Set `ZARF_ANSIBLE_FLAGS_BINARY` to name a zarf that is not on `PATH`. It deliberately does not read the flags at test time -- the unit tests run with no zarf on the machine, in CI and on a contributor's laptop, and a test that needs a binary to say anything is a test that gets skipped.
