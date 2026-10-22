# Package prerequisites

A Zarf package can state what it needs of a cluster, in a document a machine can read, carried inside the package itself. This page is the format, where it lives, and what reads it.

## The failure this exists for

A bundle split into independently versioned packages has an order, and nothing enforces it. Deploy a GitOps workload package onto a cluster whose `helm-controller` is absent and every step succeeds: zarf applies the `OCIRepository` and the `HelmRelease`, the API server accepts both, and `zarf package deploy` reports a successful deployment. Nothing reconciles them. The only symptom is a `HelmRelease` that never becomes ready, in a cluster nobody is watching, hours later.

The same shape of failure covers a missing CRD, a cluster too old for a chart's API versions, and a `PersistentVolumeClaim` with no provisioner to bind it. In each case the package deployed and the workload did not arrive.

What makes it hard to prevent is that the requirement is known -- by whoever built the package -- and written somewhere nothing can parse. A README says "requires FluxCD 2.9 or newer". A package variable is called `flux.version` in one team's packages and `fluxcd` in another's. An orchestrator cannot check any of it, so it deploys and hopes.

## The document

The contract is a `colonel-byte.dev/v1alpha1` `PackagePrerequisites` document. It is YAML, it is validated by a [generated JSON Schema](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/schema/zarf-v1alpha1-package-prerequisites-schema.json), and the Go types it is generated from are in [`api/colonel-byte.dev/v1alpha1/prerequisites`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/api/colonel-byte.dev/v1alpha1/prerequisites/spec.go).

| Section                | Required | What it states                                                                                                                                                      |
| ---------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `apiVersion`           | yes      | Fixed to `colonel-byte.dev/v1alpha1`.                                                                                                                               |
| `kind`                 | yes      | Fixed to `PackagePrerequisites`.                                                                                                                                    |
| `metadata`             | yes      | The package's name and version, and a sentence for a human.                                                                                                         |
| `requires`             | yes      | What the cluster must already be.                                                                                                                                   |
| `requires.kubernetes`  | no       | A SemVer constraint on the API server's version.                                                                                                                    |
| `requires.fluxcd`      | no       | A SemVer constraint on the installed Flux. Mutually exclusive with `requires.argocd`.                                                                               |
| `requires.argocd`      | no       | A SemVer constraint on the installed Argo CD. Mutually exclusive with `requires.fluxcd`.                                                                            |
| `requires.controllers` | no       | The workloads that must be present and available, by name, namespace and kind, each with an optional SemVer constraint on its own version.                          |
| `requires.crds`        | no       | The CustomResourceDefinitions that must be established.                                                                                                             |
| `requires.storage`     | no       | A default StorageClass, named StorageClasses, and the access modes claims ask for.                                                                                  |
| `requires.packages`    | no       | Other Zarf packages that must be deployed first, with SemVer constraints. An entry names one package with `name`, or every name it is published under with `names`. |
| `provides`             | no       | The capabilities, CRDs and namespaces this package contributes to the cluster.                                                                                      |

`requires` is required and `provides` is not, which is the asymmetry the format is for: a package that states nothing it requires has no reason to ship the document, while a leaf workload that provides nothing downstream is the common case.

```yaml
# yaml-language-server: $schema=../../schema/zarf-v1alpha1-package-prerequisites-schema.json
---
apiVersion: colonel-byte.dev/v1alpha1
kind: PackagePrerequisites
metadata:
  package: operator-external-secrets
  version: 2.12.0
  description: Prerequisites for the External Secrets Operator and its webhook

requires:
  kubernetes:
    version: ">=1.30.0"
  fluxcd:
    version: ">=2.9.0"
  controllers:
    - name: source-controller
      namespace: flux-system
      minAvailable: 1
    - name: helm-controller
      namespace: flux-system
      minAvailable: 1
  crds:
    - helmreleases.helm.toolkit.fluxcd.io
    - ocirepositories.source.toolkit.fluxcd.io
  packages:
    - name: app-fluxcd
      version: ">=2.9.0"

provides:
  capabilities:
    - name: external-secrets-engine
      version: "2.12.0"
  crds:
    - externalsecrets.external-secrets.io
    - secretstores.external-secrets.io
    - clustersecretstores.external-secrets.io
  namespaces:
    - external-secrets
```

