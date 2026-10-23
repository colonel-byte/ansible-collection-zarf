# Grouping alternative package prerequisites with id

This guide describes how to use `id` groups in `colonel-byte.dev/v1alpha1` `PackagePrerequisites` documents. It covers how `id` groups organize alternatives with different version requirements and how the `colonel_byte.zarf.prerequisites` role evaluates them.

## The problem: alternatives with different versions

A package often accepts more than one upstream provider to satisfy a requirement. For example, a package may run on either Rook Ceph or Longhorn for persistent storage.

When alternatives share the same version history, use `names`:

```yaml
requires:
  packages:
    - names:
        - init
        - init-slim
      version: ">=0.85.0"
```

The `names` list applies a single `version` constraint across every name in the list.

When alternatives have different version histories, a single version constraint cannot describe both packages. Rook Ceph may need `>=1.21.0` while Longhorn needs `>=1.13.0`.

## The solution: id groups

Set an optional `id` string on each alternative entry under `requires.packages`:

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

Entries with the same `id` value form a single disjunctive (OR) requirement group.

## Evaluation rules

The `colonel_byte.zarf.prerequisites` role evaluates `id` groups with the following rules:

1. **Local scope**: The `id` key applies only to `requires.packages`. It does not affect other requirement types.
2. **Group satisfaction**: The cluster satisfies a group if at least one entry in the group is deployed and meets its version constraint.
3. **Multiple deployed packages**: When a cluster has multiple packages from the same group installed, the check passes if at least one package meets its constraint. A failing sibling does not fail the group.
4. **Independent entries**: Entries without an `id` remain independent requirements. The cluster must satisfy all ungrouped entries.
5. **A group is stricter than a plain entry.** A plain `requires.packages` entry whose version is a range this role cannot read -- a compound range such as `>=1.30.0 <1.34.0`, which [the format's one-comparator limit](package-prerequisites.md) covers -- is reported in `zarf_prerequisites_skipped` and does **not** fail the play, because the package's presence was proven by a separate check and only its version is unknown. Inside a group there is no such separate proof: the group's whole contract is "one of these, at an acceptable version", and a constraint that cannot be read proves nothing about any branch. So a group whose only deployed branch carries an unreadable range **fails**, and says which of the three things went wrong -- see the messages below. Keep constraints inside a group to one comparator.
6. **Names per branch**: An entry inside an `id` group can still use `names` to cover aliases for that branch:

```yaml
requires:
  packages:
    - names:
        - csi-rook-ceph
        - csi-ceph
      version: ">=1.21.0"
      id: storage
    - name: csi-longhorn
      version: ">=1.13.0"
      id: storage
```

## Error reporting

When no branch in an `id` group meets its requirements, the role records one failure in `zarf_prerequisites_unmet`. The message names every option in the group, and then says which of three things happened -- because "nothing is deployed", "the wrong version is deployed" and "the constraint could not be read" are different problems with different fixes.

Nothing in the group is deployed:

```text
one of the alternative zarf packages for storage (csi-rook-ceph >=1.21.0 or csi-longhorn >=1.13.0) is required, and none of them is deployed to this cluster
```

Something is deployed, and its version was compared and did not satisfy the constraint:

```text
one of the alternative zarf packages for storage (init >=9.0.0 or csi-longhorn >=1.13.0) is required, and what is deployed does not satisfy its version constraint
```

Something is deployed, and its constraint is a range this role cannot read, so no branch could be verified. The range is named in `zarf_prerequisites_skipped` as well, which is the half that tells you what to simplify:

```text
one of the alternative zarf packages for storage (init >=0.85.0 <1.0.0 or csi-longhorn >=1.13.0) is required, and what is deployed carries a version range this role cannot evaluate (the deployed zarf package init), so no alternative could be verified
```

If packages from the group are not installed, the role reports:

```text
one of the alternative zarf packages for storage (csi-rook-ceph or csi-longhorn) is required and is not deployed to this cluster
```
