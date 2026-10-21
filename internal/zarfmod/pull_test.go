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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zarf-dev/zarf/src/cmd"
)

// Tests for the zarf_package_pull module: the command line it renders, the flags that command line
// is held against, what it refuses, and what it reports having fetched.

func TestBuildPullArgs(t *testing.T) {
	yes := true
	number := 4

	tests := []struct {
		name   string
		params pullParams
		want   []string
	}{
		{
			name: "a package and nothing else",
			params: pullParams{
				Package: "oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0",
			},
			want: []string{
				"package", "pull", "oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0",
				"--no-color", "--log-format", "json",
			},
		},
		{
			name: "a staged fetch over https",
			params: pullParams{
				Package:         "https://packages.bubbles.test/zarf-package-monitoring-amd64.tar.zst",
				OutputDirectory: "/srv/staging",
				Shasum:          "0000",
				OCIConcurrency:  &number,
			},
			want: []string{
				"package", "pull", "https://packages.bubbles.test/zarf-package-monitoring-amd64.tar.zst",
				"--no-color", "--log-format", "json",
				"--output-directory", "/srv/staging",
				"--shasum", "0000",
				"--oci-concurrency", "4",
			},
		},
		{
			name: "the registry parameters",
			params: pullParams{
				Package:               "oci://registry.bubbles.test/monitoring:1.0.0",
				PlainHTTP:             &yes,
				InsecureSkipTLSVerify: &yes,
				Architecture:          "arm64",
				LogFormat:             "console",
				LogLevel:              "debug",
			},
			want: []string{
				"package", "pull", "oci://registry.bubbles.test/monitoring:1.0.0",
				"--no-color", "--log-format", "console", "--log-level", "debug",
				"--architecture", "arm64",
				"--plain-http=true",
				"--insecure-skip-tls-verify=true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPullArgs(&tt.params)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildPullArgs rendered\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

// TestPullArgsAreKnownFlags parses every flag the module can render against zarf's own
// `package pull` command, so pflag is the thing that decides whether a flag exists. The reasoning
// is on TestInitArgsAreKnownFlags; what this one adds is that pull takes no --confirm, which is
// the flag the other converging modules always render.
func TestPullArgsAreKnownFlags(t *testing.T) {
	widest := fullPullParams()
	argv := buildPullArgs(widest)

	root := cmd.NewZarfCommand()
	pruneRootShorthands(root)

	pullCmd, flags, err := root.Find(argv)
	if err != nil {
		t.Fatalf("zarf has no command for %q: %v", argv, err)
	}
	if path := pullCmd.CommandPath(); path != "zarf package pull" {
		t.Fatalf("the module renders %q, which zarf resolves to %q", argv, path)
	}
	if err := pullCmd.ParseFlags(flags); err != nil {
		t.Fatalf("the module renders something zarf package pull does not accept: %v", err)
	}
	// Exactly one positional argument, which is what the command takes.
	if got := pullCmd.Flags().Args(); len(got) != 1 || got[0] != widest.Package {
		t.Errorf("zarf reads the positional arguments as %q, want exactly [%s]", got, widest.Package)
	}
}

func TestRunPackagePullRefusesWhatCannotBeFetched(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		names  string
	}{
		{
			name:   "no package",
			params: map[string]any{},
			names:  "package",
		},
		{
			name:   "a local path",
			params: map[string]any{"package": "/srv/staging/zarf-package-monitoring-amd64.tar.zst"},
			names:  "local path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{Zarf: &Detail{Module: "package_pull"}}
			run := &fakeRunner{}

			err := runPackagePull(context.Background(), run, argsFor(t, tt.params), resp)
			if err == nil {
				t.Fatalf("runPackagePull accepted %v", tt.params)
			}
			if !strings.Contains(err.Error(), tt.names) {
				t.Errorf("the error does not say %q: %v", tt.names, err)
			}
			if run.got.Args != nil {
				t.Errorf("it ran %q anyway", run.got.Args)
			}
		})
	}
}

func TestRunPackagePullCheckModeSkips(t *testing.T) {
	args := argsFor(t, map[string]any{
		"package":             "oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0",
		"_ansible_check_mode": true,
	})
	resp := &Response{Zarf: &Detail{Module: "package_pull"}}
	run := &fakeRunner{}

	if err := runPackagePull(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runPackagePull reported %v", err)
	}
	if !resp.Skipped || resp.Changed {
		t.Errorf("a check-mode run reported skipped=%v changed=%v", resp.Skipped, resp.Changed)
	}
	if run.got.Args != nil {
		t.Errorf("a check-mode run executed %q", run.got.Args)
	}
}

// TestRunPackagePullReportsWhatArrived is the module's one real changed signal: the file that was
// not in the output directory before the run and is now.
func TestRunPackagePullReportsWhatArrived(t *testing.T) {
	dir := t.TempDir()
	// A package that was already staged, so the test proves the difference rather than the
	// listing: this one must not be reported.
	if err := os.WriteFile(filepath.Join(dir, "zarf-package-already-amd64.tar.zst"),
		[]byte("not really a package"), 0o600); err != nil {
		t.Fatalf("unable to stage the existing package: %v", err)
	}

	args := argsFor(t, map[string]any{
		"package":          "oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0",
		"output_directory": dir,
		"zarf_config":      "/etc/zarf/zarf-config.yaml",
	})
	resp := &Response{Zarf: &Detail{Module: "package_pull"}}
	// The fetch is what the runner stands in for, so the file it would have written is written
	// here instead.
	arrival := filepath.Join(dir, "zarf-package-monitoring-amd64-1.0.0.tar.zst")
	run := &writingRunner{path: arrival}

	if err := runPackagePull(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runPackagePull reported %v", err)
	}

	if got, want := strings.Join(resp.Zarf.PackagesPulled, ","),
		"zarf-package-monitoring-amd64-1.0.0.tar.zst"; got != want {
		t.Errorf("packagesPulled was %q, want %q", got, want)
	}
	if !resp.Changed || resp.Zarf.ChangedSignal != SignalComplete {
		t.Errorf("the run reported changed=%v signal=%q", resp.Changed, resp.Zarf.ChangedSignal)
	}
	if !containsEnv(run.got.Env, "ZARF_CONFIG=/etc/zarf/zarf-config.yaml") {
		t.Errorf("the environment was %v, want it to carry ZARF_CONFIG", run.got.Env)
	}
}

// TestRunPackagePullReportsNoChangeWhenNothingArrived is the other half: zarf overwrites a package
// it fetches again, so a second run of the same pull adds no filename and the task says so.
func TestRunPackagePullReportsNoChangeWhenNothingArrived(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "zarf-package-monitoring-amd64-1.0.0.tar.zst")
	if err := os.WriteFile(staged, []byte("not really a package"), 0o600); err != nil {
		t.Fatalf("unable to stage the existing package: %v", err)
	}

	args := argsFor(t, map[string]any{
		"package":          "oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0",
		"output_directory": dir,
	})
	resp := &Response{Zarf: &Detail{Module: "package_pull"}}
	run := &writingRunner{path: staged}

	if err := runPackagePull(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runPackagePull reported %v", err)
	}
	if resp.Changed || len(resp.Zarf.PackagesPulled) != 0 {
		t.Errorf("the run reported changed=%v packagesPulled=%v",
			resp.Changed, resp.Zarf.PackagesPulled)
	}
}

// writingRunner stands in for a zarf that fetched a package, by writing the file one would have
// left behind.
type writingRunner struct {
	path string

	got Invocation
}

func (w *writingRunner) Run(_ context.Context, in Invocation) (Result, error) {
	w.got = in
	if err := os.WriteFile(w.path, []byte("not really a package"), 0o600); err != nil {
		return Result{ExitCode: 1}, err
	}
	return Result{}, nil
}
