package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phm-dev/phm/internal/config"
	"github.com/phm-dev/phm/internal/pkg"
	"github.com/phm-dev/phm/internal/repo"
)

// WizardStep represents the current step in the wizard
type WizardStep int

const (
	StepMainMenu WizardStep = iota
	StepSelectVersion
	StepSelectSAPIs
	StepSelectExtensions
	StepConfirmInstall
	StepInstalling
	StepSelectInstalledVersion
	StepModifyMenu
	StepModifyExtensions
	StepConfirmRemove
	StepRemoving
	StepUpgrading
	StepFPMMenu
	StepFPMAction
	StepDone
)

// MainMenuOption represents main menu choices
type MainMenuOption int

const (
	MenuInstallPHP MainMenuOption = iota
	MenuModifyPHP
	MenuManageFPM
	MenuUpgradeAll
	MenuRemovePHP
	MenuExit
)

// Model represents the TUI state
type Model struct {
	cfg        *config.Config
	repo       *repo.Repository
	manager    *pkg.Manager
	linker     *pkg.Linker
	fpmManager *pkg.FPMManager

	// Wizard state
	step       WizardStep
	cursor     int
	message    string
	err        error
	menuAction MainMenuOption

	// Main menu
	mainMenuOptions []string

	// Installation wizard
	availableVersions []string
	selectedVersion   string
	sapiOptions       []string
	selectedSAPIs     map[int]bool
	extOptions        []extOption
	selectedExts      map[int]bool

	// Modify wizard
	installedVersions []string

	// FPM management
	fpmStatuses      []*pkg.FPMStatus
	selectedFPMIdx   int
	fpmActionOptions []string

	// Progress
	spinner spinner.Model
	logs    []string

	// Dimensions
	width  int
	height int
}

type extOption struct {
	name string
	desc string
}

// NewModel creates a new TUI model
func NewModel(cfg *config.Config) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	m := &Model{
		cfg:     cfg,
		step:    StepMainMenu,
		spinner: s,
		mainMenuOptions: []string{
			"Install PHP",
			"Modify installed PHP",
			"Manage PHP-FPM",
			"Upgrade all packages",
			"Remove PHP version",
			"Exit",
		},
		sapiOptions: []string{
			"cli - Command line interface",
			"fpm - FastCGI Process Manager (for web servers)",
			"cgi - CGI binary",
		},
		fpmActionOptions: []string{
			"Start",
			"Stop",
			"Restart",
			"Enable (start at boot)",
			"Disable (don't start at boot)",
			"Back to FPM list",
		},
		selectedSAPIs: make(map[int]bool),
		selectedExts:  make(map[int]bool),
		width:         80,
		height:        24,
	}

	// Pre-select CLI by default
	m.selectedSAPIs[0] = true

	return m
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadData,
	)
}

// loadData loads repository and installed packages
func (m *Model) loadData() tea.Msg {
	// Load repository
	m.repo = repo.New(m.cfg)
	if err := m.repo.LoadIndex(); err != nil {
		return errMsg{err}
	}

	// Load manager
	m.manager = pkg.NewManager(m.cfg.InstallPrefix, m.cfg.DataDir)
	_ = m.manager.LoadInstalled()

	// Load linker
	m.linker = pkg.NewLinker(m.cfg.InstallPrefix)

	// Load FPM manager
	m.fpmManager = pkg.NewFPMManager(m.cfg.InstallPrefix)

	return dataLoadedMsg{}
}

