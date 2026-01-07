package pkg

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Manager handles package operations
type Manager struct {
	installPrefix string
	dataDir       string
	installed     map[string]*InstalledPackage
}

// NewManager creates a new package manager
func NewManager(installPrefix, dataDir string) *Manager {
	return &Manager{
		installPrefix: installPrefix,
		dataDir:       dataDir,
		installed:     make(map[string]*InstalledPackage),
	}
}

// getInstallingUser returns the username and group of the user running the installation
// If run with sudo, it returns SUDO_USER instead of root
func getInstallingUser() (username, groupname string) {
	// Default to current user
	username = os.Getenv("USER")
	groupname = "staff" // Default macOS group

	// If running with sudo, get the actual user
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		username = sudoUser
	}

	// Try to get user info for group
	if u, err := user.Lookup(username); err == nil {
		if g, err := user.LookupGroupId(u.Gid); err == nil {
			groupname = g.Name
		}
	}

	return username, groupname
}

// replaceConfigPlaceholders replaces {{PHM_USER}} and {{PHM_GROUP}} in config data
func replaceConfigPlaceholders(data []byte) []byte {
	username, groupname := getInstallingUser()

	data = bytes.ReplaceAll(data, []byte("{{PHM_USER}}"), []byte(username))
	data = bytes.ReplaceAll(data, []byte("{{PHM_GROUP}}"), []byte(groupname))

	return data
}

// isConfigFile checks if the file is a config file that should have placeholders replaced
func isConfigFile(path string) bool {
	// Replace placeholders in files under etc/ directory
	if strings.Contains(path, "/etc/") {
		ext := filepath.Ext(path)
		return ext == ".conf" || ext == ".ini" || ext == ""
	}
	return false
}

// LoadInstalled loads installed packages database
func (m *Manager) LoadInstalled() error {
	dbDir := filepath.Join(m.dataDir, "installed")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dbDir, entry.Name()))
		if err != nil {
			continue
		}

		var pkg InstalledPackage
		if err := json.Unmarshal(data, &pkg); err != nil {
			continue
		}

		m.installed[pkg.Name] = &pkg
	}

	return nil
}

// IsInstalled checks if a package is installed
func (m *Manager) IsInstalled(name string) bool {
	_, ok := m.installed[name]
	return ok
}

// GetInstalled returns an installed package
func (m *Manager) GetInstalled(name string) *InstalledPackage {
	return m.installed[name]
}

// GetInstalledByPrefix returns all installed packages matching a prefix
func (m *Manager) GetInstalledByPrefix(prefix string) []*InstalledPackage {
	var result []*InstalledPackage
	for _, pkg := range m.installed {
		if strings.HasPrefix(pkg.Name, prefix) {
			result = append(result, pkg)
		}
	}
	return result
}

// Dependency represents a parsed dependency
type Dependency struct {
	Name       string
	Constraint string // >=, =, <=, etc.
	Version    string
}

