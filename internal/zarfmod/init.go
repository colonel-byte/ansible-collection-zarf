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
	"path/filepath"
	"strings"

	"github.com/zarf-dev/zarf/src/config"
)

// The flags of zarf init the module renders. They are spelled out here because a module parameter
// is not a flag -- the names differ, several parameters render nothing, and verify has to be
// rendered in the equals form -- so the rendering is this module's job either way.
//
// What they are no longer is unverifiable. zarf's command tree is linked in, so
// TestInitArgsAreKnownFlags parses what this renders against the real init command and pflag is
// what decides whether a flag exists. A zarf release that renames one fails that test rather than
// a playbook, which is what the recorded list this replaced could never manage. See
// LibraryRunner.
const (
	flagAgentMutationPolicy        = "agent-mutation-policy"
	flagAgentTLSCA                 = "agent-tls-ca"
	flagAgentTLSCert               = "agent-tls-cert"
	flagAgentTLSKey                = "agent-tls-key"
	flagArchitecture               = "architecture"
	flagCache                      = "cache"
	flagComponents                 = "components"
	flagConfirm                    = "confirm"
	flagForceConflicts             = "force-conflicts"
	flagGitPullPassword            = "git-pull-password"
	flagGitPullUsername            = "git-pull-username"
	flagGitPushPassword            = "git-push-password"
	flagGitPushUsername            = "git-push-username"
	flagGitURL                     = "git-url"
	flagInjectorImage              = "injector-image"
	flagInjectorPort               = "injector-port"
	flagInsecureSkipTLSVerify      = "insecure-skip-tls-verify"
	flagKey                        = "key"
	flagLogFormat                  = "log-format"
	flagLogLevel                   = "log-level"
	flagNoColor                    = "no-color"
	flagOCIConcurrency             = "oci-concurrency"
	flagPlainHTTP                  = "plain-http"
	flagRegistryMode               = "registry-mode"
	flagRegistryPort               = "registry-port"
	flagRegistryPullPassword       = "registry-pull-password"
	flagRegistryPullUsername       = "registry-pull-username"
	flagRegistryPushPassword       = "registry-push-password"
	flagRegistryPushUsername       = "registry-push-username"
	flagRegistrySecret             = "registry-secret"
	flagRegistryURL                = "registry-url"
	flagRetries                    = "retries"
	flagSetValues                  = "set-values"
	flagSetVariables               = "set-variables"
	flagSkipValuesSchemaValidation = "skip-values-schema-validation"
	flagStorageClass               = "storage-class"
	flagTakeOwnership              = "take-ownership"
	flagTimeout                    = "timeout"
	flagTmpdir                     = "tmpdir"
	flagValues                     = "values"
	flagVerify                     = "verify"
)

// credentialFlags are the flags whose values are secrets. The response reports the command line it
// ran so an operator can reproduce it, and reporting these with their values would write registry
// and git passwords into every log the task's result reaches.
var credentialFlags = map[string]bool{
	flagGitPullPassword:      true,
	flagGitPushPassword:      true,
	flagRegistryPullPassword: true,
	flagRegistryPushPassword: true,
	flagRegistrySecret:       true,
}

// commandName is the first word of the command a result reports. Nothing is executed by that
// name -- zarf is linked in -- so it exists to give an operator reading a failed task a command
// line they could run by hand.
const commandName = "zarf"

