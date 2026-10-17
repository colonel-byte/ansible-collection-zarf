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

// Package prerequisites is the colonel-byte.dev/v1alpha1 PackagePrerequisites API: the
// machine-readable contract a Zarf package states its cluster requirements in.
//
// The document is authored by a package rather than by this repository. It is written to
// `docs/prerequisites.yaml` inside the package's own source tree and referenced from `zarf.yaml`
// under zarf's top-level `documentation:` mapping, which makes it a layer of the built package
// that `zarf package inspect documentation` extracts. Nothing here reads a cluster or a package:
// these are the types the document is, and the only thing this repository does with them is
// generate the JSON Schema that editors and CI validate the document against. See
// magefiles/pkg/gen/schema and docs/guides/package-prerequisites.md.
//
// The types carry `json` tags rather than `yaml` ones, which is what both consumers need: the
// schema generator reflects over the json names, and a YAML document decoded through
// sigs.k8s.io/yaml or goccy/go-yaml's JSON compatibility reads the same names. The `jsonschema`
// tags are read only by the generator; the pattern is the one
// github.com/colonel-byte/cargoship/types uses for the cargoship config schema.
//
// It is versioned v1alpha1 and named for a domain this repository does not serve, the way a
// Kubernetes API group is: the name is a namespace for the kind, not a URL anybody fetches.
package prerequisites

// APIVersion and Kind are the two fixed fields of the document, and the pair a consumer checks
// before trusting the rest of it. They are constants rather than documentation because the Ansible
// side checks them by name: a file that parses as YAML but carries some other kind is not a
// prerequisites document, and reporting that is more use than reporting an absent `requires` key.
const (
	// APIVersion is the only value the apiVersion field may hold.
	APIVersion = "colonel-byte.dev/v1alpha1"
	// Kind is the only value the kind field may hold.
	Kind = "PackagePrerequisites"
)

// Document is a whole prerequisites document: the file a package ships as its `prerequisites`
// documentation key.
//
// `requires` is required and `provides` is not, which is the asymmetry the format is for. A
// package that states nothing it requires has no reason to ship this document at all, whereas a
// leaf workload that provides nothing to anything downstream is the common case.
type Document struct {
	// APIVersion is fixed to colonel-byte.dev/v1alpha1.
	APIVersion string `json:"apiVersion" jsonschema:"enum=colonel-byte.dev/v1alpha1"`
	// Kind is fixed to PackagePrerequisites.
	Kind string `json:"kind" jsonschema:"enum=PackagePrerequisites"`
	// Metadata names the package the document belongs to.
	Metadata Metadata `json:"metadata"`
	// Requires is what the cluster must already be, for this package to deploy onto it.
	Requires Requires `json:"requires"`
	// Provides is what the cluster becomes once it has, for the packages that require it.
	Provides *Provides `json:"provides,omitempty"`
}

// Metadata names the package the document belongs to.
//
// The package name and version are repeated here rather than read from the `zarf.yaml` beside it,
// because the two travel separately: `zarf package inspect documentation` extracts this file on
// its own, and a document that does not say which package it describes cannot be checked against
// the one that was inspected. A consumer that has both compares them and says so when they
// disagree.
type Metadata struct {
	// Package is the zarf package's metadata.name, as written in its zarf.yaml.
	Package string `json:"package" jsonschema:"example=operator-external-secrets,example=app-fluxcd"`
	// Version is the zarf package's metadata.version.
	Version string `json:"version" jsonschema:"example=2.21.0"`
	// Description is a sentence saying what the package needs and why, for a human reading the
	// file rather than validating against it.
	Description string `json:"description,omitempty"`
}

// Requires is the environmental prerequisites: what has to be true of the cluster before the
// package is deployed onto it.
//
// Every section is optional, and a document with an empty `requires` is valid but pointless. They
// are separate sections rather than one list of assertions because each is checked a different
// way -- a version range against an API server, a name against a list of CRDs, a readiness count
// against a Deployment -- and a consumer that cannot check one of them should be able to say which
// one it skipped.
type Requires struct {
	// Kubernetes constrains the cluster's own version.
	Kubernetes *KubernetesRequirement `json:"kubernetes,omitempty"`
	// FluxCD requires a GitOps engine that is installed, running, and new enough.
	FluxCD *FluxCDRequirement `json:"fluxcd,omitempty"`
	// Storage requires somewhere for a PersistentVolumeClaim to land.
	Storage *StorageRequirement `json:"storage,omitempty"`
	// Packages requires other Zarf packages to be deployed first.
	Packages []PackageRequirement `json:"packages,omitempty"`
}

// KubernetesRequirement constrains the cluster's own version.
type KubernetesRequirement struct {
	// Version is a SemVer range the API server's version must satisfy.
	Version string `json:"version" jsonschema:"example=>=1.30.0,example=>=1.30.0 <1.34.0"`
}

