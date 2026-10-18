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
)

// Tests for the WANT_JSON intake: the two ways a module's parameters arrive, and what an
// unrecognised one does.

func TestUnknownParameterIsNamed(t *testing.T) {
	args := argsFor(t, map[string]any{
		"init_package":  "/srv/staging/zarf-init-amd64.tar.zst",
		"registry_urll": "registry.bubbles.test",
	})

	var p initParams
	err := args.Params(&p)
	if err == nil {
		t.Fatal("a misspelled parameter was accepted")
	}
	if !strings.Contains(err.Error(), "registry_urll") {
		t.Errorf("the error did not name the misspelling: %v", err)
	}
}

func TestReadArgsFromStdin(t *testing.T) {
	args, err := ReadArgs(strings.NewReader(
		`{"init_package":"/srv/staging/zarf-init-amd64.tar.zst","_ansible_verbosity":2}`))
	if err != nil {
		t.Fatalf("ReadArgs reported %v", err)
	}
	if args.Control.Verbosity != 2 {
		t.Errorf("verbosity was %d, want 2", args.Control.Verbosity)
	}

	var p initParams
	if err := args.Params(&p); err != nil {
		t.Fatalf("Params reported %v", err)
	}
	if p.InitPackage != "/srv/staging/zarf-init-amd64.tar.zst" {
		t.Errorf("init_package was %q", p.InitPackage)
	}
}
