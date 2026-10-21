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

import "strings"

// commonParams are the parameters both wrapper modules take, and the flags both render from them.
//
// They are one embedded struct rather than two copies because every parameter in this repository is
// already declared four times -- the Go field, the statement that renders its flag, the Python
// DOCUMENTATION block, and the test fixture that populates it -- and four tests exist to hold those
// four in step. Twenty-four of the twenty-eight parameters zarf_package_deploy takes are also
// zarf_init's, so a second copy on the Go side makes it six declarations for those, with the two
// new ones held in step by nothing at all: no test compares initParams to deployParams, and nothing
// would have noticed `--timeout` being added to one and not the other.
//
// It is embedded, so encoding/json promotes these fields and the modules decode exactly the
// parameters they did before. jsonFieldNames walks into an embedded struct for the same reason,
// which is what keeps TestActionPluginDocsMatchModuleParams reading the whole surface.
//
// What is deliberately not here is anything the two modules treat differently. Directory is shared
// because both modules take it; workingDir and deployWorkingDir are not, because init infers a
// directory from the init package and a deploy has nothing to infer. A field belongs here when
// both modules mean the same thing by it, not when both happen to have one.
type commonParams struct {
	// Directory is the working directory to run zarf in. What each module does with it differs;
	// see workingDir and deployWorkingDir.
	Directory string `json:"directory"`
	// Kubeconfig is the cluster to act on. Zarf has no --kubeconfig flag, so it is passed as
	// KUBECONFIG in the child's environment.
	Kubeconfig string `json:"kubeconfig"`
	// ZarfConfig is an operator-supplied zarf config file, passed as ZARF_CONFIG. It is how a
	// playbook keeps credentials out of the argument vector entirely: zarf reads them from the
	// file, so they never reach the process table.
	ZarfConfig string `json:"zarf_config"`
	// StatusFile overrides the heartbeat file. The action plugin sets ZARF_STATUS_FILE instead,
	// and this is for a run reproduced by hand.
	StatusFile string `json:"status_file"`

	// Components is also what lets the progress display report a total; see componentCount.
	Components string `json:"components"`

	TakeOwnership         *bool `json:"take_ownership"`
	ForceConflicts        *bool `json:"force_conflicts"`
	SkipValuesSchemaValid *bool `json:"skip_values_schema_validation"`
	InsecureSkipTLSVerify *bool `json:"insecure_skip_tls_verify"`
	PlainHTTP             *bool `json:"plain_http"`

	Retries        *int   `json:"retries"`
	OCIConcurrency *int   `json:"oci_concurrency"`
	Architecture   string `json:"architecture"`
	Cache          string `json:"cache"`
	Tmpdir         string `json:"tmpdir"`
	Timeout        string `json:"timeout"`
	PublicKey      string `json:"public_key"`
	Verify         string `json:"verify"`

	Values       []string          `json:"values"`
	SetValues    map[string]string `json:"set_values"`
	SetVariables map[string]string `json:"set_variables"`

	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

// childEnv is what the wrapper adds to zarf's environment.
func (p *commonParams) childEnv(hb *Heartbeat) []string {
	var env []string
	if p.Kubeconfig != "" {
		env = append(env, "KUBECONFIG="+p.Kubeconfig)
	}
	if p.ZarfConfig != "" {
		env = append(env, "ZARF_CONFIG="+p.ZarfConfig)
	}
	// The wrapper is the thing writing heartbeats, not zarf, so the child is told nothing about
	// the status file. It is unset in the child precisely so a future zarf that writes heartbeats
	// of its own does not race the wrapper for the same file.
	if hb.Path() != "" {
		env = append(env, EnvStatusFile+"=")
	}
	// A module is not a terminal. Zarf checks for one, and a child whose stdout is a pipe already
	// renders plainly, but saying so costs nothing and makes the rendering the same under a
	// playbook as it is under a shell redirect.
	env = append(env, "NO_COLOR=1")
	return env
}

// applyCommonFlags renders every flag both modules share, and is called before either adds its own.
//
// Before anything else, because the ordering has to be one thing or the other and this is the one
// that reads as a command line an operator would have typed: how the run is logged and which
// components it covers, then how zarf behaves, then what this particular command is about. Flag
// order is nothing to pflag, so the only thing it costs is the argument vector the response reports
// and the want lists in the two rendering tests.
func (p *commonParams) applyCommonFlags(c *command) {
	// A module that asked for confirmation would never get it: there is no terminal on the other
	// end. The playbook task is the confirmation.
	c.bare(flagConfirm)
	// Ansible captures stderr as a text blob, and escape sequences in it are noise in every report
	// that blob ends up in.
	c.bare(flagNoColor)

	// JSON by default, because the progress the action plugin displays is read back out of this
	// stream and the console rendering is not a format. An operator who overrides it gets a
	// coarser display and a run that is otherwise identical.
	format := p.LogFormat
	if format == "" {
		format = "json"
	}
	c.flag(flagLogFormat, format)
	c.flag(flagLogLevel, p.LogLevel)

	c.flag(flagComponents, p.Components)

	c.boolFlag(flagTakeOwnership, p.TakeOwnership)
	c.boolFlag(flagForceConflicts, p.ForceConflicts)
	c.boolFlag(flagSkipValuesSchemaValidation, p.SkipValuesSchemaValid)
	c.boolFlag(flagInsecureSkipTLSVerify, p.InsecureSkipTLSVerify)
	c.boolFlag(flagPlainHTTP, p.PlainHTTP)

	c.intFlag(flagRetries, p.Retries)
	c.intFlag(flagOCIConcurrency, p.OCIConcurrency)
	c.flag(flagArchitecture, p.Architecture)
	c.flag(flagCache, p.Cache)
	c.flag(flagTmpdir, p.Tmpdir)
	c.flag(flagTimeout, p.Timeout)
	c.flag(flagKey, p.PublicKey)
	c.valueFlag(flagVerify, p.Verify)

	c.repeated(flagValues, p.Values)
	c.pairs(flagSetValues, p.SetValues)
	c.pairs(flagSetVariables, p.SetVariables)
}

// isRemoteSource reports whether a package source is one zarf fetches rather than reads from disk.
// The schemes are the ones zarf's package loader accepts.
func isRemoteSource(source string) bool {
	for _, scheme := range []string{"oci://", "http://", "https://"} {
		if strings.HasPrefix(source, scheme) {
			return true
		}
	}
	return false
}
