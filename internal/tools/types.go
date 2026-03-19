package tools

import "time"

// ToolType represents the type of tool
type ToolType string

const (
	ToolTypePhar      ToolType = "phar"      // Phar installed via composer
	ToolTypeBinary    ToolType = "binary"    // Binary from GitHub releases
	ToolTypeBootstrap ToolType = "bootstrap" // Special case: composer itself
)

// Tool represents a tool definition
type Tool struct {
	Name        string   // Tool name (e.g., "composer")
	Description string   // Short description
	Type        ToolType // phar, binary, or bootstrap

	// For composer-based phars (ToolTypePhar)
	ComposerPkg  string // Packagist name (e.g., "phpstan/phpstan")
	PharInVendor string // Path to phar relative to vendor/ (e.g., "phpstan/phpstan/phpstan.phar")

	// For binaries from GitHub (ToolTypeBinary)
	GitHubRepo     string            // GitHub repo (e.g., "symfony-cli/symfony-cli")
	PlatformAssets map[string]string // Platform-specific asset names

	// For composer bootstrap (ToolTypeBootstrap)
	VersionURL string // URL to check version (getcomposer.org/versions)
}

// InstalledTool represents an installed tool
type InstalledTool struct {
	Name           string    `json:"name"`
	Version        string    `json:"version"`
	Type           ToolType  `json:"type"`
	InstalledAt    time.Time `json:"installed_at"`
	InstalledFiles []string  `json:"installed_files"`
	SourceURL      string    `json:"source_url"`
}
