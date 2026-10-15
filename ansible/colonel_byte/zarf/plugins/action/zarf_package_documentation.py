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

"""Action plugin for the zarf_package_documentation module.

It extracts the documentation layer of a package tarball, an OCI reference, or a deployed package,
and returns what the files contain. The plumbing is in ZarfInfoActionBase; see
plugins/plugin_utils/info.py and docs/agent/choice-zarf-info-modules.md.

This module is the one that makes a colonel-byte.dev/v1alpha1 PackagePrerequisites document
reachable from a playbook. zarf_package_inspect returns the package's `documentation` mapping, but
a mapping of names to paths inside a tarball is not something a playbook can read -- the file has
to come out of the package first, and `zarf package inspect documentation` is what does that. See
docs/guides/package-prerequisites.md and docs/agent/choice-package-documentation-module.md.
"""


from __future__ import absolute_import, division, print_function

__metaclass__ = type

import os
import shutil
import tempfile

import yaml

from ansible_collections.colonel_byte.zarf.plugins.plugin_utils.info import (
    ZarfInfoActionBase,
)

# The cli_flag key on each option is a cargoship extension, not a standard Ansible doc key. Here it
# is held against command_parts below by internal/zarfmod/infoplugins_test.go.

DOCUMENTATION = r"""
module: zarf_package_documentation
short_description: Extract and parse the documentation a Zarf package carries
description:
  - "Runs C(zarf package inspect documentation) against a local package tarball, an C(oci://) reference, or a deployed package name, and returns the extracted files."
  - "Zarf packages every file named in its C(zarf.yaml)'s top-level C(documentation:) mapping into a documentation layer, which this module extracts without deploying anything."
  - "Each extracted file is returned as text in C(documents), keyed by its documentation key, and parsed into C(parsed) when it is YAML or JSON. A file that is neither is returned as text only, rather than failing the task."
  - "The C(prerequisites) key is the one this collection has a format for. When the package carries one and it declares C(apiVersion: colonel-byte.dev/v1alpha1) and C(kind: PackagePrerequisites), it is also returned as C(prerequisites); see the C(prerequisites) role, which checks a cluster against it."
  - "Signature verification is zarf's C(if-possible) unless C(verify) says otherwise, so an unsigned package extracts cleanly even when C(public_key) is given."
  - "Runs on the node the task is delegated to (typically localhost)."
version_added: "0.33.0"
author:
  - Allen Conlon (@colonel-byte)
notes:
  - "Set C(run_once: true) and C(delegate_to: localhost) on the task."
  - "The module runs in check mode and reports C(changed: false). It writes files, but only the ones it extracts, and only under C(destination) or a temporary directory of its own - nothing on the cluster or in the package changes."
  - "Left unset, C(destination) is a temporary directory that is removed before the task returns, so C(files) names paths that no longer exist. Set it when a later task has to read a file rather than its contents."
options:
  package:
    description:
      - "Path to a local package tarball, an C(oci://) reference, an C(https://) URL, or a deployed package name."
    type: str
    required: true
    cli_flag: None
  keys:
    description:
      - "The documentation keys to extract. Left unset, every key the package carries is extracted."
      - "Naming a key the package does not carry is not an error: zarf extracts what it has, and the key is simply absent from C(documents)."
    type: list
    elements: str
    required: false
    cli_flag: "--keys"
  destination:
    description:
      - "Directory to extract into. Zarf creates a C(<package-name>-documentation) subdirectory of it, and C(directory) reports the subdirectory it created."
      - "Left unset, a temporary directory is used and removed before the task returns."
    type: path
    required: false
    cli_flag: "--output"
  public_key:
    description:
      - "Path to the public key a package signature is verified against."
      - "Supplying it does not by itself require a signature: zarf verifies when it can and passes an unsigned package otherwise. Set C(verify: always) to reject one."
    type: path
    required: false
    cli_flag: "--key"
  verify:
    description:
      - "When to verify the package signature. Left unset, zarf's own default applies, which is C(if-possible)."
    type: str
    required: false
    choices:
      - never
      - if-possible
      - always
    cli_flag: "--verify"
  kubeconfig:
    description:
      - "Path to the kubeconfig file, used when extracting from a package already deployed to a cluster."
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
- name: Read the prerequisites a package declares
  colonel_byte.zarf.zarf_package_documentation:
    package: /srv/staging/zarf-package-operator-external-secrets-amd64-2.12.0.tar.zst
    keys:
      - prerequisites
    public_key: /etc/zarf/colonel-byte-zarf-packages.pub
    verify: always
  delegate_to: localhost
  run_once: true
  register: pkg_docs

- name: Report the Flux version the package requires
  ansible.builtin.debug:
    msg: "{{ pkg_docs.prerequisites.metadata.package }} requires flux {{ pkg_docs.prerequisites.requires.fluxcd.version }}"
  when: pkg_docs.prerequisites is defined

- name: Keep the extracted documentation for a later task to read
  colonel_byte.zarf.zarf_package_documentation:
    package: oci://ghcr.io/colonel-byte/packages/app-fluxcd:2.9.5
    destination: /srv/staging/documentation
  delegate_to: localhost
  run_once: true
  register: pkg_docs

- name: Show where it landed
  ansible.builtin.debug:
    msg: "{{ pkg_docs.directory }} holds {{ pkg_docs.files | dict2items | map(attribute='value') | list | join(', ') }}"
"""

