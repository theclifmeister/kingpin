package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
)

// The upgrades tree is one branch at a time (#120): right and left turn
// it to the next branch round the end, up and down walk the shown
// branch's nodes in tree order and stop at its ends, the tabs bracket
// the shown branch, and enter still buys the node under the cursor.
func TestUpgradeArrowsTurnTheBranch(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.w.Player.DirtyCash = 8000
	m.Update(key("6"))
	id := func() string {
		u, _ := m.upgradeSelected()
		return u.ID
	}
	ops := m.upgradeRows()
	if m.shownBranch() != "operations" || id() != ops[0].ID {
		t.Fatalf("the tree opens on %s at %s", m.shownBranch(), id())
	}
	m.Update(key("up"))
	if id() != ops[0].ID {
		t.Fatalf("up at the top moved to %s", id())
	}
	for range ops {
		m.Update(key("down"))
	}
	if id() != ops[len(ops)-1].ID {
		t.Fatalf("down past the end of Operations selected %s", id())
	}
	m.Update(key("right"))
	sec := m.upgradeRows()
	if m.shownBranch() != "security" || id() != sec[0].ID {
		t.Fatalf("right turned to %s at %s, want security at %s", m.shownBranch(), id(), sec[0].ID)
	}
	if !strings.Contains(stripANSI(m.View()), "[ Security ]") {
		t.Fatalf("the tabs do not bracket Security:\n%s", stripANSI(m.View()))
	}
	m.Update(key("left"))
	if m.shownBranch() != "operations" {
		t.Fatalf("left turned to %s", m.shownBranch())
	}
	m.Update(key("left"))
	if m.shownBranch() != content.Branches[len(content.Branches)-1] {
		t.Fatalf("left at the first branch turned to %s, want the last", m.shownBranch())
	}
	m.Update(key("right"))
	if m.shownBranch() != "operations" || m.screen != screenUpgrades || m.mode != modePlay {
		t.Fatalf("arrows left the tree: branch %s screen %v mode %v", m.shownBranch(), m.screen, m.mode)
	}
	// Enter buys the node under the cursor: the first of Security.
	m.Update(key("right"))
	m.Update(key("enter"))
	if m.mode != modeConfirmUpgrade || m.upgradeID != sec[0].ID {
		t.Fatalf("enter: mode %v id %q status %q", m.mode, m.upgradeID, m.status)
	}
	m.Update(key("y"))
	if !m.w.Owns(sec[0].ID) || m.w.Day != 0 {
		t.Fatalf("y did not buy %s: owns %v day %d status %q", sec[0].ID, m.w.Owns(sec[0].ID), m.w.Day, m.status)
	}
}

// TestBranchCursorSticks: each branch keeps its own node cursor, so a
// branch left and come back to is on the node it was on (#120).
func TestBranchCursorSticks(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.Update(key("6"))
	id := func() string {
		u, _ := m.upgradeSelected()
		return u.ID
	}
	ops := m.upgradeRows()
	m.Update(key("down"))
	m.Update(key("down"))
	if id() != ops[2].ID {
		t.Fatalf("cursor on %s, want %s", id(), ops[2].ID)
	}
	m.Update(key("right"))
	sec := m.upgradeRows()
	if id() != sec[0].ID {
		t.Fatalf("a branch first shown opens at %s, want its top %s", id(), sec[0].ID)
	}
	m.Update(key("down"))
	m.Update(key("right"))
	m.Update(key("right"))
	m.Update(key("left"))
	m.Update(key("left"))
	if m.shownBranch() != "security" || id() != sec[1].ID {
		t.Fatalf("back on %s at %s, want security at %s", m.shownBranch(), id(), sec[1].ID)
	}
	m.Update(key("left"))
	if m.shownBranch() != "operations" || id() != ops[2].ID {
		t.Fatalf("back on %s at %s, want operations at %s", m.shownBranch(), id(), ops[2].ID)
	}
	// Round the end too.
	for range content.Branches {
		m.Update(key("right"))
	}
	if m.shownBranch() != "operations" || id() != ops[2].ID {
		t.Fatalf("round the branches: %s at %s, want operations at %s", m.shownBranch(), id(), ops[2].ID)
	}
}

