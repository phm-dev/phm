package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Manager handles tool installation and management
type Manager struct {
	toolsPrefix string                    // /opt/phm/bin
	dataDir     string                    // ~/.local/share/phm/tools
	installed   map[string]*InstalledTool // name -> installed info
	platform    string                    // darwin-arm64 or darwin-amd64
	phpBin      string                    // /opt/php/bin/php
	composerBin string                    // /opt/phm/bin/composer
}

// NewManager creates a new tools manager
func NewManager(toolsPrefix, dataDir string) *Manager {
	arch := runtime.GOARCH
	platform := fmt.Sprintf("darwin-%s", arch)

	return &Manager{
		toolsPrefix: toolsPrefix,
		dataDir:     dataDir,
		installed:   make(map[string]*InstalledTool),
		platform:    platform,
		phpBin:      "/opt/php/bin/php",
		composerBin: filepath.Join(toolsPrefix, "composer"),
	}
}

// LoadInstalled loads the installed tools database
func (m *Manager) LoadInstalled() error {
	files, err := filepath.Glob(filepath.Join(m.dataDir, "*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var tool InstalledTool
		if err := json.Unmarshal(data, &tool); err != nil {
			continue
		}

		m.installed[tool.Name] = &tool
	}

	return nil
}

// saveInstalled saves an installed tool record
func (m *Manager) saveInstalled(tool *InstalledTool) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tool, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(m.dataDir, tool.Name+".json")
	return os.WriteFile(path, data, 0644)
}

// removeInstalled removes an installed tool record
func (m *Manager) removeInstalled(name string) error {
	path := filepath.Join(m.dataDir, name+".json")
	delete(m.installed, name)
	return os.Remove(path)
}

// IsInstalled checks if a tool is installed
func (m *Manager) IsInstalled(name string) bool {
	_, exists := m.installed[name]
	return exists
}

// GetInstalled returns installed tool info
func (m *Manager) GetInstalled(name string) *InstalledTool {
	return m.installed[name]
}

// GetAllInstalled returns all installed tools
func (m *Manager) GetAllInstalled() []*InstalledTool {
	var result []*InstalledTool
	for _, t := range m.installed {
		result = append(result, t)
	}
	return result
}

// Install installs a tool
func (m *Manager) Install(name string, force bool) error {
	tool := GetTool(name)
	if tool == nil {
		return fmt.Errorf("unknown tool: %s", name)
	}

	// Check if already installed
	if m.IsInstalled(name) && !force {
		return fmt.Errorf("tool %s is already installed (use --force to reinstall)", name)
	}

	// Ensure tools directory exists (may need sudo)
	if err := m.ensureToolsDir(); err != nil {
		return fmt.Errorf("failed to create tools directory: %w", err)
	}

	var err error
	switch tool.Type {
	case ToolTypeBootstrap:
		err = m.installComposer(tool, force)
	case ToolTypeBinary:
		err = m.installBinary(tool, force)
	case ToolTypePhar:
		err = m.installPharViaComposer(tool, force)
	default:
		return fmt.Errorf("unknown tool type: %s", tool.Type)
	}

	return err
}

