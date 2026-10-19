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
	"reflect"
	"strings"
)

// Scaffolding the tests in this package share, named for what it holds rather than for a source
// file, the way internal/zarfmod/helpers_test.go is.

// jsonFieldNames returns the json name of every field of t and of every struct it holds, with the
// slices and pointers walked through.
//
// The whole tree rather than the top level, because a section added to an existing one is as
// invisible to the role as a whole new section: requires.fluxcd.version and requires.fluxcd are
// the same kind of drift one level apart.
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
