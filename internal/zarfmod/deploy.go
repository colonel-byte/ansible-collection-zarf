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

	"github.com/zarf-dev/zarf/src/config"
)

// The flags of zarf package deploy the module renders, beyond the ones it shares with init.
// TestDeployArgsAreKnownFlags parses them against zarf's own package deploy command, for the
// reason given on the init flag block.
const (
	flagConnected = "connected"
	flagNamespace = "namespace"
	flagShasum    = "shasum"
)

// deployParams is the zarf_package_deploy module's parameter surface: everything both wrappers
// take, plus the package and the three flags only a deploy has.
//
// A deploy is what follows an init: the cluster exists, zarf's state is in it, and this puts a
// package on it. Which is why the shared set is almost the whole of it -- everything that
// configures the registry and git server belongs to the init that deployed them.
type deployParams struct {
	commonParams

	// Package is what to deploy: a path to a tarball, an oci:// reference, or an https:// URL.
	// It is the positional argument of the command. A relative path is resolved against
	// Directory, in commonParams, when one was named; see deployPackage.
	Package string `json:"package"`

	Namespace string `json:"namespace"`
	Shasum    string `json:"shasum"`

	Connected *bool `json:"connected"`
}

// runPackageDeploy deploys one package onto an initialised cluster.
func runPackageDeploy(ctx context.Context, run Runner, args *Args, resp *Response) error {
	var p deployParams
	if err := args.Params(&p); err != nil {
		return err
	}
	if p.Package == "" {
		return errors.New(`the "package" parameter is required: it is the package to deploy, ` +
			`as a path, an oci:// reference, or an https:// URL`)
	}

	if args.Control.CheckMode {
		// Same answer as init, for the same reason: zarf has no dry run to deploy with, and
		// deploying anyway during a run an operator asked to be told about is the one thing check
		// mode must not do.
		resp.Skipped = true
		resp.Msg = "check mode: zarf package deploy has no dry run, so nothing was done"
		resp.Zarf.ChangedSignal = SignalUnknown
		return nil
	}

	dir, err := deployWorkingDir(&p)
	if err != nil {
		return err
	}

	// The zarf that does the work is the one linked into this binary, so the version is a
	// constant rather than something to go and ask for. Detail.Version is still reported, because
	// a run which failed is worth having the version of the zarf that failed it attached to.
	resp.Zarf.Version = config.CLIVersion

	hb := NewHeartbeat(p.StatusFile)
	defer hb.Clear()
	resp.Zarf.StatusFile = hb.Path()

	prog := newProgress(hb, componentCount(p.Components))
	argv := buildDeployArgs(&p)

	configEnv, cleanupConfig, err := p.configEnv()
	if err != nil {
		return err
	}
	defer cleanupConfig()

	resp.Zarf.Command = append([]string{commandName}, redact(argv)...)
	resp.Zarf.Directory = dir

	res, runErr := run.Run(ctx, Invocation{
		Args:   argv,
		Dir:    dir,
		Env:    append(p.childEnv(hb), configEnv...),
		OnLine: prog.line,
	})
	resp.Zarf.ExitCode = res.ExitCode
	resp.Zarf.ComponentsRan = prog.components

	if runErr != nil {
		prog.finish("failed", runErr)
		reportChanged(resp, prog)
		return fmt.Errorf("zarf package deploy failed: %w", runErr)
	}
	prog.finish("completed", nil)

	reportChanged(resp, prog)
	resp.Msg = "deployed " + p.Package
	return nil
}

// deployWorkingDir is the directory zarf runs in.
//
// Unlike init, a deploy names its package outright, so there is nothing to infer: an operator who
// said nothing gets the directory the module was invoked from, which is the one Ansible chose.
func deployWorkingDir(p *deployParams) (string, error) {
	if p.Directory == "" {
		return "", nil
	}
	info, err := os.Stat(p.Directory)
	if err != nil {
		return "", fmt.Errorf("unable to read the working directory %s: %w", p.Directory, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("the %q parameter is %s, which is not a directory", "directory", p.Directory)
	}
	return p.Directory, nil
}

// buildDeployArgs renders the parameters as the zarf command line an operator would have typed.
func buildDeployArgs(p *deployParams) []string {
	c := newCommand("package", "deploy", deployPackage(p))
	p.applyCommonFlags(c)

	c.flag(flagNamespace, p.Namespace)
	c.flag(flagShasum, p.Shasum)
	c.boolFlag(flagConnected, p.Connected)

	return c.args()
}

// deployPackage is the package as the command line should carry it.
//
// A local path is made absolute, because the working directory a deploy runs in is whatever
// Ansible chose unless the operator named one, and a relative path that resolved against the
// wrong directory is a failure that reads as a missing package. A remote reference is left alone:
// oci:// and https:// are not paths.
func deployPackage(p *deployParams) string {
	if isRemoteSource(p.Package) {
		return p.Package
	}
	if filepath.IsAbs(p.Package) {
		return p.Package
	}
	if p.Directory != "" {
		// The command runs in Directory, so a path relative to it is already right, and making it
		// absolute here would be the same string with more ways to be wrong.
		return p.Package
	}
	abs, err := filepath.Abs(p.Package)
	if err != nil {
		return p.Package
	}
	return abs
}
