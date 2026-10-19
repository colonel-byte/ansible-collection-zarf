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

package testenv

import (
	"os"
	"strings"
	"testing"
)

// TestRunTempDirDoesNotNestInsideTMPDIR is the whole point of RunTempDir, and the regression it
// exists to prevent is one that passes on a laptop and fails in CI.
//
// The mage runner points TMPDIR at build/tmp inside the repository. ansible-core runs its task
// queue through a multiprocessing manager whose listener socket lives under TMPDIR as
// pymp-xxxxxxxx/listener-xxxxxxxx -- 32 bytes -- and Linux caps an AF_UNIX path at 107. On a
// GitHub runner the repository path alone is 65 of those, so build/tmp leaves 32 exactly: enough
// for the socket and nothing else. A run directory created *under* it overflows, and every
// ansible-playbook in the non-cluster suite dies with "Local RPC server did not start".
//
// So the assertion is not that the path is tidy. It is that the path does not inherit a TMPDIR
// that has already spent the budget, and that it lands on runTempRoot -- which is disk-backed,
// because the cluster suite puts close to a gigabyte here and /tmp is RAM on most distributions.
func TestRunTempDirDoesNotNestInsideTMPDIR(t *testing.T) {
	const ansibleSocketLen, maxUnixPathLen = 32, 107

	// A stand-in for the one the mage runner sets, long enough that nesting under it overflows.
	crowded := t.TempDir() + "/" + strings.Repeat("d", 80)
	if err := os.MkdirAll(crowded, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", crowded)

	dir, err := RunTempDir("testenv-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	}()

	if strings.HasPrefix(dir, crowded) {
		t.Errorf("the run directory is %s, nested inside the TMPDIR it was meant to replace", dir)
	}
	// Checking the filesystem rather than the path: a prefix assertion against runTempRoot would
	// compare the constant with itself and pass whatever it was changed to. The cluster suite
	// peaks near a gigabyte in here, so what matters is that the mount is not RAM.
	if mounts, err := os.ReadFile("/proc/mounts"); err != nil {
		t.Logf("no /proc/mounts, so the filesystem backing %s went unchecked: %v", runTempRoot, err)
	} else if strings.Contains(string(mounts), " "+runTempRoot+" tmpfs ") {
		t.Errorf("%s is a tmpfs mount, so the suites would stage close to a gigabyte in RAM", runTempRoot)
	}
	if got := len(dir) + ansibleSocketLen; got > maxUnixPathLen {
		t.Errorf("%s leaves %d bytes for ansible's socket, past the %d-byte AF_UNIX limit", dir, got, maxUnixPathLen)
	}
	if os.Getenv("TMPDIR") != dir {
		t.Errorf("TMPDIR is %s, and every child reads the run directory from it", os.Getenv("TMPDIR"))
	}
}
