# Why the GitOps engine is a key under `requires`, not a field beside a version

`PackagePrerequisites` first shipped with a `requires.fluxcd` section holding three things: a version, the controller Deployments that must be available, and the CRDs that must be established. When Argo CD had to be supported, the obvious move was a `requires.argocd` beside it with the same three fields. That is not what happened, and this records why.

## What is actually engine-specific

Almost nothing. "These workloads must be available" and "these CRDs must be established" are the same question whatever reconciles the cluster, and they were only ever spelled `requires.fluxcd.*` because Flux was the only engine anybody had written a document for. A `requires.argocd` would have duplicated the struct, the schema, and all three of the role's checks -- and made the third engine worse, not better.

Exactly two things differ per engine:

- **The namespace a controller defaults to.** `flux-system` for Flux, `argocd` for Argo CD. A
document whose controllers name their namespaces never needs the engine; one that leaves them off does.
- **The kind of the workload.** Flux's controllers are all Deployments. Argo CD runs
`argocd-application-controller` as a StatefulSet and everything else as Deployments.

So `controllers` and `crds` were hoisted to `requires`, `ControllerRequirement` gained `kind`, and what was left of the engine -- its identity and its version -- became a section of its own.

## The identity is the key, not a field

That section was first written as `requires.gitops` holding `engine` and `version` side by side, and that was wrong in a way that took a second pass to see. The two engines do not number together: Flux is on 2.x while Argo CD is on 3.x. So a document could say `engine: argocd` next to `version: ">=2.9.0"` -- an author who meant Flux, editing a copied document -- and the two would disagree with nothing to notice it.

The disagreement is not symmetric, which is what makes it dangerous. Checked against a real cluster running Argo CD v3.5.5, `>=2.9.0` is **satisfied**, because 3.5 is greater than 2.9. The constraint passes against an engine it was never about, and the run reports success. The reverse -- meaning Argo CD and writing `fluxcd` -- fails loudly against Flux 2.9.5. The quiet direction is the one a reader is most likely to produce, and silent success is the one failure this whole format exists to prevent.

The fix is to leave nothing for the version to contradict. The engine is now the key -- `requires.fluxcd` or `requires.argocd` -- each holding only a version, in that engine's own numbering. A mismatch is no longer something to validate; it is something that cannot be written down.

The two are mutually exclusive, stated in both places it can be: `Requires.JSONSchemaExtend` emits `not: {required: [fluxcd, argocd]}` so an editor refuses it while the document is being written, and the role asserts it too, because a document reaches the role through `zarf_prerequisites` without ever meeting a validator.

What was lost is that `engine` as a single enum field was mutually exclusive for free. That is a real cost, paid for a property the enum could not give at all.

## Why the identity is still needed

The version is read off the `app.kubernetes.io/version` label, and the label means different things per engine: for Flux it is the flux2 release the manifests were built from, which is not any controller's own version. Without knowing the engine, the role cannot default a namespace and cannot say what the version it read is a version *of*.

A version required of something that is **not** an engine has its own home: `requires.controllers[].version`, on the workload itself. Without it the only version a document could put on a workload was the engine's, so a package needing cert-manager v1.14 had to require Flux or Argo CD purely to get a version field -- a prerequisite that refuses clusters for no reason.

An engine the format does not know is now unrepresentable rather than reported: the key is one of two the schema names. A document that requires neither is fine, and means a package with no opinion about what reconciles the cluster.

## Why now, and why it was breaking

It was done while `v1alpha1` had two consumers in this repository and no known consumers outside it. The alternative -- a `requires.argocd` that never breaks anything -- would have locked the engine-shaped mistake in permanently, for a format whose whole value is being readable by tools nobody here wrote.

The additive half was deliberately left for later and then done anyway in the same change: the StatefulSet query in `zarf_cluster_info` and the `app-argocd` reference package break nothing and could have waited for a real Argo CD user. They are here because the format change is unprovable without them -- a discriminator with one engine behind it is a field, not a decision.

## What this cost

`zarf_cluster_info` runs a fifth query. `deployments` and `statefulsets` are separate result keys rather than one `workloads` list with a `kind` on each entry, because merging them would have been a breaking change to a module result for no gain; the `prerequisites` role merges them itself, in one `set_fact`, and matches on `(namespace, name, kind)`.
