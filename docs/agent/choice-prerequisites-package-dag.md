# Ordering a directory of Zarf packages into a deployment DAG from prerequisites

This document records the architectural decisions, constraints, and trade-offs for automatically ordering a directory of Zarf package archives into a Directed Acyclic Graph (DAG) for deployment, using each package's embedded prerequisites configuration.

## The problem and goal

Today, playbooks deploying stacks of Zarf packages (e.g. through `deploy_packages`) require authors to specify an explicit deployment list or numeric step prefix. When operators receive a directory of arbitrary Zarf package tarballs, manually sorting them requires unpacking or inspecting each package to deduce dependency order (e.g. init packages, CRDs, service meshes, storage, application workloads).

Because packages in this ecosystem carry `PackagePrerequisites` (`colonel-byte.dev/v1alpha1`) inside their embedded `documentation.prerequisites` mapping, a bundle planner can extract these documents without deploying anything, build a dependency graph, verify that the graph is acyclic, and emit a topological deployment order.

## Static inspection vs runtime deployment

The primary virtue of the `PackagePrerequisites` design is offline planning:
- `zarf package inspect documentation` extracts the embedded `docs/prerequisites.yaml` directly from the package archive without access to a Kubernetes cluster or network.
- `zarf_package_documentation` already implements non-destructive inspection and YAML decoding of documentation keys for single packages.
- A directory-level planner reads every archive in the target directory, extracts its prerequisites document (or handles packages without one), and constructs the dependency model entirely in memory before any cluster mutation occurs.

Inspecting many large archives introduces disk I/O and process overhead from running `zarf package inspect documentation` across tarballs. A realistic planner caches extracted documents by archive checksum/mtime to avoid redundant unpacking across runs.

## Dependency resolution mechanics

Each inspected package carries:
- Identity: `metadata.package` and `metadata.version` in `PackagePrerequisites` (mapping to the concrete archive path in the directory).
- Provides: `provides.capabilities`, `provides.crds`, `provides.namespaces`.
- Requires: `requires.packages` (with `name`, `names`, and optional group `id`), `requires.crds`, `requires.storage_classes`, and engine/controller requirements.

In a directory bundle, edges in the DAG are constructed primarily via package dependencies:

1. **Direct package edges (`requires.packages[].name`)**:
   If package $B$ requires package $A$ by name, directed edge $A \to B$ is added ($A$ must deploy before $B$). If a version constraint is declared, the bundle planner verifies whether $A$'s version in the directory satisfies $B$'s constraint.

2. **Alias edges (`requires.packages[].names`)**:
   If package $B$ allows `names: [init, init-slim]`, and the directory contains a package matching an alias (e.g. `init`), an edge is added from that package to $B$. If multiple satisfying packages are present in the directory, the planner flags the bundle as ambiguous unless disambiguated by explicit configuration.

3. **Alternative group edges (`requires.packages[].id`)**:
   When a consumer package declares alternative requirements sharing an `id` in `requires.packages` (e.g. `csi-rook-ceph` vs `csi-longhorn` grouped under `id: storage`), the requirement is a disjunction on the consumer side. If exactly one provider from that group exists in the directory, an edge is created from that provider to the consumer. If multiple candidate providers in the group are present in the directory, the planner refuses the ambiguity unless the operator explicitly selects the target provider.

4. **Transitive CRD and capability resolution**:
   Packages can also declare `requires.crds`. If package $A$ lists CRD $C$ in `provides.crds` and package $B$ lists $C$ in `requires.crds`, an implicit dependency edge $A \to B$ exists even if $B$ does not explicitly name $A$ in `requires.packages`.

## Edge cases and failure modes

- **Cycles**: If circular dependencies exist ($A \to B \to A$), topological sort fails. The planner reports the cycle trace clearly and aborts before deploying any package.

```mermaid
flowchart TD
    PkgA[Package A<br/>requires: B] --> PkgB[Package B<br/>requires: A]
    PkgB --> PkgA
```

