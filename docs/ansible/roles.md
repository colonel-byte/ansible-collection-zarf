# Roles

| Role                                         | What it does                                                                           |
| -------------------------------------------- | -------------------------------------------------------------------------------------- |
| [`deploy_packages`](role_deploy_packages.md) | Deploys a list of Zarf packages with component-level drift detection.                  |
| [`packages`](role_packages.md)               | Queries the cluster via `zarf package list` and sets the `zarf_packages` fact.         |
| [`package_inspect`](role_package_inspect.md) | Inspects a Zarf package definition and sets structured metadata facts.                 |
| [`prerequisites`](role_prerequisites.md)     | Checks a cluster against the prerequisites a package declares, and refuses to proceed. |
| [`state`](role_state.md)                     | Queries the cluster for the `zarf-state` Secret and sets the parsed `zarf_state` fact. |

## deploy_packages

`deploy_packages` deploys a sequence of Zarf packages while inspecting each package definition and checking the cluster's deployed components to avoid unnecessary deployments.

Skipping settles for a package that one item deploys. It does **not** settle for a package that several items deploy in phases -- the bundle's two-phase init, for instance -- because `zarf package list` reports only the most recent deploy's components, so each phase sees the other's and both re-deploy on every run. Re-deploying reconciles rather than fails, but it is never reported as unchanged. [choice-deploy-packages-drift-scope](../agent/choice-deploy-packages-drift-scope.md) has the reasoning and the alternatives that were declined.

An item is also only skipped when the cluster's version of the package matches the one the item carries, so an upgraded tarball whose component names did not change is not mistaken for what is already deployed.

Skipping is per component, and an item that names no `components` is deployed every run. That is a limit rather than an oversight: with no components named, zarf picks the set itself -- every non-optional component, plus any optional one marked `default`, with `required` and `optional` carrying inverted defaults between the v1alpha1 and v1beta1 package schemas -- and the role does not reimplement that choice to guess what it would be. Naming the components is what turns drift detection on.

The role sets `zarf_deploy_packages_changed` to whether any item reached zarf, and refreshes the `zarf_packages` fact once at the end when one did.

Set `zarf_result_dir` and each item's result is written to `<position>-<package file name>.json` inside it. The position leads the name because the package does not identify the item -- a bundle names the same init package twice, once for the seed phase and once for the rest -- and it puts a listing of the directory in deployment order. `log_dest` on an item overrides the whole path.

`registry_mode`, `connected`, `take_ownership` and `force_conflicts` are settings the role passes through rather than decides. Each has a role-level default (`zarf_registry_mode`, `zarf_connected`, `zarf_take_ownership`, `zarf_force_conflicts`) that a package item can override, and each is unset by default -- which leaves zarf's own behaviour alone. Turning `force_conflicts` on means a deploy takes fields another controller owns, so the role will not do it on an operator's behalf.

The boolean item keys below take `true` or `false`. Their role-level counterparts are typed `str` with an empty choice instead, because a role default needs a third state -- empty means "say nothing and leave zarf's own behaviour alone", which `false` cannot express.

Each dictionary entry in `zarf_packages_to_deploy` supports:

| Key               | Type            | Description                                                                    |
| ----------------- | --------------- | ------------------------------------------------------------------------------ |
| `path`            | `str`           | (Required) Path to the Zarf package file or OCI source.                        |
| `package_type`    | `str`           | Type of package: `init`, `package`, or `deploy` (default: `package`).          |
| `components`      | `list` of `str` | Components to deploy. Prefix with `-` to exclude (e.g. `-git-server`).         |
| `set_variables`   | `dict`          | Key-value variable overrides passed to Zarf templates (`--set`).               |
| `kubeconfig`      | `str`           | Kubeconfig path override for this package.                                     |
| `public_key`      | `str`           | Public key path override for signature verification.                           |
| `verify`          | `str`           | Verification mode override (`never`, `if-possible`, `always`).                 |
| `timeout`         | `str`           | Timeout override (e.g. `15m`, `1h`).                                           |
| `wrapper_binary`  | `str`           | Wrapper binary path override.                                                  |
| `registry_mode`   | `str`           | Registry mode for an `init` package: `nodeport`, `proxy` or `external`.        |
| `connected`       | `bool`          | Whether the cluster pulls the package's images itself. Non-init packages only. |
| `take_ownership`  | `bool`          | Whether zarf adopts cluster resources it did not create.                       |
| `force_conflicts` | `bool`          | Whether zarf overrides a server-side-apply conflict.                           |
| `log_dest`        | `str`           | Result file path, overriding the name built under `zarf_result_dir`.           |
| `delegate_to`     | `str`           | Target node override for executing inspection and deployment.                  |
| `zarf_binary`     | `str`           | Path to `zarf` binary override.                                                |

Example:

```yaml
- name: Deploy package stack
  ansible.builtin.include_role:
    name: colonel_byte.zarf.deploy_packages
  vars:
    zarf_kubeconfig: /etc/rancher/rke2/rke2.yaml
    zarf_packages_to_deploy:
      - path: "{{ zarf_package_dir }}/zarf-init-amd64-v0.88.0.tar.zst"
        package_type: init
        components:
          - k3s
      - path: "{{ zarf_package_dir }}/zarf-package-csi-rook-ceph-amd64-v1.21.0-upstream.tar.zst"
        components:
          - operator-images
          - cluster-images
          - operator-seed-chart
          - cluster-seed-chart
          - registry-bucket
        package_type: package
        set_variables: "{{ zarf_storage_set_variables }}"
```

