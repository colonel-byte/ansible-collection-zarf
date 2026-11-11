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

# A home-manager module that installs the colonel_byte.zarf collection for one user.
#
# What it does is the two halves a playbook needs: the module files on PATH, which is how the
# action plugins find the wrapper, and the collection linked where Ansible already looks. It
# deliberately does not set ANSIBLE_COLLECTIONS_PATH -- that variable replaces the search path
# rather than adding to it, so setting it here would hide every other collection the user has.
#
# See docs/dev/nix.md.
{ flake }:
{
  config,
  lib,
  pkgs,
  ...
}:
let
  cfg = config.programs.zarf-ansible-modules;
  # The path Ansible resolves `colonel_byte.zarf` to, under whichever collections directory.
  collection = "ansible_collections/colonel_byte/zarf";
in
{
  options.programs.zarf-ansible-modules = with lib; {
    enable = mkOption {
      type = types.bool;
      default = false;
      description = "Whether to install the colonel_byte.zarf Ansible collection and its module files.";
    };
    package = mkOption {
      type = types.package;
      default = flake.packages.${pkgs.stdenv.hostPlatform.system}.default;
      defaultText = literalExpression "zarf-ansible-modules.packages.\${system}.default";
      description = "The package holding the collection and the module wrapper.";
    };
    path = mkOption {
      type = types.str;
      default = ".ansible/collections";
      description = ''
        Where the collection is linked, relative to the home directory. The default is one of the
        two directories ansible-core searches without being told, so a playbook finds the
        collection with nothing exported.
      '';
    };
    installZarf = mkOption {
      type = types.bool;
      default = false;
      description = ''
        Whether to install `zarf` alongside. The five read-only modules are action plugins that run
        the installed zarf; the seven that act have one linked in and need none. Off by default
        because a management node usually pins its own zarf, and the one in nixpkgs may be an
        older release than the one compiled into the module files.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [ cfg.package ] ++ lib.optional cfg.installZarf pkgs.zarf;

    home.file."${cfg.path}/${collection}".source =
      "${cfg.package}/share/ansible/collections/${collection}";
  };
}
