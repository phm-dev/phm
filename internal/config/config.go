package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Config holds PHM configuration
type Config struct {
	// Mode flags
	Offline bool
	Debug   bool

	// Paths
	RepoURL       string // Online repository URL
	RepoPath      string // Local repository path (offline mode)
	InstallPrefix string
	CacheDir      string
	DataDir       string
	ConfigDir     string
}

// New creates a new Config with default values
func New() *Config {
	homeDir, _ := os.UserHomeDir()

	cfg := &Config{
		Offline:       false,
		Debug:         false,
		RepoURL:       "https://raw.githubusercontent.com/phm-dev/php-packages/main",
		RepoPath:      "",
		InstallPrefix: "/opt/php",
		CacheDir:      filepath.Join(homeDir, ".cache", "phm"),
		DataDir:       filepath.Join(homeDir, ".local", "share", "phm"),
		ConfigDir:     filepath.Join(homeDir, ".config", "phm"),
	}

	return cfg
}

// GetRepoURL returns the repository URL based on mode
func (c *Config) GetRepoURL() string {
	if c.Offline || c.RepoPath != "" {
		if c.RepoPath != "" {
			return "file://" + c.RepoPath
		}
		return "file://./dist"
	}
	return c.RepoURL
}

// GetIndexPath returns the path to index.json
func (c *Config) GetIndexPath() string {
	if c.Offline || c.RepoPath != "" {
		path := c.RepoPath
		if path == "" {
			path = "./dist"
		}
		return filepath.Join(path, "index.json")
	}
	return filepath.Join(c.CacheDir, "index.json")
}

// GetPackagePath returns the path to a package file
func (c *Config) GetPackagePath(filename string) string {
	if c.Offline || c.RepoPath != "" {
		path := c.RepoPath
		if path == "" {
			path = "./dist"
		}
		return filepath.Join(path, filename)
	}
	return filepath.Join(c.CacheDir, "packages", filename)
}

// Platform returns the current platform identifier
func (c *Config) Platform() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Normalize arch names
	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	}

	return os + "-" + arch
}

// EnsureDirs creates necessary directories
func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.CacheDir,
		filepath.Join(c.CacheDir, "packages"),
		c.DataDir,
		filepath.Join(c.DataDir, "installed"),
		c.ConfigDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// InstalledDBPath returns path to installed packages database
func (c *Config) InstalledDBPath() string {
	return filepath.Join(c.DataDir, "installed")
}
