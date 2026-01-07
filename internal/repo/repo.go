package repo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phm-dev/phm/internal/config"
	"github.com/phm-dev/phm/internal/pkg"
	"github.com/schollz/progressbar/v3"
)

// Repository handles package index and downloads
type Repository struct {
	cfg   *config.Config
	index *pkg.Index
}

// New creates a new Repository
func New(cfg *config.Config) *Repository {
	return &Repository{
		cfg: cfg,
	}
}

// LoadIndex loads the package index
func (r *Repository) LoadIndex() error {
	indexPath := r.cfg.GetIndexPath()

	// Check if it's a local file
	if r.cfg.Offline || r.cfg.RepoPath != "" || strings.HasPrefix(indexPath, "/") {
		return r.loadLocalIndex(indexPath)
	}

	// Always fetch fresh index from remote
	return r.FetchIndex()
}

// loadLocalIndex loads index from a local file
func (r *Repository) loadLocalIndex(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read index: %w", err)
	}

	var index pkg.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("failed to parse index: %w", err)
	}

	r.index = &index
	return nil
}

// FetchIndex fetches the index from remote repository
func (r *Repository) FetchIndex() error {
	if r.cfg.Offline {
		return r.loadLocalIndex(r.cfg.GetIndexPath())
	}

	// Add timestamp to bypass GitHub's CDN cache
	url := fmt.Sprintf("%s/index.json?t=%d", r.cfg.RepoURL, time.Now().Unix())

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch index: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read index: %w", err)
	}

	var index pkg.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("failed to parse index: %w", err)
	}

	r.index = &index

	// Cache the index
	cacheDir := r.cfg.CacheDir
	if err := os.MkdirAll(cacheDir, 0755); err == nil {
		cachePath := filepath.Join(cacheDir, "index.json")
		_ = os.WriteFile(cachePath, data, 0644)
	}

	return nil
}

// GetIndex returns the loaded index
func (r *Repository) GetIndex() *pkg.Index {
	return r.index
}

// GetPackages returns all packages for current platform
func (r *Repository) GetPackages() []pkg.Package {
	if r.index == nil {
		return nil
	}

	platform := r.cfg.Platform()
	if p, ok := r.index.Platforms[platform]; ok {
		return p.Packages
	}

	return nil
}

// GetPackage returns a specific package by name (latest version)
func (r *Repository) GetPackage(name string) *pkg.Package {
	packages := r.GetPackages()
	var latest *pkg.Package
	for i := range packages {
		if packages[i].Name == name {
			if latest == nil || pkg.CompareVersions(packages[i].Version, latest.Version) > 0 {
				latest = &packages[i]
			}
		}
	}
	return latest
}

// SearchPackages searches packages by query
func (r *Repository) SearchPackages(query string) []pkg.Package {
	var results []pkg.Package
	packages := r.GetPackages()

	query = strings.ToLower(query)
	for _, p := range packages {
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Description), query) {
			results = append(results, p)
		}
	}

	return results
}

// DownloadPackage downloads a package to cache with progress bar
func (r *Repository) DownloadPackage(p *pkg.Package) (string, error) {
	filename := fmt.Sprintf("%s_%s-%d_%s.tar.zst", p.Name, p.Version, p.Revision, p.Platform)

	// In offline mode, return local path
	if r.cfg.Offline || r.cfg.RepoPath != "" {
		localPath := r.cfg.GetPackagePath(filename)
		if _, err := os.Stat(localPath); err != nil {
			return "", fmt.Errorf("package not found: %s", localPath)
		}
		return localPath, nil
	}

	// Check cache
	cachePath := r.cfg.GetPackagePath(filename)
	if _, err := os.Stat(cachePath); err == nil {
		// TODO: verify checksum
		return cachePath, nil
	}

	// Download
	url := p.URL
	if url == "" {
		url = r.cfg.RepoURL + "/" + filename
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return "", err
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(cachePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Create progress bar
	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription("    Downloading"),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
	)

	// Copy with progress
	if _, err := io.Copy(io.MultiWriter(out, bar), resp.Body); err != nil {
		os.Remove(cachePath)
		return "", err
	}

	return cachePath, nil
}
