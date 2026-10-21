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
	"fmt"
	"os"
	"strings"

	"github.com/zarf-dev/zarf/src/cmd"
	"github.com/zarf-dev/zarf/src/config"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// kubectlParams is the zarf_kubectl module's parameter surface.
//
// It shares nothing with the two converging modules, and that is the point: `zarf tools kubectl`
// is vendored kubectl, and every flag in commonParams -- --confirm, --log-format, --retries -- is
// a zarf flag kubectl would refuse. What an operator passes to kubectl is kubectl's own argument
// vector, so the module takes it as one and renders nothing into it.
type kubectlParams struct {
	// Args is the kubectl argument vector, as a list rather than a string: a shell is what splits
	// a command line into words, and there is no shell here, so a single string would have to be
	// split by this module and would get a quoted selector wrong. It is positional, so no flag.
	Args []string `json:"args"`
	// Kubeconfig is the cluster to act on, passed as KUBECONFIG. Zarf has no --kubeconfig flag;
	// kubectl does, and an operator who prefers it can pass it in Args.
	Kubeconfig string `json:"kubeconfig"`
}

// runKubectl runs one kubectl command through the zarf linked into this binary.
//
// It is a passthrough, which is the whole of its design. Zarf already vendors kubectl and already
// resolves which cluster to talk to, so a module that projected kubectl's own flag surface into
// Ansible parameters would be reimplementing kubectl in YAML. What this adds over the command
// module is that there is no kubectl to install: the one zarf ships is in this binary.
func runKubectl(ctx context.Context, run Runner, args *Args, resp *Response) error {
	var p kubectlParams
	if err := args.Params(&p); err != nil {
		return err
	}
	if len(p.Args) == 0 {
		return errors.New(`the "args" parameter is required: it is the kubectl argument vector, ` +
			`as a list such as ["get", "pods", "-A"]`)
	}
	for _, arg := range p.Args {
		if arg == "" {
			return errors.New(`the "args" parameter holds an empty entry: kubectl would read it ` +
				`as an empty positional argument, which is never what was meant`)
		}
	}

	if err := validateKubectlCommand(p.Args); err != nil {
		return err
	}

	if args.Control.CheckMode {
		// Skipped rather than run, because this module cannot tell a `get` from an `apply`: the
		// argument vector is kubectl's, and nothing here parses it. Running the command anyway
		// during a run an operator asked to be told about is the one thing check mode must not do.
		resp.Skipped = true
		resp.Msg = "check mode: zarf_kubectl cannot tell a read from a write, so nothing was done"
		resp.Zarf.ChangedSignal = SignalUnknown
		return nil
	}

	resp.Zarf.Version = config.CLIVersion

	argv := append([]string{"tools", "kubectl"}, p.Args...)
	resp.Zarf.Command = append([]string{commandName}, argv...)

	// kubectl writes its answer to os.Stdout, and it takes that file when zarf builds its command
	// tree -- which happens inside the runner, so this swap has to be in place before the call.
	// By then os.Stdout is the stdout guard's stand-in for stderr, so without this the output
	// would reach an operator only as module stderr. A file rather than a pipe: nothing drains a
	// pipe while the command runs, and `get -o yaml` over a large cluster would fill one and hang.
	restore, err := captureStdout()
	if err != nil {
		return err
	}

	res, runErr := runCatchingKubectlExit(ctx, run, Invocation{
		Args: argv,
		Env:  p.env(),
	})
	resp.Zarf.Output = restore()
	resp.Zarf.ExitCode = res.ExitCode

	if runErr != nil {
		return fmt.Errorf("zarf tools kubectl failed: %w", runErr)
	}

	// Changed, with the signal saying how much that is worth. The argument vector is kubectl's, so
	// this module knows whether the command succeeded and nothing about whether it altered the
	// cluster. Reporting changed: false would tell a playbook to skip the handlers after an apply
	// that did change something; a read-only command is what changed_when: false is for.
	resp.Changed = true
	resp.Zarf.ChangedSignal = SignalUnknown
	resp.Msg = "ran kubectl " + strings.Join(p.Args, " ")
	return nil
}

