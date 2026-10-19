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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the zarf version floor: the version it refuses to run against, and the version it
// cannot read and runs against anyway.

// TestRunInitRefusesAnOldZarf covers the floor. Before v0.72.0 zarf init took no positional
// argument, so passing the init package would fail as a usage error naming neither the parameter
// nor the version that accepts it.
func TestRunInitRefusesAnOldZarf(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "zarf-init-amd64-v0.71.0.tar.zst")
	if err := os.WriteFile(pkg, []byte("not really a package"), 0o600); err != nil {
		t.Fatalf("unable to stage the test init package: %v", err)
	}

	args := argsFor(t, map[string]any{"init_package": pkg})
	resp := &Response{Zarf: &Detail{Module: "init"}}
	run := &fakeRunner{version: "v0.71.1"}

	err := runInit(context.Background(), run, args, resp)
	if !errors.Is(err, ErrZarfTooOld) {
		t.Fatalf("runInit reported %v, want ErrZarfTooOld", err)
	}
	if !strings.Contains(err.Error(), MinZarfVersion) {
		t.Errorf("the error did not name the version it needs: %v", err)
	}
	if resp.Zarf.Version != "v0.71.1" {
		t.Errorf("the version it found was not reported: %q", resp.Zarf.Version)
	}
	for _, in := range run.ran {
		if len(in.Args) > 0 && in.Args[0] == "init" {
			t.Error("the init ran against a zarf that is too old")
		}
	}
}

// TestRunInitToleratesAnUnreadableVersion pins the other half of that check: the floor exists to
// turn a confusing failure into a clear one, so a version string it cannot parse must not become a
// refusal of its own. The run proceeds and the version comes back as whatever was printed.
func TestRunInitToleratesAnUnreadableVersion(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "zarf-init-amd64-dev.tar.zst")
	if err := os.WriteFile(pkg, []byte("not really a package"), 0o600); err != nil {
		t.Fatalf("unable to stage the test init package: %v", err)
	}

	args := argsFor(t, map[string]any{"init_package": pkg})
	resp := &Response{Zarf: &Detail{Module: "init"}}
	run := &fakeRunner{version: "built from source"}

	if err := runInit(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runInit reported %v", err)
	}
	if resp.Zarf.Version != "built from source" {
		t.Errorf("the version it found was not reported: %q", resp.Zarf.Version)
	}
}
