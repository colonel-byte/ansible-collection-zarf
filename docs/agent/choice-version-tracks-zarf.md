# Why the collection's version tracks zarf's

`colonel_byte.zarf` is versioned to loosely follow the version of zarf it is written against, rather than counting its own releases from zero. The collection released as `0.87.0` is the one to install beside zarf `v0.87.x`.

This is a deliberate break from how the collection was versioned in [colonel-byte/cargoship](https://github.com/colonel-byte/cargoship), where it inherited the cargoship tag and reached `0.32.1` -- a number that said something true about cargoship and nothing at all about zarf.

## Why not start from 0.1.0

Two reasons, and the first is not negotiable.

**0.32.1 is already published.** Cargoship's `v0.32.1` release ships `colonel_byte-zarf-0.32.1.tar.gz` and the `zarf_ansible_modules_Linux_*` archives. Those are in the wild under this collection's name. A first release here at `0.1.1` would be a *lower* version of the same collection: `ansible-galaxy` resolves semver, so the new line would read as older than what has already shipped, and a `>=0.32.0` pin would never pick it up. Any starting point has to be above `0.32.1`.

**A number that means nothing is worse than no number.** Having to pick something above `0.32.1` anyway, the choice was between an arbitrary `0.33.0` and a number that answers the question an operator actually has: which zarf does this work with? The modules render a zarf command line and are checked against a recorded list of that zarf's flags -- see [choice-zarf-ansible-module](choice-zarf-ansible-module.md) -- so "which zarf" is the single most load-bearing fact about a given release.

## What "loosely" means

Loosely, because the two cannot be locked together:

- **Not every zarf release needs one here.** A zarf release that changes nothing this collection renders produces no release, and the version simply lags. A lag is honest: it says the last zarf this was checked against.
- **This collection can release without zarf doing so.** A fix to a role, an action plugin, or the redaction is a patch bump on the current line. That is why the patch component is this collection's to spend, and why `0.87.1` means "the first fix against zarf 0.87", not "zarf 0.87.1".
- **Nothing enforces it.** Release-please computes the next version from conventional commits, so a `feat:` on the `0.87.x` line produces `0.88.0` whether or not zarf 0.88 exists. Moving the line to a new zarf release is a manual edit to `.release-please-manifest.json` and `galaxy.yml`, made deliberately when the recorded flag lists are refreshed with `generate:zarfFlags`.

So the version is a statement of intent rather than a guarantee, and the guarantee lives where it can be checked: `internal/zarfmod/testdata/zarf-*-flags.txt` records the flag surface actually tested against.

## What has to move together

Bumping the line to a new zarf is more than the two version strings. Refresh the recorded flag lists against the new zarf, and read the diff -- a flag that disappeared is a module parameter that no longer works:

```sh
go run ./magefiles/core generate:zarfFlags
```

The state fixture in `test/e2e/noncluster/testdata/zarf-state.json` is a recording of the shape zarf writes, and it goes stale the same way. It is the thing most likely to drift without failing anything, because the non-cluster suite checks the module against the recording rather than against zarf.