type dataLoadedMsg struct{}
type errMsg struct{ err error }
type installDoneMsg struct{ err error }
type upgradeDoneMsg struct{ err error }
type removeDoneMsg struct{ err error }
type fpmActionDoneMsg struct {
	action  string
	version string
	err     error
}

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case dataLoadedMsg:
		m.loadVersions()
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case installDoneMsg:
		m.step = StepDone
		if msg.err != nil {
			m.message = fmt.Sprintf("Installation failed: %v", msg.err)
		} else {
			m.message = "Installation completed successfully!"
		}
		return m, nil

	case upgradeDoneMsg:
		m.step = StepDone
		if msg.err != nil {
			m.message = fmt.Sprintf("Upgrade failed: %v", msg.err)
		} else {
			m.message = "Upgrade completed successfully!"
		}
		return m, nil

	case removeDoneMsg:
		m.step = StepDone
		if msg.err != nil {
			m.message = fmt.Sprintf("Removal failed: %v", msg.err)
		} else {
			m.message = "PHP removed successfully!"
		}
		m.loadInstalledVersions()
		return m, nil

	case fpmActionDoneMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("FPM %s failed: %v", msg.action, msg.err)
		} else {
			m.message = fmt.Sprintf("PHP-FPM %s: %s successful", msg.version, msg.action)
		}
		m.loadFPMStatuses()
		m.step = StepFPMMenu
		m.cursor = 0
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.step == StepInstalling || m.step == StepUpgrading || m.step == StepRemoving {
			return m, nil // Don't quit during operations
		}
		return m, tea.Quit

	case "esc":
		return m.goBack()

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		m.cursor = min(m.cursor+1, m.getMaxCursor())

	case " ":
		m.toggleSelection()

	case "enter":
		return m.handleEnter()

	case "a":
		// Select all in multi-select steps
		if m.step == StepSelectExtensions {
			for i := range m.extOptions {
				m.selectedExts[i] = true
			}
		}

	case "n":
		// Deselect all in multi-select steps
		if m.step == StepSelectExtensions {
			m.selectedExts = make(map[int]bool)
		}
	}

	return m, nil
}

func (m *Model) goBack() (tea.Model, tea.Cmd) {
	m.message = ""
	switch m.step {
	case StepSelectVersion:
		m.step = StepMainMenu
		m.cursor = 0
	case StepSelectSAPIs:
		m.step = StepSelectVersion
		m.cursor = 0
	case StepSelectExtensions:
		m.step = StepSelectSAPIs
		m.cursor = 0
	case StepConfirmInstall:
		m.step = StepSelectExtensions
		m.cursor = 0
	case StepSelectInstalledVersion:
		m.step = StepMainMenu
		m.cursor = 0
	case StepModifyMenu:
		m.step = StepSelectInstalledVersion
		m.cursor = 0
	case StepModifyExtensions:
		m.step = StepModifyMenu
		m.cursor = 0
	case StepConfirmRemove:
		m.step = StepSelectInstalledVersion
		m.cursor = 0
	case StepFPMMenu:
		m.step = StepMainMenu
		m.cursor = 0
	case StepFPMAction:
		m.step = StepFPMMenu
		m.cursor = 0
	case StepDone:
		m.step = StepMainMenu
		m.cursor = 0
		m.message = ""
		m.logs = nil
	}
	return m, nil
}

func (m *Model) handleEnter() (tea.Model, tea.Cmd) {
	m.message = ""
	switch m.step {
	case StepMainMenu:
		return m.handleMainMenu()
	case StepSelectVersion:
		return m.handleSelectVersion()
	case StepSelectSAPIs:
		return m.handleSelectSAPIs()
	case StepSelectExtensions:
		return m.handleSelectExtensions()
	case StepConfirmInstall:
		return m.handleConfirmInstall()
	case StepSelectInstalledVersion:
		return m.handleSelectInstalledVersion()
	case StepModifyMenu:
		return m.handleModifyMenu()
	case StepConfirmRemove:
		return m.handleConfirmRemove()
	case StepFPMMenu:
		return m.handleFPMMenu()
	case StepFPMAction:
		return m.handleFPMAction()
	case StepDone:
		m.step = StepMainMenu
		m.cursor = 0
		m.message = ""
		m.logs = nil
	}
	return m, nil
}

