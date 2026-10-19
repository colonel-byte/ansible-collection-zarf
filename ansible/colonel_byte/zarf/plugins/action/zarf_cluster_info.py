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

"""Action plugin for the zarf_cluster_info module.

It reports the four things a colonel-byte.dev/v1alpha1 PackagePrerequisites document can require
of a cluster: its Kubernetes version, the CRDs established on it, its StorageClasses, and the
readiness of its Deployments. The plumbing is in ZarfInfoActionBase; see
plugins/plugin_utils/info.py and docs/agent/choice-zarf-info-modules.md.

Everything is read through `zarf tools kubectl`, which is a whole vendored kubectl, rather than
through kubernetes.core. That keeps this collection's only requirement the zarf binary it already
requires: the management node a playbook runs this from is typically air-gapped, and a second
Galaxy collection to stage onto it is a cost paid by every consumer of this one. zarf_state_info
reads the zarf-state Secret the same way, and for the same reason.

The queries are digested rather than returned whole. A cluster with two hundred CRDs answers
`get crd -o json` with megabytes of OpenAPI schemas, none of which a prerequisite check reads, and
all of which would land in a registered variable and then in a fact. What comes back here is names
and counts.
"""


from __future__ import absolute_import, division, print_function

__metaclass__ = type

import json
import re

from ansible_collections.colonel_byte.zarf.plugins.plugin_utils.info import (
    ZarfInfoActionBase,
)

# The cli_flag key on each option is a cargoship extension, not a standard Ansible doc key. Here it
# is held against command_parts below by internal/zarfmod/infoplugins_test.go.

DOCUMENTATION = r"""
module: zarf_cluster_info
short_description: Report the cluster facts a package's prerequisites are checked against
description:
  - "Reads the cluster through C(zarf tools kubectl) and returns the facts a C(colonel-byte.dev/v1alpha1) C(PackagePrerequisites) document can require: the Kubernetes version, the established CRDs, the StorageClasses, and the readiness of every Deployment and StatefulSet."
  - "Needs no C(kubernetes.core): everything is read through the C(zarf) binary this collection already requires, which carries its own kubectl. The management node is typically air-gapped, and a second collection to stage onto it is a cost every consumer would pay."
  - "The answers are digested rather than returned whole. C(crds) is a list of names, not of objects: a cluster with two hundred CRDs answers with megabytes of schemas that no prerequisite check reads."
  - "Runs five kubectl queries. The first one to fail fails the task, and C(query) names which it was."
  - "Runs on the node the task is delegated to (typically localhost), reaching the cluster through a kubeconfig."
version_added: "0.33.0"
author:
  - Allen Conlon (@colonel-byte)
notes:
  - "Set C(run_once: true) and C(delegate_to: localhost) on the task."
  - "The module runs in check mode and reports C(changed: false), because reading a cluster changes nothing."
  - "C(requires.storage.accessModes) in a prerequisites document is not reported here and cannot be: what a provisioner will actually bind is not discoverable without creating a PersistentVolumeClaim, which is not a thing a read-only module does. The C(prerequisites) role skips that check and says so."
  - "C(deployments) and C(statefulsets) cover every namespace, and carry the same five fields each. A cluster running hundreds of workloads returns hundreds of entries."
  - "StatefulSets are reported separately because a GitOps engine is not all Deployments. Argo CD runs C(argocd-application-controller) as a StatefulSet and the rest of itself as Deployments, so a check that read only C(deployments) would report the component doing all of the reconciling as absent from a healthy cluster."
options:
  kubeconfig:
    description:
      - "Path to the kubeconfig file used to reach the cluster."
    type: path
    required: false
    cli_flag: None
  zarf_binary:
    description:
      - "Path to the C(zarf) CLI binary on the delegated host. When omitted, C(zarf) on C(PATH) is used."
    type: path
    default: "zarf"
    required: false
    cli_flag: None
"""

EXAMPLES = r"""
- name: Read the cluster's prerequisite-bearing facts
  colonel_byte.zarf.zarf_cluster_info:
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true
  register: cluster

- name: Require a cluster new enough for the bundle
  ansible.builtin.assert:
    that:
      - cluster.kubernetes_semver is version('1.30.0', '>=')
    fail_msg: "the cluster is {{ cluster.kubernetes_version }}, and the bundle needs 1.30.0 or newer"

- name: Require an available helm-controller
  ansible.builtin.assert:
    that:
      - cluster.deployments | selectattr('namespace', 'equalto', 'flux-system')
        | selectattr('name', 'equalto', 'helm-controller')
        | selectattr('available', 'ge', 1) | list | length > 0
    fail_msg: no available helm-controller, so a HelmRelease would never be reconciled

- name: Require an available argocd-application-controller, which is a StatefulSet
  ansible.builtin.assert:
    that:
      - cluster.statefulsets | selectattr('namespace', 'equalto', 'argocd')
        | selectattr('name', 'equalto', 'argocd-application-controller')
        | selectattr('available', 'ge', 1) | list | length > 0
    fail_msg: no available argocd-application-controller, so an Application would never be reconciled

- name: Report what is there
  ansible.builtin.debug:
    msg:
      kubernetes: "{{ cluster.kubernetes_version }}"
      defaultStorageClass: "{{ cluster.default_storage_class or 'none' }}"
      fluxCRDs: "{{ cluster.crds | select('search', 'toolkit.fluxcd.io$') | list | length }}"
"""

