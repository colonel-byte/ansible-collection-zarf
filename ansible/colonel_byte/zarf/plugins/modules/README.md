# The modules are one wrapper binary

There are no module files in this directory in the source tree, and there is no Python module behind any of them. Every module that acts on a cluster is one compiled binary under several names: invoked under a name carrying the `zarf_` prefix, it answers as that module, reads its parameters as JSON -- from the WANT_JSON arguments file a conventional caller passes, or from stdin when this collection's action plugin invokes it -- and writes one JSON object back.

It is built from `cmd/zarfmod` in this repository, which `build:modules` does along with a symlink per module name:

```sh
go run ./magefiles/core build:modules   # build/zarfmod, zarf_init, zarf_package_deploy, zarf_kubectl
```

Put the results somewhere on the path of the node the play runs on, or name a path with the module's `wrapper_binary` parameter. Create the same `zarf_<module>` symlinks here as well for a conventional Ansible caller that stages the module file itself -- the action plugins do not need them, because they invoke the installed binaries directly.

ZEP-0072 proposes that this dispatch live inside the `zarf` binary, with each `zarf_<action>` as a symlink to `/usr/bin/zarf` and no wrapper at all. This collection proves the shape from outside, with the symlinks pointing at a binary of its own, because a proof built in this repository cannot change zarf. See [choice-zarf-ansible-module](../../../../../docs/agent/choice-zarf-ansible-module.md).