// installComposer installs composer from getcomposer.org
func (m *Manager) installComposer(tool *Tool, force bool) error {
	fmt.Printf("\033[34m==>\033[0m Fetching latest version of composer...\n")

	version, downloadURL, err := GetComposerLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	fmt.Printf("    Latest version: %s\n", version)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "phm-composer-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pharPath := filepath.Join(tmpDir, "composer.phar")

	fmt.Printf("\033[34m==>\033[0m Downloading composer.phar...\n")

	if err := DownloadFile(pharPath, downloadURL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Install phar
	destPhar := filepath.Join(m.toolsPrefix, "composer.phar")
	if err := m.sudoCopy(pharPath, destPhar); err != nil {
		return fmt.Errorf("failed to install phar: %w", err)
	}

	// Create wrapper
	wrapperPath := filepath.Join(m.toolsPrefix, "composer")
	wrapperContent := m.createPharWrapper(destPhar)

	wrapperTmp := filepath.Join(tmpDir, "composer")
	if err := os.WriteFile(wrapperTmp, []byte(wrapperContent), 0755); err != nil {
		return fmt.Errorf("failed to create wrapper: %w", err)
	}

	if err := m.sudoCopy(wrapperTmp, wrapperPath); err != nil {
		return fmt.Errorf("failed to install wrapper: %w", err)
	}

	// Save installation record
	installed := &InstalledTool{
		Name:           "composer",
		Version:        version,
		Type:           ToolTypeBootstrap,
		InstalledAt:    time.Now(),
		InstalledFiles: []string{destPhar, wrapperPath},
		SourceURL:      downloadURL,
	}

	if err := m.saveInstalled(installed); err != nil {
		return fmt.Errorf("failed to save installation record: %w", err)
	}

	m.installed["composer"] = installed

	fmt.Printf("\033[32m[OK]\033[0m composer %s installed\n", version)
	fmt.Printf("    Path: %s\n", wrapperPath)

	return nil
}

// installBinary installs a binary tool from GitHub releases
func (m *Manager) installBinary(tool *Tool, force bool) error {
	fmt.Printf("\033[34m==>\033[0m Fetching latest version of %s...\n", tool.Name)

	version, downloadURL, err := GetBinaryLatestVersion(tool, m.platform)
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	fmt.Printf("    Latest version: %s\n", version)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "phm-tool-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Determine if archive or bare binary
	isArchive := strings.HasSuffix(downloadURL, ".tar.gz") || strings.HasSuffix(downloadURL, ".tgz")

	var binaryPath string

	if isArchive {
		archivePath := filepath.Join(tmpDir, "archive.tar.gz")

		fmt.Printf("\033[34m==>\033[0m Downloading archive...\n")

		if err := DownloadFile(archivePath, downloadURL); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		fmt.Printf("\033[34m==>\033[0m Extracting...\n")

		binaryPath, err = ExtractTarGz(archivePath, tmpDir, tool.Name)
		if err != nil {
			return fmt.Errorf("extraction failed: %w", err)
		}
	} else {
		// Bare binary download (like castor)
		binaryPath = filepath.Join(tmpDir, tool.Name)

		fmt.Printf("\033[34m==>\033[0m Downloading binary...\n")

		if err := DownloadFile(binaryPath, downloadURL); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if err := SetExecutable(binaryPath); err != nil {
			return fmt.Errorf("failed to set executable: %w", err)
		}
	}

	// Install binary
	destPath := filepath.Join(m.toolsPrefix, tool.Name)
	if err := m.sudoCopy(binaryPath, destPath); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	// Save installation record
	installed := &InstalledTool{
		Name:           tool.Name,
		Version:        version,
		Type:           ToolTypeBinary,
		InstalledAt:    time.Now(),
		InstalledFiles: []string{destPath},
		SourceURL:      downloadURL,
	}

	if err := m.saveInstalled(installed); err != nil {
		return fmt.Errorf("failed to save installation record: %w", err)
	}

	m.installed[tool.Name] = installed

	fmt.Printf("\033[32m[OK]\033[0m %s %s installed\n", tool.Name, version)
	fmt.Printf("    Path: %s\n", destPath)

	return nil
}

// installPharViaComposer installs a phar tool using composer require
func (m *Manager) installPharViaComposer(tool *Tool, force bool) error {
	// Check if composer is installed
	if !m.IsInstalled("composer") {
		// Check if composer exists on disk anyway
		if _, err := os.Stat(m.composerBin); os.IsNotExist(err) {
			return fmt.Errorf("composer is required to install %s. Run: phm install composer", tool.Name)
		}
	}

	// Check if PHP is installed
	if _, err := os.Stat(m.phpBin); os.IsNotExist(err) {
		return fmt.Errorf("PHP is required to install %s. Run: phm install php8.5-cli", tool.Name)
	}

	fmt.Printf("\033[34m==>\033[0m Installing %s via composer...\n", tool.Name)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "phm-composer-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Run composer require
	fmt.Printf("    Running: composer require %s\n", tool.ComposerPkg)

	cmd := exec.Command(m.composerBin, "require", tool.ComposerPkg, "--no-interaction", "--no-progress")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "COMPOSER_HOME="+tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("composer require failed: %w\n%s", err, string(output))
	}

	// Find the phar/script in vendor
	pharPath := filepath.Join(tmpDir, "vendor", tool.PharInVendor)
	if _, err := os.Stat(pharPath); os.IsNotExist(err) {
		return fmt.Errorf("expected file not found: vendor/%s", tool.PharInVendor)
	}

	// Get version from composer.lock
	version := m.getVersionFromComposerLock(tmpDir, tool.ComposerPkg)
	if version == "" {
		version = "unknown"
	}

	// Determine destination paths
	var destPath string
	var installedFiles []string

	if strings.HasSuffix(tool.PharInVendor, ".phar") {
		// It's a phar - copy it and create wrapper
		destPhar := filepath.Join(m.toolsPrefix, tool.Name+".phar")
		if err := m.sudoCopy(pharPath, destPhar); err != nil {
			return fmt.Errorf("failed to install phar: %w", err)
		}

		wrapperPath := filepath.Join(m.toolsPrefix, tool.Name)
		wrapperContent := m.createPharWrapper(destPhar)

		wrapperTmp := filepath.Join(tmpDir, tool.Name)
		if err := os.WriteFile(wrapperTmp, []byte(wrapperContent), 0755); err != nil {
			return fmt.Errorf("failed to create wrapper: %w", err)
		}

		if err := m.sudoCopy(wrapperTmp, wrapperPath); err != nil {
			return fmt.Errorf("failed to install wrapper: %w", err)
		}

		destPath = wrapperPath
		installedFiles = []string{destPhar, wrapperPath}
	} else {
		// It's a PHP script - copy and create wrapper
		destScript := filepath.Join(m.toolsPrefix, tool.Name+".php")
		if err := m.sudoCopy(pharPath, destScript); err != nil {
			return fmt.Errorf("failed to install script: %w", err)
		}

		wrapperPath := filepath.Join(m.toolsPrefix, tool.Name)
		wrapperContent := m.createPharWrapper(destScript)

		wrapperTmp := filepath.Join(tmpDir, tool.Name)
		if err := os.WriteFile(wrapperTmp, []byte(wrapperContent), 0755); err != nil {
			return fmt.Errorf("failed to create wrapper: %w", err)
		}

		if err := m.sudoCopy(wrapperTmp, wrapperPath); err != nil {
			return fmt.Errorf("failed to install wrapper: %w", err)
		}

		destPath = wrapperPath
		installedFiles = []string{destScript, wrapperPath}
	}

	// Save installation record
	installed := &InstalledTool{
		Name:           tool.Name,
		Version:        version,
		Type:           ToolTypePhar,
		InstalledAt:    time.Now(),
		InstalledFiles: installedFiles,
		SourceURL:      "composer:" + tool.ComposerPkg,
	}

	if err := m.saveInstalled(installed); err != nil {
		return fmt.Errorf("failed to save installation record: %w", err)
	}

	m.installed[tool.Name] = installed

	fmt.Printf("\033[32m[OK]\033[0m %s %s installed\n", tool.Name, version)
	fmt.Printf("    Path: %s\n", destPath)

	return nil
}

