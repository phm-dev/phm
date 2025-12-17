package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/phm-dev/phm/internal/config"
	"github.com/phm-dev/phm/internal/pkg"
	"github.com/phm-dev/phm/internal/repo"
	"github.com/phm-dev/phm/internal/tui"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
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
		newUpdateCmd(),
		newUpgradeCmd(),
		newInfoCmd(),
		newUseCmd(),
		newFpmCmd(),
		newExtCmd(),
		newUICmd(),
		newConfigCmd(),
		newDestructCmd(),
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

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update package index",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate()
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

// getRepo creates and initializes repository
func getRepo() (*repo.Repository, error) {
	// If --repo is set, enable offline mode
	if cfg.RepoPath != "" {
		cfg.Offline = true
	}

	r := repo.New(cfg)
	if err := r.LoadIndex(); err != nil {
		// If index doesn't exist, try to fetch it automatically
		fmt.Printf("\033[34m==>\033[0m Package index not found, fetching...\n")
		if fetchErr := r.FetchIndex(); fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch index: %w\n\nRun 'phm update' to download the package index", fetchErr)
		}
		fmt.Printf("\033[32m[OK]\033[0m Package index updated\n\n")
	}
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
	// Always update index before install
	fmt.Println("\033[34m==>\033[0m Updating package index...")
	r := repo.New(cfg)
	if err := r.FetchIndex(); err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}
	fmt.Println("\033[32m[OK]\033[0m Package index updated")
	fmt.Println()

	mgr := getManager()
	if err := mgr.LoadInstalled(); err != nil {
		fmt.Printf("\033[33mWarning:\033[0m Could not load installed packages: %v\n", err)
	}

	linker := getLinker()
	allAvailable := r.GetPackages()

	// Expand meta-packages (slim/full) before processing
	packages = expandMetaPackages(packages, allAvailable)

	// Collect all packages to install (with dependencies resolved)
	var allToInstall []pkg.Package
	seenPackages := make(map[string]bool)
	installedVersions := make(map[string]bool) // Track PHP versions being installed

	for _, name := range packages {
		pkgToInstall := r.GetPackage(name)
		if pkgToInstall == nil {
			fmt.Printf("\033[31mError:\033[0m Package not found: %s\n", name)
			continue
		}

		// Resolve dependencies
		toInstall, err := mgr.ResolveDependencies(pkgToInstall, allAvailable)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to resolve dependencies: %v\n", err)
			continue
		}

		// Add to install list (deduplicated)
		for _, p := range toInstall {
			if !seenPackages[p.Name] {
				seenPackages[p.Name] = true
				allToInstall = append(allToInstall, p)
				// Track PHP version
				if v := extractPHPVersion(p.Name); v != "" {
					installedVersions[v] = true
				}
			}
		}
	}

	if len(allToInstall) == 0 {
		fmt.Println("No packages to install.")
		return nil
	}

	// Show installation plan
	fmt.Printf("\n\033[1mThe following packages will be installed:\033[0m\n")
	for _, p := range allToInstall {
		status := ""
		if mgr.IsInstalled(p.Name) {
			status = " \033[33m(reinstall)\033[0m"
		}
		fmt.Printf("  - %s (%s)%s\n", p.Name, p.Version, status)
	}
	fmt.Println()

	// Install all packages
	for _, p := range allToInstall {
		if mgr.IsInstalled(p.Name) && !force {
			fmt.Printf("\033[33m[SKIP]\033[0m %s already installed\n", p.Name)
			continue
		}

		fmt.Printf("\033[34m==>\033[0m Installing %s (%s)...\n", p.Name, p.Version)

		// Download package
		path, err := r.DownloadPackage(&p)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to download: %v\n", err)
			continue
		}

		// Install package
		_, err = mgr.Install(path)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to install: %v\n", err)
			continue
		}

		fmt.Printf("\033[32m[OK]\033[0m %s installed\n", p.Name)
	}

	// Setup symlinks for all installed PHP versions (once at the end)
	for phpVersion := range installedVersions {
		fmt.Printf("\033[34m==>\033[0m Setting up symlinks for PHP %s...\n", phpVersion)
		if err := linker.SetupVersionLinks(phpVersion); err != nil {
			fmt.Printf("\033[33mWarning:\033[0m Could not create symlinks: %v\n", err)
		} else {
			macportsVer := strings.ReplaceAll(phpVersion, ".", "")
			fmt.Printf("\033[32m[OK]\033[0m Created: php%s, /opt/local/bin/php%s\n", phpVersion, macportsVer)
		}
	}

	// Handle default version (only once at the end)
	// Get first installed version (for single version install) or let user choose
	var targetVersion string
	for v := range installedVersions {
		targetVersion = v
		break
	}

	if targetVersion != "" {
		currentDefault := linker.GetDefaultVersion()
		if currentDefault == "" {
			// First install - auto-set as default
			fmt.Printf("\n\033[34m==>\033[0m Setting PHP %s as default...\n", targetVersion)
			if err := linker.SetDefaultVersion(targetVersion); err != nil {
				fmt.Printf("\033[33mWarning:\033[0m Could not set default: %v\n", err)
			} else {
				fmt.Printf("\033[32m[OK]\033[0m Default set to PHP %s\n", targetVersion)
				fmt.Printf("\n\033[33mNote:\033[0m Add to your PATH: export PATH=\"/opt/php/bin:$PATH\"\n")
				fmt.Printf("      Or run: phm use %s --system\n", targetVersion)
			}
		} else if currentDefault != targetVersion {
			// Different version is default - ask user (once!)
			fmt.Printf("\n\033[33mCurrent default is PHP %s.\033[0m\n", currentDefault)
			fmt.Printf("Set PHP %s as default? [y/N]: ", targetVersion)
			var answer string
			_, _ = fmt.Scanln(&answer)
			if answer == "y" || answer == "Y" || answer == "yes" {
				if err := linker.SetDefaultVersion(targetVersion); err != nil {
					fmt.Printf("\033[33mWarning:\033[0m Could not set default: %v\n", err)
				} else {
					fmt.Printf("\033[32m[OK]\033[0m Default set to PHP %s\n", targetVersion)
				}
			}
		}
	}

	fmt.Println()
	return nil
}

