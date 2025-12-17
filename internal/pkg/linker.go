package pkg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Linker handles PHP version linking and symlinks
type Linker struct {
	installPrefix  string // /opt/php
	phmBinDir      string // /opt/php/bin (PHM-managed, safe)
	systemBinDir   string // /usr/local/bin (shared with Homebrew)
	symfonyBinDir  string // /opt/local/bin (Symfony CLI MacPorts detection)
	symfonySbinDir string // /opt/local/sbin (Symfony CLI MacPorts detection)
}

// HomebrewConflict represents a detected Homebrew conflict
type HomebrewConflict struct {
	Binary string // e.g., "php"
	Path   string // e.g., "/usr/local/bin/php"
	Target string // e.g., "/usr/local/opt/php@8.3/bin/php"
}

// NewLinker creates a new linker
func NewLinker(installPrefix string) *Linker {
	return &Linker{
		installPrefix:  installPrefix,
		phmBinDir:      filepath.Join(installPrefix, "bin"),
		systemBinDir:   "/usr/local/bin",
		symfonyBinDir:  "/opt/local/bin",
		symfonySbinDir: "/opt/local/sbin",
	}
}

// GetPHMBinDir returns the PHM bin directory path
func (l *Linker) GetPHMBinDir() string {
	return l.phmBinDir
}

// getBinaries returns the list of PHP binaries to link
func (l *Linker) getBinaries() []string {
	return []string{"php", "phpize", "php-config", "phar", "pecl", "pear", "php-fpm"}
}

// getSourcePath returns the source path for a binary
func (l *Linker) getSourcePath(version, binary string) string {
	if binary == "php-fpm" {
		return filepath.Join(l.installPrefix, version, "sbin", binary)
	}
	return filepath.Join(l.installPrefix, version, "bin", binary)
}

// SetupVersionLinks creates version-specific symlinks (php8.5, php8.4, etc.) in /opt/php/bin
// and Symfony CLI compatible symlinks in /opt/local/bin (php85, php84, etc.)
func (l *Linker) SetupVersionLinks(version string) error {
	phpBin := filepath.Join(l.installPrefix, version, "bin", "php")

	// Check if PHP binary exists
	if _, err := os.Stat(phpBin); os.IsNotExist(err) {
		return fmt.Errorf("PHP %s not installed at %s", version, phpBin)
	}

	// Ensure PHM bin directory exists
	if err := os.MkdirAll(l.phmBinDir, 0755); err != nil {
		cmd := exec.Command("sudo", "mkdir", "-p", l.phmBinDir)
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("failed to create PHM bin dir %s: %v: %w", l.phmBinDir, err, runErr)
		}
	}

	// Create version-specific symlinks in /opt/php/bin (e.g., php8.5, phpize8.5)
	for _, binary := range l.getBinaries() {
		source := l.getSourcePath(version, binary)
		target := filepath.Join(l.phmBinDir, binary+version)

		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		}

		if err := l.createSymlink(source, target); err != nil {
			return err
		}
	}

	// Also create Symfony CLI compatible symlinks in /opt/local
	if err := l.setupSymfonyLinksInternal(version); err != nil {
		return err
	}

	return nil
}

