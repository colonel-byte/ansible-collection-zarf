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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zarf-dev/zarf/src/cmd"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// zarfModulePath is the module path of the zarf this binary links against. It is how the version
// compiled in is found at run time, so nothing here has to be kept in step with go.mod by hand.
const zarfModulePath = "github.com/zarf-dev/zarf"

// LibraryRunner runs a zarf command inside this process, through zarf's own command tree.
//
// This is the proof ZEP-0072 actually asks for. Running the installed zarf as a child, which is
// what both modules used to do, demonstrates that zarf can be driven from outside -- which was
// never in question. This runner enters the same `zarf init` and `zarf package deploy` an
// operator's command line enters: real cobra parsing, real flag validation, the real agent TLS
// load, the real existing-state guard, the real init package search. The upstream change the
// proposal proposes is this, minus the linking: a dispatch hook in zarf's own main, which is
// already shaped to take one.
//
// Nothing above the Runner interface knows the difference. Invocation goes in and Result comes
// out, so the command rendering, the progress reader, the heartbeat and the response are the same
// code they always were.
type LibraryRunner struct{}

// Run executes the invocation against the linked-in zarf.
func (LibraryRunner) Run(ctx context.Context, in Invocation) (Result, error) {
	// Every line zarf logs is fed to the progress reader and mirrored to stderr, which is where a
	// child's streams used to be wired. Stdout belongs to the module's JSON result.
	lines := &lineWriter{onLine: in.OnLine, mirror: os.Stderr}
	defer lines.flush()

	// These are process globals rather than per-call options, which is safe only because a module
	// process runs one command and exits. zarf's own preRun builds the logger itself and would
	// discard one passed in by context, so the destination is the only hook that survives it.
	logger.DestinationDefault = lines

	// OutputWriter is where zarf prints user-facing command output, and it is the one real hazard
	// of linking zarf in: it is a *os.File initialised to os.Stdout when the cmd package loads,
	// which is before the stdout guard runs, so it holds the true file descriptor 1 that Ansible
	// parses the module's JSON from. Moving it to stderr is not tidying -- left alone, anything
	// zarf printed through it would land in the middle of that JSON object.
	cmd.OutputWriter = os.Stderr

	if in.Dir != "" {
		// Chdir rather than a per-command working directory, because there is no child to give one
		// to. Zarf resolves the init package relative to the working directory, so this is
		// load-bearing: see Invocation.Dir.
		previous, err := os.Getwd()
		if err != nil {
			return Result{}, fmt.Errorf("unable to read the working directory: %w", err)
		}
		if err := os.Chdir(in.Dir); err != nil {
			return Result{}, fmt.Errorf("unable to enter %s: %w", in.Dir, err)
		}
		defer os.Chdir(previous) //nolint:errcheck // the process is about to exit
	}

	// Set rather than passed to a child, so these outlive the call. That is what a module process
	// running one command and exiting makes safe.
	for _, kv := range in.Env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return Result{}, fmt.Errorf("unable to set %s: %w", name, err)
		}
	}

	root := cmd.NewZarfCommand()
	pruneRootShorthands(root)
	pruneInitGitPushUserDefault(root)
	root.SetArgs(in.Args)
	root.SetOut(lines)
	root.SetErr(lines)

	// The root command is built with SilenceErrors and SilenceUsage, so a failure comes back here
	// rather than being printed and exited on. Executing the tree directly also stays clear of
	// cmd.Execute, which exits the process for some tools subcommands.
	if err := root.ExecuteContext(ctx); err != nil {
		// Report it the way zarf's own Execute does, because SilenceErrors means nobody else
		// will. Without this a failure that happens before zarf logs anything -- a package that
		// is not there is the common one -- leaves an operator the module's own message and none
		// of zarf's diagnosis, which is the half that says what to do about it.
		logger.Default().Error(err.Error())
		return Result{ExitCode: 1}, err
	}
	return Result{ExitCode: 0}, nil
}