// validateKubectlCommand refuses an argument vector that names no kubectl command.
//
// Cobra checks for an unknown subcommand only at a root command, and kubectl's root is not one
// here -- it hangs off `zarf tools` -- while its Run prints help. So a typo reaches help text and
// a successful result: `args: [frobincate]` would report `changed: true` and no failure at all.
// Zarf's own CLI is saved from this by the plugin handler it installs, which the module
// deliberately does not have; see pruneRootShorthands.
//
// The check is cobra's own: resolve as far as the tree matches, then let the resolved command parse
// the rest. A leftover positional argument on a command that has subcommands is a subcommand that
// does not exist -- `create -f values.yaml` leaves none, because -f consumed it.
//
// It builds a command tree of its own rather than reusing the one the runner executes, because
// parsing a flag set twice appends to every repeatable flag twice.
func validateKubectlCommand(args []string) error {
	root := cmd.NewZarfCommand()
	pruneRootShorthands(root)

	resolved, rest, err := root.Find(append([]string{"tools", "kubectl"}, args...))
	if err != nil {
		return fmt.Errorf("kubectl has no command for %s: %w", strings.Join(args, " "), err)
	}
	if !resolved.HasSubCommands() {
		return nil
	}
	if err := resolved.ParseFlags(rest); err != nil {
		return fmt.Errorf("%s does not accept those flags: %w", resolved.CommandPath(), err)
	}
	if positional := resolved.Flags().Args(); len(positional) > 0 {
		return fmt.Errorf("%q is not a command of %s", positional[0], resolved.CommandPath())
	}
	return nil
}

// runCatchingKubectlExit runs the invocation with kubectl's fatal-error exit turned into an error.
//
// kubectl reports a fatal error through cmdutil.CheckErr, which prints and calls os.Exit -- the
// right answer for a CLI and the wrong one here, because a module that exits without writing its
// JSON object leaves Ansible reporting MODULE FAILURE instead of what went wrong. Every ordinary
// failure goes that way: a cluster it cannot reach, a resource that is not there, a manifest it
// will not accept. BehaviorOnFatal is kubectl's own hook for a caller that is not its main, and
// its documentation names panic as the thing to do instead.
func runCatchingKubectlExit(ctx context.Context, run Runner, in Invocation) (res Result, err error) {
	var fatal *kubectlExit
	cmdutil.BehaviorOnFatal(func(msg string, code int) {
		fatal = &kubectlExit{msg: msg, code: code}
		panic(fatal)
	})
	defer cmdutil.DefaultBehaviorOnFatal()

	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if _, ok := r.(*kubectlExit); !ok {
			// Not kubectl's exit, so not this function's to swallow.
			panic(r)
		}
		res, err = Result{ExitCode: fatal.code}, fatal
	}()

	return run.Run(ctx, in)
}

// kubectlExit is the fatal error kubectl would have exited the process over.
type kubectlExit struct {
	msg  string
	code int
}

func (e *kubectlExit) Error() string {
	// kubectl prefixes its own messages with "error: ", which reads badly wrapped in another
	// error's sentence.
	return strings.TrimPrefix(strings.TrimSpace(e.msg), "error: ")
}

// env is what the module adds to the environment the command runs in.
func (p *kubectlParams) env() []string {
	var env []string
	if p.Kubeconfig != "" {
		env = append(env, "KUBECONFIG="+p.Kubeconfig)
	}
	// Ansible captures stderr as a text blob, and escape sequences in it are noise in every report
	// that blob ends up in. kubectl colours its diagnostics for a terminal; this is not one.
	env = append(env, "NO_COLOR=1")
	return env
}

// captureStdout points os.Stdout at a temporary file and hands back a function that puts os.Stdout
// back and returns what was written to it. The returned function is called once, by the caller.
func captureStdout() (func() string, error) {
	f, err := os.CreateTemp("", "zarf-kubectl-stdout-*")
	if err != nil {
		return nil, fmt.Errorf("unable to capture kubectl's output: %w", err)
	}
	previous := os.Stdout
	os.Stdout = f

	return func() string {
		os.Stdout = previous
		defer os.Remove(f.Name()) //nolint:errcheck // a temporary file in TMPDIR
		defer f.Close()           //nolint:errcheck // read below is what had to succeed

		b, err := os.ReadFile(f.Name())
		if err != nil {
			return ""
		}
		return strings.TrimRight(string(b), "\n")
	}, nil
}
