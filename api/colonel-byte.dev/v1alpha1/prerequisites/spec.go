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

import "github.com/invopop/jsonschema"

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
	// FluxCD requires Flux, installed and new enough. Mutually exclusive with ArgoCD.
	FluxCD *EngineRequirement `json:"fluxcd,omitempty"`
	// ArgoCD requires Argo CD, installed and new enough. Mutually exclusive with FluxCD.
	ArgoCD *EngineRequirement `json:"argocd,omitempty"`
	// Controllers are the workloads that must be present and available.
	Controllers []ControllerRequirement `json:"controllers,omitempty"`
	// CRDs are the custom resource definitions that must be established on the cluster.
	CRDs []string `json:"crds,omitempty" jsonschema:"example=helmreleases.helm.toolkit.fluxcd.io,example=applications.argoproj.io"`
	// Storage requires somewhere for a PersistentVolumeClaim to land.
	Storage *StorageRequirement `json:"storage,omitempty"`
	// Packages requires other Zarf packages to be deployed first.
	Packages []PackageRequirement `json:"packages,omitempty"`
}

// JSONSchemaExtend refuses a document that requires both engines at once.
//
// Expressed in the schema rather than only in the role, so that the editor writing the document
// says so while it is being written. The role refuses it too -- a document can be handed to the
// role directly, through the zarf_prerequisites variable, without ever meeting a validator -- but
// by then the author has gone.
//
// `not: {required: [fluxcd, argocd]}` reads as "it must not be the case that both are present",
// which is the whole of the rule: either engine alone is fine, and neither is fine for a package
// that needs no GitOps engine at all.
//
// It is written here rather than in magefiles/pkg/gen/schema because the constraint belongs to
// the type. A rule the generator applied from the outside would hold for the schema and not for
// anything else reflecting over these types.
func (Requires) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.Not = &jsonschema.Schema{
		Required:    []string{EngineFluxCD, EngineArgoCD},
		Description: "a package is reconciled by one GitOps engine, so requiring both is refused",
	}
}

// KubernetesRequirement constrains the cluster's own version.
type KubernetesRequirement struct {
	// Version is a SemVer range the API server's version must satisfy.
	//
	// The examples are single comparators on purpose. A compound range like ">=1.30.0 <1.34.0"
	// is a legal value of this field and nothing here rejects one, but the only consumer that
	// exists -- the colonel_byte.zarf.prerequisites role -- reads one comparator and one version,
	// and records anything else as unevaluated rather than satisfied. Advertising a form that
	// reaches that consumer as a constraint it has to skip is advertising a check that does not
	// happen; see tasks/version_check.yml for why Jinja is not a semver library. The same applies
	// to EngineRequirement.Version and PackageRequirement.Version, which are the same comparison
	// over different data.
	Version string `json:"version" jsonschema:"example=>=1.30.0,example=>=1.31.0"`
}

// Engine names a GitOps engine this format knows how to check a cluster for, which is also the
// key a document requires it under: `requires.fluxcd` or `requires.argocd`.
//
// They are constants because the checking differs per engine in exactly two ways -- which
// namespace a controller defaults to, and which workloads carry the version label -- and a
// consumer handed an engine it has never heard of should say so rather than guess at both.
const (
	// EngineFluxCD is Flux: https://fluxcd.io.
	EngineFluxCD = "fluxcd"
	// EngineArgoCD is Argo CD: https://argo-cd.readthedocs.io.
	EngineArgoCD = "argocd"
)

// EngineRequirement requires a GitOps engine that is installed and new enough.
//
// The engine is the key this sits under rather than a field inside it, and that is the whole
// design. The two engines do not number together -- Flux is on 2.x while Argo CD is on 3.x -- so
// a version and a separate engine field could disagree, and the dangerous direction of that
// disagreement is silent: a document meaning "Flux >= 2.9" that named argocd by mistake is
// satisfied by Argo CD 3.5, because 3.5 >= 2.9. The constraint passes against an engine it was
// never about, which is the silent success this whole format exists to prevent, reached through
// the format itself. Naming the engine with the key leaves nothing for the version to contradict.
//
// Requiring both engines at once is refused rather than checked. A package is reconciled by one
// of them, and a document asking for two is a document that was edited rather than written.
//
// It is one small section rather than a per-engine copy of everything, because almost nothing
// about checking an engine is engine-specific. "These workloads must be available" and "these
// CRDs must be established" are the same question whatever reconciles the cluster, which is why
// Requires.Controllers and Requires.CRDs sit beside this rather than inside it.
//
// The version alone is the least interesting of the three checks a document can make. A cluster
// can hold the right version with its controllers scaled to zero, or hold the controllers without
// the CRDs the package's own Application or HelmRelease needs -- which is the silent case this
// whole format exists for: zarf deploys the manifests, nothing reconciles them, and the
// deployment is reported as a success.
type EngineRequirement struct {
	// Version is a SemVer range the installed engine's version must satisfy, in that engine's own
	// numbering: a flux2 release for fluxcd, an Argo CD release for argocd.
	//
	// It is read from the app.kubernetes.io/version label on the engine's own workloads, and
	// neither Flux's nor Argo CD's upstream manifests carry that label -- a package that vendors
	// them has to add it. A cluster whose engine carries no version label has the constraint
	// reported as unchecked rather than satisfied.
	Version string `json:"version,omitempty" jsonschema:"example=>=2.9.0,example=>=3.5.0"`
}

// ControllerRequirement is one workload that must be present and available.
type ControllerRequirement struct {
	// Name is the workload's name.
	Name string `json:"name" jsonschema:"example=source-controller,example=argocd-application-controller"`
	// Namespace is the namespace it is deployed into. A requirement that names none is checked in
	// the engine's own default namespace: flux-system for fluxcd, argocd for argocd.
	Namespace string `json:"namespace,omitempty" jsonschema:"example=flux-system,example=argocd"`
	// Kind is the workload's kind, Deployment when a requirement names none.
	//
	// Two values, because those are the two a GitOps engine's components are: Flux's controllers
	// are all Deployments, and Argo CD's argocd-application-controller is a StatefulSet while the
	// rest of it is Deployments. A format that assumed Deployment would report Argo CD's single
	// most important component as absent on a healthy cluster.
	Kind string `json:"kind,omitempty" jsonschema:"enum=Deployment,enum=StatefulSet,default=Deployment"`
	// Version is a SemVer range the workload's own version must satisfy, read from the same
	// app.kubernetes.io/version label the engine sections are read from.
	//
	// It is here so that a version can be required of something that is not a GitOps engine. A
	// package needing cert-manager v1.14, or an ingress controller new enough for an API it uses,
	// has no engine section to put that in -- and requiring an engine it does not use, purely to
	// get a version field, would be a prerequisite that refuses clusters for no reason.
	//
	// Unconstrained when empty, which is the common case: most controller requirements are about
	// a workload being there and available at all.
	Version string `json:"version,omitempty" jsonschema:"example=>=1.14.0"`
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