// extractPHPVersion extracts PHP version from package name (e.g., "php8.5-cli" -> "8.5")
func extractPHPVersion(name string) string {
	re := regexp.MustCompile(`^php(\d+\.\d+)`)
	if matches := re.FindStringSubmatch(name); len(matches) > 1 {
		return matches[1]
	}
	return ""
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

func runUpdate() error {
	fmt.Println("\033[34m==>\033[0m Updating package index...")

	r := repo.New(cfg)
	if err := r.FetchIndex(); err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}

	fmt.Println("\033[32m[OK]\033[0m Package index updated")
	return nil
}

func runUpgrade(packages []string) error {
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
		name       string
		oldVersion string
		newVersion string
	}
	var upgrades []upgrade

	fmt.Println("\033[34m==>\033[0m Checking for upgrades...")

	for _, name := range toCheck {
		installed := mgr.GetInstalled(name)
		if installed == nil {
			continue
		}

		available := r.GetPackage(name)
		if available == nil {
			continue
		}

		if newVer := mgr.CheckUpgrade(name, available.Version); newVer != "" {
			upgrades = append(upgrades, upgrade{
				name:       name,
				oldVersion: installed.Version,
				newVersion: newVer,
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
		fmt.Printf("  %s: %s -> %s\n", u.name, u.oldVersion, u.newVersion)
	}
	fmt.Printf("\n%d package(s) to upgrade.\n\n", len(upgrades))

	// Perform upgrades
	linker := getLinker()
	allAvailable := r.GetPackages()

	for _, u := range upgrades {
		pkgToInstall := r.GetPackage(u.name)
		if pkgToInstall == nil {
			continue
		}

		// Resolve dependencies
		toInstall, err := mgr.ResolveDependencies(pkgToInstall, allAvailable)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to resolve dependencies for %s: %v\n", u.name, err)
			continue
		}

		// Install each package (including dependencies that need upgrade)
		for _, p := range toInstall {
			newVer := mgr.CheckUpgrade(p.Name, p.Version)
			if newVer == "" && mgr.IsInstalled(p.Name) {
				continue // Already installed and up to date
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
		phpVersion := extractPHPVersion(u.name)
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
		fmt.Println("  No extensions found in mods-available")
		return nil
	}

	fmt.Printf("  \033[1m%-20s %-10s %-10s\033[0m\n", "Extension", "CLI", "FPM")
	fmt.Printf("  %-20s %-10s %-10s\n", strings.Repeat("-", 20), strings.Repeat("-", 10), strings.Repeat("-", 10))

	for _, ext := range extensions {
		cliStatus := "\033[31mdisabled\033[0m"
		if ext.EnabledCLI {
			cliStatus = "\033[32menabled\033[0m"
		}

		fpmStatus := "\033[31mdisabled\033[0m"
		if ext.EnabledFPM {
			fpmStatus = "\033[32menabled\033[0m"
		}

		fmt.Printf("  %-20s %-19s %-19s\n", ext.Name, cliStatus, fpmStatus)
	}

	fmt.Printf("\n  Enable with:  phm ext enable <extension> [--sapi=cli|fpm|all]\n")
	fmt.Printf("  Disable with: phm ext disable <extension> [--sapi=cli|fpm|all]\n")

	return nil
}

func runExtEnable(extMgr *pkg.ExtensionManager, version, extension, sapi string) error {
	fmt.Printf("\033[34m==>\033[0m Enabling %s for %s (PHP %s)...\n", extension, sapi, version)

	if err := extMgr.Enable(version, extension, sapi); err != nil {
		return err
	}

	fmt.Printf("\033[32m[OK]\033[0m %s enabled\n", extension)

	if sapi == "fpm" || sapi == "all" {
		fmt.Printf("\n\033[33mNote:\033[0m Restart PHP-FPM to apply changes: phm fpm restart %s\n", version)
	}

	return nil
}

func runExtDisable(extMgr *pkg.ExtensionManager, version, extension, sapi string) error {
	fmt.Printf("\033[34m==>\033[0m Disabling %s for %s (PHP %s)...\n", extension, sapi, version)

	if err := extMgr.Disable(version, extension, sapi); err != nil {
		return err
	}

	fmt.Printf("\033[32m[OK]\033[0m %s disabled\n", extension)

	if sapi == "fpm" || sapi == "all" {
		fmt.Printf("\n\033[33mNote:\033[0m Restart PHP-FPM to apply changes: phm fpm restart %s\n", version)
	}

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