func (m *Model) handleMainMenu() (tea.Model, tea.Cmd) {
	m.menuAction = MainMenuOption(m.cursor)
	switch m.menuAction {
	case MenuInstallPHP:
		m.step = StepSelectVersion
		m.cursor = 0
		m.selectedSAPIs = map[int]bool{0: true}
		m.selectedExts = make(map[int]bool)
	case MenuModifyPHP:
		m.loadInstalledVersions()
		if len(m.installedVersions) == 0 {
			m.message = "No PHP versions installed. Install one first!"
			return m, nil
		}
		m.menuAction = MenuModifyPHP
		m.step = StepSelectInstalledVersion
		m.cursor = 0
	case MenuManageFPM:
		m.loadFPMStatuses()
		if len(m.fpmStatuses) == 0 {
			m.message = "No PHP-FPM installed. Install php-fpm first!"
			return m, nil
		}
		// Ensure sudo credentials are cached before entering FPM menu
		if !m.fpmManager.EnsureSudo() {
			m.message = "Administrator privileges required for FPM management"
			return m, nil
		}
		m.step = StepFPMMenu
		m.cursor = 0
	case MenuUpgradeAll:
		m.step = StepUpgrading
		m.logs = []string{}
		return m, tea.Batch(m.spinner.Tick, m.doUpgrade)
	case MenuRemovePHP:
		m.loadInstalledVersions()
		if len(m.installedVersions) == 0 {
			m.message = "No PHP versions installed"
			return m, nil
		}
		m.menuAction = MenuRemovePHP
		m.step = StepSelectInstalledVersion
		m.cursor = 0
	case MenuExit:
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) handleSelectVersion() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.availableVersions) {
		m.selectedVersion = m.availableVersions[m.cursor]
		m.loadExtensions()
		m.step = StepSelectSAPIs
		m.cursor = 0
	}
	return m, nil
}

func (m *Model) handleSelectSAPIs() (tea.Model, tea.Cmd) {
	hasSelection := false
	for _, v := range m.selectedSAPIs {
		if v {
			hasSelection = true
			break
		}
	}
	if !hasSelection {
		m.message = "Please select at least one SAPI (use SPACE to toggle)"
		return m, nil
	}
	m.step = StepSelectExtensions
	m.cursor = 0
	return m, nil
}

func (m *Model) handleSelectExtensions() (tea.Model, tea.Cmd) {
	m.step = StepConfirmInstall
	m.cursor = 0
	return m, nil
}

func (m *Model) handleConfirmInstall() (tea.Model, tea.Cmd) {
	if m.cursor == 0 {
		m.step = StepInstalling
		m.logs = []string{}
		return m, tea.Batch(m.spinner.Tick, m.doInstall)
	}
	m.step = StepSelectExtensions
	m.cursor = 0
	return m, nil
}

func (m *Model) handleSelectInstalledVersion() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.installedVersions) {
		m.selectedVersion = m.installedVersions[m.cursor]
		if m.menuAction == MenuRemovePHP {
			m.step = StepConfirmRemove
			m.cursor = 0
		} else {
			m.loadExtensions()
			m.step = StepModifyMenu
			m.cursor = 0
		}
	}
	return m, nil
}

func (m *Model) handleModifyMenu() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0: // Add extensions
		m.loadExtensions()
		m.step = StepModifyExtensions
		m.cursor = 0
	case 1: // Set as default
		_ = m.linker.SetDefaultVersion(m.selectedVersion)
		m.message = fmt.Sprintf("PHP %s is now the default version", m.selectedVersion)
		m.step = StepMainMenu
		m.cursor = 0
	}
	return m, nil
}

func (m *Model) handleConfirmRemove() (tea.Model, tea.Cmd) {
	if m.cursor == 0 {
		m.step = StepRemoving
		m.logs = []string{}
		return m, tea.Batch(m.spinner.Tick, m.doRemove)
	}
	m.step = StepSelectInstalledVersion
	m.cursor = 0
	return m, nil
}

func (m *Model) handleFPMMenu() (tea.Model, tea.Cmd) {
	if m.cursor < len(m.fpmStatuses) {
		m.selectedFPMIdx = m.cursor
		m.step = StepFPMAction
		m.cursor = 0
	}
	return m, nil
}

func (m *Model) handleFPMAction() (tea.Model, tea.Cmd) {
	if m.selectedFPMIdx >= len(m.fpmStatuses) {
		return m, nil
	}

	status := m.fpmStatuses[m.selectedFPMIdx]
	version := status.Version

	switch m.cursor {
	case 0: // Start
		return m, m.doFPMAction("start", version)
	case 1: // Stop
		return m, m.doFPMAction("stop", version)
	case 2: // Restart
		return m, m.doFPMAction("restart", version)
	case 3: // Enable
		return m, m.doFPMAction("enable", version)
	case 4: // Disable
		return m, m.doFPMAction("disable", version)
	case 5: // Back
		m.step = StepFPMMenu
		m.cursor = 0
	}
	return m, nil
}

