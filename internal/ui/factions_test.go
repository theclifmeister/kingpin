package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// tableModel is a run with four factions dealt (#43): the rival at home
// on a corner, two more seated on corners of their own with a deal and
// an offer between them, and the fourth still to arrive.
func tableModel(t *testing.T, w, h int) *Model {
	t.Helper()
	t.Setenv("KINGPIN_HOME", t.TempDir())
	cfg := content.MustLoad()
	cfg.Rivals.Factions.Min, cfg.Rivals.Factions.Max = 4, 4
	m, err := New(cfg, Options{Anim: false})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.startRun(11)
	world := m.w
	if len(world.Rivals) != 4 {
		t.Fatalf("%d factions", len(world.Rivals))
	}
	home := world.Home()
	for i, corner := range []int{1, 2, 4} { // rail yard, old mill, bus depot; you stand on fourth
		r := world.Rivals[i]
		r.Arrived, r.Observed, r.Muscle, r.Cash = 1, true, 3 + i, 20_000
		home.Corners[corner].Owner, home.Corners[corner].Faction = game.OwnerRival, r.Faction()
	}
	world.Rivals[2].Deals = []game.Deal{{Kind: game.DealTruce, Terms: game.Terms{Days: 30}, Since: world.Day, Until: world.Day + 30, Faction: world.Rivals[2].Faction()}}
	world.Offers = []game.Offer{{ID: 1, Deal: game.Deal{Kind: game.DealTribute, Terms: game.Terms{PerDay: 500}, Offered: true, Faction: world.Rivals[1].Faction()}, Expires: world.Day + 4, Faction: world.Rivals[1].Faction()}}
	return m
}

// The rivals screen turns to a faction (#43): [ and ] move the cursor
// round the table and the FACTIONS table, the leader line, DEALS and
// the pane follow it; the screen fits every size with the table on;
// the offer names who made it.
func TestRivalsScreenTurnsToAFaction(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}} {
		m := tableModel(t, sz[0], sz[1])
		m.Update(key("8"))
		view := stripANSI(m.View())
		assertFits(t, m.View(), sz[0], sz[1], "rivals with the table")
		for _, r := range m.w.Rivals {
			if !strings.Contains(view, truncate(r.Leader, 14)) {
				t.Fatalf("%dx%d: the FACTIONS table does not list %s:\n%s", sz[0], sz[1], r.Leader, view)
			}
		}
		if !strings.Contains(view, "not yet") || !strings.Contains(view, "truce") {
			t.Fatalf("%dx%d: stances missing:\n%s", sz[0], sz[1], view)
		}
		if m.faction() != m.w.Rival() || !strings.Contains(view, m.w.Rival().Leader+"'s crew") {
			t.Fatalf("%dx%d: the screen does not open on the rival at home", sz[0], sz[1])
		}
		m.Update(key("]"))
		m.Update(key("]"))
		f3 := m.w.Rivals[2]
		if m.faction() != f3 {
			t.Fatalf("]] did not turn to the third faction: %d", m.factionCursor)
		}
		view = stripANSI(m.View())
		assertFits(t, m.View(), sz[0], sz[1], "rivals turned")
		if !strings.Contains(view, f3.Leader+"'s crew") || !strings.Contains(view, "30") {
			t.Fatalf("%dx%d: the third faction's line and truce are not shown:\n%s", sz[0], sz[1], view)
		}
		if !strings.Contains(view, truncate(m.w.Rivals[1].Leader, 12)) {
			t.Fatalf("%dx%d: the offer does not name who made it:\n%s", sz[0], sz[1], view)
		}
		m.Update(key("["))
		m.Update(key("["))
		m.Update(key("["))
		if m.faction() != m.w.Rivals[3] {
			t.Fatalf("[ did not wrap round the table: %d", m.factionCursor)
		}
		view = stripANSI(m.View())
		if !strings.Contains(view, "have not moved in yet") {
			t.Fatalf("%dx%d: the fourth faction is not said to be in the wings:\n%s", sz[0], sz[1], view)
		}
		// Proposing goes to the faction shown.
		m.Update(key("]"))
		if m.faction() != m.w.Rival() {
			t.Fatalf("] did not wrap back: %d", m.factionCursor)
		}
		m.Update(key("]"))
		m.Update(key("d"))
		if m.mode != modePropose {
			t.Fatalf("d: mode %v status %q", m.mode, m.status)
		}
		if !strings.Contains(stripANSI(m.View()), m.w.Rivals[1].Leader) {
			t.Fatalf("the propose dialog does not name the faction shown:\n%s", stripANSI(m.View()))
		}
		m.Update(key("enter"))
		m.Update(key("enter"))
		if p := m.w.Today.Proposal; p == nil || m.w.Faction(p.Faction) != m.w.Rivals[1] {
			t.Fatalf("the proposal went to %+v", p)
		}
	}
}

// The map colours a corner by the faction holding it (#43): the rival
// at home's purple, then a colour of its own for each seat, none of
// them another meaning's.
func TestMapColoursCornersByFaction(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(0) // termenv.TrueColor
	defer lipgloss.SetColorProfile(profile)
	m := tableModel(t, 120, 40)
	m.Update(key("5"))
	view := m.View()
	home := m.w.Home()
	for i, corner := range []int{1, 2, 4} {
		name := strings.ToUpper(home.Corners[corner].Name)
		colour := strings.SplitN(theme.FactionText(i).Render("x"), "x", 2)[0] // the escape that opens the colour
		if !strings.Contains(view, colour+"▴ "+name) {
			t.Fatalf("%s is not drawn in faction %d's colour", name, i)
		}
	}
	if theme.Faction(0) != theme.Rivals || theme.Faction(1) == theme.Rivals || theme.Faction(9) != theme.Faction(len(theme.Factions)-1) {
		t.Fatal("the faction colours")
	}
	seen := map[lipgloss.Color]bool{theme.Market: true, theme.Heat: true, theme.Crew: true, theme.Money: true, theme.Logistics: true, theme.Warn: true, theme.World: true}
	for _, c := range theme.Factions {
		if seen[c] {
			t.Fatalf("faction colour %v means something else", c)
		}
		seen[c] = true
	}
	// The inspector names the holder.
	for i := range home.Corners {
		if home.Corners[i].ID == home.Corners[2].ID {
			m.mapCursor = i
		}
	}
	if text := paneText(m); !strings.Contains(text, m.w.Rivals[1].Leader+"'s since") {
		t.Fatalf("the inspector does not name the holder:\n%s", text)
	}
}

// The dashboard's RIVALS panel lists the table (#43): the rival at home
// as before, and the other factions with their corners on the line the
// trust bar had; the narrow layout's line counts them.
func TestDashboardListsTheFactions(t *testing.T) {
	m := tableModel(t, 120, 40)
	view := stripANSI(m.View())
	assertFits(t, m.View(), 120, 40, "dashboard with the table")
	for _, r := range m.w.Rivals[1:] {
		if !strings.Contains(view, r.Leader) {
			t.Fatalf("the RIVALS panel does not list %s:\n%s", r.Leader, view)
		}
	}
	m = tableModel(t, 80, 24)
	view = stripANSI(m.View())
	assertFits(t, m.View(), 80, 24, "dashboard with the table")
	if !strings.Contains(view, "+3 more") {
		t.Fatalf("the narrow layout does not count the table:\n%s", view)
	}
}
