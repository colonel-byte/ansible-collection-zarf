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
	"path/filepath"
	"strings"
	"testing"

	"github.com/zarf-dev/zarf/src/cmd"
	"github.com/zarf-dev/zarf/src/config"
)

func TestBuildDeployArgs(t *testing.T) {
	yes := true
	no := false

	tests := []struct {
		name   string
		params deployParams
		want   []string
	}{
		{
			name: "an oci reference is passed through unchanged",
			params: deployParams{
				Package: "oci://ghcr.io/colonel-byte/zarf/csi-local-path-provider:0.0.37-upstream",
			},
			want: []string{
				"package", "deploy",
				"oci://ghcr.io/colonel-byte/zarf/csi-local-path-provider:0.0.37-upstream",
				"--confirm",
				"--no-color",
				"--log-format", "json",
			},
		},
		{
			name: "components and signature verification",
			params: deployParams{
				Package: "/srv/staging/csi-local-path.tar.zst",
				commonParams: commonParams{
					Components: "local-path-images,local-path-chart",
					PublicKey:  "/etc/zarf/packages.pub",
					Verify:     "always",
					Timeout:    "15m",
				},
			},
			want: []string{
				"package", "deploy",
				"/srv/staging/csi-local-path.tar.zst",
				"--confirm",
				"--no-color",
				"--log-format", "json",
				"--components", "local-path-images,local-path-chart",
				"--timeout", "15m",
				"--key", "/etc/zarf/packages.pub",
				"--verify=always",
			},
		},
		{
			name: "a false boolean renders, an unset one does not",
			params: deployParams{
				Package:   "/srv/staging/pkg.tar.zst",
				Connected: &yes,
				commonParams: commonParams{
					PlainHTTP: &no,
				},
			},
			want: []string{
				"package", "deploy",
				"/srv/staging/pkg.tar.zst",
				"--confirm",
				"--no-color",
				"--log-format", "json",
				// plain_http is shared with zarf_init and renders in applyCommonFlags, which runs
				// before the module adds its own -- so --plain-http precedes --connected, which
				// only a deploy has.
				"--plain-http=false",
				"--connected=true",
			},
		},
		{
			name: "variables render one flag per entry, sorted",
			params: deployParams{
				Package: "/srv/staging/pkg.tar.zst",
				commonParams: commonParams{
					SetVariables: map[string]string{
						"STORAGE_CLASS": "local-path",
						"NODE_PATH":     "/var/lib/local-path-provisioner",
					},
				},
			},
			want: []string{
				"package", "deploy",
				"/srv/staging/pkg.tar.zst",
				"--confirm",
				"--no-color",
				"--log-format", "json",
				"--set-variables", "NODE_PATH=/var/lib/local-path-provisioner",
				"--set-variables", "STORAGE_CLASS=local-path",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDeployArgs(&tt.params)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildDeployArgs rendered\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

// TestDeployPackageResolvesALocalPath proves the one case where the module rewrites what the
// operator wrote: a relative path with no working directory named, where the directory the module
// inherits is Ansible's rather than the operator's.
func TestDeployPackageResolvesALocalPath(t *testing.T) {
	absolute := deployPackage(&deployParams{Package: "pkg.tar.zst"})
	if !filepath.IsAbs(absolute) {
		t.Errorf("a relative package with no directory was left relative: %q", absolute)
	}

	kept := deployPackage(&deployParams{
		Package: "pkg.tar.zst",
		commonParams: commonParams{
			Directory: "/srv/staging",
		},
	})
	if kept != "pkg.tar.zst" {
		t.Errorf("a relative package with a directory was rewritten to %q", kept)
	}

	remote := "oci://ghcr.io/colonel-byte/zarf/init/slim:v0.82.0-upstream"
	if got := deployPackage(&deployParams{Package: remote}); got != remote {
		t.Errorf("a remote reference was rewritten to %q", got)
	}
}

// TestDeployArgsAreKnownFlags parses every flag the module can render against zarf's own
// `package deploy` command, so pflag is the thing that decides whether a flag exists.
//
// The init side of this is TestInitArgsAreKnownFlags, and the reasoning is there. Both modules now
// execute zarf's command tree, so neither renders against a recorded list any more.
func TestDeployArgsAreKnownFlags(t *testing.T) {
	yes := true
	number := 1
	widest := deployParams{
		Package:   "/srv/staging/pkg.tar.zst",
		Namespace: "storage",
		Shasum:    "0000000000000000000000000000000000000000000000000000000000000000",
		Connected: &yes,
		commonParams: commonParams{
			Components:            "local-path-chart",
			ForceConflicts:        &yes,
			SkipValuesSchemaValid: &yes,
			InsecureSkipTLSVerify: &yes,
			PlainHTTP:             &yes,
			TakeOwnership:         &yes,
			Retries:               &number,
			OCIConcurrency:        &number,
			Architecture:          "amd64",
			Cache:                 "/var/cache/zarf",
			Tmpdir:                "/var/tmp/zarf",
			Timeout:               "15m",
			PublicKey:             "/etc/zarf/packages.pub",
			Verify:                "always",
			LogLevel:              "debug",
			LogFormat:             "json",
			Values:                []string{"/etc/zarf/values.yaml"},
			SetValues:             map[string]string{"key": "value"},
			SetVariables:          map[string]string{"KEY": "value"},
		},
	}

	argv := buildDeployArgs(&widest)

	// Find resolves the nested subcommand the way an invocation would, so a flag the module puts
	// on `package deploy` that zarf keeps on the root or on `package` is still parsed through the
	// inherited set rather than missed.
	deployCmd, flags, err := cmd.NewZarfCommand().Find(argv)
	if err != nil {
		t.Fatalf("zarf has no command for %q: %v", argv, err)
	}
	if deployCmd.CommandPath() != "zarf package deploy" {
		t.Fatalf("the module renders %q, which zarf resolves to %q", argv, deployCmd.CommandPath())
	}

	if err := deployCmd.ParseFlags(flags); err != nil {
		t.Fatalf("the module renders something zarf package deploy does not accept: %v", err)
	}

	// The package is deploy's positional PACKAGE_SOURCE, so prove it survived as one argument
	// rather than being eaten as some flag's value -- the mistake --verify=always exists to avoid.
	if got := deployCmd.Flags().Args(); len(got) != 1 || got[0] != widest.Package {
		t.Errorf("zarf reads the positional arguments as %q, want exactly [%s]", got, widest.Package)
	}
}

func TestRunPackageDeployRequiresAPackage(t *testing.T) {
	args := argsFor(t, map[string]any{
		"kubeconfig": "/etc/rancher/rke2/rke2.yaml",
	})
	resp := &Response{Zarf: &Detail{Module: "package_deploy"}}

	err := runPackageDeploy(context.Background(), &fakeRunner{}, args, resp)
	if err == nil {
		t.Fatal("runPackageDeploy accepted parameters naming no package")
	}
	if !strings.Contains(err.Error(), "package") {
		t.Errorf("the error did not name the missing parameter: %v", err)
	}
}

func TestRunPackageDeployReportsTheRun(t *testing.T) {
	dir := t.TempDir()

	args := argsFor(t, map[string]any{
		"package":     "oci://ghcr.io/colonel-byte/zarf/csi-local-path-provider:0.0.37-upstream",
		"kubeconfig":  "/etc/rancher/rke2/rke2.yaml",
		"components":  "local-path-images,local-path-chart",
		"status_file": filepath.Join(dir, "status.json"),
	})
	resp := &Response{Zarf: &Detail{Module: "package_deploy"}}
	run := &fakeRunner{
		lines: []string{
			`{"msg":"Loading Zarf Package"}`,
			`{"msg":"deploying component","name":"local-path-images"}`,
			`{"msg":"deploying component","name":"local-path-chart"}`,
		},
	}

	if err := runPackageDeploy(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runPackageDeploy reported %v", err)
	}

	if got, want := strings.Join(resp.Zarf.ComponentsRan, ","), "local-path-images,local-path-chart"; got != want {
		t.Errorf("componentsRan was %q, want %q", got, want)
	}
	if !resp.Changed || resp.Zarf.ChangedSignal != SignalPartial {
		t.Errorf("the run reported changed=%v signal=%q", resp.Changed, resp.Zarf.ChangedSignal)
	}
	if !strings.Contains(resp.Msg, "csi-local-path-provider") {
		t.Errorf("the message does not name what was deployed: %q", resp.Msg)
	}
	if run.got.Dir != "" {
		t.Errorf("zarf ran in %q, want the directory Ansible chose", run.got.Dir)
	}
	// Reported for the same reason init reports it: a result an operator reads after a failure
	// should say which zarf produced it. Linked in, that is a constant.
	if resp.Zarf.Version != config.CLIVersion {
		t.Errorf("the reported zarf version was %q, want the linked-in %q",
			resp.Zarf.Version, config.CLIVersion)
	}
}

func TestRunPackageDeployCheckModeSkips(t *testing.T) {
	args := argsFor(t, map[string]any{
		"package":             "oci://ghcr.io/colonel-byte/zarf/csi-local-path-provider:0.0.37-upstream",
		"_ansible_check_mode": true,
	})
	resp := &Response{Zarf: &Detail{Module: "package_deploy"}}
	run := &fakeRunner{}

	if err := runPackageDeploy(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runPackageDeploy reported %v", err)
	}
	if !resp.Skipped || resp.Changed {
		t.Errorf("a check-mode run reported skipped=%v changed=%v", resp.Skipped, resp.Changed)
	}
	if run.got.Args != nil {
		t.Errorf("a check-mode run executed %q", run.got.Args)
	}
}
