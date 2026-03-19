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
	"time"

	"github.com/klauspost/compress/zstd"
)

// extractMode determines how file extraction handles existing files
type extractMode int

const (
	// extractOverwrite overwrites all existing files
	extractOverwrite extractMode = iota
	// extractMerge skips config files that already exist (preserves user config)
	extractMerge
)

var (
	dependencyRegex    = regexp.MustCompile(`^([a-zA-Z0-9._-]+)\s*\(([<>=]+)\s*([0-9.]+)\)$`)
	installedVersionRe = regexp.MustCompile(`^php(\d+\.\d+)`)
	// installSlotRegex validates InstallSlot values (e.g., "8.5" or "8.5.1")
	installSlotRegex = regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)
	// safeNameRegex validates package names for safe use in file paths
	safeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+\-]*$`)
	// safeVersionRegex validates version strings (e.g., "8.5.0", "6.1.0", "0.3.0+pie")
	safeVersionRegex = regexp.MustCompile(`^\d+(\.\d+)+([+][a-zA-Z0-9]+)?$`)
	// safeUsernameRegex validates OS usernames for config placeholder injection prevention
	safeUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
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

	// Validate to prevent config injection via crafted env vars
	if !safeUsernameRegex.MatchString(username) {
		username = "nobody"
	}
	if !safeUsernameRegex.MatchString(groupname) {
		groupname = "staff"
	}

	data = bytes.ReplaceAll(data, []byte("{{PHM_USER}}"), []byte(username))
	data = bytes.ReplaceAll(data, []byte("{{PHM_GROUP}}"), []byte(groupname))

	return data
}

// isConfigFile checks if the file is a config file that should have placeholders replaced
func isConfigFile(path string) bool {
	// Replace placeholders only in known config file types under etc/ directory
	if strings.Contains(path, "/etc/") {
		ext := filepath.Ext(path)
		return ext == ".conf" || ext == ".ini"
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
			fmt.Fprintf(os.Stderr, "warning: cannot read package database entry %s: %v\n", entry.Name(), err)
			continue
		}

		var pkg InstalledPackage
		if err := json.Unmarshal(data, &pkg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: corrupted package database entry %s: %v\n", entry.Name(), err)
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

	matches := dependencyRegex.FindStringSubmatch(strings.TrimSpace(dep))

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
	inProgress := make(map[string]bool)
	var result []Package

	var resolve func(p *Package) error
	resolve = func(p *Package) error {
		if resolved[p.Name] {
			return nil
		}
		if inProgress[p.Name] {
			return fmt.Errorf("circular dependency detected: %s", p.Name)
		}
		inProgress[p.Name] = true

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

		delete(inProgress, p.Name)
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

// stripPreRelease extracts the leading numeric portion from a version segment.
// e.g., "0-beta1" -> "0", "1rc2" -> "1", "5" -> "5"
func stripPreRelease(segment string) string {
	for i, c := range segment {
		if c < '0' || c > '9' {
			return segment[:i]
		}
	}
	return segment
}

// compareVersions compares two version strings.
// Non-numeric suffixes (e.g., "-beta1", "rc2") are stripped before comparison.
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
			if v, err := strconv.Atoi(stripPreRelease(partsA[i])); err == nil {
				numA = v
			}
		}
		if i < len(partsB) {
			if v, err := strconv.Atoi(stripPreRelease(partsB[i])); err == nil {
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

// allowedSystemPrefixes lists system directories that packages may legitimately install to
// (e.g., FPM LaunchDaemon plists)
var allowedSystemPrefixes = []string{
	"/Library/LaunchDaemons/",
}

// validateInstallPath checks that destPath is under the install prefix or a known system path
func (m *Manager) validateInstallPath(destPath string) error {
	cleanDest := filepath.Clean(destPath)
	cleanPrefix := filepath.Clean(m.installPrefix) + string(os.PathSeparator)

	// Allow install prefix
	if strings.HasPrefix(cleanDest, cleanPrefix) {
		return nil
	}

	// Allow known system paths
	for _, sysPrefix := range allowedSystemPrefixes {
		if strings.HasPrefix(cleanDest, sysPrefix) {
			return nil
		}
	}

	return fmt.Errorf("path traversal detected: %q escapes %q", destPath, m.installPrefix)
}

// Install installs a package from a tarball with default options
func (m *Manager) Install(pkgPath string) (*InstalledPackage, error) {
	return m.installFromTarball(pkgPath, InstallOptions{}, extractOverwrite)
}

// InstallWithOptions installs a package from a tarball with custom options
func (m *Manager) InstallWithOptions(pkgPath string, opts InstallOptions) (*InstalledPackage, error) {
	return m.installFromTarball(pkgPath, opts, extractOverwrite)
}

// validateInstallSlot ensures the slot value is safe (e.g., "8.5" or "8.5.1")
func validateInstallSlot(slot string) error {
	if slot == "" {
		return nil
	}
	if !installSlotRegex.MatchString(slot) {
		return fmt.Errorf("invalid install slot %q: must match X.Y or X.Y.Z", slot)
	}
	return nil
}

// readPkgInfo reads and validates pkginfo.json from the tarball (first pass).
func readPkgInfo(pkgPath string) (*Package, string, error) {
	f, err := os.Open(pkgPath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, "", err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if header.Name == "pkginfo.json" {
			data, err := io.ReadAll(io.LimitReader(tr, 1*1024*1024))
			if err != nil {
				return nil, "", err
			}
			var pkgInfo Package
			if err := json.Unmarshal(data, &pkgInfo); err != nil {
				return nil, "", fmt.Errorf("failed to parse pkginfo.json: %w", err)
			}
			if pkgInfo.Name == "" || pkgInfo.Version == "" {
				return nil, "", fmt.Errorf("invalid pkginfo.json: name and version are required")
			}
			if !safeNameRegex.MatchString(pkgInfo.Name) {
				return nil, "", fmt.Errorf("invalid package name %q: contains disallowed characters", pkgInfo.Name)
			}
			if !safeVersionRegex.MatchString(pkgInfo.Version) {
				return nil, "", fmt.Errorf("invalid package version %q: must be numeric (e.g., 8.5.0)", pkgInfo.Version)
			}
			// Derive sourceSlot from PHPVersion (for extensions) or Version (for core packages).
			// Extensions have Version like "6.1.0" but PHPVersion like "8.5.0",
			// and their tar paths are under /opt/php/8.5/, not /opt/php/6.1/.
			slotSource := pkgInfo.Version
			if pkgInfo.PHPVersion != "" {
				slotSource = pkgInfo.PHPVersion
			}
			var sourceSlot string
			if parts := strings.Split(slotSource, "."); len(parts) >= 2 {
				sourceSlot = parts[0] + "." + parts[1]
			}
			return &pkgInfo, sourceSlot, nil
		}
	}
	return nil, "", fmt.Errorf("pkginfo.json not found in package")
}

// maxFileSize is the maximum size for a single extracted file (500MB)
const maxFileSize int64 = 500 * 1024 * 1024

// maxConfigSize is the maximum size for a config file buffered into memory (1MB)
const maxConfigSize int64 = 1 * 1024 * 1024

// sanitizeFileMode strips setuid, setgid, and sticky bits from tar-supplied mode
func sanitizeFileMode(mode int64) os.FileMode {
	return os.FileMode(mode) & 0777
}

// installFromTarball is the unified extraction logic for both Install and InstallWithMerge.
// Uses two-pass approach: first reads and validates pkginfo.json, then extracts files.
// In extractMerge mode, config files that already exist on disk are skipped.
func (m *Manager) installFromTarball(pkgPath string, opts InstallOptions, mode extractMode) (*InstalledPackage, error) {
	if err := validateInstallSlot(opts.InstallSlot); err != nil {
		return nil, err
	}

	// Pass 1: read and validate pkginfo.json before extracting any files
	pkgInfo, sourceSlot, err := readPkgInfo(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package metadata: %w", err)
	}

	// Pass 2: extract files
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
	var installedFiles []string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Skip pkginfo.json (already read in pass 1)
		if header.Name == "pkginfo.json" {
			continue
		}

		if !strings.HasPrefix(header.Name, "files/") {
			continue
		}

		relPath := strings.TrimPrefix(header.Name, "files/")
		if relPath == "" {
			continue
		}

		destPath := "/" + relPath

		// Skip directory entries and non-regular files early (before validation)
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}

		// Rewrite path if installing to a different slot
		if opts.InstallSlot != "" && sourceSlot != "" && opts.InstallSlot != sourceSlot {
			oldPrefix := m.installPrefix + "/" + sourceSlot + "/"
			newPrefix := m.installPrefix + "/" + opts.InstallSlot + "/"
			if strings.HasPrefix(destPath, oldPrefix) {
				destPath = newPrefix + strings.TrimPrefix(destPath, oldPrefix)
			}
		}

		// Validate path stays within allowed prefixes
		if err := m.validateInstallPath(destPath); err != nil {
			return nil, err
		}

		destDir := filepath.Dir(destPath)

		// Create directory
		if err := os.MkdirAll(destDir, 0755); err != nil {
			if err2 := exec.Command("sudo", "mkdir", "-p", destDir).Run(); err2 != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", destDir, err2)
			}
		}

		// Reject files that exceed the maximum allowed size
		if header.Size > maxFileSize {
			return nil, fmt.Errorf("file %s exceeds maximum size (%d > %d bytes)", header.Name, header.Size, maxFileSize)
		}

		// In merge mode, skip config files that already exist
		if mode == extractMerge && !isBinaryPath(destPath) {
			if _, err := os.Stat(destPath); err == nil {
				installedFiles = append(installedFiles, destPath)
				continue
			}
		}

		// Extract file — stream to temp file, then move into place
		tmp, err := os.CreateTemp("", "phm-install-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmp.Name()

		if isConfigFile(destPath) {
			// Config files: buffer into memory for placeholder replacement
			if header.Size > maxConfigSize {
				tmp.Close()
				os.Remove(tmpPath)
				return nil, fmt.Errorf("config file %s exceeds maximum size (%d > %d bytes)", header.Name, header.Size, maxConfigSize)
			}
			data, err := io.ReadAll(io.LimitReader(tr, maxConfigSize))
			if err != nil {
				tmp.Close()
				os.Remove(tmpPath)
				return nil, err
			}
			data = replaceConfigPlaceholders(data)
			if _, err := tmp.Write(data); err != nil {
				tmp.Close()
				os.Remove(tmpPath)
				return nil, err
			}
		} else {
			// Binary files: stream directly to disk
			written, err := io.Copy(tmp, io.LimitReader(tr, maxFileSize))
			if err != nil {
				tmp.Close()
				os.Remove(tmpPath)
				return nil, err
			}
			if header.Size > 0 && written != header.Size {
				tmp.Close()
				os.Remove(tmpPath)
				return nil, fmt.Errorf("incomplete extraction of %s: got %d bytes, expected %d", header.Name, written, header.Size)
			}
		}
		tmp.Close()

		// Move temp file to destination (try directly, then with sudo)
		if err := os.Rename(tmpPath, destPath); err != nil {
			cmd := exec.Command("sudo", "cp", tmpPath, destPath)
			if err := cmd.Run(); err != nil {
				os.Remove(tmpPath)
				return nil, fmt.Errorf("failed to install %s: %w", destPath, err)
			}
			os.Remove(tmpPath)
		}

		// Set permissions with setuid/setgid/sticky bits stripped
		safeMode := sanitizeFileMode(header.Mode)
		if err := os.Chmod(destPath, safeMode); err != nil {
			_ = exec.Command("sudo", "chmod", fmt.Sprintf("%o", safeMode), destPath).Run()
		}

		installedFiles = append(installedFiles, destPath)
	}

	pkgInfoVal := *pkgInfo

	// Determine the actual install slot
	installSlot := sourceSlot
	if opts.InstallSlot != "" {
		installSlot = opts.InstallSlot
	}

	// Determine package name for database
	pkgName := pkgInfoVal.Name
	if opts.CustomName != "" {
		if !safeNameRegex.MatchString(opts.CustomName) {
			return nil, fmt.Errorf("invalid custom package name %q: contains disallowed characters", opts.CustomName)
		}
		pkgName = opts.CustomName
	}

	// Create installed package record
	installed := &InstalledPackage{
		Package:        pkgInfoVal,
		InstalledFiles: installedFiles,
		InstallSlot:    installSlot,
		Pinned:         opts.Pinned,
		InstalledAt:    time.Now(),
	}
	installed.Name = pkgName

	// Save to database
	if err := m.saveInstalled(installed); err != nil {
		return nil, err
	}

	m.installed[pkgName] = installed
	return installed, nil
}

// saveInstalled saves installed package info to database using atomic write
func (m *Manager) saveInstalled(pkg *InstalledPackage) error {
	if !safeNameRegex.MatchString(pkg.Name) {
		return fmt.Errorf("invalid package name for database: %q", pkg.Name)
	}

	dbDir := filepath.Join(m.dataDir, "installed")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file in same directory, then rename
	destPath := filepath.Join(dbDir, pkg.Name+".json")
	tmp, err := os.CreateTemp(dbDir, ".phm-db-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp database file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to save package database: %w", err)
	}
	return nil
}

// Remove removes an installed package
func (m *Manager) Remove(name string) error {
	pkg := m.installed[name]
	if pkg == nil {
		return fmt.Errorf("package not installed: %s", name)
	}

	// Remove files (only if they are under allowed paths)
	cleanPrefix := filepath.Clean(m.installPrefix) + string(os.PathSeparator)
	for _, file := range pkg.InstalledFiles {
		cleanFile := filepath.Clean(file)
		allowed := strings.HasPrefix(cleanFile, cleanPrefix)
		if !allowed {
			for _, sysPrefix := range allowedSystemPrefixes {
				if strings.HasPrefix(cleanFile, sysPrefix) {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			fmt.Fprintf(os.Stderr, "warning: skipping removal of %s (outside allowed paths)\n", file)
			continue
		}
		if err := os.Remove(cleanFile); err != nil {
			_ = exec.Command("sudo", "rm", "-f", cleanFile).Run()
		}
	}

	// Clean up empty parent directories (only within install prefix)
	cleanedDirs := make(map[string]bool)
	cleanInstallPrefix := filepath.Clean(m.installPrefix)
	for _, file := range pkg.InstalledFiles {
		dir := filepath.Clean(filepath.Dir(file))
		for !cleanedDirs[dir] {
			// Stop at or above install prefix
			if dir == cleanInstallPrefix || !strings.HasPrefix(dir, cleanInstallPrefix+string(os.PathSeparator)) {
				break
			}
			cleanedDirs[dir] = true
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) > 0 {
				break
			}
			if err := os.Remove(dir); err != nil {
				_ = exec.Command("sudo", "rmdir", dir).Run()
			}
			dir = filepath.Dir(dir)
		}
	}

	// Remove database entry
	if safeNameRegex.MatchString(name) {
		dbFile := filepath.Join(m.dataDir, "installed", name+".json")
		os.Remove(dbFile)
	}

	delete(m.installed, name)
	return nil
}

// GetInstalledVersions returns all installed PHP versions
func (m *Manager) GetInstalledVersions() []string {
	versions := make(map[string]bool)

	for name := range m.installed {
		if strings.HasPrefix(name, "php") {
			if matches := installedVersionRe.FindStringSubmatch(name); len(matches) > 1 {
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

// isBinaryPath checks if the path is a binary file that should always be overwritten
// Returns true for executables and shared libraries, false for config files
func isBinaryPath(relPath string) bool {
	// Config files - don't overwrite if exists
	if strings.Contains(relPath, "/etc/") {
		return false
	}

	// Binary directories - always overwrite
	if strings.Contains(relPath, "/bin/") ||
		strings.Contains(relPath, "/sbin/") ||
		strings.Contains(relPath, "/lib/") ||
		strings.Contains(relPath, "/libexec/") {
		return true
	}

	// Extensions (.so files) - always overwrite
	if strings.HasSuffix(relPath, ".so") {
		return true
	}

	// Default: treat as binary (overwrite)
	return true
}

// InstallWithMerge installs a package using merge strategy:
// - Binary files (bin/, sbin/, lib/, *.so) are always overwritten
// - Config files (etc/) are only written if they don't exist
// This solves macOS code signing issues when adding extensions to existing PHP installation
func (m *Manager) InstallWithMerge(pkgPath string, opts InstallOptions) (*InstalledPackage, error) {
	return m.installFromTarball(pkgPath, opts, extractMerge)
}

// GetInstalledForVersion returns all installed packages for a specific PHP version slot
func (m *Manager) GetInstalledForVersion(version string) []*InstalledPackage {
	var result []*InstalledPackage
	prefix := "php" + version + "-"

	for _, pkg := range m.installed {
		if strings.HasPrefix(pkg.Name, prefix) {
			result = append(result, pkg)
		}
	}

	return result
}
