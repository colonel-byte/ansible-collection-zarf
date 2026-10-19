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
	"os"
	"path/filepath"
	"testing"
)

// Scaffolding the tests in this package share: a zarf that answers without one being installed,
// the WANT_JSON intake a module is really driven through, and a reader for a child's environment.

// fakeRunner stands in for the installed zarf.
type fakeRunner struct {
	lines    []string
	exitCode int
	err      error
	// version is what `zarf version` prints. The init module reads it before it runs, so a
	// runner that answered the real lines to both calls would have the version check parsing
	// log output. Empty means a zarf recent enough to not be the subject of the test.
	version string

	got Invocation
	ran []Invocation
}

func (f *fakeRunner) Run(_ context.Context, in Invocation) (Result, error) {
	f.got = in
	f.ran = append(f.ran, in)

	if len(in.Args) > 0 && in.Args[0] == "version" {
		version := f.version
		if version == "" {
			version = "v" + MinZarfVersion
		}
		if in.OnLine != nil {
			in.OnLine(version)
		}
		return Result{}, nil
	}

	for _, line := range f.lines {
		if in.OnLine != nil {
			in.OnLine(line)
		}
	}
	return Result{ExitCode: f.exitCode}, f.err
}

// argsFor writes an arguments file holding params and reads it back the way a WANT_JSON caller
// delivers it.
func argsFor(t *testing.T, params map[string]any) *Args {
	t.Helper()
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("unable to encode the test parameters: %v", err)
	}
	path := filepath.Join(t.TempDir(), "args")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("unable to write the test arguments file: %v", err)
	}
	args, err := ReadArgsFile(path)
	if err != nil {
		t.Fatalf("unable to read the test arguments file: %v", err)
	}
	return args
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
