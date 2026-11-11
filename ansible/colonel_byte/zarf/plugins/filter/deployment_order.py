# Copyright 2026 colonel-byte
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""The deployment planner: a directory of Zarf packages ordered into a deployment sequence.

It is a filter rather than a module because nothing here reads a cluster or a package. By the time
this runs, the packages have already been inspected -- `zarf_package_inspect` for the name and
version every archive has, `zarf_package_documentation` for the `PackagePrerequisites` document the
ones that carry it have -- and what is left is arithmetic over those documents. A module would be a
second process and a second JSON round trip to do a topological sort in.

The rules it implements, and the reasoning behind each, are in
docs/agent/choice-prerequisites-package-dag.md. In short:

- An edge per `requires.packages[]` entry, from the provider to the consumer.
- Entries sharing an `id` are alternatives, one of which must be deployed. Which one is decided by
  the cluster first, then by the operator, then by the candidates' own `provides.priority`, and a
  group that none of the three resolves is refused rather than guessed at.
- The plan is the subgraph reachable from the goals, so a losing candidate is dropped and so is
  anything only that candidate required.
- A cycle, an unsatisfiable version constraint, a requirement nothing provides and a group that
  stays ambiguous are all refusals. The planner never deploys a partial plan: it is read before
  anything reaches the cluster, which is the one moment where failing costs nothing.
"""

from __future__ import annotations

from ansible.errors import AnsibleFilterError

DOCUMENTATION = r"""
name: deployment_order
short_description: Order inspected Zarf packages into a deployment sequence
description:
  - "Takes the packages found in a staging directory, each with the C(PackagePrerequisites) document it carries, and returns them in an order that satisfies every declared requirement."
  - "Alternatives sharing a C(requires.packages[].id) are resolved to one candidate, and packages no goal reaches are dropped from the plan rather than deployed."
  - "Refuses rather than guesses: a cycle, a requirement nothing in the directory provides, a version constraint no candidate satisfies, and a group that stays ambiguous are all errors raised before anything is deployed."
options:
  _input:
    description:
      - "The inspected packages. Each entry is a mapping with C(path), C(name), C(version), and C(prerequisites) holding the parsed document or nothing at all when the package carries none."
    type: list
    elements: dict
    required: true
  deployed:
    description:
      - "The packages already on the cluster, as the C(colonel_byte.zarf.packages) role sets C(zarf_packages). An entry may be a name or a mapping carrying C(package)."
      - "A candidate already deployed wins its group outright, which is why this outranks every other rule: replacing a running provider because a bundle rates another one higher is not a planner's decision."
    type: list
    default: []
  preferred:
    description:
      - "Package names in preference order. The first of them present among a group's candidates wins the group."
    type: list
    default: []
"""

EXAMPLES = r"""
- name: Order the staging directory into a deployment plan
  ansible.builtin.set_fact:
    zarf_packages_to_deploy: >-
      {{ zarf_plan_candidates
         | colonel_byte.zarf.deployment_order(deployed=zarf_packages,
                                              preferred=zarf_plan_preferred) }}
"""

RETURN = r"""
_value:
  description:
    - "The packages to deploy, in order. Each entry carries the C(path), C(name) and C(version) of the package it came from, so the list is usable as C(zarf_packages_to_deploy) directly."
  type: list
  elements: dict
