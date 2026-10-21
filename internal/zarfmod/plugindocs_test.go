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
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

// collectionRoot is the colonel_byte.zarf collection, from this package's directory.
const collectionRoot = "../../ansible/colonel_byte/zarf"

// documentationBlock pulls the DOCUMENTATION literal out of an action plugin, the same way
// magefiles/pkg/gen/docs/ansible.go reads it to generate the reference pages.
var documentationBlock = regexp.MustCompile(`(?sm)^DOCUMENTATION = r"""\n(.*?)\n"""$`)

// pluginParameters is the part of a DOCUMENTATION block this test reads. Order is not checked
// here: it is the row order of the generated page, which is the generator's business, not the
// contract's.
type pluginParameters struct {
	Options map[string]struct {
		// CLIFlag is the flag the parameter renders, or "None" when it renders none.
		CLIFlag string `yaml:"cli_flag"`
	} `yaml:"options"`
}

// pluginOnlyParameters are documented but have no Go field, because the action plugin consumes
// them and the wrapper never sees them. See plugins/plugin_utils/projection.py.
var pluginOnlyParameters = map[string]bool{
	"parameters":     true,
	"wrapper_binary": true,
}

// moduleOwnFlags are the flags a module adds for itself rather than for any parameter: the
// confirmation there is no terminal to give, and the colour Ansible would capture as noise. No
// documented parameter controls them, so the every-flag-is-documented direction below has to let
// them through.
//
// --log-format is not among them. It is rendered on every run, but it is rendered from the
// log_format parameter, whose default the documentation states.
var moduleOwnFlags = map[string]bool{
	flagConfirm: true,
	flagNoColor: true,
}

// TestActionPluginDocsMatchModuleParams is what stops a module's documented interface drifting
// from the one it has, and so what keeps the generated pages under docs/ansible/zarf honest.
//
// The documentation is YAML inside a Python file and the interface is a Go struct, so nothing but
// a test relates them. Two ways they drift, both of which reach an operator as a parameter that is
// documented and rejected, or accepted and undocumented:
//
//   - a parameter added to, removed from, or renamed in the Go struct without the DOCUMENTATION
//     block following it. Checked against jsonFieldNames, which is what Args.Params itself decodes
//     through, so the test reads the same list the module accepts.
//   - a parameter whose documented cli_flag is not the flag the module renders. A flag cannot be
//     read off a struct tag -- it exists only as a statement inside buildInitArgs or
//     buildDeployArgs -- so each case populates every parameter, renders the command line, and
//     checks the documented flags against the ones that came out.
//
// wrapperModuleCase is one module whose parameters are a Go struct and whose command line a Go
// function renders, which is what lets the two directions below be checked against the real thing.
type wrapperModuleCase struct {
	module string
	params any
	argv   []string
}

// wrapperModuleCases are the modules built that way. The read-only modules are not among them and
// deliberately so; TestEveryActionPluginIsCovered in infoplugins_test.go is what holds this list
// and that one against the plugins on disk, so that a module in neither cannot go unnoticed.
func wrapperModuleCases() []wrapperModuleCase {
	return []wrapperModuleCase{
		{
			module: "zarf_init",
			params: &initParams{},
			// The package source is the command's positional argument rather than a flag, so it
			// is left out here: this test reads the flags the module renders against the flags
			// the plugin documents, and a positional argument is neither.
			argv: buildInitArgs(fullInitParams(), ""),
		},
		{
			module: "zarf_package_deploy",
			params: &deployParams{},
			argv:   buildDeployArgs(fullDeployParams()),
		},
		{
			module: "zarf_package_mirror_resources",
			params: &mirrorParams{},
			// Two renders joined, because images and repos are mutually exclusive and no single
			// command line can carry both. What this case checks is the flags the module can
			// render, which is the union of the two.
			argv: append(buildMirrorArgs(fullMirrorParams()),
				buildMirrorArgs(fullMirrorParamsMirroringRepos())...),
		},
		{
			module: "zarf_update_creds",
			params: &updateCredsParams{},
			// Three renders joined. Zarf has one subcommand per service and each takes only that
			// service's credentials, so no single command line carries them all; what this case
			// checks is the flags the module can render.
			argv: slices.Concat(
				buildUpdateCredsArgs(fullUpdateCredsParams()),
				buildUpdateCredsArgs(fullUpdateCredsParamsFor("git")),
				buildUpdateCredsArgs(fullUpdateCredsParamsFor("agent")),
			),
		},
		{
			module: "zarf_package_pull",
			params: &pullParams{},
			argv:   buildPullArgs(fullPullParams()),
		},
		{
			module: "zarf_package_remove",
			params: &removeParams{},
			argv:   buildRemoveArgs(fullRemoveParams()),
		},
		{
			module: "zarf_kubectl",
			params: &kubectlParams{},
			// A passthrough renders no flags at all: args is kubectl's own argument vector and
			// kubeconfig is an environment variable. The case is here for the other direction --
			// a parameter added to the Go struct and not to the documentation.
			argv: nil,
		},
	}
}