// initParams is the zarf_init module's parameter surface: everything both wrappers take, plus the
// init package and everything that configures the registry and git server the init package deploys.
//
// Every optional value is a pointer or an empty-able type for one reason: a parameter the operator
// did not set must render no flag at all. Zarf reads a config file of its own, and a module that
// renders a flag for an unset parameter overrules whatever that file said, which is the opposite
// of an operator saying nothing.
type initParams struct {
	commonParams

	// InitPackage is the init package to deploy: a path to a staged tarball, an oci:// reference,
	// or an https:// URL. It is the positional argument of the command, which zarf init has taken
	// since v0.72.0. The file needs no particular name, because nothing looks it up by name any
	// more.
	//
	// Directory, in commonParams, is where relative paths resolve from -- both a relative init
	// package and a relative path inside an operator-supplied zarf config. Giving a directory and
	// no package leaves zarf to find the package itself, which is its own documented search: the
	// working directory, then the directory holding the binary, then its cache. See workingDir.
	InitPackage string `json:"init_package"`

	StorageClass string `json:"storage_class"`

	RegistryURL          string `json:"registry_url"`
	RegistryMode         string `json:"registry_mode"`
	RegistryPort         *int   `json:"registry_port"`
	RegistrySecret       string `json:"registry_secret"`
	RegistryPushUsername string `json:"registry_push_username"`
	RegistryPushPassword string `json:"registry_push_password"`
	RegistryPullUsername string `json:"registry_pull_username"`
	RegistryPullPassword string `json:"registry_pull_password"`

	GitURL          string `json:"git_url"`
	GitPushUsername string `json:"git_push_username"`
	GitPushPassword string `json:"git_push_password"`
	GitPullUsername string `json:"git_pull_username"`
	GitPullPassword string `json:"git_pull_password"`

	InjectorImage string `json:"injector_image"`
	InjectorPort  *int   `json:"injector_port"`

	AgentMutationPolicy string `json:"agent_mutation_policy"`
	AgentTLSCA          string `json:"agent_tls_ca"`
	AgentTLSCert        string `json:"agent_tls_cert"`
	AgentTLSKey         string `json:"agent_tls_key"`
}

// runInit initialises a cluster with the staged zarf init package and reports what it deployed.
func runInit(ctx context.Context, run Runner, args *Args, resp *Response) error {
	var p initParams
	if err := args.Params(&p); err != nil {
		return err
	}

	if args.Control.CheckMode {
		// Zarf init has no dry run. Ansible skips a task whose module declared it cannot check,
		// and a binary module has nowhere to declare that, so it says so in its result. Running
		// anyway would initialise a cluster during a run an operator asked to be told about.
		resp.Skipped = true
		resp.Msg = "check mode: zarf init has no dry run, so nothing was done"
		resp.Zarf.ChangedSignal = SignalUnknown
		return nil
	}

	dir, err := workingDir(&p)
	if err != nil {
		return err
	}
	source, err := initSource(&p)
	if err != nil {
		return err
	}

	// The zarf that does the work is the one linked into this binary, so the version is a
	// constant rather than something to go and ask for. There used to be a floor checked against
	// the installed zarf here; go.mod is the floor now.
	resp.Zarf.Version = config.CLIVersion

	hb := NewHeartbeat(p.StatusFile)
	defer hb.Clear()
	resp.Zarf.StatusFile = hb.Path()

	prog := newProgress(hb, componentCount(p.Components))
	argv := buildInitArgs(&p, source)

	resp.Zarf.Command = append([]string{commandName}, redact(argv)...)
	resp.Zarf.Directory = dir

	res, runErr := run.Run(ctx, Invocation{
		Args:   argv,
		Dir:    dir,
		Env:    p.childEnv(hb),
		OnLine: prog.line,
	})
	resp.Zarf.ExitCode = res.ExitCode
	resp.Zarf.ComponentsRan = prog.components

	if runErr != nil {
		prog.finish("failed", runErr)
		reportChanged(resp, prog)
		return fmt.Errorf("zarf init failed: %w", runErr)
	}
	prog.finish("completed", nil)

	reportChanged(resp, prog)
	resp.Msg = "initialised the cluster"
	return nil
}

// reportChanged fills in the changed field and the signal behind it.
//
// Zarf does not report whether a component found the cluster already in the state it wanted, so
// the honest answer is never complete. What the wrapper knows is which components ran, and that
// none of them said: changed is true, the signal is partial, and the components that did not
// declare are named. Claiming changed: false because nothing was observed would tell a playbook
// to skip the handlers that follow an initialisation that did happen.
func reportChanged(resp *Response, prog *progress) {
	resp.Changed = true
	if len(prog.components) == 0 {
		resp.Zarf.ChangedSignal = SignalUnknown
		return
	}
	resp.Zarf.ChangedSignal = SignalPartial
	resp.Zarf.ChangedUndeclared = prog.components
}

