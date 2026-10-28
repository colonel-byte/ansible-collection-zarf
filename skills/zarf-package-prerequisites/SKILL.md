---
name: zarf-package-prerequisites
description: >
  Writes the document a Zarf package carries to state what it needs of a cluster before it
  deploys -- a colonel-byte.dev/v1alpha1 PackagePrerequisites document, authored alongside
  the package on a connected machine. Use when writing or fixing a docs/prerequisites.yaml,
  or when a package must require a Kubernetes version, a GitOps engine (Flux or Argo CD),
  CRDs, a StorageClass or another deployed package. Not for debugging a deployment that
  already happened: this is how a package states its requirements, not how a cluster is
  diagnosed. For the zarf.yaml itself, use zarf-package.
---

# Package prerequisites

A Zarf package deploys onto a cluster that cannot run it and reports success. Zarf applies the `HelmRelease`, the API server accepts it, nothing reconciles it, and the only symptom is a resource that never becomes ready in a cluster nobody is watching. A `PackagePrerequisites` document is the package stating its requirements where a machine can check them.

**Scope: authoring, beside the package.** This page is about writing that document while you have the package's own charts and manifests in front of you, which is the only time its contents can be derived rather than guessed. Diagnosing a deployment that has already gone wrong is out of scope -- a document is what stops the next one, not a tool for reading the last one. The role below is here so that an author knows what consumes the document and writes a better one, not as a debugging guide.

The format's full reference -- every section and why each is shaped the way it is -- is [`docs/guides/package-prerequisites.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/docs/guides/package-prerequisites.md), and the three [reference packages](https://github.com/colonel-byte/ansible-collection-zarf/tree/main/examples/packages) are two providers of one capability and a consumer of it, to be read as working copies. Everything needed to write a document is below.

## The document

| Section                | Required | What it states                                                                                                                                                                                                                        |
| ---------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `apiVersion`           | yes      | Fixed to `colonel-byte.dev/v1alpha1`.                                                                                                                                                                                                 |
| `kind`                 | yes      | Fixed to `PackagePrerequisites`.                                                                                                                                                                                                      |
| `metadata`             | yes      | The package's name and version, and a sentence for a human.                                                                                                                                                                           |
| `requires`             | yes      | What the cluster must already be.                                                                                                                                                                                                     |
| `requires.kubernetes`  | no       | A SemVer constraint on the API server's version.                                                                                                                                                                                      |
| `requires.fluxcd`      | no       | A SemVer constraint on the installed Flux. Mutually exclusive with `requires.argocd`.                                                                                                                                                 |
| `requires.argocd`      | no       | A SemVer constraint on the installed Argo CD. Mutually exclusive with `requires.fluxcd`.                                                                                                                                              |
| `requires.controllers` | no       | The workloads that must be present and available, by name, namespace and kind, each with an optional SemVer constraint on its own version.                                                                                            |
| `requires.crds`        | no       | The CustomResourceDefinitions that must be established.                                                                                                                                                                               |
| `requires.storage`     | no       | A default StorageClass, named StorageClasses, and the access modes claims ask for.                                                                                                                                                    |
| `requires.packages`    | no       | Other Zarf packages that must be deployed first, with SemVer constraints. An entry names one package with `name`, or every name it is published under with `names`. Alternative packages with different versions are grouped by `id`. |
| `provides`             | no       | The capabilities, CRDs and namespaces this package contributes to the cluster.                                                                                                                                                        |

`requires` is required and `provides` is not, which is the asymmetry the format is for: a package that requires nothing has no reason to ship the document, while a leaf workload that provides nothing downstream is the common case.

