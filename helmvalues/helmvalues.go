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
	"ocm.software/ocm/api/oci"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	v1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"sigs.k8s.io/yaml"
)

const (
	// HelmValuesTemplateLabelName is the label used to identify Helm values templates in OCM resources
	HelmValuesTemplateLabelName = "opendefense.cloud/helm/values-for"
)

var (
	// ErrNotFound is returned when a requested Helm values template is not found
	ErrNotFound = errors.New("not found")
)

// HelmValuesTemplate represents a Helm values template found in an OCM component.
// It contains the template content along with metadata about its resource.
type HelmValuesTemplate struct {
	ResourceName    string
	ResourceVersion string
	TemplateContent string
}

// ImageReference is a representation of an OCI image reference with all its parts.
// All fields are expected to be non-empty after parsing a reference (see ParseOCIRef) and Host might include a port.
type ImageReference struct {
	Host       string
	Repository string
	Tag        string
	Digest     string
}

// RenderingInput contains all the data needed to render a Helm values template.
// It provides access to component resources and the component descriptor for template processing.
type RenderingInput struct {
	OCIResources map[string]ImageReference
	Component    *compdesc.ComponentSpec
}

// RenderOption is a functional option for configuring Render behavior
type RenderOption func(*renderConfig)

// renderConfig holds configuration for the Render function
type renderConfig struct {
	validateYAML bool
}

// WithYAMLValidation enables YAML validation of the rendered output
func WithYAMLValidation() RenderOption {
	return func(rc *renderConfig) {
		rc.validateYAML = true
	}
}

// FindHelmValuesTemplate searches for a Helm values template in an OCM component version
// for a specific chart resource. It looks for resources labeled with the HelmValuesTemplateLabelName
// where the label value matches the provided chartResourceName.
//
// Parameters:
//   - compVer: An OCM ComponentVersionAccess object
//   - chartResourceName: The name of the Helm chart resource to find the template for
//
// Returns the ResourceAccess for the template if found, or ErrNotFound if no matching template exists.
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

// FindFirstHelmValuesTemplate searches for the first Helm values template in an OCM component version.
// It looks for any resource labeled with the HelmValuesTemplateLabelName, regardless of the label value.
//
// Parameters:
//   - compVer: An OCM ComponentVersionAccess object
//
// Returns the ResourceAccess for the first template found, or ErrNotFound if no template exists.
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

// FetchHelmValuesTemplate extracts the content from a Helm values template resource.
//
// Parameters:
//   - res: The OCM ResourceAccess to download
//
// Returns a HelmValuesTemplate with the downloaded content, or an error if download/read fails.
func FetchHelmValuesTemplate(res ocm.ResourceAccess) (*HelmValuesTemplate, error) {
	blobaccess, err := res.BlobAccess()
	if err != nil {
		return nil, fmt.Errorf("failed to get blob access for resource: %w", err)
	}
	defer func() {
		_ = blobaccess.Close()
	}()
	r, err := blobaccess.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to get reader for resource access: %w", err)
	}
	defer func() {
		_ = r.Close()
	}()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource content: %w", err)
	}
	templateContent := string(b)

	return &HelmValuesTemplate{
		ResourceName:    res.Meta().Name,
		ResourceVersion: res.Meta().Version,
		TemplateContent: templateContent,
	}, nil
}

// GetHelmValuesTemplate searches for and retrieves a Helm values template from an OCM component.
// This is a convenience function that combines FindHelmValuesTemplate and FetchHelmValuesTemplate.
//
// Parameters:
//   - compVer: An OCM ComponentVersionAccess object
//   - chartResourceName: The name of the Helm chart resource to find the template for
//
// Returns a HelmValuesTemplate with the downloaded content, or an error if not found or download fails.
func GetHelmValuesTemplate(compVer ocm.ComponentVersionAccess, chartResourceName string) (*HelmValuesTemplate, error) {
	res, err := FindHelmValuesTemplate(compVer, chartResourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find helm values template: %w", err)
	}

	return FetchHelmValuesTemplate(res)
}

// GetFirstHelmValuesTemplate retrieves the first Helm values template from an OCM component.
// This is a convenience function that combines FindFirstHelmValuesTemplate and FetchHelmValuesTemplate.
//
// Parameters:
//   - compVer: An OCM ComponentVersionAccess object
//
// Returns a HelmValuesTemplate with the downloaded content, or an error if not found or download fails.
func GetFirstHelmValuesTemplate(compVer ocm.ComponentVersionAccess) (*HelmValuesTemplate, error) {
	res, err := FindFirstHelmValuesTemplate(compVer)
	if err != nil {
		return nil, fmt.Errorf("failed to find first helm values template: %w", err)
	}

	return FetchHelmValuesTemplate(res)
}