// setupSymfonyLinksInternal creates symlinks in /opt/local for Symfony CLI MacPorts-style detection
func (l *Linker) setupSymfonyLinksInternal(version string) error {
	// Convert version "8.5" to MacPorts style "85"
	macportsVersion := strings.ReplaceAll(version, ".", "")

	// Ensure directories exist
	for _, dir := range []string{l.symfonyBinDir, l.symfonySbinDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			cmd := exec.Command("sudo", "mkdir", "-p", dir)
			if runErr := cmd.Run(); runErr != nil {
				return fmt.Errorf("failed to create Symfony dir %s: %v: %w", dir, err, runErr)
			}
		}
	}

	// Create /opt/local/bin/php85 -> /opt/php/8.5/bin/php
	phpSource := filepath.Join(l.installPrefix, version, "bin", "php")
	phpTarget := filepath.Join(l.symfonyBinDir, "php"+macportsVersion)
	if _, err := os.Stat(phpSource); err == nil {
		if err := l.createSymlink(phpSource, phpTarget); err != nil {
			return err
		}
	}

	// Create /opt/local/sbin/php-fpm85 -> /opt/php/8.5/sbin/php-fpm
	fpmSource := filepath.Join(l.installPrefix, version, "sbin", "php-fpm")
	fpmTarget := filepath.Join(l.symfonySbinDir, "php-fpm"+macportsVersion)
	if _, err := os.Stat(fpmSource); err == nil {
		if err := l.createSymlink(fpmSource, fpmTarget); err != nil {
			return err
		}
	}

	return nil
}

// removeSymfonyLinksInternal removes Symfony CLI symlinks for a PHP version
func (l *Linker) removeSymfonyLinksInternal(version string) error {
	macportsVersion := strings.ReplaceAll(version, ".", "")

	// Remove /opt/local/bin/php85
	phpTarget := filepath.Join(l.symfonyBinDir, "php"+macportsVersion)
	if err := l.removePath(phpTarget); err != nil {
		return err
	}

	// Remove /opt/local/sbin/php-fpm85
	fpmTarget := filepath.Join(l.symfonySbinDir, "php-fpm"+macportsVersion)
	if err := l.removePath(fpmTarget); err != nil {
		return err
	}

	return nil
}

// SetDefaultVersion sets the default PHP version (creates php, phpize, etc. symlinks in /opt/php/bin)
func (l *Linker) SetDefaultVersion(version string) error {
	phpBin := filepath.Join(l.installPrefix, version, "bin", "php")

	// Check if PHP binary exists
	if _, err := os.Stat(phpBin); os.IsNotExist(err) {
		return fmt.Errorf("PHP %s not installed at %s", version, phpBin)
	}

	// Ensure PHM bin directory exists
	if err := os.MkdirAll(l.phmBinDir, 0755); err != nil {
		cmd := exec.Command("sudo", "mkdir", "-p", l.phmBinDir)
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("failed to create PHM bin dir %s: %v: %w", l.phmBinDir, err, runErr)
		}
	}

	// Create default symlinks in /opt/php/bin (e.g., php, phpize)
	for _, binary := range l.getBinaries() {
		source := l.getSourcePath(version, binary)
		target := filepath.Join(l.phmBinDir, binary)

		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		}

		if err := l.createSymlink(source, target); err != nil {
			return err
		}
	}

	// Save current version to a file
	versionFile := filepath.Join(l.installPrefix, ".current")
	if err := os.WriteFile(versionFile, []byte(version), 0644); err != nil {
		cmd := exec.Command("sudo", "sh", "-c", fmt.Sprintf("echo '%s' > %s", version, versionFile))
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("failed to write version file %s: %v: %w", versionFile, err, runErr)
		}
	}

	return nil
}

// SetSystemDefault creates symlinks in /usr/local/bin (use with caution - may conflict with Homebrew)
func (l *Linker) SetSystemDefault(version string) error {
	phpBin := filepath.Join(l.installPrefix, version, "bin", "php")

	// Check if PHP binary exists
	if _, err := os.Stat(phpBin); os.IsNotExist(err) {
		return fmt.Errorf("PHP %s not installed at %s", version, phpBin)
	}

	// Create symlinks in /usr/local/bin
	for _, binary := range l.getBinaries() {
		source := l.getSourcePath(version, binary)
		target := filepath.Join(l.systemBinDir, binary)

		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		}

		if err := l.createSymlink(source, target); err != nil {
			return err
		}
	}

	return nil
}