```yaml
# yaml-language-server: $schema=../schema/zarf-v1alpha1-package-prerequisites-schema.json
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

The engine section, `controllers` and `crds` are three things rather than one because a version alone catches the least interesting failure: a cluster can hold the right engine release with its controllers scaled to zero, or hold the controllers and not the CRDs the package's own `HelmRelease` or `Application` needs. `controllers` and `crds` sit at the top because they are not engine-specific -- "these workloads must be available" and "these CRDs must be established" are the same question whatever reconciles the cluster.

## Deriving what to require, from the package you have

Do not start from another package's document. Start from the manifests and charts this package deploys -- the derivation is mechanical, and it is the difference between a document that is defensible and one that is copied.

Render first, then read what came out. A chart's templates say nothing until values are applied:

```sh
zarf dev find-images <dir>                              # also the images: list, so run it anyway
helm template <release> <chart> -f values/<file>.yaml   # per chart
kustomize build <dir>                                   # per kustomization
```

Then, over the rendered YAML:

| What you find in it                                                 | What it means you require                                                |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| An `apiVersion` that is not a core Kubernetes group                 | `requires.crds` -- that CRD must be established first                    |
| `HelmRelease`, `Kustomization`, `GitRepository`, `OCIRepository`    | `requires.fluxcd`, and the controllers that reconcile those kinds        |
| `Application`, `ApplicationSet`, `AppProject`                       | `requires.argocd`, and `argocd-application-controller`                   |
| A `PersistentVolumeClaim` with no `storageClassName`                | `requires.storage.defaultClass: true`                                    |
| A `PersistentVolumeClaim` naming a class                            | that class in `requires.storage.classes`                                 |
| A claim whose `accessModes` is not `ReadWriteOnce`                  | `requires.storage.accessModes` -- recorded, never checked; see the traps |
| An API your chart uses that was added in a known Kubernetes release | `requires.kubernetes.version`                                            |
| A workload your manifests need at a version, engine or not          | `requires.controllers[].version` -- no engine key required               |

The CRD line is the one that pays. It is a `grep` over rendered output, and it catches the exact failure this format exists for -- the package whose `HelmRelease` the API server accepts and nothing reconciles:

```sh
helm template <release> <chart> -f values/<file>.yaml | grep -E '^apiVersion:' | sort -u
```

Two things not to derive this way. `requires.packages` is a statement about how your bundle is ordered, not about your manifests, so it comes from knowing which package deploys the thing you need. And `requires.kubernetes.version` should be a version you can name a reason for -- an API your chart actually uses -- rather than whatever the cluster you tested on happened to run.

## Adding a document to a package

1. **Write `docs/prerequisites.yaml`** in the package's source tree, from the derivation above and the shape of the example.
2. **Keep the `$schema` line, pointing at a path that resolves from the document.** `zarf-v1alpha1-package-prerequisites-schema.json` sits beside this page -- copy it into the package, under `schema/` beside the `zarf.yaml`, and name it relative to `docs/prerequisites.yaml` as above. It is what makes an editor complete and validate the file as it is written, and a package carrying its own schema can be validated in the air-gapped facility it is opened in.
3. **Add it to the package's `documentation:` mapping**, which is what packages it into a layer that `zarf package inspect documentation` extracts without deploying anything:

   ```yaml
   documentation:
     prerequisites: docs/prerequisites.yaml
     schema: schema/zarf-v1alpha1-package-prerequisites-schema.json
   ```

   Naming the schema there packages it too, so the document can be validated wherever the tarball is opened.

4. **State only what you can defend.** Every requirement is a deployment somebody will be refused. A CRD the package's own manifests reference is defensible; a Kubernetes version copied from another package is not.
5. **Lint and check it**: `zarf dev lint <dir>`, then run the role against a real cluster.

## Checking a cluster against it

```yaml
- name: Check the cluster against what this package requires
  ansible.builtin.include_role:
    name: colonel_byte.zarf.prerequisites
  vars:
    zarf_package: /srv/staging/zarf-package-operator-external-secrets-amd64-2.12.0.tar.zst
    zarf_kubeconfig: /etc/rancher/rke2/rke2.yaml
    zarf_public_key: /etc/zarf/colonel-byte-zarf-packages.pub
```

The role extracts the document with `zarf_package_documentation`, reads the cluster with `zarf_cluster_info`, and fails once with every unmet requirement named. It sets three facts, and the third matters as much as the other two:

| Fact                         | What it holds                    |
| ---------------------------- | -------------------------------- |
| `zarf_prerequisites_checked` | What was examined                |
| `zarf_prerequisites_unmet`   | What did not hold                |
| `zarf_prerequisites_skipped` | What could not be checked at all |

**Print `zarf_prerequisites_skipped` even on a successful run.** A run that printed only its successes reads as though the whole document had been verified. Set `zarf_prerequisites_enforce: false` to report without failing the play, and `zarf_check_packages: false` on a cluster that is not initialised yet -- `zarf package list` needs one.

Deploying a bundle in order means checking inside the loop, not ahead of it: the first package is what satisfies the second one's requirements. See [`ansible/examples/playbooks/zarf-prerequisites.yml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/ansible/examples/playbooks/zarf-prerequisites.yml).

