package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/phm-dev/phm/internal/config"
	"github.com/phm-dev/phm/internal/pkg"
	"github.com/phm-dev/phm/internal/repo"
	"github.com/phm-dev/phm/internal/tui"
	"github.com/spf13/cobra"
)

var (
	version = "dev" // injected via ldflags during release build
	cfg     *config.Config
)

func main() {
	cfg = config.New()

	rootCmd := &cobra.Command{
		Use:     "phm",
		Short:   "PHM - PHP Manager for macOS",
		Long:    "A package manager for PHP installations on macOS with TUI interface",
		Version: version,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&cfg.Offline, "offline", false, "Use offline mode (local repository)")
	rootCmd.PersistentFlags().StringVar(&cfg.RepoPath, "repo", "", "Path to local repository (implies --offline)")
	rootCmd.PersistentFlags().BoolVar(&cfg.Debug, "debug", false, "Enable debug output")

	// Commands
	rootCmd.AddCommand(
		newInstallCmd(),
		newRemoveCmd(),
		newListCmd(),
		newSearchCmd(),
		newUpgradeCmd(),
		newInfoCmd(),
		newUseCmd(),
		newFpmCmd(),
		newExtCmd(),
		newUICmd(),
		newConfigCmd(),
		newDestructCmd(),
		newSelfUpdateCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newInstallCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "install [packages...]",
		Aliases: []string{"i"},
		Short:   "Install packages",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(args, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force reinstall")
	return cmd
}

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [packages...]",
		Aliases: []string{"rm", "uninstall"},
		Short:   "Remove packages",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(args)
		},
	}
	return cmd
}

func newListCmd() *cobra.Command {
	var available, installed bool

	cmd := &cobra.Command{
		Use:     "list [pattern]",
		Aliases: []string{"ls"},
		Short:   "List packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern := ""
			if len(args) > 0 {
				pattern = args[0]
			}
			return runList(pattern, available, installed)
		},
	}

	cmd.Flags().BoolVarP(&available, "available", "a", false, "Show available packages")
	cmd.Flags().BoolVarP(&installed, "installed", "i", true, "Show installed packages")
	return cmd
}

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "search <query>",
		Aliases: []string{"s"},
		Short:   "Search packages",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(args[0])
		},
	}
	return cmd
}

func newUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade [packages...]",
		Short: "Upgrade packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(args)
		},
	}
	return cmd
}

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "info <package>",
		Aliases: []string{"show"},
		Short:   "Show package information",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInfo(args[0])
		},
	}
	return cmd
}

func newUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(cfg)
		},
	}
	return cmd
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfig()
		},
	}
	return cmd
}

func newUseCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "use <version>",
		Short: "Set default PHP version",
		Long: `Set the default PHP version.

By default, symlinks are created in /opt/php/bin/ which requires adding
this directory to your PATH. This is safe and won't conflict with Homebrew.

Use --system to also create symlinks in /usr/local/bin (may conflict with Homebrew).

Examples:
  phm use 8.5           # Set PHP 8.5 as default (in /opt/php/bin)
  phm use 8.5 --system  # Also create symlinks in /usr/local/bin
  phm use               # Show current and available versions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runUseList()
			}
			return runUse(args[0], system)
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "Also create symlinks in /usr/local/bin (may conflict with Homebrew)")

	return cmd
}

func newFpmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fpm <action> [version]",
		Short: "Manage PHP-FPM services",
		Long: `Manage PHP-FPM services for different PHP versions.

Actions:
  status           Show status of all PHP-FPM services
  start <version>  Start PHP-FPM for a specific version
  stop <version>   Stop PHP-FPM for a specific version
  restart <version> Restart PHP-FPM
  reload <version> Reload PHP-FPM configuration
  enable <version> Enable PHP-FPM to start at boot
  disable <version> Disable PHP-FPM from starting at boot

Examples:
  phm fpm status
  phm fpm start 8.5
  phm fpm stop 8.4
  phm fpm enable 8.5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runFpmStatus()
			}
			action := args[0]
			version := ""
			if len(args) > 1 {
				version = args[1]
			}
			return runFpm(action, version)
		},
	}
	return cmd
}

// ensureSudo prompts for sudo password upfront to avoid interruptions during installation
func ensureSudo() error {
	fmt.Printf("\033[34m==>\033[0m Checking root privileges...\n")
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to obtain root privileges: %w", err)
	}
	return nil
}

// getRepo creates and initializes repository with fresh index
func getRepo() (*repo.Repository, error) {
	// If --repo is set, enable offline mode
	if cfg.RepoPath != "" {
		cfg.Offline = true
	}

	r := repo.New(cfg)

	// In offline mode, just load local index
	if cfg.Offline {
		if err := r.LoadIndex(); err != nil {
			return nil, fmt.Errorf("failed to load index: %w", err)
		}
		return r, nil
	}

	// Always fetch fresh index (auto-sync)
	fmt.Printf("\033[34m==>\033[0m Syncing package index...\n")
	if err := r.FetchIndex(); err != nil {
		// Fall back to cached index if available
		if loadErr := r.LoadIndex(); loadErr != nil {
			return nil, fmt.Errorf("failed to fetch index: %w", err)
		}
		fmt.Printf("\033[33m[!]\033[0m Using cached index (fetch failed: %v)\n", err)
	} else {
		fmt.Printf("\033[32m[OK]\033[0m Package index synced\n")
	}
	fmt.Println()

	return r, nil
}

// getManager returns a package manager instance
func getManager() *pkg.Manager {
	return pkg.NewManager(cfg.InstallPrefix, cfg.DataDir)
}

// getLinker returns a linker instance
func getLinker() *pkg.Linker {
	return pkg.NewLinker(cfg.InstallPrefix)
}

