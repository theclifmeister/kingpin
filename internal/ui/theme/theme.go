// Package theme is the single place colours and styles are defined. One
// accent per simulation, used consistently so the player learns to read them.
package theme

import "github.com/charmbracelet/lipgloss"

var (
	Market    = lipgloss.Color("#5fd77a") // green
	Heat      = lipgloss.Color("#ff5f5f") // red
	Rivals    = lipgloss.Color("#b58cff") // purple
	Crew      = lipgloss.Color("#5fafff") // blue
	Money     = lipgloss.Color("#ffd75f") // gold
	Logistics = lipgloss.Color("#5fd7d7") // teal
	News      = lipgloss.Color("#c0c0c0") // grey
	Dim       = lipgloss.Color("#6c6c6c")
	Text      = lipgloss.Color("#e4e4e4")
	Bg        = lipgloss.Color("#1c1c1c")
	Warn      = lipgloss.Color("#ffaf5f") // orange

	Title    = lipgloss.NewStyle().Bold(true).Foreground(Money)
	Subtle   = lipgloss.NewStyle().Foreground(Dim)
	Body     = lipgloss.NewStyle().Foreground(Text)
	Bold     = lipgloss.NewStyle().Bold(true).Foreground(Text)
	Good     = lipgloss.NewStyle().Foreground(Market)
	Bad      = lipgloss.NewStyle().Foreground(Heat)
	Warning  = lipgloss.NewStyle().Foreground(Warn)
	Gold     = lipgloss.NewStyle().Foreground(Money)
	Rival    = lipgloss.NewStyle().Foreground(Rivals)
	Selected = lipgloss.NewStyle().Bold(true).Foreground(Bg).Background(Money)
	Key      = lipgloss.NewStyle().Foreground(Money)
	Tab      = lipgloss.NewStyle().Foreground(Dim).Padding(0, 1)
	TabOn    = lipgloss.NewStyle().Bold(true).Foreground(Bg).Background(Money).Padding(0, 1)

	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Dim).
		Padding(0, 1)
	PanelTitle = lipgloss.NewStyle().Bold(true)
	Modal      = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(Money).
			Padding(1, 2)
)

// Source returns the accent colour for a simulation name.
func Source(name string) lipgloss.Color {
	switch name {
	case "market":
		return Market
	case "heat":
		return Heat
	case "rivals", "territory":
		return Rivals
	case "crew":
		return Crew
	case "money", "laundering":
		return Money
	case "logistics":
		return Logistics
	case "reputation", "dilemma":
		return Warn
	default:
		return News
	}
}
