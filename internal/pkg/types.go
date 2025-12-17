package pkg

import "time"

// Package represents a PHP package
type Package struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Revision      int      `json:"revision"`
	PHPVersion    string   `json:"php_version,omitempty"`
	Description   string   `json:"description"`
	Platform      string   `json:"platform"`
	Depends       []string `json:"depends"`
	Conflicts     []string `json:"conflicts,omitempty"`
	Provides      []string `json:"provides"`
	InstalledSize int64    `json:"installed_size"`
	Maintainer    string   `json:"maintainer,omitempty"`
	URL           string   `json:"url,omitempty"`
	SHA256        string   `json:"sha256,omitempty"`
	Size          int64    `json:"size,omitempty"`
}

// InstalledPackage extends Package with installation info
type InstalledPackage struct {
	Package
	InstalledAt    time.Time `json:"installed_at"`
	InstalledFiles []string  `json:"installed_files"`
}

// Index represents the package index
type Index struct {
	Version   int                  `json:"version"`
	Generated string               `json:"generated"`
	Platforms map[string]*Platform `json:"platforms"`
}

// Platform contains packages for a specific platform
type Platform struct {
	Packages []Package `json:"packages"`
}

// PackageState represents the state of a package
type PackageState int

const (
	StateNotInstalled PackageState = iota
	StateInstalled
	StateUpgradable
)

// String returns string representation of PackageState
func (s PackageState) String() string {
	switch s {
	case StateInstalled:
		return "installed"
	case StateUpgradable:
		return "upgradable"
	default:
		return "not installed"
	}
}

// PackageInfo combines package data with its state
type PackageInfo struct {
	Package          Package
	State            PackageState
	InstalledVersion string
}