// RemoveSystemLinks removes PHM symlinks from /usr/local/bin
func (l *Linker) RemoveSystemLinks() error {
	for _, binary := range l.getBinaries() {
		target := filepath.Join(l.systemBinDir, binary)

		// Only remove if it's a symlink pointing to our installation
		linkTarget, err := os.Readlink(target)
		if err != nil {
			continue // Not a symlink or doesn't exist
		}

		if strings.HasPrefix(linkTarget, l.installPrefix) {
			if err := l.removePath(target); err != nil {
				return err
			}
		}
	}

	return nil
}

// DetectHomebrewConflicts checks for existing Homebrew PHP installations
func (l *Linker) DetectHomebrewConflicts() []HomebrewConflict {
	var conflicts []HomebrewConflict

	for _, binary := range l.getBinaries() {
		target := filepath.Join(l.systemBinDir, binary)

		linkTarget, err := os.Readlink(target)
		if err != nil {
			// Check if it's a regular file (not a symlink)
			if _, statErr := os.Stat(target); statErr == nil {
				// File exists but is not a symlink - could be Homebrew
				conflicts = append(conflicts, HomebrewConflict{
					Binary: binary,
					Path:   target,
					Target: "(regular file)",
				})
			}
			continue
		}

		// Check if symlink points to Homebrew
		if strings.Contains(linkTarget, "/usr/local/opt/") ||
			strings.Contains(linkTarget, "/usr/local/Cellar/") ||
			strings.Contains(linkTarget, "/opt/homebrew/") {
			conflicts = append(conflicts, HomebrewConflict{
				Binary: binary,
				Path:   target,
				Target: linkTarget,
			})
		}
	}

	return conflicts
}

// IsSystemLinked checks if PHM has symlinks in /usr/local/bin
func (l *Linker) IsSystemLinked() bool {
	phpTarget := filepath.Join(l.systemBinDir, "php")
	linkTarget, err := os.Readlink(phpTarget)
	if err != nil {
		return false
	}
	return strings.HasPrefix(linkTarget, l.installPrefix)
}

// GetDefaultVersion returns the currently set default PHP version
func (l *Linker) GetDefaultVersion() string {
	versionFile := filepath.Join(l.installPrefix, ".current")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// GetAvailableVersions returns all installed PHP versions
func (l *Linker) GetAvailableVersions() []string {
	var versions []string

	entries, err := os.ReadDir(l.installPrefix)
	if err != nil {
		return versions
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Check if it's a version directory (e.g., "8.5", "8.4")
		if len(name) >= 3 && name[0] >= '0' && name[0] <= '9' {
			phpBin := filepath.Join(l.installPrefix, name, "bin", "php")
			if _, err := os.Stat(phpBin); err == nil {
				versions = append(versions, name)
			}
		}
	}

	return versions
}

// RemoveVersionLinks removes version-specific symlinks from /opt/php/bin
func (l *Linker) RemoveVersionLinks(version string) error {
	for _, binary := range l.getBinaries() {
		target := filepath.Join(l.phmBinDir, binary+version)
		if err := l.removePath(target); err != nil {
			return err
		}
	}

	// Also remove Symfony CLI symlinks
	if err := l.removeSymfonyLinksInternal(version); err != nil {
		return err
	}

	return nil
}

// removePath removes a file or symlink, falling back to sudo if needed.
func (l *Linker) removePath(target string) error {
	if err := os.Remove(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		cmd := exec.Command("sudo", "rm", "-f", target)
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("failed to remove %s: %w", target, runErr)
		}
	}
	return nil
}

// createSymlink creates a symlink, using sudo if necessary
func (l *Linker) createSymlink(source, target string) error {
	// Remove existing symlink/file
	if err := l.removePath(target); err != nil {
		return err
	}

	// Create new symlink
	if err := os.Symlink(source, target); err != nil {
		cmd := exec.Command("sudo", "ln", "-sf", source, target)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create symlink %s: %w", target, err)
		}
	}

	return nil
}
