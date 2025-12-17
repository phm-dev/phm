package pkg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FPMManager manages PHP-FPM services
type FPMManager struct {
	installPrefix string
}

// FPMStatus represents the status of a PHP-FPM service
type FPMStatus struct {
	Version string
	Running bool
	PID     int
	Socket  string
	Enabled bool
}

// NewFPMManager creates a new FPM manager
func NewFPMManager(installPrefix string) *FPMManager {
	return &FPMManager{
		installPrefix: installPrefix,
	}
}

// EnsureSudo prompts for sudo password if needed and caches credentials
// Returns true if sudo is available, false otherwise
func (f *FPMManager) EnsureSudo() bool {
	// Clear terminal and show message
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to top
	fmt.Println("PHP-FPM Management requires administrator privileges.")
	fmt.Println("Please enter your password when prompted.")
	fmt.Println()

	// Run sudo -v to prompt for password and cache credentials
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false
	}

	// Clear screen after successful auth
	fmt.Print("\033[2J\033[H")
	return true
}

// GetServiceName returns the launchd service name for a PHP version
func (f *FPMManager) GetServiceName(version string) string {
	return fmt.Sprintf("com.phm.php%s-fpm", version)
}

// GetPlistPath returns the path to the launchd plist
func (f *FPMManager) GetPlistPath(version string) string {
	return fmt.Sprintf("/Library/LaunchDaemons/com.phm.php%s-fpm.plist", version)
}

// GetSocketPath returns the socket path for a PHP version
func (f *FPMManager) GetSocketPath(version string) string {
	return fmt.Sprintf("/var/run/php/php%s-fpm.sock", version)
}

// GetPIDPath returns the PID file path for a PHP version
func (f *FPMManager) GetPIDPath(version string) string {
	return fmt.Sprintf("/var/run/php/php%s-fpm.pid", version)
}

// IsInstalled checks if PHP-FPM is installed for a version
func (f *FPMManager) IsInstalled(version string) bool {
	fpmBin := filepath.Join(f.installPrefix, version, "sbin", "php-fpm")
	_, err := os.Stat(fpmBin)
	return err == nil
}

// IsRunning checks if PHP-FPM is running for a version
func (f *FPMManager) IsRunning(version string) bool {
	// Method 1: Check PID file
	pidFile := f.GetPIDPath(version)
	data, err := os.ReadFile(pidFile)
	if err == nil {
		pid := strings.TrimSpace(string(data))
		if pid != "" {
			cmd := exec.Command("kill", "-0", pid)
			if cmd.Run() == nil {
				return true
			}
		}
	}

	// Method 2: Check if process is running via pgrep
	fpmBin := filepath.Join(f.installPrefix, version, "sbin", "php-fpm")
	cmd := exec.Command("pgrep", "-f", fpmBin)
	return cmd.Run() == nil
}

// GetPID returns the PID of running PHP-FPM
func (f *FPMManager) GetPID(version string) int {
	pidFile := f.GetPIDPath(version)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// IsEnabled checks if the service is enabled (RunAtLoad)
func (f *FPMManager) IsEnabled(version string) bool {
	plistPath := f.GetPlistPath(version)

	// Check if plist exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return false
	}

	// Check RunAtLoad value in plist (without sudo)
	cmd := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :RunAtLoad", plistPath)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// Start starts PHP-FPM for a version
func (f *FPMManager) Start(version string) error {
	if !f.IsInstalled(version) {
		return fmt.Errorf("PHP-FPM %s is not installed", version)
	}

	if f.IsRunning(version) {
		return fmt.Errorf("PHP-FPM %s is already running", version)
	}

	// Ensure run directory exists
	runDir := "/var/run/php"
	cmd := exec.Command("sudo", "mkdir", "-p", runDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create run dir %s: %w", runDir, err)
	}
	cmd = exec.Command("sudo", "chmod", "755", runDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", runDir, err)
	}

	// Load and start the service
	plistPath := f.GetPlistPath(version)
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("LaunchDaemon plist not found: %s", plistPath)
	}

	// Bootstrap the service (macOS 10.10+)
	cmd = exec.Command("sudo", "launchctl", "bootstrap", "system", plistPath)
	if err := cmd.Run(); err != nil {
		// Try legacy load command
		cmd = exec.Command("sudo", "launchctl", "load", plistPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}
	}

	return nil
}

// Stop stops PHP-FPM for a version
func (f *FPMManager) Stop(version string) error {
	if !f.IsRunning(version) {
		return fmt.Errorf("PHP-FPM %s is not running", version)
	}

	serviceName := f.GetServiceName(version)
	plistPath := f.GetPlistPath(version)

	// Bootout the service (macOS 10.10+)
	cmd := exec.Command("sudo", "launchctl", "bootout", "system/"+serviceName)
	if err := cmd.Run(); err != nil {
		// Try legacy unload command
		cmd = exec.Command("sudo", "launchctl", "unload", plistPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
	}

	return nil
}

// Restart restarts PHP-FPM for a version
func (f *FPMManager) Restart(version string) error {
	if f.IsRunning(version) {
		if err := f.Stop(version); err != nil {
			return err
		}
	}
	return f.Start(version)
}

// Reload reloads PHP-FPM configuration
func (f *FPMManager) Reload(version string) error {
	if !f.IsRunning(version) {
		return fmt.Errorf("PHP-FPM %s is not running", version)
	}

	pid := f.GetPID(version)
	if pid == 0 {
		return fmt.Errorf("could not find PHP-FPM PID")
	}

	cmd := exec.Command("sudo", "kill", "-USR2", fmt.Sprintf("%d", pid))
	return cmd.Run()
}

// Enable enables PHP-FPM to start at boot
func (f *FPMManager) Enable(version string) error {
	plistPath := f.GetPlistPath(version)

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("LaunchDaemon plist not found: %s", plistPath)
	}

	// Modify plist to set RunAtLoad to true
	cmd := exec.Command("sudo", "/usr/libexec/PlistBuddy", "-c", "Set :RunAtLoad true", plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

// Disable disables PHP-FPM from starting at boot
func (f *FPMManager) Disable(version string) error {
	plistPath := f.GetPlistPath(version)

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("LaunchDaemon plist not found: %s", plistPath)
	}

	// Modify plist to set RunAtLoad to false
	cmd := exec.Command("sudo", "/usr/libexec/PlistBuddy", "-c", "Set :RunAtLoad false", plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}

	return nil
}

// GetStatus returns the status of PHP-FPM for a version
func (f *FPMManager) GetStatus(version string) *FPMStatus {
	return &FPMStatus{
		Version: version,
		Running: f.IsRunning(version),
		PID:     f.GetPID(version),
		Socket:  f.GetSocketPath(version),
		Enabled: f.IsEnabled(version),
	}
}

// GetAllStatus returns status of all installed PHP-FPM versions
func (f *FPMManager) GetAllStatus() []*FPMStatus {
	var statuses []*FPMStatus

	entries, err := os.ReadDir(f.installPrefix)
	if err != nil {
		return statuses
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		version := entry.Name()
		// Check if it looks like a version directory
		if len(version) < 3 || version[0] < '0' || version[0] > '9' {
			continue
		}

		if f.IsInstalled(version) {
			statuses = append(statuses, f.GetStatus(version))
		}
	}

	return statuses
}
