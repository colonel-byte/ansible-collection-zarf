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
func TestCommittedSchemasAreCurrent(t *testing.T) {
	for _, tt := range []struct {
		name   string
		file   string
		render func() ([]byte, error)
	}{
		{
			name:   "the PackagePrerequisites schema",
			file:   PrerequisitesFile,
			render: RenderPrerequisites,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(Dir, tt.file)

			committed, err := os.ReadFile(path) //nolint:gosec // a path from this package's own constants
			if err != nil {
				t.Fatalf("unable to read the committed schema at %s: %v", path, err)
			}

			rendered, err := tt.render()
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
