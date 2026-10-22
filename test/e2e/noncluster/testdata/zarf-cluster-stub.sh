#!/bin/sh
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

# A stand-in for zarf, for the zarf_cluster_info and prerequisites-role tests.
#
# Unlike the other stubs in this directory, this one reads its arguments. It has to: zarf_cluster_info
# asks the cluster five questions, and the whole point of the module is that each answer lands in a
# different result key -- a stub that printed one document whatever it was asked would prove that
# the module can parse one document, not that it asked five questions and kept the answers apart.
# A dispatch that does not recognise a command fails loudly for the same reason: a silent empty
# answer is a cluster that looks empty, and an empty cluster satisfies nothing, which would make
# this test pass for the wrong reason the day the module's commands change.
#
# The fixtures are named by ZARF_ANSIBLE_E2E_CLUSTER_DIR. Nothing here is contacted.
set -eu

: "${ZARF_ANSIBLE_E2E_CLUSTER_DIR:?the fixture directory is required}"

# The queries, matched on the two words that identify each: the subcommand and the resource. The
# resource names are fully qualified the way the module spells them, so that a module which started
# asking for `crd` instead of `customresourcedefinitions.apiextensions.k8s.io` fails here rather
# than being served the same answer under a different name.
case " $* " in
  *" version "*)
    cat "${ZARF_ANSIBLE_E2E_CLUSTER_DIR}/zarf-cluster-version.json"
    ;;
  *" customresourcedefinitions.apiextensions.k8s.io "*)
    cat "${ZARF_ANSIBLE_E2E_CLUSTER_DIR}/zarf-cluster-crds.txt"
    ;;
  *" storageclasses.storage.k8s.io "*)
    cat "${ZARF_ANSIBLE_E2E_CLUSTER_DIR}/zarf-cluster-storageclasses.json"
    ;;
  *" deployments.apps "*)
    cat "${ZARF_ANSIBLE_E2E_CLUSTER_DIR}/zarf-cluster-deployments.json"
    ;;
  *" statefulsets.apps "*)
    cat "${ZARF_ANSIBLE_E2E_CLUSTER_DIR}/zarf-cluster-statefulsets.json"
    ;;
  *" list "*)
    cat "${ZARF_ANSIBLE_E2E_CLUSTER_DIR}/zarf-package-list.json"
    ;;
  # Handed over for the same reason as the documentation layer below: inspecting a definition
  # needs a real package, and the deploy_packages role inspects every item it is given to learn
  # the package's name and version.
  *" inspect definition "*)
    : "${ZARF_ANSIBLE_E2E_REAL_ZARF:?a real zarf is required to inspect a definition}"
    exec "${ZARF_ANSIBLE_E2E_REAL_ZARF}" "$@"
    ;;
  # Spelled as one segment rather than as *" inspect "*" documentation "*, which does not match:
  # the first segment consumes the space that the second one then needs in front of its own word.
  *" inspect documentation "*)
    # Handed to the real zarf rather than served from a fixture. Extracting a documentation layer
    # needs a real package, and the Go test builds one; faking this is what would make the
    # prerequisites role's own extraction branch untested, since the role takes one zarf binary for
    # both the extraction and the cluster reads and cannot be given a real one for half of them.
    : "${ZARF_ANSIBLE_E2E_REAL_ZARF:?a real zarf is required to extract documentation}"
    exec "${ZARF_ANSIBLE_E2E_REAL_ZARF}" "$@"
    ;;
  *)
    echo "zarf-cluster-stub.sh was asked something it does not serve: $*" >&2
    exit 64
    ;;
esac