func (m *Model) doFPMAction(action, version string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch action {
		case "start":
			err = m.fpmManager.Start(version)
		case "stop":
			err = m.fpmManager.Stop(version)
		case "restart":
			err = m.fpmManager.Restart(version)
		case "enable":
			err = m.fpmManager.Enable(version)
		case "disable":
			err = m.fpmManager.Disable(version)
		}
		// Wait a moment for the service to fully start/stop
		if err == nil && (action == "start" || action == "stop" || action == "restart") {
			time.Sleep(500 * time.Millisecond)
		}
		return fpmActionDoneMsg{action: action, version: version, err: err}
	}
}

func (m *Model) toggleSelection() {
	switch m.step {
	case StepSelectSAPIs:
		m.selectedSAPIs[m.cursor] = !m.selectedSAPIs[m.cursor]
	case StepSelectExtensions, StepModifyExtensions:
		m.selectedExts[m.cursor] = !m.selectedExts[m.cursor]
	}
}

func (m *Model) getMaxCursor() int {
	switch m.step {
	case StepMainMenu:
		return len(m.mainMenuOptions) - 1
	case StepSelectVersion:
		return max(0, len(m.availableVersions)-1)
	case StepSelectSAPIs:
		return len(m.sapiOptions) - 1
	case StepSelectExtensions, StepModifyExtensions:
		return max(0, len(m.extOptions)-1)
	case StepConfirmInstall, StepConfirmRemove:
		return 1
	case StepSelectInstalledVersion:
		return max(0, len(m.installedVersions)-1)
	case StepModifyMenu:
		return 1
	case StepFPMMenu:
		return max(0, len(m.fpmStatuses)-1)
	case StepFPMAction:
		return len(m.fpmActionOptions) - 1
	}
	return 0
}

func (m *Model) loadVersions() {
	versions := make(map[string]bool)
	if m.repo != nil {
		for _, p := range m.repo.GetPackages() {
			if strings.HasPrefix(p.Name, "php") && strings.Contains(p.Name, "-common") {
				parts := strings.Split(p.Name, "-")
				if len(parts) >= 2 {
					ver := strings.TrimPrefix(parts[0], "php")
					versions[ver] = true
				}
			}
		}
	}

	m.availableVersions = nil
	for v := range versions {
		m.availableVersions = append(m.availableVersions, v)
	}
	// Sort descending
	for i := 0; i < len(m.availableVersions); i++ {
		for j := i + 1; j < len(m.availableVersions); j++ {
			if m.availableVersions[i] < m.availableVersions[j] {
				m.availableVersions[i], m.availableVersions[j] = m.availableVersions[j], m.availableVersions[i]
			}
		}
	}
}

func (m *Model) loadInstalledVersions() {
	m.installedVersions = m.linker.GetAvailableVersions()
}

func (m *Model) loadFPMStatuses() {
	if m.fpmManager != nil {
		m.fpmStatuses = m.fpmManager.GetAllStatus()
	}
}

func (m *Model) loadExtensions() {
	m.extOptions = nil
	m.selectedExts = make(map[int]bool)
	if m.repo == nil {
		return
	}

	prefix := fmt.Sprintf("php%s-", m.selectedVersion)
	seen := make(map[string]bool)

	for _, p := range m.repo.GetPackages() {
		if strings.HasPrefix(p.Name, prefix) {
			ext := strings.TrimPrefix(p.Name, prefix)
			if ext == "cli" || ext == "fpm" || ext == "cgi" || ext == "common" || ext == "dev" || ext == "pear" {
				continue
			}
			if !seen[ext] {
				seen[ext] = true
				desc := p.Description
				if len(desc) > 35 {
					desc = desc[:32] + "..."
				}
				m.extOptions = append(m.extOptions, extOption{name: ext, desc: desc})
			}
		}
	}
}

