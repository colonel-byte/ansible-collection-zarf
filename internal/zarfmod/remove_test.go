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
	"strings"
	"testing"

	"github.com/zarf-dev/zarf/src/cmd"
	"github.com/zarf-dev/zarf/src/config"
)

// Tests for the zarf_package_remove module: the command line it renders, the flags that command
// line is held against, what it refuses, and what it reports having removed.

func TestBuildRemoveArgs(t *testing.T) {
	yes := true
	number := 4

	tests := []struct {
		name   string
		params removeParams
		want   []string
	}{
		{
			name: "a deployed package by name",
			params: removeParams{
				Package: "csi-local-path-provider",
			},
			// --confirm on every run: there is no terminal to answer zarf's prompt.
			want: []string{
				"package", "remove", "csi-local-path-provider",
				"--confirm", "--no-color", "--log-format", "json",
			},
		},
		{
			name: "one component of a package in a namespace",
			params: removeParams{
				Package:    "monitoring",
				Components: "grafana-dashboards",
				Namespace:  "observability",
			},
			want: []string{
				"package", "remove", "monitoring",
				"--confirm", "--no-color", "--log-format", "json",
				"--components", "grafana-dashboards",
				"--namespace", "observability",
			},
		},
		{
			name: "the on-remove action values",
			params: removeParams{
				Package:               "monitoring",
				Values:                []string{"/etc/zarf/values.yaml", "/etc/zarf/overrides.yaml"},
				SetValues:             map[string]string{"second": "2", "first": "1"},
				OCIConcurrency:        &number,
				PlainHTTP:             &yes,
				InsecureSkipTLSVerify: &yes,
				Cache:                 "/var/cache/zarf",
				Tmpdir:                "/var/tmp/zarf",
				Architecture:          "amd64",
				LogLevel:              "debug",
			},
			// set_values is sorted, so the same parameters always render the same vector.
			want: []string{
				"package", "remove", "monitoring",
				"--confirm", "--no-color", "--log-format", "json", "--log-level", "debug",
				"--oci-concurrency", "4",
				"--values", "/etc/zarf/values.yaml",
				"--values", "/etc/zarf/overrides.yaml",
				"--set-values", "first=1",
				"--set-values", "second=2",
				"--architecture", "amd64",
				"--cache", "/var/cache/zarf",
				"--tmpdir", "/var/tmp/zarf",
				"--plain-http=true",
				"--insecure-skip-tls-verify=true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRemoveArgs(&tt.params)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildRemoveArgs rendered\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

// TestRemoveArgsAreKnownFlags parses every flag the module can render against zarf's own
// `package remove` command. The reasoning is on TestInitArgsAreKnownFlags.
func TestRemoveArgsAreKnownFlags(t *testing.T) {
	widest := fullRemoveParams()
	argv := buildRemoveArgs(widest)

	root := cmd.NewZarfCommand()
	pruneRootShorthands(root)

	removeCmd, flags, err := root.Find(argv)
	if err != nil {
		t.Fatalf("zarf has no command for %q: %v", argv, err)
	}
	if path := removeCmd.CommandPath(); path != "zarf package remove" {
		t.Fatalf("the module renders %q, which zarf resolves to %q", argv, path)
	}
	if err := removeCmd.ParseFlags(flags); err != nil {
		t.Fatalf("the module renders something zarf package remove does not accept: %v", err)
	}
	// The command takes at most one positional argument, so a flag whose value was eaten as one
	// shows up here rather than as a parse error.
	if got := removeCmd.Flags().Args(); len(got) != 1 || got[0] != widest.Package {
		t.Errorf("zarf reads the positional arguments as %q, want exactly [%s]", got, widest.Package)
	}
}

func TestRunPackageRemoveRequiresAPackage(t *testing.T) {
	resp := &Response{Zarf: &Detail{Module: "package_remove"}}
	run := &fakeRunner{}

	err := runPackageRemove(context.Background(), run, argsFor(t, map[string]any{}), resp)
	if err == nil {
		t.Fatal("runPackageRemove accepted a task with no package")
	}
	if run.got.Args != nil {
		t.Errorf("it ran %q anyway", run.got.Args)
	}
}

// TestRunPackageRemoveCheckModeSkips is the one that matters most of the three check-mode tests in
// this package: a check-mode run that went ahead would delete something.
func TestRunPackageRemoveCheckModeSkips(t *testing.T) {
	args := argsFor(t, map[string]any{
		"package":             "csi-local-path-provider",
		"_ansible_check_mode": true,
	})
	resp := &Response{Zarf: &Detail{Module: "package_remove"}}
	run := &fakeRunner{}

	if err := runPackageRemove(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runPackageRemove reported %v", err)
	}
	if !resp.Skipped || resp.Changed {
		t.Errorf("a check-mode run reported skipped=%v changed=%v", resp.Skipped, resp.Changed)
	}
	if run.got.Args != nil {
		t.Errorf("a check-mode run executed %q", run.got.Args)
	}
	if resp.Zarf.ChangedSignal != SignalUnknown {
		t.Errorf("the changed signal was %q, want %q", resp.Zarf.ChangedSignal, SignalUnknown)
	}
}

func TestRunPackageRemoveReportsTheRun(t *testing.T) {
	args := argsFor(t, map[string]any{
		"package":    "csi-local-path-provider",
		"kubeconfig": "/etc/rancher/rke2/rke2.yaml",
		"components": "local-path-chart",
	})
	resp := &Response{Zarf: &Detail{Module: "package_remove"}}
	run := &fakeRunner{}

	if err := runPackageRemove(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runPackageRemove reported %v", err)
	}

	if len(run.got.Args) < 3 || run.got.Args[2] != "csi-local-path-provider" {
		t.Errorf("zarf was run as %v, want the package as the positional argument", run.got.Args)
	}
	if !containsEnv(run.got.Env, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml") {
		t.Errorf("the environment was %v, want it to carry KUBECONFIG", run.got.Env)
	}
	// Unknown, not partial: zarf names no component on the way out, so there is no component list
	// behind the signal.
	if !resp.Changed || resp.Zarf.ChangedSignal != SignalUnknown {
		t.Errorf("the run reported changed=%v signal=%q", resp.Changed, resp.Zarf.ChangedSignal)
	}
	if resp.Zarf.Version != config.CLIVersion {
		t.Errorf("the reported zarf version was %q, want the linked-in %q",
			resp.Zarf.Version, config.CLIVersion)
	}
	if !strings.Contains(resp.Msg, "csi-local-path-provider") {
		t.Errorf("the message does not name what was removed: %q", resp.Msg)
	}
}
