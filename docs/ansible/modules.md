# Modules

Twelve modules ship. Seven of them act, and five read a cluster.

`zarf_init` and `zarf_package_deploy` are the converging pair: they are the proof of [ZEP-0072](https://github.com/zarf-dev/proposals/pull/73), each has its parameters and command line in Go, and [choice-zarf-ansible-module](../agent/choice-zarf-ansible-module.md) records how. Package creation is not among them -- it builds an artifact from a definition in a repository, which belongs in a pipeline rather than in a convergence run -- and neither is package removal, which nothing here has needed yet.

`zarf_package_remove` is the other direction of a deploy: it takes a deployed package, or some of its components, off a cluster. ZEP-0072 names it, and a playbook that can only add is not reconciling anything. `zarf_package_pull` acts on a directory rather than a cluster -- it fetches a package to disk, which is how a package reaches an air-gapped management node before a deploy reads it -- and it is the one module here whose `changed` is an observation: it reports the package files that were not in the output directory before the run.

`zarf_package_mirror_resources` pushes a package's images and git repositories into a registry and a git server without deploying anything, which is the step a `connected: true` deploy needs to have happened first -- and it follows the components the way a deploy does, so it reports the same `partial` signal. `zarf_update_creds` rotates the credentials an initialised cluster holds: it is the write half of `zarf_state_info`, and it is one zarf subcommand per service, so a task names the service it is rotating.

`zarf_kubectl` is the last that acts, and it is not a convergence module: it passes an argument vector to the kubectl zarf vendors and hands back what it printed. It exists because that kubectl is already inside the module file, so a management node reading or patching one resource on an air-gapped cluster needs no kubectl of its own. For declarative resource management, `kubernetes.core.k8s` is the module to reach for.

All seven are the same binary, invoked under seven names. It dispatches on `argv[0]`, which is what ZEP-0072 proposes zarf itself do, so each module file under `plugins/modules/` is a symlink rather than a file of its own.

`zarf_cluster_info`, `zarf_package_documentation`, `zarf_package_info`, `zarf_package_inspect` and `zarf_state_info` read. They change nothing, they answer during a check-mode play rather than skipping, and they are plain action plugins with no wrapper binary behind them, for the reasons in [choice-zarf-info-modules](../agent/choice-zarf-info-modules.md) and -- for the two added for the prerequisites format, which are the first to push past the threshold that record names -- in [choice-package-documentation-module](../agent/choice-package-documentation-module.md).

| Module                                                                | What it does                                                                                    |
| --------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| [`zarf_init`](module_init.md)                                         | Initialises a cluster: the injector, the seed registry, the registry, the agent.                |
| [`zarf_package_deploy`](module_package_deploy.md)                     | Puts a package on a cluster, from a path, an `oci://` reference, or an `https://` URL.          |
| [`zarf_package_pull`](module_package_pull.md)                         | Fetches a package to disk from an `oci://` reference or an `https://` URL.                      |
| [`zarf_package_remove`](module_package_remove.md)                     | Removes a deployed package, or some of its components, from a cluster.                          |
| [`zarf_package_mirror_resources`](module_package_mirror_resources.md) | Pushes a package's images and repositories into a registry and git server.                      |
| [`zarf_update_creds`](module_update_creds.md)                         | Rotates the registry, git or agent credentials an initialised cluster holds.                    |
| [`zarf_kubectl`](module_kubectl.md)                                   | Runs a kubectl command through the kubectl zarf carries, and returns its output.                |
| [`zarf_cluster_info`](module_cluster_info.md)                         | Reports the cluster's Kubernetes version, CRDs, StorageClasses and Deployment readiness.        |
| [`zarf_package_documentation`](module_package_documentation.md)       | Extracts the files a package carries in its `documentation:` mapping, and parses the YAML ones. |
| [`zarf_package_info`](module_package_info.md)                         | Lists the packages deployed to a cluster, via `zarf package list`.                              |
| [`zarf_package_inspect`](module_package_inspect.md)                   | Reads the `zarf.yaml` of a package tarball, an `oci://` reference, or a deployed package.       |
| [`zarf_state_info`](module_state_info.md)                             | Reads the cluster's `zarf-state` Secret, withholding the credentials in it.                     |

## The surfaces differ because the commands do

`zarf init` configures the registry and git server the cluster will use, so it takes the registry and git parameters and no package. A removal takes seven flags where a deploy takes twenty-eight, because there is no signature to verify and nothing to push once the thing is being taken away; a pull takes neither a kubeconfig nor `--confirm`, because it reaches no cluster and the command has no such flag; and `zarf_update_creds` takes only the credentials of the service it names, because zarf has one subcommand per service and each accepts only its own -- a credential given for another service is an error naming it. `zarf package deploy` names a package outright and takes none of that configuration, but takes `connected`, `namespace` and `shasum`, which only a deploy has. A parameter offered to the wrong module is an error naming it, rather than a flag quietly dropped.

Each module's page lists its full parameter set and the flag each parameter renders. A parameter that renders no flag is listed as `None`: it is either the command's positional argument, something passed to zarf in its environment, or something the action plugin consumes before the command line is built.

## What every task needs

A parameter left unset is left unset: the module passes no flag for it, so whatever zarf's own configuration file sets still applies. That is why `plain_http: false` and omitting `plain_http` are different.

Set `run_once: true` and `delegate_to` on every task. One `zarf init` initialises the whole cluster, one deploy puts the package on all of it, and one read answers for all of it, so a play over the cluster's own inventory would otherwise do the work once per host. Set `no_log: true` on any task carrying a credential: a binary module has no per-parameter `no_log`, so the task-level setting is what suppresses the arguments and the result.

Credentials given as parameters are rendered as flags, which puts them in the process table for the life of the run. `zarf_config` names a zarf configuration file instead, which zarf reads them from.

## What the result says

The two converging modules report `changed: true` with `changedSignal: partial` on every successful run, and name every component they deployed in `changedUndeclared`. Zarf reports which components ran and not whether any of them found the cluster already as it wanted, so `partial` is the honest answer rather than an unfinished one. Check mode reports the task as skipped, because neither command has a dry run.

`zarf_package_mirror_resources` reports `changed: true` with `changedSignal: partial` and the components it walked, as the converging pair do and for the same reason. `zarf_update_creds` reports `changed: true` with `changedSignal: unknown`: zarf says it updated the service, not whether it already held what was given, so a credential set twice is reported as changed both times.

`zarf_package_remove` reports `changed: true` with `changedSignal: unknown`: zarf logs a removal as one line and names no component on the way, so there is nothing that distinguishes a package that was there from one that was already gone -- a removal that finds nothing fails in zarf. `zarf_package_pull` is the exception in the collection: it reports `changedSignal: complete`, because the package file either arrived in the output directory or it did not, and the filenames are in `zarf.packagesPulled`.

`zarf_kubectl` reports `changed: true` with `changedSignal: unknown`, because the argument vector is kubectl's and nothing in the module reads it: a successful run says the command worked and nothing about whether the cluster moved. A read wants `changed_when: false` on the task. Check mode skips it for the same reason -- nothing there can tell a `get` from an `apply`.

The five read-only modules report `changed: false`, which is not an approximation, and they run in check mode rather than skipping: answering a question during a run an operator asked to be told about is the honest thing to do.

`zarf_state_info` withholds what it reads. The `zarf-state` Secret carries the registry, git and artifact server credentials and the agent webhook's TLS private key, so the returned `state` has them removed and `redacted` names the paths that went. `include_credentials: true` returns them and censors the task's own output in the same breath; the task that reads the result afterwards needs its own `no_log`, because `set_fact` prints what the module hid.

## The two that exist for the prerequisites format

`zarf_package_documentation` and `zarf_cluster_info` are the two halves of a check that runs before a deployment. A package states what it needs of a cluster in a [`PackagePrerequisites`](../guides/package-prerequisites.md) document carried in its own `documentation:` mapping; the first extracts that document, the second reports the cluster facts it is checked against, and the [`prerequisites`](role_prerequisites.md) role is what holds the two together and fails the play. Reach for the role rather than the modules unless you are checking something the format does not describe.

`zarf_cluster_info` reads the cluster through `zarf tools kubectl`, which is a whole vendored kubectl, rather than through `kubernetes.core`. That keeps this collection's only requirement the zarf binary it already requires: the management node a playbook runs from is typically air-gapped, and a second Galaxy collection to stage onto it is a cost every consumer of this one would pay. `zarf_state_info` reads the `zarf-state` Secret the same way, and for the same reason.

The [module guide](../guides/zarf-ansible-module.md) covers installing the modules, the ordering a cluster with no storage provider needs, the live progress display, and the shape of the result.
