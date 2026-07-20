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
	rest := ref
	if before, after, found := strings.Cut(ref, "://"); found {
		protocol = before
		rest = after
	}

	// Split host and the rest
	host, path, found := strings.Cut(rest, "/")
	if !found {
		return nil, invalidFormatErr(ref)
	}

	// Split path by double slash
	pathParts := strings.Split(path, "//")
	if len(pathParts) != 2 {
		return nil, invalidFormatErr(ref)
	}
	namespace := pathParts[0]

	// Split component name and version
	componentAndVersion := pathParts[1]
	versionParts := strings.Split(componentAndVersion, ":")
	if len(versionParts) != 2 {
		return nil, invalidFormatErr(ref)
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

func invalidFormatErr(ref string) error {
	return fmt.Errorf("invalid component version reference: %s: expected [<protocol>://]<host>/<namespace>//<component-name>:<version>", ref)
}