// Command implementations
func runInstall(packages []string, force bool) error {
	// Prompt for sudo password upfront
	if err := ensureSudo(); err != nil {
		return err
	}

	r, err := getRepo()
	if err != nil {
		return err
	}

	mgr := getManager()
	if err := mgr.LoadInstalled(); err != nil {
		fmt.Printf("\033[33mWarning:\033[0m Could not load installed packages: %v\n", err)
	}

	linker := getLinker()
	allAvailable := r.GetPackages()

	// Expand meta-packages (slim/full) before processing
	packages = expandMetaPackages(packages, allAvailable)

	// Collect all packages to install (with dependencies resolved)
	var allToInstall []*installRequest
	seenPackages := make(map[string]bool)
	installedSlots := make(map[string]bool) // Track install slots (e.g., "8.5", "8.5.1")

	for _, name := range packages {
		// Parse the install request to handle both php8.5-cli and php8.5.1-cli
		req := parseInstallRequest(name, allAvailable)
		if req == nil {
			// Try direct lookup for backwards compatibility
			pkgToInstall := r.GetPackage(name)
			if pkgToInstall == nil {
				fmt.Printf("\033[31mError:\033[0m Package not found: %s\n", name)
				continue
			}
			req = &installRequest{
				RequestedName: name,
				CanonicalName: name,
				InstallSlot:   extractPHPVersion(name),
				IsPinned:      false,
				Package:       *pkgToInstall,
			}
		}

		// Resolve dependencies using canonical package
		toInstall, err := mgr.ResolveDependencies(&req.Package, allAvailable)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to resolve dependencies: %v\n", err)
			continue
		}

		// Add to install list (deduplicated by requested name)
		for _, p := range toInstall {
			// For dependencies, determine their install request
			depReqName := p.Name
			depSlot := req.InstallSlot
			depPinned := req.IsPinned

			// If this is a dependency (not the main package), adjust its name
			if p.Name != req.CanonicalName && req.IsPinned {
				// Dependency of a pinned package should also be pinned to same slot
				vinfo := pkg.ParsePackageName(p.Name)
				if vinfo != nil {
					depReqName = "php" + req.InstallSlot + "-" + vinfo.PackageType
				}
			} else if p.Name == req.CanonicalName {
				depReqName = req.RequestedName
			}

			if !seenPackages[depReqName] {
				seenPackages[depReqName] = true
				allToInstall = append(allToInstall, &installRequest{
					RequestedName: depReqName,
					CanonicalName: p.Name,
					InstallSlot:   depSlot,
					IsPinned:      depPinned,
					Package:       p,
				})
				if depSlot != "" {
					installedSlots[depSlot] = true
				}
			}
		}
	}

	if len(allToInstall) == 0 {
		fmt.Println("No packages to install.")
		return nil
	}

	// Auto-upgrade: Check if any installed packages of the same install slot need upgrading
	// Only for non-pinned slots (minor versions like 8.5)
	var packagesToUpgrade []*installRequest
	for slot := range installedSlots {
		// Skip pinned slots (patch versions like 8.5.1)
		if strings.Count(slot, ".") >= 2 {
			continue
		}

		installedPkgs := mgr.GetInstalledByPrefix("php" + slot)
		for _, installed := range installedPkgs {
			// Skip packages we're about to install (they'll be upgraded anyway)
			if seenPackages[installed.Name] {
				continue
			}
			// Skip pinned packages
			if installed.Pinned {
				continue
			}
			// Check if upgrade is available
			if available := r.GetPackage(installed.Name); available != nil {
				if pkg.CompareVersions(available.Version, installed.Version) > 0 {
					packagesToUpgrade = append(packagesToUpgrade, &installRequest{
						RequestedName: installed.Name,
						CanonicalName: installed.Name,
						InstallSlot:   slot,
						IsPinned:      false,
						Package:       *available,
					})
				}
			}
		}
	}

	// Perform auto-upgrade if needed
	if len(packagesToUpgrade) > 0 {
		fmt.Printf("\033[34m==>\033[0m Auto-upgrading %d installed package(s) to ensure compatibility...\n", len(packagesToUpgrade))
		for _, req := range packagesToUpgrade {
			installed := mgr.GetInstalled(req.RequestedName)
			fmt.Printf("    %s: %s -> %s\n", req.RequestedName, installed.Version, req.Package.Version)
		}
		fmt.Println()

		for _, req := range packagesToUpgrade {
			fmt.Printf("\033[34m==>\033[0m Upgrading %s to %s...\n", req.RequestedName, req.Package.Version)
			path, err := r.DownloadPackage(&req.Package)
			if err != nil {
				fmt.Printf("\033[31mError:\033[0m Failed to download %s: %v\n", req.RequestedName, err)
				continue
			}
			opts := pkg.InstallOptions{
				InstallSlot: req.InstallSlot,
				Pinned:      req.IsPinned,
				CustomName:  req.RequestedName,
			}
			if _, err := mgr.InstallWithOptions(path, opts); err != nil {
				fmt.Printf("\033[31mError:\033[0m Failed to upgrade %s: %v\n", req.RequestedName, err)
				continue
			}
			fmt.Printf("\033[32m[OK]\033[0m %s upgraded\n", req.RequestedName)
		}
		fmt.Println()
	}

	// Show installation plan
	fmt.Printf("\033[1mThe following packages will be installed:\033[0m\n")
	for _, req := range allToInstall {
		status := ""
		location := ""
		if mgr.IsInstalled(req.RequestedName) {
			status = " \033[33m(reinstall)\033[0m"
		}
		if req.IsPinned {
			location = fmt.Sprintf(" \033[36m[pinned -> /opt/php/%s]\033[0m", req.InstallSlot)
		}
		fmt.Printf("  - %s (%s)%s%s\n", req.RequestedName, req.Package.Version, status, location)
	}
	fmt.Println()

	// Install all packages
	var installedPkgs []pkg.Package
	for _, req := range allToInstall {
		if mgr.IsInstalled(req.RequestedName) && !force {
			fmt.Printf("\033[33m[SKIP]\033[0m %s already installed\n", req.RequestedName)
			continue
		}

		fmt.Printf("\033[34m==>\033[0m Installing %s (%s)...\n", req.RequestedName, req.Package.Version)

		// Download package
		path, err := r.DownloadPackage(&req.Package)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to download: %v\n", err)
			continue
		}

		// Install package with options
		opts := pkg.InstallOptions{
			InstallSlot: req.InstallSlot,
			Pinned:      req.IsPinned,
			CustomName:  req.RequestedName,
		}
		_, err = mgr.InstallWithOptions(path, opts)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to install: %v\n", err)
			continue
		}

		installedPkgs = append(installedPkgs, req.Package)
		fmt.Printf("\033[32m[OK]\033[0m %s installed\n", req.RequestedName)
	}

	// Setup symlinks for all installed slots (once at the end)
	for slot := range installedSlots {
		fmt.Printf("\033[34m==>\033[0m Setting up symlinks for PHP %s...\n", slot)
		if err := linker.SetupVersionLinks(slot); err != nil {
			fmt.Printf("\033[33mWarning:\033[0m Could not create symlinks: %v\n", err)
		} else {
			macportsVer := strings.ReplaceAll(slot, ".", "")
			fmt.Printf("\033[32m[OK]\033[0m Created: php%s, /opt/local/bin/php%s\n", slot, macportsVer)
		}
	}

	// Handle default version (only once at the end)
	// Prefer minor version slots (8.5) over pinned slots (8.5.1)
	var targetSlot string
	for slot := range installedSlots {
		if targetSlot == "" {
			targetSlot = slot
		} else if strings.Count(slot, ".") < strings.Count(targetSlot, ".") {
			// Prefer minor version (fewer dots)
			targetSlot = slot
		}
	}

	if targetSlot != "" {
		allVersions := linker.GetAvailableVersions()
		currentDefault := linker.GetDefaultVersion()

		if len(allVersions) == 1 {
			// Only one PHP version installed - auto-set as default
			fmt.Printf("\n\033[34m==>\033[0m Setting PHP %s as default...\n", targetSlot)
			if err := linker.SetDefaultVersion(targetSlot); err != nil {
				fmt.Printf("\033[33mWarning:\033[0m Could not set default: %v\n", err)
			} else {
				fmt.Printf("\033[32m[OK]\033[0m Default set to PHP %s\n", targetSlot)
				fmt.Printf("\n\033[33mNote:\033[0m Add to your PATH: export PATH=\"/opt/php/bin:$PATH\"\n")
				fmt.Printf("      Or run: phm use %s --system\n", targetSlot)
			}
		} else if currentDefault != targetSlot {
			// Multiple versions installed and different version is default - ask user
			fmt.Printf("\n\033[33mCurrent default is PHP %s.\033[0m\n", currentDefault)
			fmt.Printf("Set PHP %s as default? [y/N]: ", targetSlot)
			var answer string
			_, _ = fmt.Scanln(&answer)
			if answer == "y" || answer == "Y" || answer == "yes" {
				if err := linker.SetDefaultVersion(targetSlot); err != nil {
					fmt.Printf("\033[33mWarning:\033[0m Could not set default: %v\n", err)
				} else {
					fmt.Printf("\033[32m[OK]\033[0m Default set to PHP %s\n", targetSlot)
				}
			}
		}
	}

	// Print summary
	var upgradedPkgs []pkg.Package
	for _, req := range packagesToUpgrade {
		upgradedPkgs = append(upgradedPkgs, req.Package)
	}
	printInstallSummary(installedPkgs, upgradedPkgs, installedSlots, linker)

	return nil
}

