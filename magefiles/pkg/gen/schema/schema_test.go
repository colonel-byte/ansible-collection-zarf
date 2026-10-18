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
	"reflect"
	"strings"
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

// roleTaskFiles are the colonel_byte.zarf.prerequisites role's task files, which are the only
// consumer of the requires half of the format that this repository ships.
var roleTaskFiles = []string{
	filepath.Join("ansible", "colonel_byte", "zarf", "roles", "prerequisites", "tasks", "main.yml"),
	filepath.Join("ansible", "colonel_byte", "zarf", "roles", "prerequisites", "tasks", "version_check.yml"),
}

// TestEveryRequirementIsNamedByTheRole holds the format against the role that checks it.
//
// This is the one drift in the prerequisites format that nothing else catches. Add a section to
// Requires, run `generate:schema`, and TestCommittedSchemasAreCurrent and
// TestExampleDocumentsMatchTheTypes both pass: the schema describes the new field, the examples
// decode, and the role goes on ignoring it. A package then declares a prerequisite, the role
// reports success, and nothing checked it -- which is the silent success the whole format exists
// to prevent, reached through the format rather than through a cluster.
//
// What it proves is narrower than its name suggests, and deliberately so: it holds that every
// field name under `requires` appears somewhere in the role's tasks, not that the role checks it
// correctly. A field named `version` added to a new section would pass for free, because `version`
// is already there. The case it does catch is the one that actually happens -- a new section or a
// new field under a new name, which no expression in the role mentions -- and when it fires, the
// fix is to check the field or to record it in zarf_prerequisites_skipped. Naming it as unchecked
// is a legitimate way to pass this test, because an unchecked field an operator is told about is
// the honest outcome; silently ignoring one is not.
//
// provides is not covered. Nothing in this repository reads it: it is declared for an orchestrator
// ordering a bundle, which is the half of the contract this collection does not implement.
func TestEveryRequirementIsNamedByTheRole(t *testing.T) {
	var tasks string
	for _, path := range roleTaskFiles {
		source, err := os.ReadFile(path) //nolint:gosec // a path from this test's own constants
		if err != nil {
			t.Fatalf("unable to read %s: %v", path, err)
		}
		tasks += string(source)
	}

	names := jsonFieldNames(reflect.TypeFor[prerequisites.Requires]())
	if len(names) == 0 {
		t.Fatal("no json field names found on prerequisites.Requires, so this test proves nothing")
	}

	for _, name := range names {
		if !strings.Contains(tasks, name) {
			t.Errorf("no task in the prerequisites role mentions %q, which requires.%s declares. "+
				"Either check it, or append it to zarf_prerequisites_skipped so an operator is "+
				"told it was not.", name, name)
		}
	}
}

// jsonFieldNames returns the json name of every field of t and of every struct it holds, with the
// slices and pointers walked through.
//
// The whole tree rather than the top level, because a section added to an existing one is as
// invisible to the role as a whole new section: requires.fluxcd.crds and requires.fluxcd are the
// same kind of drift one level apart.
func jsonFieldNames(t reflect.Type) []string {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	var names []string
	for i := range t.NumField() {
		field := t.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		names = append(names, name)
		names = append(names, jsonFieldNames(field.Type)...)
	}
	return names
}
