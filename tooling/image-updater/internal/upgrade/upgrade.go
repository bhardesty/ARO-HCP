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

package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/Azure/ARO-HCP/tooling/image-updater/internal/config"
)

// repoVersionSuffix matches a trailing version suffix like "216", "211", or "29"
// in repository names such as "acm-operator-bundle-acm-216".
// Group 1 captures the major version (single digit), group 2 captures the minor version (1+ digits).
var repoVersionSuffix = regexp.MustCompile(`^(\d)(\d+)$`)

// Result holds the check-upgrade result for a single ACM/MCE component.
type Result struct {
	ComponentName    string
	CurrentRepo      string
	CurrentVersion   string // e.g. "2.16"
	NextRepo         string
	NextVersion      string // e.g. "2.17"
	NextRepoExists   bool
	LatestTag        string // Latest version tag in next repo (if exists)
	LatestTagDate    string // Date of latest tag
	UpgradeAvailable bool
}

// quayRepoResponse is the minimal Quay API response for a repository.
type quayRepoResponse struct {
	Name string `json:"name"`
}

// quayTag mirrors the tag fields we care about from Quay's tag list API.
type quayTag struct {
	Name         string `json:"name"`
	LastModified string `json:"last_modified"`
}

// quayTagsResponse is the Quay API response for listing tags.
type quayTagsResponse struct {
	Tags []quayTag `json:"tags"`
}

// Checker performs ACM/MCE upgrade checks against Quay.
type Checker struct {
	httpClient *http.Client
	baseURL    string
	config     *config.Config
}

// NewChecker creates a new upgrade checker.
func NewChecker(cfg *config.Config) *Checker {
	return &Checker{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://quay.io/api/v1",
		config:  cfg,
	}
}

// CheckAll checks all components with repoVersionUpgrade configured for available upgrades.
func (c *Checker) CheckAll(ctx context.Context) ([]Result, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("logger not found in context: %w", err)
	}

	var results []Result

	for componentName, imageConfig := range c.config.Images {
		if imageConfig.Source.RepoVersionUpgrade == nil {
			continue
		}

		prefix := imageConfig.Source.RepoVersionUpgrade.RepoPrefix
		logger.V(1).Info("checking component for repo version upgrade", "component", componentName, "prefix", prefix)

		result, err := c.checkComponent(ctx, componentName, prefix, imageConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to check component %s: %w", componentName, err)
		}

		results = append(results, *result)
	}

	return results, nil
}

// checkComponent checks a single ACM/MCE component for available upgrades.
func (c *Checker) checkComponent(ctx context.Context, componentName, prefix string, imageConfig config.ImageConfig) (*Result, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("logger not found in context: %w", err)
	}

	_, repository, err := imageConfig.Source.ParseImageReference()
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	currentVersion, err := extractVersion(repository, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to extract version from %s: %w", repository, err)
	}

	nextVersion, err := incrementMinorVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to increment version %s: %w", currentVersion, err)
	}

	nextRepo := buildNextRepo(repository, prefix, nextVersion)
	logger.V(1).Info("checking for next version repo",
		"component", componentName,
		"currentRepo", repository,
		"currentVersion", currentVersion,
		"nextRepo", nextRepo,
		"nextVersion", nextVersion,
	)

	result := &Result{
		ComponentName:  componentName,
		CurrentRepo:    repository,
		CurrentVersion: currentVersion,
		NextRepo:       nextRepo,
		NextVersion:    nextVersion,
	}

	exists, err := c.repoExists(ctx, nextRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to check repo existence for %s: %w", nextRepo, err)
	}
	result.NextRepoExists = exists

	if !exists {
		logger.V(1).Info("next version repo does not exist", "component", componentName, "nextRepo", nextRepo)
		return result, nil
	}

	logger.Info("next version repo found", "component", componentName, "nextRepo", nextRepo, "nextVersion", nextVersion)
	result.UpgradeAvailable = true

	// Fetch latest version tag from the next repo
	latestTag, latestDate, err := c.getLatestVersionTag(ctx, nextRepo, imageConfig.Source.TagPattern)
	if err != nil {
		logger.V(1).Info("failed to fetch latest tag from next repo", "component", componentName, "error", err)
		// Non-fatal: repo exists but we couldn't list tags
		return result, nil
	}
	result.LatestTag = latestTag
	result.LatestTagDate = latestDate

	return result, nil
}

