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

# syntax=docker/dockerfile:1

# The zarf binary the collection's read-only modules drive. The module wrapper has zarf linked
# into it and needs none, but zarf_package_info, zarf_package_inspect and
# zarf_state_info are plain action plugins that shell out to the installed zarf -- see
# docs/agent/choice-zarf-info-modules.md -- so without one in the image every task using those
# three fails on a binary it cannot find. `zarf completion bash` below also comes from it.
#
# Keep this tag in step with the github.com/zarf-dev/zarf version in go.mod. Nothing enforces it,
# and a mismatch means the read-only modules report on a different zarf release than the
# converging two execute.
#
# It comes from the agent image rather than from a release archive because that image is already a
# signed, multi-architecture artifact carrying the whole CLI at /zarf -- zarf's own agent runs
# `zarf internal agent` out of it -- so there is no tarball to fetch, verify and unpack at build
# time, and the architecture follows the image the way every other FROM here does. It is pinned by
# the digest of the image index, not of one platform's manifest, so the arm64 build resolves too.
#
# It costs about 200MB in the image, which is zarf's own size: it vendors Helm, the Kubernetes
# clients, Kustomize and cosign. There is no smaller artifact that is still the whole CLI.
FROM ghcr.io/zarf-dev/zarf/agent:v0.88.0@sha256:288b3b4d1eb171eec11e5a794e92746001c3931319a9ef203f0cbd6dfc73a389 AS zarf

FROM ghcr.io/almalinux/10-base:10.2@sha256:f72ef25730edaa7bde18a0bc7f8010ec7d1d8d6cef7fa47ecdd280aef65fa785

ARG TARGETPLATFORM
# Both pins are moved by `go run ./magefiles/core generate:dnfPins`, which reads the newest
# version of each out of the AlmaLinux 10 repodata and rewrites these lines and the matching
# build_args in .goreleaser.yaml together. Dependabot moves the two FROM digests above and
# leaves these alone.
ARG ANSIBLE_CORE_VERSION="2.16.16-2.el10_2.1"
ARG BASH_COMPLETION_VERSION="2.11-16.el10"

# hadolint ignore=DL3040
RUN dnf install -y --nodocs --setopt=install_weak_deps=0 ansible-core-${ANSIBLE_CORE_VERSION} bash-completion-${BASH_COMPLETION_VERSION} \
  && dnf clean all \
  && rm -rf /var/cache/yum

ARG UID=65532
ARG GID=65532
RUN groupadd --gid "${GID}" nonroot \
  && useradd --uid "${UID}" --gid "${GID}" --home-dir /home/nonroot --create-home nonroot \
  && mkdir -p \
    /home/nonroot/.zarf-cache \
    /home/nonroot/.ssh \
    /workspace \
  && chmod 0700 /home/nonroot /home/nonroot/.ssh \
  && chown -R "${UID}:${GID}" /home/nonroot /workspace

# 0755 explicitly: the binary is 0700 in the agent image, where it only ever runs as that image's
# own user, and a mode nobody else can execute is not what belongs on a management node's PATH.
COPY --from=zarf --chmod=0755 /zarf /usr/local/bin/zarf

