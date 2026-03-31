package ui

// Theme holds all color constants for the TUI.
// Colors are 256-color ANSI codes (as strings for lipgloss.Color).
type Theme struct {
	// Pane structure
	BorderColor string

	// Left pane — navigation
	NormalFg string
	DirFg    string
	StatFg   string
	CursorFg string
	CursorBg string

	// Right pane — metadata
	MetaTitleFg    string
	MetaLabelFg    string
	MetaValueFg    string
	MetaErrFg      string
	MetaLabelWidth int

	// Status bar
	StatusFg    string
	StatusBg    string
	StatusErrFg string

	// Help bar
	HelpFg string

	// Spinner / loading
	SpinnerFg string

	// Partial-listing indicator
	PartialFg string
}

// SpectralTheme is the default theme: cold navigation, warm discovery.
// Navigation elements use the cool spectrum (electric blue, cyan); metadata
// elements use the warm spectrum (coral, salmon). The two panes communicate
// the tool's purpose — file browsing vs. media inspection — through color.
var SpectralTheme = Theme{
	BorderColor:    "241",
	NormalFg:       "252",
	DirFg:          "39",
	StatFg:         "240",
	CursorFg:       "232",
	CursorBg:       "45",
	MetaTitleFg:    "209",
	MetaLabelFg:    "242",
	MetaValueFg:    "252",
	MetaErrFg:      "203",
	MetaLabelWidth: 12,
	StatusFg:       "209",
	StatusBg:       "235",
	StatusErrFg:    "203",
	HelpFg:         "238",
	SpinnerFg:      "45",
	PartialFg:      "214", // amber — cache confidence signal
}
