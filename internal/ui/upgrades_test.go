package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/game"
)

// The upgrades tree is walked as three columns: right from a node in
// Operations selects the Security node at the same row, or the last one
// if that column is shorter; left at the first column is a no-op; up and
// down stay within a column; enter still buys the node under the cursor.
func TestUpgradeArrowsMoveColumns(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 8000
	m.Update(key("6"))
	ops := m.cfg.Upgrades.Branch("operations")
	sec := m.cfg.Upgrades.Branch("security")
	legal := m.cfg.Upgrades.Branch("legal")
	if len(ops) <= len(sec) || len(sec) <= len(legal) {
		t.Skipf("branches are %d/%d/%d nodes; the test wants them shrinking", len(ops), len(sec), len(legal))
	}
	id := func() string {
		u, _ := m.upgradeSelected()
		return u.ID
	}
	m.Update(key("left"))
	if id() != ops[0].ID {
		t.Fatalf("left at the first column moved to %s", id())
	}
	m.Update(key("down"))
	m.Update(key("right"))
	if id() != sec[1].ID {
		t.Fatalf("right from %s selected %s, want %s", ops[1].ID, id(), sec[1].ID)
	}
	m.Update(key("left"))
	if id() != ops[1].ID {
		t.Fatalf("left back selected %s, want %s", id(), ops[1].ID)
	}
	// Down to the bottom of Operations, then right lands on the last of
	// each shorter column and up/down never leave it.
	for range ops {
		m.Update(key("down"))
	}
	if id() != ops[len(ops)-1].ID {
		t.Fatalf("down past the end of Operations selected %s", id())
	}
	m.Update(key("right"))
	if id() != sec[len(sec)-1].ID {
		t.Fatalf("right from the bottom of Operations selected %s, want %s", id(), sec[len(sec)-1].ID)
	}
	m.Update(key("down"))
	if id() != sec[len(sec)-1].ID {
		t.Fatalf("down at the bottom of Security selected %s", id())
	}
	m.Update(key("right"))
	m.Update(key("right"))
	if id() != legal[len(legal)-1].ID {
		t.Fatalf("right at the last column selected %s, want %s", id(), legal[len(legal)-1].ID)
	}
	for range legal {
		m.Update(key("up"))
	}
	if id() != legal[0].ID {
		t.Fatalf("up past the top of Legal selected %s", id())
	}
	if m.screen != screenUpgrades || m.mode != modePlay {
		t.Fatalf("arrows left the tree: screen %v mode %v", m.screen, m.mode)
	}
	// Enter buys the node under the cursor: back to the top of Security.
	m.Update(key("left"))
	m.Update(key("left"))
	m.Update(key("right"))
	if id() != sec[0].ID {
		t.Fatalf("cursor is on %s, want %s", id(), sec[0].ID)
	}
	m.Update(key("enter"))
	if m.mode != modeConfirmUpgrade || m.upgradeID != sec[0].ID {
		t.Fatalf("enter: mode %v id %q status %q", m.mode, m.upgradeID, m.status)
	}
	m.Update(key("y"))
	if !m.w.Owns(sec[0].ID) || m.w.Day != 0 {
		t.Fatalf("y did not buy %s: owns %v day %d status %q", sec[0].ID, m.w.Owns(sec[0].ID), m.w.Day, m.status)
	}
}

