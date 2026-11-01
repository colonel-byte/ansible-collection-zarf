# The bundle fixture pool

Thirteen Zarf packages for testing the deployment planner described in [`docs/agent/choice-prerequisites-package-dag.md`](../../../../../docs/agent/choice-prerequisites-package-dag.md): the thing that reads a directory of archives, resolves each package's `PackagePrerequisites` document against the others, and emits a deployment order.

They are a **pool, not a bundle**. A planner takes a list of archives, so each test names the subset it needs rather than pointing at a directory that has to hold exactly the right packages -- which is what keeps the cycle and the version conflict out of the happy path without a second directory for each. Feeding in everything here is itself a case: it is ambiguous twice over and must be refused.

| Package           | What it is there for                                                                                                                 |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `init`            | The root. Empty `requires`, and every other document names it, so a plan that does not order it first is wrong                       |
| `csi-rook-ceph`   | `storage` candidate, `priority: 100`                                                                                                 |
| `csi-longhorn`    | `storage` candidate, `priority: 50`. The one to mark as already deployed when testing cluster precedence                             |
| `csi-nfs`         | `storage` candidate, `priority: 100` -- the deliberate tie with `csi-rook-ceph`                                                      |
| `csi-aws-ebs`     | `storage` candidate that requires `pkg-aws-ccm`, so losing its group has to prune the CCM with it                                    |
| `pkg-aws-ccm`     | The dependency `csi-aws-ebs` drags in. Nothing names it directly, so it is in a plan only when that candidate wins                   |
| `gitops-fluxcd`   | `gitops-engine` candidate, `priority: 100`                                                                                           |
| `gitops-argocd`   | `gitops-engine` candidate, `priority: 50`, numbering on 3.x against Flux's 2.x                                                       |
| `metrics-server`  | Carries no `documentation:` mapping, so the planner has to take its identity from the package definition rather than from a document |
| `workload-app`    | The consumer. Declares both `id` groups, and is the leaf a plan is built towards                                                     |
| `workload-pinned` | Requires `csi-rook-ceph <=1.0.0` while the pool's is 1.18.0: the version conflict                                                    |
| `cycle-a`         | Requires `cycle-b`                                                                                                                   |
| `cycle-b`         | Requires `cycle-a`. Two packages rather than a self-reference, because catching `A -> A` is not catching `A -> B -> A`               |

## What each case is fed

These are the cases `test/e2e/noncluster/testdata/deployment-order.yml` runs, and the orders below are what the planner actually returns. Where more than one order satisfies the graph, the planner breaks the tie lexically, which is why `metrics-server` lands where it does rather than beside the other root.

| Case                | Subset                                                                                                      | Expected                                                                   |
| ------------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| Clean plan          | `init`, `metrics-server`, `csi-rook-ceph`, `csi-longhorn`, `gitops-fluxcd`, `gitops-argocd`, `workload-app` | `init`, `csi-rook-ceph`, `gitops-fluxcd`, `metrics-server`, `workload-app` |
| Cluster precedence  | the same, with `csi-longhorn` reported deployed                                                             | `csi-longhorn` replaces `csi-rook-ceph`                                    |
| Operator preference | the same, with `preferred=['csi-longhorn']`                                                                 | the same again, without the cluster being read                             |
| Priority tie        | the same, plus `csi-nfs`                                                                                    | Refused, naming the group and both priority-100 candidates                 |
| Transitive pull-in  | `init`, `metrics-server`, `csi-aws-ebs`, `pkg-aws-ccm`, `gitops-fluxcd`, `workload-app`                     | `pkg-aws-ccm` ordered before `csi-aws-ebs`                                 |
| Pruning             | the same, plus `csi-rook-ceph`                                                                              | Rook-Ceph wins; `csi-aws-ebs` and `pkg-aws-ccm` are both dropped           |
| Version conflict    | `init`, `csi-rook-ceph`, `workload-pinned`                                                                  | Refused, naming `<=1.0.0` against 1.18.0                                   |
| Cycle               | `cycle-a`, `cycle-b`                                                                                        | Refused, naming both packages                                              |
| Missing provider    | `workload-app` alone                                                                                        | Refused, naming `init`                                                     |

## The documentless case

`metrics-server` carries no `documentation:` mapping, and `workload-app` requires it by name anyway. That pairing is the case: the requirement is written exactly as any other, so the planner cannot read a `metadata.package` to match it against and has to take the name and version from the package definition instead.

It is the common shape rather than an exotic one. Every other fixture here carries a document because each was written for this pool; a real staging directory holds archives from elsewhere, and most of those will never carry one.

## How they are built

Each package is one component with no resources at all -- no files, no images, no charts -- which is the smallest thing `zarf package create` builds and needs no registry, no cluster and no network. They are built at test time rather than committed as tarballs, for the same reason [`zarf-inspect-fixture`](../zarf-inspect-fixture) is: a `.tar.zst` is a binary blob nobody can review, and what a test cares about is the document.

```sh
zarf package create test/e2e/noncluster/testdata/bundle-fixture/init --confirm -o "$BUNDLE_DIR"
```

## The one thing here that does not exist yet

`provides.priority` is specified in the choice document and is **not yet a field on the Go type**, so the six documents carrying it do not validate against `schema/zarf-v1alpha1-package-prerequisites-schema.json` until it is added to `api/colonel-byte.dev/v1alpha1/prerequisites/spec.go` and `generate:schema` is run. Nothing enforces that today -- the `check-jsonschema` hook and `TestExampleDocumentsMatchTheTypes` are both scoped to `examples/packages/` -- which is why this paragraph is here instead.

Nothing in this directory is contacted.