# The nfpm rpm goreleaser staged for this platform: the collection tree and both module file
# symlinks. It is installed rather than copied in so that the image carries the same layout a
# management node gets from the package, which is what the checks below are worth anything about.
#
# There is no `rpm --checksig` and no public key imported, because this repository holds no signing
# key: `nfpms` here signs nothing, and a `--checksig` against an unsigned package fails the build
# for a reason that is not the image's. Verification of a released package belongs to whoever
# installs it, against the cosign bundle the release carries.
COPY ${TARGETPLATFORM}/*.rpm /tmp/rpms/

RUN rpm --install --verbose /tmp/rpms/*.rpm \
  && rm -rf /tmp/rpms \
  && zarf version

# The collection the package installs is on ansible-core's default collections path, and both of
# its module files answer. This is the only place in the pipeline where that is true at once: the
# collection, the binaries it dispatches to, and an ansible-core to find them are all here.
#
# These checks are the ZEP-0072 proof's contract, and each one fails the image build rather than a
# playbook. Invoked under a zarf_ name that is not an action it answers as, a module file must
# refuse with a non-zero status rather than emit JSON nobody asked for -- package_remove is such a
# name, because this binary answers as init, kubectl, package_deploy, package_mirror_resources,
# package_pull, package_remove and update_creds and nothing else -- package_create is deliberately
# not among them; given parameters
# it cannot read, a module must still answer with one JSON object and exit 0, because a non-zero
# status makes Ansible report MODULE FAILURE instead of the message the module wrote; and in check
# mode it must report the task as skipped, since zarf init has no dry run. Every module file name
# is checked to be a link to the same binary, because that is what makes argv[0] dispatch the
# thing being proved rather than an implementation detail. zarf_kubectl is checked three ways: a
# command kubectl does not have must be refused by name rather than printing help and reporting
# success, and `kubectl version --client` must come back in zarf.output, which needs no cluster and
# is the whole of what that module is. The checks that pipe read
# their parameters on stdin, which is the path this collection's action plugin uses.
# The checks below pipe the module's output into a parser, and a wrapper that crashed mid-pipe
# must fail the build rather than be covered by the parser's exit status.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN ansible-galaxy collection list >/tmp/collections 2>/dev/null \
  && grep -q 'colonel_byte.zarf' /tmp/collections \
  && rm -f /tmp/collections \
  && test -L /usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_init \
  && ln -s /usr/bin/zarf_init /tmp/zarf_package_create \
  && ! /tmp/zarf_package_create >/dev/null 2>&1 \
  && rm -f /tmp/zarf_package_create \
  && ZARF_ANSIBLE_MODULE=init zarf_init /dev/null \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") else 1)' \
  && printf '%s' '{"init_package":"/tmp","_ansible_check_mode":true}' \
    | ZARF_ANSIBLE_MODULE=init zarf_init \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("skipped") else 1)' \
  && test -L /usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_package_deploy \
  && test "$(readlink -f /usr/bin/zarf_init)" = "$(readlink -f /usr/bin/zarf_package_deploy)" \
  && ZARF_ANSIBLE_MODULE=package_deploy zarf_package_deploy /dev/null \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") else 1)' \
  && printf '%s' '{"package":"/tmp/pkg.tar.zst","_ansible_check_mode":true}' \
    | ZARF_ANSIBLE_MODULE=package_deploy zarf_package_deploy \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("skipped") else 1)' \
  && printf '%s' '{"package":"/tmp/pkg.tar.zst","kubeconfig":"/tmp/none"}' \
    | ZARF_ANSIBLE_MODULE=package_deploy zarf_package_deploy \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); d=r.get("zarf",{}); sys.exit(0 if r.get("failed") and d.get("exitCode") else 1)' \
  && test -L /usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_package_pull \
  && ZARF_ANSIBLE_MODULE=package_pull zarf_package_pull /dev/null \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") else 1)' \
  && printf '%s' '{"package":"/srv/staging/local.tar.zst"}' \
    | ZARF_ANSIBLE_MODULE=package_pull zarf_package_pull \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") and "local path" in r.get("msg","") else 1)' \
  && test -L /usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_package_remove \
  && printf '%s' '{"package":"monitoring","_ansible_check_mode":true}' \
    | ZARF_ANSIBLE_MODULE=package_remove zarf_package_remove \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("skipped") and not r.get("changed") else 1)' \
  && test -L /usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_package_mirror_resources \
  && printf '%s' '{"package":"/tmp/pkg.tar.zst","images":true,"repos":true}' \
    | ZARF_ANSIBLE_MODULE=package_mirror_resources zarf_package_mirror_resources \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") and "mutually exclusive" in r.get("msg","") else 1)' \
  && test -L /usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_update_creds \
  && printf '%s' '{"service":"registry","git_push_password":"hunter2"}' \
    | ZARF_ANSIBLE_MODULE=update_creds zarf_update_creds \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") and "git_push_password" in r.get("msg","") else 1)' \
  && test -L /usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_kubectl \
  && ZARF_ANSIBLE_MODULE=kubectl zarf_kubectl /dev/null \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") else 1)' \
  && printf '%s' '{"args":["apply","-f","/tmp/none.yaml"],"_ansible_check_mode":true}' \
    | ZARF_ANSIBLE_MODULE=kubectl zarf_kubectl \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("skipped") else 1)' \
  && printf '%s' '{"args":["frobnicate"]}' \
    | ZARF_ANSIBLE_MODULE=kubectl zarf_kubectl \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); sys.exit(0 if r.get("failed") and "frobnicate" in r.get("msg","") else 1)' \
  && printf '%s' '{"args":["version","--client","-o","json"]}' \
    | ZARF_ANSIBLE_MODULE=kubectl zarf_kubectl \
    | python3 -c 'import json,sys; r=json.load(sys.stdin); d=r.get("zarf",{}); sys.exit(0 if not r.get("failed") and json.loads(d.get("output",""))  else 1)' \
  && zarf completion bash |  tee /etc/bash_completion.d/zarf > /dev/null

ENV HOME=/home/nonroot

WORKDIR /workspace

USER 65532:65532

CMD ["ansible-playbook", "--help"]
