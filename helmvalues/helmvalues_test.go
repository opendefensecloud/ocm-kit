package helmvalues

import (
	"encoding/json"
	"strings"
	"testing"

	"ocm.software/ocm/api/oci"
)

// TestRender tests the Render function with various template scenarios
func TestRender(t *testing.T) {
	tests := []struct {
		name      string
		template  *HelmValuesTemplate
		input     *RenderingInput
		options   []RenderOption
		wantMatch string
		wantErr   bool
	}{
		{
			name: "simple template with resources",
			template: &HelmValuesTemplate{
				ResourceName:    "test-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `image: {{ index .OCIResources "app" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{
					"app": mkImageRef("myregistry.com/myapp:1.0.0"),
				},
			},
			wantMatch: "image: myregistry.com/myapp:1.0.0",
			wantErr:   false,
		},
		{
			name:     "nil template",
			template: nil,
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
			},
			wantErr: true,
		},
		{
			name: "nil input",
			template: &HelmValuesTemplate{
				ResourceName:    "test",
				ResourceVersion: "1.0.0",
				TemplateContent: "test",
			},
			input:   nil,
			wantErr: true,
		},
		{
			name: "invalid template syntax",
			template: &HelmValuesTemplate{
				ResourceName:    "invalid",
				ResourceVersion: "1.0.0",
				TemplateContent: `{{.OCIResources | invalid_func}}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
			},
			wantErr: true,
		},
		{
			name: "template with conditional logic",
			template: &HelmValuesTemplate{
				ResourceName:    "conditional",
				ResourceVersion: "1.0.0",
				TemplateContent: `{{- if index .OCIResources "app" -}}app exists{{- else -}}app missing{{- end -}}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{
					"app": mkImageRef("present"),
				},
			},
			wantMatch: "app exists",
			wantErr:   false,
		},
		{
			name: "template with range over resources",
			template: &HelmValuesTemplate{
				ResourceName:    "range-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `{{- range $k, $v := .OCIResources }}{{ $k }}: {{ $v }}
{{- end }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{
					"app1": mkImageRef("image1"),
					"app2": mkImageRef("image2"),
				},
			},
			wantMatch: "app1: image1",
			wantErr:   false,
		},
		{
			name: "template with invalid yaml and validation disabled",
			template: &HelmValuesTemplate{
				ResourceName:    "invalid-yaml-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `{key1: value1, key2: : value2}`,
			},
			input:   &RenderingInput{},
			wantErr: false,
		},
		{
			name: "template with invalid yaml and validation enabled",
			template: &HelmValuesTemplate{
				ResourceName:    "invalid-yaml-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `{key1: value1, key2: : value2}`,
			},
			input:   &RenderingInput{},
			options: []RenderOption{WithYAMLValidation()},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.template, tt.input, tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if tt.wantMatch != "" && !strings.Contains(got, tt.wantMatch) {
				t.Errorf("Render() output doesn't contain expected text.\nGot: %s\nExpected to contain: %s", got, tt.wantMatch)
			}
		})
	}
}

// TestParseOCIRef tests the ParseOCIRef function with various OCI reference formats
func TestParseOCIRef(t *testing.T) {
	tests := []struct {
		name     string
		imageRef string
		wantHost string
		wantPath string
		wantTag  string
		wantErr  bool
	}{
		{
			name:     "simple reference with tag",
			imageRef: "http://localhost:5000/my-components/opendefensecloud/charts/arc:0.1.4@sha256:43d0a3045598b20ca8f39ac1b709e2a574d3a710d27aab5edf5b98ef40fe4d60",
			wantHost: "localhost:5000",
			wantPath: "my-components/opendefensecloud/charts/arc",
			wantTag:  "0.1.4",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOCIRef(tt.imageRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOCIRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Verify the parsed reference has expected components
			ref := got.String()
			if ref == "" {
				t.Errorf("ParseOCIRef() returned empty string")
			}

			// For valid references, check that basic parsing succeeded
			if tt.wantTag != "" && !strings.Contains(ref, tt.wantTag) {
				t.Errorf("ParseOCIRef() tag not found. Got: %s, Expected to contain: %s", ref, tt.wantTag)
			}
			if tt.wantHost != "" && !strings.Contains(ref, tt.wantHost) {
				t.Errorf("ParseOCIRef() host not found. Got: %s, Expected to contain: %s", ref, tt.wantHost)
			}
		})
	}
}

// TestMatchLabelValue tests the matchLabelValue function with different value types
func TestMatchLabelValue(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		target string
		want   bool
	}{
		{
			name:   "string value match",
			value:  "helm-chart",
			target: "helm-chart",
			want:   true,
		},
		{
			name:   "string value no match",
			value:  "other-chart",
			target: "helm-chart",
			want:   false,
		},
		{
			name:   "json.RawMessage match",
			value:  json.RawMessage(`"helm-chart"`),
			target: "helm-chart",
			want:   true,
		},
		{
			name:   "json.RawMessage no match",
			value:  json.RawMessage(`"different-chart"`),
			target: "helm-chart",
			want:   false,
		},
		{
			name:   "json.RawMessage without quotes",
			value:  json.RawMessage(`helm-chart`),
			target: "helm-chart",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchLabelValue(tt.value, tt.target)
			if got != tt.want {
				t.Errorf("matchLabelValue(%v, %q) = %v, want %v", tt.value, tt.target, got, tt.want)
			}
		})
	}
}

