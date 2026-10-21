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
	"strings"
	"testing"

	"github.com/zarf-dev/zarf/src/cmd"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// Tests for the zarf_kubectl module: the argument vector it hands zarf, what it refuses, and what
// it reports having done.

func TestRunKubectlPassesTheArgumentVector(t *testing.T) {
	args := argsFor(t, map[string]any{
		"args":       []string{"get", "pods", "-A", "-o", "json"},
		"kubeconfig": "/etc/rancher/rke2/rke2.yaml",
	})
	resp := &Response{Zarf: &Detail{Module: "kubectl"}}
	run := &fakeRunner{}

	if err := runKubectl(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runKubectl reported %v", err)
	}

	want := []string{"tools", "kubectl", "get", "pods", "-A", "-o", "json"}
	if strings.Join(run.got.Args, " ") != strings.Join(want, " ") {
		t.Errorf("zarf was run as %v, want %v", run.got.Args, want)
	}
	if !containsEnv(run.got.Env, "KUBECONFIG=/etc/rancher/rke2/rke2.yaml") {
		t.Errorf("the environment was %v, want it to carry KUBECONFIG", run.got.Env)
	}
	// Unknown rather than partial: kubectl's argument vector is not something this module reads,
	// so a successful run says nothing about whether the cluster changed.
	if !resp.Changed || resp.Zarf.ChangedSignal != SignalUnknown {
		t.Errorf("the run reported changed=%v signal=%q", resp.Changed, resp.Zarf.ChangedSignal)
	}
	if len(resp.Zarf.Command) == 0 || resp.Zarf.Command[0] != commandName {
		t.Errorf("the reported command was %v, want it to begin with %q",
			resp.Zarf.Command, commandName)
	}
}

func TestRunKubectlRefusesAnEmptyArgumentVector(t *testing.T) {
	tests := []struct {
		name string
		args any
	}{
		{
			name: "no args at all",
			args: []string{},
		},
		{
			name: "an empty entry",
			args: []string{"get", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := argsFor(t, map[string]any{"args": tt.args})
			resp := &Response{Zarf: &Detail{Module: "kubectl"}}
			run := &fakeRunner{}

			if err := runKubectl(context.Background(), run, args, resp); err == nil {
				t.Fatal("runKubectl accepted it")
			}
			if run.got.Args != nil {
				t.Errorf("it ran %q anyway", run.got.Args)
			}
		})
	}
}

func TestRunKubectlCheckModeSkips(t *testing.T) {
	args := argsFor(t, map[string]any{
		"args":                []string{"apply", "-f", "/srv/manifests"},
		"_ansible_check_mode": true,
	})
	resp := &Response{Zarf: &Detail{Module: "kubectl"}}
	run := &fakeRunner{}

	if err := runKubectl(context.Background(), run, args, resp); err != nil {
		t.Fatalf("runKubectl reported %v", err)
	}
	if !resp.Skipped || resp.Changed {
		t.Errorf("a check-mode run reported skipped=%v changed=%v", resp.Skipped, resp.Changed)
	}
	if run.got.Args != nil {
		t.Errorf("a check-mode run executed %q", run.got.Args)
	}
}

// TestKubectlArgsResolveToTheVendoredCommand is the kubectl side of what
// TestInitArgsAreKnownFlags does for init: the linked-in command tree is what says the module's
// argument vector reaches kubectl at all. A zarf that moved or renamed `tools kubectl` fails here
// rather than in a playbook.
func TestKubectlArgsResolveToTheVendoredCommand(t *testing.T) {
	argv := []string{"tools", "kubectl", "get", "pods"}

	root := cmd.NewZarfCommand()
	// Through the same call the runner makes, and that is the half of this test worth having:
	// without it zarf's -l for --log-level collides with kubectl's for --selector and cobra
	// panics merging the two flag sets, which Find is enough to reach.
	pruneRootShorthands(root)

	kubectlCmd, flags, err := root.Find(argv)
	if err != nil {
		t.Fatalf("zarf has no command for %q: %v", argv, err)
	}
	// kubectl's whole command tree is registered under `zarf tools kubectl`, so Find descends
	// into `get` rather than stopping at kubectl -- which is itself the proof that the vector
	// arrives as kubectl's own rather than as positional arguments to a wrapper.
	if path := kubectlCmd.CommandPath(); path != "zarf tools kubectl get" {
		t.Fatalf("zarf resolves %q to %q", argv, path)
	}
	if strings.Join(flags, " ") != "pods" {
		t.Errorf("zarf passes %q through to kubectl get, want %q", flags, "pods")
	}
	if err := kubectlCmd.ParseFlags(flags); err != nil {
		t.Errorf("kubectl get does not accept %q: %v", flags, err)
	}
}