## packages

`packages` queries installed Zarf packages and exposes them as a structured list under the `zarf_packages` fact:

```yaml
- name: List installed Zarf packages
  ansible.builtin.include_role:
    name: colonel_byte.zarf.packages
  vars:
    zarf_kubeconfig: /etc/rancher/rke2/rke2.yaml

- name: Report installed packages
  ansible.builtin.debug:
    msg: "{{ zarf_packages | map(attribute='package') | list }}"
```

## package_inspect

`package_inspect` parses a package tarball, OCI reference, or cluster package definition and sets facts.

Naming `zarf_public_key` requires a signature. That is the role being stricter than the module, which passes `verify` through and leaves zarf's own `if-possible` default alone -- and `if-possible` accepts an unsigned package whether or not a key was given. Set `zarf_verify` explicitly to override, including to `if-possible` to get zarf's default back.

```yaml
- name: Inspect storage package definition, rejecting it when unsigned
  ansible.builtin.include_role:
    name: colonel_byte.zarf.package_inspect
  vars:
    zarf_package: /srv/staging/zarf-package-csi-rook-ceph-amd64-v1.20.7-upstream.tar.zst
    zarf_public_key: /etc/zarf/colonel-byte-zarf-packages.pub

- name: Report package details
  ansible.builtin.debug:
    msg:
      name: "{{ zarf_package_name }}"
      version: "{{ zarf_package_version }}"
      flavor: "{{ zarf_package_flavor }}"
```

## prerequisites

`prerequisites` is the check that belongs in front of `zarf_package_deploy`. It reads the [`PackagePrerequisites`](../guides/package-prerequisites.md) document a package carries in its `documentation:` mapping, reads the cluster, and fails the play when the cluster does not satisfy it.

It exists for one failure in particular. A workload package delivered as a `HelmRelease` needs a running `helm-controller` to do anything at all; on a cluster without one, zarf writes the manifests, reports the deployment as a success, and nothing is ever reconciled. The only symptom is a `HelmRelease` that never becomes ready.

Three facts come out of it, and the third matters as much as the other two. `zarf_prerequisites_checked` is what was checked and held, `zarf_prerequisites_unmet` is what did not, and `zarf_prerequisites_skipped` is what could not be checked at all -- a compound version range the role will not half-evaluate, or an `accessModes` requirement that cannot be verified without creating a PersistentVolumeClaim. A check that cannot be made is named rather than quietly passed.

Every unmet prerequisite is reported at once, from one assert at the end, so an operator who has to install Flux *and* a storage provider is told both rather than told one, fixing it, and being told the next.

```yaml
- name: Refuse to deploy onto a cluster that cannot run this package
  ansible.builtin.include_role:
    name: colonel_byte.zarf.prerequisites
  vars:
    zarf_package: /srv/staging/zarf-package-operator-external-secrets-amd64-2.12.0.tar.zst
    zarf_public_key: /etc/zarf/colonel-byte-zarf-packages.pub
    zarf_kubeconfig: /etc/rancher/rke2/rke2.yaml

- name: Report what could not be checked
  ansible.builtin.debug:
    msg: "{{ zarf_prerequisites_skipped }}"
  when: zarf_prerequisites_skipped | length > 0
```

A package that carries no document is not a failure: nothing is checked, `zarf_prerequisites_checked` is empty, and the play continues. Set `zarf_prerequisites_enforce: false` to report rather than enforce, which is what a survey across a fleet of clusters wants and what a deployment does not. [`ansible/examples/playbooks/zarf-prerequisites.yml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/ansible/examples/playbooks/zarf-prerequisites.yml) deploys a two-package bundle this way.

## state

`state` is the convenient way to retrieve and parse Zarf's cluster state. It runs `zarf tools kubectl get secret zarf-state -n zarf -o jsonpath='{.data.state}'`, decodes the payload, and sets the resulting dictionary as the `zarf_state` fact.

The credentials in that Secret -- the registry push, pull and seed secrets, the git server passwords, the artifact server token, and the agent webhook's TLS private key -- are withheld. The paths withheld are set as `zarf_state_redacted`, and `zarf_include_credentials: true` returns them instead. The role defaults `no_log` on both of its tasks, which is also why reaching for the role beats calling the module directly here: `set_fact` prints what the module censored, so the module alone cannot keep a credential out of a `-v` transcript. See [choice-zarf-info-modules](../agent/choice-zarf-info-modules.md).

```yaml
- name: Load cluster zarf state
  ansible.builtin.include_role:
    name: colonel_byte.zarf.state
  vars:
    zarf_kubeconfig: /etc/rancher/rke2/rke2.yaml

- name: Inspect cluster registry mode
  ansible.builtin.debug:
    msg: "Cluster registry mode is {{ zarf_state.registryInfo.registryMode }}"

- name: Report which paths were withheld
  ansible.builtin.debug:
    var: zarf_state_redacted
```
