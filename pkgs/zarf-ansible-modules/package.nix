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

# The colonel_byte.zarf collection and the module wrapper it invokes, as one package.
#
# What it produces is what a management node needs: the wrapper on PATH under every name it
# answers as, and the collection on a path Ansible can be pointed at. See docs/dev/nix.md.
{
  lib,
  buildGoModule,
  go_1_27,
  stdenv,
}:
# go.mod asks for 1.27.2 and nixpkgs' default go is older, so the toolchain is pinned rather than
# inherited. A mismatch here fails the build with a toolchain download attempt, which the sandbox
# refuses -- an obscure failure for an ordinary reason.
(buildGoModule.override { go = go_1_27; }) (finalAttrs: {
  pname = "zarf-ansible-modules";
  # The collection's version, which is what an operator installs and what galaxy.yml carries.
  version = "0.87.0";

  src = lib.fileset.toSource {
    root = ../..;
    # Named rather than cleaned, so that an unrelated file appearing in the tree does not change
    # the derivation and rebuild a 270MB binary.
    fileset = lib.fileset.unions [
      # keep-sorted start
      ../../ansible/colonel_byte/zarf
      ../../api
      ../../cmd
      ../../go.mod
      ../../go.sum
      ../../internal
      ../../vendor
      # keep-sorted end
    ];
  };

  # The dependency tree is committed under vendor/, which is the point of this repository: the
  # management node it runs on is typically air-gapped, so the build reaches no module proxy.
  vendorHash = null;

  subPackages = [ "cmd/zarfmod" ];

  env.CGO_ENABLED = 0;
  ldflags = [
    "-s"
    "-w"
  ];

  # Nothing in internal/zarfmod's tests needs a cluster, but the contract tests read the collection
  # out of ansible/, and the fileset above is why they can. The e2e suites are not here: they need
  # Ansible, k3d and Docker.
  preCheck = ''
    export ZARF_ANSIBLE_SKIP_E2E=1
  '';
  checkFlags = [ "-skip=TestFuzz" ];

  postInstall = ''
    collection="$out/share/ansible/collections/ansible_collections/colonel_byte/zarf"
    mkdir -p "$(dirname "$collection")"
    cp -r ansible/colonel_byte/zarf "$collection"
    chmod -R u+w "$collection"

    # One symlink per module, under both names a caller can reach it by: on PATH, which is what
    # the action plugins resolve, and under the collection's plugins/modules, which is what a
    # conventional Ansible caller stages. The binary dispatches on argv[0], so the name is what
    # selects the module -- see cmd/zarfmod.
    #
    # The list comes from the action plugins rather than being written here, because they are
    # where a module's name is already declared. zarfmod.ModuleFiles() is the other copy, and the
    # installCheck below holds this list against it by running every name.
    modules=$(grep -h 'DEFAULT_WRAPPER = ' "$collection"/plugins/action/*.py |
      sed 's/.*"\(.*\)".*/\1/' | sort)
    test -n "$modules"

    for module in $modules; do
      ln -s zarfmod "$out/bin/$module"
      ln -s "$out/bin/$module" "$collection/plugins/modules/$module"
    done
  '';

  doInstallCheck = true;
  # The dispatch contract, the same one containers/ansible/Dockerfile asserts: invoked under a
  # module name with parameters it cannot read, a module file still writes one JSON object and
  # exits 0, because a non-zero status makes Ansible report MODULE FAILURE instead of the message
  # the module wrote. A name the binary does not answer as must refuse instead.
  installCheckPhase = ''
    runHook preInstallCheck

    for module in $out/bin/zarf_*; do
      echo "checking $(basename "$module")"
      "$module" /dev/null | grep -q '"failed":true'
    done

    ln -s "$out/bin/zarfmod" "$TMPDIR/zarf_package_create"
    if "$TMPDIR/zarf_package_create" >/dev/null 2>&1; then
      echo "the wrapper answered as zarf_package_create, which is not a module it dispatches" >&2
      exit 1
    fi

    runHook postInstallCheck
  '';

  meta = {
    description = "Ansible collection and module files for zarf, with zarf linked in";
    longDescription = ''
      The colonel_byte.zarf collection: modules that initialise an air-gapped Kubernetes cluster
      with zarf, stage and deploy packages onto it, mirror their resources, remove them, rotate the
      cluster's credentials, and read it. The modules that act are one compiled binary with zarf
      linked into it, invoked under one name per module, so they need no zarf on the node; the
      read-only modules are action plugins that run the installed zarf.
    '';
    homepage = "https://github.com/colonel-byte/ansible-collection-zarf";
    license = lib.licenses.asl20;
    mainProgram = "zarfmod";
    # Ansible runs a module file on the management node, and the management nodes this is for run
    # Linux. The release builds carry the same restriction; see .goreleaser.yaml.
    platforms = lib.platforms.linux;
    broken = !stdenv.hostPlatform.isLinux;
  };
})
