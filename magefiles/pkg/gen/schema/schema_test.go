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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
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
// invisible to the role as a whole new section: requires.gitops.engine and requires.gitops are the
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

func skillPages(t *testing.T) []string {
	t.Helper()

	pages, err := filepath.Glob(filepath.Join(filepath.Dir(SkillDir), "*", "SKILL.md"))
	if err != nil {
		t.Fatalf("unable to list the skill pages: %v", err)
	}
	if len(pages) == 0 {
		t.Fatalf("no SKILL.md found under %s, so these tests prove nothing", filepath.Dir(SkillDir))
	}
	return pages
}

// fencedYAML pulls every top-level ```yaml block out of a markdown page.
//
// Anchored at the start of a line, which is the whole of how an indented block is excluded. An
// unanchored pattern cannot see the closing fence of a block nested in a list item -- that fence
// is indented too -- so it runs on to the next one at column zero and captures the prose in
// between. The first version of this test did exactly that and silently had a 588-byte "document"
// made of a snippet, a task and two paragraphs.
//
// Excluding them is also right rather than merely convenient. A block indented into a list is a
// fragment illustrating one key; a whole document is written at the margin.
var fencedYAML = regexp.MustCompile("(?sm)^```yaml\\n(.*?)\\n^```$")

// prerequisiteExamples returns every fenced block on a page that is a whole PackagePrerequisites
// document, found by what it declares rather than by position.
func prerequisiteExamples(source []byte) [][]byte {
	var found [][]byte
	for _, match := range fencedYAML.FindAllSubmatch(source, -1) {
		if bytes.Contains(match[1], []byte("kind: "+prerequisites.Kind)) {
			found = append(found, match[1])
		}
	}
	return found
}

// TestSkillYAMLBlocksParse holds that every YAML block a skill shows is YAML.
//
// Weaker than the type check below and it covers more: skills/zarf-package shows a zarf.yaml, and
// zarf's own schema is not something this repository has Go types for, so the most that can be
// said about that example is that it parses. It is worth saying. A skill is copied from, and an
// example that does not parse wastes the time of whoever copies it before they find out.
func TestSkillYAMLBlocksParse(t *testing.T) {
	for _, page := range skillPages(t) {
		t.Run(page, func(t *testing.T) {
			source, err := os.ReadFile(page) //nolint:gosec // a path from the glob above
			if err != nil {
				t.Fatalf("unable to read %s: %v", page, err)
			}

			blocks := fencedYAML.FindAllSubmatch(source, -1)
			if len(blocks) == 0 {
				t.Fatalf("%s shows no YAML at all, which is not how any of these pages teach", page)
			}
			for i, block := range blocks {
				var parsed any
				if err := yaml.Unmarshal(block[1], &parsed); err != nil {
					t.Errorf("YAML block %d does not parse: %v\n%s", i, err, block[1])
				}
			}
		})
	}
}

