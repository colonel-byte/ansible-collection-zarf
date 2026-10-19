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
	"regexp"
	"strings"
	"testing"

	"github.com/colonel-byte/ansible-collection-zarf/api/colonel-byte.dev/v1alpha1/prerequisites"
	"sigs.k8s.io/yaml"
)

// Tests that hold the reference packages under examples/ against the format.

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

// TestExampleDocumentVersionsMatchTheirPackage holds each reference package's prerequisites
// document against the zarf.yaml beside it.
//
// Nothing else does. `examples:vendor` writes the pinned upstream tag into zarf.yaml's
// metadata.version and stops there, and TestExampleDocumentsMatchTheTypes checks that
// metadata.package is the directory name but deliberately not the version -- so a pin that moved
// left app-fluxcd's document saying 2.9.3 against a package built from v2.9.5, on the one file
// README.md tells a reader to copy.
//
// The leading v is stripped from the zarf.yaml side rather than required on both: the tag is an
// upstream release name, and the document's version is a semver that the role and the schema both
// compare as one.
func TestExampleDocumentVersionsMatchTheirPackage(t *testing.T) {
	documents, err := filepath.Glob(filepath.Join("examples", "packages", "*", "docs", "prerequisites.yaml"))
	if err != nil {
		t.Fatalf("unable to list the reference packages' prerequisites documents: %v", err)
	}
	if len(documents) == 0 {
		t.Fatal("no prerequisites documents found under examples/packages, so this test proves nothing")
	}

	metadataVersion := regexp.MustCompile(`(?m)^  version: (\S+)`)
	for _, path := range documents {
		t.Run(path, func(t *testing.T) {
			pkg := filepath.Join(filepath.Dir(filepath.Dir(path)), "zarf.yaml")

			declared := map[string]string{}
			for name, file := range map[string]string{"the document": path, "the package": pkg} {
				source, err := os.ReadFile(file) //nolint:gosec // a path from the glob above
				if err != nil {
					t.Fatalf("unable to read %s: %v", file, err)
				}
				match := metadataVersion.FindSubmatch(source)
				if match == nil {
					t.Fatalf("%s declares no metadata version", file)
				}
				declared[name] = strings.TrimPrefix(string(match[1]), "v")
			}

			if declared["the document"] != declared["the package"] {
				t.Errorf("%s requires itself at %s, and %s is built at %s",
					path, declared["the document"], pkg, declared["the package"])
			}
		})
	}
}
