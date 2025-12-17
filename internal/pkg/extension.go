package pkg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ExtensionManager handles PHP extension management
type ExtensionManager struct {
	installPrefix string
}

// ExtensionStatus represents the status of an extension
type ExtensionStatus struct {
	Name       string
	Available  bool
	EnabledCLI bool
	EnabledFPM bool
	IniFile    string
}

// NewExtensionManager creates a new extension manager
func NewExtensionManager(installPrefix string) *ExtensionManager {
	return &ExtensionManager{
		installPrefix: installPrefix,
	}
}

// getModsDir returns the mods-available directory for a PHP version
func (e *ExtensionManager) getModsDir(version string) string {
	return filepath.Join(e.installPrefix, version, "etc", "mods-available")
}

// getConfDir returns the conf.d directory for a SAPI
func (e *ExtensionManager) getConfDir(version, sapi string) string {
	return filepath.Join(e.installPrefix, version, "etc", sapi, "conf.d")
}

// ListExtensions returns all available extensions and their status
func (e *ExtensionManager) ListExtensions(version string) ([]ExtensionStatus, error) {
	modsDir := e.getModsDir(version)

	entries, err := os.ReadDir(modsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("PHP %s is not installed or has no extensions", version)
		}
		return nil, err
	}

	var extensions []ExtensionStatus

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ini") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".ini")
		// Remove priority prefix if present (e.g., "10-opcache.ini" -> "opcache")
		if idx := strings.Index(name, "-"); idx > 0 && idx < 3 {
			name = name[idx+1:]
		}

		status := ExtensionStatus{
			Name:       name,
			Available:  true,
			IniFile:    entry.Name(),
			EnabledCLI: e.isEnabled(version, "cli", entry.Name()),
			EnabledFPM: e.isEnabled(version, "fpm", entry.Name()),
		}

		extensions = append(extensions, status)
	}

	sort.Slice(extensions, func(i, j int) bool {
		return extensions[i].Name < extensions[j].Name
	})

	return extensions, nil
}

// isEnabled checks if an extension is enabled for a SAPI
func (e *ExtensionManager) isEnabled(version, sapi, iniFile string) bool {
	confDir := e.getConfDir(version, sapi)
	linkPath := filepath.Join(confDir, iniFile)
	_, err := os.Lstat(linkPath)
	return err == nil
}

// Enable enables an extension for a SAPI
func (e *ExtensionManager) Enable(version, extension, sapi string) error {
	modsDir := e.getModsDir(version)

	// Find the ini file
	iniFile, err := e.findIniFile(modsDir, extension)
	if err != nil {
		return err
	}

	// Handle "all" sapi
	sapis := []string{sapi}
	if sapi == "all" {
		sapis = []string{"cli", "fpm"}
	}

	for _, s := range sapis {
		if err := e.enableForSapi(version, s, iniFile); err != nil {
			return err
		}
	}

	return nil
}

// enableForSapi enables an extension for a specific SAPI
func (e *ExtensionManager) enableForSapi(version, sapi, iniFile string) error {
	modsDir := e.getModsDir(version)
	confDir := e.getConfDir(version, sapi)

	// Ensure conf.d directory exists
	if err := os.MkdirAll(confDir, 0755); err != nil {
		cmd := exec.Command("sudo", "mkdir", "-p", confDir)
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("failed to create config dir %s: %v: %w", confDir, err, runErr)
		}
	}

	sourcePath := filepath.Join(modsDir, iniFile)
	linkPath := filepath.Join(confDir, iniFile)

	// Check if already enabled
	if _, err := os.Lstat(linkPath); err == nil {
		return nil // Already enabled
	}

	// Create symlink
	if err := os.Symlink(sourcePath, linkPath); err != nil {
		// Try with sudo
		cmd := exec.Command("sudo", "ln", "-sf", sourcePath, linkPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to enable %s for %s: %w", iniFile, sapi, err)
		}
	}

	return nil
}

// Disable disables an extension for a SAPI
func (e *ExtensionManager) Disable(version, extension, sapi string) error {
	modsDir := e.getModsDir(version)

	// Find the ini file
	iniFile, err := e.findIniFile(modsDir, extension)
	if err != nil {
		return err
	}

	// Handle "all" sapi
	sapis := []string{sapi}
	if sapi == "all" {
		sapis = []string{"cli", "fpm"}
	}

	for _, s := range sapis {
		if err := e.disableForSapi(version, s, iniFile); err != nil {
			return err
		}
	}

	return nil
}

// disableForSapi disables an extension for a specific SAPI
func (e *ExtensionManager) disableForSapi(version, sapi, iniFile string) error {
	confDir := e.getConfDir(version, sapi)
	linkPath := filepath.Join(confDir, iniFile)

	// Check if enabled
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		return nil // Already disabled
	}

	// Remove symlink
	if err := os.Remove(linkPath); err != nil {
		cmd := exec.Command("sudo", "rm", "-f", linkPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to disable %s for %s: %w", iniFile, sapi, err)
		}
	}

	return nil
}

// findIniFile finds the ini file for an extension in mods-available
func (e *ExtensionManager) findIniFile(modsDir, extension string) (string, error) {
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return "", err
	}

	// First try exact match
	exactMatch := extension + ".ini"
	for _, entry := range entries {
		if entry.Name() == exactMatch {
			return exactMatch, nil
		}
	}

	// Try with priority prefix (e.g., "10-opcache.ini")
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "-"+extension+".ini") {
			return name, nil
		}
	}

	return "", fmt.Errorf("extension '%s' not found in %s", extension, modsDir)
}

// GetInstalledVersions returns all PHP versions that have the extension structure
func (e *ExtensionManager) GetInstalledVersions() []string {
	var versions []string

	entries, err := os.ReadDir(e.installPrefix)
	if err != nil {
		return versions
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Check if it's a version directory
		if len(name) >= 3 && name[0] >= '0' && name[0] <= '9' {
			modsDir := e.getModsDir(name)
			if _, err := os.Stat(modsDir); err == nil {
				versions = append(versions, name)
			}
		}
	}

	return versions
}