The `$schema` line is what makes an editor complete and validate the file while it is being written. Point it at the schema wherever it resolves for you: a relative path in a checkout, a URL otherwise.

### Why the Flux section is three things

A version alone catches the least interesting failure. A cluster can hold the right Flux release with its controllers scaled to zero, or hold the controllers and not the CRDs a package's own `HelmRelease` needs. Each of those is the silent case, so each is stated separately.

`minAvailable` is about availability rather than readiness. A replica that is ready and not yet available has passed its probes and not satisfied `minReadySeconds`, and a controller in that state is in the middle of its own rollout.

### Why `provides` is declared rather than discovered

So that a bundle can be ordered without deploying any of it. A planner that reads every package's document can satisfy each `requires` against another package's `provides` and sort the bundle; one that can only read a live cluster has to deploy to find out. The reference packages are shaped for exactly this reason -- `app-fluxcd` and `app-argocd` both provide the `gitops-engine` capability, and each also provides an engine-specific name for a package that genuinely needs one engine's own API. Note what `requires` cannot yet do with that: there is no `requires.capabilities`, so a consumer names the package it needs by name in `requires.packages`. The capability is declared for a planner reading a bundle, and nothing in this repository matches one.

## Where it lives: zarf's `documentation:` mapping

The document goes inside the package, under zarf's own top-level `documentation:` key. That key is a mapping of a name to a path in the package's source tree, and zarf packages every file it names into a documentation layer of the built package.

```yaml
kind: ZarfPackageConfig
metadata:
  name: operator-external-secrets
  version: 2.12.0
  annotations:
    colonel-byte.dev/requires-fluxcd: ">=2.9.0"
    colonel-byte.dev/delivery-mode: gitops-helmrelease

documentation:
  prerequisites: docs/prerequisites.yaml
  schema: schema/zarf-v1alpha1-package-prerequisites-schema.json
  readme: README.md
```

That is the whole reason to use `documentation:` rather than a file in the repository: the document travels with the package. A tarball copied onto a USB stick and carried into an air-gapped facility carries its own requirements, and `zarf package inspect documentation` extracts them without deploying anything or reaching a network.

The annotations beside it are a deliberate duplicate of two facts from the document. They are readable from `zarf package inspect definition` alone, which is one command and no extraction, so a consumer that only needs to order a bundle can read them. The document is for the consumer that is going to check a cluster against it, and it is the source of truth.

## Reading it

Three things in this collection read a package's documentation, and they do different amounts of work.

| What                                                                       | Reads                                                                          |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [`zarf_package_inspect`](../ansible/module_package_inspect.md)             | The `documentation:` mapping itself, as `package_documentation`: keys to paths |
| [`zarf_package_documentation`](../ansible/module_package_documentation.md) | The files, extracted and parsed, including the prerequisites document          |
| [`prerequisites`](../ansible/role_prerequisites.md)                        | The document, plus the cluster, and fails the play                             |

The mapping alone tells you the package declares a prerequisites document and where it was in the source tree. It does not get you the contents -- the file is inside the tarball, and `zarf package inspect documentation` is what takes it out.

```yaml
- name: Report the documentation a package carries
  colonel_byte.zarf.zarf_package_inspect:
    package: /srv/staging/zarf-package-operator-external-secrets-amd64-2.12.0.tar.zst
  delegate_to: localhost
  run_once: true
  register: pkg

- name: Read the prerequisites document out of the package
  colonel_byte.zarf.zarf_package_documentation:
    package: /srv/staging/zarf-package-operator-external-secrets-amd64-2.12.0.tar.zst
    keys:
      - prerequisites
  delegate_to: localhost
  run_once: true
  register: docs

- name: Report the Flux version it needs
  ansible.builtin.debug:
    msg: "{{ docs.prerequisites.requires.fluxcd.version }}"
  when: docs.prerequisites is defined
```

