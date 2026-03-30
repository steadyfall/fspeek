package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Pane borders and backgrounds.
	listPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(SpectralTheme.BorderColor))

	metaPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(SpectralTheme.BorderColor))

	// Status bar at the bottom.
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(SpectralTheme.StatusBg)).
			Foreground(lipgloss.Color(SpectralTheme.StatusFg)).
			Padding(0, 1)

	statusErrStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(SpectralTheme.StatusBg)).
			Foreground(lipgloss.Color(SpectralTheme.StatusErrFg)).
			Padding(0, 1)

	// Highlighted row in the directory list.
	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(SpectralTheme.CursorBg)).
			Foreground(lipgloss.Color(SpectralTheme.CursorFg)).
			Bold(true)

	// Normal row.
	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.NormalFg))

	// Directory entries.
	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.DirFg)).
			Bold(true)

	// Secondary stat text (file size, dir item count) — dimmer than the entry name.
	statStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.StatFg))

	// Metadata sidebar sections.
	metaLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.MetaLabelFg)).
			Width(SpectralTheme.MetaLabelWidth)

	metaValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.MetaValueFg))

	metaTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.MetaTitleFg)).
			Bold(true).
			MarginBottom(1)

	// Error display in sidebar.
	metaErrStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.MetaErrFg)).
			Italic(true)

	// Spinner / loading indicator.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.SpinnerFg))

	// Help bar at the very bottom.
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(SpectralTheme.HelpFg)).
			Padding(0, 1)
)