// ParseDependency parses a dependency string like "php8.5-common (>= 8.5.0)"
func ParseDependency(dep string) Dependency {
	d := Dependency{Name: dep}

	// Match pattern: name (constraint version)
	re := regexp.MustCompile(`^([a-zA-Z0-9._-]+)\s*\(([<>=]+)\s*([0-9.]+)\)$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(dep))

	if len(matches) == 4 {
		d.Name = matches[1]
		d.Constraint = matches[2]
		d.Version = matches[3]
	}

	return d
}

// ResolveDependencies resolves all dependencies for a package
func (m *Manager) ResolveDependencies(pkg *Package, available []Package) ([]Package, error) {
	resolved := make(map[string]bool)
	var result []Package

	var resolve func(p *Package) error
	resolve = func(p *Package) error {
		if resolved[p.Name] {
			return nil
		}

		for _, depStr := range p.Depends {
			dep := ParseDependency(depStr)

			// Check if already installed with correct version
			if installed := m.GetInstalled(dep.Name); installed != nil {
				if m.versionSatisfies(installed.Version, dep.Constraint, dep.Version) {
					continue
				}
			}

			// Find in available packages
			var depPkg *Package
			for i := range available {
				if available[i].Name == dep.Name {
					depPkg = &available[i]
					break
				}
			}

			if depPkg == nil {
				return fmt.Errorf("dependency not found: %s", dep.Name)
			}

			// Resolve dependencies of this dependency
			if err := resolve(depPkg); err != nil {
				return err
			}
		}

		resolved[p.Name] = true
		result = append(result, *p)
		return nil
	}

	if err := resolve(pkg); err != nil {
		return nil, err
	}

	return result, nil
}

// versionSatisfies checks if version satisfies constraint
func (m *Manager) versionSatisfies(version, constraint, required string) bool {
	if constraint == "" || required == "" {
		return true
	}

	cmp := compareVersions(version, required)

	switch constraint {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case "<":
		return cmp < 0
	case "=", "==":
		return cmp == 0
	default:
		return true
	}
}

// compareVersions compares two version strings
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		numA, numB := 0, 0
		if i < len(partsA) {
			if v, err := strconv.Atoi(partsA[i]); err == nil {
				numA = v
			}
		}
		if i < len(partsB) {
			if v, err := strconv.Atoi(partsB[i]); err == nil {
				numB = v
			}
		}

		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
	}

	return 0
}

// InstallOptions contains options for package installation
type InstallOptions struct {
	// InstallSlot is the target directory slot (e.g., "8.5" or "8.5.1")
	// If empty, uses the default from the tarball
	InstallSlot string
	// Pinned marks the package as pinned (won't be upgraded)
	Pinned bool
	// CustomName overrides the package name in the database
	// Used for pinned versions: php8.5.1-cli vs php8.5-cli
	CustomName string
}

// Install installs a package from a tarball with default options
func (m *Manager) Install(pkgPath string) (*InstalledPackage, error) {
	return m.InstallWithOptions(pkgPath, InstallOptions{})
}

// InstallWithOptions installs a package from a tarball with custom options
func (m *Manager) InstallWithOptions(pkgPath string, opts InstallOptions) (*InstalledPackage, error) {
	// Open tarball
	f, err := os.Open(pkgPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	var pkgInfo Package
	var installedFiles []string
	var sourceSlot string // The original install slot from tarball (e.g., "8.5")

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch header.Name {
		case "pkginfo.json":
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &pkgInfo); err != nil {
				return nil, err
			}
			// Detect source slot from package version (e.g., "8.5.0" -> "8.5")
			if parts := strings.Split(pkgInfo.Version, "."); len(parts) >= 2 {
				sourceSlot = parts[0] + "." + parts[1]
			}

		default:
			if strings.HasPrefix(header.Name, "files/") {
				relPath := strings.TrimPrefix(header.Name, "files/")
				if relPath == "" {
					continue
				}

				destPath := "/" + relPath

				// Rewrite path if installing to a different slot
				// e.g., /opt/php/8.5/bin/php -> /opt/php/8.5.1/bin/php
				if opts.InstallSlot != "" && sourceSlot != "" && opts.InstallSlot != sourceSlot {
					oldPrefix := "/opt/php/" + sourceSlot + "/"
					newPrefix := "/opt/php/" + opts.InstallSlot + "/"
					if strings.HasPrefix(destPath, oldPrefix) {
						destPath = newPrefix + strings.TrimPrefix(destPath, oldPrefix)
					}
				}

				destDir := filepath.Dir(destPath)

				// Create directory
				if err := os.MkdirAll(destDir, 0755); err != nil {
					// Try with sudo
					_ = exec.Command("sudo", "mkdir", "-p", destDir).Run()
				}

				if header.Typeflag == tar.TypeDir {
					continue
				}

				// Extract file
				data, err := io.ReadAll(tr)
				if err != nil {
					return nil, err
				}

				// Replace placeholders in config files
				if isConfigFile(destPath) {
					data = replaceConfigPlaceholders(data)
				}

				// Write file (try directly, then with sudo)
				if err := os.WriteFile(destPath, data, os.FileMode(header.Mode)); err != nil {
					// Use sudo
					tmpFile := filepath.Join(os.TempDir(), filepath.Base(destPath))
					if err := os.WriteFile(tmpFile, data, 0644); err != nil {
						return nil, err
					}
					cmd := exec.Command("sudo", "cp", tmpFile, destPath)
					if err := cmd.Run(); err != nil {
						os.Remove(tmpFile)
						return nil, fmt.Errorf("failed to install %s: %w", destPath, err)
					}
					_ = exec.Command("sudo", "chmod", fmt.Sprintf("%o", header.Mode), destPath).Run()
					os.Remove(tmpFile)
				}

				installedFiles = append(installedFiles, destPath)
			}
		}
	}

	// Determine the actual install slot
	installSlot := sourceSlot
	if opts.InstallSlot != "" {
		installSlot = opts.InstallSlot
	}

	// Determine package name for database
	pkgName := pkgInfo.Name
	if opts.CustomName != "" {
		pkgName = opts.CustomName
	}

	// Create installed package record
	installed := &InstalledPackage{
		Package:        pkgInfo,
		InstalledFiles: installedFiles,
		InstallSlot:    installSlot,
		Pinned:         opts.Pinned,
	}
	// Override name if custom name provided
	installed.Name = pkgName

	// Save to database
	if err := m.saveInstalled(installed); err != nil {
		return nil, err
	}

	m.installed[pkgName] = installed
	return installed, nil
}

// saveInstalled saves installed package info to database
func (m *Manager) saveInstalled(pkg *InstalledPackage) error {
	dbDir := filepath.Join(m.dataDir, "installed")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dbDir, pkg.Name+".json"), data, 0644)
}

// Remove removes an installed package
func (m *Manager) Remove(name string) error {
	pkg := m.installed[name]
	if pkg == nil {
		return fmt.Errorf("package not installed: %s", name)
	}

	// Remove files
	for _, file := range pkg.InstalledFiles {
		if err := os.Remove(file); err != nil {
			_ = exec.Command("sudo", "rm", "-f", file).Run()
		}
	}

	// Remove database entry
	dbFile := filepath.Join(m.dataDir, "installed", name+".json")
	os.Remove(dbFile)

	delete(m.installed, name)
	return nil
}

// GetInstalledVersions returns all installed PHP versions
func (m *Manager) GetInstalledVersions() []string {
	versions := make(map[string]bool)

	for name := range m.installed {
		if strings.HasPrefix(name, "php") {
			// Extract version like "8.5" from "php8.5-cli"
			re := regexp.MustCompile(`^php(\d+\.\d+)`)
			if matches := re.FindStringSubmatch(name); len(matches) > 1 {
				versions[matches[1]] = true
			}
		}
	}

	var result []string
	for v := range versions {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}

// GetDependents returns packages that depend on the given package
func (m *Manager) GetDependents(name string) []string {
	var dependents []string

	for pkgName, pkg := range m.installed {
		if pkgName == name {
			continue
		}

		for _, depStr := range pkg.Depends {
			dep := ParseDependency(depStr)
			if dep.Name == name {
				dependents = append(dependents, pkgName)
				break
			}
		}
	}

	sort.Strings(dependents)
	return dependents
}

// GetAllInstalled returns all installed packages
func (m *Manager) GetAllInstalled() []*InstalledPackage {
	var result []*InstalledPackage
	for _, pkg := range m.installed {
		result = append(result, pkg)
	}
	return result
}

// CheckUpgrade checks if a package has an available upgrade
// Returns the new version if upgrade available, empty string otherwise
func (m *Manager) CheckUpgrade(name string, availableVersion string) string {
	installed := m.GetInstalled(name)
	if installed == nil {
		return ""
	}

	// Don't upgrade pinned packages (installed with specific patch version like php8.5.1-cli)
	if installed.Pinned {
		return ""
	}

	if compareVersions(availableVersion, installed.Version) > 0 {
		return availableVersion
	}

	return ""
}

// CheckUpgradeWithPHP checks if upgrade is needed considering both extension and PHP version
// For extensions, the extension version might be the same but PHP version different (e.g., redis 6.3.0 for PHP 8.5.0 vs 8.5.1)
func (m *Manager) CheckUpgradeWithPHP(name string, availableVersion string, availablePHPVersion string) string {
	installed := m.GetInstalled(name)
	if installed == nil {
		return ""
	}

	// Don't upgrade pinned packages
	if installed.Pinned {
		return ""
	}

	// Check extension version first
	versionCmp := compareVersions(availableVersion, installed.Version)
	if versionCmp > 0 {
		return availableVersion
	}

	// Same extension version - check PHP version (for extensions rebuilt for newer PHP)
	if versionCmp == 0 && availablePHPVersion != "" && installed.PHPVersion != "" {
		if compareVersions(availablePHPVersion, installed.PHPVersion) > 0 {
			return availableVersion
		}
	}

	return ""
}

// CompareVersions compares two version strings (exported wrapper)
func CompareVersions(a, b string) int {
	return compareVersions(a, b)
}
