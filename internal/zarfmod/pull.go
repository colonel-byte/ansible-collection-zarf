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
	"sort"

	"github.com/zarf-dev/zarf/src/config"
)

// The flag zarf package pull renders beyond the ones it shares with the other modules.
// TestPullArgsAreKnownFlags parses them against zarf's own package pull command.
const flagOutputDirectory = "output-directory"

// pullParams is the zarf_package_pull module's parameter surface.
//
// It takes neither commonParams nor most of what the converging modules take, and the reason is
// the command: `zarf package pull` fetches a package to disk and touches no cluster, so there is
// no kubeconfig, no components, no timeout and -- the one that would fail outright -- no
// --confirm, which the command does not have. What is left is zarf's root flags, the two the
// command adds, and where to put the result.
type pullParams struct {
	// Package is what to fetch: an oci:// reference or an https:// URL. It is the positional
	// argument of the command, which takes exactly one.
	Package string `json:"package"`
	// OutputDirectory is where the tarball is written. Zarf defaults it to the working directory,
	// which under Ansible is whatever directory the module was invoked from, so a playbook that
	// cares where the package lands says so.
	OutputDirectory string `json:"output_directory"`
	// Shasum is the checksum of a package fetched over https://, which zarf requires for that
	// scheme rather than merely accepting.
	Shasum string `json:"shasum"`

	OCIConcurrency *int `json:"oci_concurrency"`

	Architecture          string `json:"architecture"`
	Cache                 string `json:"cache"`
	Tmpdir                string `json:"tmpdir"`
	PlainHTTP             *bool  `json:"plain_http"`
	InsecureSkipTLSVerify *bool  `json:"insecure_skip_tls_verify"`

	ZarfConfig string `json:"zarf_config"`

	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

// runPackagePull fetches one package to disk.
//
// It is the staging half of an air-gapped workflow: a connected node pulls the package, something
// carries the file through the airlock, and zarf_package_deploy reads it from the path it landed
// at. Nothing about it reaches a cluster, which is why it is the one module here that reports a
// `changed` worth believing -- see below.
func runPackagePull(ctx context.Context, run Runner, args *Args, resp *Response) error {
	var p pullParams
	if err := args.Params(&p); err != nil {
		return err
	}
	if p.Package == "" {
		return errors.New(`the "package" parameter is required: it is the package to fetch, ` +
			`as an oci:// reference or an https:// URL`)
	}
	if !isRemoteSource(p.Package) {
		// A local path is not a thing to pull, and zarf's own error for one names a package
		// loader rather than the parameter.
		return fmt.Errorf("the \"package\" parameter is %q, which is a local path: "+
			"zarf package pull fetches a remote package, so it takes an oci:// reference or an "+
			"https:// URL", p.Package)
	}

	if args.Control.CheckMode {
		resp.Skipped = true
		resp.Msg = "check mode: zarf package pull has no dry run, so nothing was fetched"
		resp.Zarf.ChangedSignal = SignalUnknown
		return nil
	}

	resp.Zarf.Version = config.CLIVersion

	argv := buildPullArgs(&p)
	resp.Zarf.Command = append([]string{commandName}, redact(argv)...)
	resp.Zarf.Directory = p.OutputDirectory

	// What the directory held before, so that the result can name the file that arrived rather
	// than leaving a playbook to guess it. The name is zarf's to choose -- it comes from the
	// package's own metadata -- so there is nothing to predict and nothing to stat for.
	before := packagesInDirectory(p.OutputDirectory)

	res, runErr := run.Run(ctx, Invocation{
		Args: argv,
		Dir:  "",
		Env:  p.env(),
	})
	resp.Zarf.ExitCode = res.ExitCode

	if runErr != nil {
		return fmt.Errorf("zarf package pull failed: %w", runErr)
	}

	// Changed is observable here, unlike everywhere else in this collection: a pull either left a
	// file that was not there or it did not. Zarf overwrites a package it fetches again, so a
	// second run of the same pull reports changed: false only because the name was already
	// present -- which is the honest answer to "did this task add a package".
	arrived := newPackages(before, packagesInDirectory(p.OutputDirectory))
	resp.Zarf.PackagesPulled = arrived
	resp.Changed = len(arrived) > 0
	resp.Zarf.ChangedSignal = SignalComplete
	resp.Msg = "pulled " + p.Package
	return nil
}

// buildPullArgs renders the parameters as the zarf command line an operator would have typed.
func buildPullArgs(p *pullParams) []string {
	c := newCommand("package", "pull", p.Package)

	// No --confirm: the command has none. Everything else is as the other modules render it.
	c.bare(flagNoColor)

	format := p.LogFormat
	if format == "" {
		format = "json"
	}
	c.flag(flagLogFormat, format)
	c.flag(flagLogLevel, p.LogLevel)

	c.flag(flagOutputDirectory, p.OutputDirectory)
	c.flag(flagShasum, p.Shasum)
	c.intFlag(flagOCIConcurrency, p.OCIConcurrency)

	c.flag(flagArchitecture, p.Architecture)
	c.flag(flagCache, p.Cache)
	c.flag(flagTmpdir, p.Tmpdir)
	c.boolFlag(flagPlainHTTP, p.PlainHTTP)
	c.boolFlag(flagInsecureSkipTLSVerify, p.InsecureSkipTLSVerify)

	return c.args()
}

// env is what the module adds to the environment zarf runs in.
func (p *pullParams) env() []string {
	var env []string
	if p.ZarfConfig != "" {
		env = append(env, "ZARF_CONFIG="+p.ZarfConfig)
	}
	env = append(env, "NO_COLOR=1")
	return env
}

// packagesInDirectory names the package tarballs in a directory. An unreadable directory is an
// empty set rather than an error: it is read to compare two moments, and a pull that cannot write
// where it was told fails on its own.
func packagesInDirectory(dir string) map[string]bool {
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	found := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if name := entry.Name(); isPackageFile(name) {
			found[name] = true
		}
	}
	return found
}

// isPackageFile reports whether a filename is a zarf package tarball, which is the only thing a
// pull writes.
func isPackageFile(name string) bool {
	return len(name) > len(".tar.zst") &&
		(name[len(name)-len(".tar.zst"):] == ".tar.zst" || name[len(name)-len(".tar"):] == ".tar")
}

// newPackages names what the second listing holds and the first did not, sorted.
func newPackages(before, after map[string]bool) []string {
	var arrived []string
	for name := range after {
		if !before[name] {
			arrived = append(arrived, name)
		}
	}
	sort.Strings(arrived)
	return arrived
}
