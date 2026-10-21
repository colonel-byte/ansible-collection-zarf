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
	"github.com/zarf-dev/zarf/src/config"
)

// Tests for the zarf_init module: the command line it renders, the flags that command line is
// held against -- zarf's own init command, linked in -- what it refuses, and what it reports
// having done.

func TestBuildInitArgs(t *testing.T) {
	ownership := true
	plainHTTP := false
	port := 31999

	tests := []struct {
		name   string
		params initParams
		source string
		want   []string
	}{
		{
			name:   "the flags every run renders",
			params: initParams{},
			want: []string{
				"init",
				"--confirm",
				"--no-color",
				"--log-format", "json",
			},
		},
		{
			name:   "the package source leads the argument vector",
			params: initParams{},
			source: "/srv/staging/zarf-init-amd64-v0.86.0.tar.zst",
			want: []string{
				"init",
				"/srv/staging/zarf-init-amd64-v0.86.0.tar.zst",
				"--confirm",
				"--no-color",
				"--log-format", "json",
			},
		},
		{
			name:   "an oci reference is passed through as the source",
			params: initParams{},
			source: "oci://ghcr.io/zarf-dev/packages/init:v0.86.0",
			want: []string{
				"init",
				"oci://ghcr.io/zarf-dev/packages/init:v0.86.0",
				"--confirm",
				"--no-color",
				"--log-format", "json",
			},
		},
		{
			name: "an overridden log format is not replaced",
			params: initParams{
				commonParams: commonParams{
					LogFormat: "console",
					LogLevel:  "debug",
				},
			},
			want: []string{
				"init",
				"--confirm",
				"--no-color",
				"--log-format", "console",
				"--log-level", "debug",
			},
		},
		{
			name: "a false boolean renders, an unset one does not",
			params: initParams{
				commonParams: commonParams{
					TakeOwnership: &ownership,
					PlainHTTP:     &plainHTTP,
				},
			},
			want: []string{
				"init",
				"--confirm",
				"--no-color",
				"--log-format", "json",
				"--take-ownership=true",
				"--plain-http=false",
			},
		},
		{
			name: "registry and git credentials",
			params: initParams{
				RegistryURL:          "registry.bubbles.test:5000",
				RegistryPort:         &port,
				RegistryPushUsername: "zarf-push",
				RegistryPushPassword: "hunter2",
				GitURL:               "https://git.bubbles.test",
				GitPushPassword:      "hunter3",
			},
			want: []string{
				"init",
				"--confirm",
				"--no-color",
				"--log-format", "json",
				"--registry-url", "registry.bubbles.test:5000",
				"--registry-port", "31999",
				"--registry-push-username", "zarf-push",
				"--registry-push-password", "hunter2",
				"--git-url", "https://git.bubbles.test",
				"--git-push-password", "hunter3",
			},
		},
		{
			name: "repeated and map flags render one copy per entry, sorted",
			params: initParams{
				commonParams: commonParams{
					Values: []string{"/etc/zarf/a.yaml", "/etc/zarf/b.yaml"},
					SetValues: map[string]string{
						"zebra":   "stripes",
						"leopard": "spots",
					},
					SetVariables: map[string]string{
						"DOMAIN": "bubbles.test",
					},
				},
			},
			want: []string{
				"init",
				"--confirm",
				"--no-color",
				"--log-format", "json",
				"--values", "/etc/zarf/a.yaml",
				"--values", "/etc/zarf/b.yaml",
				"--set-values", "leopard=spots",
				"--set-values", "zebra=stripes",
				"--set-variables", "DOMAIN=bubbles.test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInitArgs(&tt.params, tt.source)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildInitArgs rendered\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

// TestInitArgsAreKnownFlags parses every flag the module can render against zarf's own init
// command, so pflag is the thing that decides whether a flag exists.
//
// This used to hold the rendered flags against testdata/zarf-init-flags.txt, a recording that
// could catch a flag zarf never had but not a flag zarf renamed. Now that zarf's command tree is
// linked in, the real one answers: an unknown flag, a value a flag will not take, and a flag that
// moved between the root and the subcommand are all parse errors here. That is the check the
// wrapper could not have, and it is most of why linking zarf in is worth the dependency.
func TestInitArgsAreKnownFlags(t *testing.T) {
	// Every parameter set, so that every flag the module can render is in one vector.
	yes := true
	number := 1
	widest := initParams{
		StorageClass:         "local-path",
		RegistryURL:          "registry.bubbles.test",
		RegistryMode:         "internal",
		RegistryPort:         &number,
		RegistrySecret:       "secret",
		RegistryPushUsername: "push",
		RegistryPushPassword: "push",
		RegistryPullUsername: "pull",
		RegistryPullPassword: "pull",
		GitURL:               "https://git.bubbles.test",
		GitPushUsername:      "push",
		GitPushPassword:      "push",
		GitPullUsername:      "pull",
		GitPullPassword:      "pull",
		InjectorImage:        "registry.bubbles.test/injector:1",
		InjectorPort:         &number,
		AgentMutationPolicy:  "all",
		AgentTLSCA:           "/etc/zarf/ca.pem",
		AgentTLSCert:         "/etc/zarf/tls.pem",
		AgentTLSKey:          "/etc/zarf/tls.key",
		commonParams: commonParams{
			Components:            "zarf-registry",
			TakeOwnership:         &yes,
			ForceConflicts:        &yes,
			SkipValuesSchemaValid: &yes,
			InsecureSkipTLSVerify: &yes,
			PlainHTTP:             &yes,
			Retries:               &number,
			OCIConcurrency:        &number,
			Architecture:          "amd64",
			Cache:                 "/var/cache/zarf",
			Tmpdir:                "/var/tmp/zarf",
			Timeout:               "30m",
			PublicKey:             "/etc/zarf/cosign.pub",
			Verify:                "always",
			LogLevel:              "debug",
			LogFormat:             "json",
			Values:                []string{"/etc/zarf/values.yaml"},
			SetValues:             map[string]string{"key": "value"},
			SetVariables:          map[string]string{"KEY": "value"},
		},
	}

	const source = "/srv/staging/zarf-init-amd64-v0.88.0.tar.zst"
	argv := buildInitArgs(&widest, source)

	// Find resolves the subcommand the way an invocation would, so a flag the module puts on init
	// that zarf keeps on the root is still parsed through the inherited set rather than missed.
	initCmd, flags, err := cmd.NewZarfCommand().Find(argv)
	if err != nil {
		t.Fatalf("zarf has no command for %q: %v", argv, err)
	}
	if initCmd.Name() != "init" {
		t.Fatalf("the module renders %q, which zarf resolves to %q", argv, initCmd.CommandPath())
	}

	if err := initCmd.ParseFlags(flags); err != nil {
		t.Fatalf("the module renders something zarf init does not accept: %v", err)
	}

	// Find hands back the positional PACKAGE_SOURCE alongside the flags, so prove it survived as
	// one argument rather than being eaten as some flag's value. A flag with a no-argument default
	// written in the space form does exactly that, which is the mistake --verify=always exists to
	// avoid, and it shows up here as a second positional rather than as a parse error.
	if got := initCmd.Flags().Args(); len(got) != 1 || got[0] != source {
		t.Errorf("zarf reads the positional arguments as %q, want exactly [%s]", got, source)
	}
}

func TestRedact(t *testing.T) {
	argv := []string{
		"init",
		"--registry-push-password", "hunter2",
		"--git-pull-password", "hunter3",
		"--registry-url", "registry.bubbles.test",
	}
	want := []string{
		"init",
		"--registry-push-password", "<redacted>",
		"--git-pull-password", "<redacted>",
		"--registry-url", "registry.bubbles.test",
	}

	got := redact(argv)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("redact returned\n  %v\nwant\n  %v", got, want)
	}
}

func TestRunInitCheckModeSkips(t *testing.T) {
	args := argsFor(t, map[string]any{
		"init_package":        "/srv/staging/zarf-init-amd64-v0.70.0.tar.zst",
		"_ansible_check_mode": true,
	})
	resp := &Response{Zarf: &Detail{Module: "init"}}
	run := &fakeRunner{}

	if err := runInit(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runInit reported %v", err)
	}
	if !resp.Skipped {
		t.Error("a check-mode run was not reported as skipped")
	}
	if resp.Changed {
		t.Error("a check-mode run reported changed")
	}
	if run.got.Args != nil {
		t.Errorf("a check-mode run executed %q", run.got.Args)
	}
	if resp.Zarf.ChangedSignal != SignalUnknown {
		t.Errorf("the changed signal was %q, want %q", resp.Zarf.ChangedSignal, SignalUnknown)
	}
}

func TestRunInitReportsTheRun(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "zarf-init-amd64-v0.70.0.tar.zst")
	if err := os.WriteFile(pkg, []byte("not really a package"), 0o600); err != nil {
		t.Fatalf("unable to stage the test init package: %v", err)
	}

	args := argsFor(t, map[string]any{
		"init_package":           pkg,
		"kubeconfig":             "/etc/rancher/rke2/rke2.yaml",
		"registry_push_password": "hunter2",
		"status_file":            filepath.Join(dir, "status.json"),
	})
	resp := &Response{Zarf: &Detail{Module: "init"}}
	run := &fakeRunner{
		lines: []string{
			`{"msg":"Loading Zarf Package"}`,
			`{"msg":"Deploying component","name":"zarf-injector"}`,
			`{"msg":"Deploying component","name":"zarf-seed-registry"}`,
		},
	}

	if err := runInit(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runInit reported %v", err)
	}

	if run.got.Dir != dir {
		t.Errorf("zarf ran in %q, want the init package's directory %q", run.got.Dir, dir)
	}
	if len(run.got.Args) < 2 || run.got.Args[0] != "init" || run.got.Args[1] != pkg {
		t.Errorf("zarf was run as %v, want the init package as the positional source %q", run.got.Args, pkg)
	}
	if resp.Zarf.Version != config.CLIVersion {
		t.Errorf("the reported zarf version was %q, want the linked-in %q",
			resp.Zarf.Version, config.CLIVersion)
	}
	if len(resp.Zarf.Command) == 0 || resp.Zarf.Command[0] != commandName {
		t.Errorf("the reported command was %v, want it to begin with %q",
			resp.Zarf.Command, commandName)
	}
	if !containsEnv(run.got.Env, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml") {
		t.Errorf("the child environment was %v, want it to carry KUBECONFIG", run.got.Env)
	}
	if got, want := strings.Join(resp.Zarf.ComponentsRan, ","), "zarf-injector,zarf-seed-registry"; got != want {
		t.Errorf("componentsRan was %q, want %q", got, want)
	}
	if !resp.Changed || resp.Zarf.ChangedSignal != SignalPartial {
		t.Errorf("the run reported changed=%v signal=%q", resp.Changed, resp.Zarf.ChangedSignal)
	}
	if strings.Contains(strings.Join(resp.Zarf.Command, " "), "hunter2") {
		t.Errorf("the reported command line carried a password: %v", resp.Zarf.Command)
	}
}

func TestRunInitRequiresAPackageOrADirectory(t *testing.T) {
	args := argsFor(t, map[string]any{})
	resp := &Response{Zarf: &Detail{Module: "init"}}

	err := runInit(context.Background(), &fakeRunner{}, args, resp)
	if err == nil {
		t.Fatal("runInit accepted parameters naming neither a package nor a directory")
	}
	if !strings.Contains(err.Error(), "init_package") {
		t.Errorf("the error did not name the missing parameter: %v", err)
	}
}

// TestInitSourceAndWorkingDir covers what the two parameters now mean separately: the package is
// the command's positional source, and the directory is only where relative paths resolve from.
// Before zarf v0.72.0 the package could only be chosen by the directory, and the two could not be
// set together.
func TestInitSourceAndWorkingDir(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "zarf-init-amd64-v0.86.0.tar.zst")
	if err := os.WriteFile(pkg, []byte("not really a package"), 0o600); err != nil {
		t.Fatalf("unable to stage the test init package: %v", err)
	}
	elsewhere := t.TempDir()

	cases := []struct {
		name       string
		params     initParams
		wantSource string
		wantDir    string
	}{
		{
			name:       "a staged file is the source, and its directory is where zarf runs",
			params:     initParams{InitPackage: pkg},
			wantSource: pkg,
			wantDir:    dir,
		},
		{
			name:       "a directory is the working directory, and zarf searches it",
			params:     initParams{InitPackage: dir},
			wantSource: "",
			wantDir:    dir,
		},
		{
			name: "both parameters say different things now",
			params: initParams{
				InitPackage: pkg,
				commonParams: commonParams{
					Directory: elsewhere,
				},
			},
			wantSource: pkg,
			wantDir:    elsewhere,
		},
		{
			name:       "a remote reference needs no working directory",
			params:     initParams{InitPackage: "oci://ghcr.io/zarf-dev/packages/init:v0.86.0"},
			wantSource: "oci://ghcr.io/zarf-dev/packages/init:v0.86.0",
			wantDir:    "",
		},
		{
			name:       "a directory alone leaves zarf to search it",
			params:     initParams{commonParams: commonParams{Directory: dir}},
			wantSource: "",
			wantDir:    dir,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, err := initSource(&tc.params)
			if err != nil {
				t.Fatalf("initSource reported %v", err)
			}
			if source != tc.wantSource {
				t.Errorf("initSource was %q, want %q", source, tc.wantSource)
			}

			gotDir, err := workingDir(&tc.params)
			if err != nil {
				t.Fatalf("workingDir reported %v", err)
			}
			if gotDir != tc.wantDir {
				t.Errorf("workingDir was %q, want %q", gotDir, tc.wantDir)
			}
		})
	}
}
