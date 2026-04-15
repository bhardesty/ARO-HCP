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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Azure/ARO-HCP/tooling/image-updater/internal/config"
	"github.com/Azure/ARO-HCP/tooling/image-updater/internal/upgrade"
)

func NewRepositoryVersionUpgradeCommand() *cobra.Command {
	var configPath string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "repository-version-upgrade",
		Short: "Check and upgrade ACM/MCE major version repos from Quay",
		Long: `repository-version-upgrade reads the current ACM/MCE component configurations and checks
Quay.io for the existence of next major version repositories.

For example, if the current ACM operator repo is acm-operator-bundle-acm-216,
this command checks if acm-operator-bundle-acm-217 exists.

By default, if upgrades are detected the command updates the repository references
in both the image-updater config and the target config files (derived from component
targets in the config). Use --dry-run to only report without making changes.

The Prow job script handles running post-bump steps, creating the PR with /hold,
and sending Slack notifications.

Exit codes:
  0 - Success (upgrades applied, or no upgrades found)
  1 - Error occurred`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepositoryVersionUpgrade(cmd, configPath, dryRun)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to image-updater configuration file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only report upgrades without modifying config files")
	if err := cmd.MarkFlagRequired("config"); err != nil {
		return nil
	}

	return cmd
}

func runRepositoryVersionUpgrade(cmd *cobra.Command, configPath string, dryRun bool) error {
	ctx := cmd.Context()

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	checker := upgrade.NewChecker(cfg)
	results, err := checker.CheckAll(ctx)
	if err != nil {
		return fmt.Errorf("repository-version-upgrade failed: %w", err)
	}

	fmt.Print(upgrade.FormatResults(results))

	if !upgrade.HasUpgrades(results) {
		return nil
	}

	if !dryRun {
		if err := upgrade.ApplyUpgrades(results, configPath, cfg); err != nil {
			return fmt.Errorf("failed to apply upgrades: %w", err)
		}
		fmt.Println("Config files updated successfully.")
	}

	return nil
}