// GetRenderingInput extracts and prepares data needed to render a Helm values template.
// It iterates through all resources in the component and processes them based on their access method.
// OCI artifacts are automatically parsed into ImageRef structures for easy access in templates.
// Other access methods are stored as-is or converted appropriately.
//
// Parameters:
//   - compVer: An OCM ComponentVersionAccess object
//
// Returns a RenderingInput containing all the data needed to render templates, or an error if extraction fails.
func GetRenderingInput(compVer ocm.ComponentVersionAccess) (*RenderingInput, error) {
	descriptor := compVer.GetDescriptor()
	if descriptor == nil {
		return nil, fmt.Errorf("component descriptor is nil")
	}
	componentSpec := &descriptor.ComponentSpec

	// Extract oci resource information
	ociResourceMap := make(map[string]ImageReference)

	for _, res := range compVer.GetResources() {
		ga := res.GlobalAccess()
		if ga == nil {
			continue
		}
		spec, ok := ga.(*ociartifact.AccessSpec)
		if !ok {
			continue
		}

		// Parse OCI reference and store it
		parsedRef, err := ParseOCIRef(spec.ImageReference)
		if err != nil {
			return nil, fmt.Errorf("resources access contained invalid image reference: %w", err)
		}
		digest := ""
		if parsedRef.Digest != nil {
			digest = string(*parsedRef.Digest)
		}
		ociResourceMap[res.Meta().Name] = ImageReference{
			Host:       parsedRef.Host,
			Repository: parsedRef.Repository,
			Tag:        derefOrEmpty(parsedRef.Tag),
			Digest:     digest,
		}
	}

	return &RenderingInput{
		OCIResources: ociResourceMap,
		Component:    componentSpec,
	}, nil
}

// Render processes a Helm values template with the provided rendering input.
// It uses Go's text/template engine with sprig functions for flexible template processing.
// The template has access to all data in the RenderingInput through dot notation.
//
// Parameters:
//   - tmpl: The HelmValuesTemplate to render
//   - input: The RenderingInput containing template data
//   - opts: Optional RenderOption functions to configure behavior (e.g., WithYAMLValidation)
//
// Returns the rendered template as a string, or an error if parsing or execution fails.
// If WithYAMLValidation is enabled, also returns an error if the output is not valid YAML.
func Render(tmpl *HelmValuesTemplate, input *RenderingInput, opts ...RenderOption) (string, error) {
	if tmpl == nil {
		return "", fmt.Errorf("template is nil")
	}
	if input == nil {
		return "", fmt.Errorf("rendering input is nil")
	}

	// Apply options to config
	cfg := &renderConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Create template with custom function map
	t, err := template.New(tmpl.ResourceName).
		Option("missingkey=error").
		Funcs(getFuncMap()).
		Parse(tmpl.TemplateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var out bytes.Buffer
	if err := t.Execute(&out, input); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	result := out.String()

	// Validate YAML if enabled
	if cfg.validateYAML {
		var jsonData interface{}
		if err := yaml.Unmarshal([]byte(result), &jsonData); err != nil {
			return "", fmt.Errorf("rendered output is not valid YAML: %w", err)
		}
	}

	return result, nil
}

// ParseOCIRef parses an OCI image reference and extracts its components.
// Returns an oci.RefSpec containing the parsed reference details.
//
// Parameters:
//   - imageRef: The OCI image reference string (e.g., "registry.example.com/repo/image:tag")
//
// Returns an oci.RefSpec with the parsed reference, or an error if parsing fails.
func ParseOCIRef(imageRef string) (oci.RefSpec, error) {
	return oci.ParseRef(imageRef)
}

// matchLabelValue checks if a label value matches the target string.
// Label values can be either json.RawMessage or string, so this function handles both types.
//
// Parameters:
//   - value: The label value to check (can be json.RawMessage or string)
//   - target: The target string to match against
//
// Returns true if the value matches the target, false otherwise.
func matchLabelValue(value any, target string) bool {
	switch v := value.(type) {
	case json.RawMessage:
		return string(v) == fmt.Sprintf("\"%s\"", target)
	case string:
		return v == target
	}
	return false
}

// getFuncMap creates and returns the template function map for rendering templates.
// It includes all sprig template functions (except potentially unsafe ones like env and expandenv)
// plus custom functions for JSON conversion and OCI reference parsing.
//
// Returns a template.FuncMap with all available template functions.
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

func (r ImageReference) String() string {
	s := ""
	if r.Host != "" {
		s += r.Host + "/"
	}
	if r.Repository != "" {
		s += r.Repository
	}
	if r.Tag != "" {
		s += ":" + r.Tag
	}
	return s
}

func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	} else {
		return *s
	}
}
