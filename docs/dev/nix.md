# Installing with Nix

The repository is a flake. What it produces is one package -- the module wrapper under every name it answers as, plus the collection on a path Ansible can be pointed at -- so a management node needs nothing else from this repository.

```sh
nix build github:colonel-byte/ansible-collection-zarf     # into ./result
nix profile install github:colonel-byte/ansible-collection-zarf
```

## Pointing Ansible at it

Ansible looks for collections in `~/.ansible/collections` and `/usr/share/ansible/collections`, and Nix installs into neither. Name the path instead:

```sh
export ANSIBLE_COLLECTIONS_PATH="$HOME/.nix-profile/share/ansible/collections"
ansible-playbook -i management.ini playbooks/zarf-bundle-order.yml
```

Or link it into the path Ansible already reads, which is what a long-lived management node wants:

```sh
mkdir -p ~/.ansible/collections/ansible_collections/colonel_byte
ln -sfn "$HOME/.nix-profile/share/ansible/collections/ansible_collections/colonel_byte/zarf" \
  ~/.ansible/collections/ansible_collections/colonel_byte/zarf
```

The module files resolve on `PATH`, which `nix profile install` already handles: each `zarf_<module>` in the profile's `bin/` is a symlink to the one wrapper, and the name is what selects the module. A task can also name one outright with `wrapper_binary`.

## As a home-manager module

For a user whose environment is declarative, the flake exports a module that does both halves: the module files on `PATH` and the collection linked where Ansible already looks.

```nix
{
  inputs.zarf-ansible-modules.url = "github:colonel-byte/ansible-collection-zarf";

  # ... in the home-manager configuration:
  imports = [ inputs.zarf-ansible-modules.homeManagerModules.default ];

  programs.zarf-ansible-modules = {
    enable = true;
    # installZarf = true;   # for the read-only modules, which run the installed zarf
  };
}
```

| Option        | Default                | What it is                                                                       |
| ------------- | ---------------------- | -------------------------------------------------------------------------------- |
| `enable`      | `false`                | Installs the package and links the collection                                    |
| `package`     | this flake's package   | The package holding the collection and the wrapper, for pinning or overriding it |
| `path`        | `.ansible/collections` | Where the collection is linked, relative to the home directory                   |
| `installZarf` | `false`                | Installs `pkgs.zarf` alongside, for the read-only modules                        |

The default `path` is one of the two directories ansible-core searches on its own, so a playbook resolves `colonel_byte.zarf` with nothing exported. The module deliberately leaves `ANSIBLE_COLLECTIONS_PATH` alone: that variable *replaces* the search path rather than adding to it, so setting it would hide every other collection the user has.

`installZarf` is off by default because a management node usually pins its own zarf, and nixpkgs may carry an older release than the one compiled into the module files -- see the note below.

## What the package holds

| Path                                                              | What it is                                                                                      |
| ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `bin/zarfmod`                                                     | The wrapper binary, with zarf linked in. Run by hand it prints the names it answers as          |
| `bin/zarf_init`, `bin/zarf_package_deploy`, ...                   | One symlink per module in the dispatch table, which is how `argv[0]` selects the module         |
| `share/ansible/collections/ansible_collections/colonel_byte/zarf` | The collection: the action plugins, the shared Python, the roles                                |
| `.../zarf/plugins/modules/zarf_<module>`                          | The same names again, for a caller that stages a module file rather than resolving it on `PATH` |

The seven names come from the action plugins' `DEFAULT_WRAPPER`, read at build time, so a module added to the collection is installed without this derivation being edited. The build then runs every name it installed and requires one JSON object back, and requires a name the binary does not answer as to refuse -- the same dispatch contract [`containers/ansible/Dockerfile`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/containers/ansible/Dockerfile) asserts.

## Working in the tree

`.envrc` is a direnv shim for the same shell, so `direnv allow` once gives every later `cd` into the repository the collections path, the module files on `PATH` from `result/bin`, and the toolchain:

```sh
direnv allow
nix build            # or `go run ./magefiles/core build:modules`, which writes build/ instead
```

The flake has no `formatter` output and no `pre-commit-hooks` input. [`.pre-commit-config.yaml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.pre-commit-config.yaml) is the hook list and [`AGENTS.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/AGENTS.md) says to run it with `prek`; a second hook manager or formatter over the same tree would mean two pins of the same tools and, for Markdown, a formatter that undoes the hand-formatting [`docs/AGENTS.md`](../AGENTS.md) requires.

`nix flake check` builds the package and runs its dispatch check. Dependabot's `nix` ecosystem watches `flake.lock` daily and groups the inputs into one pull request, the same way the `gomod` and `docker` ecosystems are already configured in [`.github/dependabot.yaml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.github/dependabot.yaml).

## A `zarf` is still worth having

The five read-only modules are action plugins that run the installed `zarf`; the seven that act have one linked in and need none. `nix develop` provides both, along with `ansible-core`, `go_1_27` and the collections path already set:

```sh
nix develop                      # a management node, ready to run a playbook
```

`nixpkgs` carries an older zarf than the one compiled into the wrapper, so a read-only module may report on a different zarf release than a converging one executes. That is the same skew a management node has when its `zarf` is older than its module files, and [choice-version-tracks-zarf](../agent/choice-version-tracks-zarf.md) is the record of how the two are meant to move.

## The toolchain is pinned, and that is load-bearing

`pkgs/zarf-ansible-modules/package.nix` builds with `go_1_27` rather than nixpkgs' default Go, because `go.mod` asks for 1.27.2 -- and that version is not arbitrary. Go 1.27 is the first release that embeds module dependency information for a **vendored** build, and `internal/zarfmod/libraryrunner.go` recovers the linked-in zarf's version from exactly that information: lower the directive and `config.CLIVersion` stays `unset-development-only`, so `zarf init` looks for `zarf-init-<arch>-unset-development-only.tar.zst` and never finds a staged package. `TestLinkedZarfVersionIsFilledBeforeAnythingReadsIt` is what fails when somebody tries.

Nothing in the vendored tree needs more than 1.26.9 -- zarf itself asks for that -- so the directive looks free to lower. It is not.
