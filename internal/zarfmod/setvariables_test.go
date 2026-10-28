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
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/spf13/pflag"
)

// A PEM private key is the value that found this: the base64 padding supplies the second `=` that
// sends pflag down its csv branch, and the line breaks are what that branch then truncates at.
const pemKey = "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA==\n-----END RSA PRIVATE KEY-----"

// TestStringToStringTruncatesAtALineBreak is the evidence for finding 7, and it is here rather
// than in the finding so that a pflag release which fixes this says so by failing. Nothing in this
// repository is on trial: the flag is parsed exactly as zarf declares it.
func TestStringToStringTruncatesAtALineBreak(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "a second equals sign sends the value through csv, which stops at the line break",
			value: pemKey,
			want:  "-----BEGIN RSA PRIVATE KEY-----",
		},
		{
			name:  "the same value without the padding survives, which is why this looks intermittent",
			value: strings.ReplaceAll(pemKey, "==", "Ab"),
			want:  strings.ReplaceAll(pemKey, "==", "Ab"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got map[string]string
			fs := pflag.NewFlagSet("deploy", pflag.ContinueOnError)
			fs.StringToStringVar(&got, flagSetVariables, nil, "")
			if err := fs.Parse([]string{"--" + flagSetVariables, "TLS_KEY=" + tt.value}); err != nil {
				t.Fatalf("parsing the flag: %v", err)
			}
			if got["TLS_KEY"] != tt.want {
				t.Errorf("TLS_KEY = %q, want %q", got["TLS_KEY"], tt.want)
			}
		})
	}
}

// TestMultilineValuesLeaveTheCommandLine holds the half of the fix that prevents the loss: a map
// that cannot survive the command line is not rendered onto it at all. Rendering one pair would be
// worse than rendering none, because the flag's first appearance replaces the map zarf read from
// the config file rather than merging into it.
func TestMultilineValuesLeaveTheCommandLine(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   []string
	}{
		{
			name:   "single-line values are rendered as before",
			values: map[string]string{"B": "two", "A": "one"},
			want:   []string{"--" + flagSetVariables, "A=one", "--" + flagSetVariables, "B=two"},
		},
		{
			name:   "one multi-line value takes the whole map off the command line",
			values: map[string]string{"PLAIN": "kept", "TLS_KEY": pemKey},
			want:   nil,
		},
		{
			name:   "a lone carriage return counts, because csv ends a record on one",
			values: map[string]string{"TLS_KEY": "first\rsecond"},
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &command{}
			c.pairs(flagSetVariables, tt.values)
			if strings.Join(c.args(), " ") != strings.Join(tt.want, " ") {
				t.Errorf("rendered %q, want %q", c.args(), tt.want)
			}
		})
	}
}

// TestZarfConfigEnvWritesTheKeysZarfReads holds the other half: the file the module writes is
// shaped like the one zarf's viper reads its flag defaults from, nested under the dotted keys that
// zarf's own constants name.
func TestZarfConfigEnvWritesTheKeysZarfReads(t *testing.T) {
	t.Setenv(envZarfConfig, "")

	env, cleanup, err := zarfConfigEnv(map[string]map[string]string{
		viperDeploySet:       {"TLS_KEY": pemKey},
		viperDeploySetValues: {"ONE_LINE": "stays on the command line"},
	})
	if err != nil {
		t.Fatalf("writing the config file: %v", err)
	}
	defer cleanup()

	if len(env) != 1 || !strings.HasPrefix(env[0], envZarfConfig+"=") {
		t.Fatalf("env = %q, want one %s entry", env, envZarfConfig)
	}
	path := strings.TrimPrefix(env[0], envZarfConfig+"=")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reading the config file: %v", err)
	}
	// It holds whatever the operator declined to put on a command line.
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("config file mode = %o, want 600", mode)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the config file: %v", err)
	}
	var doc struct {
		Package struct {
			Deploy struct {
				Set       map[string]string `yaml:"set"`
				SetValues map[string]string `yaml:"set_values"`
			} `yaml:"deploy"`
		} `yaml:"package"`
	}
	if err := goyaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parsing the config file: %v", err)
	}
	if doc.Package.Deploy.Set["TLS_KEY"] != pemKey {
		t.Errorf("package.deploy.set.TLS_KEY = %q, want the whole key", doc.Package.Deploy.Set["TLS_KEY"])
	}
	// The single-line map is already on the command line, and writing it here too would be the
	// one that loses: pflag replaces the config file's map rather than merging into it.
	if doc.Package.Deploy.SetValues != nil {
		t.Errorf("package.deploy.set_values = %v, want it left on the command line", doc.Package.Deploy.SetValues)
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the config file outlived the run: %v", err)
	}
}

// TestZarfConfigEnvWritesNothingWithoutAMultilineValue keeps the common case exactly as it was:
// no file, no environment entry, and the same command line the module always rendered.
func TestZarfConfigEnvWritesNothingWithoutAMultilineValue(t *testing.T) {
	t.Setenv(envZarfConfig, "")

	env, cleanup, err := zarfConfigEnv(map[string]map[string]string{
		viperDeploySet: {"PLAIN": "one line"},
	})
	if err != nil {
		t.Fatalf("zarfConfigEnv: %v", err)
	}
	defer cleanup()
	if env != nil {
		t.Errorf("env = %q, want nothing written", env)
	}
}

// TestZarfConfigEnvRefusesAnOperatorsOwnConfig is the stated limit of the shortcut: the module
// writes the file zarf reads, so it cannot also honour one the operator chose.
func TestZarfConfigEnvRefusesAnOperatorsOwnConfig(t *testing.T) {
	t.Setenv(envZarfConfig, "/etc/zarf/zarf-config.yaml")

	_, cleanup, err := zarfConfigEnv(map[string]map[string]string{
		viperDeploySet: {"TLS_KEY": pemKey},
	})
	defer cleanup()
	if err == nil {
		t.Fatal("a multi-line value was accepted while ZARF_CONFIG named another file")
	}
	if !strings.Contains(err.Error(), "/etc/zarf/zarf-config.yaml") {
		t.Errorf("the refusal does not name the operator's file: %v", err)
	}
}
