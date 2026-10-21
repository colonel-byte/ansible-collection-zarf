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

// The flags zarf package mirror-resources renders beyond the ones it shares with the other
// modules. TestMirrorArgsAreKnownFlags parses them against zarf's own command.
const (
	flagImages                  = "images"
	flagNoImgChecksum           = "no-img-checksum"
	flagRepos                   = "repos"
	flagSkipSignatureValidation = "skip-signature-validation"
)

// mirrorParams is the zarf_package_mirror_resources module's parameter surface.
//
// `zarf package mirror-resources` pushes a package's images and git repositories into a registry
// and git server without deploying anything, which is the step a cluster's own registry has to
// have had before a connected deploy can work. Its flags are the push half of a deploy's -- there
// is nothing to template and no component to wait on -- plus the registry and git destinations,
// which it takes outright rather than reading from a cluster's zarf state.
type mirrorParams struct {
	// Package is what to mirror: a path to a tarball, an oci:// reference, or an https:// URL. It
	// is the positional argument of the command.
	Package string `json:"package"`

	// Kubeconfig is the cluster whose registry and git server are the destination, for a mirror
	// that reads them out of the cluster's zarf state rather than being told. Zarf has no
	// --kubeconfig flag, so it is passed as KUBECONFIG in the environment.
	Kubeconfig string `json:"kubeconfig"`
	// ZarfConfig is an operator-supplied zarf config file, passed as ZARF_CONFIG. It is how a
	// playbook keeps the push credentials out of the argument vector.
	ZarfConfig string `json:"zarf_config"`
	// StatusFile overrides the heartbeat file, as it does on the converging modules: the action
	// plugin creates a private temporary file and names it, and an operator reproducing a run by
	// hand can name one.
	StatusFile string `json:"status_file"`

	// Images and Repos narrow the mirror to one kind of resource. Zarf marks them mutually
	// exclusive, so the module refuses both rather than letting zarf reject the command after the
	// result has reported it.
	Images *bool `json:"images"`
	Repos  *bool `json:"repos"`

	Components string `json:"components"`

	RegistryURL          string `json:"registry_url"`
	RegistryPushUsername string `json:"registry_push_username"`
	RegistryPushPassword string `json:"registry_push_password"`

	GitURL          string `json:"git_url"`
	GitPushUsername string `json:"git_push_username"`
	GitPushPassword string `json:"git_push_password"`

	Shasum        string `json:"shasum"`
	NoImgChecksum *bool  `json:"no_img_checksum"`

	PublicKey               string `json:"public_key"`
	Verify                  string `json:"verify"`
	SkipSignatureValidation *bool  `json:"skip_signature_validation"`

	Retries        *int `json:"retries"`
	OCIConcurrency *int `json:"oci_concurrency"`

	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

// runMirrorResources pushes one package's images and repositories into a registry and git server.
//
// It is the air-gap seeding step: the registry a cluster deploys from has to hold the images
// before a connected deploy can find them, and before this module a playbook had no way to say so.
func runMirrorResources(ctx context.Context, run Runner, args *Args, resp *Response) error {
	var p mirrorParams
	if err := args.Params(&p); err != nil {
		return err
	}
	if p.Package == "" {
		return errors.New(`the "package" parameter is required: it is the package whose ` +
			`resources are mirrored, as a path, an oci:// reference, or an https:// URL`)
	}
	if boolValue(p.Images) && boolValue(p.Repos) {
		return errors.New(`"images" and "repos" are both true: zarf marks them mutually ` +
			`exclusive, because each one narrows the mirror to that kind of resource. Leave both ` +
			`unset to mirror everything`)
	}

	if args.Control.CheckMode {
		resp.Skipped = true
		resp.Msg = "check mode: zarf package mirror-resources has no dry run, so nothing was pushed"
		resp.Zarf.ChangedSignal = SignalUnknown
		return nil
	}

	resp.Zarf.Version = config.CLIVersion

	hb := NewHeartbeat(p.StatusFile)
	defer hb.Clear()
	resp.Zarf.StatusFile = hb.Path()

	// A mirror walks the package component by component the way a deploy does, and announces each
	// one the same way, so the progress reader reads it with nothing added.
	prog := newProgress(hb, componentCount(p.Components))
	argv := buildMirrorArgs(&p)
	resp.Zarf.Command = append([]string{commandName}, redact(argv)...)

	res, runErr := run.Run(ctx, Invocation{
		Args:   argv,
		Env:    p.env(hb),
		OnLine: prog.line,
	})
	resp.Zarf.ExitCode = res.ExitCode
	resp.Zarf.ComponentsRan = prog.components

	if runErr != nil {
		prog.finish("failed", runErr)
		reportChanged(resp, prog)
		return fmt.Errorf("zarf package mirror-resources failed: %w", runErr)
	}
	prog.finish("completed", nil)

	reportChanged(resp, prog)
	resp.Msg = "mirrored " + p.Package
	return nil
}

// buildMirrorArgs renders the parameters as the zarf command line an operator would have typed.
func buildMirrorArgs(p *mirrorParams) []string {
	c := newCommand("package", "mirror-resources", p.Package)

	c.bare(flagConfirm)
	c.bare(flagNoColor)

	format := p.LogFormat
	if format == "" {
		format = "json"
	}
	c.flag(flagLogFormat, format)
	c.flag(flagLogLevel, p.LogLevel)

	c.flag(flagComponents, p.Components)

	c.flag(flagRegistryURL, p.RegistryURL)
	c.flag(flagRegistryPushUsername, p.RegistryPushUsername)
	c.flag(flagRegistryPushPassword, p.RegistryPushPassword)

	c.flag(flagGitURL, p.GitURL)
	c.flag(flagGitPushUsername, p.GitPushUsername)
	c.flag(flagGitPushPassword, p.GitPushPassword)

	c.boolFlag(flagImages, p.Images)
	c.boolFlag(flagRepos, p.Repos)

	c.flag(flagShasum, p.Shasum)
	c.boolFlag(flagNoImgChecksum, p.NoImgChecksum)

	c.flag(flagKey, p.PublicKey)
	c.valueFlag(flagVerify, p.Verify)
	c.boolFlag(flagSkipSignatureValidation, p.SkipSignatureValidation)

	c.intFlag(flagRetries, p.Retries)
	c.intFlag(flagOCIConcurrency, p.OCIConcurrency)

	return c.args()
}

// env is what the module adds to the environment zarf runs in.
func (p *mirrorParams) env(hb *Heartbeat) []string {
	var env []string
	if p.Kubeconfig != "" {
		env = append(env, "KUBECONFIG="+p.Kubeconfig)
	}
	if p.ZarfConfig != "" {
		env = append(env, "ZARF_CONFIG="+p.ZarfConfig)
	}
	if hb.Path() != "" {
		// The wrapper writes the heartbeats, not zarf. See childEnv.
		env = append(env, EnvStatusFile+"=")
	}
	env = append(env, "NO_COLOR=1")
	return env
}

// boolValue reads an optional boolean, where unset is false.
func boolValue(value *bool) bool {
	return value != nil && *value
}
