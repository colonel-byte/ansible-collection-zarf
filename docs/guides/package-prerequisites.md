# Package prerequisites

A Zarf package can state what it needs of a cluster, in a document a machine can read, carried inside the package itself. This page is the format, where it lives, and what reads it.

## The failure this exists for

A bundle split into independently versioned packages has an order, and nothing enforces it. Deploy a GitOps workload package onto a cluster whose `helm-controller` is absent and every step succeeds: zarf applies the `OCIRepository` and the `HelmRelease`, the API server accepts both, and `zarf package deploy` reports a successful deployment. Nothing reconciles them. The only symptom is a `HelmRelease` that never becomes ready, in a cluster nobody is watching, hours later.

The same shape of failure covers a missing CRD, a cluster too old for a chart's API versions, and a `PersistentVolumeClaim` with no provisioner to bind it. In each case the package deployed and the workload did not arrive.

What makes it hard to prevent is that the requirement is known -- by whoever built the package -- and written somewhere nothing can parse. A README says "requires FluxCD 2.9 or newer". A package variable is called `flux.version` in one team's packages and `fluxcd` in another's. An orchestrator cannot check any of it, so it deploys and hopes.

## The document

The contract is a `colonel-byte.dev/v1alpha1` `PackagePrerequisites` document. It is YAML, it is validated by a [generated JSON Schema](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/schema/zarf-v1alpha1-package-prerequisites-schema.json), and the Go types it is generated from are in [`api/colonel-byte.dev/v1alpha1/prerequisites`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/api/colonel-byte.dev/v1alpha1/prerequisites/spec.go).

| Section               | Required | What it states                                                                     |
| --------------------- | -------- | ---------------------------------------------------------------------------------- |
| `apiVersion`          | yes      | Fixed to `colonel-byte.dev/v1alpha1`.                                              |
| `kind`                | yes      | Fixed to `PackagePrerequisites`.                                                   |
| `metadata`            | yes      | The package's name and version, and a sentence for a human.                        |
| `requires`            | yes      | What the cluster must already be.                                                  |
| `requires.kubernetes` | no       | A SemVer constraint on the API server's version.                                   |
| `requires.fluxcd`     | no       | A Flux version, the controller Deployments that must be available, and the CRDs.   |
| `requires.storage`    | no       | A default StorageClass, named StorageClasses, and the access modes claims ask for. |
| `requires.packages`   | no       | Other Zarf packages that must be deployed first, with SemVer constraints.          |
| `provides`            | no       | The capabilities, CRDs and namespaces this package contributes to the cluster.     |

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

So that a bundle can be ordered without deploying any of it. A planner that reads every package's document can satisfy each `requires` against another package's `provides` and sort the bundle; one that can only read a live cluster has to deploy to find out. The two reference packages are a pair for exactly this reason -- `app-fluxcd` provides the `fluxcd-engine` capability and the CRDs that `operator-external-secrets` requires.

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

## The reference packages

[`examples/packages`](https://github.com/colonel-byte/ansible-collection-zarf/tree/main/examples/packages) holds two packages that declare documents, as a provider and a consumer of the same capability.

| Package                     | Role     | What it shows                                                                                        |
| --------------------------- | -------- | ---------------------------------------------------------------------------------------------------- |
| `app-fluxcd`                | Provider | A `provides:` section, and the generated kustomization that makes its version readable off a cluster |
| `operator-external-secrets` | Consumer | A `requires:` section for a HelmRelease that needs a reconciler to exist                             |

Neither is built or deployed by anything here -- `zarf package create` on either reaches the network. They are to be read, linted and inspected:

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