// extractPHPVersion extracts PHP version from package name (e.g., "php8.5-cli" -> "8.5", "php8.5.1-cli" -> "8.5.1")
func extractPHPVersion(name string) string {
	// Try patch version first (e.g., php8.5.1-cli -> 8.5.1)
	re := regexp.MustCompile(`^php(\d+\.\d+\.\d+)`)
	if matches := re.FindStringSubmatch(name); len(matches) > 1 {
		return matches[1]
	}
	// Try minor version (e.g., php8.5-cli -> 8.5)
	re = regexp.MustCompile(`^php(\d+\.\d+)`)
	if matches := re.FindStringSubmatch(name); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// normalizePackageName converts old-style package names to new canonical names
// Examples:
//   - php8.5.0-cli -> php8.5-cli
//   - php8.5.0-redis6.3.0 -> php8.5-redis
//   - php8.5-cli -> php8.5-cli (unchanged)
func normalizePackageName(name string) string {
	// Match old format: php{major}.{minor}.{patch}-{component}{version}
	// e.g., php8.5.0-redis6.3.0 or php8.5.0-cli
	re := regexp.MustCompile(`^php(\d+\.\d+)\.\d+-([a-z]+)[\d.]*$`)
	if matches := re.FindStringSubmatch(name); len(matches) == 3 {
		return fmt.Sprintf("php%s-%s", matches[1], matches[2])
	}
	return name
}

// installRequest represents a package installation request with version info
type installRequest struct {
	RequestedName string // What user requested (e.g., "php8.5.1-cli")
	CanonicalName string // Package name in index (e.g., "php8.5-cli")
	InstallSlot   string // Directory slot (e.g., "8.5" or "8.5.1")
	IsPinned      bool   // Whether version is pinned
	Package       pkg.Package
}

// parseInstallRequest parses a package name and returns installation request info
func parseInstallRequest(name string, available []pkg.Package) *installRequest {
	versionInfo := pkg.ParsePackageName(name)
	if versionInfo == nil {
		// Not a PHP package, use as-is
		for _, p := range available {
			if p.Name == name {
				return &installRequest{
					RequestedName: name,
					CanonicalName: name,
					InstallSlot:   "",
					IsPinned:      false,
					Package:       p,
				}
			}
		}
		return nil
	}

	// Look up canonical package name in index
	canonicalName := versionInfo.GetCanonicalName()
	var foundPkg *pkg.Package
	for i := range available {
		if available[i].Name == canonicalName {
			foundPkg = &available[i]
			break
		}
	}

	if foundPkg == nil {
		return nil
	}

	// For pinned versions, verify the requested version matches available version
	if versionInfo.IsPinned {
		if foundPkg.Version != versionInfo.PatchVersion {
			// Requested version doesn't match available version
			return nil
		}
	}

	return &installRequest{
		RequestedName: name,
		CanonicalName: canonicalName,
		InstallSlot:   versionInfo.GetInstallSlot(),
		IsPinned:      versionInfo.IsPinned,
		Package:       *foundPkg,
	}
}

// printInstallSummary prints a nice summary after installation
func printInstallSummary(installed []pkg.Package, upgraded []pkg.Package, versions map[string]bool, linker *pkg.Linker) {
	fmt.Println()
	separator := strings.Repeat("─", 50)
	fmt.Printf("\033[1;32m%s\033[0m\n", separator)
	fmt.Printf("\033[1;32m  Installation Complete!\033[0m\n")
	fmt.Printf("\033[1;32m%s\033[0m\n\n", separator)

	// Installed packages
	if len(installed) > 0 {
		fmt.Printf("\033[1mInstalled:\033[0m\n")
		for _, p := range installed {
			fmt.Printf("  \033[32m+\033[0m %s (%s)\n", p.Name, p.Version)
		}
	}

	// Upgraded packages
	if len(upgraded) > 0 {
		fmt.Printf("\n\033[1mUpgraded:\033[0m\n")
		for _, p := range upgraded {
			fmt.Printf("  \033[34m↑\033[0m %s (%s)\n", p.Name, p.Version)
		}
	}

	// Show PHP version for each installed version
	for version := range versions {
		phpBin := filepath.Join(cfg.InstallPrefix, version, "bin", "php")
		if _, err := os.Stat(phpBin); err == nil {
			fmt.Printf("\n\033[1mPHP %s:\033[0m\n", version)
			cmd := exec.Command(phpBin, "-v")
			if output, err := cmd.Output(); err == nil {
				lines := strings.Split(string(output), "\n")
				if len(lines) > 0 {
					fmt.Printf("  %s\n", lines[0])
				}
			}

			// Show loaded extensions count
			cmd = exec.Command(phpBin, "-m")
			if output, err := cmd.Output(); err == nil {
				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				extCount := 0
				for _, line := range lines {
					if line != "" && line != "[PHP Modules]" && line != "[Zend Modules]" {
						extCount++
					}
				}
				fmt.Printf("  Extensions: %d loaded\n", extCount)
			}
		}
	}

	// Show current default
	defaultVersion := linker.GetDefaultVersion()
	if defaultVersion != "" {
		fmt.Printf("\n\033[1mDefault version:\033[0m PHP %s\n", defaultVersion)
	}

	// Quick tips
	fmt.Printf("\n\033[1mQuick tips:\033[0m\n")
	fmt.Printf("  phm ext list           # Show available extensions\n")
	fmt.Printf("  phm fpm start %s      # Start PHP-FPM\n", defaultVersion)
	fmt.Printf("  phm use <version>      # Change default PHP version\n")
	fmt.Println()
}

// expandMetaPackages expands meta-packages (slim/full) into real package lists
// php8.5-slim -> common, cli, fpm, cgi, dev, pear
// php8.5-full -> slim + all available extensions
func expandMetaPackages(packages []string, available []pkg.Package) []string {
	var result []string

	for _, name := range packages {
		// Check for slim meta-package (e.g., php8.5-slim)
		if matches := regexp.MustCompile(`^php(\d+\.\d+)-slim$`).FindStringSubmatch(name); len(matches) > 1 {
			version := matches[1]
			slimPkgs := []string{
				"php" + version + "-common",
				"php" + version + "-cli",
				"php" + version + "-fpm",
				"php" + version + "-cgi",
				"php" + version + "-dev",
				"php" + version + "-pear",
			}
			// Only add packages that exist in available
			for _, p := range slimPkgs {
				if packageExists(p, available) {
					result = append(result, p)
				}
			}
			fmt.Printf("\033[34m==>\033[0m Expanding %s to: %s\n\n", name, strings.Join(slimPkgs, ", "))
			continue
		}

		// Check for full meta-package (e.g., php8.5-full)
		if matches := regexp.MustCompile(`^php(\d+\.\d+)-full$`).FindStringSubmatch(name); len(matches) > 1 {
			version := matches[1]
			prefix := "php" + version + "-"

			// First add slim packages
			slimPkgs := []string{"common", "cli", "fpm", "cgi", "dev", "pear"}
			var fullPkgs []string
			for _, p := range slimPkgs {
				pkgName := prefix + p
				if packageExists(pkgName, available) {
					fullPkgs = append(fullPkgs, pkgName)
				}
			}

			// Then add all extensions for this version
			for _, p := range available {
				if strings.HasPrefix(p.Name, prefix) {
					// Skip core packages (already added)
					suffix := strings.TrimPrefix(p.Name, prefix)
					isCore := false
					for _, core := range slimPkgs {
						if suffix == core {
							isCore = true
							break
						}
					}
					if !isCore {
						fullPkgs = append(fullPkgs, p.Name)
					}
				}
			}

			result = append(result, fullPkgs...)
			fmt.Printf("\033[34m==>\033[0m Expanding %s to %d packages\n\n", name, len(fullPkgs))
			continue
		}

		// Not a meta-package, keep as is
		result = append(result, name)
	}

	return result
}

// packageExists checks if a package exists in the available list
func packageExists(name string, available []pkg.Package) bool {
	for _, p := range available {
		if p.Name == name {
			return true
		}
	}
	return false
}

func runRemove(packages []string) error {
	// Prompt for sudo password upfront
	if err := ensureSudo(); err != nil {
		return err
	}

	mgr := getManager()
	if err := mgr.LoadInstalled(); err != nil {
		return fmt.Errorf("could not load installed packages: %w", err)
	}

	linker := getLinker()

	for _, name := range packages {
		if !mgr.IsInstalled(name) {
			fmt.Printf("\033[33mWarning:\033[0m Package %s is not installed\n", name)
			continue
		}

		// Check if other packages depend on this one
		dependents := mgr.GetDependents(name)
		if len(dependents) > 0 {
			fmt.Printf("\033[31mError:\033[0m Cannot remove %s, required by:\n", name)
			for _, dep := range dependents {
				fmt.Printf("  - %s\n", dep)
			}
			fmt.Printf("\nRemove dependent packages first, or use --force\n")
			continue
		}

		fmt.Printf("\033[34m==>\033[0m Removing %s...\n", name)

		if err := mgr.Remove(name); err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to remove %s: %v\n", name, err)
			continue
		}

		fmt.Printf("\033[32m[OK]\033[0m %s removed\n", name)

		// Check if this was the last package for a PHP version
		phpVersion := extractPHPVersion(name)
		if phpVersion != "" {
			// Check if any packages for this version remain
			remaining := mgr.GetInstalledByPrefix("php" + phpVersion)
			if len(remaining) == 0 {
				fmt.Printf("\033[34m==>\033[0m Removing symlinks for PHP %s...\n", phpVersion)
				_ = linker.RemoveVersionLinks(phpVersion)

				// If this was the default version, clear it
				if linker.GetDefaultVersion() == phpVersion {
					// Try to set another version as default
					available := linker.GetAvailableVersions()
					if len(available) > 0 {
						fmt.Printf("\033[34m==>\033[0m Setting PHP %s as new default...\n", available[0])
						_ = linker.SetDefaultVersion(available[0])
					}
				}
			}
		}
	}

	return nil
}

