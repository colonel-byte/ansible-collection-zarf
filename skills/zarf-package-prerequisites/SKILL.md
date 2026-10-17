---
name: zarf-package-prerequisites
description: >
  Declares what a Zarf package needs of a cluster before it deploys, as a
  colonel-byte.dev/v1alpha1 PackagePrerequisites document carried inside the package, and
  checks a cluster against it with the colonel_byte.zarf prerequisites role. Use when a
  package must require a Kubernetes version, a Flux install, CRDs, a StorageClass or
  another deployed package; when writing or fixing a docs/prerequisites.yaml; or when a
  deployment "succeeded" and nothing was reconciled. For the zarf.yaml itself, use
  zarf-package.
---

# Package prerequisites

A Zarf package deploys onto a cluster that cannot run it and reports success. Zarf applies the `HelmRelease`, the API server accepts it, nothing reconciles it, and the only symptom is a resource that never becomes ready in a cluster nobody is watching. A `PackagePrerequisites` document is the package stating its requirements where a machine can check them.

**Read [`docs/guides/package-prerequisites.md`](../../docs/guides/package-prerequisites.md) first.** It is the format's reference: every section, a full example, and why each one is shaped the way it is. The two reference packages under [`examples/packages/`](../../examples/packages/) are working copies to start from. This page is the procedure and the traps.

## Adding a document to a package

1. **Write `docs/prerequisites.yaml`** in the package's source tree, starting from `examples/packages/operator-external-secrets/docs/prerequisites.yaml`.
2. **Point `$schema` at [`schema/zarf-v1alpha1-package-prerequisites-schema.json`](../../schema/zarf-v1alpha1-package-prerequisites-schema.json)** -- a relative path in a checkout, a URL otherwise. It is what makes an editor complete and validate the file as it is written.
3. **Add it to the package's `documentation:` mapping**, which is what packages it into a layer that `zarf package inspect documentation` extracts without deploying anything:

   ```yaml
   documentation:
     prerequisites: docs/prerequisites.yaml
   ```

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
| `zarf_prerequisites_checked` | What was checked and held        |
| `zarf_prerequisites_unmet`   | What did not hold                |
| `zarf_prerequisites_skipped` | What could not be checked at all |

**Print `zarf_prerequisites_skipped` even on a successful run.** A run that printed only its successes reads as though the whole document had been verified. Set `zarf_prerequisites_enforce: false` to report without failing the play, and `zarf_check_packages: false` on a cluster that is not initialised yet -- `zarf package list` needs one.

Deploying a bundle in order means checking inside the loop, not ahead of it: the first package is what satisfies the second one's requirements. See [`ansible/examples/playbooks/zarf-prerequisites.yml`](../../ansible/examples/playbooks/zarf-prerequisites.yml).

## The traps

- **`apiVersion` and `kind` must be exact.** `colonel-byte.dev/v1alpha1` and `PackagePrerequisites`. A file under the `prerequisites` key that says anything else is returned as plain text and never surfaces as a document -- deliberately, because a prose file returned as a document would hand the role a mapping with no `requires`.
- **`metadata.package` must match the package's `metadata.name`.** The document travels on its own, so one copied from a neighbouring package and not fully edited would otherwise be checked against a cluster and pass while describing something else.
- **One comparator and one version.** `>=2.9.0` is evaluated; `>=1.30.0 <1.34.0` is not, and lands in `zarf_prerequisites_skipped`. Jinja is not a semver library, and a half-parsed range would check its first half and pass.
- **`requires.storage.accessModes` cannot be checked** -- what a provisioner will actually bind is not discoverable without creating a `PersistentVolumeClaim`. It is reported as skipped, never as satisfied.
- **The Flux version is the flux2 release, not a controller's.** It is read from the `app.kubernetes.io/version` label on the controller Deployments. A package whose manifests are vendored from upstream has to stamp that label itself -- upstream's carry no version anywhere -- or the constraint is reported as unchecked.
- **`minAvailable` is availability, not readiness.** A replica that is ready and not yet available has passed its probes and not satisfied `minReadySeconds`, which means its controller is mid-rollout.
- **`requires.packages[].name` is what `zarf package list` reports**, which for the init package is `init`, not `cluster-init`.
- **State `provides` on anything another package requires.** It is what lets a bundle be ordered without deploying any of it; `requires` alone can only be satisfied by a cluster that already exists.

## Two statements of one fact

A package may also carry `colonel-byte.dev/*` annotations in its `metadata.annotations`, readable from `zarf package inspect definition` with no extraction -- one command, for a consumer that only needs to order a bundle. They duplicate what the document says, so the document is the source of truth and the role reads only that. Keep them in step or drop them.
