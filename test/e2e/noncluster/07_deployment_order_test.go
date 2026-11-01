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

package noncluster

import "testing"

const (
	deploymentOrderPlaybook = "test/e2e/noncluster/testdata/deployment-order.yml"
	deploymentOrderFilter   = "ansible/colonel_byte/zarf/plugins/filter/deployment_order.py"
)

// TestDeploymentOrderPlansTheFixtureBundle drives colonel_byte.zarf.deployment_order over the
// fixture package pool, once per rule the planner implements.
//
// It needs nothing installed but ansible-playbook. The filter takes parsed documents rather than
// archives, so the playbook reads testdata/bundle-fixture/*/docs/prerequisites.yaml directly and
// no package is built, no cluster is reached and no zarf is run. That is the whole reason the
// planner is a filter: the ordering is arithmetic over documents that have already been extracted,
// and arithmetic can be tested without the extraction.
//
// Half the cases are refusals, and they are the half worth having. A planner that picks a provider
// out of an ambiguous group, or returns an empty plan for a cycle, fails in the direction nothing
// downstream can detect -- deploy_packages would be handed a plan that looks ordered and is wrong,
// and the first sign of it would be a workload that never starts.
func TestDeploymentOrderPlansTheFixtureBundle(t *testing.T) {
	env := zarfPlaybookEnv(t, []string{deploymentOrderFilter})
	runZarfPlaybook(t, env, deploymentOrderPlaybook, nil, nil)
}