func runList(pattern string, showAvailable, showInstalled bool) error {
	mgr := getManager()
	if err := mgr.LoadInstalled(); err != nil {
		// Not fatal, just won't show installed status
		if cfg.Debug {
			fmt.Printf("\033[33mWarning:\033[0m Could not load installed packages: %v\n", err)
		}
	}

	// If showing installed packages only
	if showInstalled && !showAvailable {
		installedPkgs := mgr.GetAllInstalled()
		if len(installedPkgs) == 0 {
			fmt.Println("No packages installed")
			fmt.Println("\nUse: phm list -a  to show available packages")
			return nil
		}

		fmt.Printf("\n\033[1m%-35s %-12s %s\033[0m\n", "Package", "Version", "Description")
		fmt.Printf("%-35s %-12s %s\n", strings.Repeat("-", 35), strings.Repeat("-", 12), strings.Repeat("-", 30))

		count := 0
		for _, pkg := range installedPkgs {
			if pattern != "" && !strings.Contains(pkg.Name, pattern) {
				continue
			}

			desc := pkg.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}

			fmt.Printf("%-35s %-12s %s\n", pkg.Name, pkg.Version, desc)
			count++
		}

		fmt.Printf("\nInstalled: %d package(s)\n", count)
		return nil
	}

	// Show available packages (from repo)
	r, err := getRepo()
	if err != nil {
		return err
	}

	packages := r.GetPackages()
	if len(packages) == 0 {
		fmt.Println("No packages found in repository")
		fmt.Println("\nRun: phm update  to fetch package index")
		return nil
	}

	fmt.Printf("\n\033[1m%-35s %-12s %-12s %s\033[0m\n", "Package", "Available", "Installed", "Description")
	fmt.Printf("%-35s %-12s %-12s %s\n", strings.Repeat("-", 35), strings.Repeat("-", 12), strings.Repeat("-", 12), strings.Repeat("-", 25))

	countAvailable := 0
	countInstalled := 0

	for _, p := range packages {
		if pattern != "" && !strings.Contains(p.Name, pattern) {
			continue
		}

		installedVer := "-"
		installedPkg := mgr.GetInstalled(p.Name)
		if installedPkg != nil {
			countInstalled++

			// Highlight if upgrade available
			if pkg.CompareVersions(p.Version, installedPkg.Version) > 0 {
				installedVer = fmt.Sprintf("\033[33m%s\033[0m", installedPkg.Version)
			} else {
				installedVer = fmt.Sprintf("\033[32m%s\033[0m", installedPkg.Version)
			}
		}

		desc := p.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}

		fmt.Printf("%-35s %-12s %-21s %s\n", p.Name, p.Version, installedVer, desc)
		countAvailable++
	}

	fmt.Printf("\nAvailable: %d, Installed: %d\n", countAvailable, countInstalled)

	if countInstalled > 0 {
		// Check for upgrades
		upgradeCount := 0
		for _, p := range packages {
			if installedPkg := mgr.GetInstalled(p.Name); installedPkg != nil {
				if pkg.CompareVersions(p.Version, installedPkg.Version) > 0 {
					upgradeCount++
				}
			}
		}
		if upgradeCount > 0 {
			fmt.Printf("\033[33m%d package(s) can be upgraded. Run: phm upgrade\033[0m\n", upgradeCount)
		}
	}

	return nil
}

func runSearch(query string) error {
	r, err := getRepo()
	if err != nil {
		return err
	}

	results := r.SearchPackages(query)
	if len(results) == 0 {
		fmt.Printf("No packages found matching '%s'\n", query)
		return nil
	}

	fmt.Printf("\n\033[1mSearch results for '%s':\033[0m\n\n", query)

	for _, pkg := range results {
		fmt.Printf("  \033[1m%s\033[0m (%s)\n", pkg.Name, pkg.Version)
		if pkg.Description != "" {
			fmt.Printf("      %s\n", pkg.Description)
		}
		fmt.Println()
	}

	fmt.Printf("Found %d package(s)\n", len(results))
	return nil
}

