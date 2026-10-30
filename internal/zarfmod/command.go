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
	"sort"
	"strconv"
	"strings"
)

// command accumulates the argument vector a module renders. Every flag it adds is optional in the
// same way: a parameter the operator did not set adds nothing.
type command struct {
	argv []string
}

// newCommand starts a command line with the action and its positional arguments.
func newCommand(action string, positional ...string) *command {
	c := &command{argv: []string{action}}
	c.argv = append(c.argv, positional...)
	return c
}

// bare adds a flag that takes no value.
func (c *command) bare(name string) {
	c.argv = append(c.argv, "--"+name)
}

// flag adds a flag and its value, and adds nothing when the value is empty.
func (c *command) flag(name, value string) {
	if value == "" {
		return
	}
	c.argv = append(c.argv, "--"+name, value)
}

// boolFlag adds a flag written as --name=value, which is how a boolean flag is given a false value
// on a pflag command line. Nothing is added when the operator said nothing.
func (c *command) boolFlag(name string, value *bool) {
	if value == nil {
		return
	}
	c.argv = append(c.argv, "--"+name+"="+strconv.FormatBool(*value))
}

// valueFlag adds a flag written as --name=value, for a flag that takes a value but declares a
// default for being given none. pflag reads such a flag the way it reads a boolean: the space form
// consumes no argument, so `--verify always` leaves "always" as a positional argument and zarf
// rejects the command for having one too many.
func (c *command) valueFlag(name, value string) {
	if value == "" {
		return
	}
	c.argv = append(c.argv, "--"+name+"="+value)
}

// intFlag adds a flag and its integer value, and adds nothing when the operator said nothing.
func (c *command) intFlag(name string, value *int) {
	if value == nil {
		return
	}
	c.argv = append(c.argv, "--"+name, strconv.Itoa(*value))
}

// repeated adds one copy of the flag per value, which is how a string slice flag is given more
// than one.
func (c *command) repeated(name string, values []string) {
	for _, value := range values {
		c.argv = append(c.argv, "--"+name, value)
	}
}

// pairs adds one copy of the flag per entry, as key=value, which is how pflag's stringToString
// reads a map. The keys are sorted so that the same parameters render the same command line: the
// response reports the vector it ran, and a vector whose order comes from Go's map iteration
// cannot be compared between two runs or asserted in a test.
func (c *command) pairs(name string, values map[string]string) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		c.argv = append(c.argv, "--"+name, csvSafePair(key+"="+values[key]))
	}
}

// csvSafePair renders one key=value pair in the form pflag's stringToString will read back whole.
//
// That type's Set counts the `=` in what it is given, and anything past the first parses the whole
// string through encoding/csv. A csv record ends at the first line break and a field at the first
// comma, so a value carrying either is silently cut short -- a PEM private key supplies the second
// `=` as base64 padding and a line break on every line, and arrives as its first line alone. See
// finding 7 in docs/ZEP-finds.md.
//
// A csv field may hold both, so long as the whole field is quoted and its own quotes are doubled.
// Quoting the pair rather than the value is what makes the key survive with it: the field is split
// on `=` after csv has handed it back, not before.
//
// A pair holding one `=` never reaches csv, and goes as it always did. Quoting it would be worse
// than leaving it alone, because that branch strips the quotes with strings.Trim and would take a
// quote the value itself ended with. pflag loses such a quote either way, which is the one shape
// this cannot render and finding 7's proposed change to zarf would fix.
func csvSafePair(pair string) string {
	if strings.Count(pair, "=") < 2 {
		return pair
	}
	return `"` + strings.ReplaceAll(pair, `"`, `""`) + `"`
}

// args is the finished argument vector.
func (c *command) args() []string {
	return c.argv
}
