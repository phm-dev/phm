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
	Name    string
	Enabled bool
	IniFile string
}

// NewExtensionManager creates a new extension manager
func NewExtensionManager(installPrefix string) *ExtensionManager {
	return &ExtensionManager{
		installPrefix: installPrefix,
	}
}

// getConfDir returns the conf.d directory for a PHP version
// PHP scans this directory for additional ini files
func (e *ExtensionManager) getConfDir(version string) string {
	return filepath.Join(e.installPrefix, version, "etc", "conf.d")
}

// getExtensionDir returns the directory where .so files are stored
func (e *ExtensionManager) getExtensionDir(version string) string {
	// Find the actual extension directory
	baseDir := filepath.Join(e.installPrefix, version, "lib", "php", "extensions")
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "no-debug-") {
			return filepath.Join(baseDir, entry.Name())
		}
	}
	return baseDir
}

// ListExtensions returns all extensions and their status
func (e *ExtensionManager) ListExtensions(version string) ([]ExtensionStatus, error) {
	confDir := e.getConfDir(version)
	extDir := e.getExtensionDir(version)

	if extDir == "" {
		return nil, fmt.Errorf("PHP %s extension directory not found", version)
	}

	// Get all .so files from extension directory
	soFiles, err := os.ReadDir(extDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read extension directory: %w", err)
	}

	// Get enabled extensions from conf.d
	enabledExts := make(map[string]string) // extension name -> ini file
	if entries, err := os.ReadDir(confDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ini") {
				extName := e.extractExtensionName(entry.Name())
				enabledExts[extName] = entry.Name()
			}
		}
	}

	var extensions []ExtensionStatus
	seen := make(map[string]bool)

	// List all available .so extensions
	for _, entry := range soFiles {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".so")
		if seen[name] {
			continue
		}
		seen[name] = true

		iniFile, enabled := enabledExts[name]
		extensions = append(extensions, ExtensionStatus{
			Name:    name,
			Enabled: enabled,
			IniFile: iniFile,
		})
	}

	// Also add any enabled extensions that might not have .so visible
	for extName, iniFile := range enabledExts {
		if !seen[extName] {
			extensions = append(extensions, ExtensionStatus{
				Name:    extName,
				Enabled: true,
				IniFile: iniFile,
			})
		}
	}

	sort.Slice(extensions, func(i, j int) bool {
		return extensions[i].Name < extensions[j].Name
	})

	return extensions, nil
}

// extractExtensionName extracts extension name from ini filename
// e.g., "20-redis.ini" -> "redis", "opcache.ini" -> "opcache"
func (e *ExtensionManager) extractExtensionName(filename string) string {
	name := strings.TrimSuffix(filename, ".ini")
	// Remove priority prefix if present (e.g., "20-redis" -> "redis")
	if idx := strings.Index(name, "-"); idx > 0 && idx <= 2 {
		name = name[idx+1:]
	}
	return name
}

// Enable enables an extension
func (e *ExtensionManager) Enable(version, extension, sapi string) error {
	confDir := e.getConfDir(version)
	extDir := e.getExtensionDir(version)

	// Check if .so file exists
	soPath := filepath.Join(extDir, extension+".so")
	if _, err := os.Stat(soPath); os.IsNotExist(err) {
		return fmt.Errorf("extension '%s' not found (no %s.so in %s)", extension, extension, extDir)
	}

	// Ensure conf.d directory exists
	if err := os.MkdirAll(confDir, 0755); err != nil {
		cmd := exec.Command("sudo", "mkdir", "-p", confDir)
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("failed to create config dir: %w", runErr)
		}
	}

	// Check if already enabled
	iniFile := "20-" + extension + ".ini"
	iniPath := filepath.Join(confDir, iniFile)
	if _, err := os.Stat(iniPath); err == nil {
		return nil // Already enabled
	}

	// Create ini file
	var content string
	if extension == "opcache" {
		content = "zend_extension=opcache.so\n"
	} else {
		content = fmt.Sprintf("extension=%s.so\n", extension)
	}

	if err := os.WriteFile(iniPath, []byte(content), 0644); err != nil {
		// Try with sudo
		tmpFile := filepath.Join(os.TempDir(), iniFile)
		if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create ini file: %w", err)
		}
		cmd := exec.Command("sudo", "cp", tmpFile, iniPath)
		if err := cmd.Run(); err != nil {
			os.Remove(tmpFile)
			return fmt.Errorf("failed to enable %s: %w", extension, err)
		}
		os.Remove(tmpFile)
	}

	return nil
}

// Disable disables an extension
func (e *ExtensionManager) Disable(version, extension, sapi string) error {
	confDir := e.getConfDir(version)

	// Find the ini file for this extension
	entries, err := os.ReadDir(confDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // conf.d doesn't exist, nothing to disable
		}
		return err
	}

	var iniPath string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extName := e.extractExtensionName(entry.Name())
		if extName == extension {
			iniPath = filepath.Join(confDir, entry.Name())
			break
		}
	}

	if iniPath == "" {
		return nil // Not enabled, nothing to do
	}

	// Remove ini file
	if err := os.Remove(iniPath); err != nil {
		cmd := exec.Command("sudo", "rm", "-f", iniPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to disable %s: %w", extension, err)
		}
	}

	return nil
}

// GetInstalledVersions returns all PHP versions that have extensions
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
		// Check if it's a version directory (e.g., "8.5")
		if len(name) >= 3 && name[0] >= '0' && name[0] <= '9' {
			confDir := e.getConfDir(name)
			if _, err := os.Stat(filepath.Dir(confDir)); err == nil {
				versions = append(versions, name)
			}
		}
	}

	return versions
}