- **Phased component limits**: DAG planning at the archive level cannot resolve intra-package cyclic bootstrap sequences where the same package archive is deployed in multiple component phases (such as `init` seed components $\to$ storage provider $\to$ `init` full components, as recorded in `choice-deploy-packages-drift-scope.md`). Such bootstrap sequences remain the domain of explicit multi-phase playbooks.
- **Missing prerequisites**: If package $B$ requires a package or CRD not provided by any package in the directory, the planner distinguishes two modes:
  1. *Standalone bundle mode*: Reject the plan because the bundle is self-contained and incomplete.
  2. *Cluster-aware planning mode*: Check if the missing prerequisite is already satisfied on the target cluster (via `zarf_package_info` / `zarf_cluster_info`). If already satisfied on the cluster, the requirement is satisfied and does not need a local provider in the DAG.

```mermaid
flowchart TD
    PkgC[Package C<br/>requires: cluster-storage] -.->|Satisfied on Cluster?| Clust[(Target Cluster<br/>zarf_package_info)]
    PkgC -.->|Missing in Bundle?| Fail[Abort or Query Cluster]
```

- **Packages without prerequisites**: A package in the directory with no prerequisites document or an empty `requires` has in-degree 0 (no prerequisites) and out-degree 0 (unless another package requires it). Independent nodes are sorted deterministically using lexical package name ordering as a tie-breaker.
- **Multiple packages in the same `id` group**: When a directory contains multiple provider archives that satisfy the same consumer `id` group (e.g. both `csi-rook-ceph` and `csi-longhorn` for `id: storage`):
  1. *Default fail-fast rule*: The planner refuses the plan and errors out immediately. Attempting to deploy both mutually exclusive providers, or arbitrarily picking one without operator intent, risks conflicting storage classes or incorrect infrastructure.
  2. *Cluster precedence (existing installation preference)*: If one candidate from the group is already deployed on the cluster (e.g. `csi-longhorn` is already installed and healthy), cluster-aware mode prefers the existing installation. The requirement is satisfied by the cluster, avoiding an unnecessary redeployment or duplicate provider rollout.
  3. *Priority / weight ranking*: If offline bundles must choose between multiple candidate providers without operator intervention, an explicit priority breaks the tie. The ordered rule these three settle into, the `provides.priority` field it reads and what becomes of the losing candidate's own dependencies are specified in [Choosing between alternatives](#choosing-between-alternatives) below.

```mermaid
flowchart TD
    Alt1["Package X<br/>csi-rook-ceph (priority: 100)"]
    Alt2["Package Y<br/>csi-longhorn (priority: 50)"]
    Consumer["Package Z<br/>requires id: storage"]
    Cluster[("Installed on Cluster:<br/>csi-longhorn")]
    Alt1 -.->|"Both in folder: Ambiguous failure<br/>unless prioritized or installed"| Consumer
    Alt2 -.->|"Prefer existing install or weight"| Consumer
    Cluster -.->|"Precedence to installed"| Consumer
```

- **Multiple versions of the same package**: When the directory holds multiple versions of a single package (e.g. `operator-cert-manager` v1.13.0 and v1.14.0):
  1. *Default selection*: The planner selects the highest version present.
  2. *Constraint conflict risk*: If a dependent package requires a specific or constrained lower version (e.g. `<=1.13.5`), selecting the highest available version (v1.14.0) causes the version check to fail.
  3. *Planner resolution options*: The planner can evaluate version constraints against available candidate archives before picking the latest version that satisfies all consumers' constraints, or fail fast and require the operator to prune superseded archives from the directory.

```mermaid
flowchart TD
    VLow["Package M v1.13.0"]
    VHigh["Package M v1.14.0<br/>Highest candidate"]
    Dep["Package N<br/>requires: M <=1.13.5"]
    VHigh -.->|"Default to highest"| Dep
    Dep -.->|"Constraint Fails!"| Error["Version Conflict"]
```

- **Multiple architectures in the bundle directory**: Air-gapped delivery directories often stage archives for multiple CPU architectures side-by-side (e.g. `zarf-package-app-amd64-v1.0.0.tar.zst` and `zarf-package-app-arm64-v1.0.0.tar.zst`). Keying graph nodes strictly by package identity creates duplicate node collisions unless architecture is pruned first.
  1. *Architecture disambiguation*: The planner filters candidate archives by target architecture before building vertices. In cluster-aware mode, the architecture is queried from the cluster nodes (`kubernetes.io/arch`); in offline mode, it is provided explicitly (defaulting to the delegate host architecture).

