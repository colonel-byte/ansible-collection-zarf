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
// root, and waiting on a cluster.
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
