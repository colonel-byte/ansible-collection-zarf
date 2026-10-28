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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/cmd"
)

// The viper keys zarf reads each map flag's default from, named through zarf's own constants so a
// release that renames one fails the build rather than leaving a config file nothing reads.
var (
	viperDeploySet       = cmd.VPkgDeploySet
	viperDeploySetValues = cmd.VPkgDeploySetValues
	viperRemoveSetValues = cmd.VPkgRemoveSetValues
)

// envZarfConfig names the config file zarf reads instead of searching for one.
const envZarfConfig = "ZARF_CONFIG"

// hasMultiline reports whether any value in the map carries a line break, which is what makes the
// map impossible to give zarf on the command line.
//
// zarf declares --set-variables and --set-values with pflag's StringToString, whose Set parses the
// value through encoding/csv as soon as it holds more than one `=`. A csv record ends at the first
// line break, so everything past it is dropped and the flag is left holding the first line alone.
// A PEM private key supplies the second `=` as base64 padding and the line break on every line, so
// it arrives as "-----BEGIN RSA PRIVATE KEY-----" and the cluster gets a secret with no key in it.
// See finding 7 in docs/ZEP-finds.md.
//
// The line break alone is tested rather than pflag's own `=` threshold: a value that survives
// today because it carries one `=` would start truncating the day that heuristic is tuned, and
// routing it through the config file is correct either way.
func hasMultiline(values map[string]string) bool {
	for _, value := range values {
		if strings.ContainsAny(value, "\r\n") {
			return true
		}
	}
	return false
}

// zarfConfigEnv writes the maps the command line cannot carry to a zarf config file, and returns
// the environment entry pointing zarf at it along with the func that removes it again.
//
// Routing is per flag and all-or-nothing, because pflag's StringToString replaces the whole map on
// the flag's first appearance rather than merging into the default it was registered with. A map
// split between the config file and the command line would therefore lose the config file's half.
//
// Nothing is written when every value is a single line, which is the common case: the command line
// stays exactly what it was, and so does the vector the module reports having run.
func zarfConfigEnv(maps map[string]map[string]string) ([]string, func(), error) {
	noop := func() {}

	keys := make([]string, 0, len(maps))
	for key, values := range maps {
		if hasMultiline(values) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil, noop, nil
	}
	sort.Strings(keys)

	// shortcut: an operator who set ZARF_CONFIG is refused rather than merged with, because
	// merging means reading their file and deciding which half wins. Read and merge it here if a
	// package ever needs both a multi-line variable and an operator's own config file.
	if existing := os.Getenv(envZarfConfig); existing != "" {
		return nil, noop, fmt.Errorf(
			"a multi-line value must be passed to zarf through a config file, but %s already names one (%s): "+
				"move those settings onto the module's parameters, or pass the value on one line",
			envZarfConfig, existing)
	}

	doc := map[string]any{}
	for _, key := range keys {
		nest(doc, key, maps[key])
	}
	body, err := goyaml.Marshal(doc)
	if err != nil {
		return nil, noop, fmt.Errorf("unable to render the zarf config file: %w", err)
	}

	// The values here are the ones an operator chose not to put on a command line, so the
	// directory is the process's own and the file is readable by nobody else.
	dir, err := os.MkdirTemp("", "zarfmod-config-")
	if err != nil {
		return nil, noop, fmt.Errorf("unable to create a directory for the zarf config file: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) } //nolint:errcheck // the process is about to exit

	path := filepath.Join(dir, "zarf-config.yaml")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		cleanup()
		return nil, noop, fmt.Errorf("unable to write the zarf config file: %w", err)
	}
	return []string{envZarfConfig + "=" + path}, cleanup, nil
}

// nest writes values at a dotted viper key, building the maps along the way.
func nest(doc map[string]any, key string, values map[string]string) {
	parts := strings.Split(key, ".")
	for _, part := range parts[:len(parts)-1] {
		child, ok := doc[part].(map[string]any)
		if !ok {
			child = map[string]any{}
			doc[part] = child
		}
		doc = child
	}
	doc[parts[len(parts)-1]] = values
}

// configEnv routes the maps shared by init and deploy, both of which zarf reads from the deploy
// keys.
func (p *commonParams) configEnv() ([]string, func(), error) {
	return zarfConfigEnv(map[string]map[string]string{
		viperDeploySet:       p.SetVariables,
		viperDeploySetValues: p.SetValues,
	})
}
