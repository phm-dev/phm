package pkg

import (
	"regexp"
	"time"
)

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
	// InstallSlot is the directory where this package is installed (e.g., "8.5" or "8.5.1")
	// For minor version installs (php8.5-cli), this is "8.5"
	// For pinned version installs (php8.5.1-cli), this is "8.5.1"
	InstallSlot string `json:"install_slot,omitempty"`
	// Pinned indicates if this package is pinned to a specific patch version
	// Pinned packages are not upgraded by `phm upgrade`
	Pinned bool `json:"pinned,omitempty"`
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

// VersionInfo contains parsed version information from a package name
type VersionInfo struct {
	// MinorVersion is the minor version (e.g., "8.5" from "php8.5-cli" or "php8.5.1-cli")
	MinorVersion string
	// PatchVersion is the full patch version if specified (e.g., "8.5.1" from "php8.5.1-cli")
	// Empty for minor version requests like "php8.5-cli"
	PatchVersion string
	// IsPinned is true if a specific patch version was requested
	IsPinned bool
	// PackageType is the package suffix (e.g., "cli", "fpm", "redis")
	PackageType string
}

var (
	// Matches php8.5.1-cli (pinned patch version)
	patchVersionRegex = regexp.MustCompile(`^php(\d+)\.(\d+)\.(\d+)-(.+)$`)
	// Matches php8.5-cli (minor version, tracks latest)
	minorVersionRegex = regexp.MustCompile(`^php(\d+)\.(\d+)-(.+)$`)
)

// ParsePackageName extracts version information from a package name
// Examples:
//   - "php8.5-cli" -> MinorVersion: "8.5", PatchVersion: "", IsPinned: false, PackageType: "cli"
//   - "php8.5.1-cli" -> MinorVersion: "8.5", PatchVersion: "8.5.1", IsPinned: true, PackageType: "cli"
func ParsePackageName(name string) *VersionInfo {
	// Try patch version first (more specific)
	if matches := patchVersionRegex.FindStringSubmatch(name); len(matches) == 5 {
		return &VersionInfo{
			MinorVersion: matches[1] + "." + matches[2],
			PatchVersion: matches[1] + "." + matches[2] + "." + matches[3],
			IsPinned:     true,
			PackageType:  matches[4],
		}
	}

	// Try minor version
	if matches := minorVersionRegex.FindStringSubmatch(name); len(matches) == 4 {
		return &VersionInfo{
			MinorVersion: matches[1] + "." + matches[2],
			PatchVersion: "",
			IsPinned:     false,
			PackageType:  matches[3],
		}
	}

	return nil
}

// GetInstallSlot returns the installation directory slot for a package
// For minor version packages (php8.5-cli), returns "8.5"
// For pinned packages (php8.5.1-cli), returns "8.5.1"
func (v *VersionInfo) GetInstallSlot() string {
	if v.IsPinned {
		return v.PatchVersion
	}
	return v.MinorVersion
}

// GetCanonicalName returns the canonical package name for index lookup
// Pinned packages like "php8.5.1-cli" map to "php8.5-cli" in the index
func (v *VersionInfo) GetCanonicalName() string {
	return "php" + v.MinorVersion + "-" + v.PackageType
}