func TestActionPluginDocsMatchModuleParams(t *testing.T) {
	for _, tt := range wrapperModuleCases() {
		t.Run(tt.module, func(t *testing.T) {
			documented := documentedOptions(t, tt.module)

			t.Run("every parameter is documented", func(t *testing.T) {
				names := make([]string, 0, len(documented))
				for name := range documented {
					if pluginOnlyParameters[name] {
						continue
					}
					names = append(names, name)
				}
				sort.Strings(names)

				want := sortedKeys(jsonFieldNames(reflect.TypeOf(tt.params)))
				if strings.Join(names, ",") != strings.Join(want, ",") {
					t.Errorf("the documented parameters are\n  %v\nand the module accepts\n  %v",
						names, want)
				}
			})

			t.Run("every documented flag is rendered", func(t *testing.T) {
				rendered := renderedFlags(tt.argv)
				for name, option := range documented {
					flag, renders := strings.CutPrefix(option.CLIFlag, "--")
					if !renders {
						// "None", the documented spelling for a parameter that renders no flag.
						if option.CLIFlag != "None" {
							t.Errorf("parameter %s documents cli_flag %q, which is neither a flag nor None",
								name, option.CLIFlag)
						}
						continue
					}
					if !contains(rendered, flag) {
						t.Errorf("parameter %s documents cli_flag %q, which the module does not render",
							name, option.CLIFlag)
					}
				}
			})

			t.Run("every rendered flag is documented", func(t *testing.T) {
				flags := map[string]bool{}
				for _, option := range documented {
					if flag, renders := strings.CutPrefix(option.CLIFlag, "--"); renders {
						flags[flag] = true
					}
				}
				for _, flag := range renderedFlags(tt.argv) {
					if moduleOwnFlags[flag] {
						continue
					}
					if !flags[flag] {
						t.Errorf("the module renders --%s, which no documented parameter names", flag)
					}
				}
			})
		})
	}
}

// documentedOptions reads one action plugin's documented parameters.
func documentedOptions(t *testing.T, module string) map[string]struct {
	CLIFlag string `yaml:"cli_flag"`
} {
	t.Helper()

	path := filepath.Join(collectionRoot, "plugins", "action", module+".py")
	source, err := os.ReadFile(path) //nolint:gosec // a path assembled from this test's own constants
	if err != nil {
		t.Fatalf("unable to read %s: %v", path, err)
	}

	block := documentationBlock.FindSubmatch(source)
	if block == nil {
		t.Fatalf(`%s has no DOCUMENTATION = r""" block`, path)
	}

	var parsed pluginParameters
	if err := goyaml.Unmarshal(block[1], &parsed); err != nil {
		t.Fatalf("the DOCUMENTATION block in %s: %v", path, err)
	}
	if len(parsed.Options) == 0 {
		t.Fatalf("%s documents no options", path)
	}
	return parsed.Options
}