// getVersionFromComposerLock extracts package version from composer.lock
func (m *Manager) getVersionFromComposerLock(dir, pkg string) string {
	lockPath := filepath.Join(dir, "composer.lock")
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return ""
	}

	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}

	if err := json.Unmarshal(data, &lock); err != nil {
		return ""
	}

	for _, p := range lock.Packages {
		if p.Name == pkg {
			return strings.TrimPrefix(p.Version, "v")
		}
	}

	return ""
}

// createPharWrapper creates a wrapper script that uses PHM's PHP
func (m *Manager) createPharWrapper(pharPath string) string {
	return fmt.Sprintf(`#!/bin/sh
exec %s "%s" "$@"
`, m.phpBin, pharPath)
}

// ensureToolsDir creates the tools directory if it doesn't exist
func (m *Manager) ensureToolsDir() error {
	if _, err := os.Stat(m.toolsPrefix); err == nil {
		return nil
	}

	// Need to create with sudo
	cmd := exec.Command("sudo", "mkdir", "-p", m.toolsPrefix)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// sudoCopy copies a file using sudo
func (m *Manager) sudoCopy(src, dest string) error {
	cmd := exec.Command("sudo", "cp", src, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Set permissions
	cmd = exec.Command("sudo", "chmod", "755", dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// sudoRemove removes a file using sudo
func (m *Manager) sudoRemove(path string) error {
	cmd := exec.Command("sudo", "rm", "-f", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Remove removes an installed tool
func (m *Manager) Remove(name string) error {
	installed := m.GetInstalled(name)
	if installed == nil {
		return fmt.Errorf("tool %s is not installed", name)
	}

	fmt.Printf("\033[34m==>\033[0m Removing %s...\n", name)

	// Remove installed files
	for _, file := range installed.InstalledFiles {
		if err := m.sudoRemove(file); err != nil {
			fmt.Printf("\033[33mWarning:\033[0m Could not remove %s: %v\n", file, err)
		}
	}

	// Remove installation record
	if err := m.removeInstalled(name); err != nil {
		return fmt.Errorf("failed to remove installation record: %w", err)
	}

	fmt.Printf("\033[32m[OK]\033[0m %s removed\n", name)
	return nil
}

// Upgrade upgrades an installed tool to the latest version
func (m *Manager) Upgrade(name string) error {
	installed := m.GetInstalled(name)
	if installed == nil {
		return fmt.Errorf("tool %s is not installed", name)
	}

	tool := GetTool(name)
	if tool == nil {
		return fmt.Errorf("unknown tool: %s", name)
	}

	fmt.Printf("\033[34m==>\033[0m Checking for updates to %s...\n", name)

	// Get current version
	currentVersion := installed.Version

	// Check latest version based on type
	var latestVersion string
	var err error

	switch tool.Type {
	case ToolTypeBootstrap:
		latestVersion, _, err = GetComposerLatestVersion()
	case ToolTypeBinary:
		latestVersion, _, err = GetBinaryLatestVersion(tool, m.platform)
	case ToolTypePhar:
		// For composer-based tools, we need to check packagist
		// For now, just force reinstall
		fmt.Printf("    Reinstalling to get latest version...\n")
		return m.Install(name, true)
	}

	if err != nil {
		return fmt.Errorf("failed to check latest version: %w", err)
	}

	if latestVersion == currentVersion {
		fmt.Printf("\033[32m[OK]\033[0m %s is already up-to-date (%s)\n", name, currentVersion)
		return nil
	}

	fmt.Printf("    Upgrade available: %s -> %s\n", currentVersion, latestVersion)

	// Force reinstall
	return m.Install(name, true)
}

// CheckUpgrade checks if a tool has an available upgrade
func (m *Manager) CheckUpgrade(name string) (currentVersion, latestVersion string, hasUpgrade bool, err error) {
	installed := m.GetInstalled(name)
	if installed == nil {
		return "", "", false, fmt.Errorf("tool %s is not installed", name)
	}

	tool := GetTool(name)
	if tool == nil {
		return installed.Version, "", false, fmt.Errorf("unknown tool: %s", name)
	}

	switch tool.Type {
	case ToolTypeBootstrap:
		latestVersion, _, err = GetComposerLatestVersion()
	case ToolTypeBinary:
		latestVersion, _, err = GetBinaryLatestVersion(tool, m.platform)
	case ToolTypePhar:
		// Can't easily check without running composer
		return installed.Version, "", false, nil
	}

	if err != nil {
		return installed.Version, "", false, err
	}

	return installed.Version, latestVersion, latestVersion != installed.Version, nil
}