// repoExists checks if a Quay repository exists by calling the repository API.
func (c *Checker) repoExists(ctx context.Context, repository string) (bool, error) {
	url := fmt.Sprintf("%s/repository/%s", c.baseURL, repository)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to query Quay API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	logger, _ := logr.FromContext(ctx)

	// Quay returns 401 (not 404) for non-existent repos in the Konflux
	// redhat-user-workloads namespace, so treat both as "not found".
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
		logger.V(1).Info("repo not found", "repository", repository, "status", resp.StatusCode)
		return false, nil
	}

	logger.V(1).Info("repo existence check failed", "repository", repository, "status", resp.StatusCode)
	return false, fmt.Errorf("quay API returned unexpected status %d for %s", resp.StatusCode, repository)
}

// getLatestVersionTag fetches the most recent tag matching the given pattern from a Quay repo.
func (c *Checker) getLatestVersionTag(ctx context.Context, repository, tagPattern string) (string, string, error) {
	url := fmt.Sprintf("%s/repository/%s/tag/?limit=50&onlyActiveTags=true", c.baseURL, repository)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to query Quay API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("quay API returned status %d for %s", resp.StatusCode, repository)
	}

	var tagsResp quayTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return "", "", fmt.Errorf("failed to decode tags response: %w", err)
	}

	var pattern *regexp.Regexp
	if tagPattern != "" {
		pattern, err = regexp.Compile(tagPattern)
		if err != nil {
			return "", "", fmt.Errorf("invalid tag pattern %q: %w", tagPattern, err)
		}
	}

	// Find the latest matching tag (tags are returned newest-first by Quay)
	for _, tag := range tagsResp.Tags {
		if pattern != nil && !pattern.MatchString(tag.Name) {
			continue
		}
		return tag.Name, tag.LastModified, nil
	}

	return "", "", fmt.Errorf("no matching tags found in %s", repository)
}

// extractVersion extracts the "major.minor" version string from a repository name.
// For example, given prefix "acm-operator-bundle-acm-" and repo
// ".../acm-operator-bundle-acm-216", it returns "2.16".
func extractVersion(repository, prefix string) (string, error) {
	idx := strings.LastIndex(repository, prefix)
	if idx == -1 {
		return "", fmt.Errorf("prefix %q not found in repository %q", prefix, repository)
	}

	suffix := repository[idx+len(prefix):]
	match := repoVersionSuffix.FindStringSubmatch(suffix)
	if match == nil {
		return "", fmt.Errorf("cannot parse version suffix from %q in repository %q", suffix, repository)
	}

	major := match[1]
	minor := match[2]
	// Remove leading zero from minor if present (e.g., "11" stays "11", "06" becomes "6")
	minorInt, err := strconv.Atoi(minor)
	if err != nil {
		return "", fmt.Errorf("failed to parse minor version %q: %w", minor, err)
	}

	return fmt.Sprintf("%s.%d", major, minorInt), nil
}

// incrementMinorVersion takes a "major.minor" version string and returns the next minor version.
// For example, "2.16" returns "2.17".
func incrementMinorVersion(version string) (string, error) {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid version format %q: expected major.minor", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}

	return fmt.Sprintf("%d.%d", major, minor+1), nil
}

// buildNextRepo constructs the next version repository path.
// It replaces the version suffix in the current repo with the new version.
func buildNextRepo(currentRepo, prefix, nextVersion string) string {
	idx := strings.LastIndex(currentRepo, prefix)
	if idx == -1 {
		return currentRepo
	}

	// Build the new suffix: "2.17" -> "217"
	parts := strings.SplitN(nextVersion, ".", 2)
	if len(parts) != 2 {
		return currentRepo
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return currentRepo
	}

	newSuffix := fmt.Sprintf("%s%d", parts[0], minor)
	return currentRepo[:idx+len(prefix)] + newSuffix
}

