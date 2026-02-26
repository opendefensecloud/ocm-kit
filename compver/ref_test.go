package compver

import (
	"testing"
)

func TestSplitRef(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *ComponentVersionRef
		wantErr bool
	}{
		{
			name:  "valid input with http protocol",
			input: "http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0",
			want: &ComponentVersionRef{
				Protocol:      "http",
				Host:          "localhost:5000",
				Namespace:     "my-components",
				ComponentName: "opendefense.cloud/arc",
				Version:       "0.1.0",
			},
			wantErr: false,
		},
		{
			name:  "valid input without protocol (default to oci)",
			input: "localhost:5000/my-components//opendefense.cloud/arc:0.1.0",
			want: &ComponentVersionRef{
				Protocol:      "oci",
				Host:          "localhost:5000",
				Namespace:     "my-components",
				ComponentName: "opendefense.cloud/arc",
				Version:       "0.1.0",
			},
			wantErr: false,
		},
		{
			name:  "valid input with oci protocol explicitly",
			input: "oci://registry.example.com/stable//example.com/mycomponent:1.2.3",
			want: &ComponentVersionRef{
				Protocol:      "oci",
				Host:          "registry.example.com",
				Namespace:     "stable",
				ComponentName: "example.com/mycomponent",
				Version:       "1.2.3",
			},
			wantErr: false,
		},
		{
			name:  "valid input with different namespace",
			input: "https://repo.example.com/products/stable//acme.corp/app-engine:2.0.0-rc1",
			want: &ComponentVersionRef{
				Protocol:      "https",
				Host:          "repo.example.com",
				Namespace:     "products/stable",
				ComponentName: "acme.corp/app-engine",
				Version:       "2.0.0-rc1",
			},
			wantErr: false,
		},
		{
			name:    "missing namespace separator",
			input:   "http://localhost:5000/my-componentsopendefense.cloud/arc:0.1.0",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing version separator",
			input:   "http://localhost:5000/my-components//opendefense.cloud/arc0.1.0",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing path",
			input:   "http://localhost:5000",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			want:    nil,
			wantErr: true,
		},
		{
			name:  "complex namespace path",
			input: "oci://registry.io/ns1/ns2/ns3//domain.com/component:1.0",
			want: &ComponentVersionRef{
				Protocol:      "oci",
				Host:          "registry.io",
				Namespace:     "ns1/ns2/ns3",
				ComponentName: "domain.com/component",
				Version:       "1.0",
			},
			wantErr: false,
		},
		{
			name:  "version with plus sign",
			input: "oci://localhost/app//my.company/service:1.0.0+build123",
			want: &ComponentVersionRef{
				Protocol:      "oci",
				Host:          "localhost",
				Namespace:     "app",
				ComponentName: "my.company/service",
				Version:       "1.0.0+build123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SplitRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.Protocol != tt.want.Protocol {
				t.Errorf("Protocol: got %q, want %q", got.Protocol, tt.want.Protocol)
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host: got %q, want %q", got.Host, tt.want.Host)
			}
			if got.Namespace != tt.want.Namespace {
				t.Errorf("Namespace: got %q, want %q", got.Namespace, tt.want.Namespace)
			}
			if got.ComponentName != tt.want.ComponentName {
				t.Errorf("ComponentName: got %q, want %q", got.ComponentName, tt.want.ComponentName)
			}
			if got.Version != tt.want.Version {
				t.Errorf("Version: got %q, want %q", got.Version, tt.want.Version)
			}
		})
	}
}
