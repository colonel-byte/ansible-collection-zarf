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
)

// Tests for the zarf_package_mirror_resources module: the command line it renders, the flags that
// command line is held against, what it refuses, and what it reports having mirrored.

func TestBuildMirrorArgs(t *testing.T) {
	yes := true
	number := 3

	tests := []struct {
		name   string
		params mirrorParams
		want   []string
	}{
		{
			name: "a package and the cluster's own registry",
			params: mirrorParams{
				Package: "/srv/staging/zarf-package-monitoring-amd64.tar.zst",
			},
			want: []string{
				"package", "mirror-resources", "/srv/staging/zarf-package-monitoring-amd64.tar.zst",
				"--confirm", "--no-color", "--log-format", "json",
			},
		},
		{
			name: "images only, to a registry outside the cluster",
			params: mirrorParams{
				Package:              "oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0",
				Images:               &yes,
				RegistryURL:          "registry.bubbles.test",
				RegistryPushUsername: "zarf-push",
				RegistryPushPassword: "hunter2",
				Retries:              &number,
			},
			want: []string{
				"package", "mirror-resources", "oci://ghcr.io/colonel-byte/zarf/monitoring:1.0.0",
				"--confirm", "--no-color", "--log-format", "json",
				"--registry-url", "registry.bubbles.test",
				"--registry-push-username", "zarf-push",
				"--registry-push-password", "hunter2",
				"--images=true",
				"--retries", "3",
			},
		},
		{
			name: "a signed package verified on the way through",
			params: mirrorParams{
				Package:    "https://packages.bubbles.test/zarf-package-monitoring-amd64.tar.zst",
				Shasum:     "0000",
				PublicKey:  "/etc/zarf/cosign.pub",
				Verify:     "always",
				Components: "monitoring-images",
			},
			// --verify is rendered in the equals form: it declares a default for being given
			// none, so the space form would leave "always" as a positional argument.
			want: []string{
				"package", "mirror-resources", "https://packages.bubbles.test/zarf-package-monitoring-amd64.tar.zst",
				"--confirm", "--no-color", "--log-format", "json",
				"--components", "monitoring-images",
				"--shasum", "0000",
				"--key", "/etc/zarf/cosign.pub",
				"--verify=always",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMirrorArgs(&tt.params)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildMirrorArgs rendered\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

// TestMirrorArgsAreKnownFlags parses every flag the module can render against zarf's own
// `package mirror-resources` command. The reasoning is on TestInitArgsAreKnownFlags.
func TestMirrorArgsAreKnownFlags(t *testing.T) {
	for _, widest := range []*mirrorParams{fullMirrorParams(), fullMirrorParamsMirroringRepos()} {
		argv := buildMirrorArgs(widest)

		root := cmd.NewZarfCommand()
		pruneRootShorthands(root)

		mirrorCmd, flags, err := root.Find(argv)
		if err != nil {
			t.Fatalf("zarf has no command for %q: %v", argv, err)
		}
		if path := mirrorCmd.CommandPath(); path != "zarf package mirror-resources" {
			t.Fatalf("the module renders %q, which zarf resolves to %q", argv, path)
		}
		if err := mirrorCmd.ParseFlags(flags); err != nil {
			t.Fatalf("the module renders something zarf package mirror-resources does not accept: %v", err)
		}
		if got := mirrorCmd.Flags().Args(); len(got) != 1 || got[0] != widest.Package {
			t.Errorf("zarf reads the positional arguments as %q, want exactly [%s]", got, widest.Package)
		}
	}
}

func TestRunMirrorResourcesRefusesWhatZarfWould(t *testing.T) {
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
			name: "both kinds of resource at once",
			params: map[string]any{
				"package": "/srv/staging/zarf-package-monitoring-amd64.tar.zst",
				"images":  true,
				"repos":   true,
			},
			names: "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{Zarf: &Detail{Module: "package_mirror_resources"}}
			run := &fakeRunner{}

			err := runMirrorResources(context.Background(), run, argsFor(t, tt.params), resp)
			if err == nil {
				t.Fatalf("runMirrorResources accepted %v", tt.params)
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

// TestMirrorExclusivityIsZarfsOwn pins the refusal above to zarf's own declaration, so that a zarf
// release which stopped marking the two exclusive would show up as a module refusing something
// zarf allows.
func TestMirrorExclusivityIsZarfsOwn(t *testing.T) {
	argv := []string{"package", "mirror-resources", "/srv/staging/pkg.tar.zst",
		"--images=true", "--repos=true"}

	root := cmd.NewZarfCommand()
	pruneRootShorthands(root)

	mirrorCmd, flags, err := root.Find(argv)
	if err != nil {
		t.Fatalf("zarf has no command for %q: %v", argv, err)
	}
	if err := mirrorCmd.ParseFlags(flags); err != nil {
		t.Fatalf("zarf does not accept the flags at all: %v", err)
	}
	// The exclusivity is checked when the command runs rather than when its flags parse, so what
	// is asserted here is the declaration itself.
	if err := mirrorCmd.ValidateFlagGroups(); err == nil {
		t.Error("zarf no longer marks --images and --repos mutually exclusive, so the module's " +
			"refusal of both is its own invention")
	}
}

func TestRunMirrorResourcesCheckModeSkips(t *testing.T) {
	args := argsFor(t, map[string]any{
		"package":             "/srv/staging/zarf-package-monitoring-amd64.tar.zst",
		"_ansible_check_mode": true,
	})
	resp := &Response{Zarf: &Detail{Module: "package_mirror_resources"}}
	run := &fakeRunner{}

	if err := runMirrorResources(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runMirrorResources reported %v", err)
	}
	if !resp.Skipped || resp.Changed {
		t.Errorf("a check-mode run reported skipped=%v changed=%v", resp.Skipped, resp.Changed)
	}
	if run.got.Args != nil {
		t.Errorf("a check-mode run executed %q", run.got.Args)
	}
}

// TestRunMirrorResourcesReportsTheRun covers what a mirror can say: that it ran, with the push
// credential kept out of the result. What it cannot say is which components it walked -- zarf
// announces the images rather than the component they came from -- so the signal is unknown and
// there is no component list. See runMirrorResources.
func TestRunMirrorResourcesReportsTheRun(t *testing.T) {
	args := argsFor(t, map[string]any{
		"package":                "/srv/staging/zarf-package-monitoring-amd64.tar.zst",
		"kubeconfig":             "/etc/rancher/rke2/rke2.yaml",
		"registry_push_password": "hunter2",
		"components":             "monitoring-images",
	})
	resp := &Response{Zarf: &Detail{Module: "package_mirror_resources"}}
	run := &fakeRunner{
		lines: []string{
			`{"msg":"mirroring images"}`,
			`{"msg":"pushing image","image":"ghcr.io/colonel-byte/monitoring:1.0.0"}`,
			`{"msg":"done pushing images"}`,
		},
	}

	if err := runMirrorResources(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runMirrorResources reported %v", err)
	}

	if !resp.Changed || resp.Zarf.ChangedSignal != SignalUnknown {
		t.Errorf("the run reported changed=%v signal=%q", resp.Changed, resp.Zarf.ChangedSignal)
	}
	if len(resp.Zarf.ComponentsRan) != 0 {
		t.Errorf("the run named the components %v, which a mirror does not announce",
			resp.Zarf.ComponentsRan)
	}
	if strings.Contains(strings.Join(resp.Zarf.Command, " "), "hunter2") {
		t.Errorf("the reported command carries the push credential: %v", resp.Zarf.Command)
	}
	if !containsEnv(run.got.Env, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml") {
		t.Errorf("the environment was %v, want it to carry KUBECONFIG", run.got.Env)
	}
}