## The traps

- **`apiVersion` and `kind` must be exact.** `colonel-byte.dev/v1alpha1` and `PackagePrerequisites`. A file under the `prerequisites` key that says anything else is returned as plain text and never surfaces as a document -- deliberately, because a prose file returned as a document would hand the role a mapping with no `requires`.
- **`metadata.package` must match the package's `metadata.name`.** The document travels on its own, so one copied from a neighbouring package and not fully edited would otherwise be checked against a cluster and pass while describing something else.
- **Name the engine when the controllers do not name their namespaces.** The engine is the key -- `requires.fluxcd` or `requires.argocd` -- and it decides the namespace a controller requirement that names none is checked in, `flux-system` or `argocd`. The two are mutually exclusive: a package is reconciled by one engine, and the schema and the role both refuse a document naming both.
- **Argo CD's `argocd-application-controller` is a `StatefulSet`.** Set `kind: StatefulSet` on it. The default is `Deployment`, which is right for every Flux controller and for the rest of Argo CD, and a requirement that leaves it wrong reports a healthy controller as absent.
- **One comparator and one version.** `>=2.9.0` is evaluated; `>=1.30.0 <1.34.0` is not, and lands in `zarf_prerequisites_skipped`. Jinja is not a semver library, and a half-parsed range would check its first half and pass.
- **`requires.storage.accessModes` cannot be checked** -- what a provisioner will actually bind is not discoverable without creating a `PersistentVolumeClaim`. It is reported as skipped, never as satisfied.
- **A version on a controller needs no engine.** `requires.controllers[].version` constrains that workload's own `app.kubernetes.io/version`, so a package needing cert-manager v1.14 does not have to require Flux or Argo CD just to get a version field.
- **A version belongs to the engine whose key it sits under.** Flux is on 2.x and Argo CD on 3.x, so the numbers are not comparable. This is why the engine is the key and not a field beside the version: a document meaning `>=2.9.0` for Flux that said `argocd` instead would have been satisfied by Argo CD 3.5, passing against an engine it was never about.
- **The engine version is the engine's release, not a controller's.** For Flux, `>=2.9.0` is the flux2 release and not `source-controller` v1.9.5. It is read from the `app.kubernetes.io/version` label on the engine's own workloads, and neither upstream sets it: `flux install` adds it from its own build, and Argo CD's `install.yaml` carries none at all. A package that vendors either has to stamp the label itself -- in a kustomize `labels:` block with `includeSelectors: false`, since a label that reached `spec.selector.matchLabels` would make the next version bump unupgradable -- or the constraint is reported as unchecked.
- **`minAvailable` is availability, not readiness.** A replica that is ready and not yet available has passed its probes and not satisfied `minReadySeconds`, which means its controller is mid-rollout.
- **`requires.packages[].name` is what `zarf package list` reports**, which for the init package is `init`, not `cluster-init`.
- **Use `names` when a package is published under more than one name, and never both keys.** The upstream init reports itself as `init` and a rebuild of it as `init-slim`, and a package needing an initialised cluster needs either -- so the requirement names both, and the version constraint is stated once for the requirement rather than per name. An entry carries `name` or `names` and exactly one of the two; the schema and the role both refuse an entry carrying both, or neither, because the role reads `names` in preference to `name` and would otherwise check something other than what the document says.

  ```yaml
  requires:
    packages:
      - names:
          - init
          - init-slim
        version: ">=0.85.0"
  ```
- **Group alternative packages with different versions using `id`.** When alternatives have different version histories (such as two different storage providers satisfying a requirement), a single `version` constraint under `names` cannot describe both. Setting the same `id` on each alternative creates a disjunctive group: the requirement is satisfied if at least one branch in the group is deployed and meets its version constraint.

  ```yaml
  requires:
    packages:
      - name: csi-rook-ceph
        version: ">=1.21.0"
        id: storage
      - name: csi-longhorn
        version: ">=1.13.0"
        id: storage
  ```
- **State `provides` on anything another package requires.** It is what lets a bundle be ordered without deploying any of it; `requires` alone can only be satisfied by a cluster that already exists.

## Two statements of one fact

A package may also carry `colonel-byte.dev/*` annotations in its `metadata.annotations`, readable from `zarf package inspect definition` with no extraction -- one command, for a consumer that only needs to order a bundle. They duplicate what the document says, so the document is the source of truth and the role reads only that. Keep them in step or drop them.