// FluxCDRequirement requires a GitOps engine that is installed, running, and new enough.
//
// Three things rather than one, because each catches a different failure, and the version alone
// catches the least interesting of them. A cluster can hold the right Flux version with its
// controllers scaled to zero, or hold the controllers without the CRDs the package's own
// HelmRelease needs -- which is the silent case this whole format exists for: zarf deploys the
// manifests, nothing reconciles them, and the deployment is reported as a success.
type FluxCDRequirement struct {
	// Version is a SemVer range the installed Flux version must satisfy.
	Version string `json:"version,omitempty" jsonschema:"example=>=2.9.0"`
	// Controllers are the controller Deployments that must be present and available.
	Controllers []ControllerRequirement `json:"controllers,omitempty"`
	// CRDs are the custom resource definitions that must be established on the cluster.
	CRDs []string `json:"crds,omitempty" jsonschema:"example=helmreleases.helm.toolkit.fluxcd.io,example=ocirepositories.source.toolkit.fluxcd.io"`
}

// ControllerRequirement is one controller Deployment that must be present and available.
type ControllerRequirement struct {
	// Name is the Deployment's name.
	Name string `json:"name" jsonschema:"example=source-controller,example=helm-controller"`
	// Namespace is the namespace it is deployed into.
	Namespace string `json:"namespace,omitempty" jsonschema:"default=flux-system"`
	// MinAvailable is how many replicas must report available. One is enough to reconcile; a
	// higher number is a statement about the package's tolerance for a controller restart during
	// its own deployment.
	MinAvailable int `json:"minAvailable,omitempty" jsonschema:"default=1,minimum=1"`
}

// StorageRequirement requires somewhere for a PersistentVolumeClaim to land.
//
// `defaultClass` is separate from `classes` because the two ask different questions. A chart whose
// values name no storageClassName binds to whatever the cluster's default is, so it needs a
// default to exist at all; a chart pinned to a class by name needs that class, and does not care
// what the default is. A package that templates neither needs no storage section.
type StorageRequirement struct {
	// DefaultClass requires the cluster to have a StorageClass marked as its default.
	DefaultClass bool `json:"defaultClass,omitempty"`
	// Classes are StorageClasses that must exist by name.
	Classes []string `json:"classes,omitempty" jsonschema:"example=local-path,example=rook-ceph-block"`
	// AccessModes are the PersistentVolume access modes the package's claims ask for. A
	// single-node provider that offers only ReadWriteOnce is a prerequisite failure for a package
	// whose claims are ReadWriteMany, and it is one that surfaces as a Pending Pod rather than as
	// a deployment error.
	AccessModes []string `json:"accessModes,omitempty" jsonschema:"enum=ReadWriteOnce,enum=ReadOnlyMany,enum=ReadWriteMany,enum=ReadWriteOncePod"`
}

// PackageRequirement is one other Zarf package that must be deployed first.
//
// The name is the deployed package's name as `zarf package list` reports it, which is what makes
// this checkable with colonel_byte.zarf.zarf_package_info and no cluster-specific knowledge.
type PackageRequirement struct {
	// Name is the prerequisite package's metadata.name.
	Name string `json:"name" jsonschema:"example=app-fluxcd,example=cluster-init"`
	// Version is a SemVer range the deployed package's version must satisfy.
	Version string `json:"version,omitempty" jsonschema:"example=>=2.9.0"`
}

// Provides is what the cluster becomes once the package is deployed: the half of the contract the
// packages downstream of this one require.
//
// It is declared rather than discovered so that an orchestrator can order a bundle without
// deploying any of it. A planner that reads every package's document can satisfy each `requires`
// against another package's `provides` and sort the bundle; one that can only read a live cluster
// has to deploy to find out.
type Provides struct {
	// Capabilities are named, versioned capabilities other packages may require. The name is a
	// label this format owns rather than anything Kubernetes knows about, which is what lets two
	// different packages provide the same capability -- `fluxcd-engine` from an upstream chart or
	// from a vendor's bundle.
	Capabilities []Capability `json:"capabilities,omitempty"`
	// CRDs are the custom resource definitions the package establishes.
	CRDs []string `json:"crds,omitempty"`
	// Namespaces are the namespaces the package creates.
	Namespaces []string `json:"namespaces,omitempty"`
}

// Capability is one named, versioned capability the package contributes to the cluster.
type Capability struct {
	// Name is the capability's name.
	Name string `json:"name" jsonschema:"example=fluxcd-engine"`
	// Version is the capability's version, as a concrete version rather than a range: this is a
	// statement about what was installed, not a constraint on it.
	Version string `json:"version,omitempty" jsonschema:"example=2.9.3"`
}
