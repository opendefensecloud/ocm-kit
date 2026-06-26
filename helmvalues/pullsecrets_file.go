package helmvalues

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// pullSecretsFile holds mappings from registries to pull secrets in the cluster
type pullSecretsFile struct {
	PullSecrets []pullSecretMapping `json:"pullSecrets"`
}

// PullSecretEntry maps an OCI registry to the Kubernetes secret containing its credentials.
type pullSecretMapping struct {
	Registry   string `json:"registry"`
	SecretName string `json:"secretName"`
}

func ParsePullSecretsFile(path string) (PullSecrets, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read pull secrets file: %w", err)
	}

	var psf pullSecretsFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&psf); err != nil {
		return nil, fmt.Errorf("failed to parse pull secrets file: %w", err)
	}

	ps := make(PullSecrets, len(psf.PullSecrets))
	for _, e := range psf.PullSecrets {
		if e.Registry == "" || e.SecretName == "" {
			return nil, fmt.Errorf("invalid pull secret entry: registry and secretName must be non-empty")
		}
		ps[e.Registry] = e.SecretName
	}

	return ps, nil
}
