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
	"encoding/json"
	"strings"
	"testing"
)

// Tests for the module result: exactly one JSON object on standard output, whatever happened.

func TestResponseIsOneJSONObject(t *testing.T) {
	var out strings.Builder
	resp := &Response{
		Changed: true,
		Msg:     "initialised the cluster",
		Zarf: &Detail{
			Module:        "init",
			ComponentsRan: []string{"zarf-injector"},
			ChangedSignal: SignalPartial,
		},
	}
	if err := resp.emit(&out); err != nil {
		t.Fatalf("emit reported %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("the response was not one JSON object: %v (%s)", err, out.String())
	}
	if _, ok := decoded["zarf"]; !ok {
		t.Errorf("the response carried no zarf detail: %s", out.String())
	}
}
