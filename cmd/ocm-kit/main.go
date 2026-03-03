package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/repositories/ocireg"

	"go.opendefense.cloud/ocm-kit/compver"
	"go.opendefense.cloud/ocm-kit/helmvalues"
)

func main() {
	var (
		chartResName            string
		localHelmValuesTemplate string
	)

	rootCmd := &cobra.Command{
		Use:   "ocm-kit <component-version-ref>",
		Short: "OCM Kit - Render Helm values templates from OCM components",
		Long: `OCM Kit renders Helm values templates embedded in OCM (Open Component Model) components.

It takes a component version reference and renders the first Helm values template for a specified chart.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			componentVersionRef := args[0]

			cvr, err := compver.SplitRef(componentVersionRef)
			if err != nil {
				return fmt.Errorf("failed to split component version reference: %w", err)
			}

			ctx := context.Background()
			octx := ocm.FromContext(ctx)
			repo, err := octx.RepositoryForSpec(ocireg.NewRepositorySpec(cvr.BaseURL()))
			if err != nil {
				return fmt.Errorf("failed to construct repository: %w", err)
			}
			defer func() { _ = repo.Close() }()

			compVer, err := repo.LookupComponentVersion(cvr.ComponentName, cvr.Version)
			if err != nil {
				return fmt.Errorf("failed to lookup component version: %w", err)
			}
			defer func() { _ = compVer.Close() }()

			var template *helmvalues.HelmValuesTemplate

			// Use local file if provided, otherwise fetch from component
			if localHelmValuesTemplate != "" {
				content, err := os.ReadFile(localHelmValuesTemplate)
				if err != nil {
					return fmt.Errorf("failed to read local helm values template: %w", err)
				}
				template = &helmvalues.HelmValuesTemplate{
					ResourceName:    "local-file",
					ResourceVersion: "0.0.0",
					TemplateContent: string(content),
				}
			} else if chartResName != "" {
				template, err = helmvalues.GetHelmValuesTemplate(compVer, chartResName)
				if err != nil {
					return fmt.Errorf("failed to get helm values template: %w", err)
				}
			} else {
				template, err = helmvalues.GetFirstHelmValuesTemplate(compVer)
				if err != nil {
					return fmt.Errorf("failed to get helm values template: %w", err)
				}
			}

			input, err := helmvalues.GetRenderingInput(compVer)
			if err != nil {
				return fmt.Errorf("failed to build rendering input: %w", err)
			}

			output, err := helmvalues.Render(template, input)
			if err != nil {
				return fmt.Errorf("failed to render helm values template: %w", err)
			}

			fmt.Println(output)
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&chartResName, "chart-resource", "r", "", "Name of the Helm chart resource in the component to render a specific helm values template")
	rootCmd.Flags().StringVarP(&localHelmValuesTemplate, "local-helm-values-template", "f", "", "Path to a local Helm values template file (overrides component template)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
