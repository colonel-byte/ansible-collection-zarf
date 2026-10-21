# End-to-end tests

There are two suites, split by whether they need a cluster. Both drive a real `ansible-playbook`, because every module here is an action plugin and only Ansible can run one. Neither runs under a bare `go test ./...`: both are behind mage targets, and every test skips with a reason when a prerequisite is missing rather than failing.

## `test/e2e/noncluster`

Needs `ansible-playbook`, and a `zarf` on `PATH` for the tests that build a real package. It covers the five read-only modules and the `state`, `package_inspect` and `prerequisites` roles.

The one impure step in most read-only modules is running zarf, so the suite stands a stub in its place: `testdata/zarf-state-stub.sh` prints the base64 of a recorded `zarf-state` Secret, and `zarf-package-list-stub.sh` prints a recorded package list. The playbook passes the stub as the module's `zarf_binary`.

`testdata/zarf-cluster-stub.sh` is the exception that reads its arguments. `zarf_cluster_info` asks the cluster four questions -- the Kubernetes version, the CRDs, the StorageClasses and the Deployments -- and a stub that printed one document whatever it was asked would show that the module can parse a document, not that it asked four questions and kept the answers apart. It fails loudly on a command it does not recognise, for the same reason: a silent empty answer is a cluster that looks empty, and an empty cluster satisfies no prerequisite.

The tests that need a real zarf build a package rather than committing one. `testdata/zarf-inspect-fixture` is the smallest package that builds -- a files-only component needs no registry, no cluster and no network -- and it carries a `documentation:` mapping with a `PackagePrerequisites` document and a Markdown file, which is the fixture for `zarf_package_inspect`, `zarf_package_documentation` and the `prerequisites` role. A `.tar.zst` is a binary blob nobody can review; the document in that directory is what the tests are actually about.

The prerequisites test is the one combination worth knowing: the cluster is the stub, and the stub hands `package inspect documentation` to the real zarf. The role takes one zarf binary for both the extraction and the cluster reads, so it cannot be given a real one for half of them -- and faking the extraction would leave the role's own extraction branch untested, which is the branch an operator uses.

```sh
go run ./magefiles/core test:endToEndNonCluster
```

Two things about these tests are deliberate rather than incidental, and both are easy to undo by accident:

- **They count the assertions that reported passing.** A renamed fact or a skipped block leaves every remaining assertion passing, and `failed=0` would call that a success. The expected count is read out of the playbook rather than written in the Go file, so adding an assertion does not quietly stop the count from meaning anything.
- **They read the playbook, the fixture and the plugin before running anything.** Go's test cache keys on the files the test process opens, and everything these tests exercise is opened by the `ansible-playbook` child. Without those reads, editing a plugin leaves a passing result cached -- which is the shape of mistake that gets a credential leak marked as fixed.

## `test/e2e/cluster`

Needs `k3d`, a `zarf` on `PATH`, `ansible-playbook`, Docker, and the network to pull three packages on the first run. It builds the module wrapper first, stands up two k3d clusters, and walks `zarf_init` and `zarf_package_deploy` across each.

```sh
go run ./magefiles/core test:endToEndCluster
```

Both walks deploy the two packages of the [`init-local-path` uds-bundle](https://github.com/colonel-byte/uds-bundles/blob/main/upstream/init-local-path/uds-bundle.yaml), pinned in `main_test.go`. The init package is that bundle's *slim* init, whose five components are all optional, and that is what shapes the walk: each step names the components it wants, so the bundle's alternation of seed phase, storage provider, the rest of init, storage provider again is four steps rather than two. `bundle-order.yml` adds a fifth the bundle has not got -- the second init again, unchanged -- because a playbook that cannot be re-run is not a playbook. See [choice-zarf-ansible-module](../agent/choice-zarf-ansible-module.md) for the two tag traps that came with those packages.

Two clusters, because the walk differs by what storage the cluster has. `testdata/k3d-config.yaml` is [the config from colonel-byte/uds-bundles](https://github.com/colonel-byte/uds-bundles/blob/main/k3d-config.yaml) copied verbatim, so that what the modules initialise is the cluster shape that repository already deploys onto; `k3d-config-local-storage.yaml` is the same file with k3s local-storage left enabled, which is what lets a walk begin with `init` at all. Each fixture carries a header saying which walk it serves and why. The reasoning behind needing both is in [choice-zarf-ansible-module](../agent/choice-zarf-ansible-module.md).

| Variable                           | What it does                                                                                                                                                                                                                                                                                                                                    |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ZARF_ANSIBLE_E2E_INIT_PACKAGE`    | Names an init package to use instead of pulling one. Set it for an offline run, or to test a package built from a branch. It must be a slim init whose components are optional -- each walk step names the components it wants, so the standard zarf-dev init package, whose components are all required, deploys everything on the first step. |
| `ZARF_ANSIBLE_E2E_STORAGE_PACKAGE` | The same, for the storage provider package.                                                                                                                                                                                                                                                                                                     |
| `ZARF_ANSIBLE_E2E_KEEP`            | Leaves both clusters running afterwards, for looking at what the modules did.                                                                                                                                                                                                                                                                   |

Packages are cached under `build/tmp/zarf-e2e` between runs, so the downloads happen once. The init package is cached in a subdirectory of its own, which matters: `zarf package pull` names a `ZarfInitConfig` after its kind and version, so the slim init and the standard zarf-dev one both land as `zarf-init-<arch>-v0.88.0.tar.zst`, and a flat cache that had ever held the standard one would serve it to every walk. `test:cleanClusters` removes the clusters a run interrupted before its own cleanup left behind.

Two findings from this suite are worth knowing before changing anything it covers:

- **`--verify` must be rendered as `--verify=always`.** It is a flag with a no-argument default, so pflag reads it the way it reads a boolean: the space form leaves `always` as a positional argument and zarf rejects the command for having one argument too many.
- **Registry proxy mode is what makes a local cluster usable.** Zarf's default nodeport mode pushes every image through a Kubernetes NodePort, and on k3d that push times out and retries for minutes. `registry_mode: proxy` puts a DaemonSet on a host port instead, and the same initialisation finishes in about a minute.