// init does the process-global configuration zarf expects its own main to have done.
//
// It is an init rather than something Run calls because the values it sets are read before any
// command runs -- a module reports config.CLIVersion in its result -- and an ordering where the
// reader goes first is a silent wrong answer, not a failure. Go has already initialised the zarf
// packages this one imports by the time this runs, so there is nothing to race with.
func init() {
	// What zarf's own main sets. Left at its default, a zarf used as a library shells out to
	// whatever zarf is on PATH to run a package's actions, which is the child process linking
	// zarf in exists to not be.
	config.ActionsUseSystemZarf = false

	// zarf stamps CLIVersion in with ldflags at release time, so linked in as a library it reads
	// "unset-development-only". That is not cosmetic: config.GetInitPackageName builds the init
	// package filename from it, so the default sends the package search looking for
	// zarf-init-<arch>-unset-development-only.tar.zst and it never finds a staged package.
	if config.CLIVersion != config.UnsetCLIVersion {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, dep := range info.Deps {
		if dep.Path == zarfModulePath {
			config.CLIVersion = dep.Version
			return
		}
	}
}

// pruneRootShorthands drops the single-letter form of every flag on zarf's root command.
//
// Cobra merges a parent's persistent flags into the command it is about to run, and the vendored
// kubectl under `zarf tools kubectl` brings flag sets of its own: `-l` is zarf's --log-level and
// kubectl's --selector, so merging the two panics -- `unable to redefine "l" shorthand`. Nothing
// here renders a shorthand, so the long forms are the whole of what a module needs, and dropping
// the letters is what makes the two sets mergeable. Duplicate long names need no handling:
// pflag's AddFlagSet skips a name the child already has.
//
// Zarf's own CLI avoids this by not installing root flags at all for a vendored command, which it
// decides by reading os.Args. Presenting the rendered command line there would work and is what an
// earlier version of this did -- but it also makes kubectl install its plugin handler, which
// os.Exit(1)s on an unknown subcommand and execs a plugin it does find. Either one leaves the
// module process without writing the JSON object Ansible is waiting for, which is a worse failure
// than the one being avoided: see docs/agent/choice-zarf-ansible-module.md.
//
// It is every shorthand rather than just `-l` because the next collision is the same bug: a zarf
// release that gives a root flag a letter kubectl already uses would panic in a playbook, and
// nothing in this repository would have seen it coming.
func pruneRootShorthands(root *cobra.Command) {
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		f.Shorthand = ""
	})
}

// pruneInitGitPushUserDefault clears the leaked default value for --git-push-username on zarf init.
//
// In zarf's package.go (mirror command registration), viper registers a default for VInitGitPushUser
// ("zarf-git-user"). Because viper is a process-level singleton, newInitCommand() picks this up when
// binding the --git-push-username flag. On a multi-phase init where the initial phase omitted git-server,
// zarf-state holds an empty GitServer struct; a subsequent init without git-push-username set then
// compares "zarf-git-user" != "" via helpers.IsNotZeroAndNotEqual and fails.
// Clearing the flag's DefValue and Value restores an empty default when unset.
func pruneInitGitPushUserDefault(root *cobra.Command) {
	initCmd, _, err := root.Find([]string{"init"})
	if err != nil || initCmd == nil {
		return
	}
	f := initCmd.Flags().Lookup(flagGitPushUsername)
	if f == nil || f.Value.String() != "zarf-git-user" {
		return
	}
	f.DefValue = ""
	// Set cannot fail for a string flag, and there is nothing to do about it if it ever did: the
	// value it is being given is the zero value the flag should have had.
	if err := f.Value.Set(""); err != nil {
		return
	}
}

// lineWriter splits what zarf writes into lines and reports each one.
//
// A log handler writes a record at a time, but nothing promises a write ends on a newline, so
// partial lines are held until the rest arrives rather than reported as lines of their own.
type lineWriter struct {
	onLine func(string)
	mirror io.Writer

	mu      sync.Mutex
	partial []byte
}

func (w *lineWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.mirror != nil {
		// Mirrored whole, so a partial line is not held back from a human reading stderr.
		fmt.Fprint(w.mirror, string(b))
	}

	w.partial = append(w.partial, b...)
	for {
		i := bytes.IndexByte(w.partial, '\n')
		if i < 0 {
			return len(b), nil
		}
		line := string(w.partial[:i])
		w.partial = w.partial[i+1:]
		if w.onLine != nil {
			w.onLine(line)
		}
	}
}

// flush reports a trailing line that never ended in a newline.
func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.partial) == 0 || w.onLine == nil {
		return
	}
	w.onLine(string(w.partial))
	w.partial = nil
}
