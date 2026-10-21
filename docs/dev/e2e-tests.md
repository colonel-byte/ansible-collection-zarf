# End-to-end tests

There are two suites, split by whether they need a cluster. Both drive a real `ansible-playbook`, because every module here reaches zarf through an action plugin and only Ansible can run one. Neither runs under a bare `go test ./...`: both are behind mage targets, and every test skips with a reason when a prerequisite is missing rather than failing.

## `test/e2e/noncluster`

Needs `ansible-playbook`, and a `zarf` on `PATH` for the tests that build a real package. It covers the five read-only modules, the `state`, `package_inspect` and `prerequisites` roles, and every acting module's refusals and check-mode answers.

The one impure step in most read-only modules is running zarf, so the suite stands a stub in its place: `testdata/zarf-state-stub.sh` prints the base64 of a recorded `zarf-state` Secret, and `zarf-package-list-stub.sh` prints a recorded package list. The playbook passes the stub as the module's `zarf_binary`.

`testdata/zarf-cluster-stub.sh` is the exception that reads its arguments. `zarf_cluster_info` asks the cluster four questions -- the Kubernetes version, the CRDs, the StorageClasses and the Deployments -- and a stub that printed one document whatever it was asked would show that the module can parse a document, not that it asked four questions and kept the answers apart. It fails loudly on a command it does not recognise, for the same reason: a silent empty answer is a cluster that looks empty, and an empty cluster satisfies no prerequisite.

The tests that need a real zarf build a package rather than committing one. `testdata/zarf-inspect-fixture` is the smallest package that builds -- a files-only component needs no registry, no cluster and no network -- and it carries a `documentation:` mapping with a `PackagePrerequisites` document and a Markdown file, which is the fixture for `zarf_package_inspect`, `zarf_package_documentation` and the `prerequisites` role. A `.tar.zst` is a binary blob nobody can review; the document in that directory is what the tests are actually about.

`testdata/acting-modules.yml` is the part that needs no stub, because the modules it drives are the wrapper binary with zarf inside it. It runs all seven acting modules through the paths that reach no cluster: what each one refuses before it runs anything -- a pull given a local path, a mirror given both `images` and `repos`, a credential given for the wrong service, a kubectl command that does not exist -- and what each one does in check mode. `zarf_kubectl` is the exception that runs for real: `kubectl version --client` reads nothing but the binary it is in, so the module whose answer is a vendored tool's output is covered end to end here rather than only against a cluster.

That playbook is also where `argv[0]` dispatch is exercised through Ansible for every module at once: each task names a different `zarf_<module>` symlink, so a dispatch entry that went missing fails as the module for it. The suite builds the wrapper, and the test skips with a reason when it is not there, so `go test` over the package still works without the mage target.

The prerequisites test is the one combination worth knowing: the cluster is the stub, and the stub hands `package inspect documentation` to the real zarf. The role takes one zarf binary for both the extraction and the cluster reads, so it cannot be given a real one for half of them -- and faking the extraction would leave the role's own extraction branch untested, which is the branch an operator uses.

```sh
go run ./magefiles/core test:endToEndNonCluster
```

Two things about these tests are deliberate rather than incidental, and both are easy to undo by accident:

- **They count the assertions that reported passing.** A renamed fact or a skipped block leaves every remaining assertion passing, and `failed=0` would call that a success. The expected count is read out of the playbook rather than written in the Go file, so adding an assertion does not quietly stop the count from meaning anything.
- **They read the playbook, the fixture and the plugin before running anything.** Go's test cache keys on the files the test process opens, and everything these tests exercise is opened by the `ansible-playbook` child. Without those reads, editing a plugin leaves a passing result cached -- which is the shape of mistake that gets a credential leak marked as fixed.

## `test/e2e/cluster`

Needs `k3d`, a `zarf` on `PATH`, `ansible-playbook`, Docker, and the network to pull three packages on the first run. It builds the module wrapper first, stands up two k3d clusters, and walks `zarf_init` and `zarf_package_deploy` across each. The other five acting modules run against the cluster the first walk leaves behind.

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

### The acting modules the walks do not cover

`testdata/acting-modules.yml` runs after the bundle-order walk, on the cluster that walk initialised, and covers the half of those five modules that cannot be faked:

