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
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// pemKey is the value that found this. The base64 padding supplies the second `=` that sends the
// pair through pflag's csv branch, and the line breaks are what that branch truncates at.
const pemKey = "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA==\n-----END RSA PRIVATE KEY-----"

// extraEnvs is the shape that found it a second time: a block of YAML passed as one init variable.
const extraEnvs = `- name: REGISTRY_STORAGE_S3_REGION
  value: "us-east-1"
- name: REGISTRY_STORAGE_S3_ACCESSKEY
  valueFrom:
    secretKeyRef:
      name: registry-bucket
      key: AWS_ACCESS_KEY_ID`

// TestPairsSurviveZarfsOwnFlagParsing is the test that matters: what the module renders is parsed
// by the flag type zarf declares, and the value has to come back whole. It is the only thing that
// distinguishes rendering a pair from rendering one that reads back as its first line.
func TestPairsSurviveZarfsOwnFlagParsing(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "a PEM private key, which holds both a line break and a second equals sign",
			value: pemKey,
		},
		{
			name:  "a block of YAML given as one variable",
			value: extraEnvs,
		},
		{
			name:  "a line break with no second equals sign, which never reached csv",
			value: "-----BEGIN KEY-----\nMIIEowI\n-----END KEY-----",
		},
		{
			name:  "a comma, which csv would otherwise read as the end of the field",
			value: "one,two=three",
		},
		{
			name:  "a quote the value carries itself",
			value: `say "hi" and = too`,
		},
		{
			name:  "an ordinary single-line value",
			value: "plain",
		},
		{
			name:  "a value that is itself a key=value pair",
			value: "a=b=c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &command{}
			c.pairs(flagSetVariables, map[string]string{"TLS_KEY": tt.value})

			var got map[string]string
			fs := pflag.NewFlagSet("deploy", pflag.ContinueOnError)
			fs.StringToStringVar(&got, flagSetVariables, nil, "")
			if err := fs.Parse(c.args()); err != nil {
				t.Fatalf("zarf's own flag type rejected %q: %v", c.args(), err)
			}
			if got["TLS_KEY"] != tt.value {
				t.Errorf("TLS_KEY came back as\n%q\nwant\n%q", got["TLS_KEY"], tt.value)
			}
		})
	}
}

// TestStringToStringTruncatesAnUnquotedPair is the evidence for finding 7, and it is here rather
// than in the finding so that a pflag release which fixes this says so by failing. Nothing in this
// repository is on trial: the pair is the one the module used to render.
func TestStringToStringTruncatesAnUnquotedPair(t *testing.T) {
	var got map[string]string
	fs := pflag.NewFlagSet("deploy", pflag.ContinueOnError)
	fs.StringToStringVar(&got, flagSetVariables, nil, "")
	if err := fs.Parse([]string{"--" + flagSetVariables, "TLS_KEY=" + pemKey}); err != nil {
		t.Fatalf("parsing the flag: %v", err)
	}
	if got["TLS_KEY"] != "-----BEGIN RSA PRIVATE KEY-----" {
		t.Errorf("TLS_KEY = %q, want the first line alone -- has pflag fixed this?", got["TLS_KEY"])
	}
}

// TestPairsKeepsTheirOrderAndLeavesSimpleValuesAlone holds the rendering itself, because the
// response reports the vector it ran and a quote added where none was needed would show up there.
// Only a pair that reaches csv is quoted, which is why the two single-`=` pairs are untouched.
func TestPairsKeepsTheirOrderAndLeavesSimpleValuesAlone(t *testing.T) {
	c := &command{}
	c.pairs(flagSetValues, map[string]string{
		"B":      "two",
		"A":      "one",
		"NESTED": "x=y",
	})
	want := []string{
		"--" + flagSetValues, "A=one",
		"--" + flagSetValues, "B=two",
		"--" + flagSetValues, `"NESTED=x=y"`,
	}
	if strings.Join(c.args(), " ") != strings.Join(want, " ") {
		t.Errorf("rendered %q, want %q", c.args(), want)
	}
}
