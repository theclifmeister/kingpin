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

	// One meaning each (#88): Market green is the market and good, Heat
	// red is heat, the law and danger, Rival purple is theirs only, Crew
	// blue is yours, Money gold is money, the cursor, keys and titles,
	// Logistics teal is the road, Warn orange is a warning line and the
	// reputation and dilemma accent.
	Title      = lipgloss.NewStyle().Bold(true).Foreground(Money)
	Subtle     = lipgloss.NewStyle().Foreground(Dim)
	Body       = lipgloss.NewStyle().Foreground(Text)
	Bold       = lipgloss.NewStyle().Bold(true).Foreground(Text)
	Good       = lipgloss.NewStyle().Foreground(Market)
	Bad        = lipgloss.NewStyle().Foreground(Heat)
	Warning    = lipgloss.NewStyle().Foreground(Warn)
	Gold       = lipgloss.NewStyle().Foreground(Money)
	MarketText = lipgloss.NewStyle().Foreground(Market)    // the market's lines
	LawText    = lipgloss.NewStyle().Foreground(Heat)      // the law: the chief, the DA, pressure
	RivalText  = lipgloss.NewStyle().Foreground(Rivals)    // theirs: the rival's corners and lines
	CrewText   = lipgloss.NewStyle().Foreground(Crew)      // yours: the crew, your corners, a lieutenant's order
	RoadText   = lipgloss.NewStyle().Foreground(Logistics) // the road
	NewsText   = lipgloss.NewStyle().Foreground(News)      // the paper
	DialOn     = lipgloss.NewStyle().Foreground(Money)     // the chosen notch of a dial: `[normal]`
	Selected   = lipgloss.NewStyle().Bold(true).Foreground(Bg).Background(Money)
	Key        = lipgloss.NewStyle().Foreground(Money)
	Tab        = lipgloss.NewStyle().Foreground(Dim).Padding(0, 1)
	TabOn      = lipgloss.NewStyle().Bold(true).Foreground(Bg).Background(Money).Padding(0, 1)
	Plain      = lipgloss.NewStyle() // no colour: the layout's own sizing and wrapping

	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Dim).
		Padding(0, 1)
	PanelTitle = lipgloss.NewStyle().Bold(true)
	Modal      = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(Money).
			Padding(0, 1)
)

// Dial is the style of one notch of a dial: the chosen one in the accent
// (DialOn, drawn `[normal]`), the rest Subtle.
func Dial(on bool) lipgloss.Style {
	if on {
		return DialOn
	}
	return Subtle
}

// Fg is text in a colour: a headline in its source's accent, a bar in
// its axis's. View files take their colour from here and build none.
func Fg(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// Heading is a bold title in an accent: a panel's name, a pane section.
func Heading(c lipgloss.Color) lipgloss.Style { return lipgloss.NewStyle().Bold(true).Foreground(c) }

// SourceText is the style of a headline from a simulation.
func SourceText(name string) lipgloss.Style { return Fg(Source(name)) }

// Source returns the accent colour for a simulation name.
func Source(name string) lipgloss.Color {
	switch name {
	case "market", "buyers":
		return Market
	case "heat", "law":
		return Heat
	case "rivals", "territory":
		return Rivals
	case "crew":
		return Crew
	case "money", "laundering", "unlock":
		return Money
	case "logistics":
		return Logistics
	case "reputation", "dilemma":
		return Warn
	default:
		return News
	}
}
