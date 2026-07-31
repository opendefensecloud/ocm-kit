# RFC01 OCM Component Contracts

- **Status:** Draft
- **Date:** 2026-06-02
- **Authors:** Niklas Voss (with brainstorming assistance from Claude)
- **Related:** existing `helmvalues` package and label convention

## 1. Problem statement

An OCM component bundles deployable artifacts — Helm charts, OCI images, manifests — but is silent about two questions that operators and tooling repeatedly need to answer:

1. **What does this package need from its target environment to work?**
   Examples: a Kubernetes version range, total fleet capacity, GPU-bearing nodes, the presence of specific CRDs (e.g. `cert-manager.io/Certificate`), the existence of an in-cluster capability (e.g. an OIDC provider, a configured `Issuer`).
2. **What does this package contribute back to the environment once installed?**
   Examples: the CRDs it installs, the in-cluster services/capabilities it makes available for other packages to depend on.

Today this information lives in READMEs, prose docs, and tribal knowledge. Operators rediscover it by trial-installing, scraping rendered manifests, or asking the package author. There is also no machine-readable way for two OCM packages to declare a dependency on each other: a workload that needs `cert-manager.io/Certificate` cannot say so in a form that an installer or a CI gate can act on.

The goal of this RFC is to add a typed, declarative component contract resource to OCM components, and to extend `ocm-kit` so it can parse those declarations, verify them against a live cluster, and resolve them against other components' provisions.

## 2. Goals and non-goals

### Goals

- A typed YAML schema for declaring **requires** and **provides** on an OCM component.
- A label-based discovery convention that mirrors the existing helm-values resource (`opendefense.cloud/helm/values-for`).
- A Go API in `ocm-kit` for parsing, validating, and verifying declarations.
- CLI subcommands for human inspection, live-cluster verification, and metadata resolution against other components.
- Hybrid verification: a requirement is satisfied if **either** a live cluster probe succeeds **or** another component's `provides` claims the capability.
- Honest distinction between three outcomes: "requirement is unmet," "probe failed so we don't know," and "no probe could evaluate this requirement" (e.g. a catalog-only run asked about `kubernetes.version`).

### Non-goals (of initial version)

- **Aggregation across OCM component references.** v1 treats each ComponentVersion independently; component-reference aggregation is deferred to future versions.
- **CRD-instance provides** (e.g. "I install a `ClusterIssuer` named `letsencrypt-prod`"). Modelling cluster state, not declarations.
- **Soft/optional requirements.** Today every requirement is hard. A `severity` field is a natural future extension.
- **Authoring helpers** that scan a chart and propose a starter declaration.
- **Live-cluster e2e tests in CI.** The cluster probe is covered by fake-client unit tests; running a real kind/k3d cluster in CI is out of scope.

## 3. Use cases

1. **Pre-install gating in automation.**
   Automation runs `ocm-kit contract verify <ref> --kubeconfig ...` (or code equivalent) against cluster and refuses to deploy if the package's requirements aren't met.
2. **Catalog dependency check.**
   A platform maintains a curated catalog of OCM packages. The **requires** and **provides** can be discovered and used for pre-flight checks and dependency resolution.
3. **Human discovery.**
   `ocm-kit contract show <ref>` prints the typed declaration so an operator can read at a glance "this package needs cert-manager and a GPU node; it provides `foo.opendefense.cloud/Bar`."