| Module                          | What it does there                                                                                                                                                          |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `zarf_kubectl`                  | Reads the cluster's nodes, then reads a namespace through `-l kubernetes.io/metadata.name=zarf` -- the selector is the regression, because `-l` is zarf's `--log-level` too |
| `zarf_package_pull`             | Fetches the storage provider package, then fetches it again into the same directory, which must report no change                                                            |
| `zarf_package_mirror_resources` | Pushes that package's images into the registry `zarf init` stood up; it reports no component list, and that is the assertion                                                |
| `zarf_update_creds`             | Asks for a rotation in check mode, which must reach the cluster and change nothing -- a real rotation cannot complete here, see below                                       |
| `zarf_package_remove`           | Takes the storage provider off the cluster, which the Go side confirms out of `zarf package list`                                                                           |

The order in that file is load-bearing and the Go side runs it last on that cluster: the removal takes away a package the read-only module tests expect to find, and the rotation rewrites credentials the cluster is using, so it goes last of all. Nothing may be added after it without a cluster of its own.

`zarf_package_remove` is measured by asking the cluster rather than by reading the module's report, and that is the point of running it here: it reports `changedSignal: unknown` because zarf names no component on the way out, so `zarf package list` no longer holding the package is the only evidence there is. The check-mode rotation is checked the same way -- the cluster's registry credentials are still there and are not the value the task offered.

The rotation is asked for in check mode because neither subcommand can complete against the cluster this suite builds, and both reasons are zarf's rather than the module's. `update-creds git` updates the gitea users `zarf-state` names, and the read-only user it names is never created by the uds-bundle's slim init: it fails with `failed to update git user "zarf-git-read-user": unexpected status code 404` and its rollback fails after it, reporting `the cluster may be in an inconsistent state`. `update-creds registry` fails with `error finding init package with registry service`, because the walk's second init records only the components that step deployed, so the deployed package zarf looks up for the registry is not the one that owns it. What a real cluster is left to prove is that the module reaches it and refuses to act; the rendering of all three subcommands is held against zarf's own commands by `TestUpdateCredsArgsAreKnownFlags`, and the refusals by the non-cluster suite.

Two findings from this suite are worth knowing before changing anything it covers:

- **`--verify` must be rendered as `--verify=always`.** It is a flag with a no-argument default, so pflag reads it the way it reads a boolean: the space form leaves `always` as a positional argument and zarf rejects the command for having one argument too many.
- **Local hooks do not reproduce the lint CI runs.** `.pre-commit-config.yaml` pins the same golangci-lint as the workflow, v2.15.0, and `prek run golangci-lint --all-files` still passed a discarded type-assertion `ok` that the `lint` job rejected under `errcheck`. Treat a green hook as necessary and not sufficient, and read the `lint` check on the pull request.
- **A mirror announces resources, not components.** `zarf_package_mirror_resources` was written expecting the component announcements a deploy makes, so that the progress reader could follow it. This suite is what showed that it cannot: what zarf logs on the way through a mirror is `mirroring images`, `pushing image` and `done pushing images`, never the component they came from. The module reports `changedSignal: unknown` with no component list for that reason, and the e2e assertion is that the list is empty -- a list appearing there would mean the progress reader had started matching something it should not.
- **The init package is tracked under its own metadata name.** `zarf package list` reports the uds-bundle's slim init as `init-slim`, not as the `init` the zarf-dev package records, so the fixtures name that.
- **A two-phase init needs `git-server` in the first phase.** Zarf records the git server's credentials in `zarf-state` only when the git-server component is in the set an init was asked for, and `zarf init` carries `--git-push-username=zarf-git-user` as a flag default whether or not anybody set it -- a viper default another command registers, which zarf's own `root.go` warns about leaking. A second init onto a cluster whose state holds no git server therefore reads as changing the git server and is refused with `cannot change git server information after initial init`. The walks deploy git-server in their seed step for that reason, which is a deviation from the bundle's own split and is commented as one in both playbooks. Pinning the credentials does not help: after a seed-only init there is nothing in state for them to match.
- **Registry proxy mode is what makes a local cluster usable.** Zarf's default nodeport mode pushes every image through a Kubernetes NodePort, and on k3d that push times out and retries for minutes. `registry_mode: proxy` puts a DaemonSet on a host port instead, and the same initialisation finishes in about a minute.
