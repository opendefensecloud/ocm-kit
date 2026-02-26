package main

import (
	"context"
	"fmt"
	"log"

	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	"go.opendefense.cloud/ocm-kit/compver"
	"go.opendefense.cloud/ocm-kit/helmvalues"
)

func main() {
	componentVersionRef := "http://localhost:5000/my-components//opendefense.cloud/arc:0.1.0" // should be a flag
	chartResName := "helm-chart"                                                              // should be a flag

	cvr, err := compver.SplitRef(componentVersionRef)
	if err != nil {
		log.Fatal("failed to split component version reference", err)
	}

	ctx := context.Background()
	octx := ocm.FromContext(ctx)
	repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec(cvr.BaseURL()))
	if err != nil {
		log.Fatal("failed to construct repository: ", err)
	}
	defer func() { _ = repo.Close() }()

	compVer, err := repo.LookupComponentVersion(cvr.ComponentName, cvr.Version)
	if err != nil {
		log.Fatal("failed to lookup component version: ", err)
	}
	defer func() { _ = compVer.Close() }()

	template, err := helmvalues.GetHelmValuesTemplate(compVer, chartResName)
	if err != nil {
		log.Fatal("failed to get helm values template: ", err)
	}

	input, err := helmvalues.GetRenderingInput(compVer)
	if err != nil {
		log.Fatal("failed to build rendering input: ", err)
	}

	output, err := helmvalues.Render(template, input)
	if err != nil {
		log.Fatal("failed to render helm values template: ", err)
	}

	fmt.Println(output)
}