// workingDir is the directory zarf runs in.
//
// It is no longer how the init package is chosen -- the package is the command's positional
// argument -- so the two parameters can now be set together and mean two different things: which
// package, and where relative paths resolve from. What is still refused is neither of them, since
// that leaves zarf searching a working directory the module did not choose: a playbook task runs
// wherever Ansible happens to have put the module, and an init that silently picked up a package
// from there, or downloaded one, is worse than a task that says what it needs.
//
// A directory given as the init package is still honoured as a directory. Zarf would reject it as
// a package source, and an operator who pointed at their staging directory meant the search zarf
// does inside it.
func workingDir(p *initParams) (string, error) {
	if p.Directory != "" {
		return p.Directory, nil
	}
	if p.InitPackage == "" {
		return "", errors.New(`one of the "init_package" or "directory" parameters is required: ` +
			`without either, zarf init searches whatever directory the module was run in`)
	}
	if isRemoteSource(p.InitPackage) {
		// Nothing local is being read, so there is no directory the run belongs in. Zarf resolves
		// its cache from the environment, not from here.
		return "", nil
	}

	info, err := os.Stat(p.InitPackage)
	if err != nil {
		return "", fmt.Errorf("unable to read the init package %s: %w", p.InitPackage, err)
	}
	if info.IsDir() {
		return p.InitPackage, nil
	}
	return filepath.Dir(p.InitPackage), nil
}

// initSource is the package source the command is given, empty when zarf is left to search.
//
// A local path is passed as the absolute path it resolves to rather than relative to the working
// directory the run is about to take. Both work, and the absolute form is what makes the reported
// command line reproducible by hand from any directory.
func initSource(p *initParams) (string, error) {
	if p.InitPackage == "" {
		return "", nil
	}
	if isRemoteSource(p.InitPackage) {
		return p.InitPackage, nil
	}

	info, err := os.Stat(p.InitPackage)
	if err != nil {
		return "", fmt.Errorf("unable to read the init package %s: %w", p.InitPackage, err)
	}
	if info.IsDir() {
		// The directory is the working directory instead; zarf searches it for a package named
		// after its own version, which is what it does when given no source at all.
		return "", nil
	}
	return filepath.Abs(p.InitPackage)
}

// componentCount is how many components the run will deploy, 0 when the wrapper cannot know. A
// heartbeat with a total of 0 renders as "phase 2" rather than "phase 2/8", which is what an
// honest unknown looks like in a progress display.
func componentCount(components string) int {
	if strings.TrimSpace(components) == "" {
		return 0
	}
	n := 0
	for _, part := range strings.Split(components, ",") {
		if strings.TrimSpace(part) != "" {
			n++
		}
	}
	return n
}

// buildInitArgs renders the parameters as the zarf command line an operator would have typed.
func buildInitArgs(p *initParams, source string) []string {
	// The source is zarf init's positional PACKAGE_SOURCE. Empty leaves zarf to search, which is
	// what a directory-only invocation asks for.
	var positional []string
	if source != "" {
		positional = append(positional, source)
	}
	c := newCommand("init", positional...)
	p.applyCommonFlags(c)

	c.flag(flagStorageClass, p.StorageClass)

	c.flag(flagRegistryURL, p.RegistryURL)
	c.flag(flagRegistryMode, p.RegistryMode)
	c.intFlag(flagRegistryPort, p.RegistryPort)
	c.flag(flagRegistrySecret, p.RegistrySecret)
	c.flag(flagRegistryPushUsername, p.RegistryPushUsername)
	c.flag(flagRegistryPushPassword, p.RegistryPushPassword)
	c.flag(flagRegistryPullUsername, p.RegistryPullUsername)
	c.flag(flagRegistryPullPassword, p.RegistryPullPassword)

	c.flag(flagGitURL, p.GitURL)
	c.flag(flagGitPushUsername, p.GitPushUsername)
	c.flag(flagGitPushPassword, p.GitPushPassword)
	c.flag(flagGitPullUsername, p.GitPullUsername)
	c.flag(flagGitPullPassword, p.GitPullPassword)

	c.flag(flagInjectorImage, p.InjectorImage)
	c.intFlag(flagInjectorPort, p.InjectorPort)

	c.flag(flagAgentMutationPolicy, p.AgentMutationPolicy)
	c.flag(flagAgentTLSCA, p.AgentTLSCA)
	c.flag(flagAgentTLSCert, p.AgentTLSCert)
	c.flag(flagAgentTLSKey, p.AgentTLSKey)

	return c.args()
}

// redact replaces the value of every credential flag in an argument vector.
func redact(argv []string) []string {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		out = append(out, argv[i])
		name, ok := strings.CutPrefix(argv[i], "--")
		if !ok || !credentialFlags[name] || i+1 >= len(argv) {
			continue
		}
		out = append(out, "<redacted>")
		i++
	}
	return out
}
