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
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// FuzzReadArgs holds the property Args.Params exists for: a parameter the module does not accept
// is an error naming it, never a value silently dropped.
//
// A binary module has no argument_spec, so Params is the whole of what stands between a mistyped
// parameter and a run that ignores it. The JSON decode underneath is the standard library's, but
// the accepted-name walk is hand-written reflection over a struct with embedded fields, which is
// the part a fuzzer can reach.
func FuzzReadArgs(f *testing.F) {
	for _, seed := range []string{
		`{"storage_class":"local-path","init_package":"/srv/zarf-init.tar.zst","_ansible_check_mode":true}`,
		`{"registry_port":5000,"_ansible_verbosity":3,"_ansible_future_key":{"a":1}}`,
		`{"storage_clazz":"local-path"}`,
		`{"":null}`,
		`{"Binary":"zarf"}`,
		`{}`,
		`[]`,
		`{"init_package":1}`,
	} {
		f.Add([]byte(seed))
	}

	accepted := jsonFieldNames(reflect.TypeOf(&initParams{}))

	f.Fuzz(func(t *testing.T, document []byte) {
		args, err := ReadArgs(bytes.NewReader(document))
		if err != nil {
			return
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(document, &raw); err != nil {
			t.Fatalf("ReadArgs accepted a document encoding/json rejects: %v", err)
		}

		var unknown []string
		for name := range raw {
			if !strings.HasPrefix(name, controlPrefix) && !accepted[name] {
				unknown = append(unknown, name)
			}
		}

		var params initParams
		err = args.Params(&params)
		if len(unknown) > 0 && err == nil {
			t.Fatalf("Params accepted the unknown parameters %q from %q", unknown, document)
		}
	})
}
