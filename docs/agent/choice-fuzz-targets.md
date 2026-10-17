# Why there are two fuzz targets, and why they are not under `fuzz/`

This repository has two Go fuzz targets, both in `internal/zarfmod`, beside the code they exercise. This is the record of why those two and not others, and why a `fuzz/` directory was considered and rejected.

## Why fuzzing at all

The OpenSSF Scorecard's Fuzzing check reports whether a repository fuzzes anything, and a proof-of-concept collection that scores zero on it invites the question. That is the reason the question was asked; it is not the reason these two targets exist. A target written to satisfy a scanner and nothing else is a test that passes forever and catches nothing, which is worse than no target at all, because it looks like coverage.

So the surface was read for places where fuzzing earns its keep: loosely-shaped input meeting hand-written parsing. Most of this tree does not qualify. `command.go` renders strings rather than reading them, and its inputs come from the module's own parameters. `version.go` hands its input to `semver.NewVersion` and treats every failure as "unreadable, carry on" by design, so the parsing is upstream's to fuzz and the failure path is already the lenient one. `ModuleName` in `dispatch.go` is a prefix trim against a fixed map. `response.go` and the `stdout*.go` files parse nothing. Two places survived.

## The two targets

`FuzzComponentName` is the one worth having. `componentName` in `progress.go` is the only real parser in the tree: it reads zarf's stdout one line at a time, in either slog-JSON or console form, because the operator can override zarf's log format and the wrapper has no API to ask instead. The property it holds is not "does not crash" but "a line that announces a component names one". `progress.line` uses a non-empty `current` as its "a component is in flight" flag and `progress.finish` returns early without one, so a `("", true)` answer would set `current` back to empty and silently drop the run's last heartbeat. Nothing would panic and no existing test would fail; the progress display would just stop reporting. The invariant holds today - the console regex's capture group cannot match empty, and the message fallback is reached only for a message containing both "component" and "deploy" - and nothing in the tree said so until this target did.

`FuzzReadArgs` holds the property `Args.Params` exists for: a parameter the module does not accept is an error naming it, never a value silently dropped. The JSON decode underneath is the standard library's and will not crash, but the accepted-name walk in `jsonFieldNames` is hand-written reflection over a struct with embedded fields, and a binary module has no `argument_spec` - as that function's own comment says, it is the whole of what stands between a mistyped parameter and a run that ignores it.

Both seed their corpus with `f.Add` calls in the Go source rather than files under `testdata/fuzz/`, which keeps [`.gitattributes`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.gitattributes) untouched and invents no fixture format. A crasher Go writes there during a campaign is a finding to fix, not a file to commit.

## Why not `fuzz/`

A `fuzz/` directory is the conventional home for fuzz harnesses in a C or C++ project, and for Go projects that drive go-fuzz or OSS-Fuzz builds. Neither applies here, for two reasons.

**Scorecard does not look at paths.** Its Go-native detection globs `*_test.go` across the whole repository and matches `func\s+Fuzz\w+\s*\(\w+\s+\*testing.F\)` against the contents. Where the file sits is not part of the check, so a directory move buys nothing on the score it was proposed for.

**A Go fuzz target lives in the package it tests.** Both targets reach unexported identifiers - `componentName`, `controlPrefix`, `jsonFieldNames`, `initParams` - and there is no exported path to the progress parser at all. A `fuzz/` package could only see the exported API, so the move would mean either exporting internals to suit a test, or dropping the target that guards something real. Exporting a function because a test in the wrong directory cannot see it is the test dictating the API, and the API is the thing with the longer life.

## What is deliberately not fuzzed

`redact_state` in `plugins/action/zarf_state_info.py` is the only place in this repository where a bug leaks a credential, and it is where a fuzzer would most like to go. It is Python, so no Go target reaches it, and fuzzing it means `atheris` or `hypothesis`: an unvendored dependency on a tree whose premise is that the build works with no proxy reachable. The function is already written to fail safe - it matches credential-bearing leaf names anywhere in the document rather than a list of paths, so a field zarf adds under a name like those is withheld before anybody reads that file - and the non-cluster suite covers it. More cases there are the cheaper answer than a fuzzing runtime; see [choice-zarf-info-modules.md](choice-zarf-info-modules.md) for why the redaction removes keys rather than blanking them.

## The cost

Neither target has a CI job, and that is on purpose. Go runs a fuzz target's seed corpus as an ordinary unit test, so `test:unit` exercises the seeds and the recorded crashers for free, and that is the part that protects against a regression. A campaign that explores new input is a by-hand run:

```sh
go test ./internal/zarfmod -run=FuzzComponentName -fuzz=FuzzComponentName -fuzztime=30s
```

What that means is that nobody is fuzzing continuously. The seeds run on every push and find nothing new by definition, so these targets protect the two invariants against a future edit rather than searching for a bug that is there now. Thirty seconds of each found nothing when they were written. If one of these parsers grows a real state machine, a scheduled campaign - or ClusterFuzzLite, which Scorecard also recognises and which would want a `.clusterfuzzlite/` directory - becomes worth its CI minutes. Today it is not.
