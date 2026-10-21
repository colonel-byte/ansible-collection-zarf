# Unit tests

Runs `test:unit` -- everything but the two suites that have runners of their own. Runs on every pull request and on the merge queue.

This is where the collection and the Go code are held against each other. The four contract tests in `internal/zarfmod` all run here, and all of them need nothing installed:

| Test                                                       | What it would catch                                                                                     |
| ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `TestActionPluginDocsMatchModuleParams`                    | A parameter added to a wrapper module's Go struct and not to its Python `DOCUMENTATION`, or the reverse |
| `TestInfoPluginDocsMatchRenderedFlags`                     | A read-only module's documented `cli_flag` values disagreeing with the flags it renders                 |
| `TestEveryActionPluginIsCovered`                           | A new action plugin that no test looks at, or a test list naming a plugin that no longer exists         |
| `TestInitArgsAreKnownFlags`, `TestDeployArgsAreKnownFlags` | A flag the module renders that the recorded zarf does not accept                                        |

The last pair parses what each module renders against zarf's own command tree, which is linked in, so pflag decides whether a flag exists and a zarf release that renamed one fails the job. There is nothing to refresh by hand; the zarf they check against is the `github.com/zarf-dev/zarf` version in `go.mod`.

The suites under `./test/...` need Ansible, k3d and Docker, so they are excluded here and run in [`e2e.yaml`](e2e.yaml.md) and [`e2e-cluster.yaml`](e2e-cluster.yaml.md) instead. See `Test.Unit` in `magefiles/test.go` for the package selection.

The step runs `go run -mod=vendor ./magefiles/core test:unit` rather than `mage`: same target, and it needs no mage binary on the runner. `GOPROXY=off` makes accidental network access fail fast rather than quietly succeed on a machine that happens to be online.