# The apiVersion and kind a prerequisites document must declare to be returned as one. They are
# the constants in api/colonel-byte.dev/v1alpha1/prerequisites, repeated here because Python
# cannot read a Go constant; the module fails no task over a mismatch, it simply does not claim a
# document is something it is not.
PREREQUISITES_API_VERSION = "colonel-byte.dev/v1alpha1"
PREREQUISITES_KIND = "PackagePrerequisites"

# The documentation key a prerequisites document is carried under, by convention of this format
# rather than by anything zarf enforces.
PREREQUISITES_KEY = "prerequisites"

# The suffixes worth handing to the YAML parser. YAML is a superset of JSON, so one parser covers
# both; anything else -- a README, an architecture diagram -- is returned as text and not parsed.
# Parsing by suffix rather than attempting every file is deliberate: yaml.safe_load succeeds on a
# plain Markdown document and returns the whole thing as one string, which looks like a parse and
# is not.
PARSED_SUFFIXES = (".yaml", ".yml", ".json")

# The subdirectory zarf creates under --output. It names the package, which this module does not
# know before it runs, so the directory is found by this suffix rather than spelled out.
EXTRACTED_SUFFIX = "-documentation"


def command_parts(params):
    """Return the zarf command line this module runs.

    A plain function of the parameters, and the only place a flag is named, so that
    internal/zarfmod/infoplugins_test.go can read the flags the module renders out of the source
    and hold them against the documented cli_flag values.

    It renders three flags that a parameter controls, which is more than any other read-only module
    here. docs/agent/choice-package-documentation-module.md records why that did not make it a
    wrapper binary, and what covers it instead.
    """
    parts = [
        params["zarf_binary"],
        "package",
        "inspect",
        "documentation",
        params["package"],
        # Nothing parses this command's stdout -- the output is files -- but zarf logs to the
        # stream Ansible captures, and colour codes in a failure message are noise in a playbook's
        # output.
        "--no-color",
        "--output",
        params["destination"],
    ]
    if params.get("keys"):
        parts.extend(["--keys", ",".join(str(key) for key in params["keys"])])
    if params.get("public_key"):
        parts.extend(["--key", params["public_key"]])
    if params.get("verify"):
        # --verify=always, never --verify always. It is a flag with a no-argument default, so pflag
        # reads it the way it reads a boolean: the space form leaves the mode as a positional
        # argument and zarf rejects the command for having one too many. See
        # docs/agent/choice-zarf-ansible-module.md.
        parts.append("--verify=%s" % params["verify"])
    return parts


def _extracted_directory(destination):
    """Return the directory zarf extracted into, under the destination it was given.

    Zarf names it after the package, which this module does not know before running: the package
    parameter may be a tarball path, an OCI reference or a deployed name, and only the first of
    those carries the name in a parseable position. So the directory is found rather than
    predicted, and the destination is one this module owns, which is what makes exactly one match
    the expected outcome.
    """
    candidates = []
    for name in sorted(os.listdir(destination)):
        path = os.path.join(destination, name)
        if os.path.isdir(path) and name.endswith(EXTRACTED_SUFFIX):
            candidates.append(path)

    if len(candidates) == 1:
        return candidates[0]
    if not candidates:
        # A package carrying no documentation at all: zarf creates nothing and says so. Reporting
        # the destination keeps `directory` a real path, and `documents` is empty, which is what a
        # caller checks.
        return destination
    raise ValueError(
        "zarf extracted %d documentation directories into %s, and this module expects one: %s"
        % (len(candidates), destination, ", ".join(os.path.basename(c) for c in candidates))
    )


