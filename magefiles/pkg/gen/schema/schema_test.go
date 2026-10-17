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
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/colonel-byte/ansible-collection-zarf/api/colonel-byte.dev/v1alpha1/prerequisites"
	"github.com/colonel-byte/ansible-collection-zarf/internal/testenv"
)

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

// TestExampleDocumentsMatchTheTypes decodes every reference package's prerequisites document into
// the Go types, with unknown fields rejected.
//
// The schema is generated from those types, so this is the same contract the schema states -- but
// checked without a JSON Schema validator, which this repository would otherwise have to vendor a
// dependency for. What it catches is the thing the reference packages are most likely to get
// wrong: an example written against an earlier shape of the format, which goes on reading
// perfectly well as a document nobody can validate.
//
// It is decoded through sigs.k8s.io/yaml rather than through goccy/go-yaml, which the rest of this
// repository parses YAML with, for the reason the types carry json tags: that package converts
// YAML to JSON and then decodes it, so the names it reads are the names the schema states. Reading
// the same file through a parser keyed on `yaml` tags would silently match nothing and pass.
//
// UnmarshalStrict is the load-bearing word. A non-strict decode ignores a key it does not know,
// which is exactly the misspelling this is here to catch.
func TestExampleDocumentsMatchTheTypes(t *testing.T) {
	documents, err := filepath.Glob(filepath.Join("examples", "packages", "*", "docs", "prerequisites.yaml"))
	if err != nil {
		t.Fatalf("unable to list the reference packages' prerequisites documents: %v", err)
	}
	if len(documents) == 0 {
		t.Fatal("no prerequisites documents found under examples/packages, so this test proves nothing")
	}

	for _, path := range documents {
		t.Run(path, func(t *testing.T) {
			source, err := os.ReadFile(path) //nolint:gosec // a path from the glob above
			if err != nil {
				t.Fatalf("unable to read %s: %v", path, err)
			}

			var document prerequisites.Document
			if err := yaml.UnmarshalStrict(source, &document); err != nil {
				t.Fatalf("%s does not decode into the prerequisites types: %v", path, err)
			}

			if document.APIVersion != prerequisites.APIVersion {
				t.Errorf("apiVersion is %q, want %q", document.APIVersion, prerequisites.APIVersion)
			}
			if document.Kind != prerequisites.Kind {
				t.Errorf("kind is %q, want %q", document.Kind, prerequisites.Kind)
			}
			if document.Metadata.Package == "" {
				t.Error("metadata.package is empty, so the document does not say which package it describes")
			}

			// The directory is named after the package, which is what makes a document that was
			// copied from the package beside it and not fully edited fail here rather than in a
			// playbook that reports the wrong package's requirements.
			dir := filepath.Base(filepath.Dir(filepath.Dir(path)))
			if document.Metadata.Package != dir {
				t.Errorf("metadata.package is %q, but the document sits in examples/packages/%s",
					document.Metadata.Package, dir)
			}
		})
	}
}
