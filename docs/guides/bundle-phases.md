# The phased bundle the DAG has to produce

Each box is an alternative group: the bundle carries several providers and the plan picks one, which is `requires.packages[].id` on the consumer side. The arrows are the phase order a topological sort must recover from the documents alone.

```mermaid
flowchart TD
    subgraph seed ["Seed phase"]
        SeedInit["zarf-init<br/>seed components only"]
        SeedCSI["csi-rook-ceph<br/>csi-longhorn<br/>csi-aws-efs<br/>csi-aws-ebs"]
        SeedInit --> SeedCSI
    end

    FullInit["zarf-init<br/>remaining components"]

    subgraph storage ["Storage (id: storage)"]
        S1["csi-rook-ceph"]
        S2["csi-longhorn"]
        S3["csi-aws-efs"]
        S4["csi-aws-ebs"]
    end

    subgraph gitops ["GitOps (id: gitops-engine)"]
        G1["gitops-fluxcd"]
        G2["gitops-argocd"]
    end

    subgraph lb ["Load balancer (id: load-balancer)"]
        L1["lb-cilium"]
        L2["lb-metallb"]
        L3["lb-lox-lb"]
        L4["lb-aws-lb"]
    end

    subgraph mesh ["Service mesh (id: mesh)"]
        M1["mesh-istio-sidecar"]
        M2["mesh-istio-ambient"]
        M3["mesh-linkerd"]
        M4["mesh-greymatter"]
    end

    seed --> FullInit
    FullInit --> storage
    storage --> gitops
    gitops --> lb
    lb --> mesh
```

Two things on the whiteboard that the planner cannot do today, both already recorded in `docs/agent/choice-prerequisites-package-dag.md`:

- The seed phase deploys the **same** `zarf-init` archive twice, with different components, around the storage provider. A graph keyed by package identity has one node for it, so this bootstrap stays an explicit two-item stack.
- Every box is a pick-one group, so a directory holding all four CSI archives is ambiguous by default and needs an operator selection.