```mermaid
flowchart TD
    ArchA["Package A (amd64)"]
    ArchB["Package A (arm64)"]
    Planner{"Target Arch Filter<br/>(amd64)"}
    NodeA["DAG Node: Package A<br/>path: app-amd64.tar.zst"]
    Ignored["Ignored Archive"]
    ArchA --> Planner
    ArchB --> Planner
    Planner -->|Matches| NodeA
    Planner -->|Mismatches| Ignored
```

- **Package flavors and variant selection**: Packages can be built under distinct flavors (e.g. `flavor: upstream` vs `flavor: hardened` or vendor-specific variants). Unlike architecture, flavor selection may be an intentional deployment requirement for a cluster stack.
  1. *Bundle ambiguity*: When multiple flavors of the same package and version exist in the folder, the planner requires an explicit flavor selector; otherwise, it fails fast on ambiguity.
  2. *Upstream Zarf state limitation & RFC*: Today, Zarf's cluster state secret (`zarf package list`) does not record or report the deployed package flavor. A cluster-aware planner cannot determine which flavor is currently running on the cluster. Recording and exposing deployed flavor data requires an upstream Zarf RFC (ZEP) to include build flavor in deployed package state. Until upstream tracks flavor on-cluster, cluster-aware checks cannot assert flavor parity without external package inspection.

```mermaid
flowchart TD
    FlavorA["Package A (flavor: hardened)"]
    FlavorB["Package A (flavor: upstream)"]
    Selector{"Flavor Selector / Override"}
    Chosen["Selected for DAG"]
    StateCheck{"Cluster-Aware Check:<br/>zarf package list"}
    RFC["Upstream RFC Needed:<br/>State secret lacks flavor"]
    FlavorA --> Selector
    FlavorB --> Selector
    Selector --> Chosen
    Chosen --> StateCheck
    StateCheck -.->|Flavor not in state!| RFC
```

- **Asynchronous CRD and StorageClass readiness races**: While Zarf deploys package archives sequentially, Kubernetes registers CRDs and CSI storage drivers asynchronously. If package $A$ installs a CRD or CSI driver and package $B$ immediately creates Custom Resources or PersistentVolumeClaims, the Kubernetes API server may reject $B$'s manifests with webhook timeouts or missing resource errors.
  1. *Topological ordering vs readiness barrier*: The DAG planner establishes deployment order ($A \to B$), but execution must enforce a readiness barrier between dependent steps (e.g. awaiting CRD `Established` condition or CSI controller availability) before initiating $B$'s deployment.

```mermaid
flowchart TD
    StepA["Deploy Package A<br/>installs CRD / StorageClass"]
    Barrier{"Readiness Barrier<br/>wait CRD Established<br/>wait CSI Available"}
    StepB["Deploy Package B<br/>applies CR / PVC"]
    StepA --> Barrier
    Barrier -->|Ready| StepB
    Barrier -.->|Timeout / Not Ready| Abort["Abort Deploy"]
```

- **Partial or unhealthy cluster state**: In cluster-aware planning mode, a package may appear in `zarf package list`, but a previous deployment failed midway or omitted required components.
  1. *Health and component completeness check*: Rather than assuming package presence in `zarf package list` satisfies an edge, cluster-aware planning must verify that required components are deployed and controller workloads meet availability constraints (`minAvailable >= 1`). If unhealthy or incomplete, the edge must remain active in the local deployment DAG to re-reconcile the package.

```mermaid
flowchart TD
    ClusterList["zarf package list:<br/>Package A present"]
    HealthCheck{"Health & Component<br/>Completeness Check"}
    Satisfied["Edge Satisfied<br/>Skip local deploy"]
    Reconcile["Keep Edge Active<br/>Re-reconcile Package A"]
    ClusterList --> HealthCheck
    HealthCheck -->|Healthy & Complete| Satisfied
    HealthCheck -->|Degraded or Incomplete| Reconcile
```

- **Directory hygiene and non-archive artifacts**: Staging directories frequently hold non-package artifacts (`.sha256` checksums, `.sig` Cosign signatures, `README.md`, or unrelated files). Furthermore, Zarf packages already embed internal checksums and metadata specifications.
  1. *Archive filtering*: The planner must strictly filter for valid archive extensions (`*.tar.zst`, `*.tar`) and ignore non-tarball companion files.
  2. *Graceful warning on non-Zarf tarballs*: When an archive in the directory cannot be parsed by Zarf (e.g. an arbitrary tarball missing `zarf.yaml` or failing internal checksum validation), the planner warns or ignores the file rather than aborting the entire bundle plan.

