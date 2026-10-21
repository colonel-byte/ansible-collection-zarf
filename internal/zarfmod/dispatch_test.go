// Copyright 2026 colonel-byte
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zarfmod

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// argsFileFor writes a WANT_JSON arguments file and returns the argv Ansible would invoke the
// module with: the module path, then the file.
func argsFileFor(t *testing.T, params map[string]any) []string {
	t.Helper()

	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("unable to encode the test parameters: %v", err)
	}
	path := filepath.Join(t.TempDir(), "args")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("unable to write the test arguments file: %v", err)
	}
	return []string{"/usr/bin/zarf_init", path}
}

// captureRun calls Run with standard output pointed at a file, and returns what landed on it.
//
// Pointing os.Stdout at a file before the call is what makes this a test of the guard as well as
// of the response: newStdoutGuard duplicates whatever descriptor os.Stdout holds at the moment it
// runs, so the duplicate is this file, and anything the run writes to os.Stdout afterwards goes to
// stderr instead and never reaches it.
func captureRun(t *testing.T, name string, argv []string, run Runner) (string, int) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "stdout")
	file, err := os.Create(path) //nolint:gosec // a path under t.TempDir
	if err != nil {
		t.Fatalf("unable to create the capture file: %v", err)
	}

	saved := os.Stdout
	os.Stdout = file
	status := Run(context.Background(), name, argv, run)
	os.Stdout = saved
	if err := file.Close(); err != nil {
		t.Fatalf("unable to close the capture file: %v", err)
	}

	out, err := os.ReadFile(path) //nolint:gosec // a path under t.TempDir
	if err != nil {
		t.Fatalf("unable to read the capture file: %v", err)
	}
	return string(out), status
}

// TestRunEmitsExactlyOneObjectAndExitsZero is the module contract, held end to end.
//
// Three things have to be true of every run whatever happened inside it, and none of them is
// visible from the functions the other tests in this package call directly:
//
//   - standard output carries exactly one JSON object and nothing else. Ansible parses the whole
//     of stdout as one document, so a stray line from anything the wrapper calls is a module that
//     fails to parse rather than one that reports a problem.
//   - the process exits 0 even for a failed run. A non-zero status makes Ansible report MODULE
//     FAILURE and print its own diagnosis in place of the message the module wrote, which is the
//     less useful of the two.
//   - a failure is reported in the failed field, with the reason in msg.
//
// The failing cases are the point. A run that only ever succeeded would satisfy the first two by
// accident.
func TestRunEmitsExactlyOneObjectAndExitsZero(t *testing.T) {
	for _, tt := range []struct {
		name       string
		module     string
		argv       []string
		run        Runner
		wantFailed bool
		wantMsg    string
	}{
		{
			name:   "a run that succeeded",
			module: "init",
			argv: argsFileFor(t, map[string]any{
				"init_package": "oci://ghcr.io/zarf-dev/packages/init:v0.86.0",
			}),
			run:        &fakeRunner{},
			wantFailed: false,
			wantMsg:    "initialised the cluster",
		},
		{
			name:   "a module this binary does not answer as",
			module: "package_remove",
			argv:   argsFileFor(t, map[string]any{}),
			run:    &fakeRunner{},
			// ZEP-0072 names package_remove and nothing here implements it, so this is the
			// message an operator of a near-future playbook actually hits.
			wantFailed: true,
			wantMsg:    "is not a zarf Ansible module",
		},
		{
			name:       "no parameters delivered at all",
			module:     "init",
			argv:       []string{"/usr/bin/zarf_init"},
			run:        &fakeRunner{},
			wantFailed: true,
			wantMsg:    "no module parameters",
		},
		{
			name:   "a parameter the module refuses",
			module: "init",
			// Neither init_package nor directory, which leaves zarf searching whatever directory
			// Ansible happened to put the module in.
			argv:       argsFileFor(t, map[string]any{}),
			run:        &fakeRunner{},
			wantFailed: true,
			wantMsg:    "is required",
		},
		{
			name:   "zarf itself exited non-zero",
			module: "init",
			argv: argsFileFor(t, map[string]any{
				"init_package": "oci://ghcr.io/zarf-dev/packages/init:v0.86.0",
			}),
			run:        &fakeRunner{exitCode: 1, err: errors.New("exit status 1")},
			wantFailed: true,
			wantMsg:    "zarf init failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, status := captureRun(t, tt.module, tt.argv, tt.run)

			if status != 0 {
				t.Errorf("Run returned %d, want 0: a module reports failure in its result, not in "+
					"its exit status", status)
			}

			// Exactly one line, and it decodes. strings.Count of a newline rather than a split,
			// so that a trailing newline is not mistaken for a second document.
			if n := strings.Count(strings.TrimRight(out, "\n"), "\n"); n != 0 {
				t.Fatalf("standard output carried %d extra line(s), want one JSON object:\n%s",
					n, out)
			}
			var got Response
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("standard output did not decode as one JSON object: %v\n%s", err, out)
			}

			if got.Failed != tt.wantFailed {
				t.Errorf("failed was %v, want %v (msg: %q)", got.Failed, tt.wantFailed, got.Msg)
			}
			if !strings.Contains(got.Msg, tt.wantMsg) {
				t.Errorf("msg was %q, want one naming %q", got.Msg, tt.wantMsg)
			}
			if got.Zarf == nil || got.Zarf.Module != tt.module {
				t.Errorf("the result does not name the module it answered as: %+v", got.Zarf)
			}
		})
	}
}

