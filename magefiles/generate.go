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

//go:build mage
// +build mage

package main

import (
	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/gen/docs"
	"github.com/colonel-byte/ansible-collection-zarf/magefiles/pkg/gen/zarfflags"
	"github.com/magefile/mage/mg"
)

type (
	Generate mg.Namespace
)

// Document regenerates every generated page in the book: the collection reference under
// docs/ansible, the Go reference under docs/golang, docs/index.md and docs/security.md, and
// docs/SUMMARY.md.
func (Generate) Document() error {
	return docs.GenerateDocument()
}

// ZarfFlags records the flag surface of the zarf commands the modules wrap, read from the
// installed zarf. Set ZARF_ANSIBLE_FLAGS_BINARY to name one that is not on PATH.
func (Generate) ZarfFlags() error {
	return zarfflags.Generate()
}
