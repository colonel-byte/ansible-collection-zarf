# Test data in the e2e suites

## Put test documents in files, not in Go string constants

A YAML or JSON document a test parses, serves, or drives a playbook with belongs in a file under a `testdata/` directory, read at the top of the test. It does not belong in a `const doc = ` + backtick string in the `_test.go` file beside it.

```go
// Yes.
fixture, err := os.ReadFile(filepath.Join(env.root, stateFixture))
require.NoError(t, err)
credentials := fixtureCredentials(t, fixture)
```

```go
// No.
const stateTestDoc = `{
  "registryInfo": {"pushPassword": "hunter2"},
...
`
```

Both suites are built this way. `test/e2e/noncluster/testdata/` holds the recorded `zarf-state.json` and `zarf-package-list.json`, the playbooks that drive the modules over them, and the stub `zarf` scripts that serve them; `test/e2e/cluster/testdata/` holds the two k3d configs and the playbooks the cluster suite runs.

### Why

- **One document, one copy.** `testdata/zarf-state.json` is read by the Go test, served by `testdata/zarf-state-stub.sh`, and asserted over by `testdata/zarf-state-redaction.yml`. Three copies would drift, and the drift shows up as a test that passes over a state document zarf does not write.
- **A fixture carries its own explanation.** `testdata/zarf-state-redaction.yml` opens with what it covers, which eight credentials the `zarf-state` Secret holds, why the stub stands in for zarf, and a line saying nothing in it is contacted. A Go string literal sitting inside a test function has nowhere to put that.
- **Indentation survives.** A backtick string sits at column 0 inside an indented function, so YAML in it reads badly and is easy to get wrong. A file is just the file.
- **The document is diffable.** A reviewer reading a change to a fixture sees a YAML or JSON diff rather than a diff of a Go literal, and `git blame` on the fixture answers why a field is there.
- **A verbatim copy stays verbatim.** `testdata/k3d-config.yaml` is a byte-for-byte copy of the upstream `uds-bundles` config, kept that way so the suite initialises the cluster shape that repository actually deploys. Its header says to refresh it from upstream rather than edit it to suit a test, which is a thing a file can say and a string constant cannot.

### How

Add the document to the `testdata/` directory of the suite that uses it, with a header that says what shape it covers and why, ending in a line stating that nothing in it is contacted:

```yaml
---
# What colonel_byte.zarf.zarf_state_info returns, and what it withholds.
#
# zarf_binary is the stub beside this file rather than a real zarf, so the play needs no cluster.
#
# Nothing here is contacted.
- name: zarf_state_info withholds the credentials in the zarf-state Secret
```

Then name it in a constant beside the test and read it through the suite's own helpers rather than with a bare `os.ReadFile`:

```go
const (
	stateRedactionPlaybook = "test/e2e/noncluster/testdata/zarf-state-redaction.yml"
	stateFixture           = "test/e2e/noncluster/testdata/zarf-state.json"
	zarfStub               = "test/e2e/noncluster/testdata/zarf-state-stub.sh"
)
```

The paths are relative to the repository root, because `internal/testenv` chdirs there before any test runs -- that is what the package is for, along with waiting on a cluster. A path written relative to the test's own package directory is a file that is not there, reported as a failure to read it.

A stub script needs the executable bit at run time, which `env.executable(t, zarfStub)` handles; do not rely on the mode the file is checked in with.

These playbooks and stubs are read, not rewritten. Where a test does need to modify a fixture, copy it into `t.TempDir()` first and leave the checked-in file alone.

### Two things to avoid in tests over a fixture

- **Do not count lines to a value.** `const keyLine = 10 // the agent TLS key` breaks the next time somebody adds a comment to the fixture, and it breaks as though the code under test moved something. Find the value by its key or its prefix instead.
- **Do not assert on a count where the values themselves can be named.** `len(credentials) != 8` passes when the wrong eight values were found. `fixtureCredentials` in `noncluster/17_zarf_state_redaction_test.go` walks the fixture and collects every value carrying the `FIXTURE-` prefix, then asserts the module's output contains none of them -- so a credential added to the fixture is covered without the test being edited, and a renamed field is a failure rather than a set of assertions that quietly stop running. Give any `switch` over fixture contents a `default` that fails, for the same reason.

## When a document does belong in the Go file

Only when the document is not a document: a few bytes that are deliberately malformed, or a single scalar. A parse-error case that is two lines long is clearer beside the assertion than in a file of its own.

A valid document that a test then mutates is not one of these. Read the fixture and mutate the copy:

```go
fixture, err := os.ReadFile(filepath.Join(env.root, stateFixture))
require.NoError(t, err)
doc := strings.Replace(string(fixture), `"distro": "k3s"`, `"distro": "rke2"`, 1)
```

## The suites skip rather than fail

Every test in both suites checks its prerequisites and calls `t.Skip` with a reason when one is missing, so a green run is not proof that anything ran. This is deliberate -- the alternative is a suite that fails on every contributor's laptop -- but it means a suite that silently stopped running looks exactly like a suite that passed. Read the output, and see [`docs/dev/e2e-tests.md`](../../docs/dev/e2e-tests.md) for what each suite needs installed.
