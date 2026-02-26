package helmvalues

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"ocm.software/ocm/api/oci"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	v1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/git"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/helm"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociblob"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/s3"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/wget"
	"ocm.software/ocm/api/ocm/extensions/download"
)

const (
	// HelmValuesTemplateLabelName is the label used to identify Helm values templates in OCM resources
	HelmValuesTemplateLabelName = "opendefense.cloud/helm/values-for"
)

var (
	ErrNotFound = errors.New("not found")
)

// HelmValuesTemplate represents a Helm values template found in an OCM component
type HelmValuesTemplate struct {
	ResourceName    string
	ResourceVersion string
	TemplateContent string
}

// RenderingInput contains all the data needed to render a Helm values template
type RenderingInput struct {
	Resources map[string]any
	Component *compdesc.ComponentSpec
}

// FindHelmValuesTemplate searches for a Helm values template in an OCM component version
// for a specific chart resource. It returns the template if found, or an error if not found.
//
// Parameters:
// - compVer: An OCM ComponentVersionAccess object
// - chartResourceName: The name of the Helm chart resource to find the template for
//
// Returns the HelmValuesTemplate if found, or an error if not found.
func FindHelmValuesTemplate(compVer ocm.ComponentVersionAccess, chartResourceName string) (ocm.ResourceAccess, error) {
	for _, res := range compVer.GetResources() {
		labels := res.Meta().GetLabels()
		if slices.ContainsFunc(labels, func(x v1.Label) bool {
			return x.Name == HelmValuesTemplateLabelName && matchLabelValue(x.Value, chartResourceName)
		}) {
			return res, nil
		}
	}

	return nil, ErrNotFound
}

func FindFirstHelmValuesTemplate(compVer ocm.ComponentVersionAccess) (ocm.ResourceAccess, error) {
	for _, res := range compVer.GetResources() {
		labels := res.Meta().GetLabels()
		if slices.ContainsFunc(labels, func(x v1.Label) bool {
			return x.Name == HelmValuesTemplateLabelName
		}) {
			return res, nil
		}
	}

	return nil, ErrNotFound
}

func FetchHelmValuesTemplate(res ocm.ResourceAccess) (*HelmValuesTemplate, error) {
	// Download the resource
	mfs := memoryfs.New()
	effPath, err := download.DownloadResource(res.GetOCMContext(), res, res.Meta().Name, download.WithFileSystem(mfs))
	if err != nil {
		return nil, fmt.Errorf("failed to download resource: %w", err)
	}

	// Read the template content
	templateContent, err := readFileContent(mfs, effPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template content: %w", err)
	}

	return &HelmValuesTemplate{
		ResourceName:    res.Meta().Name,
		ResourceVersion: res.Meta().Version,
		TemplateContent: templateContent,
	}, nil
}

// FindHelmValuesTemplate searches for a Helm values template in an OCM component version
// for a specific chart resource. It returns the template if found, or an error if not found.
//
// Parameters:
// - compVer: An OCM ComponentVersionAccess object
// - chartResourceName: The name of the Helm chart resource to find the template for
//
// Returns the HelmValuesTemplate if found, or an error if not found.
func GetHelmValuesTemplate(compVer ocm.ComponentVersionAccess, chartResourceName string) (*HelmValuesTemplate, error) {
	res, err := FindHelmValuesTemplate(compVer, chartResourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find helm values template: %w", err)
	}

	return FetchHelmValuesTemplate(res)
}

// GetRenderingInput prepares the data needed to render a Helm values template.
// It extracts resource information from the component and prepares it in a format suitable for templating.
//
// Parameters:
// - compVer: An OCM ComponentVersionAccess object
//
// Returns a RenderingInput containing all the data needed to render templates.
func GetRenderingInput(compVer ocm.ComponentVersionAccess) (*RenderingInput, error) {
	descriptor := compVer.GetDescriptor()
	if descriptor == nil {
		return nil, fmt.Errorf("component descriptor is nil")
	}
	componentSpec := &descriptor.ComponentSpec

	// Extract resource information
	resourceMap := make(map[string]any)

	for _, res := range componentSpec.Resources {
		// Try to typecast access methods to concrete types
		if res.Access != nil {
			// Use a switch to handle different access method types
			switch spec := res.Access.(type) {
			case *ociartifact.AccessSpec:
				// Handle OCI artifact access
				parsedRef, err := ParseOCIRef(spec.ImageReference)
				if err != nil {
					resourceMap[res.Name] = spec
					continue
				}
				resourceMap[res.Name] = parsedRef

			case *ociblob.AccessSpec:
				// Handle OCI blob access
				resourceMap[res.Name] = spec

			case *helm.AccessSpec:
				// Handle Helm repository access
				resourceMap[res.Name] = spec

			case *wget.AccessSpec:
				// Handle Wget access
				resourceMap[res.Name] = spec

			case *s3.AccessSpec:
				// Handle S3 access
				resourceMap[res.Name] = spec

			case *git.AccessSpec:
				// Handle Git access
				resourceMap[res.Name] = spec

			default:
				// Just assign res.Access
				resourceMap[res.Name] = res.Access
				continue
			}
		}
	}

	return &RenderingInput{
		Resources: resourceMap,
		Component: componentSpec,
	}, nil
}

// Render renders a Helm values template using the provided rendering input.
// It applies Go template functions including sprig functions for template processing.
func Render(tmpl *HelmValuesTemplate, input *RenderingInput) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("template is nil")
	}
	if input == nil {
		return "", fmt.Errorf("rendering input is nil")
	}

	// Create template with custom function map
	t, err := template.New(tmpl.ResourceName).Funcs(getFuncMap()).Parse(tmpl.TemplateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare data for template execution
	data := map[string]any{
		"resources": input.Resources,
		"component": input.Component,
	}

	// Execute template
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return out.String(), nil
}

// ParseOCIRef parses an OCI image reference and extracts its components
func ParseOCIRef(imageRef string) (oci.RefSpec, error) {
	return oci.ParseRef(imageRef)
}

func readFileContent(fs vfs.FileSystem, path string) (string, error) {
	f, err := fs.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	b, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(b), nil
}

// matchLabelValue matches a label value (which can be RawMessage or string)
func matchLabelValue(value any, target string) bool {
	switch v := value.(type) {
	case json.RawMessage:
		return string(v) == fmt.Sprintf("\"%s\"", target)
	case string:
		return v == target
	}
	return false
}

// getFuncMap returns the template function map with sprig functions and custom functions
func getFuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	// Remove potentially unsafe functions
	delete(f, "env")
	delete(f, "expandenv")

	// Add custom functions
	f["toJSON"] = func(v any) string {
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	}

	f["parseRef"] = ParseOCIRef

	return f
}
