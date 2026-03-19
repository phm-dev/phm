package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/phm-dev/phm/internal/httputil"
)

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// ComposerVersions represents the response from getcomposer.org/versions
type ComposerVersions struct {
	Stable []struct {
		Path    string `json:"path"`
		Version string `json:"version"`
		MinPHP  int    `json:"min-php"`
	} `json:"stable"`
}

// GetLatestGitHubRelease fetches the latest release from GitHub for a repo
func GetLatestGitHubRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	resp, err := httputil.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// GetComposerLatestVersion fetches the latest stable composer version
func GetComposerLatestVersion() (version string, downloadURL string, err error) {
	resp, err := httputil.Client.Get("https://getcomposer.org/versions")
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch composer versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("getcomposer.org returned status %d", resp.StatusCode)
	}

	var versions ComposerVersions
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", "", fmt.Errorf("failed to parse versions: %w", err)
	}

	if len(versions.Stable) == 0 {
		return "", "", fmt.Errorf("no stable version found")
	}

	latest := versions.Stable[0]
	downloadURL = "https://getcomposer.org" + latest.Path

	return latest.Version, downloadURL, nil
}

// GetBinaryLatestVersion returns the latest version and download URL for a binary tool from GitHub
func GetBinaryLatestVersion(tool *Tool, platform string) (version string, downloadURL string, err error) {
	if tool.GitHubRepo == "" {
		return "", "", fmt.Errorf("tool %s has no GitHub repo configured", tool.Name)
	}

	release, err := GetLatestGitHubRelease(tool.GitHubRepo)
	if err != nil {
		return "", "", err
	}

	version = strings.TrimPrefix(release.TagName, "v")

	assetName := tool.PlatformAssets[platform]
	if assetName == "" {
		return "", "", fmt.Errorf("no asset for platform %s", platform)
	}

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return version, asset.BrowserDownloadURL, nil
		}
	}

	return "", "", fmt.Errorf("asset %s not found in release %s", assetName, release.TagName)
}