`prerequisites` is returned only when the extracted file declares the right `apiVersion` and `kind`. A package is free to use that documentation key for a prose file, and returning that as a prerequisites document would hand the role a mapping with no `requires` for the most boring possible reason.

## Enforcing it

The [`prerequisites`](../ansible/role_prerequisites.md) role is what a deployment actually calls. It extracts the document, reads the cluster with [`zarf_cluster_info`](../ansible/module_cluster_info.md), and fails the play before `zarf_package_deploy` runs.

```yaml
- name: Check the cluster against what this package requires
  ansible.builtin.include_role:
    name: colonel_byte.zarf.prerequisites
  vars:
    zarf_package: "{{ package }}"
    zarf_public_key: /etc/zarf/colonel-byte-zarf-packages.pub
    zarf_kubeconfig: /etc/rancher/rke2/rke2.yaml
```

The cluster is read through `zarf tools kubectl`, which is a whole vendored kubectl, so the role needs no `kubernetes.core` -- the management node is typically air-gapped, and a second Galaxy collection to stage onto it is a cost every consumer would pay.

Check each package as its turn comes rather than checking the whole bundle up front. The first package in a bundle is often what satisfies the second one's prerequisites, so a check of everything at the start refuses a deployment that is about to be correct. [`ansible/examples/playbooks/zarf-prerequisites.yml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/ansible/examples/playbooks/zarf-prerequisites.yml) loops a bundle that way.

### What it cannot check, and says so

Two things come back in `zarf_prerequisites_skipped` rather than being counted as satisfied.

**A compound version range.** The role reads one comparator and one version, which covers `>=2.9.0`. Evaluating `>=1.30.0 <1.34.0` needs a semver library and Jinja is not one, and a half-parsed range would silently check only its first half. The format allows the range; the role names it as unevaluated.

**`requires.storage.accessModes`.** What a provisioner will actually bind is not discoverable without creating a `PersistentVolumeClaim`, which is not a thing a read-only check does.

Print that list even on a successful run. A run that reported only its successes would read as though everything in the document had been verified.

### Where the engine version comes from

The version under `requires.fluxcd` or `requires.argocd` is checked against the `app.kubernetes.io/version` label on the engine's own workloads. For Flux that is the flux2 release the manifests were built from -- not the controllers' own versions, which are a different set of numbers entirely: a document naming `>=2.9.0` is not talking about `source-controller` v1.9.5.