# The queries, and the name each one's output is collected under. The names are the result keys
# they populate, so that the "query" key on a failure names something the reader can look up in
# this module's own documentation.
QUERY_VERSION = "version"
QUERY_CRDS = "crds"
QUERY_STORAGE_CLASSES = "storage_classes"
QUERY_DEPLOYMENTS = "deployments"
QUERY_STATEFULSETS = "statefulsets"

# The annotation marking a StorageClass as the cluster's default, and the one older provisioners
# still set. Both are checked: a cluster installed years ago may carry only the beta spelling, and
# a package whose chart templates no storageClassName binds to whichever the cluster honours.
DEFAULT_CLASS_ANNOTATIONS = (
    "storageclass.kubernetes.io/is-default-class",
    "storageclass.beta.kubernetes.io/is-default-class",
)

# The label a GitOps engine's workloads -- and most charts following the Kubernetes recommended
# labels -- record their version under. It is how a workload's version is reported without asking
# the registry what tag its image resolves to.
#
# Neither Flux's nor Argo CD's upstream manifests set it; `flux install` adds it, and a package
# that vendors either has to add it too. A workload without it reports an empty version, which the
# prerequisites role names as unchecked rather than treating as a failed comparison.
VERSION_LABEL = "app.kubernetes.io/version"

# A Kubernetes gitVersion, reduced to the three numbers Ansible's version test compares. The
# distribution suffix is dropped rather than kept: k3s reports v1.31.2+k3s1 and RKE2 reports
# v1.31.2+rke2r1, and `is version('1.30.0', '>=')` on either is comparing a build identifier
# against nothing.
SEMVER_PREFIX = re.compile(r"^v?(\d+\.\d+\.\d+)")


def command_parts(params):
    """Return the zarf command line this module runs first.

    The module runs four commands rather than one, and command_parts returns the first of them:
    it is the hook internal/zarfmod/infoplugins_test.go reads flags out of, and every flag this
    module renders is here, because the other three render none that a parameter controls. See
    command_parts_all, which is what actually runs.

    --no-color is deliberately absent, as it is in zarf_state_info: `zarf tools kubectl` hands its
    arguments to kubectl, which answers "unknown flag: --no-color" rather than ignoring it.
    """
    return [params["zarf_binary"], "tools", "kubectl", "version", "-o", "json"]


def command_parts_all(params):
    """Return every zarf command line this module runs, as (name, argv) pairs.

    Four queries, because they are four kubectl commands and no flag makes them one. The order is
    the order they are reported in, and it puts the version first on purpose: it is the query that
    fails when the cluster is simply unreachable, and failing there gives the clearest message.

    The CRDs are fetched as `-o name` rather than `-o json`, which is the one place this module
    trades a little parsing for a lot of output: a CRD object carries the full OpenAPI schema of
    every version of its resource, and a cluster with a few operators on it answers `-o json` with
    tens of megabytes. `-o name` answers with one line per CRD.
    """
    zarf = params["zarf_binary"]
    kubectl = [zarf, "tools", "kubectl"]
    return [
        (QUERY_VERSION, command_parts(params)),
        (QUERY_CRDS, kubectl + ["get", "customresourcedefinitions.apiextensions.k8s.io", "-o", "name"]),
        (QUERY_STORAGE_CLASSES, kubectl + ["get", "storageclasses.storage.k8s.io", "-o", "json"]),
        (QUERY_DEPLOYMENTS, kubectl + ["get", "deployments.apps", "--all-namespaces", "-o", "json"]),
        # StatefulSets are a query of their own rather than a `get deploy,sts` that would save a
        # round trip, because the two answer with one List each and merging them would mean
        # telling them apart by a field kubectl omits from an item inside a List. Argo CD's
        # argocd-application-controller is a StatefulSet while the rest of Argo CD is Deployments,
        # so a role that looked only at Deployments would report the component that does all the
        # reconciling as absent from a perfectly healthy cluster.
        (QUERY_STATEFULSETS, kubectl + ["get", "statefulsets.apps", "--all-namespaces", "-o", "json"]),
    ]