def _read_documents(directory):
    """Return the extracted files as (documents, parsed, files, failures).

    The documentation key is recovered from each filename, because that is what zarf writes: a key
    of `prerequisites` naming `docs/prerequisites.yaml` is extracted as `prerequisites.yaml`, so
    the key is the basename without its suffix. Recovering it is what lets the result be keyed the
    way the package declared it rather than by a filename the caller never wrote.
    """
    documents = {}
    parsed = {}
    files = {}
    failures = {}

    if not os.path.isdir(directory):
        return documents, parsed, files, failures

    for name in sorted(os.listdir(directory)):
        path = os.path.join(directory, name)
        if not os.path.isfile(path):
            continue

        key, suffix = os.path.splitext(name)
        files[key] = path

        try:
            with open(path, "rb") as handle:
                text = handle.read().decode("utf-8")
        except (OSError, UnicodeDecodeError) as exc:
            # A binary or unreadable documentation file -- an architecture diagram, say. It is
            # named in `files` and absent from `documents`, which is more use than failing the
            # task over a file the caller may not have asked about.
            failures[key] = "unable to read %s as UTF-8 text: %s" % (name, exc)
            continue

        documents[key] = text

        if suffix.lower() not in PARSED_SUFFIXES:
            continue
        try:
            document = yaml.safe_load(text)
        except Exception as exc:
            failures[key] = "unable to parse %s as YAML: %s" % (name, exc)
            continue
        if document is not None:
            parsed[key] = document

    return documents, parsed, files, failures


def _prerequisites(parsed):
    """Return the parsed prerequisites document, if the package carries one and it says it is one.

    The apiVersion and kind are checked rather than assumed. A package is free to use the
    `prerequisites` documentation key for a prose file, and returning that as `prerequisites` would
    hand the role a mapping whose `requires` key is missing for the most boring possible reason.
    """
    document = parsed.get(PREREQUISITES_KEY)
    if not isinstance(document, dict):
        return None
    if document.get("apiVersion") != PREREQUISITES_API_VERSION:
        return None
    if document.get("kind") != PREREQUISITES_KIND:
        return None
    return document


class ActionModule(ZarfInfoActionBase):
    """Action plugin that extracts a package's documentation layer and returns what is in it."""

    REQUIRED = ("package",)

    def zarf_argv(self, params):
        return command_parts(params)

    def run(self, tmp=None, task_vars=None):
        """Run the extraction, giving zarf somewhere to extract into.

        The base class runs one command and hands its stdout to interpret(). This command's output
        is files rather than stdout, so the only thing added here is the directory they go in: an
        unset destination becomes a temporary directory, which is removed before returning so that
        a play which inspects a hundred packages does not leave a hundred trees behind.
        """
        args = dict(self._task.args)
        if args.get("destination"):
            self._task.args["destination"] = args["destination"]
            return super(ActionModule, self).run(tmp, task_vars)

        # mkdtemp rather than TemporaryDirectory: this collection supports the Python versions
        # Ansible does, and the explicit cleanup below is needed either way, because the extracted
        # contents are read before it happens.
        destination = tempfile.mkdtemp(prefix="zarf-documentation-")
        self._task.args["destination"] = destination
        try:
            return super(ActionModule, self).run(tmp, task_vars)
        finally:
            self._remove_tree(destination)

    @staticmethod
    def _remove_tree(directory):
        """Remove a directory this module created, without failing the task if it cannot."""
        shutil.rmtree(directory, ignore_errors=True)

    def interpret(self, result, stdout, params):
        del stdout  # the output of this command is files, not a stream

        destination = params["destination"]
        try:
            directory = _extracted_directory(destination)
        except (OSError, ValueError) as exc:
            result["failed"] = True
            result["msg"] = str(exc)
            return

        documents, parsed, files, failures = _read_documents(directory)

        result["directory"] = directory
        result["documents"] = documents
        result["parsed"] = parsed
        result["files"] = files
        result["unparsed"] = failures

        prerequisites = _prerequisites(parsed)
        if prerequisites is not None:
            result["prerequisites"] = prerequisites

        extracted = ", ".join(sorted(documents)) or "nothing"
        result["msg"] = "extracted %s from %s" % (extracted, params["package"])
