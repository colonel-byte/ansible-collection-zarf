# colonel_byte.zarf

An Ansible collection that initialises an air-gapped Kubernetes cluster with [zarf](https://zarf.dev) and deploys zarf packages onto it, from a playbook.

It is a working proof of [ZEP-0072](https://github.com/zarf-dev/proposals/pull/73), which proposes native Ansible module support for zarf: the `zarf` binary dispatches on `argv[0]`, so a `zarf_init` symlink to `/usr/bin/zarf` is an Ansible module, with a thin Python action plugin alongside it. Everything the proposal describes is implemented here -- `argv[0]` dispatch, the WANT_JSON arguments file, the stdout guard, exit 0 with failure reported inside the JSON, the heartbeat status file and its polling monitor -- with exactly one deliberate divergence, recorded in [choice-zarf-ansible-module](docs/agent/choice-zarf-ansible-module.md): the modules are wrapper binaries that run the installed zarf as a child process, because a proof built outside the zarf tree cannot put dispatch inside the zarf binary.

The proof began in [colonel-byte/cargoship](https://github.com/colonel-byte/cargoship), beside the `colonel_byte.cargoship` collection whose shape it copies.

## Modules

| Module                 | What it does                                                                | How it is built                   |
| ---------------------- | --------------------------------------------------------------------------- | --------------------------------- |
| `zarf_init`            | `zarf init` against the cluster in a kubeconfig                             | Wrapper binary, from `cmd/init`   |
| `zarf_package_deploy`  | `zarf package deploy`                                                       | Wrapper binary, from `cmd/deploy` |
| `zarf_package_info`    | Lists the packages deployed to a cluster                                    | Action plugin                     |
| `zarf_package_inspect` | Reads and verifies a package without deploying it                           | Action plugin                     |
| `zarf_state_info`      | Reads the `zarf-state` Secret, withholding its eight credentials by default | Action plugin                     |

The first two converge a cluster and are the proof. The last three read one, are not part of the proof, and are plain action plugins with no binary behind them -- [choice-zarf-info-modules](docs/agent/choice-zarf-info-modules.md) records why, and why `zarf_state_info` redacts.

Each read-only module has a role apiece: `packages`, `package_inspect` and `state`.

## Package prerequisites

A package can state what it needs of a cluster in a machine-readable document carried inside the package itself, so that a deployment can refuse to proceed when the cluster does not satisfy it. That is the second thing in this repository: a `colonel-byte.dev/v1alpha1` `PackagePrerequisites` format and a [generated JSON Schema](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/schema/zarf-v1alpha1-package-prerequisites-schema.json) for it.

It exists for a failure that reports itself as a success. A workload package delivered as a `HelmRelease` needs a running `helm-controller` to do anything at all; deploy it onto a cluster without one and zarf applies the manifests, the API server accepts them, `zarf package deploy` reports a successful deployment, and nothing is ever reconciled.

The document travels inside the package, under zarf's own `documentation:` mapping, so a tarball carried into an air-gapped facility carries its own requirements. [The guide](docs/guides/package-prerequisites.md) is the format and how it is written.

## Installing

Build the two wrapper binaries and put them on the path of the node the play runs on:

```sh
go build -o zarf_init ./cmd/init
go build -o zarf_package_deploy ./cmd/deploy
sudo install -m 0755 zarf_init zarf_package_deploy /usr/local/bin/
```

Then make the collection visible to Ansible:

```sh
ln -s "$PWD/ansible/colonel_byte/zarf" ~/.ansible/collections/ansible_collections/colonel_byte/zarf
```

Zarf itself is a prerequisite: the modules run the installed `zarf`, and nothing is installed on the cluster's nodes. `ghcr.io/colonel-byte/zarf-ansible-modules` comes with all of it -- `ansible-core`, the collection, both module files and a `zarf` -- for a management node that would rather not install any of them:

<!-- x-release-please-start-version -->

```sh
docker run --rm -v "$PWD:/workspace" ghcr.io/colonel-byte/zarf-ansible-modules:v0.87.0 \
  ansible-playbook -i management.ini playbooks/zarf-bundle-order.yml
```

<!-- x-release-please-end-version -->

## Using it

```yaml
- name: Initialise the cluster
  hosts: localhost
  connection: local
  tasks:
    - name: Run zarf init
      colonel_byte.zarf.zarf_init:
        init_package: /srv/staging/zarf-init-amd64-v0.85.0.tar.zst
        kubeconfig: /etc/rancher/rke2/rke2.yaml
        registry_mode: proxy
        timeout: 15m
      delegate_to: localhost
      run_once: true
      no_log: true
```

A play running with `-v` shows each component as zarf reaches it:

```
[zarf] component 1: zarf-injector [running]
[zarf] component 1: zarf-injector [done]
[zarf] component 2: zarf-seed-registry [running]
```

Reading a cluster back, with the credentials withheld:

```yaml
    - name: Read the zarf state
      colonel_byte.zarf.zarf_state_info:
        kubeconfig: /etc/rancher/rke2/rke2.yaml
      register: zarf_state
```

More playbooks are in [`ansible/examples/playbooks/`](https://github.com/colonel-byte/ansible-collection-zarf/tree/main/ansible/examples/playbooks), and the guide at [`docs/guides/zarf-ansible-module.md`](docs/guides/zarf-ansible-module.md) walks the whole thing.

## Why there are two converging modules

A cluster whose distribution ships no storage provider cannot be initialised on its own: zarf's registry claims a PVC during the seed phase, and with no `StorageClass` the claim stays `Pending` until the Helm install times out. A storage provider package deployed with `connected: true` pushes no images, so it needs no registry, and so needs no initialised cluster -- which makes it the one thing that can come first.

That alternation is the shape of the [`init-local-path` uds-bundle](https://github.com/colonel-byte/uds-bundles/blob/main/upstream/init-local-path/uds-bundle.yaml), and the cluster suite walks it in both orders against two real k3d clusters.

## Building and testing

Targets run through [mage](https://magefile.org). `go run ./magefiles/core <target>` needs only the Go toolchain:

```sh
go run ./magefiles/core build:modules            # both module files into build/
go run ./magefiles/core test:unit                # everything but the e2e suites
go run ./magefiles/core test:endToEndNonCluster  # needs ansible-playbook; no cluster
go run ./magefiles/core test:endToEndCluster     # needs k3d, zarf, Docker and the network
go run ./magefiles/core generate:document        # regenerate docs/ansible, docs/golang, docs/index.md, docs/SUMMARY.md
go run ./magefiles/core generate:schema          # regenerate schema/ from the Go types in api/
```

The unit tests are where the collection and the Go code are held against each other: `TestActionPluginDocsMatchModuleParams` holds each action plugin's `DOCUMENTATION` block against the Go parameter struct, `TestInfoPluginDocsMatchRenderedFlags` does the same for the read-only modules against the flags their `command_parts` functions name, and `TestEveryActionPluginIsCovered` fails on a plugin that neither test looks at. `TestInitArgsAreKnownFlags` holds every flag the wrappers can render against a recorded list; refresh it with `generate:zarfFlags` when the zarf they run moves. On the prerequisites side, `TestCommittedSchemasAreCurrent` fails when the committed schema stops matching the Go type it is reflected from.

## Known limits of the proof

- The flag surface is a recorded list rather than a read of zarf's own command tree. It catches a flag zarf does not accept; it cannot catch a zarf release that renamed one without the list being refreshed.
- Component progress is read back out of zarf's log stream, so the modules render `--log-format json` by default and report a total component count of `0` unless `components` was named.
- `changedSignal` is `partial` on every successful run. Zarf reports which components it deployed, not whether a component found the cluster already in the state it wanted, so a wrapper reading that stream cannot honestly report `complete`.
- Check mode reports the converging tasks as skipped: neither `zarf init` nor `zarf package deploy` has a dry run. The read-only modules answer in check mode, because reading a cluster changes nothing.

All four collapse if dispatch moves inside zarf, which is the point of the proposal.

## License

Apache 2.0. See [LICENSE](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/LICENSE).