func (m *Model) doInstall() tea.Msg {
	if m.repo == nil {
		return installDoneMsg{err: fmt.Errorf("repository not loaded")}
	}

	var packages []string
	packages = append(packages, fmt.Sprintf("php%s-common", m.selectedVersion))

	sapiNames := []string{"cli", "fpm", "cgi"}
	for i, selected := range m.selectedSAPIs {
		if selected && i < len(sapiNames) {
			packages = append(packages, fmt.Sprintf("php%s-%s", m.selectedVersion, sapiNames[i]))
		}
	}

	for i, selected := range m.selectedExts {
		if selected && i < len(m.extOptions) {
			packages = append(packages, fmt.Sprintf("php%s-%s", m.selectedVersion, m.extOptions[i].name))
		}
	}

	allAvailable := m.repo.GetPackages()
	for _, pkgName := range packages {
		pkgToInstall := m.repo.GetPackage(pkgName)
		if pkgToInstall == nil {
			continue
		}

		toInstall, err := m.manager.ResolveDependencies(pkgToInstall, allAvailable)
		if err != nil {
			return installDoneMsg{err: err}
		}

		for _, p := range toInstall {
			if m.manager.IsInstalled(p.Name) {
				continue
			}

			m.logs = append(m.logs, fmt.Sprintf("Installing %s...", p.Name))

			path, err := m.repo.DownloadPackage(&p)
			if err != nil {
				return installDoneMsg{err: err}
			}

			if _, err := m.manager.Install(path); err != nil {
				return installDoneMsg{err: err}
			}
		}
	}

	_ = m.linker.SetupVersionLinks(m.selectedVersion)
	if m.linker.GetDefaultVersion() == "" {
		_ = m.linker.SetDefaultVersion(m.selectedVersion)
	}

	m.logs = append(m.logs, "Done!")
	return installDoneMsg{err: nil}
}

func (m *Model) doUpgrade() tea.Msg {
	if m.repo == nil {
		return upgradeDoneMsg{err: fmt.Errorf("repository not loaded")}
	}

	installed := m.manager.GetAllInstalled()
	if len(installed) == 0 {
		m.logs = append(m.logs, "No packages installed")
		return upgradeDoneMsg{err: nil}
	}

	allAvailable := m.repo.GetPackages()
	upgraded := 0

	for _, inst := range installed {
		available := m.repo.GetPackage(inst.Name)
		if available == nil {
			continue
		}

		if pkg.CompareVersions(available.Version, inst.Version) > 0 {
			m.logs = append(m.logs, fmt.Sprintf("Upgrading %s: %s -> %s", inst.Name, inst.Version, available.Version))

			toInstall, err := m.manager.ResolveDependencies(available, allAvailable)
			if err != nil {
				continue
			}

			for _, p := range toInstall {
				path, err := m.repo.DownloadPackage(&p)
				if err != nil {
					continue
				}
				_, _ = m.manager.Install(path)
			}
			upgraded++
		}
	}

	if upgraded == 0 {
		m.logs = append(m.logs, "All packages are up to date!")
	} else {
		m.logs = append(m.logs, fmt.Sprintf("Upgraded %d package(s)", upgraded))
	}

	return upgradeDoneMsg{err: nil}
}

func (m *Model) doRemove() tea.Msg {
	prefix := fmt.Sprintf("php%s-", m.selectedVersion)
	installed := m.manager.GetAllInstalled()

	// Find all packages for this version
	var toRemove []string
	for _, p := range installed {
		if strings.HasPrefix(p.Name, prefix) {
			toRemove = append(toRemove, p.Name)
		}
	}

	if len(toRemove) == 0 {
		return removeDoneMsg{err: fmt.Errorf("no packages found for PHP %s", m.selectedVersion)}
	}

	// Remove in reverse order (dependents first)
	for i := len(toRemove) - 1; i >= 0; i-- {
		pkgName := toRemove[i]
		m.logs = append(m.logs, fmt.Sprintf("Removing %s...", pkgName))
		if err := m.manager.Remove(pkgName); err != nil {
			m.logs = append(m.logs, fmt.Sprintf("Warning: %v", err))
		}
	}

	// Remove symlinks
	_ = m.linker.RemoveVersionLinks(m.selectedVersion)

	// If this was default, set another version
	if m.linker.GetDefaultVersion() == m.selectedVersion {
		available := m.linker.GetAvailableVersions()
		if len(available) > 0 {
			_ = m.linker.SetDefaultVersion(available[0])
			m.logs = append(m.logs, fmt.Sprintf("Default changed to PHP %s", available[0]))
		}
	}

	m.logs = append(m.logs, "Done!")
	return removeDoneMsg{err: nil}
}

