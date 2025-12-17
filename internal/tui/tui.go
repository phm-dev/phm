package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phm-dev/phm/internal/config"
)

// Run starts the TUI application
func Run(cfg *config.Config) error {
	// Ensure directories exist
	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	model := NewModel(cfg)

	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// lipgloss is used for styling - re-export for model.go
var _ = lipgloss.NewStyle
