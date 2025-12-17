package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Yellow
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray

	// Base styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginBottom(1)

	// List styles
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Status styles
	installedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	upgradableStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	notInstalledStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	// Help style
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// Header style
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(mutedColor).
			MarginBottom(1).
			Width(60)

	// Muted text style
	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Error style (for messages)
	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Warning style
	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)
)

// StatusStyle returns the appropriate style for a package state
func StatusStyle(installed bool, upgradable bool) lipgloss.Style {
	if upgradable {
		return upgradableStyle
	}
	if installed {
		return installedStyle
	}
	return notInstalledStyle
}

// StatusText returns the status text for display
func StatusText(installed bool, upgradable bool) string {
	if upgradable {
		return "[upgrade]"
	}
	if installed {
		return "[installed]"
	}
	return ""
}