// TestSkillExamplesMatchTheTypes decodes every PackagePrerequisites example embedded in a skill.
//
// The skills ship a committed copy of the JSON Schema and TestCommittedSchemasAreCurrent holds it
// against the Go types -- so the schema beside the page cannot go stale. The page itself could,
// and did: the example and the prose survived a change to the format that renamed a whole section,
// because regenerating the schema said nothing about the markdown next to it.
//
// That failure is worse here than in docs/. A guide is read by a person who can notice that it
// disagrees with the schema in front of them; a skill is instructions handed to an agent working
// in a package tree that has no other copy of the format, and the example is what gets copied.
//
// Every such block rather than the first, so that a page which grows a second example -- an Argo CD
// one beside the Flux one, say -- does not grow an unchecked one.
//
// UnmarshalStrict is the load-bearing word, the same as in TestExampleDocumentsMatchTheTypes: a
// non-strict decode ignores a key it does not know, which is exactly the renamed section this is
// here to catch.
func TestSkillExamplesMatchTheTypes(t *testing.T) {
	var total int
	for _, page := range skillPages(t) {
		source, err := os.ReadFile(page) //nolint:gosec // a path from the glob above
		if err != nil {
			t.Fatalf("unable to read %s: %v", page, err)
		}

		for i, example := range prerequisiteExamples(source) {
			total++
			t.Run(fmt.Sprintf("%s/%d", page, i), func(t *testing.T) {
				var document prerequisites.Document
				if err := yaml.UnmarshalStrict(example, &document); err != nil {
					t.Fatalf("the example does not decode into the prerequisites types: %v\n%s",
						err, example)
				}

				if document.APIVersion != prerequisites.APIVersion {
					t.Errorf("apiVersion is %q, want %q",
						document.APIVersion, prerequisites.APIVersion)
				}
				if document.Kind != prerequisites.Kind {
					t.Errorf("kind is %q, want %q", document.Kind, prerequisites.Kind)
				}

				// Independent checks rather than a switch. A switch here is an else-if chain, so
				// an example that had lost three sections reported one -- in the test holding the
				// page that teaches a role whose whole design is naming everything wrong at once.
				if document.Requires.Kubernetes == nil {
					t.Error("the example states no requires.kubernetes")
				}
				if document.Requires.FluxCD == nil && document.Requires.ArgoCD == nil {
					t.Error("the example names no GitOps engine, so it shows neither key")
				}
				if document.Requires.FluxCD != nil && document.Requires.ArgoCD != nil {
					t.Error("the example requires both GitOps engines, which the schema refuses")
				}
				if len(document.Requires.Controllers) == 0 {
					t.Error("the example states no requires.controllers")
				}
				if len(document.Requires.CRDs) == 0 {
					t.Error("the example states no requires.crds")
				}
				if len(document.Requires.Packages) == 0 {
					t.Error("the example states no requires.packages")
				}
				if document.Provides == nil {
					t.Error("the example states no provides")
				}
			})
		}
	}

	// The example is the one place a reader sees every section at once. A page that lost it would
	// pass every subtest above by having none to run.
	if total == 0 {
		t.Error("no skill shows a PackagePrerequisites example, so this test proves nothing")
	}
}

// TestSkillSchemaPathResolvesFromTheDocument holds the $schema line in each example against the
// layout the skill's own instructions prescribe.
//
// The two are written in different places on the page and drifted apart: the example said
// ../../schema/, while the steps say to copy the schema to schema/ beside the zarf.yaml -- which
// from docs/prerequisites.yaml is ../schema/. Following the page produced a document whose
// $schema resolved nowhere, and the symptom is an editor that silently validates nothing, which
// is the one thing that step exists to provide.
func TestSkillSchemaPathResolvesFromTheDocument(t *testing.T) {
	const prefix = "# yaml-language-server: $schema="

	for _, page := range skillPages(t) {
		source, err := os.ReadFile(page) //nolint:gosec // a path from the glob above
		if err != nil {
			t.Fatalf("unable to read %s: %v", page, err)
		}

		for i, example := range prerequisiteExamples(source) {
			t.Run(fmt.Sprintf("%s/%d", page, i), func(t *testing.T) {
				first, _, _ := strings.Cut(string(example), "\n")
				rel, ok := strings.CutPrefix(first, prefix)
				if !ok {
					t.Fatalf("the example does not open with %q, so an editor validates "+
						"nothing:\n%s", prefix, first)
				}

				// The document lives at docs/prerequisites.yaml in the package the skill tells
				// you to build, and the schema at schema/<name>.json beside its zarf.yaml.
				resolved := filepath.Join("docs", rel)
				want := filepath.Join("schema", filepath.Base(PrerequisitesFile))
				if resolved != want {
					t.Errorf("the example's $schema is %q, which resolves from "+
						"docs/prerequisites.yaml to %q; the layout the page prescribes puts it "+
						"at %q", rel, resolved, want)
				}
			})
		}
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
