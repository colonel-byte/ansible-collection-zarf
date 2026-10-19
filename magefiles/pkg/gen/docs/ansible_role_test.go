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

package docs

import (
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

// TestReconcileRoleDefaultsHoldsTheTwoFilesTogether covers the contract between a role's
// defaults/main.yml and its meta/argument_specs.yml.
//
// A role's variables are declared twice, and only this function relates them. Each way they can
// disagree reaches a reader as a page that is wrong rather than a run that fails: a variable with
// a default and no documentation is a row silently missing from the table, one documented with no
// default is a row describing a variable the role does not have, and a default spelled differently
// in the two files is a page stating a value the role does not use.
//
// The order matters too. The page lists variables in defaults/main.yml order, because that file is
// the one an operator copies from.
func TestReconcileRoleDefaultsHoldsTheTwoFilesTogether(t *testing.T) {
	documented := []namedOption{
		{
			name: "zarf_binary",
			docOption: docOption{
				Default: "zarf",
			},
		},
		{
			name: "zarf_quiet",
			docOption: docOption{
				Default: true,
			},
		},
	}
	defaults := goyaml.MapSlice{
		{
			Key:   "zarf_quiet",
			Value: true,
		},
		{
			Key:   "zarf_binary",
			Value: "zarf",
		},
	}

	t.Run("the page follows defaults order, not documentation order", func(t *testing.T) {
		ordered, err := reconcileRoleDefaults("prerequisites", documented, defaults)
		if err != nil {
			t.Fatalf("a matching pair was rejected: %v", err)
		}
		var names []string
		for _, o := range ordered {
			names = append(names, o.name)
		}
		if got, want := strings.Join(names, ","), "zarf_quiet,zarf_binary"; got != want {
			t.Errorf("the options came out as %q, want defaults/main.yml order %q", got, want)
		}
	})

	for _, tt := range []struct {
		name       string
		documented []namedOption
		defaults   goyaml.MapSlice
		wantErr    string
	}{
		{
			name:       "a default with no documentation",
			documented: documented,
			defaults: append(goyaml.MapSlice{
				{
					Key:   "zarf_engine_namespaces",
					Value: "flux-system",
				},
			}, defaults...),
			wantErr: "has a default but no entry in meta/argument_specs.yml",
		},
		{
			name: "documentation with no default",
			documented: append([]namedOption{
				{
					name: "zarf_engine_namespaces",
					docOption: docOption{
						Default: "flux-system",
					},
				},
			}, documented...),
			defaults: defaults,
			wantErr:  "is documented in meta/argument_specs.yml but has no default",
		},
		{
			name: "the same variable defaulted differently in each file",
			documented: []namedOption{
				{
					name: "zarf_binary",
					docOption: docOption{
						Default: "/usr/bin/zarf",
					},
				},
				{
					name: "zarf_quiet",
					docOption: docOption{
						Default: true,
					},
				},
			},
			defaults: defaults,
			wantErr:  "defaults to zarf in defaults/main.yml but /usr/bin/zarf in meta/argument_specs.yml",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := reconcileRoleDefaults("prerequisites", tt.documented, tt.defaults)
			if err == nil {
				t.Fatalf("the mismatch was accepted, want an error naming %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("the error was %q, want one naming %q", err, tt.wantErr)
			}
		})
	}
}
