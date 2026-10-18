# Why `requires.gitops.engine` exists instead of a section per engine

`PackagePrerequisites` first shipped with a `requires.fluxcd` section holding three things: a version, the controller Deployments that must be available, and the CRDs that must be established. When Argo CD had to be supported, the obvious move was a `requires.argocd` beside it with the same three fields. That is not what happened, and this records why.

## What is actually engine-specific

Almost nothing. "These workloads must be available" and "these CRDs must be established" are the same question whatever reconciles the cluster, and they were only ever spelled `requires.fluxcd.*` because Flux was the only engine anybody had written a document for. A `requires.argocd` would have duplicated the struct, the schema, and all three of the role's checks -- and made the third engine worse, not better.

Exactly two things differ per engine:

- **The namespace a controller defaults to.** `flux-system` for Flux, `argocd` for Argo CD. A
document whose controllers name their namespaces never needs the engine; one that leaves them off does.
- **The kind of the workload.** Flux's controllers are all Deployments. Argo CD runs
`argocd-application-controller` as a StatefulSet and everything else as Deployments.

So `controllers` and `crds` were hoisted to `requires`, `ControllerRequirement` gained `kind`, and what was left of the engine -- its identity and its version -- became `requires.gitops`.

## Why the identity is still needed

The version is read off the `app.kubernetes.io/version` label, and the label means different things per engine: for Flux it is the flux2 release the manifests were built from, which is not any controller's own version. Without knowing the engine, the role cannot default a namespace and cannot say what the version it read is a version *of*.

An engine the role does not recognise is appended to `zarf_prerequisites_skipped` rather than defaulted to `flux-system` and checked anyway. A role whose entire purpose is to stop a deployment succeeding against a cluster that cannot reconcile it must not report a check it did not make.

## Why now, and why it was breaking

It was done while `v1alpha1` had two consumers in this repository and no known consumers outside it. The alternative -- a `requires.argocd` that never breaks anything -- would have locked the engine-shaped mistake in permanently, for a format whose whole value is being readable by tools nobody here wrote.

The additive half was deliberately left for later and then done anyway in the same change: the StatefulSet query in `zarf_cluster_info` and the `app-argocd` reference package break nothing and could have waited for a real Argo CD user. They are here because the format change is unprovable without them -- a discriminator with one engine behind it is a field, not a decision.

## What this cost

`zarf_cluster_info` runs a fifth query. `deployments` and `statefulsets` are separate result keys rather than one `workloads` list with a `kind` on each entry, because merging them would have been a breaking change to a module result for no gain; the `prerequisites` role merges them itself, in one `set_fact`, and matches on `(namespace, name, kind)`.
