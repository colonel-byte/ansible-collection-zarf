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

// Package schema generates the JSON Schema documents this repository publishes, from the Go types
// that define them.
//
// There are two. The colonel-byte.dev/v1alpha1 PackagePrerequisites schema is reflected from
// api/colonel-byte.dev/v1alpha1/prerequisites, and is the one a package author validates a document
// against. The package-resources schema is reflected from magefiles/pkg/pkgvendor, and describes the
// resources.json a reference package declares its upstream pin in.
//
// Both are generated rather than hand-written for the usual reason -- a hand-written schema beside a
// Go struct is two definitions of one format, and the one nobody runs goes stale -- and both are
// committed rather than built on demand because the files that validate against them name a
// `$schema` path that has to resolve to a file that is simply there.
//
// The descriptions in the output are the godoc comments on the types, read by the reflector's
// AddGoComments. That is the whole reason the doc comments on those types are written as prose for
// a package author rather than as notes for a Go reader: they are the schema's documentation, and
// an editor with YAML schema support shows them while the document is being written.
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/invopop/jsonschema"

	"github.com/colonel-byte/ansible-collection-zarf/api/colonel-byte.dev/v1alpha1/prerequisites"
	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/pkgvendor"
)

// Dir is where the generated schemas are written, at the repository root so that a package in
// another repository can reference one by a path that survives being vendored next to it.
var Dir = "schema"

// ResourcesFile is the schema of a reference package's resources.json, the declaration
// magefiles/pkg/pkgvendor reads to vendor upstream manifests from a pinned release.
//
// It is generated for the same reason the prerequisites schema is -- the Go type is the definition,
// and a hand-written schema beside it is a second one that goes stale -- and it is committed
// because the file it validates names it in a `$schema` key that has to resolve to something.
// colonel-byte/zarf-pkg-fluxcd commits a hand-written resources.schema.json; this is that file,
// reflected from the type that actually reads it.
const ResourcesFile = "package-resources-schema.json"

// PrerequisitesFile is the PackagePrerequisites schema's name under Dir.
//
// It is spelled zarf-v1alpha1-... rather than colonel-byte.dev-... because the name a package
// author writes it as is a filename in their own tree, beside the zarf.yaml it belongs to: the
// document is about a zarf package, and the API group is already inside the file.
const PrerequisitesFile = "zarf-v1alpha1-package-prerequisites-schema.json"

// resourcesDir is the directory holding the resources.json type, parsed for its doc comments the
// same way apiDir is. It is under magefiles/ rather than under api/ because it is a build input
// read by a mage target, not an API a package author writes against.
const resourcesDir = "./magefiles/pkg/pkgvendor"

// modulePath and apiDir are what AddGoComments needs to find the doc comments: the module the
// types live in, and the directory to parse relative to the working directory. The generator runs
// from the repository root, which every mage target in this repository does.
const (
	modulePath = "github.com/colonel-byte/ansible-collection-zarf"
	apiDir     = "./api"
)

// schemaIDBase is the prefix of the $id a generated document carries. Nothing fetches it -- an $id
// is an identifier, not a location -- and it is set explicitly so that the output does not carry
// the reflector's default, which is built from the Go package's import path and would name a
// directory on somebody's disk.
const schemaIDBase = "https://colonel-byte.github.io/ansible-collection-zarf/schema/"

// Generate writes every schema under Dir.
func Generate() error {
	if err := os.MkdirAll(Dir, 0o775); err != nil {
		return err
	}

	for _, document := range []struct {
		file   string
		render func() ([]byte, error)
	}{
		{
			file:   PrerequisitesFile,
			render: RenderPrerequisites,
		},
		{
			file:   ResourcesFile,
			render: RenderResources,
		},
	} {
		rendered, err := document.render()
		if err != nil {
			return err
		}

		path := filepath.Join(Dir, document.file)
		fmt.Println(path)
		if err := os.WriteFile(path, rendered, 0o644); err != nil { //nolint:gosec // a published schema is world-readable
			return err
		}
	}
	return nil
}

// RenderPrerequisites returns the PackagePrerequisites schema as it belongs on disk.
//
// Rendering is separate from writing so that schema_test.go can hold the committed file against a
// fresh render. That test is what makes the committed copy trustworthy: `go build` is perfectly
// happy with a schema that stopped matching the struct it came from, and a stale schema does not
// fail -- it validates the wrong document, which is worse than not validating one.
func RenderPrerequisites() ([]byte, error) {
	return render(&prerequisites.Document{}, apiDir, PrerequisitesFile)
}

// RenderResources returns the package-resources schema as it belongs on disk.
func RenderResources() ([]byte, error) {
	return render(&pkgvendor.Resources{}, resourcesDir, ResourcesFile)
}

// render reflects one type into a schema document, reading its descriptions from the doc comments
// in sourceDir.
func render(subject any, sourceDir, file string) ([]byte, error) {
	reflector := &jsonschema.Reflector{
		// The root is the document itself rather than a $ref into $defs. An author writes
		// `$schema=` at the top of a file and expects the editor to complete the keys of that
		// file; a root that only holds a reference works in a validator and not in every editor.
		ExpandedStruct: true,
		// An unknown key is an error, which is the behaviour both formats want: the common mistake
		// is a misspelled or misplaced key -- `crd:` for `crds:`, or `version` under `requires`
		// rather than under `requires.kubernetes` -- and a schema that allows additional properties
		// accepts every one of them silently.
		AllowAdditionalProperties: false,
	}
	if err := reflector.AddGoComments(modulePath, sourceDir); err != nil {
		return nil, fmt.Errorf("unable to read the doc comments under %s: %w", sourceDir, err)
	}

	document := reflector.Reflect(subject)
	document.ID = jsonschema.ID(schemaIDBase + file)

	// MarshalIndent rather than Encoder.SetIndent: the trailing newline is added below, because the
	// Encoder writes one and MarshalIndent does not, and the file has to end with exactly one
	// either way for the end-of-file-fixer hook to leave it alone.
	rendered, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(bytes.TrimRight(rendered, "\n"), '\n'), nil
}