// View implements tea.Model
func (m *Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Error
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Message
	if m.message != "" {
		b.WriteString(warningStyle.Render(m.message))
		b.WriteString("\n\n")
	}

	// Content
	switch m.step {
	case StepMainMenu:
		b.WriteString(m.viewMainMenu())
	case StepSelectVersion:
		b.WriteString(m.viewSelectVersion())
	case StepSelectSAPIs:
		b.WriteString(m.viewSelectSAPIs())
	case StepSelectExtensions, StepModifyExtensions:
		b.WriteString(m.viewSelectExtensions())
	case StepConfirmInstall:
		b.WriteString(m.viewConfirmInstall())
	case StepInstalling, StepUpgrading, StepRemoving:
		b.WriteString(m.viewProgress())
	case StepSelectInstalledVersion:
		b.WriteString(m.viewSelectInstalledVersion())
	case StepModifyMenu:
		b.WriteString(m.viewModifyMenu())
	case StepConfirmRemove:
		b.WriteString(m.viewConfirmRemove())
	case StepFPMMenu:
		b.WriteString(m.viewFPMMenu())
	case StepFPMAction:
		b.WriteString(m.viewFPMAction())
	case StepDone:
		b.WriteString(m.viewDone())
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.viewHelp())

	return b.String()
}

func (m *Model) renderHeader() string {
	title := "PHM - PHP Manager for macOS"
	return headerStyle.Render(title)
}

func (m *Model) viewMainMenu() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("What would you like to do?"))
	b.WriteString("\n\n")

	icons := []string{"  ", "  ", "  ", "  ", "  ", "  "}

	for i, opt := range m.mainMenuOptions {
		line := icons[i] + opt
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + line))
		} else {
			b.WriteString(normalStyle.Render("   " + line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewSelectVersion() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Step 1/4: Select PHP version"))
	b.WriteString("\n\n")

	if len(m.availableVersions) == 0 {
		b.WriteString("No PHP versions available.\n")
		b.WriteString("Run 'phm update' to fetch package index.\n")
		return b.String()
	}

	installedVersions := m.linker.GetAvailableVersions()
	installedMap := make(map[string]bool)
	for _, v := range installedVersions {
		installedMap[v] = true
	}

	for i, ver := range m.availableVersions {
		label := fmt.Sprintf("PHP %s", ver)

		if installedMap[ver] {
			label += " " + installedStyle.Render("[installed]")
		}

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + label))
		} else {
			b.WriteString(normalStyle.Render("   " + label))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewSelectSAPIs() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Step 2/4: Select SAPIs for PHP %s", m.selectedVersion)))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Use SPACE to toggle selection, ENTER to continue"))
	b.WriteString("\n\n")

	for i, opt := range m.sapiOptions {
		checkbox := "[ ]"
		if m.selectedSAPIs[i] {
			checkbox = "[x]"
		}

		line := fmt.Sprintf("%s %s", checkbox, opt)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + line))
		} else {
			b.WriteString(normalStyle.Render("   " + line))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewSelectExtensions() string {
	var b strings.Builder

	stepNum := "3/4"
	if m.step == StepModifyExtensions {
		stepNum = "2/2"
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf("Step %s: Select extensions for PHP %s", stepNum, m.selectedVersion)))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("SPACE toggle | A select all | N deselect all | ENTER continue"))
	b.WriteString("\n\n")

	if len(m.extOptions) == 0 {
		b.WriteString("No extensions available\n")
		return b.String()
	}

	maxVisible := 12
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}

	for i := start; i < len(m.extOptions) && i < start+maxVisible; i++ {
		checkbox := "[ ]"
		if m.selectedExts[i] {
			checkbox = "[x]"
		}

		line := fmt.Sprintf("%s %-15s %s", checkbox, m.extOptions[i].name, mutedStyle.Render(m.extOptions[i].desc))
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + line))
		} else {
			b.WriteString(normalStyle.Render("   " + line))
		}
		b.WriteString("\n")
	}

	if len(m.extOptions) > maxVisible {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("\n   Showing %d-%d of %d extensions", start+1, min(start+maxVisible, len(m.extOptions)), len(m.extOptions))))
	}

	// Count selected
	selected := 0
	for _, v := range m.selectedExts {
		if v {
			selected++
		}
	}
	b.WriteString(mutedStyle.Render(fmt.Sprintf("\n   Selected: %d extension(s)", selected)))

	return b.String()
}