Neither engine's upstream manifests carry that label. `flux install` adds it from its own build, and Argo CD's `install.yaml` has none at all -- so a package that vendors either has to add it. [`app-fluxcd`](https://github.com/colonel-byte/ansible-collection-zarf/tree/main/examples/packages/app-fluxcd) and [`app-argocd`](https://github.com/colonel-byte/ansible-collection-zarf/tree/main/examples/packages/app-argocd) both do it in a generated kustomization, which is most of why those files are generated. A cluster whose engine carries no version label has the constraint reported as unchecked -- honestly, and uselessly.

### A version without an engine

`requires.controllers[].version` is a SemVer range on that workload's own `app.kubernetes.io/version` label, and it is how a package requires a version of something that is not a GitOps engine.

```yaml
requires:
  controllers:
    - name: cert-manager
      namespace: cert-manager
      version: ">=1.14.0"
```

No `fluxcd` or `argocd` key anywhere: the package needs that workload at that version and has no opinion about what reconciles the cluster. Without it, the only version a document could put on a workload was the engine's, so a package needing cert-manager v1.14 had to require an engine it does not use purely to get a version field -- a prerequisite that refuses clusters for no reason.

It is the same label and the same comparison the engine sections use, so the same two caveats apply: one comparator and one version, and a workload carrying no version label has the constraint reported as unchecked rather than satisfied. Most charts following the Kubernetes recommended labels set it; a workload nobody stamped does not.

### Which engine, and why the format asks

An entry under `requires.packages` names its prerequisite with `name`, or with `names` when the package is published under more than one -- the upstream init reports itself as `init` and a rebuild of it as `init-slim`, and a package needing an initialised cluster needs either. Several names rather than several entries, because the version constraint belongs to the requirement and not to each name: restating it per name is a constraint that gets bumped in one place and not the other. An entry carries one of the two keys and never both, which the schema refuses and the role refuses again for a document handed to it directly -- the role reads `names` in preference to `name`, so an entry carrying both would say one thing and be checked as another.

```yaml
requires:
  packages:
    - names:
        - init
        - init-slim
      version: ">=0.85.0"
```

The engine is named by the key -- `requires.fluxcd` or `requires.argocd` -- and the two are mutually exclusive, which the generated schema states as well as the role. The key names the engine rather than a field inside it because the two do not number together: Flux is on 2.x while Argo CD is on 3.x, so a version sitting beside a separate `engine` field could disagree with it, and the dangerous direction of that disagreement is silent. A document meaning "Flux >= 2.9" that named argocd by mistake is satisfied by Argo CD 3.5, because 3.5 >= 2.9 -- the constraint passes against an engine it was never about. Naming the engine with the key leaves nothing for the version to contradict.

Only two things about checking an engine are engine-specific at all.

The first is the namespace a controller defaults to: `flux-system` for Flux, `argocd` for Argo CD. A document whose controllers name their namespaces never needs the engine for this; one that leaves them off does.

The second is the kind. Flux's controllers are all `Deployment`s. Argo CD runs `argocd-application-controller` as a **`StatefulSet`** and the rest of itself as `Deployment`s, which is why `requires.controllers[].kind` exists at all -- a check that assumed `Deployment` would report the component doing all of Argo CD's reconciling as absent from a perfectly healthy cluster. It defaults to `Deployment`, so only the Argo CD controller has to say it.

```yaml
requires:
  argocd:
    version: ">=3.5.0"
  controllers:
    # No namespace on either: the argocd key makes both default to the argocd namespace.
    - name: argocd-application-controller
      kind: StatefulSet
    - name: argocd-server
  crds:
    - applications.argoproj.io
    - appprojects.argoproj.io
```

Everything else -- "these workloads must be available", "these CRDs must be established" -- is the same question whatever reconciles the cluster, which is why `requires.controllers` and `requires.crds` sit beside the engine section rather than inside it.

## The reference packages

[`examples/packages`](https://github.com/colonel-byte/ansible-collection-zarf/tree/main/examples/packages) holds three packages that declare documents: two providers of the same capability, and a consumer of it.

| Package                     | Role     | What it shows                                                                                                                                                      |
| --------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `app-fluxcd`                | Provider | A `provides:` section, and the generated kustomization that makes its version readable off a cluster                                                               |
| `app-argocd`                | Provider | The same capability from the other engine: a `StatefulSet` controller, its own namespace, and a vendoring source that is a tagged tree rather than a release asset |
| `operator-external-secrets` | Consumer | A `requires:` section for a HelmRelease that needs a reconciler to exist                                                                                           |

None of them is built or deployed by anything here -- `zarf package create` on either reaches the network. They are to be read, linted and inspected:

```sh
go run ./magefiles/core examples:lint
```

`app-fluxcd` is packaged from the upstream flux2 release manifests, vendored from a pinned tag by `examples:vendor` and with its image lists discovered by `examples:images`. See [the mage reference](../dev/mage.md) for those targets, and `magefiles/pkg/pkgvendor` for what they do.

## When a document changes

The schema is generated from the Go types, so changing the format is a change to [`spec.go`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/api/colonel-byte.dev/v1alpha1/prerequisites/spec.go) followed by:

```sh
go run ./magefiles/core generate:schema
```

The descriptions in the schema are the doc comments on those types, which is why they are written as prose for a package author rather than as notes for a Go reader: an editor with YAML schema support shows them while the document is being written.

Two tests hold the pieces together. `TestCommittedSchemasAreCurrent` fails when the committed schema stops matching the types -- a stale schema does not fail, it validates the wrong document, which is worse than not validating one. `TestExampleDocumentsMatchTheTypes` decodes each reference package's document into the types with unknown fields rejected, so an example written against an older shape of the format fails rather than quietly becoming the thing readers copy.

The version is `v1alpha1` and means it: the format is a proof alongside [ZEP-0072](https://github.com/zarf-dev/proposals/pull/73), not a published API.
