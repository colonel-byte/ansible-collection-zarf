# Validate pre-commit hooks

Runs the hooks from [`.pre-commit-config.yaml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.pre-commit-config.yaml) over the whole tree, so licensing, formatting and sorting can never drift from the local pre-commit setup. Runs on every pull request and on the merge queue.

It uses [`prek`](https://github.com/j178/prek) rather than `pre-commit`: same config, same hooks, faster. That matches the local instruction to prefer `prek`.

It passes `--config .pre-commit-config.yaml`, and that flag is load-bearing rather than explicit-for-its-own-sake. prek discovers nested `.pre-commit-config.yaml` files and runs each as a separate project, and two vendored dependencies ship one of their own. Without the flag this job runs *their* hooks over *their* code and fails on their toolchain -- a `shellcheck` that is not installed, a `golangci-lint` built against a different Go -- rather than on anything in this repository. Naming the config turns discovery off. The local command carries the same flag for the same reason.

Two hooks are skipped:

| Hook             | Why                                                                                               |
| ---------------- | ------------------------------------------------------------------------------------------------- |
| `docs-generator` | A `repo: local` hook that shells out to mage and regenerates files; needs the full dev toolchain. |
| `golangci-lint`  | Already covered by [`scan-lint.yaml`](scan-lint.yaml.md).                                         |

Every hook that walks files excludes `vendor/`. That is not tidiness: `addlicense` would stamp a header onto other people's code and the whitespace fixers would rewrite it, in both cases making the tree differ from what `go mod vendor` produces -- which [`check-go-mod.yaml`](check-go-mod.yaml.md) then fails on. If a hook starts touching `vendor/`, the fix is an exclude. An exclude does not cover the nested-config case above, though, because it filters the files a hook walks rather than which configs prek finds.