// TestKubectlKeepsItsOwnShorthands is the other half of the prune: the letters have to reach
// kubectl, not disappear with zarf's. `-l` is the one that collided, and an operator who passes it
// means --selector.
func TestKubectlKeepsItsOwnShorthands(t *testing.T) {
	argv := []string{"tools", "kubectl", "get", "pods", "-l", "app=zarf"}

	root := cmd.NewZarfCommand()
	pruneRootShorthands(root)

	kubectlCmd, flags, err := root.Find(argv)
	if err != nil {
		t.Fatalf("zarf has no command for %q: %v", argv, err)
	}
	if err := kubectlCmd.ParseFlags(flags); err != nil {
		t.Fatalf("kubectl get does not accept %q: %v", flags, err)
	}
	if got := kubectlCmd.Flags().Lookup("selector").Value.String(); got != "app=zarf" {
		t.Errorf("kubectl read -l as %q, want the selector %q", got, "app=zarf")
	}
	// And zarf's own long form still parses, since only the letter went.
	if f := root.PersistentFlags().Lookup("log-level"); f == nil || f.Shorthand != "" {
		t.Errorf("zarf's --log-level kept the shorthand %q", f.Shorthand)
	}
}

func TestValidateKubectlCommandRefusesWhatKubectlHasNoCommandFor(t *testing.T) {
	tests := []struct {
		name string
		args []string
		// names is what a refusal has to mention, because the operator wrote it. Empty means the
		// vector is accepted.
		names string
	}{
		{
			name: "a command that exists",
			args: []string{"get", "pods", "-A"},
		},
		{
			name: "a nested command that exists",
			args: []string{"rollout", "restart", "deployment/agent-hook"},
		},
		{
			name: "a flag value is not a subcommand",
			args: []string{"create", "-f", "/srv/manifests/secret.yaml"},
		},
		{
			name:  "a misspelled command",
			args:  []string{"frobnicate"},
			names: "frobnicate",
		},
		{
			name:  "a misspelled subcommand",
			args:  []string{"rollout", "restaart", "deployment/agent-hook"},
			names: "restaart",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKubectlCommand(tt.args)
			if tt.names == "" {
				if err != nil {
					t.Fatalf("validateKubectlCommand refused %q: %v", tt.args, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateKubectlCommand accepted %q", tt.args)
			}
			if !strings.Contains(err.Error(), tt.names) {
				t.Errorf("the error does not name %q: %v", tt.names, err)
			}
		})
	}
}

// fatalRunner fails the way kubectl does: through cmdutil.CheckErr, which prints and exits the
// process unless a caller has said otherwise. It is the ordinary failure path -- a cluster that
// cannot be reached goes through it -- so a module that exited here would exit on most bad days.
type fatalRunner struct{}

func (fatalRunner) Run(_ context.Context, _ Invocation) (Result, error) {
	cmdutil.CheckErr(errors.New("the connection to the server localhost:8080 was refused"))
	panic("unreachable: CheckErr does not return")
}

func TestRunKubectlReportsAFatalErrorInsteadOfExiting(t *testing.T) {
	args := argsFor(t, map[string]any{"args": []string{"get", "pods"}})
	resp := &Response{Zarf: &Detail{Module: "kubectl"}}

	err := runKubectl(context.Background(), fatalRunner{}, args, resp)
	if err == nil {
		t.Fatal("runKubectl reported no error for a fatal kubectl failure")
	}
	if !strings.Contains(err.Error(), "was refused") {
		t.Errorf("the error does not carry what kubectl said: %v", err)
	}
	if resp.Zarf.ExitCode != 1 {
		t.Errorf("the reported exit code was %d, want kubectl's 1", resp.Zarf.ExitCode)
	}
}
