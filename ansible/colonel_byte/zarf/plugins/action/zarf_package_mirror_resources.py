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

"""Action plugin for the zarf_package_mirror_resources module.

It exists to deliver the task's parameters to the wrapper binary. Everything it does is in
ZarfActionBase; see plugins/plugin_utils/projection.py.
"""

from __future__ import absolute_import, division, print_function

__metaclass__ = type

# The cli_flag key on each option is a cargoship extension, not a standard Ansible doc key. See
# the header of zarf_init.py and docs/agent/choice-zarf-ansible-module.md.
DOCUMENTATION = r"""
module: zarf_package_mirror_resources
short_description: Push a package's images and repositories into a registry and git server
description:
  - "Runs C(zarf package mirror-resources), which pushes the images and git repositories a package carries into a registry and a git server without deploying anything."
  - "The module is a compiled binary with zarf linked into it. It renders the C(zarf package mirror-resources) command line from these parameters, executes zarf's own command in its own process, and reports one JSON result. It needs no C(zarf) on the node."
  - "It is the air-gap seeding step. A cluster that deploys with C(connected: true) pulls the images itself, which means its registry has to hold them first, and before this module a playbook had no way to say so."
version_added: "0.88.0"
author:
  - Allen Conlon (@colonel-byte)
notes:
  - "Set C(run_once: true) and C(delegate_to) on the task. One mirror fills the registry the whole cluster pulls from."
  - "Set C(no_log: true) on the task when any push credential is given as a parameter, or name a zarf config file with C(zarf_config) and keep them out of the argument vector."
  - "The task reports C(changed: true) with C(zarf.changedSignal) set to C(unknown), and names no components. Unlike a deploy, a mirror announces the resources it pushes rather than the component they came from -- C(mirroring images), C(pushing image) -- so there is no component list to report and nothing that says whether the registry already held what was pushed."
  - "Check mode reports the task as skipped. The command has no dry run."
  - "C(images) and C(repos) are mutually exclusive, as they are in zarf. Setting both is an error naming them; leaving both unset mirrors everything."
  - "A parameter left unset renders no flag, so whatever zarf's own config file sets still applies."
options:
  package:
    description:
      - "The package whose resources are mirrored: a path to a tarball, an C(oci://) reference, or an C(https://) URL."
      - "The positional argument of the command, so it renders no flag."
    type: str
    required: true
    cli_flag: None
  kubeconfig:
    description:
      - "The kubeconfig of the cluster whose registry and git server are the destination, for a mirror that reads them out of the cluster's zarf state rather than being given them below."
      - "Zarf has no C(--kubeconfig) flag, so it is passed as C(KUBECONFIG) in zarf's environment."
    type: str
    required: false
    cli_flag: None
  zarf_config:
    description:
      - "A zarf config file, passed as C(ZARF_CONFIG). It is how a playbook keeps the push credentials out of the argument vector."
    type: str
    required: false
    cli_flag: None
  wrapper_binary:
    description:
      - "The module wrapper to run, resolved on the path when it holds no separator."
      - "The action plugin consumes this parameter; the wrapper never sees it."
    type: str
    required: false
    default: zarf_package_mirror_resources
    cli_flag: None
  parameters:
    description:
      - "Further module parameters, merged into the ones given directly."
      - "A parameter given both directly and inside C(parameters) is an error naming it."
      - "The action plugin consumes this key; the wrapper never sees it."
    type: dict
    required: false
    cli_flag: None
  images:
    description:
      - "Mirror only the images. Mutually exclusive with C(repos)."
    type: bool
    required: false
    cli_flag: "--images"
  repos:
    description:
      - "Mirror only the git repositories. Mutually exclusive with C(images)."
    type: bool
    required: false
    cli_flag: "--repos"
  components:
    description:
      - "Which of the package's components to mirror, as a comma-separated list."
    type: str
    required: false
    cli_flag: "--components"
  registry_url:
    description:
      - "The registry to push the images to, for a destination that is not the one the cluster's zarf state names."
    type: str
    required: false
    cli_flag: "--registry-url"
  registry_push_username:
    description:
      - "The registry's push username."
    type: str
    required: false
    cli_flag: "--registry-push-username"
  registry_push_password:
    description:
      - "The registry's push password."
    type: str
    required: false
    cli_flag: "--registry-push-password"
  git_url:
    description:
      - "The git server to push the repositories to."
    type: str
    required: false
    cli_flag: "--git-url"
  git_push_username:
    description:
      - "The git server's push username."
    type: str
    required: false
    cli_flag: "--git-push-username"
  git_push_password:
    description:
      - "The git server's push password."
    type: str
    required: false
    cli_flag: "--git-push-password"
  shasum:
    description:
      - "The SHA256 checksum of a package fetched over C(https://)."
    type: str
    required: false
    cli_flag: "--shasum"
  no_img_checksum:
    description:
      - "Whether to push the images without their checksum tags."
    type: bool
    required: false
    cli_flag: "--no-img-checksum"
  public_key:
    description:
      - "The public key the package signature is verified against."
    type: str
    required: false
    cli_flag: "--key"
  verify:
    description:
      - "When to verify the package signature."
    type: str
    required: false
    choices:
      - never
      - if-possible
      - always
    cli_flag: "--verify"
  skip_signature_validation:
    description:
      - "Whether zarf skips validating the package's signature altogether."
    type: bool
    required: false
    cli_flag: "--skip-signature-validation"
  retries:
    description:
      - "How many times zarf retries a failed push."
    type: int
    required: false
    cli_flag: "--retries"
  oci_concurrency:
    description:
      - "How many OCI operations zarf runs at once."
    type: int
    required: false
    cli_flag: "--oci-concurrency"
  log_level:
    description:
      - "Zarf's log level."
    type: str
    required: false
    cli_flag: "--log-level"
  log_format:
    description:
      - "Zarf's log format. Defaults to C(json), which is the format the component progress is read out of."
    type: str
    required: false
    default: json
    cli_flag: "--log-format"
"""

EXAMPLES = r"""
- name: Seed the cluster's registry so a connected deploy can find the images
  colonel_byte.zarf.zarf_package_mirror_resources:
    package: /srv/staging/zarf-package-monitoring-amd64.tar.zst
    kubeconfig: /etc/rancher/rke2/rke2.yaml
    public_key: /etc/zarf/colonel-byte-zarf-packages.pub
    verify: always
  delegate_to: localhost
  run_once: true
  register: mirror_result

- name: Deploy without pushing the images again
  colonel_byte.zarf.zarf_package_deploy:
    package: /srv/staging/zarf-package-monitoring-amd64.tar.zst
    connected: true
    kubeconfig: /etc/rancher/rke2/rke2.yaml
  delegate_to: localhost
  run_once: true

- name: Push only the images, to a registry outside the cluster
  colonel_byte.zarf.zarf_package_mirror_resources:
    package: oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0
    images: true
    registry_url: registry.bubbles.test
    registry_push_username: zarf-push
    registry_push_password: "{{ registry_password }}"
  delegate_to: localhost
  run_once: true
  no_log: true

- name: Report the command the mirror ran
  ansible.builtin.debug:
    var: mirror_result.zarf.command
"""

from ansible_collections.colonel_byte.zarf.plugins.plugin_utils.projection import (
    ZarfActionBase,
)


class ActionModule(ZarfActionBase):
    MODULE = "package_mirror_resources"
    DEFAULT_WRAPPER = "zarf_package_mirror_resources"