// TestPullSecretsResolve tests the Resolve method with OCI refs and raw registries
func TestPullSecretsResolve(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		secrets PullSecrets
		want    string
	}{
		{
			name:    "full ref matches Host/Repository",
			ref:     "ghcr.io/org/myapp:v1.0.0",
			secrets: PullSecrets{"ghcr.io/org/myapp": "repo-cred"},
			want:    "repo-cred",
		},
		{
			name:    "full ref matches host only",
			ref:     "ghcr.io/org/myapp:v1.0.0",
			secrets: PullSecrets{"ghcr.io": "org-cred"},
			want:    "org-cred",
		},
		{
			name: "Host/Repository takes priority over host",
			ref:  "ghcr.io/org/myapp:v1.0.0",
			secrets: PullSecrets{
				"ghcr.io/org/myapp": "repo-cred",
				"ghcr.io":           "org-cred",
			},
			want: "repo-cred",
		},
		{
			name:    "ref with nested path matches correctly",
			ref:     "registry.example.com/team/service/sub:v2",
			secrets: PullSecrets{"registry.example.com/team/service/sub": "nested-cred"},
			want:    "nested-cred",
		},
		{
			name:    "ref matches intermediate org path, not just host",
			ref:     "docker.io/team-a/my-repo:latest",
			secrets: PullSecrets{"docker.io/team-a": "team-a-secret"},
			want:    "team-a-secret",
		},
		{
			name: "most specific match wins among path hierarchy",
			ref:  "docker.io/team-a/my-repo:latest",
			secrets: PullSecrets{
				"docker.io/team-a/my-repo": "repo-secret",
				"docker.io/team-a":         "org-secret",
				"docker.io":                "global-secret",
			},
			want: "repo-secret",
		},
		{
			name:    "intermediate org resolves independently per organization",
			ref:     "docker.io/team-a/svc:latest",
			secrets: PullSecrets{"docker.io/team-b": "team-b-secret"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.secrets.Resolve(tt.ref)
			if got != tt.want {
				t.Errorf("PullSecrets.Resolve(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

// TestPullSecretsGet tests the PullSecrets type directly
func TestPullSecretsGet(t *testing.T) {
	tests := []struct {
		name     string
		secrets  PullSecrets
		registry string
		want     string
	}{
		{
			name:     "known registry returns secret",
			secrets:  PullSecrets{"docker.io": "regcred", "ghcr.io": "ghcr-cred"},
			registry: "docker.io",
			want:     "regcred",
		},
		{
			name:     "unknown registry returns empty string",
			secrets:  PullSecrets{"docker.io": "regcred"},
			registry: "unknown.registry.io",
			want:     "",
		},
		{
			name:     "nil PullSecrets returns empty string",
			secrets:  nil,
			registry: "docker.io",
			want:     "",
		},
		{
			name:     "empty PullSecrets returns empty string",
			secrets:  PullSecrets{},
			registry: "docker.io",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.secrets.Get(tt.registry)
			if got != tt.want {
				t.Errorf("PullSecrets.Get(%q) = %q, want %q", tt.registry, got, tt.want)
			}
		})
	}
}

// TestRenderPullSecretFor tests the pullSecretFor template function via Render
func TestRenderPullSecretFor(t *testing.T) {
	tests := []struct {
		name     string
		template *HelmValuesTemplate
		input    *RenderingInput
		want     string
		wantErr  bool
	}{
		{
			name: "pullSecretFor with matching registry",
			template: &HelmValuesTemplate{
				ResourceName:    "pull-secret-test",
				ResourceVersion: "1.0.0",
				TemplateContent: `secret: {{ pullSecretFor "docker.io" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
				PullSecrets: PullSecrets{
					"docker.io": "regcred",
				},
			},
			want:    "secret: regcred",
			wantErr: false,
		},
		{
			name: "pullSecretFor with non-matching registry",
			template: &HelmValuesTemplate{
				ResourceName:    "pull-secret-no-match",
				ResourceVersion: "1.0.0",
				TemplateContent: `secret: {{ pullSecretFor "unknown.io" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
				PullSecrets: PullSecrets{
					"docker.io": "regcred",
				},
			},
			want:    "secret: ",
			wantErr: false,
		},
		{
			name: "pullSecretFor with ref",
			template: &HelmValuesTemplate{
				ResourceName:    "pull-secret-ref",
				ResourceVersion: "1.0.0",
				TemplateContent: `secret: {{ pullSecretFor "registry.example.com/repo/image:tag" }}`,
			},
			input: &RenderingInput{
				OCIResources: map[string]ImageReference{},
				PullSecrets: PullSecrets{
					"registry.example.com": "example-cred",
				},
			},
			want:    "secret: example-cred",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.template, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func mkImageRef(ref string) ImageReference {
	parsed, err := oci.ParseRef(ref)
	if err != nil {
		panic(err)
	}
	if parsed.Host == "docker.io" {
		// special case from oci.ParseRef
		return ImageReference{
			Host:       "",
			Repository: strings.Replace(parsed.Repository, "library/", "", 1),
			Tag:        derefOrEmpty(parsed.Tag),
		}
	}

	return ImageReference{
		Host:       parsed.Host,
		Repository: parsed.Repository,
		Tag:        derefOrEmpty(parsed.Tag),
	}
}
