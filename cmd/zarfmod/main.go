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

// Command zarfmod answers Ansible as whichever zarf module it was invoked as.
//
// It is the entry point the colonel_byte.zarf collection invokes, under one of the zarf_ names
// that symlink to it: zarf_init, zarf_package_deploy, zarf_kubectl. It is deliberately this small:
// it takes over standard output, reads the module's JSON parameters, renders the zarf command
// line, executes zarf's own command in this process, follows what it does, and writes one JSON
// object. Everything it does is in internal/zarfmod.
//
// One binary under several names is what ZEP-0072 describes: the proposal has the zarf binary
// itself dispatch on argv[0], so that a zarf_init symlink to /usr/bin/zarf is an Ansible module.
// This was a main package per module until the modules linked zarf in, at which point each
// binary carried its own copy of zarf's dependency tree -- 269MB apiece -- to reach dispatch code
// that was already shared. The symlinks are the proposal's own shape and cost one copy, which is
// also the answer to what a third module would have cost.
//
// Zarf is linked in rather than run as a child, so this binary enters the same commands an
// operator's command line enters. That is what makes it a proof of ZEP-0072 rather than a
// demonstration that zarf can be driven from outside: what the proposal asks for is this with the
// dispatch hook in zarf's own main, and zarf's main is already shaped to take one. See
// docs/agent/choice-zarf-ansible-module.md.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/colonel-byte/ansible-collection-zarf/internal/zarfmod"
)

func main() {
	// Cancel on the first signal and leave on the second. The cancelled context is the one zarf
	// executes with, which is the behaviour ZEP-0072 leans toward for a controller that gives up
	// mid-run: an initialisation still mutating a cluster nobody is watching is worse than a
	// partial one a later run reconciles. This is the same wiring zarf's own main has.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		first := true
		for {
			<-signalCh
			if first {
				first = false
				cancel()
				continue
			}
			os.Exit(1)
		}
	}()

	name, ok := zarfmod.ModuleName(os.Args[0])
	if !ok {
		// Invoked under a name that is not a module -- which includes this binary's own name, so
		// that running it by hand says what it is for. Say so on stderr rather than emitting a
		// JSON object a human did not ask for.
		fmt.Fprintf(os.Stderr,
			"%s is an Ansible module file for zarf. Invoke it through one of its %s symlinks (%s), "+
				"or set %s.\n",
			os.Args[0], zarfmod.Prefix, strings.Join(zarfmod.ModuleFiles(), ", "), zarfmod.EnvModule)
		os.Exit(2)
	}

	os.Exit(zarfmod.Run(ctx, name, os.Args, zarfmod.LibraryRunner{}))
}
