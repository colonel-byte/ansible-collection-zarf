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

"""Action plugin for the zarf_kubectl module.

It exists to deliver the task's parameters to the wrapper binary. Everything it does is in
ZarfActionBase; see plugins/plugin_utils/projection.py.
"""

from __future__ import absolute_import, division, print_function

__metaclass__ = type

# The cli_flag key on each option is a cargoship extension, not a standard Ansible doc key. See
# the header of zarf_init.py and docs/agent/choice-zarf-ansible-module.md.
DOCUMENTATION = r"""
module: zarf_kubectl
short_description: Run a kubectl command through the kubectl zarf carries
description:
  - "Runs C(zarf tools kubectl) with the arguments given, against the cluster a kubeconfig names, and reports what it wrote to standard output."
  - "The module is a compiled binary with zarf linked into it, and zarf vendors kubectl. It needs no C(zarf) and no C(kubectl) on the node, which is what it is for: a management node that drives an air-gapped cluster can read and apply resources without a kubectl of its own."
  - "It is a passthrough and renders nothing into C(args). Zarf already vendors kubectl and already resolves which cluster to talk to, so projecting kubectl's flags into Ansible parameters would be reimplementing kubectl in YAML. For declarative resource management prefer M(kubernetes.core.k8s), which this is not a replacement for."
version_added: "0.88.0"
author:
  - Allen Conlon (@colonel-byte)
notes:
  - "Set C(run_once: true) and C(delegate_to) on the task. One command runs against the whole cluster, so a play over the cluster's own inventory would otherwise run it once per host."
  - "The task always reports C(changed: true), with C(zarf.changedSignal) set to C(unknown). The argument vector is kubectl's, so the module knows the command succeeded and nothing about whether it altered anything. Set C(changed_when: false) on a read."
  - "Check mode reports the task as skipped, for the same reason: nothing here can tell a C(get) from an C(apply)."
  - "Standard output comes back as C(zarf.output), with its trailing newline removed. Use C(-o json) or C(-o yaml) and C(from_json) or C(from_yaml) when a playbook has to read the result rather than show it."
  - "A failure comes back as C(failed) with kubectl's own message in C(msg) and its exit status in C(zarf.exitCode), including the failures kubectl would normally exit the process over -- a cluster it cannot reach is one."
  - "A command kubectl does not have is refused by name before anything runs, rather than printing kubectl's help and reporting success."
  - "kubectl plugins are not reachable, deliberately: the handler that resolves them can replace this process with the plugin, or exit it without a result."
options:
  args:
    description:
      - "The kubectl argument vector, as a list: C(['get', 'pods', '-A'])."
      - "A list rather than a command line, because a shell is what splits a command line into words and there is none here. A quoted selector stays one entry."
      - "Positional, so it renders no flag."
    type: list
    elements: str
    required: true
    cli_flag: None
  kubeconfig:
    description:
      - "The kubeconfig of the cluster to act on."
      - "Zarf has no C(--kubeconfig) flag, so it is passed as C(KUBECONFIG) in the environment the command runs in. kubectl's own C(--kubeconfig) works too, in C(args)."
    type: str
    required: false
    cli_flag: None
  wrapper_binary:
    description:
      - "The module wrapper to run, resolved on the path when it holds no separator."
      - "The action plugin consumes this parameter; the wrapper never sees it."
    type: str
    required: false
    default: zarf_kubectl
    cli_flag: None
  parameters:
    description:
      - "Further module parameters, merged into the ones given directly."
      - "A parameter given both directly and inside C(parameters) is an error naming it."
      - "The action plugin consumes this key; the wrapper never sees it."
    type: dict
    required: false
    cli_flag: None
"""

EXAMPLES = r"""
- name: Read the nodes of an air-gapped cluster without installing kubectl
  colonel_byte.zarf.zarf_kubectl:
    args:
      - get
      - nodes
      - -o
      - json
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true
  changed_when: false
  register: nodes_result

- name: Report how many nodes are ready
  ansible.builtin.debug:
    msg: "{{ (nodes_result.zarf.output | from_json)['items'] | length }} nodes"

- name: Restart the zarf agent after rotating its certificate
  colonel_byte.zarf.zarf_kubectl:
    args:
      - rollout
      - restart
      - deployment/agent-hook
      - -n
      - zarf
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true
"""

from ansible_collections.colonel_byte.zarf.plugins.plugin_utils.projection import (
    ZarfActionBase,
)


class ActionModule(ZarfActionBase):
    MODULE = "kubectl"
    DEFAULT_WRAPPER = "zarf_kubectl"
