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

"""Action plugin for the zarf_update_creds module.

It exists to deliver the task's parameters to the wrapper binary. Everything it does is in
ZarfActionBase; see plugins/plugin_utils/projection.py.
"""

from __future__ import absolute_import, division, print_function

__metaclass__ = type

# The cli_flag key on each option is a cargoship extension, not a standard Ansible doc key. See
# the header of zarf_init.py and docs/agent/choice-zarf-ansible-module.md.
DOCUMENTATION = r"""
module: zarf_update_creds
short_description: Rotate the credentials an initialised cluster holds
description:
  - "Runs C(zarf tools update-creds <service>) against the cluster a kubeconfig names, rewriting the credentials it holds for its registry, its git server or its agent."
  - "The module is a compiled binary with zarf linked into it. It renders the C(zarf tools update-creds) command line from these parameters, executes zarf's own command in its own process, and reports one JSON result. It needs no C(zarf) on the node."
  - "It is the write half of M(colonel_byte.zarf.zarf_state_info), which reads the same C(zarf-state) Secret and withholds what it finds. Rotating a password without it means doing in several steps what zarf does in one: the Secret, the service's own configuration, and every deployed package that pulls through it."
  - "Zarf has one subcommand per service, each taking only that service's credentials, and the bare C(zarf tools update-creds) that takes all of them warns that it is deprecated. This module renders the subcommand, so C(service) is required and a credential given for another service is an error naming it."
version_added: "0.88.0"
author:
  - Allen Conlon (@colonel-byte)
notes:
  - "Set C(run_once: true) and C(delegate_to) on the task. The credentials belong to the cluster, so a play over its own inventory would otherwise rotate them once per host."
  - "Set C(no_log: true) on the task. Every parameter here but C(service) is a credential or a path to one, and a binary module has no per-parameter C(no_log). Better still, put them in a zarf config file and name it with C(zarf_config), which keeps them out of the argument vector and so out of the process table."
  - "C(--confirm) is always rendered: there is no terminal on the other end of a playbook task, and the command asks before it rewrites anything."
  - "The task reports C(changed: true) with C(zarf.changedSignal) set to C(unknown). Zarf reports that it updated the services, not whether any of them already held what was given, so a credential set twice is reported as changed both times."
  - "Check mode reports the task as skipped. The command has no dry run."
  - "The artifact server is the fourth service zarf keeps credentials for and has no subcommand of its own. It is deprecated upstream, so this module does not update it."
  - "One service per task: a playbook rotating the registry and the git server writes two tasks."
options:
  service:
    description:
      - "Which service's credentials to rewrite. It selects the zarf subcommand, so it is required: each subcommand accepts only its own service's credentials."
      - "The subcommand name, so it renders no flag."
    type: str
    required: true
    choices:
      - registry
      - git
      - agent
    cli_flag: None
  kubeconfig:
    description:
      - "The kubeconfig of the cluster whose credentials are being rewritten."
      - "Zarf has no C(--kubeconfig) flag, so it is passed as C(KUBECONFIG) in zarf's environment."
    type: str
    required: false
    cli_flag: None
  zarf_config:
    description:
      - "A zarf config file, passed as C(ZARF_CONFIG). It is how a playbook keeps the new credentials out of the argument vector entirely."
    type: str
    required: false
    cli_flag: None
  wrapper_binary:
    description:
      - "The module wrapper to run, resolved on the path when it holds no separator."
      - "The action plugin consumes this parameter; the wrapper never sees it."
    type: str
    required: false
    default: zarf_update_creds
    cli_flag: None
  parameters:
    description:
      - "Further module parameters, merged into the ones given directly."
      - "A parameter given both directly and inside C(parameters) is an error naming it."
      - "The action plugin consumes this key; the wrapper never sees it."
    type: dict
    required: false
    cli_flag: None
  registry_url:
    description:
      - "The registry's new address. Taken by C(service: registry)."
    type: str
    required: false
    cli_flag: "--registry-url"
  registry_push_username:
    description:
      - "The registry's new push username."
    type: str
    required: false
    cli_flag: "--registry-push-username"
  registry_push_password:
    description:
      - "The registry's new push password."
    type: str
    required: false
    cli_flag: "--registry-push-password"
  registry_pull_username:
    description:
      - "The registry's new pull username."
    type: str
    required: false
    cli_flag: "--registry-pull-username"
  registry_pull_password:
    description:
      - "The registry's new pull password."
    type: str
    required: false
    cli_flag: "--registry-pull-password"
  git_url:
    description:
      - "The git server's new address. Taken by C(service: git)."
    type: str
    required: false
    cli_flag: "--git-url"
  git_push_username:
    description:
      - "The git server's new push username."
    type: str
    required: false
    cli_flag: "--git-push-username"
  git_push_password:
    description:
      - "The git server's new push password."
    type: str
    required: false
    cli_flag: "--git-push-password"
  git_pull_username:
    description:
      - "The git server's new pull username."
    type: str
    required: false
    cli_flag: "--git-pull-username"
  git_pull_password:
    description:
      - "The git server's new pull password."
    type: str
    required: false
    cli_flag: "--git-pull-password"
  agent_tls_ca:
    description:
      - "A PEM-encoded CA certificate for the zarf agent. Taken by C(service: agent)."
    type: str
    required: false
    cli_flag: "--agent-tls-ca"
  agent_tls_cert:
    description:
      - "A PEM-encoded TLS certificate for the zarf agent."
    type: str
    required: false
    cli_flag: "--agent-tls-cert"
  agent_tls_key:
    description:
      - "A PEM-encoded TLS private key for the zarf agent."
    type: str
    required: false
    cli_flag: "--agent-tls-key"
  force_conflicts:
    description:
      - "Whether zarf forces Helm ownership during server-side apply, for the deployed packages it updates."
      - "Taken by C(service: registry) and C(service: agent). The git subcommand does not have it -- it patches no Helm-owned resource -- so setting it with C(service: git) is an error naming it."
    type: bool
    required: false
    cli_flag: "--force-conflicts"
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
- name: Rotate the registry's push password
  colonel_byte.zarf.zarf_update_creds:
    service: registry
    registry_push_password: "{{ new_registry_password }}"
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true
  no_log: true

- name: Rotate the git credentials from a zarf config file, keeping them out of the process table
  colonel_byte.zarf.zarf_update_creds:
    service: git
    zarf_config: /etc/zarf/rotation.yaml
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true

- name: Install a new agent certificate
  colonel_byte.zarf.zarf_update_creds:
    service: agent
    agent_tls_ca: /etc/zarf/ca.pem
    agent_tls_cert: /etc/zarf/tls.pem
    agent_tls_key: /etc/zarf/tls.key
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true
  no_log: true
"""

from ansible_collections.colonel_byte.zarf.plugins.plugin_utils.projection import (
    ZarfActionBase,
)


class ActionModule(ZarfActionBase):
    MODULE = "update_creds"
    DEFAULT_WRAPPER = "zarf_update_creds"
