# ocm-kit

A Go library for working with Open Component Model (OCM) Helm values templates.

## Problem Statement

When an OCM ComponentVersion is transferred from one OCI Registry to another, the default values of Helm Charts will contain images of the old OCI Registry. This library provides functionality to manage and render Helm values templates that are embedded as resources in OCM components, ensuring that image references are correctly resolved regardless of the registry they're accessed from.

## Solution Overview

The library provides mechanisms to:

1. **Find Helm Values Templates**: Locate Helm values templates in OCM components using a label-based approach (`opendefense.cloud/values-for`)
2. **Render Templates**: Process the templates using Go's text/template with sprig functions for flexible value substitution
3. **Extract Component Data**: Automatically extract resource information from OCM components and prepare it for templating

## Key Features

- Find helm values templates by chart resource name
- Parse and prepare OCM component data for templating
- Render templates with access to all component resources
- Support for OCI image reference parsing
- Expose individual helper functions for flexibility in other codebases

## API Overview

### Main Functions

#### `FindHelmValuesTemplate`
```go
func FindHelmValuesTemplate(ctx context.Context, compVer ocmv1alpha1.ComponentVersion, chartResourceName string) (*HelmValuesTemplate, error)
```

Searches for a Helm values template in an OCM component version for a specific chart resource.

**Parameters:**
- `ctx`: Context for the operation
- `compVer`: The OCM ComponentVersion to search
- `chartResourceName`: The name of the Helm chart resource

**Returns:**
- `*HelmValuesTemplate`: The found template with resource metadata and content
- `error`: If template not found or operation fails

#### `GetRenderingInput`
```go
func GetRenderingInput(ctx context.Context, compVer ocmv1alpha1.ComponentVersion) (*RenderingInput, error)
```

Prepares data needed to render a Helm values template by extracting resource information from the component.

**Parameters:**
- `ctx`: Context for the operation
- `compVer`: The OCM ComponentVersion to extract data from

**Returns:**
- `*RenderingInput`: Data structure containing resources and component info
- `error`: If data extraction fails

#### `RenderHelmValuesTemplate`
```go
func RenderHelmValuesTemplate(tmpl *HelmValuesTemplate, input *RenderingInput) (string, error)
```

Renders a Helm values template using the provided rendering input.

**Parameters:**
- `tmpl`: The template to render
- `input`: The rendering input containing data

**Returns:**
- `string`: The rendered Helm values output
- `error`: If rendering fails

### Helper Functions

#### `ParseOCIRef`
```go
func ParseOCIRef(imageRef string) (*ImageRef, error)
```

Parses an OCI image reference and extracts its components (host, repository, tag).

#### `ToMap`
```go
func ToMap(obj any) (map[string]any, error)
```

Converts any object to a map[string]any using JSON marshaling/unmarshaling. Useful for flexible data handling.

#### `ToMapSafe`
```go
func ToMapSafe(obj any) map[string]any
```

Like `ToMap` but returns an empty map on error instead of failing.

#### `GetFuncMap`
```go
func GetFuncMap() template.FuncMap
```

Returns the template function map with sprig functions and custom functions for use in templates.

## Data Types

### `HelmValuesTemplate`
```go
type HelmValuesTemplate struct {
    ResourceName    string
    ResourceVersion string
    ChartName       string
    TemplateContent string
}
```

Represents a Helm values template found in an OCM component.

### `RenderingInput`
```go
type RenderingInput struct {
    Resources map[string]any
    Component ComponentInfo
}
```

Contains all the data needed to render a Helm values template.

### `ComponentInfo`
```go
type ComponentInfo struct {
    Name                string
    Version             string
    Provider            map[string]any
    CreationTime        string
    RepositoryContexts  []any
    Sources             []any
    ComponentReferences []any
}
```

Holds metadata about an OCM component.

### `ImageRef`
```go
type ImageRef struct {
    Host       string `json:"host"`
    Repository string `json:"repository"`
    Tag        string `json:"tag"`
}
```

Represents a parsed OCI image reference with components.

## Usage Example

```go
package main

import (
    "context"
    "log"
    
    "go.opendefense.cloud/ocm-kit/helmvalues"
    "ocm.software/ocm/api/ocm"
    "ocm.software/ocm/api/ocm/extensions/repositories/ocireg"
)

func main() {
    ctx := context.Background()
    
    // Setup OCM repository
    octx := ocm.FromContext(ctx)
    repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec("http://localhost:5000/my-components"))
    if err != nil {
        log.Fatal(err)
    }
    defer repo.Close()
    
    // Get component version
    compVer, err := repo.LookupComponentVersion("opendefense.cloud/arc", "0.1.0")
    if err != nil {
        log.Fatal(err)
    }
    defer compVer.Close()
    
    // Find helm values template for a specific chart
    tmpl, err := helmvalues.FindHelmValuesTemplate(ctx, compVer, "helm-chart")
    if err != nil {
        log.Fatal(err)
    }
    
    // Get rendering input with component data
    input, err := helmvalues.GetRenderingInput(ctx, compVer)
    if err != nil {
        log.Fatal(err)
    }
    
    // Render the template
    renderedValues, err := helmvalues.RenderHelmValuesTemplate(tmpl, input)
    if err != nil {
        log.Fatal(err)
    }
    
    log.Println(renderedValues)
}
```

## Template Variables

When rendering templates, the following data is available:

### `.resources`
A map of all resources in the component by resource name. For OCI images, each resource is automatically parsed into an `ImageRef` with:
- `.host` - The registry host
- `.repository` - The repository path
- `.tag` - The image tag

Access example:
```go
{{- $image := index .resources "my-image" }}
repository: {{ $image.host }}/{{ $image.repository }}
tag: {{ $image.tag }}
```

### `.component`
Component metadata including:
- `.Name` - Component name
- `.Version` - Component version
- `.Provider` - Provider information
- `.CreationTime` - Creation timestamp
- `.RepositoryContexts` - Repository contexts
- `.Sources` - Source references
- `.ComponentReferences` - Component references

## Template Functions

The library includes all [sprig](http://masterminds.github.io/sprig/) template functions, plus custom functions:

- `toJSON` - Convert value to JSON string
- `parseRef` - Parse an OCI image reference

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