func runUpgrade(packages []string) error {
	// Prompt for sudo password upfront
	if err := ensureSudo(); err != nil {
		return err
	}

	r, err := getRepo()
	if err != nil {
		return err
	}

	mgr := getManager()
	if err := mgr.LoadInstalled(); err != nil {
		return fmt.Errorf("could not load installed packages: %w", err)
	}

	// If no packages specified, check all installed packages
	var toCheck []string
	if len(packages) == 0 {
		for _, pkg := range mgr.GetAllInstalled() {
			toCheck = append(toCheck, pkg.Name)
		}
	} else {
		toCheck = packages
	}

	if len(toCheck) == 0 {
		fmt.Println("No packages installed")
		return nil
	}

	// Find packages with available upgrades
	type upgrade struct {
		installedName  string // Name in installed DB (e.g., php8.5.0-cli)
		canonicalName  string // Name in index (e.g., php8.5-cli)
		oldVersion     string
		newVersion     string
	}
	var upgrades []upgrade

	fmt.Println("\033[34m==>\033[0m Checking for upgrades...")

	for _, name := range toCheck {
		installed := mgr.GetInstalled(name)
		if installed == nil {
			continue
		}

		// Normalize old-style package names to find in index
		canonicalName := normalizePackageName(name)
		available := r.GetPackage(canonicalName)
		if available == nil {
			continue
		}

		if newVer := mgr.CheckUpgradeWithPHP(name, available.Version, available.PHPVersion); newVer != "" {
			upgrades = append(upgrades, upgrade{
				installedName:  name,
				canonicalName:  canonicalName,
				oldVersion:     installed.Version,
				newVersion:     newVer,
			})
		}
	}

	if len(upgrades) == 0 {
		fmt.Println("\033[32m[OK]\033[0m All packages are up to date")
		return nil
	}

	// Show upgrade plan
	fmt.Printf("\n\033[1mThe following packages will be upgraded:\033[0m\n")
	for _, u := range upgrades {
		fmt.Printf("  %s: %s -> %s\n", u.canonicalName, u.oldVersion, u.newVersion)
	}
	fmt.Printf("\n%d package(s) to upgrade.\n\n", len(upgrades))

	// Perform upgrades
	linker := getLinker()
	allAvailable := r.GetPackages()

	for _, u := range upgrades {
		pkgToInstall := r.GetPackage(u.canonicalName)
		if pkgToInstall == nil {
			continue
		}

		// Resolve dependencies
		toInstall, err := mgr.ResolveDependencies(pkgToInstall, allAvailable)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to resolve dependencies for %s: %v\n", u.canonicalName, err)
			continue
		}

		// Install each package (including dependencies that need upgrade)
		for _, p := range toInstall {
			// Check if upgrade needed using normalized name
			normalizedName := normalizePackageName(p.Name)
			installedPkg := mgr.GetInstalled(p.Name)
			if installedPkg == nil {
				// Try to find with old naming convention
				for _, inst := range mgr.GetAllInstalled() {
					if normalizePackageName(inst.Name) == normalizedName {
						installedPkg = inst
						break
					}
				}
			}
			if installedPkg != nil {
				// Compare extension version first
				versionCmp := pkg.CompareVersions(p.Version, installedPkg.Version)
				if versionCmp < 0 {
					continue // Available version is older
				}
				if versionCmp == 0 {
					// Same extension version - compare PHP version (e.g., 8.5.0 vs 8.5.1)
					phpCmp := pkg.CompareVersions(p.PHPVersion, installedPkg.PHPVersion)
					if phpCmp <= 0 {
						continue // Same or older PHP version
					}
				}
			}

			fmt.Printf("\033[34m==>\033[0m Upgrading %s to %s...\n", p.Name, p.Version)

			// Download package
			path, err := r.DownloadPackage(&p)
			if err != nil {
				fmt.Printf("\033[31mError:\033[0m Failed to download: %v\n", err)
				continue
			}

			// Install package (overwrites existing)
			_, err = mgr.Install(path)
			if err != nil {
				fmt.Printf("\033[31mError:\033[0m Failed to install: %v\n", err)
				continue
			}

			fmt.Printf("\033[32m[OK]\033[0m %s upgraded to %s\n", p.Name, p.Version)
		}

		// Update symlinks if needed
		phpVersion := extractPHPVersion(u.installedName)
		if phpVersion != "" {
			_ = linker.SetupVersionLinks(phpVersion)
			if linker.GetDefaultVersion() == phpVersion {
				_ = linker.SetDefaultVersion(phpVersion)
			}
		}
	}

	fmt.Println("\n\033[32m[OK]\033[0m Upgrade complete")
	return nil
}

func runInfo(pkgName string) error {
	r, err := getRepo()
	if err != nil {
		return err
	}

	mgr := getManager()
	_ = mgr.LoadInstalled() // Ignore error, just won't show installed status

	availablePkg := r.GetPackage(pkgName)
	installedPkg := mgr.GetInstalled(pkgName)

	if availablePkg == nil && installedPkg == nil {
		return fmt.Errorf("package not found: %s", pkgName)
	}

	// Use available package info if exists, otherwise use installed
	var p *pkg.Package
	if availablePkg != nil {
		p = availablePkg
	} else {
		p = &installedPkg.Package
	}

	fmt.Printf("\n\033[1mPackage: %s\033[0m\n\n", p.Name)
	fmt.Printf("  Available:    %s\n", p.Version)

	if installedPkg != nil {
		if availablePkg != nil && pkg.CompareVersions(availablePkg.Version, installedPkg.Version) > 0 {
			fmt.Printf("  Installed:    \033[33m%s\033[0m (upgrade available)\n", installedPkg.Version)
		} else {
			fmt.Printf("  Installed:    \033[32m%s\033[0m\n", installedPkg.Version)
		}
	} else {
		fmt.Printf("  Installed:    \033[31mnot installed\033[0m\n")
	}

	fmt.Printf("  Revision:     %d\n", p.Revision)
	fmt.Printf("  Description:  %s\n", p.Description)
	fmt.Printf("  Platform:     %s\n", p.Platform)

	if len(p.Depends) > 0 {
		fmt.Printf("  Dependencies: %s\n", strings.Join(p.Depends, ", "))
	}

	if len(p.Provides) > 0 {
		fmt.Printf("  Provides:     %s\n", strings.Join(p.Provides, ", "))
	}

	if p.Size > 0 {
		fmt.Printf("  Size:         %.2f KB\n", float64(p.Size)/1024)
	}

	// Show installed files if installed
	if installedPkg != nil && len(installedPkg.InstalledFiles) > 0 {
		fmt.Printf("\n  \033[1mInstalled files:\033[0m\n")
		maxFiles := 10
		for i, f := range installedPkg.InstalledFiles {
			if i >= maxFiles {
				fmt.Printf("    ... and %d more files\n", len(installedPkg.InstalledFiles)-maxFiles)
				break
			}
			fmt.Printf("    %s\n", f)
		}
	}

	// Show dependents if installed
	if installedPkg != nil {
		dependents := mgr.GetDependents(pkgName)
		if len(dependents) > 0 {
			fmt.Printf("\n  \033[1mRequired by:\033[0m %s\n", strings.Join(dependents, ", "))
		}
	}

	fmt.Println()
	return nil
}

func runConfig() error {
	mode := "online"
	if cfg.Offline || cfg.RepoPath != "" {
		mode = "offline"
	}

	fmt.Printf("\n\033[1mPHM Configuration\033[0m\n\n")
	fmt.Printf("  Mode:           %s\n", mode)
	fmt.Printf("  Repository:     %s\n", cfg.GetRepoURL())
	fmt.Printf("  Install prefix: %s\n", cfg.InstallPrefix)
	fmt.Printf("  Cache dir:      %s\n", cfg.CacheDir)
	fmt.Printf("  Data dir:       %s\n", cfg.DataDir)
	fmt.Printf("  Platform:       %s\n", cfg.Platform())
	fmt.Println()
	return nil
}