## Choosing between alternatives

A group with one candidate in the directory needs no rule: that candidate is the provider. The rule below runs only when a group has more than one, which is the case the whiteboard bundle is made of -- four CSI providers, four load balancers, four meshes and two GitOps engines staged side by side, of which one of each is deployed.

The first rule that resolves the group wins:

1. **Already deployed on the cluster.** If one candidate appears in `zarf package list`, it is the provider. A cluster that already runs Longhorn does not get Rook-Ceph because the bundle rates Rook-Ceph higher -- replacing a storage provider under a running cluster is not a thing a planner decides.
2. **Operator preference.** `zarf_plan_preferred`, an ordered list of package names; the first entry present among the candidates wins. This is where an operator states an opinion for a cluster, and where a bundle vendor ships one in `group_vars` beside the archives.
3. **`provides.priority`.** The highest value among the candidates wins.
4. **Otherwise, fail**, naming the group and every candidate in it. A tie at the top of rule 3 is a failure too: two candidates at `priority: 100` is a bundle that did not state a preference, and picking one by sort order would make the plan depend on a file name.

Rule 1 before rule 3 is the whole point of the order, and it is the case that motivated this: priority says which provider a fresh cluster should get, not which one a running cluster should be migrated to.

### The `provides.priority` field

```yaml
provides:
  priority: 100
  capabilities:
    - name: storage-class
      version: "1.7.2"
```

A new integer field on `Provides`, absent meaning zero. Three things about what it is and is not:

- **It is a rank, not a score.** Only the relative order matters, and only among packages that are candidates for the same `id` group. `100` means nothing on its own.
- **It is only ever compared within one group.** The planner never ranks a storage provider against a mesh, so the number does not need a scale that holds across the bundle. It does mean the value is unanchored -- a reader cannot tell from `priority: 100` alone what it is competing with -- and the description field beside it is where a package says.
- **It is the package's own opinion, which is why it loses to both rules above it.** Every provider rates itself highly. A cluster that already made the choice, and an operator who made it deliberately, both outrank what the archive thinks of itself.

Adding it is an API change rather than a convention: the field goes on `Provides` in `api/colonel-byte.dev/v1alpha1/prerequisites/spec.go`, `generate:schema` rewrites the committed schema, and `TestCommittedSchemasAreCurrent` and `TestExampleDocumentsMatchTheTypes` hold the two together.

### Cluster presence is not cluster health

Rule 1 reads presence in `zarf package list`, which is not the same as a working provider: a Longhorn whose deployment failed halfway is present, wins its group, and then does not provide storage. Checking `minAvailable` on its controllers first is the correct version and needs the readiness machinery the `prerequisites` role already has.

The first implementation takes presence at face value. A degraded provider that wins its group is re-reconciled by `deploy_packages`' own drift check rather than silently skipped, which is the tolerable failure; the intolerable one -- deploying a second storage provider beside a half-broken first -- is what rule 1 prevents either way.

## The chosen alternative drags its own dependencies in