func (m *Model) viewConfirmInstall() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Step 4/4: Confirm installation"))
	b.WriteString("\n\n")

	// Summary box
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("PHP Version:  %s\n\n", m.selectedVersion))

	summary.WriteString("SAPIs:        ")
	var sapis []string
	sapiNames := []string{"cli", "fpm", "cgi"}
	for i, selected := range m.selectedSAPIs {
		if selected && i < len(sapiNames) {
			sapis = append(sapis, sapiNames[i])
		}
	}
	summary.WriteString(strings.Join(sapis, ", "))
	summary.WriteString("\n\n")

	summary.WriteString("Extensions:   ")
	var exts []string
	for i, selected := range m.selectedExts {
		if selected && i < len(m.extOptions) {
			exts = append(exts, m.extOptions[i].name)
		}
	}
	if len(exts) == 0 {
		summary.WriteString("(none)")
	} else {
		summary.WriteString(strings.Join(exts, ", "))
	}

	b.WriteString(boxStyle.Render(summary.String()))
	b.WriteString("\n\n")

	b.WriteString("Proceed with installation?\n\n")

	options := []string{"Yes, install now", "No, go back"}
	for i, opt := range options {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + opt))
		} else {
			b.WriteString(normalStyle.Render("   " + opt))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewProgress() string {
	var b strings.Builder

	action := "Installing"
	if m.step == StepUpgrading {
		action = "Upgrading packages"
	} else if m.step == StepRemoving {
		action = "Removing"
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf("%s %s...", m.spinner.View(), action)))
	b.WriteString("\n\n")

	maxLogs := 10
	start := 0
	if len(m.logs) > maxLogs {
		start = len(m.logs) - maxLogs
	}
	for i := start; i < len(m.logs); i++ {
		b.WriteString(mutedStyle.Render("   " + m.logs[i]))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewSelectInstalledVersion() string {
	var b strings.Builder

	action := "modify"
	if m.menuAction == MenuRemovePHP {
		action = "remove"
	}

	b.WriteString(titleStyle.Render(fmt.Sprintf("Select PHP version to %s", action)))
	b.WriteString("\n\n")

	defaultVer := m.linker.GetDefaultVersion()

	for i, ver := range m.installedVersions {
		label := fmt.Sprintf("PHP %s", ver)
		if ver == defaultVer {
			label += " " + installedStyle.Render("[default]")
		}

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + label))
		} else {
			b.WriteString(normalStyle.Render("   " + label))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewModifyMenu() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Modify PHP %s", m.selectedVersion)))
	b.WriteString("\n\n")

	options := []string{
		"Add/remove extensions",
		"Set as default version",
	}

	for i, opt := range options {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + opt))
		} else {
			b.WriteString(normalStyle.Render("   " + opt))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewConfirmRemove() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Remove PHP %s?", m.selectedVersion)))
	b.WriteString("\n\n")

	b.WriteString(warningStyle.Render("This will remove all packages for this PHP version!"))
	b.WriteString("\n\n")

	options := []string{"Yes, remove", "No, cancel"}
	for i, opt := range options {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + opt))
		} else {
			b.WriteString(normalStyle.Render("   " + opt))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewFPMMenu() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("PHP-FPM Management"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Select a PHP version to manage"))
	b.WriteString("\n\n")

	if len(m.fpmStatuses) == 0 {
		b.WriteString("No PHP-FPM versions installed.\n")
		return b.String()
	}

	// Table header
	b.WriteString(mutedStyle.Render("   Version     Enabled    Active"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("   " + strings.Repeat("─", 32)))
	b.WriteString("\n")

	for i, status := range m.fpmStatuses {
		// Build line with fixed-width columns (no ANSI in format string)
		version := fmt.Sprintf("PHP %-4s", status.Version)

		var enabledStr, activeStr string
		if status.Enabled {
			enabledStr = installedStyle.Render("Yes")
		} else {
			enabledStr = notInstalledStyle.Render("No ")
		}

		if status.Running {
			activeStr = installedStyle.Render("Yes")
		} else {
			activeStr = notInstalledStyle.Render("No ")
		}

		// Use tabs for alignment since colored strings have different visible vs byte lengths
		line := fmt.Sprintf("%-12s %s        %s", version, enabledStr, activeStr)

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > ") + line)
		} else {
			b.WriteString("   " + line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewFPMAction() string {
	var b strings.Builder

	if m.selectedFPMIdx >= len(m.fpmStatuses) {
		return "Invalid selection"
	}

	status := m.fpmStatuses[m.selectedFPMIdx]

	b.WriteString(titleStyle.Render(fmt.Sprintf("PHP-FPM %s Actions", status.Version)))
	b.WriteString("\n\n")

	// Status box
	var statusBox strings.Builder
	statusBox.WriteString(fmt.Sprintf("Version:  PHP %s\n", status.Version))

	if status.Running {
		statusBox.WriteString(fmt.Sprintf("Status:   %s (PID: %d)\n", installedStyle.Render("Running"), status.PID))
	} else {
		statusBox.WriteString(fmt.Sprintf("Status:   %s\n", notInstalledStyle.Render("Stopped")))
	}

	if status.Enabled {
		statusBox.WriteString(fmt.Sprintf("Autostart: %s\n", installedStyle.Render("Enabled")))
	} else {
		statusBox.WriteString(fmt.Sprintf("Autostart: %s\n", notInstalledStyle.Render("Disabled")))
	}

	statusBox.WriteString(fmt.Sprintf("Socket:   %s", status.Socket))

	b.WriteString(boxStyle.Render(statusBox.String()))
	b.WriteString("\n\n")

	b.WriteString("Select action:\n\n")

	for i, opt := range m.fpmActionOptions {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(" > " + opt))
		} else {
			b.WriteString(normalStyle.Render("   " + opt))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) viewDone() string {
	var b strings.Builder

	style := installedStyle
	icon := "OK"
	if strings.Contains(m.message, "failed") {
		style = errorStyle
		icon = "ERROR"
	}

	b.WriteString(style.Render(fmt.Sprintf("[%s] %s", icon, m.message)))
	b.WriteString("\n\n")

	if len(m.logs) > 0 {
		b.WriteString(subtitleStyle.Render("Summary:"))
		b.WriteString("\n")
		maxLogs := 8
		start := 0
		if len(m.logs) > maxLogs {
			start = len(m.logs) - maxLogs
		}
		for i := start; i < len(m.logs); i++ {
			b.WriteString(mutedStyle.Render("   " + m.logs[i]))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(normalStyle.Render("Press ENTER to continue"))

	return b.String()
}

func (m *Model) viewHelp() string {
	var help string
	switch m.step {
	case StepMainMenu:
		help = "Up/Down: navigate | Enter: select | Q: quit"
	case StepSelectVersion, StepSelectInstalledVersion:
		help = "Up/Down: navigate | Enter: select | Esc: back | Q: quit"
	case StepSelectSAPIs:
		help = "Up/Down: navigate | Space: toggle | Enter: continue | Esc: back"
	case StepSelectExtensions, StepModifyExtensions:
		help = "Up/Down: navigate | Space: toggle | A: all | N: none | Enter: continue | Esc: back"
	case StepConfirmInstall, StepConfirmRemove:
		help = "Up/Down: navigate | Enter: confirm | Esc: back"
	case StepInstalling, StepUpgrading, StepRemoving:
		help = "Please wait..."
	case StepFPMMenu:
		help = "Up/Down: navigate | Enter: select | Esc: back | Q: quit"
	case StepFPMAction:
		help = "Up/Down: navigate | Enter: execute | Esc: back"
	case StepDone:
		help = "Enter: continue | Q: quit"
	default:
		help = "Esc: back | Q: quit"
	}
	return helpStyle.Render(help)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
