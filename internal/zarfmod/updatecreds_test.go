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
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/zarf-dev/zarf/src/cmd"
)

// Tests for the zarf_update_creds module: the command line it renders, the flags and the service
// names that command line is held against, and what it reports having done.

func TestBuildUpdateCredsArgs(t *testing.T) {
	yes := true

	tests := []struct {
		name   string
		params updateCredsParams
		want   []string
	}{
		{
			name: "the registry subcommand",
			params: updateCredsParams{
				Service:              serviceRegistry,
				RegistryPushPassword: "hunter2",
			},
			want: []string{
				"tools", "update-creds", "registry",
				"--confirm", "--no-color", "--log-format", "json",
				"--registry-push-password", "hunter2",
			},
		},
		{
			name: "the git subcommand",
			params: updateCredsParams{
				Service:         serviceGit,
				GitURL:          "https://git.bubbles.test",
				GitPushUsername: "zarf-push",
				GitPushPassword: "hunter2",
				GitPullUsername: "zarf-pull",
				GitPullPassword: "hunter3",
				// Set and not rendered: the registry subcommand's flag, which the git one does
				// not take. runUpdateCreds refuses this combination before it is ever rendered;
				// the renderer not carrying it is the second half of that.
				RegistryPushPassword: "hunter4",
			},
			want: []string{
				"tools", "update-creds", "git",
				"--confirm", "--no-color", "--log-format", "json",
				"--git-url", "https://git.bubbles.test",
				"--git-push-username", "zarf-push",
				"--git-push-password", "hunter2",
				"--git-pull-username", "zarf-pull",
				"--git-pull-password", "hunter3",
			},
		},
		{
			name: "the agent subcommand",
			params: updateCredsParams{
				Service:        serviceAgent,
				AgentTLSCA:     "/etc/zarf/ca.pem",
				AgentTLSCert:   "/etc/zarf/tls.pem",
				AgentTLSKey:    "/etc/zarf/tls.key",
				ForceConflicts: &yes,
				LogLevel:       "debug",
			},
			want: []string{
				"tools", "update-creds", "agent",
				"--confirm", "--no-color", "--log-format", "json", "--log-level", "debug",
				"--agent-tls-ca", "/etc/zarf/ca.pem",
				"--agent-tls-cert", "/etc/zarf/tls.pem",
				"--agent-tls-key", "/etc/zarf/tls.key",
				"--force-conflicts=true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildUpdateCredsArgs(&tt.params)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Errorf("buildUpdateCredsArgs rendered\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

// TestUpdateCredsArgsAreKnownFlags parses what the module renders for each service against zarf's
// own subcommand for it. The reasoning is on TestInitArgsAreKnownFlags; what this one adds is that
// it is three commands, so a flag on the wrong one is a parse error here.
func TestUpdateCredsArgsAreKnownFlags(t *testing.T) {
	for _, service := range updateCredsServices {
		t.Run(service, func(t *testing.T) {
			argv := buildUpdateCredsArgs(fullUpdateCredsParamsFor(service))

			root := cmd.NewZarfCommand()
			pruneRootShorthands(root)

			credsCmd, flags, err := root.Find(argv)
			if err != nil {
				t.Fatalf("zarf has no command for %q: %v", argv, err)
			}
			if path, want := credsCmd.CommandPath(), "zarf tools update-creds "+service; path != want {
				t.Fatalf("the module renders %q, which zarf resolves to %q, want %q", argv, path, want)
			}
			if err := credsCmd.ParseFlags(flags); err != nil {
				t.Fatalf("the module renders something %q does not accept: %v", credsCmd.CommandPath(), err)
			}
			// The subcommands are cobra.NoArgs, so a flag value read as a positional argument
			// fails the run rather than being ignored.
			if got := credsCmd.Flags().Args(); len(got) != 0 {
				t.Errorf("zarf reads %q as positional arguments, want none", got)
			}
		})
	}
}

// TestUpdateCredsServicesHaveSubcommands holds the documented service names against zarf's own
// command tree. The names are a constant here because the module documents them as choices, and a
// zarf release that renamed or added a subcommand would otherwise leave the two disagreeing.
func TestUpdateCredsServicesHaveSubcommands(t *testing.T) {
	root := cmd.NewZarfCommand()
	pruneRootShorthands(root)

	credsCmd, _, err := root.Find([]string{"tools", "update-creds"})
	if err != nil {
		t.Fatalf("zarf has no tools update-creds command: %v", err)
	}

	var zarfs []string
	for _, sub := range credsCmd.Commands() {
		if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}
		zarfs = append(zarfs, sub.Name())
	}
	sort.Strings(zarfs)

	documented := slices.Clone(updateCredsServices)
	sort.Strings(documented)

	if got, want := strings.Join(documented, ","), strings.Join(zarfs, ","); got != want {
		t.Errorf("the module accepts %q, want zarf's own subcommands %q", got, want)
	}
}

func TestRunUpdateCredsRefusesTheWrongService(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		names  string
	}{
		{
			name:   "no service",
			params: map[string]any{"registry_push_password": "hunter2"},
			names:  "required",
		},
		{
			name:   "a service zarf has no subcommand for",
			params: map[string]any{"service": "artifact"},
			names:  "artifact",
		},
		{
			name: "a credential belonging to another service",
			params: map[string]any{
				"service":           "registry",
				"git_push_password": "hunter2",
			},
			names: "git_push_password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{Zarf: &Detail{Module: "update_creds"}}
			run := &fakeRunner{}

			err := runUpdateCreds(context.Background(), run, argsFor(t, tt.params), resp)
			if err == nil {
				t.Fatalf("runUpdateCreds accepted %v", tt.params)
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

func TestRunUpdateCredsCheckModeSkips(t *testing.T) {
	args := argsFor(t, map[string]any{
		"service":                "registry",
		"registry_push_password": "hunter2",
		"_ansible_check_mode":    true,
	})
	resp := &Response{Zarf: &Detail{Module: "update_creds"}}
	run := &fakeRunner{}

	if err := runUpdateCreds(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runUpdateCreds reported %v", err)
	}
	if !resp.Skipped || resp.Changed {
		t.Errorf("a check-mode run reported skipped=%v changed=%v", resp.Skipped, resp.Changed)
	}
	if run.got.Args != nil {
		t.Errorf("a check-mode run executed %q", run.got.Args)
	}
}

func TestRunUpdateCredsReportsTheRunWithoutTheCredentials(t *testing.T) {
	args := argsFor(t, map[string]any{
		"service":                "registry",
		"registry_push_password": "hunter2",
		"kubeconfig":             "/etc/rancher/rke2/rke2.yaml",
	})
	resp := &Response{Zarf: &Detail{Module: "update_creds"}}
	run := &fakeRunner{}

	if err := runUpdateCreds(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runUpdateCreds reported %v", err)
	}

	// The password reaches zarf and not the result: every parameter of this module is a
	// credential, so the redaction matters more here than anywhere else.
	if !strings.Contains(strings.Join(run.got.Args, " "), "hunter2") {
		t.Errorf("zarf was run without the credential: %v", run.got.Args)
	}
	if strings.Contains(strings.Join(resp.Zarf.Command, " "), "hunter2") {
		t.Errorf("the reported command carries the credential: %v", resp.Zarf.Command)
	}
	if !containsEnv(run.got.Env, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml") {
		t.Errorf("the environment was %v, want it to carry KUBECONFIG", run.got.Env)
	}
	if !resp.Changed || resp.Zarf.ChangedSignal != SignalUnknown {
		t.Errorf("the run reported changed=%v signal=%q", resp.Changed, resp.Zarf.ChangedSignal)
	}
	if !strings.Contains(resp.Msg, "registry") {
		t.Errorf("the message does not name what was updated: %q", resp.Msg)
	}
}