// renderedFlags returns the flag names in a command line, sorted and without duplicates. A boolean
// flag is written --name=value and a repeated one appears once per value, so neither the value nor
// the count is what this compares.
func renderedFlags(argv []string) []string {
	seen := map[string]bool{}
	for _, arg := range argv {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		seen[name] = true
	}
	return sortedKeys(seen)
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// The parameter sets below are populated in full, because a flag only appears in the rendered
// command line when its parameter is set. A field left at its zero value would be read as a flag
// the module does not render, and the documentation for it as drift.
//
// Every value is arbitrary except in one respect: none of them starts with a dash, or renderedFlags
// would count it as a flag of its own.

func fullInitParams() *initParams {
	yes := true
	number := 1
	return &initParams{
		InitPackage:          "/srv/staging/zarf-init-amd64-v0.85.0.tar.zst",
		StorageClass:         "local-path",
		RegistryURL:          "registry.bubbles.test",
		RegistryMode:         "proxy",
		RegistryPort:         &number,
		RegistrySecret:       "secret",
		RegistryPushUsername: "zarf-push",
		RegistryPushPassword: "push",
		RegistryPullUsername: "zarf-pull",
		RegistryPullPassword: "pull",
		GitURL:               "https://git.bubbles.test",
		GitPushUsername:      "zarf-push",
		GitPushPassword:      "push",
		GitPullUsername:      "zarf-pull",
		GitPullPassword:      "pull",
		InjectorImage:        "registry.bubbles.test/injector:1",
		InjectorPort:         &number,
		AgentMutationPolicy:  "all",
		AgentTLSCA:           "/etc/zarf/ca.pem",
		AgentTLSCert:         "/etc/zarf/tls.pem",
		AgentTLSKey:          "/etc/zarf/tls.key",
		commonParams: commonParams{
			Directory:             "/srv/staging",
			Kubeconfig:            "/etc/rancher/rke2/rke2.yaml",
			ZarfConfig:            "/etc/zarf/zarf-config.yaml",
			StatusFile:            "/run/zarf/status.json",
			Components:            "zarf-registry",
			TakeOwnership:         &yes,
			ForceConflicts:        &yes,
			SkipValuesSchemaValid: &yes,
			InsecureSkipTLSVerify: &yes,
			PlainHTTP:             &yes,
			Retries:               &number,
			OCIConcurrency:        &number,
			Architecture:          "amd64",
			Cache:                 "/var/cache/zarf",
			Tmpdir:                "/var/tmp/zarf",
			Timeout:               "15m",
			PublicKey:             "/etc/zarf/cosign.pub",
			Verify:                "always",
			Values:                []string{"/etc/zarf/values.yaml"},
			SetValues:             map[string]string{"key": "value"},
			SetVariables:          map[string]string{"KEY": "value"},
			LogLevel:              "debug",
			LogFormat:             "json",
		},
	}
}

func fullDeployParams() *deployParams {
	yes := true
	number := 1
	return &deployParams{
		Package:   "/srv/staging/zarf-package-monitoring-amd64.tar.zst",
		Namespace: "storage",
		Shasum:    "0000000000000000000000000000000000000000000000000000000000000000",
		Connected: &yes,
		commonParams: commonParams{
			Directory:             "/srv/staging",
			Kubeconfig:            "/etc/rancher/rke2/rke2.yaml",
			ZarfConfig:            "/etc/zarf/zarf-config.yaml",
			StatusFile:            "/run/zarf/status.json",
			Components:            "local-path-chart",
			ForceConflicts:        &yes,
			SkipValuesSchemaValid: &yes,
			InsecureSkipTLSVerify: &yes,
			PlainHTTP:             &yes,
			TakeOwnership:         &yes,
			Retries:               &number,
			OCIConcurrency:        &number,
			Architecture:          "amd64",
			Cache:                 "/var/cache/zarf",
			Tmpdir:                "/var/tmp/zarf",
			Timeout:               "15m",
			PublicKey:             "/etc/zarf/cosign.pub",
			Verify:                "always",
			Values:                []string{"/etc/zarf/values.yaml"},
			SetValues:             map[string]string{"key": "value"},
			SetVariables:          map[string]string{"KEY": "value"},
			LogLevel:              "debug",
			LogFormat:             "json",
		},
	}
}

// fullPullParams populates every zarf_package_pull parameter, so that every flag the module can
// render is in the vector the test reads.
func fullPullParams() *pullParams {
	yes := true
	number := 1
	return &pullParams{
		Package:               "oci://ghcr.io/colonel-byte/zarf/csi-local-path-provider:0.0.37-upstream",
		OutputDirectory:       "/srv/staging",
		Shasum:                "0000000000000000000000000000000000000000000000000000000000000000",
		OCIConcurrency:        &number,
		Architecture:          "amd64",
		Cache:                 "/var/cache/zarf",
		Tmpdir:                "/var/tmp/zarf",
		PlainHTTP:             &yes,
		InsecureSkipTLSVerify: &yes,
		ZarfConfig:            "/etc/zarf/zarf-config.yaml",
		LogLevel:              "debug",
		LogFormat:             "json",
	}
}

// fullRemoveParams does the same for zarf_package_remove.
func fullRemoveParams() *removeParams {
	yes := true
	number := 1
	return &removeParams{
		Package:               "csi-local-path-provider",
		Kubeconfig:            "/etc/rancher/rke2/rke2.yaml",
		ZarfConfig:            "/etc/zarf/zarf-config.yaml",
		Components:            "local-path-chart",
		Namespace:             "storage",
		Values:                []string{"/etc/zarf/values.yaml"},
		SetValues:             map[string]string{"key": "value"},
		OCIConcurrency:        &number,
		Architecture:          "amd64",
		Cache:                 "/var/cache/zarf",
		Tmpdir:                "/var/tmp/zarf",
		PlainHTTP:             &yes,
		InsecureSkipTLSVerify: &yes,
		LogLevel:              "debug",
		LogFormat:             "json",
	}
}

// fullMirrorParams populates every zarf_package_mirror_resources parameter but one: images and
// repos are mutually exclusive, so the widest renderable vector carries only one of them.
func fullMirrorParams() *mirrorParams {
	yes := true
	number := 1
	return &mirrorParams{
		Package:                 "/srv/staging/zarf-package-monitoring-amd64.tar.zst",
		Kubeconfig:              "/etc/rancher/rke2/rke2.yaml",
		ZarfConfig:              "/etc/zarf/zarf-config.yaml",
		StatusFile:              "/run/zarf/status.json",
		Images:                  &yes,
		Components:              "monitoring-images",
		RegistryURL:             "registry.bubbles.test",
		RegistryPushUsername:    "zarf-push",
		RegistryPushPassword:    "push",
		GitURL:                  "https://git.bubbles.test",
		GitPushUsername:         "zarf-push",
		GitPushPassword:         "push",
		Shasum:                  "0000000000000000000000000000000000000000000000000000000000000000",
		NoImgChecksum:           &yes,
		PublicKey:               "/etc/zarf/cosign.pub",
		Verify:                  "always",
		SkipSignatureValidation: &yes,
		Retries:                 &number,
		OCIConcurrency:          &number,
		LogLevel:                "debug",
		LogFormat:               "json",
	}
}

// fullMirrorParamsMirroringRepos is the same set with the other half of that exclusive pair.
func fullMirrorParamsMirroringRepos() *mirrorParams {
	yes := true
	p := fullMirrorParams()
	p.Images = nil
	p.Repos = &yes
	return p
}

// fullUpdateCredsParamsFor is the same set rendered for another service. The parameters are the
// whole surface either way; which of them render is the subcommand's business.
func fullUpdateCredsParamsFor(service string) *updateCredsParams {
	p := fullUpdateCredsParams()
	p.Service = service
	return p
}

// fullUpdateCredsParams does the same for zarf_update_creds, as the registry subcommand.
func fullUpdateCredsParams() *updateCredsParams {
	yes := true
	return &updateCredsParams{
		Service:              "registry",
		Kubeconfig:           "/etc/rancher/rke2/rke2.yaml",
		ZarfConfig:           "/etc/zarf/zarf-config.yaml",
		RegistryURL:          "registry.bubbles.test",
		RegistryPushUsername: "zarf-push",
		RegistryPushPassword: "push",
		RegistryPullUsername: "zarf-pull",
		RegistryPullPassword: "pull",
		GitURL:               "https://git.bubbles.test",
		GitPushUsername:      "zarf-push",
		GitPushPassword:      "push",
		GitPullUsername:      "zarf-pull",
		GitPullPassword:      "pull",
		AgentTLSCA:           "/etc/zarf/ca.pem",
		AgentTLSCert:         "/etc/zarf/tls.pem",
		AgentTLSKey:          "/etc/zarf/tls.key",
		ForceConflicts:       &yes,
		LogLevel:             "debug",
		LogFormat:            "json",
	}
}
