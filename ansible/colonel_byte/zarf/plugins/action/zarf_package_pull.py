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

"""Action plugin for the zarf_package_pull module.

It exists to deliver the task's parameters to the wrapper binary. Everything it does is in
ZarfActionBase; see plugins/plugin_utils/projection.py.
"""

from __future__ import absolute_import, division, print_function

__metaclass__ = type

# The cli_flag key on each option is a cargoship extension, not a standard Ansible doc key. See
# the header of zarf_init.py and docs/agent/choice-zarf-ansible-module.md.
DOCUMENTATION = r"""
module: zarf_package_pull
short_description: Fetch a zarf package to disk
description:
  - "Runs C(zarf package pull) to fetch a package from an C(oci://) registry or an C(https://) URL and write it to a directory as a tarball."
  - "The module is a compiled binary with zarf linked into it. It renders the C(zarf package pull) command line from these parameters, executes zarf's own command in its own process, and reports one JSON result. It needs no C(zarf) on the node."
  - "It is the staging half of an air-gapped workflow: a connected node fetches the package, something carries the file through the airlock, and M(colonel_byte.zarf.zarf_package_deploy) reads it from the path it landed at. Nothing it does reaches a cluster."
version_added: "0.88.0"
author:
  - Allen Conlon (@colonel-byte)
notes:
  - "Set C(run_once: true) and C(delegate_to) on the task. The package is fetched once, to the node holding the staging directory."
  - "C(changed) is an observation here rather than a convention: the task reports changed only when the pull left a package file the output directory did not already hold, with C(zarf.changedSignal) set to C(complete) and the filenames in C(zarf.packagesPulled). Zarf derives the filename from the package's own metadata, so that list is how a playbook learns what to deploy."
  - "Check mode reports the task as skipped. C(zarf package pull) has no dry run."
  - "A C(package) that is a local path is refused: there is nothing to fetch. Use the path directly with M(colonel_byte.zarf.zarf_package_deploy)."
  - "A parameter left unset renders no flag, so whatever zarf's own config file sets still applies."
options:
  package:
    description:
      - "The package to fetch: an C(oci://) reference or an C(https://) URL."
      - "The positional argument of the command, so it renders no flag."
    type: str
    required: true
    cli_flag: None
  output_directory:
    description:
      - "The directory the tarball is written to. Zarf defaults it to the working directory, which under Ansible is whichever directory the module was invoked from, so a playbook that cares where the package lands says so."
    type: str
    required: false
    cli_flag: "--output-directory"
  shasum:
    description:
      - "The SHA256 checksum of the package. Zarf requires it for a package fetched over C(https://) rather than merely accepting it."
    type: str
    required: false
    cli_flag: "--shasum"
  zarf_config:
    description:
      - "A zarf config file, passed as C(ZARF_CONFIG). It is how a playbook keeps registry credentials out of the argument vector."
    type: str
    required: false
    cli_flag: None
  wrapper_binary:
    description:
      - "The module wrapper to run, resolved on the path when it holds no separator."
      - "The action plugin consumes this parameter; the wrapper never sees it."
    type: str
    required: false
    default: zarf_package_pull
    cli_flag: None
  parameters:
    description:
      - "Further module parameters, merged into the ones given directly."
      - "A parameter given both directly and inside C(parameters) is an error naming it."
      - "The action plugin consumes this key; the wrapper never sees it."
    type: dict
    required: false
    cli_flag: None
  oci_concurrency:
    description:
      - "How many OCI operations zarf runs at once."
    type: int
    required: false
    cli_flag: "--oci-concurrency"
  architecture:
    description:
      - "The architecture of the package to fetch, for a reference that publishes more than one."
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
- name: Stage a package for the airlock
  colonel_byte.zarf.zarf_package_pull:
    package: oci://ghcr.io/colonel-byte/zarf/csi-local-path-provider:0.0.37-upstream
    output_directory: /srv/staging
  delegate_to: localhost
  run_once: true
  register: pull_result

- name: Deploy what was staged
  colonel_byte.zarf.zarf_package_deploy:
    package: "/srv/staging/{{ pull_result.zarf.packagesPulled | first }}"
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true
  when: pull_result.zarf.packagesPulled | length > 0
"""

from ansible_collections.colonel_byte.zarf.plugins.plugin_utils.projection import (
    ZarfActionBase,
)


class ActionModule(ZarfActionBase):
    MODULE = "package_pull"
    DEFAULT_WRAPPER = "zarf_package_pull"
