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

// Package testenv holds the scaffolding the end-to-end suites share: finding the repository
// root, narrowing TMPDIR to one directory per run, and waiting on a cluster.
//
// It is a trimmed copy of the `test` package this proof grew up in, in colonel-byte/cargoship.
// What was dropped is everything that drove the cargoship CLI -- the binary lookup under build/,
// the command helpers and the staging-directory handling -- because this repository builds no
// such binary. What is left is the two things the zarf suites actually call.
package testenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Env carries what a suite needs to know about the machine it is running on.
type Env struct {
	// Arch is the architecture to ask zarf for packages of.
	Arch string
}

// Bootstrap chdirs into the repository root and reports the architecture, along with the root it
// moved into. The suites call this from TestMain: they spell every path -- testdata, playbooks,
// the collection -- relative to the repository root.
//
// Arch is runtime.GOARCH. Zarf's own config.GetArch() returns exactly that when no architecture
// has been forced on the command line, which is the only way these suites ever call it, so the
// zarf library is not worth a dependency here.
func Bootstrap() (Env, string, error) {
	root, err := RepoRoot()
	if err != nil {
		return Env{}, "", err
	}
	if err := os.Chdir(root); err != nil {
		return Env{}, "", err
	}

	return Env{Arch: runtime.GOARCH}, root, nil
}

// RepoRoot walks up from the working directory to the first parent holding a go.mod.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in any parent of %s", dir)
		}
		dir = parent
	}
}

// runTempRoot is where a suite's run directory is created: /var/tmp rather than /tmp, because the
// cluster suite peaks near a gigabyte of scratch and /tmp is a tmpfs on most Linux distributions --
// that is RAM, competing with the Docker and k3d processes the same suite is running. /var/tmp is
// disk-backed, is where the FHS puts temporaries that may be large, and at eight characters leaves
// the AF_UNIX budget below untouched.
//
// Naming an absolute path costs the portability the OS default gave: /var/tmp is on every Unix,
// so macOS is fine, but on Windows this fails in TestMain rather than skipping. That is the right
// trade here -- the cluster suite already skips on anything but Linux, the non-cluster suite needs
// an ansible-playbook that is not there, and nothing in this repository targets Windows.
const runTempRoot = "/var/tmp"

// RunTempDir creates the directory a suite points TMPDIR at for the length of one run, points
// TMPDIR at it, and returns it. The caller removes it when the run is over.
//
// Everything a suite spawns writes its temporaries somewhere, and under the mage runner that
// somewhere is build/tmp, inside the repository: k3d leaves a copy of the config it was given and
// a hosts file per cluster, zarf leaves a staging directory per deploy, and the suites leave the
// collections path they stage. Narrowing TMPDIR to one directory and removing it afterwards is
// what keeps a run from accumulating all of that where a developer has to notice it. Every child
// inherits it through os.Environ().
//
// Naming runTempRoot rather than letting os.MkdirTemp pick is the part that is not obvious, and it
// is the whole reason this is a function rather than three lines in each TestMain. MkdirTemp("")
// consults TMPDIR, so the run directory would land inside build/tmp -- 30-odd bytes deeper than
// the path it is narrowing, and the budget is already nearly spent. ansible-core runs its task
// queue through a multiprocessing manager whose listener socket lives under TMPDIR as
// pymp-xxxxxxxx/listener-xxxxxxxx, and Linux caps an AF_UNIX path at 107 bytes. On a GitHub runner
// the repository path alone is 65 of them, so nested, every ansible-playbook in the non-cluster
// suite failed with "Local RPC server did not start" -- while passing on every developer's laptop,
// where the checkout sits somewhere shorter.
func RunTempDir(prefix string) (string, error) {
	dir, err := os.MkdirTemp(runTempRoot, prefix)
	if err != nil {
		return "", err
	}

	if err := os.Setenv("TMPDIR", dir); err != nil {
		return "", err
	}

	return dir, nil
}

// KubeClient builds a Kubernetes clientset from the kubeconfig at $KUBECONFIG.
func KubeClient(t *testing.T) (*kubernetes.Clientset, error) {
	t.Helper()
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		return nil, fmt.Errorf("KUBECONFIG is not set")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kube config: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}

// WaitForNodesReady polls the cluster until expectedCount nodes report Ready, or timeout elapses.
func WaitForNodesReady(ctx context.Context, cs *kubernetes.Clientset, expectedCount int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err == nil {
			ready := 0
			for _, n := range nodes.Items {
				if nodeIsReady(n) {
					ready++
				}
			}
			if ready >= expectedCount {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %d ready nodes: %w", expectedCount, ctx.Err())
		case <-ticker.C:
		}
	}
}

func nodeIsReady(n corev1.Node) bool {
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}
