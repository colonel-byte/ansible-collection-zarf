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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/colonel-byte/ansible-collection-zarf/api/colonel-byte.dev/v1alpha1/prerequisites"
	"sigs.k8s.io/yaml"
)

// Tests that hold the pages under skills/ against the format they teach.

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
