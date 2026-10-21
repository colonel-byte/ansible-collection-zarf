# app-argocd

Argo CD, packaged from the upstream manifests, as the GitOps engine the rest of a bundle reconciles through. It is a reference implementation of the [`PackagePrerequisites`](../../../docs/guides/package-prerequisites.md) format, not a package this repository builds or ships.

It is the sibling of [`app-fluxcd`](../app-fluxcd), and it exists because the two engines are not the same shape. Three differences, each of which the format had to grow something to survive:

| Argo CD does this                                             | The format answers with                                            |
| ------------------------------------------------------------- | ------------------------------------------------------------------ |
| Runs `argocd-application-controller` as a **StatefulSet**     | `requires.controllers[].kind`                                      |
| Installs into **`argocd`**, not `flux-system`                 | `requires.argocd`, the engine key that picks the default namespace |
| Ships manifests with **no `app.kubernetes.io/version` label** | The generated kustomization stamps the pinned tag on               |

The third is the one worth dwelling on. `kubectl apply -f manifests/install.yaml` gives you an Argo CD whose version no cluster query can report, so a document requiring `>=3.5.0` would be checked against nothing. That is why `upstream/*/kustomization.yaml` is generated rather than vendored -- it adds the label, and the label is what makes the constraint answerable. Flux has the same problem and `flux install` solves it the same way.

## How it is vendored

`mage examples:vendor` reads [`resources.json`](resources.json), which pins `v3.5.5`.

Unlike `app-fluxcd`, this package vendors from a **path in the tagged source tree** rather than a release asset. Flux attaches `manifests.tar.gz` to every release; Argo CD attaches only CLI binaries and SBOMs, and its release notes tell you to apply `raw.githubusercontent.com/argoproj/argo-cd/v3.5.5/manifests/install.yaml`. `resources.json` says `tree` instead of `asset` for that reason.

The CRDs are a component of their own, taken from `manifests/crds/` rather than from `install.yaml`. Two reasons: a cluster whose `Application` CRD is not established yet cannot hold the `Application`s a downstream package deploys, and a bundle that installs both in one component has no way to wait in between; and `install.yaml` is `namespace-install.yaml` **plus** those same CRDs, so taking both would vendor 1.8 MB twice.

That 1.8 MB is most of this package. `applicationset-crd.yaml` alone is 1.4 MB of OpenAPI schema, which is simply what Argo CD's CRDs weigh.

## What it declares

[`docs/prerequisites.yaml`](docs/prerequisites.yaml) is almost all `provides`: an engine requires very little and offers a great deal. Note that it provides the capability `gitops-engine` under the same name `app-fluxcd` does, deliberately -- a package that needs something to reconcile its manifests requires that name, and either package satisfies it.