func runUse(version string, system bool) error {
	linker := getLinker()

	// Check if version is installed
	available := linker.GetAvailableVersions()
	found := false
	for _, v := range available {
		if v == version {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("\033[31mError:\033[0m PHP %s is not installed\n", version)
		fmt.Printf("\nInstalled versions:\n")
		for _, v := range available {
			fmt.Printf("  - %s\n", v)
		}
		fmt.Printf("\nInstall with: phm install php%s\n", version)
		return nil
	}

	fmt.Printf("\033[34m==>\033[0m Setting PHP %s as default...\n", version)

	if err := linker.SetDefaultVersion(version); err != nil {
		return fmt.Errorf("failed to set default version: %w", err)
	}

	fmt.Printf("\033[32m[OK]\033[0m PHP %s is now the default version\n", version)
	fmt.Printf("\nSymlinks created in %s:\n", linker.GetPHMBinDir())
	fmt.Printf("  php      -> /opt/php/%s/bin/php\n", version)
	fmt.Printf("  php%s   -> /opt/php/%s/bin/php\n", version, version)
	fmt.Printf("  phpize   -> /opt/php/%s/bin/phpize\n", version)
	fmt.Printf("  php-fpm  -> /opt/php/%s/sbin/php-fpm\n", version)

	// Handle --system flag
	if system {
		fmt.Printf("\n\033[34m==>\033[0m Creating system-wide symlinks in /usr/local/bin...\n")

		// Check for Homebrew conflicts
		conflicts := linker.DetectHomebrewConflicts()
		if len(conflicts) > 0 {
			fmt.Printf("\n\033[33mWarning:\033[0m Detected existing PHP installation (possibly Homebrew):\n")
			for _, c := range conflicts {
				fmt.Printf("  %s -> %s\n", c.Path, c.Target)
			}
			fmt.Printf("\nThese will be overwritten.\n")
		}

		if err := linker.SetSystemDefault(version); err != nil {
			return fmt.Errorf("failed to create system symlinks: %w", err)
		}

		fmt.Printf("\033[32m[OK]\033[0m System symlinks created in /usr/local/bin\n")
	} else {
		fmt.Printf("\n\033[33mNote:\033[0m Add to your shell profile (.zshrc or .bash_profile):\n")
		fmt.Printf("  export PATH=\"/opt/php/bin:$PATH\"\n")
		fmt.Printf("\nOr use --system to create symlinks in /usr/local/bin\n")
	}

	return nil
}

func runUseList() error {
	linker := getLinker()

	current := linker.GetDefaultVersion()
	available := linker.GetAvailableVersions()
	systemLinked := linker.IsSystemLinked()

	fmt.Printf("\n\033[1mPHP Versions\033[0m\n\n")

	if len(available) == 0 {
		fmt.Println("  No PHP versions installed")
		fmt.Println("\n  Install with: phm install php8.5")
		return nil
	}

	for _, v := range available {
		marker := "  "
		if v == current {
			marker = "\033[32m* \033[0m"
		}
		fmt.Printf("%s%s", marker, v)
		if v == current {
			fmt.Printf(" \033[32m(default)\033[0m")
		}
		fmt.Println()
	}

	fmt.Printf("\n\033[1mPaths:\033[0m\n")
	fmt.Printf("  PHM bin:    %s\n", linker.GetPHMBinDir())
	if systemLinked {
		fmt.Printf("  System:     /usr/local/bin \033[32m(linked)\033[0m\n")
	} else {
		fmt.Printf("  System:     /usr/local/bin \033[33m(not linked)\033[0m\n")
	}

	fmt.Printf("\n\033[1mUsage:\033[0m\n")
	fmt.Printf("  phm use <version>          Switch default version\n")
	fmt.Printf("  phm use <version> --system Also link to /usr/local/bin\n")

	if !systemLinked && current != "" {
		fmt.Printf("\n\033[33mTip:\033[0m Add to your PATH: export PATH=\"/opt/php/bin:$PATH\"\n")
	}

	return nil
}

// getFpmManager returns an FPM manager instance
func getFpmManager() *pkg.FPMManager {
	return pkg.NewFPMManager(cfg.InstallPrefix)
}

func runFpmStatus() error {
	fpm := getFpmManager()
	statuses := fpm.GetAllStatus()

	fmt.Printf("\n\033[1mPHP-FPM Status\033[0m\n\n")

	if len(statuses) == 0 {
		fmt.Println("  No PHP-FPM installations found")
		return nil
	}

	fmt.Printf("  \033[1m%-10s %-10s %-8s %-35s %s\033[0m\n", "Version", "Status", "PID", "Socket", "Boot")
	fmt.Printf("  %-10s %-10s %-8s %-35s %s\n", "-------", "------", "---", "------", "----")

	for _, s := range statuses {
		status := "\033[31mstopped\033[0m"
		pid := "-"
		if s.Running {
			status = "\033[32mrunning\033[0m"
			pid = fmt.Sprintf("%d", s.PID)
		}

		boot := "\033[33mdisabled\033[0m"
		if s.Enabled {
			boot = "\033[32menabled\033[0m"
		}

		fmt.Printf("  %-10s %-19s %-8s %-35s %s\n", s.Version, status, pid, s.Socket, boot)
	}

	fmt.Printf("\n  Manage with: phm fpm <start|stop|restart|enable|disable> <version>\n")
	return nil
}

func runFpm(action, version string) error {
	fpm := getFpmManager()

	// Actions that don't require version
	if action == "status" {
		return runFpmStatus()
	}

	// All other actions require version
	if version == "" {
		return fmt.Errorf("version required for action '%s'", action)
	}

	if !fpm.IsInstalled(version) {
		return fmt.Errorf("PHP-FPM %s is not installed", version)
	}

	switch action {
	case "start":
		fmt.Printf("\033[34m==>\033[0m Starting PHP-FPM %s...\n", version)
		if err := fpm.Start(version); err != nil {
			return err
		}
		fmt.Printf("\033[32m[OK]\033[0m PHP-FPM %s started\n", version)
		fmt.Printf("     Socket: %s\n", fpm.GetSocketPath(version))

	case "stop":
		fmt.Printf("\033[34m==>\033[0m Stopping PHP-FPM %s...\n", version)
		if err := fpm.Stop(version); err != nil {
			return err
		}
		fmt.Printf("\033[32m[OK]\033[0m PHP-FPM %s stopped\n", version)

	case "restart":
		fmt.Printf("\033[34m==>\033[0m Restarting PHP-FPM %s...\n", version)
		if err := fpm.Restart(version); err != nil {
			return err
		}
		fmt.Printf("\033[32m[OK]\033[0m PHP-FPM %s restarted\n", version)

	case "reload":
		fmt.Printf("\033[34m==>\033[0m Reloading PHP-FPM %s configuration...\n", version)
		if err := fpm.Reload(version); err != nil {
			return err
		}
		fmt.Printf("\033[32m[OK]\033[0m PHP-FPM %s configuration reloaded\n", version)

	case "enable":
		fmt.Printf("\033[34m==>\033[0m Enabling PHP-FPM %s at boot...\n", version)
		if err := fpm.Enable(version); err != nil {
			return err
		}
		fmt.Printf("\033[32m[OK]\033[0m PHP-FPM %s will start at boot\n", version)

	case "disable":
		fmt.Printf("\033[34m==>\033[0m Disabling PHP-FPM %s at boot...\n", version)
		if err := fpm.Disable(version); err != nil {
			return err
		}
		fmt.Printf("\033[32m[OK]\033[0m PHP-FPM %s will not start at boot\n", version)

	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	return nil
}

func newExtCmd() *cobra.Command {
	var sapi string
	var phpVersion string

	cmd := &cobra.Command{
		Use:   "ext <action> [extension]",
		Short: "Manage PHP extensions",
		Long: `Manage PHP extensions for CLI and FPM SAPIs.

Actions:
  list                   List available extensions and their status
  enable <extension>     Enable an extension
  disable <extension>    Disable an extension

Options:
  --sapi      SAPI to affect: cli, fpm, or all (default: all)
  --version   PHP version (default: current default version)

Examples:
  phm ext list                       # List all extensions
  phm ext list --version=8.5         # List extensions for PHP 8.5
  phm ext enable opcache             # Enable opcache for all SAPIs
  phm ext enable xdebug --sapi=cli   # Enable xdebug for CLI only
  phm ext disable xdebug --sapi=fpm  # Disable xdebug for FPM only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("action required (list, enable, disable)")
			}
			action := args[0]
			extension := ""
			if len(args) > 1 {
				extension = args[1]
			}
			return runExt(action, extension, sapi, phpVersion)
		},
	}

	cmd.Flags().StringVar(&sapi, "sapi", "all", "SAPI to affect (cli, fpm, all)")
	cmd.Flags().StringVar(&phpVersion, "version", "", "PHP version")

	return cmd
}

// getExtManager returns an extension manager instance
func getExtManager() *pkg.ExtensionManager {
	return pkg.NewExtensionManager(cfg.InstallPrefix)
}

func runExt(action, extension, sapi, version string) error {
	extMgr := getExtManager()
	linker := getLinker()

	// Use default version if not specified
	if version == "" {
		version = linker.GetDefaultVersion()
		if version == "" {
			// Try to find any installed version
			versions := extMgr.GetInstalledVersions()
			if len(versions) == 0 {
				return fmt.Errorf("no PHP versions installed")
			}
			version = versions[0]
		}
	}

	switch action {
	case "list", "ls":
		return runExtList(extMgr, version)

	case "enable":
		if extension == "" {
			return fmt.Errorf("extension name required")
		}
		return runExtEnable(extMgr, version, extension, sapi)

	case "disable":
		if extension == "" {
			return fmt.Errorf("extension name required")
		}
		return runExtDisable(extMgr, version, extension, sapi)

	default:
		return fmt.Errorf("unknown action: %s (use list, enable, or disable)", action)
	}
}

func runExtList(extMgr *pkg.ExtensionManager, version string) error {
	extensions, err := extMgr.ListExtensions(version)
	if err != nil {
		return err
	}

	fmt.Printf("\n\033[1mPHP %s Extensions\033[0m\n\n", version)

	if len(extensions) == 0 {
		fmt.Println("  No extensions found")
		return nil
	}

	fmt.Printf("  \033[1m%-20s %-10s\033[0m\n", "Extension", "Status")
	fmt.Printf("  %-20s %-10s\n", strings.Repeat("-", 20), strings.Repeat("-", 10))

	for _, ext := range extensions {
		status := "\033[31mdisabled\033[0m"
		if ext.Enabled {
			status = "\033[32menabled\033[0m"
		}

		fmt.Printf("  %-20s %-19s\n", ext.Name, status)
	}

	fmt.Printf("\n  Enable with:  phm ext enable <extension>\n")
	fmt.Printf("  Disable with: phm ext disable <extension>\n")

	return nil
}

func runExtEnable(extMgr *pkg.ExtensionManager, version, extension, sapi string) error {
	fmt.Printf("\033[34m==>\033[0m Enabling %s (PHP %s)...\n", extension, version)

	if err := extMgr.Enable(version, extension, sapi); err != nil {
		return err
	}

	fmt.Printf("\033[32m[OK]\033[0m %s enabled\n", extension)
	fmt.Printf("\n\033[33mNote:\033[0m Restart PHP-FPM to apply changes: phm fpm restart %s\n", version)

	return nil
}

func runExtDisable(extMgr *pkg.ExtensionManager, version, extension, sapi string) error {
	fmt.Printf("\033[34m==>\033[0m Disabling %s (PHP %s)...\n", extension, version)

	if err := extMgr.Disable(version, extension, sapi); err != nil {
		return err
	}

	fmt.Printf("\033[32m[OK]\033[0m %s disabled\n", extension)
	fmt.Printf("\n\033[33mNote:\033[0m Restart PHP-FPM to apply changes: phm fpm restart %s\n", version)

	return nil
}

func newDestructCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "destruct",
		Short: "Remove all PHP installations and PHM data",
		Long: `Completely remove all PHP versions installed by PHM and all PHM data.

This command will:
  - Stop all PHP-FPM services
  - Remove all PHP installations from /opt/php/
  - Remove PHM symlinks from /opt/php/bin and /usr/local/bin
  - Remove LaunchDaemons for PHP-FPM
  - Remove cache (~/.cache/phm)
  - Remove installed packages database (~/.local/share/phm)
  - Remove configuration (~/.config/phm)
  - Remove PHP-FPM sockets and logs

This does NOT remove the phm binary itself.

Use --force to skip confirmation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDestruct(force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runDestruct(force bool) error {
	homeDir, _ := os.UserHomeDir()

	// Paths to clean
	paths := struct {
		installPrefix string
		phmBinDir     string
		cacheDir      string
		dataDir       string
		configDir     string
		launchDaemons string
		fpmRunDir     string
		fpmLogPattern string
	}{
		installPrefix: cfg.InstallPrefix,
		phmBinDir:     "/opt/php/bin",
		cacheDir:      filepath.Join(homeDir, ".cache", "phm"),
		dataDir:       filepath.Join(homeDir, ".local", "share", "phm"),
		configDir:     filepath.Join(homeDir, ".config", "phm"),
		launchDaemons: "/Library/LaunchDaemons",
		fpmRunDir:     "/var/run/php",
		fpmLogPattern: "/var/log/php*-fpm*",
	}

	fmt.Printf("\n\033[1;31m⚠️  WARNING: This will completely remove all PHM-managed PHP installations!\033[0m\n\n")
	fmt.Println("The following will be removed:")
	fmt.Printf("  • PHP installations:    %s/*\n", paths.installPrefix)
	fmt.Printf("  • PHM symlinks:         %s\n", paths.phmBinDir)
	fmt.Printf("  • Cache:                %s\n", paths.cacheDir)
	fmt.Printf("  • Data:                 %s\n", paths.dataDir)
	fmt.Printf("  • Config:               %s\n", paths.configDir)
	fmt.Printf("  • LaunchDaemons:        %s/com.phm.php*\n", paths.launchDaemons)
	fmt.Printf("  • FPM sockets:          %s\n", paths.fpmRunDir)
	fmt.Printf("  • FPM logs:             %s\n", paths.fpmLogPattern)
	fmt.Println()

	if !force {
		fmt.Print("\033[1mType 'yes' to confirm: \033[0m")
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != "yes" {
			fmt.Println("\nAborted.")
			return nil
		}
	}

	// Prompt for sudo password upfront
	if err := ensureSudo(); err != nil {
		return err
	}

	fmt.Println()

	// 1. Stop all PHP-FPM services
	fmt.Printf("\033[34m==>\033[0m Stopping PHP-FPM services...\n")
	fpm := getFpmManager()
	statuses := fpm.GetAllStatus()
	for _, s := range statuses {
		if s.Running {
			fmt.Printf("    Stopping PHP-FPM %s...\n", s.Version)
			_ = fpm.Stop(s.Version)
		}
		if s.Enabled {
			fmt.Printf("    Disabling PHP-FPM %s...\n", s.Version)
			_ = fpm.Disable(s.Version)
		}
	}

	// 2. Remove LaunchDaemons
	fmt.Printf("\033[34m==>\033[0m Removing LaunchDaemons...\n")
	launchDaemonFiles, _ := filepath.Glob(filepath.Join(paths.launchDaemons, "com.phm.php*.plist"))
	for _, f := range launchDaemonFiles {
		fmt.Printf("    Removing %s\n", f)
		_ = runSudo("rm", "-f", f)
	}

	// 3. Remove symlinks from /usr/local/bin (only phm-created ones)
	fmt.Printf("\033[34m==>\033[0m Removing symlinks from /usr/local/bin...\n")
	phpBinaries := []string{"php", "phpize", "php-config", "php-cgi", "php-fpm", "pecl", "pear", "phpdbg"}
	for _, bin := range phpBinaries {
		symlink := filepath.Join("/usr/local/bin", bin)
		if target, err := os.Readlink(symlink); err == nil {
			if strings.Contains(target, "/opt/php") {
				fmt.Printf("    Removing %s -> %s\n", symlink, target)
				_ = runSudo("rm", "-f", symlink)
			}
		}
	}

	// 4. Remove /opt/php/bin (PHM symlink directory)
	fmt.Printf("\033[34m==>\033[0m Removing PHM bin directory...\n")
	if _, err := os.Stat(paths.phmBinDir); err == nil {
		fmt.Printf("    Removing %s\n", paths.phmBinDir)
		_ = runSudo("rm", "-rf", paths.phmBinDir)
	}

	// 5. Remove all PHP installations
	fmt.Printf("\033[34m==>\033[0m Removing PHP installations...\n")
	phpDirs, _ := filepath.Glob(filepath.Join(paths.installPrefix, "*"))
	versionPattern := regexp.MustCompile(`^\d+\.\d+$`)
	for _, dir := range phpDirs {
		if dir == paths.phmBinDir {
			continue // Already removed
		}
		// Only remove version directories (8.3, 8.4, 8.5, etc.)
		base := filepath.Base(dir)
		if versionPattern.MatchString(base) {
			fmt.Printf("    Removing %s\n", dir)
			_ = runSudo("rm", "-rf", dir)
		}
	}

	// 6. Remove FPM run directory
	fmt.Printf("\033[34m==>\033[0m Removing FPM sockets...\n")
	if _, err := os.Stat(paths.fpmRunDir); err == nil {
		fmt.Printf("    Removing %s\n", paths.fpmRunDir)
		_ = runSudo("rm", "-rf", paths.fpmRunDir)
	}

	// 7. Remove FPM logs
	fmt.Printf("\033[34m==>\033[0m Removing FPM logs...\n")
	fpmLogs, _ := filepath.Glob(paths.fpmLogPattern)
	for _, f := range fpmLogs {
		fmt.Printf("    Removing %s\n", f)
		_ = runSudo("rm", "-f", f)
	}

	// 8. Remove user directories (no sudo needed)
	fmt.Printf("\033[34m==>\033[0m Removing PHM data directories...\n")

	if _, err := os.Stat(paths.cacheDir); err == nil {
		fmt.Printf("    Removing %s\n", paths.cacheDir)
		os.RemoveAll(paths.cacheDir)
	}

	if _, err := os.Stat(paths.dataDir); err == nil {
		fmt.Printf("    Removing %s\n", paths.dataDir)
		os.RemoveAll(paths.dataDir)
	}

	if _, err := os.Stat(paths.configDir); err == nil {
		fmt.Printf("    Removing %s\n", paths.configDir)
		os.RemoveAll(paths.configDir)
	}

	fmt.Println()
	fmt.Println("\033[32m[OK]\033[0m All PHM-managed PHP installations have been removed.")
	fmt.Println()
	fmt.Println("To reinstall PHP:")
	fmt.Println("  phm update")
	fmt.Println("  phm install php8.5-cli php8.5-fpm")
	fmt.Println()
	fmt.Println("To remove phm itself:")
	fmt.Println("  sudo rm /usr/local/bin/phm")

	return nil
}

// runSudo executes a command with sudo
func runSudo(command string, args ...string) error {
	allArgs := append([]string{command}, args...)
	cmd := exec.Command("sudo", allArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func newSelfUpdateCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update phm to the latest version",
		Long: `Update phm CLI tool to the latest version from GitHub releases.

This command will:
  - Check the latest available version
  - Download the new binary if a newer version exists
  - Replace the current binary

Examples:
  phm self-update          # Update to latest version
  phm self-update --force  # Force update even if already up-to-date`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSelfUpdate(force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force update even if already up-to-date")

	return cmd
}

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runSelfUpdate(force bool) error {
	fmt.Printf("\033[34m==>\033[0m Checking for updates...\n")

	// Get current version
	currentVersion := version
	if currentVersion == "dev" {
		fmt.Printf("\033[33m[!]\033[0m Running development version\n")
		if !force {
			fmt.Println("Use --force to update from dev version")
			return nil
		}
	}

	// Fetch latest release from GitHub
	resp, err := http.Get("https://api.github.com/repos/phm-dev/phm/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to check for updates: HTTP %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	// Parse version (remove 'v' prefix if present)
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersionClean := strings.TrimPrefix(currentVersion, "v")

	fmt.Printf("    Current version: %s\n", currentVersion)
	fmt.Printf("    Latest version:  %s\n", latestVersion)

	// Compare versions
	if !force && currentVersionClean != "dev" {
		cmp := pkg.CompareVersions(latestVersion, currentVersionClean)
		if cmp <= 0 {
			fmt.Printf("\n\033[32m[OK]\033[0m Already up-to-date\n")
			return nil
		}
	}

	// Determine architecture
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	} else {
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Find the right asset (tag includes 'v' prefix)
	assetName := fmt.Sprintf("phm-%s-darwin-%s.tar.gz", release.TagName, arch)
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find release asset: %s", assetName)
	}

	fmt.Printf("\n\033[34m==>\033[0m Downloading %s...\n", assetName)

	// Download to temp file
	tmpDir, err := os.MkdirTemp("", "phm-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(tarPath, downloadURL); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	fmt.Printf("\033[34m==>\033[0m Extracting...\n")

	// Extract tarball
	binaryPath := filepath.Join(tmpDir, "phm")
	if err := extractTarGz(tarPath, tmpDir); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	// Verify the binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("binary not found in archive")
	}

	// Get current binary path
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current binary path: %w", err)
	}

	// Resolve symlinks
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	fmt.Printf("\033[34m==>\033[0m Installing to %s...\n", currentBinary)

	// Check if we need sudo
	needsSudo := false
	if err := os.Rename(binaryPath, currentBinary); err != nil {
		// Try with sudo
		needsSudo = true
	}

	if needsSudo {
		// Make the new binary executable first
		if err := os.Chmod(binaryPath, 0755); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}

		// Use sudo to copy
		if err := runSudo("cp", binaryPath, currentBinary); err != nil {
			return fmt.Errorf("failed to install binary: %w", err)
		}
	}

	fmt.Printf("\n\033[32m[OK]\033[0m Successfully updated to version %s\n", latestVersion)

	// Verify installation
	cmd := exec.Command(currentBinary, "--version")
	if output, err := cmd.Output(); err == nil {
		fmt.Printf("    %s", string(output))
	}

	return nil
}

// downloadFile downloads a file from URL to the specified path
func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractTarGz extracts a .tar.gz file to the specified directory
func extractTarGz(tarPath, destDir string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			// Preserve executable permissions
			if header.Mode&0111 != 0 {
				if err := os.Chmod(target, 0755); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