Choosing a provider is not choosing one node. `csi-aws-ebs` and `csi-aws-efs` both need the AWS Cloud Controller Manager, published separately as [`pkg-aws-ccm`](https://github.com/colonel-byte/zarf-csi-aws/tree/main/pkg-ccm), and a plan that deploys the CSI driver without it deploys a driver that cannot talk to EC2. Rook-Ceph and Longhorn need no such thing. So the dependency is not a property of the storage phase -- it is a property of one candidate in it, and it enters the graph only if that candidate wins.

Nothing new is needed to find the edge. `csi-aws-ebs` names `pkg-aws-ccm` under `requires.packages`, the planner resolves it like any other package edge, and the CCM is ordered first. The CCM itself carries no `documentation:` mapping today and so ships no prerequisites document at all, which is fine on the provider side: a package with no document is a root with nothing of its own to require. It would need one only to state what it requires in turn, or to be reachable by capability rather than by name.

What is new is the other half: **the plan is the subgraph reachable from the chosen candidates, not every archive in the directory.** After the groups are resolved, the planner walks forward from the winners and the ungrouped roots and keeps what it reaches; a losing candidate is dropped, and so is anything that only the loser required. Without that step a directory staging all four CSI providers deploys the AWS Cloud Controller Manager onto a bare-metal cluster that chose Rook-Ceph, because `pkg-aws-ccm` is a valid node with in-degree zero and nothing said it was optional.

```mermaid
flowchart TD
    CCM["pkg-aws-ccm"]

    subgraph storage ["Storage (id: storage)"]
        LH["csi-longhorn<br/>chosen"]
        EBS["csi-aws-ebs<br/>requires: pkg-aws-ccm"]
        EFS["csi-aws-efs<br/>requires: pkg-aws-ccm"]
        RC["csi-rook-ceph"]
    end

    App["workload-app<br/>requires id: storage"]

    CCM --> EBS
    CCM --> EFS
    LH --> App

    style LH stroke-width:3px
    style EBS stroke-dasharray: 4 4
    style EFS stroke-dasharray: 4 4
    style RC stroke-dasharray: 4 4
    style CCM stroke-dasharray: 4 4
```

Dashed nodes are pruned: the three losing candidates because they lost, and `pkg-aws-ccm` because the only packages that required it are gone. Had `csi-aws-ebs` won instead, the CCM would be solid and ordered ahead of it.

A package that is required by both a winner and a loser is kept, which falls out of reachability without a special case. A package in the directory that nothing reaches is not deployed at all -- which is the right default for a staging directory, and the one thing here an operator may want to override.

## The plan is the same every run, and the three things that change it

The order is a pure function of the package names, their versions, the documents they carry, the list of packages the cluster reports, and the operator's preference list. Nothing else reaches it: not the order the directory is listed in, not the archive filenames, not mtimes, and not Python's per-process hash seed -- every set in the planner is iterated through `sorted()`, so a set's iteration order cannot leak into a plan. Where the graph leaves the order free, the tie is broken lexically by package name.

That is a property worth stating because the failure it prevents is nearly invisible: a planner whose output followed the filesystem's listing order produces one order on the operator's laptop and another on the management node, from the same tarballs, and the difference only ever shows up as a deployment that failed in one place and not the other. `test/e2e/noncluster/testdata/deployment-order.yml` holds it by ordering a subset twice with the input reversed.

Three things do change a plan, and none of them changes it quietly:

1. **The cluster changed.** Rule 1 of the chain reads what is deployed, so a plan is a statement about one cluster at one moment rather than about the directory alone. In the ordinary case this is self-stabilising -- the first run picks the highest-priority candidate, and every run after it finds that candidate deployed and picks it again -- but a provider installed out of band legitimately moves the plan onto it. That is the rule working, and it is the deliberate cost of letting a running cluster outrank a bundle's opinion of itself.
2. **The preference list changed.** `zarf_plan_preferred` is operator input and is read in the order it is written.
3. **The directory changed.** Adding an archive can change which candidates a group has. Adding a *second archive of a package already there* -- an upgraded tarball, or the same package built for another architecture -- is refused rather than resolved, because selecting between them is exactly the choice a planner has no basis to make.

What is explicitly not a source of variation is a package carrying no prerequisites document. It is a root with nothing of its own to require, which is a defined position in the graph rather than a gap to be filled differently on each run.

## Output shape and execution contract

The DAG planner emits a linear, topologically ordered list of package items compatible with `deploy_packages` (`zarf_packages_to_deploy`), mapping each resolved node to its concrete archive path.

Because Zarf mutates cluster state sequentially through a single kubeconfig and Ansible tasks are serialized with `run_once: true` on the delegated host, execution against the cluster is serialized in topological order rather than run concurrently.

## Visualizing the dependency graph

```mermaid
flowchart TD
    subgraph Phase1 [Phase 1: Foundation]
        Init[init / init-slim]
    end

    subgraph Phase2 [Phase 2: Platform & Engine]
        Flux[app-fluxcd<br/>provides: gitops-engine]
    end

    subgraph Phase3 [Phase 3: Shared Infrastructure]
        CertManager[operator-cert-manager<br/>provides: cert-manager CRDs]
        Storage[csi-rook-ceph<br/>satisfies id: storage]
    end

    subgraph Phase4 [Phase 4: Workloads & Addons]
        ExtSecrets[operator-external-secrets]
        App[workload-app]
    end

    Init --> Flux
    Flux --> CertManager
    Init --> Storage
    CertManager --> ExtSecrets
    Flux --> ExtSecrets
    Storage --> App
    ExtSecrets --> App
```
