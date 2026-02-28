package helmvalues

import (
	"encoding/json"
	"testing"
)

// TestRender tests the Render function with various template scenarios
func TestRender(t *testing.T) {
	tests := []struct {
		name      string
		template  *HelmValuesTemplate
		input     *RenderingInput
		wantMatch string
		wantErr   bool
	}{
		{
			name: "simple template with resources",
			template: &HelmValuesTemplate{
				ResourceName:    "test-template",
				ResourceVersion: "1.0.0",
				TemplateContent: `image: {{ index .Resources "app" }}`,
			},
			input: &RenderingInput{
				Resources: map[string]any{
					"app": "myregistry.com/myapp:1.0.0",
				},
			},
			wantMatch: "image: myregistry.com/myapp:1.0.0",
			wantErr:   false,
		},
		{
			name:     "nil template",
			template: nil,
			input: &RenderingInput{
				Resources: map[string]any{},
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
				TemplateContent: `{{.Resources | invalid_func}}`,
			},
			input: &RenderingInput{
				Resources: map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "template with conditional logic",
			template: &HelmValuesTemplate{
				ResourceName:    "conditional",
				ResourceVersion: "1.0.0",
				TemplateContent: `{{- if index .Resources "app" -}}app exists{{- else -}}app missing{{- end -}}`,
			},
			input: &RenderingInput{
				Resources: map[string]any{
					"app": "present",
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
				TemplateContent: `{{- range $k, $v := .Resources }}{{ $k }}: {{ $v }}
{{- end }}`,
			},
			input: &RenderingInput{
				Resources: map[string]any{
					"app1": "image1",
					"app2": "image2",
				},
			},
			wantMatch: "app1: image1",
			wantErr:   false,
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

			if tt.wantMatch != "" && !contains(got, tt.wantMatch) {
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
			if tt.wantTag != "" && !contains(ref, tt.wantTag) {
				t.Errorf("ParseOCIRef() tag not found. Got: %s, Expected to contain: %s", ref, tt.wantTag)
			}
			if tt.wantHost != "" && !contains(ref, tt.wantHost) {
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

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || (len(s) > len(substr) &&
			((s[:len(substr)] == substr) ||
				(s[len(s)-len(substr):] == substr) ||
				findSubstring(s, substr))))
}

// Helper function to find substring anywhere in the string
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
