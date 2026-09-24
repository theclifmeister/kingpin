package ui

// The screens are one table (#274), the way the modes are (modes.go):
// screens holds, for each tab, its names, its MAIN, its pane, its accent
// and what the arrows do on it, which a switch per question used to
// spread over the package. A screen is added in the constants and in the
// table's row, and TestScreenTableIsComplete fails on a row left empty.

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

type screen int

const (
	screenDashboard screen = iota
	screenMarket
	screenJournal
	screenCrew
	screenMap
	screenUpgrades
	screenLedger
	screenRivals
	screenIntel // what you know against what is true (#45)
	screenCount
)

// screenSpec is one screen's row.
type screenSpec struct {
	name  string // the title bar's tab
	short string // the tab when the title bar is tight (Mkt since the ninth tab, #45: nine short names and the news count fit 120 columns)
	word  string // the screen in prose: `on the crew screen (4)`, the help's and the README's group

	view    func(*Model) string    // MAIN
	details func(*Model) []section // the pane's sections, the selection first; the keys are the key table's (keysFor)
	accent  lipgloss.Color         // the colour the pane and titles are drawn in: one per sim

	// move moves the screen's cursor (moveCursor): down a list, along
	// the map's grid, down the tree's branch and between its branches,
	// between the cities on the market; the journal scrolls. A screen
	// with no horizontal structure ignores dx.
	move func(m *Model, dx, dy int)
}

// screens is the table, one row a tab in the title bar's order. It is
// filled in init: a screen's view reads it back (its accent, the tabs),
// which a package-level initializer would make a cycle.
var screens [screenCount]screenSpec

func init() {
	screens = [screenCount]screenSpec{
		screenDashboard: {name: "Dashboard", short: "Dash", word: "dashboard", view: (*Model).viewDashboard, details: (*Model).dashboardDetails, accent: theme.Money,
			move: func(m *Model, _, dy int) { m.dashboardMove(dy) }},
		screenMarket: {name: "Market", short: "Mkt", word: "market", view: (*Model).viewMarket, details: (*Model).marketDetails, accent: theme.Market,
			move: (*Model).marketMove},
		screenJournal: {name: "Journal", short: "Journal", word: "journal", view: (*Model).viewJournal, details: (*Model).journalDetails, accent: theme.News,
			move: func(m *Model, _, dy int) { m.journalMove(dy) }},
		screenCrew: {name: "Crew", short: "Crew", word: "crew", view: (*Model).viewCrew, details: (*Model).crewDetails, accent: theme.Crew,
			move: func(m *Model, _, dy int) { m.crewMove(dy) }},
		screenMap: {name: "Map", short: "Map", word: "map", view: (*Model).viewMap, details: (*Model).mapDetails, accent: theme.Rivals,
			move: (*Model).mapMove},
		screenUpgrades: {name: "Upgrades", short: "Upgr", word: "upgrades", view: (*Model).viewUpgrades, details: (*Model).upgradesDetails, accent: theme.Money,
			move: (*Model).upgradeMove},
		screenLedger: {name: "Ledger", short: "Ledger", word: "ledger", view: (*Model).viewLedger, details: (*Model).ledgerDetails, accent: theme.Money,
			move: func(m *Model, _, dy int) { m.ledgerMove(dy) }},
		screenRivals: {name: "Rivals", short: "Rivals", word: "rivals", view: (*Model).viewRivals, details: (*Model).rivalsDetails, accent: theme.Rivals,
			move: (*Model).rivalsMove},
		screenIntel: {name: "Intel", short: "Intel", word: "intel", view: (*Model).viewIntel, details: (*Model).intelDetails, accent: theme.Intel,
			move: func(m *Model, _, dy int) { m.intelMove(dy) }},
	}
}

// viewScreen is the MAIN of the screen shown.
func (m *Model) viewScreen() string { return screens[m.screen].view(m) }

// details is the pane's content for the screen shown.
func (m *Model) details() []section { return screens[m.screen].details(m) }

// accent is the colour the screen shown draws its pane and titles in.
func (m *Model) accent() lipgloss.Color { return screens[m.screen].accent }

// moveCursor moves the screen's cursor. Arrows never move between tabs
// (the digits and tab do that).
func (m *Model) moveCursor(dx, dy int) { screens[m.screen].move(m, dx, dy) }

// productMove is the product cursor the dashboard shares with the
// market and the dialogs.
func (m *Model) productMove(dy int) {
	if dy < 0 && m.cursor > 0 {
		m.cursor--
	} else if dy > 0 && m.cursor < len(m.w.Products)-1 {
		m.cursor++
	}
}