"""

#: The comparators a version constraint may lead with, longest first so that `>=` is matched before
#: `>`. The set is deliberately the one the prerequisites role evaluates: a compound range is a
#: legal value of the field and is refused here rather than half-evaluated, because a constraint
#: that is silently read as its first half is worse than one that fails loudly.
_COMPARATORS = (">=", "<=", "==", "=", ">", "<")


def _version_tuple(version):
    """Split a version into integer parts, ignoring anything that is not one.

    A `v` prefix is dropped and a prerelease suffix is cut at the first `-`, which is the whole of
    the normalisation: this compares releases, and a planner that ranked `1.9.0-rc1` against
    `1.9.0` would be answering a question no document here asks.
    """
    text = str(version).strip().lstrip("vV").split("-", 1)[0].split("+", 1)[0]
    parts = []
    for piece in text.split("."):
        try:
            parts.append(int(piece))
        except ValueError:
            break
    return tuple(parts) or (0,)


def _compare(left, right):
    """Order two versions, padding the shorter so that 1.9 and 1.9.0 compare equal."""
    a, b = _version_tuple(left), _version_tuple(right)
    width = max(len(a), len(b))
    a += (0,) * (width - len(a))
    b += (0,) * (width - len(b))
    return (a > b) - (a < b)


def _satisfies(version, constraint):
    """Whether a version satisfies one comparator and one version.

    shortcut: one comparator only, so a compound range like ">=1.0.0 <2.0.0" raises rather than
    being evaluated; upgrade when a document in the wild needs one, and give the role's
    version_check.yml the same treatment at the same time so the two cannot disagree.
    """
    text = str(constraint).strip()
    if not text:
        return True
    if " " in text or "," in text:
        raise AnsibleFilterError(
            "version constraint %r is a compound range, which this planner does not evaluate; "
            "state one comparator and one version" % constraint
        )
    for comparator in _COMPARATORS:
        if text.startswith(comparator):
            wanted = text[len(comparator):].strip()
            order = _compare(version, wanted)
            if comparator == ">=":
                return order >= 0
            if comparator == "<=":
                return order <= 0
            if comparator == ">":
                return order > 0
            if comparator == "<":
                return order < 0
            return order == 0
    return _compare(version, text) == 0


def _deployed_names(deployed):
    """The names of the packages on the cluster, from either shape the fact comes in."""
    names = set()
    for entry in deployed or []:
        if isinstance(entry, dict):
            name = entry.get("package") or entry.get("name")
            if name:
                names.add(str(name))
        elif entry:
            names.add(str(entry))
    return names


def _requirement_names(requirement):
    """The package names one requirement entry accepts, in the order it wrote them.

    `names` is read in preference to `name`, which is what the role does and what the schema's
    `oneOf` means: a requirement carrying both is refused where it is written, so reaching here
    with both is a document that never met a validator.
    """
    if requirement.get("names"):
        return [str(n) for n in requirement["names"]]
    if requirement.get("name"):
        return [str(requirement["name"])]
    raise AnsibleFilterError(
        "a requirement names no package: every entry of requires.packages needs name or names"
    )


def _index(packages):
    """Map package name to its entry, refusing a directory holding one package twice."""
    index = {}
    for package in packages:
        if not isinstance(package, dict):
            raise AnsibleFilterError("each package must be a mapping, got %r" % type(package).__name__)
        name = package.get("name")
        path = package.get("path")
        if not name or not path:
            raise AnsibleFilterError("each package needs a name and a path, got %r" % (package,))
        name = str(name)
        if name in index:
            # Two archives of one package is a choice the planner has no basis to make: the newer
            # one may be exactly what a consumer's constraint excludes. Pruning the directory is
            # the operator's call.
            raise AnsibleFilterError(
                "the directory holds %s twice, as %s and %s; leave one of them"
                % (name, index[name]["path"], path)
            )
        index[name] = package
    return index


def _requirements(package):
    """The `requires.packages` entries of one package, or none at all.

    A package carrying no document is a root with nothing of its own to require, which is the
    common case for anything from outside this ecosystem rather than an error.
    """
    document = package.get("prerequisites") or {}
    if not isinstance(document, dict):
        raise AnsibleFilterError(
            "%s carries a prerequisites document that is not a mapping" % package["name"]
        )
    requires = document.get("requires") or {}
    return [r for r in (requires.get("packages") or []) if isinstance(r, dict)]


def _priority(package):
    return int(((package.get("prerequisites") or {}).get("provides") or {}).get("priority") or 0)


def _resolve_group(group_id, requirements, index, deployed, preferred, consumer):
    """Pick the one candidate of an alternative group, by the chain the choice document records."""
    candidates = []
    for requirement in requirements:
        for name in _requirement_names(requirement):
            if name in index and name not in [c[0] for c in candidates]:
                candidates.append((name, requirement))

    if not candidates:
        wanted = sorted({n for r in requirements for n in _requirement_names(r)})
        raise AnsibleFilterError(
            "%s requires one of %s for group %r, and the directory holds none of them"
            % (consumer, ", ".join(wanted), group_id)
        )

    if len(candidates) == 1:
        return candidates[0]

    # 1. The cluster has already made this choice.
    on_cluster = [c for c in candidates if c[0] in deployed]
    if len(on_cluster) == 1:
        return on_cluster[0]
    if len(on_cluster) > 1:
        raise AnsibleFilterError(
            "group %r required by %s has %s deployed on the cluster at once; the planner cannot "
            "tell which one the cluster means to keep"
            % (group_id, consumer, " and ".join(sorted(c[0] for c in on_cluster)))
        )

    # 2. The operator has.
    for name in preferred or []:
        for candidate in candidates:
            if candidate[0] == str(name):
                return candidate

    # 3. The candidates' own opinion of themselves.
    ranked = sorted(candidates, key=lambda c: -_priority(index[c[0]]))
    best = _priority(index[ranked[0][0]])
    tied = [c[0] for c in ranked if _priority(index[c[0]]) == best]
    if len(tied) == 1:
        return ranked[0]

    raise AnsibleFilterError(
        "group %r required by %s is ambiguous: %s are all candidates at priority %d. Deploy one "
        "of them to the cluster, name one in the preference list, or give them distinct "
        "provides.priority values." % (group_id, consumer, ", ".join(sorted(tied)), best)
    )


def _edges(index, deployed, preferred):
    """The provider-to-consumer edges of every package in the directory.

    Returns the adjacency and, separately, every name any requirement mentions -- the second is
    what tells a goal from a candidate, and a candidate's own name being mentioned is exactly what
    keeps it out of the goal set whether or not it goes on to win its group.
    """
    adjacency = {name: set() for name in index}
    mentioned = set()

    for name, package in sorted(index.items()):
        grouped = {}
        for requirement in _requirements(package):
            mentioned.update(_requirement_names(requirement))
            group_id = requirement.get("id")
            if group_id:
                grouped.setdefault(str(group_id), []).append(requirement)
                continue
            chosen = None
            for candidate in _requirement_names(requirement):
                if candidate in index:
                    chosen = candidate
                    break
            if chosen is None:
                wanted = " or ".join(_requirement_names(requirement))
                if set(_requirement_names(requirement)) & deployed:
                    # Already on the cluster and not in the directory: satisfied, no edge to draw.
                    continue
                raise AnsibleFilterError(
                    "%s requires %s, which is neither in the directory nor on the cluster"
                    % (name, wanted)
                )
            _check_version(name, chosen, requirement, index)
            adjacency[chosen].add(name)

        for group_id, requirements in sorted(grouped.items()):
            chosen, requirement = _resolve_group(
                group_id, requirements, index, deployed, preferred, name
            )
            _check_version(name, chosen, requirement, index)
            adjacency[chosen].add(name)

    return adjacency, mentioned


def _check_version(consumer, provider, requirement, index):
    constraint = requirement.get("version")
    if not constraint:
        return
    have = index[provider].get("version") or ""
    if not _satisfies(have, constraint):
        raise AnsibleFilterError(
            "%s requires %s %s, and the directory holds %s"
            % (consumer, provider, constraint, have or "a package with no version")
        )


def _reachable(adjacency, mentioned, index):
    """The packages a plan actually deploys: everything a goal depends on, and nothing else.

    A goal is a package no requirement anywhere names. Walking back from the goals is what drops a
    losing candidate, and what drops anything only that candidate required -- the AWS cloud
    controller manager has in-degree zero and would otherwise be deployed onto a cluster that
    chose Longhorn.
    """
    requires = {name: set() for name in index}
    for provider, consumers in adjacency.items():
        for consumer in consumers:
            requires[consumer].add(provider)

    keep = set()
    frontier = [name for name in index if name not in mentioned]
    if not frontier:
        # Every package is named by some requirement, so nothing is a goal and a plan built from
        # the goals would be empty. _sorted has already ruled out the cycle that usually causes
        # this, which leaves a directory of nothing but providers: say so rather than return [].
        raise AnsibleFilterError(
            "every package in the directory is required by another one, so there is nothing the "
            "plan is for; add the package being deployed, or deploy them by name"
        )
    while frontier:
        name = frontier.pop()
        if name in keep:
            continue
        keep.add(name)
        frontier.extend(sorted(requires[name]))
    return keep


def _sorted(adjacency):
    """Kahn's algorithm over the whole graph, lexical by name where the order is free.

    Over the whole graph rather than over the subgraph a plan keeps, and that is not a detail: a
    cycle removes every package in it from the goal set, because each one is named by the other.
    Pruning first therefore turned `cycle-a -> cycle-b -> cycle-a` into an empty plan reported as a
    success -- the one outcome worse than refusing. Sorting first means the cycle is found whether
    or not the plan would have deployed it, and a topological order restricted to a subset is
    still a topological order, so the pruning below is a filter rather than a second sort.
    """
    indegree = {name: 0 for name in adjacency}
    for consumers in adjacency.values():
        for consumer in consumers:
            indegree[consumer] += 1

    ready = sorted(n for n, d in indegree.items() if d == 0)
    order = []
    while ready:
        name = ready.pop(0)
        order.append(name)
        for consumer in sorted(adjacency[name]):
            indegree[consumer] -= 1
            if indegree[consumer] == 0:
                ready.append(consumer)
        ready.sort()

    if len(order) != len(adjacency):
        stuck = sorted(set(adjacency) - set(order))
        raise AnsibleFilterError(
            "the packages depend on each other in a cycle, so no order satisfies them: %s"
            % ", ".join(stuck)
        )
    return order


def deployment_order(packages, deployed=None, preferred=None):
    """Order inspected packages into a deployment sequence. See the module docstring."""
    if not isinstance(packages, list):
        raise AnsibleFilterError(
            "deployment_order takes a list of inspected packages, got %r" % type(packages).__name__
        )

    index = _index(packages)
    if not index:
        return []

    adjacency, mentioned = _edges(index, _deployed_names(deployed), preferred or [])
    order = _sorted(adjacency)
    keep = _reachable(adjacency, mentioned, index)
    return [
        {
            "path": index[name]["path"],
            "name": name,
            "version": index[name].get("version", ""),
        }
        for name in order
        if name in keep
    ]


class FilterModule(object):
    """The filters this collection exposes to a playbook."""

    def filters(self):
        return {"deployment_order": deployment_order}