// FormatResults formats the check-upgrade results.
// Supported formats: "table" (default, ASCII table), "markdown", "json".
func FormatResults(results []Result, format string) (string, error) {
	if len(results) == 0 {
		return "No components with repoVersionUpgrade configured.\n", nil
	}

	switch format {
	case "json":
		return formatResultsJSON(results)
	case "markdown":
		return formatResultsMarkdown(results), nil
	case "table", "":
		return formatResultsTable(results), nil
	default:
		return "", fmt.Errorf("unsupported output format %q: must be one of: table, markdown, json", format)
	}
}

func formatResultsMarkdown(results []Result) string {
	t := newResultsTable(results)
	return t.RenderMarkdown()
}

func formatResultsTable(results []Result) string {
	t := newResultsTable(results)
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = false
	t.Style().Options.DrawBorder = true
	return t.Render()
}

func newResultsTable(results []Result) table.Writer {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Component", "Current Version", "Next Version", "Status"})

	for _, r := range results {
		status := "⏳ Not available"
		if r.NextRepoExists {
			status = "✅ Available"
		}
		t.AppendRow(table.Row{r.ComponentName, r.CurrentVersion, r.NextVersion, status})
	}

	return t
}

func formatResultsJSON(results []Result) (string, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal results to JSON: %w", err)
	}
	return string(data), nil
}

// HasUpgrades returns true if any result indicates an available upgrade.
func HasUpgrades(results []Result) bool {
	for _, r := range results {
		if r.UpgradeAvailable {
			return true
		}
	}
	return false
}

// ApplyUpgrades updates repository references in both the image-updater config
// and the target config files for components with available upgrades.
// Target file paths are derived from the component targets in the config.
//
// In the image-updater config, it replaces the source.image field:
//
//	image: quay.io/.../acm-operator-bundle-acm-216 → acm-operator-bundle-acm-217
//
// In the target config, it replaces the repository field:
//
//	repository: .../acm-operator-bundle-acm-216 → acm-operator-bundle-acm-217
func ApplyUpgrades(results []Result, updaterConfigPath string, cfg *config.Config) error {
	// Build the set of old→new repo replacements
	var replacements []repoReplacement
	for _, r := range results {
		if !r.UpgradeAvailable {
			continue
		}
		replacements = append(replacements, repoReplacement{
			oldRepo: r.CurrentRepo,
			newRepo: r.NextRepo,
		})
	}

	if len(replacements) == 0 {
		return nil
	}

	// Update image-updater config (source.image contains the full registry/repo path)
	if err := applyReplacements(updaterConfigPath, replacements); err != nil {
		return fmt.Errorf("failed to update image-updater config %s: %w", updaterConfigPath, err)
	}

	// Collect unique target file paths from components with repo version upgrades
	targetFiles := make(map[string]bool)
	for _, imageConfig := range cfg.Images {
		if imageConfig.Source.RepoVersionUpgrade == nil {
			continue
		}
		for _, t := range imageConfig.Targets {
			if t.FilePath != "" {
				targetFiles[t.FilePath] = true
			}
		}
	}

	// Update each target config file
	for filePath := range targetFiles {
		if err := applyReplacements(filePath, replacements); err != nil {
			return fmt.Errorf("failed to update target config %s: %w", filePath, err)
		}
	}

	return nil
}

type repoReplacement struct {
	oldRepo string
	newRepo string
}

// applyReplacements performs string replacements in a file, preserving all formatting.
// It uses a temp-file + rename approach for atomic writes and preserves the original file mode.
func applyReplacements(filePath string, replacements []repoReplacement) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	modified := false

	for _, r := range replacements {
		if strings.Contains(content, r.oldRepo) {
			content = strings.ReplaceAll(content, r.oldRepo, r.newRepo)
			modified = true
		}
	}

	if !modified {
		return nil
	}

	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	tmpFile, err := os.CreateTemp(dir, base+".*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Chmod(info.Mode()); err != nil {
		return fmt.Errorf("failed to set file mode: %w", err)
	}

	tmpFile.Close()

	if err := os.Rename(tmpFile.Name(), filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
