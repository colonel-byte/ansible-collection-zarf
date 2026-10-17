# `operator-external-secrets`

A reference Zarf package that declares cluster prerequisites, and the harder of the two cases in [`examples/packages`](../README.md).

The External Secrets Operator is delivered here as a `HelmRelease` for Flux to reconcile, so nothing in this package installs it. It writes an `OCIRepository` and a `HelmRelease` and stops. On a cluster with no `helm-controller` running, both manifests are accepted, `zarf package deploy` reports success, and no operator ever appears -- the failure the [`PackagePrerequisites`](../../../docs/guides/package-prerequisites.md) document beside this file exists to turn into a refusal.

ESO is a pointed example of why that matters. What it reconciles are Secrets, so a cluster where it was never really installed does not fail loudly: the `ExternalSecret` objects sit there, the Secrets they promise never appear, and the first symptom is a workload that will not start because a Secret it mounts is missing -- several steps and possibly several hours from the deployment that was reported as successful.

| File                                                 | What it is                                                                                        |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| [`zarf.yaml`](zarf.yaml)                             | The package definition, including the `documentation:` mapping that carries the document          |
| [`docs/prerequisites.yaml`](docs/prerequisites.yaml) | What the cluster must already be: a Kubernetes version, a Flux version, controllers, and the CRDs |
| [`manifests/`](manifests)                            | The `OCIRepository` and `HelmRelease` the component deploys                                       |

Its `provides:` section lists the six CRDs the chart establishes and the `external-secrets-engine` capability, so a package that needs an `ExternalSecret` to be reconciled can require this one by name rather than by hoping.

Nothing builds this package: `zarf package create` would reach the network for the chart. It is here to be read, linted and inspected.

```sh
go run ./magefiles/core examples:lint
```
