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

	"github.com/zarf-dev/zarf/src/config"
)

// removeParams is the zarf_package_remove module's parameter surface.
//
// It is not deployParams with the package swapped: `zarf package remove` takes seven flags, and
// five of the twenty-eight a deploy takes have no meaning once the thing is being taken away.
// There is no signature to verify, no registry to push to, no values schema to validate, and no
// --timeout -- a removal waits on zarf's own defaults. Nor is --skip-version-check projected:
// zarf hides it, and a parameter that overrides a guard nobody asked about is one to add when
// somebody does.
type removeParams struct {
	// Package is what to remove: a path to the tarball it was deployed from, an oci:// reference,
	// or -- the form a playbook usually wants -- the deployed package's name, which is what
	// `zarf package list` reports. It is the positional argument of the command.
	Package string `json:"package"`

	// Kubeconfig is the cluster to remove from. Zarf has no --kubeconfig flag, so it is passed as
	// KUBECONFIG in the environment. It is declared here rather than shared with the converging
	// modules because the three commands that need one are the only ones that have it.
	Kubeconfig string `json:"kubeconfig"`
	// ZarfConfig is an operator-supplied zarf config file, passed as ZARF_CONFIG.
	ZarfConfig string `json:"zarf_config"`

	// Components narrows the removal to some of what was deployed. Removing a component of a
	// package and leaving the rest is the one thing this module does that a deploy cannot undo.
	Components string `json:"components"`
	// Namespace is the namespace the package was deployed into, for a package deployed with
	// zarf_package_deploy's namespace override. Alpha in zarf, as it is there.
	Namespace string `json:"namespace"`

	// Values and SetValues are for the on-remove actions a package's components can declare,
	// which are templated the same way the on-deploy ones are.
	Values    []string          `json:"values"`
	SetValues map[string]string `json:"set_values"`

	OCIConcurrency *int `json:"oci_concurrency"`

	Architecture          string `json:"architecture"`
	Cache                 string `json:"cache"`
	Tmpdir                string `json:"tmpdir"`
	PlainHTTP             *bool  `json:"plain_http"`
	InsecureSkipTLSVerify *bool  `json:"insecure_skip_tls_verify"`

	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

// runPackageRemove takes one package off a cluster.
//
// It is the module ZEP-0072 names and this repository had not needed: the two converging modules
// put things on a cluster, and a playbook that can only add is not reconciling anything. What it
// cannot do is tell an operator what it took away -- see the changed signal below.
func runPackageRemove(ctx context.Context, run Runner, args *Args, resp *Response) error {
	var p removeParams
	if err := args.Params(&p); err != nil {
		return err
	}
	if p.Package == "" {
		return errors.New(`the "package" parameter is required: it is the package to remove, ` +
			`as a deployed package name, a path to its tarball, or an oci:// reference`)
	}

	if args.Control.CheckMode {
		// Skipped, as the converging modules are, and for the stronger version of their reason:
		// `zarf package remove` has no dry run, and removing a package during a run an operator
		// asked to be told about is worse than deploying one.
		resp.Skipped = true
		resp.Msg = "check mode: zarf package remove has no dry run, so nothing was removed"
		resp.Zarf.ChangedSignal = SignalUnknown
		return nil
	}

	resp.Zarf.Version = config.CLIVersion

	argv := buildRemoveArgs(&p)
	resp.Zarf.Command = append([]string{commandName}, redact(argv)...)

	res, runErr := run.Run(ctx, Invocation{
		Args: argv,
		Env:  p.env(),
	})
	resp.Zarf.ExitCode = res.ExitCode

	if runErr != nil {
		return fmt.Errorf("zarf package remove failed: %w", runErr)
	}

	// Unknown rather than partial, which is a weaker answer than the converging modules give and
	// an honest one: zarf logs the removal of a package as one line and names no component on the
	// way, so there is no component list to report and nothing that distinguishes a package that
	// was there from one that was already gone. A removal that found nothing to remove fails in
	// zarf, which is the only signal there is.
	resp.Changed = true
	resp.Zarf.ChangedSignal = SignalUnknown
	resp.Msg = "removed " + p.Package
	return nil
}

// buildRemoveArgs renders the parameters as the zarf command line an operator would have typed.
func buildRemoveArgs(p *removeParams) []string {
	c := newCommand("package", "remove", p.Package)

	// --confirm because there is no terminal on the other end of a playbook task, and this is the
	// command where that matters most: without it zarf asks before deleting anything.
	c.bare(flagConfirm)
	c.bare(flagNoColor)

	format := p.LogFormat
	if format == "" {
		format = "json"
	}
	c.flag(flagLogFormat, format)
	c.flag(flagLogLevel, p.LogLevel)

	c.flag(flagComponents, p.Components)
	c.flag(flagNamespace, p.Namespace)
	c.intFlag(flagOCIConcurrency, p.OCIConcurrency)

	c.repeated(flagValues, p.Values)
	c.pairs(flagSetValues, p.SetValues)

	c.flag(flagArchitecture, p.Architecture)
	c.flag(flagCache, p.Cache)
	c.flag(flagTmpdir, p.Tmpdir)
	c.boolFlag(flagPlainHTTP, p.PlainHTTP)
	c.boolFlag(flagInsecureSkipTLSVerify, p.InsecureSkipTLSVerify)

	return c.args()
}

// env is what the module adds to the environment zarf runs in.
func (p *removeParams) env() []string {
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
