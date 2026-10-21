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
	"fmt"
	"slices"
	"sort"

	"github.com/zarf-dev/zarf/src/config"
)

// updateCredsParams is the zarf_update_creds module's parameter surface.
//
// `zarf tools update-creds` rewrites the credentials an initialised cluster holds -- in the
// zarf-state Secret, in the service itself, and in the deployed packages that use it -- and it is
// three commands rather than one: `update-creds registry`, `git` and `agent`, each taking only its
// own credentials. The bare `zarf tools update-creds`, which takes all of them and an optional
// service name, warns that it is deprecated, so the module renders the subcommand and nothing
// renders the deprecated form.
//
// The artifact server is the fourth service zarf keeps credentials for and has no subcommand: it
// is deprecated upstream, so this module does not update it.
type updateCredsParams struct {
	// Service is which credentials to rewrite, and selects the subcommand. It is required,
	// because each subcommand takes a different set of the parameters below and a module that
	// guessed would render flags the command refuses.
	Service string `json:"service"`

	// Kubeconfig is the cluster whose credentials are being rewritten. Zarf has no --kubeconfig
	// flag, so it is passed as KUBECONFIG in the environment.
	Kubeconfig string `json:"kubeconfig"`
	// ZarfConfig is an operator-supplied zarf config file, passed as ZARF_CONFIG. It is the way to
	// keep the new credentials out of the argument vector, which matters more here than anywhere
	// else in the collection: every parameter below is a credential or the path to one.
	ZarfConfig string `json:"zarf_config"`

	RegistryURL          string `json:"registry_url"`
	RegistryPushUsername string `json:"registry_push_username"`
	RegistryPushPassword string `json:"registry_push_password"`
	RegistryPullUsername string `json:"registry_pull_username"`
	RegistryPullPassword string `json:"registry_pull_password"`

	GitURL          string `json:"git_url"`
	GitPushUsername string `json:"git_push_username"`
	GitPushPassword string `json:"git_push_password"`
	GitPullUsername string `json:"git_pull_username"`
	GitPullPassword string `json:"git_pull_password"`

	AgentTLSCA   string `json:"agent_tls_ca"`
	AgentTLSCert string `json:"agent_tls_cert"`
	AgentTLSKey  string `json:"agent_tls_key"`

	ForceConflicts *bool `json:"force_conflicts"`

	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

// The services zarf has an update-creds subcommand for, which is what the service parameter takes.
const (
	serviceAgent    = "agent"
	serviceGit      = "git"
	serviceRegistry = "registry"
)

// forceConflictsServices are the subcommands that take --force-conflicts. The git one does not:
// it rewrites credentials in the git server and the Secret and patches no Helm-owned resource, so
// there is no ownership to force. Rendering it there is an unknown flag, which is why the
// parameter is refused rather than dropped.
var forceConflictsServices = []string{serviceRegistry, serviceAgent}

// updateCredsServices are those three, in the order the documentation lists them.
// TestUpdateCredsServicesHaveSubcommands holds them against zarf's own command tree.
var updateCredsServices = []string{serviceRegistry, serviceGit, serviceAgent}

// credentialsByService is which parameters each subcommand accepts, by the name an operator wrote.
// It exists so that a credential given for the wrong service is refused by name rather than
// rendered as a flag the subcommand has never heard of -- which reaches an operator as a zarf
// usage error about a module that looks broken.
func (p *updateCredsParams) credentialsByService() map[string]map[string]string {
	return map[string]map[string]string{
		serviceRegistry: {
			"registry_url":           p.RegistryURL,
			"registry_push_username": p.RegistryPushUsername,
			"registry_push_password": p.RegistryPushPassword,
			"registry_pull_username": p.RegistryPullUsername,
			"registry_pull_password": p.RegistryPullPassword,
		},
		serviceGit: {
			"git_url":           p.GitURL,
			"git_push_username": p.GitPushUsername,
			"git_push_password": p.GitPushPassword,
			"git_pull_username": p.GitPullUsername,
			"git_pull_password": p.GitPullPassword,
		},
		serviceAgent: {
			"agent_tls_ca":   p.AgentTLSCA,
			"agent_tls_cert": p.AgentTLSCert,
			"agent_tls_key":  p.AgentTLSKey,
		},
	}
}

// misplaced names the parameters that were set and belong to a service other than the one named.
func (p *updateCredsParams) misplaced() []string {
	var wrong []string
	for service, credentials := range p.credentialsByService() {
		if service == p.Service {
			continue
		}
		for name, value := range credentials {
			if value != "" {
				wrong = append(wrong, name)
			}
		}
	}
	sort.Strings(wrong)
	return wrong
}

// runUpdateCreds rewrites the credentials an initialised cluster holds.
//
// It is the write half of zarf_state_info, which reads the same Secret and withholds what it
// finds. Rotating a registry password without it means doing by hand what zarf does in one
// command: the Secret, the service's own configuration, and every deployed package that pulls
// through it.
func runUpdateCreds(ctx context.Context, run Runner, args *Args, resp *Response) error {
	var p updateCredsParams
	if err := args.Params(&p); err != nil {
		return err
	}
	if p.Service == "" {
		return fmt.Errorf("the \"service\" parameter is required: it is the credentials to "+
			"rewrite, one of %s", quoteAll(updateCredsServices))
	}
	if !slices.Contains(updateCredsServices, p.Service) {
		return fmt.Errorf("the \"service\" parameter is %q: zarf updates the credentials of %s",
			p.Service, quoteAll(updateCredsServices))
	}
	if p.ForceConflicts != nil && !slices.Contains(forceConflictsServices, p.Service) {
		return fmt.Errorf("the \"force_conflicts\" parameter does not apply to %q: zarf takes it "+
			"on %s, which are the subcommands that patch Helm-owned resources",
			p.Service, quoteAll(forceConflictsServices))
	}
	if wrong := p.misplaced(); len(wrong) > 0 {
		return fmt.Errorf("%q does not accept %s, which belong to another service: zarf updates "+
			"one service's credentials per run, so a playbook rotating two writes two tasks",
			p.Service, quoteAll(wrong))
	}

	if args.Control.CheckMode {
		// Skipped, as every converging module here is: the command has no dry run, and rotating a
		// cluster's credentials during a run an operator asked to be told about would be the
		// worst of the collection's check-mode mistakes to make.
		resp.Skipped = true
		resp.Msg = "check mode: zarf tools update-creds has no dry run, so nothing was changed"
		resp.Zarf.ChangedSignal = SignalUnknown
		return nil
	}

	resp.Zarf.Version = config.CLIVersion

	argv := buildUpdateCredsArgs(&p)
	resp.Zarf.Command = append([]string{commandName}, redact(argv)...)

	res, runErr := run.Run(ctx, Invocation{
		Args: argv,
		Env:  p.env(),
	})
	resp.Zarf.ExitCode = res.ExitCode

	if runErr != nil {
		return fmt.Errorf("zarf tools update-creds failed: %w", runErr)
	}

	// Unknown, and for the plainest version of the collection's usual reason: zarf reports that it
	// updated the service, not whether it already held what was given. A credential set twice is
	// reported as changed both times.
	resp.Changed = true
	resp.Zarf.ChangedSignal = SignalUnknown
	resp.Msg = "updated the " + p.Service + " credentials"
	return nil
}

// buildUpdateCredsArgs renders the parameters as the zarf command line an operator would have
// typed: the subcommand for the named service, and only the credentials that subcommand takes.
func buildUpdateCredsArgs(p *updateCredsParams) []string {
	c := newCommand("tools", "update-creds", p.Service)

	// --confirm because there is no terminal on the other end of a playbook task, and this
	// command asks before it rewrites anything.
	c.bare(flagConfirm)
	c.bare(flagNoColor)

	format := p.LogFormat
	if format == "" {
		format = "json"
	}
	c.flag(flagLogFormat, format)
	c.flag(flagLogLevel, p.LogLevel)

	switch p.Service {
	case serviceRegistry:
		c.flag(flagRegistryURL, p.RegistryURL)
		c.flag(flagRegistryPushUsername, p.RegistryPushUsername)
		c.flag(flagRegistryPushPassword, p.RegistryPushPassword)
		c.flag(flagRegistryPullUsername, p.RegistryPullUsername)
		c.flag(flagRegistryPullPassword, p.RegistryPullPassword)
		// Only two of the three subcommands take it; see forceConflictsServices.
		c.boolFlag(flagForceConflicts, p.ForceConflicts)
	case serviceGit:
		c.flag(flagGitURL, p.GitURL)
		c.flag(flagGitPushUsername, p.GitPushUsername)
		c.flag(flagGitPushPassword, p.GitPushPassword)
		c.flag(flagGitPullUsername, p.GitPullUsername)
		c.flag(flagGitPullPassword, p.GitPullPassword)
	case serviceAgent:
		c.flag(flagAgentTLSCA, p.AgentTLSCA)
		c.flag(flagAgentTLSCert, p.AgentTLSCert)
		c.flag(flagAgentTLSKey, p.AgentTLSKey)
		c.boolFlag(flagForceConflicts, p.ForceConflicts)
	}

	return c.args()
}

// env is what the module adds to the environment zarf runs in.
func (p *updateCredsParams) env() []string {
	var env []string
	if p.Kubeconfig != "" {
		env = append(env, "KUBECONFIG="+p.Kubeconfig)
	}
	if p.ZarfConfig != "" {
		env = append(env, "ZARF_CONFIG="+p.ZarfConfig)
	}
	env = append(env, "NO_COLOR=1")
	return env
}