4. **UI / portal display.**
   A downstream UI (SolAr) consumes the parsed Go struct (or JSON via the CLI's `--format json`) and renders a "this package needs / provides" panel in a self-service portal.

## 4. Architecture

```
                    +-----------------------------------+
                    |  OCM ComponentVersion             |
                    |                                   |
                    |  resources:                       |
                    |   - helm-values-template          |
                    |   - component-contract.yaml <- new|
                    |   - chart, images, ...            |
                    +-----------------+-----------------+
                                      |
                                      v (label-discovered)
        +-----------------------------------------------------------+
        |  ocm-kit / contract package                               |
        |                                                           |
        |   GetContract ----> typed Go struct (v1 schema)           |
        |   Verify(opts) ----> Result {satisfied,missing,errors}    |
        |     |                                                     |
        |     +-- ClusterProbe (kubeconfig: api-discovery, nodes)   |
        |     +-- CatalogProbe (other components' provides)         |
        +-----------------------------------------------------------+
                |                                  |
                v                                  v
        ocm-kit CLI subcommands             external installers
        (show / verify / resolve)           (consume library)
```

**Discovery.** Same pattern as the existing helm-values resource: a label on the OCM resource (`opendefense.cloud/component-contract`) marks the YAML blob. The label value carries the schema version (e.g. `v1`).

**Hybrid resolution.** Probes are tried in supplied order; the first to return `satisfied=true` wins, and its identity is recorded as the satisfaction source. Callers can pass a `ClusterProbe`, a `CatalogProbe`, both, or a custom `Probe` implementation.

**Scope of "package".** initial version treats one ComponentVersion = one declaration. Aggregation across component references is deferred to future version.

## 5. Schema (v1)

The resource is a single YAML document with typed top-level sections. Each section has built-in verification semantics in `ocm-kit`.

```yaml
apiVersion: ocm.opendefense.cloud/v1
kind: ComponentContract

requires:
  kubernetes:
    # semver constraint against kube-apiserver's reported version (/version).
    version: ">=1.27.0, <1.31.0"

  cluster:
    # Aggregate capacity across schedulable nodes. ocm-kit sums
    # `.status.allocatable` across nodes matching the optional `labels`
    # selector and compares against these thresholds.
    cpu: "32"
    memory: "128Gi"
    devices:
      - resource: nvidia.com/gpu
        count: 4
    labels: {}            # optional: restrict aggregation to matching nodes

  nodes:
    # Per-node scheduling constraint. Each entry asserts that at least
    # `count` nodes match ALL listed constraints (labels AND cpu AND
    # memory AND devices). Default count: 1.
    - count: 1
      cpu: "4"
      memory: "16Gi"
      devices:
        - resource: nvidia.com/gpu
          count: 1
      labels:
        node-role.kubernetes.io/worker: ""

  apis:
    # CRDs / GVKs that must be served by the cluster (api-discovery) OR
    # claimed under `provides.apis` by another component.
    - group: cert-manager.io
      kind: Certificate
      versions: ["v1"]    # any listed version present satisfies the requirement
    - group: cert-manager.io
      kind: Issuer
      versions: ["v1"]

  features:
    # An abstract capability identifier another package can claim under
    # `provides.features`. Catalog-probe only (not verifiable against a
    # live cluster generically).
    - capability: cert-manager.io/issuer
      hint: "An Issuer or ClusterIssuer must be configured for the cluster."

provides:
  apis:
    - group: foo.opendefense.cloud
      kind: Bar
      versions: ["v1alpha1"]

  features:
    - capability: opendefense.cloud/fizzbuzz
      # informational endpoint descriptors (not verified, just metadata)
      endpoints:
        - name: api
          port: 443
          protocol: HTTPS
```

### 5.1 Section semantics

| Section | Question it answers | Verifier strategy |
|---|---|---|
| `kubernetes.version` | Is the control plane in the allowed version range? | `GET /version` |
| `cluster` | Does the fleet have enough total capacity? | `List Nodes` → filter by `labels` → sum `.status.allocatable` |
| `nodes` | Are there N concrete nodes a workload can land on? | `List Nodes` → for each entry, count matches |
| `apis` | Is GVK X served? | Discovery client; or peer `provides.apis` |
| `features` | Is opaque capability Y available? | Peer `provides.features` only |

### 5.2 Why both `cluster` and `nodes` (not just one)

- A 4-CPU pod won't schedule on a cluster with 4 × 1-CPU nodes even though aggregate CPU is 4. `cluster` alone would falsely report "satisfied."
- A horizontally scaled stateless workload (e.g. 10 × 2-CPU pods) wants total capacity, not a single fat node. `nodes: [{cpu: 20}]` would falsely demand a giant node.
- Producers declare whichever (or both) match their workload's real constraints.

### 5.3 Schema validation rules

- `apiVersion` must equal `ocm.opendefense.cloud/v1`. Other values produce `ErrUnsupportedAPIVersion`.
- `kind` must equal `ComponentContract`.
- YAML is parsed in **strict mode** (`yaml.UnmarshalStrict`); unknown fields fail the parse with a clear path-prefixed error. This catches typos that would otherwise silently drop a requirement.
- Format-level validation runs immediately after unmarshal:
  - semver constraints parse with `github.com/Masterminds/semver/v3`
  - K8s quantity strings parse with `k8s.io/apimachinery/pkg/api/resource`
  - GVK fields (`group`, `kind`) are non-empty; `versions` is non-empty
  - `capability` strings on features are non-empty

## 6. Worked examples

### 6.1 `cert-manager` (pure provider)

```yaml
apiVersion: ocm.opendefense.cloud/v1
kind: ComponentContract
requires:
  kubernetes:
    version: ">=1.25"
provides:
  apis:
    - group: cert-manager.io
      kind: Certificate
      versions: ["v1"]
    - group: cert-manager.io
      kind: Issuer
      versions: ["v1"]
    - group: cert-manager.io
      kind: ClusterIssuer
      versions: ["v1"]
  features:
    - capability: cert-manager.io/issuer
      hint: "Once installed, configure an Issuer or ClusterIssuer for your CA."
```

Allows any downstream package's `requires.apis: cert-manager.io/Certificate` to be satisfied by the catalog probe — no cluster contact required.

### 6.2 GPU-bound ML inference

```yaml
apiVersion: ocm.opendefense.cloud/v1
kind: ComponentContract
requires:
  kubernetes:
    version: ">=1.28"
  cluster:
    memory: "256Gi"
    devices:
      - resource: nvidia.com/gpu
        count: 4
  nodes:
    - count: 1
      cpu: "8"
      memory: "64Gi"
      devices:
        - resource: nvidia.com/gpu
          count: 1
  features:
    - capability: nvidia.com/device-plugin
      hint: "The NVIDIA device plugin must be installed and reporting nvidia.com/gpu."
provides:
  apis:
    - group: inference.example.com
      kind: ModelDeployment
      versions: ["v1alpha1"]
```

### 6.3 ARC (the project's existing example)

```yaml
apiVersion: ocm.opendefense.cloud/v1
kind: ComponentContract
requires:
  kubernetes:
    version: ">=1.27,<1.31"
  apis:
    - group: cert-manager.io
      kind: Certificate
      versions: ["v1"]
    - group: cert-manager.io
      kind: Issuer
      versions: ["v1"]
provides:
  apis:
    - group: arc.opendefense.cloud
      kind: Order
      versions: ["v1alpha1"]
  features:
    - capability: opendefense.cloud/arc
      endpoints:
        - name: api
          port: 443
          protocol: HTTPS
```

In a catalog that contains both `cert-manager` and `arc`, `ocm-kit contract resolve arc --against cert-manager` reports all `requires.apis` satisfied by `component:cert-manager`. The `kubernetes.version` requirement falls into `Unverified` without a `--kubeconfig` — the catalog probe returns `ErrProbeNotApplicable` for it, and no cluster probe was supplied. This is distinct from `Missing` (which would assert the cluster's version is out of range).

## 7. Go API

### 7.1 Package layout

A new package `contract/` next to the existing `helmvalues/`. The CLI gains a `contract` subcommand group.

```
ocm-kit/
  contract/
    contract.go            # types + discovery + parse
    verify.go              # ClusterProbe + CatalogProbe + Verify orchestration
    schema.go              # v1 schema struct + apiVersion gate
    contract_test.go
  cmd/ocm-kit/
    main.go                # adds `contract` subcommand
```

### 7.2 Discovery and parse

Mirrors the existing `helmvalues` API for consistency.

```go
package contract

const (
    LabelName  = "opendefense.cloud/component-contract"
    MediaType  = "application/vnd.opendefense.component-contract.v1+yaml"
    APIVersion = "ocm.opendefense.cloud/v1"
    Kind       = "ComponentContract"
)

var (
    ErrNotFound              = errors.New("component contract resource not found")
    ErrUnsupportedAPIVersion = errors.New("unsupported component contract apiVersion")
    // Probes return ErrProbeNotApplicable when asked about a Requirement kind
    // they don't handle (e.g. a CatalogProbe asked about kubernetes.version).
    // The verifier uses this to distinguish "no probe could evaluate this"
    // from "probe ran and the requirement is unmet."
    ErrProbeNotApplicable = errors.New("probe does not evaluate this requirement kind")
)

type ComponentContract struct {
    APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
    Kind       string   `yaml:"kind"       json:"kind"`
    Requires   Requires `yaml:"requires,omitempty" json:"requires,omitempty"`
    Provides   Provides `yaml:"provides,omitempty" json:"provides,omitempty"`
}

type Requires struct {
    Kubernetes *KubernetesRequirement `yaml:"kubernetes,omitempty"`
    Cluster    *ClusterRequirement    `yaml:"cluster,omitempty"`
    Nodes      []NodeRequirement      `yaml:"nodes,omitempty"`
    APIs       []APIRequirement       `yaml:"apis,omitempty"`
    Features   []FeatureRequirement   `yaml:"features,omitempty"`
}

type Provides struct {
    APIs     []APIDeclaration     `yaml:"apis,omitempty"`
    Features []FeatureDeclaration `yaml:"features,omitempty"`
}

// (KubernetesRequirement, ClusterRequirement, NodeRequirement,
//  APIRequirement, FeatureRequirement, APIDeclaration,
//  FeatureDeclaration — typed structs matching the schema in §5.)

func FindContract(compVer ocm.ComponentVersionAccess) (ocm.ResourceAccess, error)
func FetchContract(res ocm.ResourceAccess) (*ComponentContract, error)
func GetContract(compVer ocm.ComponentVersionAccess) (*ComponentContract, error)
```

### 7.3 Verification

```go
// Probe attempts to satisfy a single Requirement. Returns:
//   - satisfied=true  + source identifier (e.g., "cluster", "component:<ref>")
//   - satisfied=false + nil error if the probe ran and the requirement is unmet
//   - satisfied=false + ErrProbeNotApplicable if the probe does not evaluate
//     this requirement kind (e.g. CatalogProbe asked about kubernetes.version)
//   - any other non-nil error if the probe itself failed (network, auth, etc.)
type Probe interface {
    Satisfy(ctx context.Context, req Requirement) (satisfied bool, source string, err error)
}

type RequirementKind int

const (
    KubernetesVersion RequirementKind = iota
    ClusterCapacity
    Node
    API
    Feature
)

type Requirement struct {
    Kind   RequirementKind
    Detail any // typed sub-struct (KubernetesRequirement, ClusterRequirement, ...)
}

type SatisfiedRequirement struct {
    Requirement Requirement
    Source      string // e.g. "cluster", "component:ghcr.io/.../cert-manager:1.2.3"
}

type ProbeError struct {
    Requirement Requirement
    Err         error
}

type Result struct {
    Satisfied  []SatisfiedRequirement
    Missing    []Requirement
    Unverified []Requirement // no probe could evaluate this requirement
    Errors     []ProbeError
}

// Verify iterates every requirement, asks each probe in order, and
// reports the result. First probe returning satisfied=true wins.
func Verify(ctx context.Context, c *ComponentContract, probes ...Probe) (*Result, error)

// NewClusterProbe verifies kubernetes.version, cluster, nodes, and apis
// against a live cluster via client-go. For features it returns
// ErrProbeNotApplicable.
func NewClusterProbe(restConfig *rest.Config) Probe

// NewCatalogProbe satisfies apis and features from peer ComponentContract docs.
// For kubernetes/cluster/nodes requirements it returns ErrProbeNotApplicable.
func NewCatalogProbe(providers []NamedContract) Probe

type NamedContract struct {
    Ref      string // human-readable source identifier
    Contract *ComponentContract
}
```

### 7.4 Error classes

The verifier surfaces four distinct outcomes for each requirement; mixing them is the most common UX failure of similar tools.

| Class | Meaning | Exit code | Result field |
|---|---|---|---|
| **Unsatisfied** | Probe ran successfully, requirement is genuinely unmet | `1` | `Missing` |
| **Probe error** | Probe failed; the requirement's actual state is unknown | `2` | `Errors` |
| **Unverified** | No supplied probe handles this requirement kind (e.g. no `--kubeconfig` for a `kubernetes.version` check) | `3` | `Unverified` |
| **Usage error** | Caller passed bad input | `64` | (returned from `Verify`) |

A CI gate can opt into stricter behavior with `--fail-on-probe-error` and/or `--fail-on-unverified`; the default exit codes (2 for probe error, 3 for unverified) are honest about which kind of uncertainty occurred.

### 7.5 Probe ordering and short-circuiting

Probes are tried in supplied order. First `satisfied=true` wins. Otherwise the requirement is classified by what the probes returned:

- If at least one probe ran (`satisfied=false`, nil error) and none satisfied, it is `Missing`.
- If any probe returned a non-`ErrProbeNotApplicable` error and none satisfied or ran cleanly, it is recorded in `Errors` — we don't know its true state.
- If every supplied probe returned `ErrProbeNotApplicable`, no probe could evaluate the requirement; it is recorded in `Unverified`. This is distinct from `Missing` (which asserts the requirement is genuinely unmet).

## 8. CLI

Three new subcommands under `ocm-kit contract`.

```bash
# Print the parsed declaration (formatted YAML / JSON via flag).
ocm-kit contract show <component-ref> [--format yaml|json]

# Verify against a live cluster, a catalog of other components, or both.
# At least one of --kubeconfig or --against must be supplied.
ocm-kit contract verify <component-ref> \
  [--kubeconfig PATH] \
  [--against COMP_REF[,COMP_REF...]] \
  [--fail-on-probe-error] \
  [--fail-on-unverified] \
  [--format text|json]

# Pure metadata resolution: which of <component-ref>'s requires are
# satisfied by --against, and which are not.
ocm-kit contract resolve <component-ref> \
  --against COMP_REF[,COMP_REF...] \
  [--format text|json]
```

### 8.1 Output

```
$ ocm-kit contract verify ./arc:0.2.0 \
    --kubeconfig ~/.kube/config \
    --against ./cert-manager:1.15.0

Requirements for ghcr.io/.../arc:0.2.0

  [OK]      kubernetes.version  >=1.27.0,<1.31.0    (cluster: v1.29.4)
  [OK]      apis  cert-manager.io/Certificate/v1    (component: cert-manager:1.15.0)
  [OK]      apis  cert-manager.io/Issuer/v1         (component: cert-manager:1.15.0)
  [MISSING] nodes count=2 cpu>=4 memory>=16Gi       (cluster: 1 matching, need 2)
  [MISSING] features capability=opendefense.cloud/oidc

2 of 5 unsatisfied. Exit 1.
```

### 8.2 CLI compatibility

Today `ocm-kit <ref>` directly renders helm values. Two options under consideration (see §11):

- Keep `ocm-kit <ref>` as an implicit alias for `ocm-kit helmvalues render <ref>` (preserves current users).
- Break compatibility now while the project is young and require `ocm-kit helmvalues render <ref>`.

Author recommendation: keep the alias.

## 9. Forward compatibility and versioning

- The schema reserves the right to add new top-level sections under `requires`/`provides` in minor versions. Strict-mode consumers must therefore upgrade to read newer documents.
- The discovery label value carries the schema version (`opendefense.cloud/component-contract: v1`); a consumer that only understands v1 can skip a v2 resource cleanly rather than failing on unknown fields.
- The CLI prints a clear "this tool supports v1; resource is v2 — upgrade ocm-kit" message on apiVersion mismatch.

## 10. Security considerations

- The component contract resource is **content, not code** — no template execution, no `exec`, no remote fetches at parse time. Unlike helm-values templates, it does not run user input through `text/template`.
- The cluster probe uses standard `client-go` with whatever permissions the supplied kubeconfig grants. The minimum needed: `get` on `/version`, `list` on `Node`, and discovery (`get` on `/apis`). Documentation will include an example RBAC `ClusterRole` for CI service accounts.
- The catalog probe is purely metadata-driven (it reads other components' `provides`); it does not "trust" them in any security sense — a malicious package claiming to provide a CRD doesn't affect cluster state. The trust boundary is whoever curates the catalog of components passed to `--against`, which is the same trust boundary as installing them.
- OCM component signatures (if present) sign all resources in the descriptor including this one, so the declaration inherits the existing signing story without extra work.

## 11. Open questions

1. **CLI compat.** Keep `ocm-kit <ref>` as an alias for `ocm-kit helmvalues render <ref>`, or break compatibility now while the project is young? *Author lean: keep the alias.*
2. **`provides.features` endpoints.** Should they be verified somehow (e.g., resolve into rendered K8s `Service` manifests in helm output), or stay purely documentation? *Verification adds a lot of complexity; documentation-only in v1 is the safer call.*
3. **Catalog source.** Should `ocm-kit` grow a "walk a registry namespace to collect provider contracts" helper, or is that a downstream concern? *Affects whether `--against` accepts a registry prefix in addition to explicit refs.*
4. **Per-node node semantics.** Currently each `nodes` entry asserts at least N nodes match ALL constraints in that entry. Should we offer an `any`/`all` switch across entries? *Author lean: keep per-entry simple; users can add multiple entries for multiple distinct node shapes.*
5. **`features.capability` identifier shape.** Currently an opaque URL-ish string. Should we move to a structured `{provider, name}` later? *v1 stays opaque to avoid locking the schema down before usage patterns emerge.*

## 12. Testing strategy

### 12.1 Unit tests (`contract_test.go`)

- **Parsing:** golden YAML inputs → expected struct; strict-mode rejects unknown fields; bad semver / bad k8s quantity / missing required fields each produce the expected typed error with field path.
- **`apiVersion` gate:** v1 parses; v2 returns `ErrUnsupportedAPIVersion`; missing `apiVersion` returns a clear error.
- **`CatalogProbe`:** built from a slice of `NamedContract`; assert exact match semantics (matching group + kind, version intersection).
- **`Verify` orchestration:** order-of-probes test (first-satisfier-wins), probe-error vs missing vs unverified distinction (including the all-probes-return-`ErrProbeNotApplicable` case), empty probes list returns usage error.

### 12.2 `ClusterProbe` against fake client

`client-go` ships `k8s.io/client-go/discovery/fake` and `k8s.io/client-go/kubernetes/fake` — sufficient for asserting the probe's behaviour without a real cluster. Cases:

- K8s version in/out of semver range
- Node listing with various label selectors and resource amounts (covers both `cluster` aggregate and `nodes` per-node logic)
- Discovery returning known and unknown GVKs

### 12.3 E2E

The existing `make e2e` provisions a local zot OCI registry and pushes a component. The new e2e test:

1. Builds a component with a `component-contract.yaml` resource labeled with `opendefense.cloud/component-contract: v1`.
2. Pushes to the local zot registry.
3. Runs `ocm-kit contract show <ref>` and asserts output.
4. Runs `ocm-kit contract resolve <ref> --against <provider-ref>` and asserts the resolution result.

A live-cluster e2e is out of scope for v1.

### 12.4 Authoring fixtures

The e2e component spec lives next to existing helm-values e2e fixtures and is built from a small YAML file via the OCM CLI, so producers reading the test directory get a copy-pasteable example of how to embed the resource.

## 13. Rollout

- **Additive only.** No existing code changes; the new package and subcommand can ship in a minor release.
- **Opt-in.** Components without a component contract continue to work. `GetContract` returns `ErrNotFound`; CLI subcommands report it cleanly and exit non-error.
- **Docs.** README gains a "Component Contract" section alongside the existing "Resource Labeling" guidance, with copy-pasteable starter templates for common archetypes (controller with CRDs, GPU workload, dependent service consumer).

## 14. Deferred to v2

- Aggregation across OCM component references (recurse through `references`, union their declarations).
- CRD-instance provides (not just CRD definitions).
- Soft/optional requirements (`severity: warn`).
- Authoring helper (scan chart → propose declaration).
- Typed sections for storage classes, ingress controllers, gateways. Likely expressible via `features` with well-known capability strings in v1; promote to typed sections if usage warrants.
