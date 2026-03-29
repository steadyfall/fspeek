package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Pane borders and backgrounds.
	listPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	metaPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	// Status bar at the bottom.
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	statusErrStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("196")).
			Padding(0, 1)

	// Highlighted row in the directory list.
	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Bold(true)

	// Normal row.
	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Directory entries.
	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")).
			Bold(true)

	// Metadata sidebar sections.
	metaLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Width(12)

	metaValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	metaTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			MarginBottom(1)

	// Error display in sidebar.
	metaErrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Italic(true)

	// Spinner / loading indicator.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	// Help bar at the very bottom.
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)
)
