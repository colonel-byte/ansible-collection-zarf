# Validate pre-commit hooks

Runs the hooks from [`.pre-commit-config.yaml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.pre-commit-config.yaml) over the whole tree, so licensing, formatting and sorting can never drift from the local pre-commit setup. Runs on every pull request and on the merge queue.

It uses [`prek`](https://github.com/j178/prek) rather than `pre-commit`: same config, same hooks, faster. That matches the local instruction to prefer `prek run --all-files`.

Two hooks are skipped:

| Hook             | Why                                                                                               |
| ---------------- | ------------------------------------------------------------------------------------------------- |
| `docs-generator` | A `repo: local` hook that shells out to mage and regenerates files; needs the full dev toolchain. |
| `golangci-lint`  | Already covered by [`scan-lint.yaml`](scan-lint.yaml.md).                                         |

Every hook that walks files excludes `vendor/`. That is not tidiness: `addlicense` would stamp a header onto other people's code and the whitespace fixers would rewrite it, in both cases making the tree differ from what `go mod vendor` produces -- which [`check-go-mod.yaml`](check-go-mod.yaml.md) then fails on. If a hook starts touching `vendor/`, the fix is an exclude.