// The upgrades screen: enter (or u) asks before buying the selected node,
// y buys it and the effect lands at once, a locked or unaffordable node
// only explains itself, the dashboard lists what is owned, the report
// lists the purchase, and none of it happens from other screens.
func TestUpgradesScreenKeys(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 8000
	m.Update(key("u"))
	if m.mode != modePlay || m.status != "Buy upgrade on the upgrades screen (6)." {
		t.Fatalf("u on the dashboard: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("6"))
	if m.screen != screenUpgrades {
		t.Fatalf("screen = %v", m.screen)
	}
	rows := m.upgradeRows()
	if rows[0].ID != "stash" {
		t.Fatalf("first node is %s", rows[0].ID)
	}
	// Enter asks; anything but y backs out.
	m.Update(key("enter"))
	if m.mode != modeConfirmUpgrade || m.upgradeID != "stash" {
		t.Fatalf("enter did not ask: mode %v id %q status %q", m.mode, m.upgradeID, m.status)
	}
	m.Update(key("esc"))
	if m.mode != modePlay || m.w.Owns("stash") || m.w.Day != 0 {
		t.Fatalf("esc bought something or ended the day: owns %v day %d", m.w.Owns("stash"), m.w.Day)
	}
	carry := m.w.Capacity(m.w.Player.Location)
	m.Update(key("u"))
	m.Update(key("y"))
	if !m.w.Owns("stash") || m.w.Player.DirtyCash != 3000 || m.w.Capacity(m.w.Player.Location) != carry+50 {
		t.Fatalf("y did not buy: owns %v cash %d capacity %d status %q", m.w.Owns("stash"), m.w.Player.DirtyCash, m.w.Capacity(m.w.Player.Location), m.status)
	}
	// Owned, locked and unaffordable nodes explain themselves without a modal.
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "already") {
		t.Fatalf("buying twice: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("j")) // stash2: needs nothing more, but $15K
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "costs") {
		t.Fatalf("unaffordable: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("j"))
	m.Update(key("j")) // supplier2: locked behind supplier
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "Supplier contact") {
		t.Fatalf("locked: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "upgrades stash") {
		t.Fatal("dashboard does not list the stash")
	}
	m.Update(key("n"))
	if m.mode != modeReport || len(m.w.Report.Upgrades) != 1 || !strings.Contains(m.w.Report.Upgrades[0], "Stash spot") {
		t.Fatalf("report: mode %v upgrades %v", m.mode, m.w.Report.Upgrades)
	}
	assertFits(t, m.View(), 100, 30, "report with an upgrade")
	if !strings.Contains(strings.Join(m.w.Report.Money, "\n"), "Stash spot -$5,000") {
		t.Fatalf("money section: %v", m.w.Report.Money)
	}
	m.Update(key("enter"))
	if m.w.Report.CashBefore != 8000 {
		t.Fatalf("cash before = %d, want the morning's 8000", m.w.Report.CashBefore)
	}
	// It survives a save and load.
	m.Update(key("ctrl+s"))
	w, err := game.Load(1, m.set.Migrations()...)
	if err != nil || !w.Owns("stash") {
		t.Fatalf("load: %v owns %v", err, w != nil && w.Owns("stash"))
	}
}

// TestUpgradesScreenInTheGrammar: MAIN is the title with the count, the
// two pools, the three columns with cash() costs and the legend; no `*`
// anywhere, a clean-cash node says `clean` in a word. The pane's first
// line is the tree's cost string with the pool and where the node
// stands, the effects one per line, and u names the price; every pane
// line fits the pane and at most one blank row sits between sections
// at 80x24 (#86).
func TestUpgradesScreenInTheGrammar(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		m.Update(key("6"))
		m.w.Player.DirtyCash += 20_000
		m.Update(key("u"))
		m.Update(key("y")) // one owned
		view := stripANSI(m.View())
		main := strings.Split(stripANSI(m.viewScreen()), "\n")
		if !strings.HasPrefix(main[0], "UPGRADES · 1 of "+fmt.Sprint(len(m.cfg.Upgrades.Nodes))+" owned") {
			t.Errorf("%dx%d title: %q", sz[0], sz[1], main[0])
		}
		if !strings.HasPrefix(main[1], "dirty "+cash(m.w.Player.DirtyCash)+" · clean "+cash(m.w.Player.CleanCash)) {
			t.Errorf("%dx%d pools: %q", sz[0], sz[1], main[1])
		}
		if strings.Contains(view, "*") {
			t.Errorf("%dx%d: a * on the upgrades screen:\n%s", sz[0], sz[1], view)
		}
		legend := false
		for _, l := range main {
			legend = legend || strings.TrimRight(l, " ") == "✓ owned  ○ available  · locked"
		}
		if !legend {
			t.Errorf("%dx%d: no legend line:\n%s", sz[0], sz[1], view)
		}
		last, blanks := "", 0
		for i, l := range main {
			l = strings.TrimRight(l, " ")
			if l == "" && last == "" && i > 0 && strings.TrimSpace(strings.Join(main[i:], "")) != "" {
				blanks++
			}
			last = l
		}
		if blanks > 0 {
			t.Errorf("%dx%d: two blank rows between sections:\n%s", sz[0], sz[1], view)
		}
		// Every node: the tree and the pane agree on the cost, the pane's
		// first line carries the pool and the state, and the lines fit.
		for i, u := range m.upgradeRows() {
			m.upgradeCursor = i
			secs := m.details()
			if len(secs) != 1 || secs[0].title != strings.ToUpper(u.Name) {
				t.Fatalf("%s: sections %v", u.ID, secs)
			}
			first := stripANSI(secs[0].lines[0])
			if !strings.HasPrefix(first, cash(u.Cost)+" "+pool(u)) {
				t.Errorf("%s: the pane's first line is %q, want it to start with %q", u.ID, first, cash(u.Cost)+" "+pool(u))
			}
			state := m.upgradeState(u)
			switch {
			case state == "locked" && !strings.Contains(strings.Join(secs[0].lines, "\n"), "needs"):
				t.Errorf("%s: locked but the pane names nothing it needs: %q", u.ID, first)
			case state != "locked" && !strings.Contains(first, "· "+state) && !strings.Contains(first, "short"):
				t.Errorf("%s: the pane's first line %q does not say %s", u.ID, first, state)
			}
			for _, l := range secs[0].lines {
				if lipgloss.Width(l) > paneTextW {
					t.Errorf("%s: pane line wider than %d: %q", u.ID, paneTextW, stripANSI(l))
				}
			}
			for _, e := range effectWords(u.Effects) {
				if !strings.Contains(strings.Join(secs[0].lines, "\n"), "  "+e) {
					t.Errorf("%s: the pane lacks the effect %q", u.ID, e)
				}
			}
			v := stripANSI(m.View())
			if strings.Count(v, cash(u.Cost)) < 2 {
				t.Errorf("%s: the tree and the pane do not both print %s:\n%s", u.ID, cash(u.Cost), v)
			}
			if state == "available" && m.canAfford(u) && !strings.Contains(strings.Join(secs[0].lines, "\n"), "u  buy it for "+cash(u.Cost)+" "+pool(u)) {
				t.Errorf("%s: the pane does not offer u for %s", u.ID, cash(u.Cost))
			}
			if u.Clean && !strings.Contains(v, "clean, ") && !strings.Contains(v, "  clean\n") {
				t.Errorf("%s: the tree does not say clean under the node:\n%s", u.ID, v)
			}
		}
	}
}
