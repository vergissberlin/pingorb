package tui

import "github.com/charmbracelet/lipgloss"

var (
	colOrange = lipgloss.Color("214")
	colTeal   = lipgloss.Color("86")
	colWhite  = lipgloss.Color("255")
	colGray   = lipgloss.Color("240")
	colGreen  = lipgloss.Color("46")
	colYellow = lipgloss.Color("220")
	colRed    = lipgloss.Color("203")
	colPink   = lipgloss.Color("212")
	colBlack  = lipgloss.Color("0")

	badgeStyle = lipgloss.NewStyle().
			Background(colOrange).
			Foreground(colBlack).
			Bold(true).
			Padding(0, 1)

	clockStyle = lipgloss.NewStyle().
			Background(colGray).
			Foreground(colWhite).
			Padding(0, 1)

	sectionStyle = lipgloss.NewStyle().Foreground(colOrange).Bold(true)
	labelStyle   = lipgloss.NewStyle().Foreground(colTeal)
	valueStyle   = lipgloss.NewStyle().Foreground(colWhite)
	dimStyle     = lipgloss.NewStyle().Foreground(colGray)
	pinkStyle    = lipgloss.NewStyle().Foreground(colPink)

	okStyle   = lipgloss.NewStyle().Foreground(colGreen).Bold(true)
	warnStyle = lipgloss.NewStyle().Foreground(colYellow).Bold(true)
	badStyle  = lipgloss.NewStyle().Foreground(colRed).Bold(true)

	selectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(colWhite).Bold(true)
	normalRowStyle   = lipgloss.NewStyle().Foreground(colWhite)

	panelBorder = lipgloss.NewStyle().
			Foreground(colGray)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colOrange).
			Padding(1, 2)

	helpStyle = lipgloss.NewStyle().Foreground(colGray)
	errStyle  = lipgloss.NewStyle().Foreground(colRed).Bold(true)
)

// statusStyle picks a color for a latency-based status: dot color reflects
// reachability and how "hot" the round-trip time is.
func statusStyle(alive bool, rttMS float64) lipgloss.Style {
	switch {
	case !alive:
		return badStyle
	case rttMS > 150:
		return warnStyle
	default:
		return okStyle
	}
}
