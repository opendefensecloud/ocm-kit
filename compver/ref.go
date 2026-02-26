package compver

import (
	"fmt"
	"strings"
)

type ComponentVersionRef struct {
	Protocol      string
	Host          string
	Namespace     string
	ComponentName string
	Version       string
}

func (cvr *ComponentVersionRef) BaseURL() string {
	return fmt.Sprintf("%s://%s/%s", cvr.Protocol, cvr.Host, cvr.Namespace)
}

func SplitRef(ref string) (*ComponentVersionRef, error) {
	// Split protocol
	protocol := "oci" // Default protocol
	parts := strings.Split(ref, "://")
	rest := parts[0]
	if len(parts) != 1 {
		protocol = parts[0]
		rest = parts[1]
	}

	// Split host and the rest
	hostAndPath := strings.SplitN(rest, "/", 2)
	if len(hostAndPath) != 2 {
		return nil, fmt.Errorf("invalid format: missing path")
	}
	host := hostAndPath[0]

	// Split path by double slash
	pathParts := strings.Split(hostAndPath[1], "//")
	if len(pathParts) != 2 {
		return nil, fmt.Errorf("invalid format: missing namespace separator")
	}
	namespace := pathParts[0]

	// Split component name and version
	componentAndVersion := pathParts[1]
	versionParts := strings.Split(componentAndVersion, ":")
	if len(versionParts) != 2 {
		return nil, fmt.Errorf("invalid format: missing version")
	}
	componentName := versionParts[0]
	version := versionParts[1]

	return &ComponentVersionRef{
		Protocol:      protocol,
		Host:          host,
		Namespace:     namespace,
		ComponentName: componentName,
		Version:       version,
	}, nil
}