// TestBranchReadsAsATree: the shown branch is laid out with every node
// under the prerequisite it hangs from, one cell deeper, and every node
// of the tree is on exactly one branch's list (#120).
func TestBranchReadsAsATree(t *testing.T) {
	m := newTestModel(t, 100, 30)
	seen := map[string]bool{}
	for _, b := range content.Branches {
		rows := m.branchRows(b)
		depth := map[string]int{}
		for i, r := range rows {
			if seen[r.node.ID] {
				t.Errorf("%s is on two branches", r.node.ID)
			}
			seen[r.node.ID] = true
			depth[r.node.ID] = r.depth
			// A node one cell deeper than the row above hangs from it;
			// one at the same depth or shallower has a prerequisite at
			// depth-1 somewhere above it, or is a root.
			if r.depth == 0 {
				for _, req := range r.node.Requires {
					if m.cfg.Upgrades.Upgrade(req).Branch == b {
						t.Errorf("%s is a root of %s but needs %s from it", r.node.ID, b, req)
					}
				}
				continue
			}
			parent := ""
			for _, req := range r.node.Requires {
				if d, ok := depth[req]; ok && d == r.depth-1 {
					parent = req
				}
			}
			if parent == "" {
				t.Errorf("%s at depth %d has no prerequisite at depth %d above it", r.node.ID, r.depth, r.depth-1)
			}
			if i > 0 && r.depth > rows[i-1].depth+1 {
				t.Errorf("%s at depth %d follows %s at depth %d", r.node.ID, r.depth, rows[i-1].node.ID, rows[i-1].depth)
			}
		}
	}
	if len(seen) != len(m.cfg.Upgrades.Nodes) {
		t.Errorf("%d nodes on the branches, %d in the tree", len(seen), len(m.cfg.Upgrades.Nodes))
	}
	// Cut-outs needs Ghost crew and Safehouse, both two deep in Security:
	// it hangs from Ghost crew, the later in the file, a cell under it.
	rows := m.branchRows("security")
	for i, r := range rows {
		if r.node.ID == "cutouts" && (rows[i-1].node.ID != "ghosts" || r.depth != rows[i-1].depth+1) {
			t.Errorf("cutouts follows %s at depth %d (its own %d)", rows[i-1].node.ID, rows[i-1].depth, r.depth)
		}
	}
}

