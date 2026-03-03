# ocm-kit

[![Build status](https://github.com/opendefensecloud/ocm-kit/actions/workflows/golang.yaml/badge.svg)](https://github.com/opendefensecloud/ocm-kit/actions/workflows/golang.yaml)
[![Coverage Status](https://coveralls.io/repos/github/opendefensecloud/ocm-kit/badge.svg?branch=main)](https://coveralls.io/github/opendefensecloud/ocm-kit?branch=main)
[![Go Report Card](https://goreportcard.com/badge/go.opendefense.cloud/ocm-kit)](https://goreportcard.com/report/go.opendefense.cloud/ocm-kit)
[![Go Reference](https://pkg.go.dev/badge/go.opendefense.cloud/ocm-kit.svg)](https://pkg.go.dev/go.opendefense.cloud/ocm-kit)
[![GitHub Release](https://img.shields.io/github/v/release/opendefensecloud/ocm-kit)
](https://github.com/opendefensecloud/ocm-kit/releases)


A Go library and CLI tool for working with Open Component Model (OCM) Helm values templates.

## Problem Statement

When an OCM ComponentVersion is transferred from one OCI Registry to another, the default values of Helm Charts will contain images of the old OCI Registry. This library provides functionality to manage and render Helm values templates that are embedded as resources in OCM components, ensuring that image references are correctly resolved regardless of the registry they're accessed from.

## Solution Overview

The library provides mechanisms to:

1. **Find Helm Values Templates**: Locate Helm values templates in OCM components using a label-based approach (`opendefense.cloud/helm/values-for`)
2. **Render Templates**: Process the templates using Go's text/template with sprig functions for flexible value substitution
3. **Extract Component Data**: Automatically extract resource information from OCM components and prepare it for templating

For a smoother development experience, a small `ocm-kit` CLI is also provided, which allows local testing of rendering helm values templates or verifying outputs.

## Installation & Building

### Prerequisites
- Go 1.25.7 or later
- Docker (for running e2e tests)
- OCM CLI (for e2e tests)

### Build
```bash
go build ./...
```

### Run Go tests

```bash
# Run all tests
make test
# Run particular tests
go test ./helmvalues
```

### Run e2e Tests
```bash
# Run e2e tests with default version (timestamp-based)
make e2e

# Run e2e tests with a stable component version
VERSION=0.1.0 make e2e

# Run e2e tests but keep zot registry running
make e2e-keep-zot

# Stop and remove zot registry
make e2e-stop-zot
```

## CLI Usage

The `ocm-kit` command-line tool renders Helm values templates from OCM components.

### Basic Usage
```bash
ocm-kit <component-version-ref> [flags]
```

Where `<component-version-ref>` is in the format: `protocol://host/namespace//component:version`

Examples:
```bash
# Render first values template from component in local OCI registry
ocm-kit "http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0"

# Render specific values template from remote registry by providing which helm resource it is for
ocm-kit "https://registry.example.com/stable//example.com/myapp:1.2.3" -r my-chart

# Use a local template file instead of component template
ocm-kit "http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" \
  --local-helm-values-template ./values.yaml.tpl
```

### Command Flags

- `-r, --chart-resource string` - Name of the Helm chart resource in the component (default: "")
- `-f, --local-helm-values-template string` - Path to a local Helm values template file (overrides component template)
- `-h, --help` - Display help message

### Example

#### Values Template

```yaml
apiserver:
  image:
    {{- $apiserver := index .OCIResources "arc-apiserver-image" }}
    repository: {{ $apiserver.Host }}/{{ $apiserver.Repository }}
    tag: {{ $apiserver.Tag }}

controller:
  image:
    {{- $controller := index .OCIResources "arc-controller-manager-image" }}
    repository: {{ $controller.Host }}/{{ $controller.Repository }}
    tag: {{ $controller.Tag }}

etcd:
  image:
    {{- $etcdImage := index .OCIResources "etcd-image" }}
    repository: {{ $etcdImage.Host }}/{{ $etcdImage.Repository }}
    tag: {{ $etcdImage.Tag }}
```

#### Render Output

```yaml
apiserver:
  image:
    repository: localhost:5000/my-components/opendefensecloud/arc-apiserver
    tag: v0.2.0

controller:
  image:
    repository: localhost:5000/my-components/opendefensecloud/arc-controller-manager
    tag: v0.2.0
```

## Library Usage Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "go.opendefense.cloud/ocm-kit/helmvalues"
    "go.opendefense.cloud/ocm-kit/compver"
    "ocm.software/ocm/api/ocm"
    "ocm.software/ocm/api/ocm/extensions/repositories/ocireg"
)

func main() {
    ctx := context.Background()
    
    // Parse component version reference
    cvr, err := compver.SplitRef("http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0")
    if err != nil {
        log.Fatal(err)
    }
    
    // Setup OCM repository
    octx := ocm.FromContext(ctx)
    repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec(cvr.BaseURL()))
    if err != nil {
        log.Fatal(err)
    }
    defer repo.Close()
    
    // Get component version
    compVer, err := repo.LookupComponentVersion(cvr.ComponentName, cvr.Version)
    if err != nil {
        log.Fatal(err)
    }
    defer compVer.Close()
    
    // Find helm values template for a specific chart
    tmpl, err := helmvalues.GetHelmValuesTemplate(compVer, "helm-chart")
    if err != nil {
        log.Fatal(err)
    }
    
    // Get rendering input with component data
    input, err := helmvalues.GetRenderingInput(compVer)
    if err != nil {
        log.Fatal(err)
    }
    
    // Render the template
    renderedValues, err := helmvalues.Render(tmpl, input)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println(renderedValues)
}
```

## Template Variables

When rendering templates, the following data is available via the context:

### `.OCIResources`
A map of all oci resources in the component by resource name. Only resources with an OCI-based access method are listed:

Each resource is automatically parsed into an object with:
- `.Host` - The registry host
- `.Repository` - The repository path
- `.Tag` - The image tag
- `.Digest` - The image digest

For other access methods (OCI blobs, local blobs, S3, Git, etc.), the relevant fields are extracted into structured maps.

Access example:
```yaml
{{- $image := index .OCIresources "my-image" }}
repository: {{ $image.host }}/{{ $image.repository }}
tag: {{ $image.tag }}
```

### `.Component`
Component metadata available as a `compdesc.ComponentSpec`, providing access to:
- Component name and version
- Provider information
- Resources list
- Sources
- References
- Repository contexts

## Resource Labeling

Helm values templates should be labeled in the OCM component descriptor with:

```yaml
labels:
  - name: opendefense.cloud/values-for
    value: <helm-chart-resource-name>
```

This label indicates which Helm chart resource this template is for.

## Dependencies

- `github.com/Masterminds/sprig/v3` - Template functions
- `github.com/mandelsoft/vfs` - Virtual filesystem handling
- `ocm.software/ocm` - OCM API

## License

See LICENSE file
