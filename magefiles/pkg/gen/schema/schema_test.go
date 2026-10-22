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

package schema

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/colonel-byte/ansible-collection-zarf/api/colonel-byte.dev/v1alpha1/prerequisites"
	"github.com/colonel-byte/ansible-collection-zarf/internal/testenv"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schemaResourceName is what the rendered schema is compiled under. The document's own $id is a
// published URL, and a compiler that resolved it would reach for the network.
const schemaResourceName = "prerequisites.json"

// Tests for the generator itself: that what it writes is what the Go types render to,
// and that the constraints it is asked to emit survive into the output.

// TestMain chdirs into the repository root, because both the generator and this test spell their
// paths relative to it: the reflector parses ./api for doc comments, and the committed schema is
// under schema/.
func TestMain(m *testing.M) {
	if _, _, err := testenv.Bootstrap(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// TestCommittedSchemasAreCurrent holds each committed schema against a fresh render.
//
// The schemas are generated from the Go types and then committed, because the files that validate
// against them name a `$schema` path that has to resolve to something -- and for the prerequisites
// schema, those files are in other repositories. That means the usual failure of a committed
// generator output: a field added to the type compiles, every other test passes, and the schema
// goes on describing the format as it was. Nothing notices, and what a stale schema does is worse
// than not validating at all -- it rejects a key that is now correct.
//
// The fix when this fails is to run the generator, not to edit the file:
//
//	go run ./magefiles/core generate:schema
//
// It covers every copy, not just the one under schema/: the prerequisites schema is committed a
// second time inside skills/zarf-package-prerequisites so that the skill is self-contained, and a
// second copy nothing checks is the same stale schema with an extra place to hide.
func TestCommittedSchemasAreCurrent(t *testing.T) {
	for _, document := range Documents() {
		path := filepath.Join(document.Dir, document.File)

		t.Run(path, func(t *testing.T) {
			committed, err := os.ReadFile(path) //nolint:gosec // a path from this package's own constants
			if err != nil {
				t.Fatalf("unable to read the committed schema at %s: %v", path, err)
			}

			rendered, err := document.Render()
			if err != nil {
				t.Fatalf("unable to render the schema: %v", err)
			}

			if string(committed) != string(rendered) {
				t.Errorf("%s is not what its Go type renders to. Run "+
					"`go run ./magefiles/core generate:schema` and commit the result.", path)
			}
		})
	}
}

// TestRenderedSchemaRefusesBothEngines holds the mutual exclusivity of the two engine sections in
// the schema the editors and CI read, not only in the role.
//
// `not: {required: [fluxcd, argocd]}` fails a document exactly when both keys are present, which
// is the whole of the rule: either engine alone is fine, and neither is fine for a package that
// needs no GitOps engine at all. It comes from Requires.JSONSchemaExtend, and nothing else in the
// generated output would notice it going away -- the schema would simply get more permissive, and
// the documents it stopped refusing are the ones nobody meant to write.
//
// The role refuses the same document, and that is not redundant: a document reaches the role
// through the zarf_prerequisites variable without ever meeting a validator. This holds the half
// that catches it while it is being written.
func TestRenderedSchemaRefusesBothEngines(t *testing.T) {
	rendered, err := RenderPrerequisites()
	if err != nil {
		t.Fatalf("unable to render the prerequisites schema: %v", err)
	}

	var document struct {
		Defs struct {
			Requires struct {
				Not *struct {
					Required []string `json:"required"`
				} `json:"not"`
			} `json:"Requires"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(rendered, &document); err != nil {
		t.Fatalf("the rendered schema does not decode: %v", err)
	}

	not := document.Defs.Requires.Not
	if not == nil {
		t.Fatal("the Requires schema states no `not`, so it accepts a document requiring both " +
			"fluxcd and argocd")
	}

	sort.Strings(not.Required)
	want := []string{prerequisites.EngineArgoCD, prerequisites.EngineFluxCD}
	sort.Strings(want)
	if strings.Join(not.Required, ",") != strings.Join(want, ",") {
		t.Errorf("the refused combination is %v, want %v", not.Required, want)
	}
}

// TestRenderedSchemaRefusesAmbiguousPackageRequirements holds the rule that a prerequisite package
// is named exactly one of two ways: by the one name it has, or by the set of names it is published
// under.
//
// It validates documents against the rendered schema rather than reading the schema's shape,
// because the shape was what went wrong. PackageRequirement.JSONSchemaExtend first expressed the
// rule as `anyOf`, which renders as branches that look right and accept a requirement carrying
// both keys -- two branches match, and `anyOf` wants one or more. A test asserting that the
// branches are present would have passed over that. Feeding the combination to a validator is
// what says so.
//
// The role refuses the same document, and that is not redundant: a document reaches the role
// through the zarf_prerequisites variable without meeting a validator, and the role reads `names`
// in preference to `name`. This holds the half that catches it while it is being written.
func TestRenderedSchemaRefusesAmbiguousPackageRequirements(t *testing.T) {
	rendered, err := RenderPrerequisites()
	if err != nil {
		t.Fatalf("unable to render the prerequisites schema: %v", err)
	}

	resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(rendered))
	if err != nil {
		t.Fatalf("the rendered schema does not decode: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaResourceName, resource); err != nil {
		t.Fatalf("unable to add the rendered schema to the compiler: %v", err)
	}
	compiled, err := compiler.Compile(schemaResourceName)
	if err != nil {
		t.Fatalf("the rendered schema does not compile: %v", err)
	}

	for _, testCase := range []struct {
		name  string
		entry map[string]any
		valid bool
	}{
		{
			name: "a package named on its own",
			entry: map[string]any{
				"name":    "init",
				"version": ">=0.85.0",
			},
			valid: true,
		},
		{
			name: "the several names a package is published under",
			entry: map[string]any{
				"names":   []any{"init", "init-slim"},
				"version": ">=0.85.0",
			},
			valid: true,
		},
		{
			name: "one name and several at once",
			entry: map[string]any{
				"name":  "init",
				"names": []any{"init", "init-slim"},
			},
			valid: false,
		},
		{
			name: "a version and nothing it constrains",
			entry: map[string]any{
				"version": ">=0.85.0",
			},
			valid: false,
		},
		{
			// Satisfies the exactly-one-key rule and then names nothing, which reaches an
			// operator as "one of the zarf packages  is required and is not deployed".
			name: "an empty list of names",
			entry: map[string]any{
				"names": []any{},
			},
			valid: false,
		},
		{
			name: "an empty name",
			entry: map[string]any{
				"name": "",
			},
			valid: false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := compiled.Validate(documentRequiring(testCase.entry))
			if testCase.valid && err != nil {
				t.Errorf("the schema refuses a requirement it should accept: %v", err)
			}
			if !testCase.valid && err == nil {
				t.Error("the schema accepts a requirement that names its package in two ways at " +
					"once, or in none, so a consumer has to invent a precedence between them")
			}
		})
	}
}

// documentRequiring is the smallest document that carries one requires.packages entry, so that a
// case is the entry and nothing else.
func documentRequiring(entry map[string]any) any {
	return map[string]any{
		"apiVersion": "colonel-byte.dev/v1alpha1",
		"kind":       "PackagePrerequisites",
		"metadata": map[string]any{
			"package": "a-package-with-a-package-requirement",
			"version": "1.0.0",
		},
		"requires": map[string]any{
			"packages": []any{entry},
		},
	}
}