// TestEffectWordsCoverTheVocabulary: every name in the effect vocabulary
// reads as a sentence in the pane, so a node that carries it is never
// shown without it (#120).
func TestEffectWordsCoverTheVocabulary(t *testing.T) {
	rt := reflect.TypeOf(content.UpgradeEffects{})
	for i := 0; i < rt.NumField(); i++ {
		var e content.UpgradeEffects
		f := reflect.ValueOf(&e).Elem().Field(i)
		switch f.Kind() {
		case reflect.Float64:
			f.SetFloat(0.9)
		case reflect.Int:
			f.SetInt(2)
		default:
			t.Fatalf("%s: a %s in the vocabulary", rt.Field(i).Name, f.Kind())
		}
		words := effectWords(e)
		if len(words) != 1 {
			t.Errorf("%s set alone reads as %q, want one sentence", rt.Field(i).Name, words)
			continue
		}
		if strings.Contains(words[0], "0.90") || strings.Contains(words[0], "%!") {
			t.Errorf("%s reads as %q", rt.Field(i).Name, words[0])
		}
	}
	if got := effectWords(content.UpgradeEffects{WageMul: 0.9, RouteRiskMul: 0.8, CrewSlots: 2, SupplierMul: 0.92}); strings.Join(got, "; ") != "supplier price ×0.92; wages ×0.9; +2 crew; route risk ×0.8" {
		t.Errorf("effects read as %q", got)
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
	if m.mode != modePlay || m.status != "Undercut on the map screen (5). Buy upgrade on the upgrades screen (6)." {
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
	m.Update(key("j")) // stash3, under stash2 in the tree: locked behind it
	m.Update(key("enter"))
	if m.mode != modePlay || !strings.Contains(m.status, "Second stash") {
		t.Fatalf("locked: mode %v status %q", m.mode, m.status)
	}
	m.Update(key("1"))
	if !strings.Contains(stripANSI(m.View()), "upgrades 1 of "+fmt.Sprint(len(m.cfg.Upgrades.Nodes))) {
		t.Fatalf("dashboard does not count the stash:\n%s", stripANSI(m.View()))
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

// TestUpgradesScreenInTheGrammar: MAIN is the title with the count and
// the two pools, the branch tabs with the shown branch bracketed, the
// branch as one table with cash() costs and a status a word long, and
// the legend; no `*` anywhere, a clean-cash node says `clean` in a
// word. Every branch at 80x24 and 120x40: the tabs fit, every node is
// reached by the cursor and its row shows its cost whole. The pane's
// first line is the tree's cost string with the pool and where the
// node stands, the effects one per line, u names the price, and the
// BRANCH section under it says what the branch is for and how much is
// owned; every pane line fits the pane and at most one blank row sits
// between sections (#86, #120).
func TestUpgradesScreenInTheGrammar(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {120, 40}} {
		m := richModel(t, sz[0], sz[1])
		m.Update(key("6"))
		m.w.Player.DirtyCash += 20_000
		m.Update(key("u"))
		m.Update(key("y")) // one owned
		total := len(m.cfg.Upgrades.Nodes)
		for bi, branch := range content.Branches {
			if got := m.shownBranch(); got != branch {
				t.Fatalf("%dx%d: branch %d shown is %s, want %s", sz[0], sz[1], bi, got, branch)
			}
			view := stripANSI(m.View())
			main := strings.Split(stripANSI(m.viewScreen()), "\n")
			want := fmt.Sprintf("UPGRADES · 1 of %d owned · dirty %s · clean %s", total, cash(m.w.Player.DirtyCash), cash(m.w.Player.CleanCash))
			if strings.TrimRight(main[0], " ") != want {
				t.Errorf("%dx%d %s title: %q, want %q", sz[0], sz[1], branch, main[0], want)
			}
			// The tabs: every branch named, the shown one bracketed, the
			// line whole (the names shorten where the long ones do not
			// fit and are never cut).
			tabs := strings.TrimRight(main[1], " ")
			if strings.Count(tabs, "[ ") != 1 || strings.Contains(tabs, "…") {
				t.Errorf("%dx%d %s tabs: %q", sz[0], sz[1], branch, tabs)
			}
			if !strings.Contains(tabs, "[ "+branchName(branch)+" ]") && !strings.Contains(tabs, "[ "+branchShort[branch]+" ]") {
				t.Errorf("%dx%d %s: the tabs do not bracket it: %q", sz[0], sz[1], branch, tabs)
			}
			for _, b := range content.Branches {
				if !strings.Contains(tabs, branchName(b)) && !strings.Contains(tabs, branchShort[b]) {
					t.Errorf("%dx%d %s: the tabs lack %s: %q", sz[0], sz[1], branch, b, tabs)
				}
			}
			if strings.Contains(view, "*") {
				t.Errorf("%dx%d: a * on the upgrades screen:\n%s", sz[0], sz[1], view)
			}
			legend := false
			for _, l := range main {
				legend = legend || strings.TrimRight(l, " ") == "✓ owned  ○ available  · locked"
			}
			if !legend {
				t.Errorf("%dx%d %s: no legend line:\n%s", sz[0], sz[1], branch, view)
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
				t.Errorf("%dx%d %s: two blank rows between sections:\n%s", sz[0], sz[1], branch, view)
			}
			// Every node of the branch: the cursor reaches it, its row
			// carries its name and cost whole, the tree and the pane agree
			// on the cost, the pane's first line carries the pool and the
			// state, the BRANCH section follows, and the lines fit.
			rows := m.upgradeRows()
			owned, _ := m.ownedCount(branch)
			for i, u := range rows {
				*m.nodeCursor() = 0
				for range i {
					m.Update(key("down"))
				}
				if sel, _ := m.upgradeSelected(); sel.ID != u.ID {
					t.Fatalf("%s: %d downs reach %s", u.ID, i, sel.ID)
				}
				v := stripANSI(m.View())
				var line string
				for _, l := range strings.Split(stripANSI(m.viewScreen()), "\n") {
					if strings.HasPrefix(l, "▸") {
						line = l
					}
				}
				if !strings.Contains(line, u.Name) || !strings.Contains(line, cash(u.Cost)) {
					t.Errorf("%dx%d %s: the cursor's row %q lacks the name or %s", sz[0], sz[1], u.ID, line, cash(u.Cost))
				}
				status, _ := m.upgradeStatus(u)
				if !strings.Contains(line, status) && !strings.Contains(line, "…") {
					t.Errorf("%dx%d %s: the cursor's row %q lacks the status %q", sz[0], sz[1], u.ID, line, status)
				}
				secs := m.details()
				if len(secs) != 2 || secs[0].title != strings.ToUpper(u.Name) || secs[1].title != strings.ToUpper(branch) {
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
				for _, sec := range secs {
					for _, l := range sec.lines {
						if lipgloss.Width(l) > paneTextW {
							t.Errorf("%s: pane line wider than %d: %q", u.ID, paneTextW, stripANSI(l))
						}
					}
				}
				for _, e := range effectWords(u.Effects) {
					if !strings.Contains(strings.Join(secs[0].lines, "\n"), "  "+e) {
						t.Errorf("%s: the pane lacks the effect %q", u.ID, e)
					}
				}
				bl := stripANSI(strings.Join(secs[1].lines, "\n"))
				if !strings.Contains(bl, branchFor[branch]) || !strings.Contains(bl, fmt.Sprintf("%d of %d", owned, len(rows))) {
					t.Errorf("%s: the BRANCH section reads %q", u.ID, bl)
				}
				if strings.Count(v, cash(u.Cost)) < 2 {
					t.Errorf("%s: the tree and the pane do not both print %s:\n%s", u.ID, cash(u.Cost), v)
				}
				if state == "available" && m.canAfford(u) && !strings.Contains(strings.Join(secs[0].lines, "\n"), "u  buy it for "+cash(u.Cost)+" "+pool(u)) {
					t.Errorf("%s: the pane does not offer u for %s", u.ID, cash(u.Cost))
				}
				if u.Clean && !strings.Contains(line, "clean · ") {
					t.Errorf("%s: the row does not say clean: %q", u.ID, line)
				}
			}
			m.Update(key("right"))
		}
	}
}
