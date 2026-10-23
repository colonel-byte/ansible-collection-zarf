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

{
  description = "The colonel_byte.zarf Ansible collection and the module wrapper it invokes";

  inputs = {
    # keep-sorted start block=yes case=no
    flake-utils = {
      inputs.systems.follows = "systems";
      url = "github:numtide/flake-utils";
    };
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable-small";
    # Linux only, as the release builds are: Ansible runs a module file on the management node, and
    # the management nodes this collection is for run Linux. See .goreleaser.yaml.
    systems.url = "github:nix-systems/default-linux";
    # keep-sorted end
  };

  # No pre-commit-hooks input and no formatter output. .pre-commit-config.yaml is this
  # repository's hook list and AGENTS.md tells a contributor to run it with prek; a second hook
  # manager or formatter over the same tree would mean two pins of the same tools, two runs of
  # everything, and -- for Markdown -- a formatter that undoes the hand-formatting docs/AGENTS.md
  # requires. What this flake does is build and install the collection.
  outputs =
    inputs@{
      # keep-sorted start
      flake-utils,
      nixpkgs,
      self,
      # keep-sorted end
      ...
    }:
    flake-utils.lib.eachSystem flake-utils.lib.defaultSystems (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };
      in
      {
        checks = {
          # The package builds its own dispatch check: every module file name it installs is run
          # and has to answer with one JSON object. See pkgs/zarf-ansible-modules/package.nix.
          inherit (self.packages.${system}) zarf-ansible-modules;
        };

        # `nix develop` is a management node: ansible-core, the collection on its collections path,
        # every module file on PATH, and a zarf for the read-only modules to run.
        devShells.default = pkgs.mkShell {
          buildInputs = [
            # keep-sorted start
            pkgs.ansible
            pkgs.go_1_27
            pkgs.zarf
            self.packages.${system}.default
            # keep-sorted end
          ];

          # Ansible reads ~/.ansible/collections and /usr/share/ansible/collections, and Nix
          # installs into neither, so the path is named outright.
          shellHook = ''
            export ANSIBLE_COLLECTIONS_PATH="${self.packages.${system}.default}/share/ansible/collections"
            echo "colonel_byte.zarf is on ANSIBLE_COLLECTIONS_PATH, and its module files are on PATH."
          '';
        };

        packages = rec {
          default = zarf-ansible-modules;
          zarf-ansible-modules = pkgs.callPackage ./pkgs/zarf-ansible-modules/package.nix { };
        };
      }
    )
    // {
      # A module and an overlay are not per-system, so both sit outside eachSystem.
      homeManagerModules = rec {
        default = zarf-ansible-modules;
        zarf-ansible-modules = import ./modules/home-manager { flake = self; };
      };

      # For a nixpkgs this is composed into rather than built from.
      overlays.default = final: _prev: {
        zarf-ansible-modules = final.callPackage ./pkgs/zarf-ansible-modules/package.nix { };
      };
    };
}
