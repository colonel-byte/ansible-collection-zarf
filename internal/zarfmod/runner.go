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

import "context"

// Invocation is one zarf command to run.
type Invocation struct {
	// Args is the argument vector, starting with the zarf subcommand.
	Args []string
	// Dir is the working directory. Zarf init resolves the init package relative to it, so this is
	// load-bearing rather than cosmetic.
	Dir string
	// Env is set for the duration of the run, as KEY=VALUE.
	Env []string
	// OnLine is called with each line zarf logs, if set. It is how progress is read back out of
	// the log stream.
	OnLine func(string)
}

// Result is what running zarf produced.
type Result struct {
	ExitCode int
}

// Runner runs a zarf command. It is an interface so a test can prove the argument vector, the
// working directory and the progress reading without reaching zarf at all.
//
// LibraryRunner is the only implementation that reaches zarf. There was a second one that ran the
// installed zarf as a child process, which is what both modules did before they linked zarf in;
// git history has it, and docs/agent/choice-zarf-ansible-module.md records why it went.
type Runner interface {
	Run(ctx context.Context, in Invocation) (Result, error)
}