// TestRunKeepsStrayOutputOffStandardOutput holds the guard rather than the response.
//
// The wrapper runs zarf as a child process and reads its log stream, and the whole of that stream
// is somebody else's idea of what to print. Anything inside the run that reaches for os.Stdout --
// a stray fmt.Println, a library that logs, a future progress renderer -- must land on stderr,
// because Ansible parses the whole of stdout as the module's one result.
func TestRunKeepsStrayOutputOffStandardOutput(t *testing.T) {
	argv := argsFileFor(t, map[string]any{
		"init_package": "oci://ghcr.io/zarf-dev/packages/init:v0.86.0",
	})

	// A runner that prints where a careless one would. os.Stdout is the module's own channel at
	// the moment this is called only if the guard failed to take it over.
	noisy := &printingRunner{line: "this must not reach the module result"}

	out, status := captureRun(t, "init", argv, noisy)
	if status != 0 {
		t.Fatalf("Run returned %d, want 0", status)
	}
	if strings.Contains(out, noisy.line) {
		t.Errorf("output written to os.Stdout during the run reached the module result:\n%s", out)
	}
	var got Response
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("standard output did not decode as one JSON object: %v\n%s", err, out)
	}
}

// printingRunner writes to os.Stdout while it runs, which is what the guard exists to make safe.
type printingRunner struct {
	line string
}

func (p *printingRunner) Run(_ context.Context, _ Invocation) (Result, error) {
	os.Stdout.WriteString(p.line + "\n") //nolint:errcheck,gosec // the write is the point
	return Result{}, nil
}

func TestModuleName(t *testing.T) {
	tests := []struct {
		name  string
		argv0 string
		env   string
		want  string
		ok    bool
	}{
		{
			name:  "module symlink",
			argv0: "/usr/share/ansible/collections/ansible_collections/colonel_byte/zarf/plugins/modules/zarf_init",
			want:  "init",
			ok:    true,
		},
		{
			name:  "staged copy carries the AnsiballZ prefix",
			argv0: "/root/.ansible/tmp/ansible-tmp-1/AnsiballZ_zarf_init",
			want:  "init",
			ok:    true,
		},
		{
			name:  "the environment wins",
			argv0: "/usr/local/bin/zarf-ansible",
			env:   "init",
			want:  "init",
			ok:    true,
		},
		{
			name:  "a build artifact carrying the prefix is not a module",
			argv0: "/build/zarf_linux_amd64",
			ok:    false,
		},
		{
			name:  "the deploy module symlink",
			argv0: "/usr/bin/zarf_package_deploy",
			want:  "package_deploy",
			ok:    true,
		},
		{
			name:  "an action this wrapper does not answer as is not a module",
			argv0: "/usr/bin/zarf_package_remove",
			ok:    false,
		},
		{
			name:  "the plain binary is not a module",
			argv0: "/usr/bin/zarf",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvModule, tt.env)

			got, ok := ModuleName(tt.argv0)
			if ok != tt.ok {
				t.Fatalf("ModuleName(%q) reported %v, want %v", tt.argv0, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ModuleName(%q) = %q, want %q", tt.argv0, got, tt.want)
			}
		})
	}
}