def parse_version(stdout):
    """Return the cluster's reported version and the three numbers of it.

    `kubectl version -o json` reports both the client's version and the server's. The server's is
    the one a prerequisites document constrains -- the client is whichever kubectl zarf was built
    with, and says nothing about the cluster.
    """
    payload = json.loads(stdout)
    server = payload.get("serverVersion") or {}
    reported = server.get("gitVersion") or ""

    matched = SEMVER_PREFIX.match(reported)
    return reported, matched.group(1) if matched else ""


def parse_crd_names(stdout):
    """Return the CRD names in `kubectl get ... -o name` output.

    Each line is a fully qualified resource reference -- customresourcedefinition.apiextensions.
    k8s.io/helmreleases.helm.toolkit.fluxcd.io -- and the name a prerequisites document lists is
    everything after the slash.
    """
    names = []
    for line in stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        names.append(line.split("/", 1)[-1])
    return sorted(names)


def parse_storage_classes(stdout):
    """Return each StorageClass as a name, a provisioner and whether it is the cluster's default."""
    classes = []
    for item in json.loads(stdout).get("items") or []:
        metadata = item.get("metadata") or {}
        annotations = metadata.get("annotations") or {}
        default = any(
            str(annotations.get(annotation, "")).lower() == "true"
            for annotation in DEFAULT_CLASS_ANNOTATIONS
        )
        classes.append(
            {
                "name": metadata.get("name", ""),
                "provisioner": item.get("provisioner", ""),
                "default": default,
            }
        )
    return sorted(classes, key=lambda entry: entry["name"])


def parse_workloads(stdout):
    """Return each workload digested to what a controller requirement is checked against.

    One function for Deployments and StatefulSets, because the three fields a requirement is
    checked against -- availableReplicas, readyReplicas and the version label -- are spelled the
    same on both. What differs is only which query the document came from, which the caller knows.

    `available` is availableReplicas rather than readyReplicas, because availability is what a
    minAvailable requirement means: a replica that is ready but not yet available is one that
    passed its probes and has not satisfied minReadySeconds, and a controller in that state is
    about to be restarted by its own rollout.

    Both default to zero. Kubernetes omits either field entirely while a Deployment has none, so a
    missing key and a zero are the same fact and the result should not make a playbook handle them
    as two.
    """
    workloads = []
    for item in json.loads(stdout).get("items") or []:
        metadata = item.get("metadata") or {}
        status = item.get("status") or {}
        labels = metadata.get("labels") or {}
        workloads.append(
            {
                "name": metadata.get("name", ""),
                "namespace": metadata.get("namespace", ""),
                "available": int(status.get("availableReplicas") or 0),
                "ready": int(status.get("readyReplicas") or 0),
                # The empty string rather than a missing key: a chart that sets no version label is
                # ordinary, and a requirement that names a version for a controller without one
                # should fail the comparison rather than the template.
                "version": str(labels.get(VERSION_LABEL, "")),
            }
        )
    return sorted(workloads, key=lambda entry: (entry["namespace"], entry["name"]))


class ActionModule(ZarfInfoActionBase):
    """Action plugin that reports the cluster facts a prerequisites document is checked against."""

    def zarf_argvs(self, params):
        return command_parts_all(params)

    def interpret_outputs(self, result, outputs, params):
        del params  # every query is unparameterised; the kubeconfig reaches zarf as an environment

        try:
            reported, semver = parse_version(outputs[QUERY_VERSION])
            crds = parse_crd_names(outputs[QUERY_CRDS])
            storage_classes = parse_storage_classes(outputs[QUERY_STORAGE_CLASSES])
            deployments = parse_workloads(outputs[QUERY_DEPLOYMENTS])
            statefulsets = parse_workloads(outputs[QUERY_STATEFULSETS])
        except (ValueError, TypeError) as exc:
            # ValueError covers json.JSONDecodeError, which is a subclass of it.
            result["failed"] = True
            result["msg"] = "unable to parse what kubectl reported about the cluster: %s" % exc
            return

        # The first class annotated as default, and the empty string when none is. A cluster with
        # two annotated as default is a misconfiguration Kubernetes permits and resolves
        # arbitrarily; `storage_classes` carries the `default` flag of every one of them, so a
        # playbook that needs to see the ambiguity can.
        defaults = [entry["name"] for entry in storage_classes if entry["default"]]

        result["kubernetes_version"] = reported
        result["kubernetes_semver"] = semver
        result["crds"] = crds
        result["storage_classes"] = storage_classes
        result["default_storage_class"] = defaults[0] if defaults else ""
        result["deployments"] = deployments
        result["statefulsets"] = statefulsets
        result["msg"] = (
            "cluster is %s with %d CRDs, %d storage classes, %d deployments and %d statefulsets" % (
                reported or "of an unreported version",
                len(crds),
                len(storage_classes),
                len(deployments),
                len(statefulsets),
            )
        )
