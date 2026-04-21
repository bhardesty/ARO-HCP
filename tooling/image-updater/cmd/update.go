// Copyright 2025 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Azure/ARO-HCP/tooling/image-updater/internal/config"
	"github.com/Azure/ARO-HCP/tooling/image-updater/internal/options"
	"github.com/Azure/ARO-HCP/tooling/image-updater/internal/upgrade"
)

func NewUpdateCommand() *cobra.Command {
	opts := options.DefaultUpdateOptions()

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update image tags/digests or repository versions",
		Long: `Update reads the configuration file and updates image references in target
configuration files.

By default, it fetches the latest image digests from source registries
and updates target files with new digests.

With --repositories/-r, it checks for next-version repositories on Quay
for components that have repoVersionUpgrade configured, and updates the
repository references in both the image-updater config and target files.

Use --dry-run to see what changes would be made without actually updating files.

Use --verbosity (or -v) to control logging verbosity:
  -v 0 or -v 1: Show only human-friendly summary output (default)
  -v 2 or higher: Show detailed debug information`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, opts)
		},
	}

	if err := options.BindUpdateOptions(opts, cmd); err != nil {
		return nil
	}

	return cmd
}

func runUpdate(cmd *cobra.Command, opts *options.RawUpdateOptions) error {
	ctx := cmd.Context()

	if opts.UpdateRepositories {
		return runUpdateRepositories(ctx, opts)
	}

	// --tags or default (neither flag set): update tags/digests
	validated, err := opts.Validate(ctx)
	if err != nil {
		return err
	}

	completed, err := validated.Complete(ctx)
	if err != nil {
		return err
	}

	return completed.UpdateImages(ctx)
}

func runUpdateRepositories(ctx context.Context, opts *options.RawUpdateOptions) error {
	// These flags are only meaningful for the default tags/digests mode
	if opts.Components != "" || opts.Groups != "" || opts.ExcludeComponents != "" {
		return fmt.Errorf("--components, --groups, and --exclude-components cannot be used with --repositories")
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	checker := upgrade.NewChecker(cfg)
	results, err := checker.CheckAll(ctx)
	if err != nil {
		return fmt.Errorf("repository version check failed: %w", err)
	}

	output, err := upgrade.FormatResults(results, opts.OutputFormat)
	if err != nil {
		return fmt.Errorf("failed to format results: %w", err)
	}

	if opts.OutputFile != "" {
		if err := os.WriteFile(opts.OutputFile, []byte(output), 0600); err != nil {
			return fmt.Errorf("failed to write output file %s: %w", opts.OutputFile, err)
		}
	} else {
		fmt.Print(output)
	}

	if !upgrade.HasUpgrades(results) {
		return nil
	}

	if !opts.DryRun {
		if err := upgrade.ApplyUpgrades(results, opts.ConfigPath, cfg); err != nil {
			return fmt.Errorf("failed to apply upgrades: %w", err)
		}
		fmt.Println("Config files updated successfully.")
	}

	return nil
}
