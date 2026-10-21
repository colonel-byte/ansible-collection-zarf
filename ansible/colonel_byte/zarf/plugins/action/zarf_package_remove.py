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

"""Action plugin for the zarf_package_remove module.

It exists to deliver the task's parameters to the wrapper binary. Everything it does is in
ZarfActionBase; see plugins/plugin_utils/projection.py.
"""

from __future__ import absolute_import, division, print_function

__metaclass__ = type

# The cli_flag key on each option is a cargoship extension, not a standard Ansible doc key. See
# the header of zarf_init.py and docs/agent/choice-zarf-ansible-module.md.
DOCUMENTATION = r"""
module: zarf_package_remove
short_description: Remove a deployed zarf package from a cluster
description:
  - "Runs C(zarf package remove) against the cluster a kubeconfig names, taking a deployed package -- or some of its components -- off it."
  - "The module is a compiled binary with zarf linked into it. It renders the C(zarf package remove) command line from these parameters, executes zarf's own command in its own process, and reports one JSON result. It needs no C(zarf) on the node."
  - "It is the other half of M(colonel_byte.zarf.zarf_package_deploy). A playbook that can only deploy is not reconciling a cluster, and removing a component of a package while leaving the rest is the one thing a deploy cannot undo."
version_added: "0.88.0"
author:
  - Allen Conlon (@colonel-byte)
notes:
  - "Set C(run_once: true) and C(delegate_to) on the task. One removal takes the package off the whole cluster, so a play over the cluster's own inventory would otherwise run it once per host."
  - "C(--confirm) is always rendered. There is no terminal on the other end of a playbook task to confirm at, and this is the command where that matters most: the task itself is the confirmation that something should be deleted."
  - "The task reports C(changed: true) with C(zarf.changedSignal) set to C(unknown). Zarf logs a removal as one line and names no component on the way, so there is nothing that distinguishes a package that was there from one that was already gone -- a removal that finds nothing fails in zarf, which is the only signal there is."
  - "Check mode reports the task as skipped. C(zarf package remove) has no dry run, and removing a package during a run an operator asked to be told about is worse than deploying one."
  - "A parameter left unset renders no flag, so whatever zarf's own config file sets still applies."
options:
  package:
    description:
      - "What to remove: the deployed package's name as C(zarf package list) reports it, a path to the tarball it was deployed from, or an C(oci://) reference."
      - "The name is usually what a playbook wants, because the tarball may be long gone by the time the package is removed."
      - "The positional argument of the command, so it renders no flag."
    type: str
    required: true
    cli_flag: None
  kubeconfig:
    description:
      - "The kubeconfig of the cluster to remove from."
      - "Zarf has no C(--kubeconfig) flag, so it is passed as C(KUBECONFIG) in zarf's environment."
    type: str
    required: false
    cli_flag: None
  zarf_config:
    description:
      - "A zarf config file, passed as C(ZARF_CONFIG)."
    type: str
    required: false
    cli_flag: None
  wrapper_binary:
    description:
      - "The module wrapper to run, resolved on the path when it holds no separator."
      - "The action plugin consumes this parameter; the wrapper never sees it."
    type: str
    required: false
    default: zarf_package_remove
    cli_flag: None
  parameters:
    description:
      - "Further module parameters, merged into the ones given directly."
      - "A parameter given both directly and inside C(parameters) is an error naming it."
      - "The action plugin consumes this key; the wrapper never sees it."
    type: dict
    required: false
    cli_flag: None
  components:
    description:
      - "Which of the package's components to remove, as a comma-separated list. Omitted, the whole package goes."
    type: str
    required: false
    cli_flag: "--components"
  namespace:
    description:
      - "The namespace the package was deployed into, for a package deployed with M(colonel_byte.zarf.zarf_package_deploy)'s C(namespace). Alpha in zarf."
    type: str
    required: false
    cli_flag: "--namespace"
  values:
    description:
      - "Values files for the package's on-remove actions, one flag per entry."
    type: list
    elements: str
    required: false
    cli_flag: "--values"
  set_values:
    description:
      - "Values for the package's on-remove actions, as a mapping. Rendered one flag per entry, sorted, so the same parameters always render the same command line."
    type: dict
    required: false
    cli_flag: "--set-values"
  oci_concurrency:
    description:
      - "How many OCI operations zarf runs at once, for a package named by an C(oci://) reference."
    type: int
    required: false
    cli_flag: "--oci-concurrency"
  architecture:
    description:
      - "The architecture of the package being removed."
    type: str
    required: false
    cli_flag: "--architecture"
  cache:
    description:
      - "Zarf's cache directory."
    type: str
    required: false
    cli_flag: "--cache"
  tmpdir:
    description:
      - "The directory zarf unpacks into while it works."
    type: str
    required: false
    cli_flag: "--tmpdir"
  plain_http:
    description:
      - "Whether zarf talks to OCI registries over plain HTTP."
    type: bool
    required: false
    cli_flag: "--plain-http"
  insecure_skip_tls_verify:
    description:
      - "Whether zarf skips verifying TLS certificates it connects to."
    type: bool
    required: false
    cli_flag: "--insecure-skip-tls-verify"
  log_level:
    description:
      - "Zarf's log level."
    type: str
    required: false
    cli_flag: "--log-level"
  log_format:
    description:
      - "Zarf's log format. Defaults to C(json)."
    type: str
    required: false
    default: json
    cli_flag: "--log-format"
"""

EXAMPLES = r"""
- name: Remove a deployed package by name
  colonel_byte.zarf.zarf_package_remove:
    package: csi-local-path-provider
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true

- name: Remove one component and leave the rest of the package in place
  colonel_byte.zarf.zarf_package_remove:
    package: monitoring
    components: grafana-dashboards
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true
  register: remove_result

- name: Report the command that ran
  ansible.builtin.debug:
    var: remove_result.zarf.command
"""

from ansible_collections.colonel_byte.zarf.plugins.plugin_utils.projection import (
    ZarfActionBase,
)


class ActionModule(ZarfActionBase):
    MODULE = "package_remove"
    DEFAULT_WRAPPER = "zarf_package_remove"
