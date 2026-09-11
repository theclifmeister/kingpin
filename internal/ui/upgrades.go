package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// upgradeRows is the list the upgrades cursor walks: every node, branch by
// branch, in tree order.
func (m *Model) upgradeRows() []content.UpgradeConfig {
	var rows []content.UpgradeConfig
	for _, b := range content.Branches {
		rows = append(rows, m.cfg.Upgrades.Branch(b)...)
	}
	return rows
}

// upgradeSelected returns the node under the cursor.
func (m *Model) upgradeSelected() (content.UpgradeConfig, bool) {
	rows := m.upgradeRows()
	if len(rows) == 0 {
		return content.UpgradeConfig{}, false
	}
	m.upgradeCursor = max(0, min(m.upgradeCursor, len(rows)-1))
	return rows[m.upgradeCursor], true
}

// upgradeMove walks the tree as columns, one a branch: dc moves to the branch
// beside this one at the same row (the last node if that column is
// shorter), dr up or down within the column. The edges are no-ops.
func (m *Model) upgradeMove(dc, dr int) {
	if _, ok := m.upgradeSelected(); !ok {
		return
	}
	var lens []int
	for _, b := range content.Branches {
		lens = append(lens, len(m.cfg.Upgrades.Branch(b)))
	}
	col, row := m.upgradeAt()
	col = max(0, min(col+dc, len(lens)-1))
	if lens[col] == 0 {
		return
	}
	row = max(0, min(row+dr, lens[col]-1))
	for _, n := range lens[:col] {
		row += n
	}
	m.upgradeCursor = row
}

// upgradeState is where a node stands for the player: owned, available
// (prerequisites met) or locked.
func (m *Model) upgradeState(u content.UpgradeConfig) string {
	switch {
	case m.w.Owns(u.ID):
		return "owned"
	case len(m.w.Missing(u)) == 0:
		return "available"
	default:
		return "locked"
	}
}

// canAfford reports whether the right pool covers the node's cost.
func (m *Model) canAfford(u content.UpgradeConfig) bool {
	if u.Clean {
		return m.w.Player.CleanCash >= u.Cost
	}
	return m.w.Player.DirtyCash >= u.Cost
}

// askUpgrade opens the confirmation for buying the selected node, or says
// why it cannot be bought.
func (m *Model) askUpgrade() {
	u, ok := m.upgradeSelected()
	if !ok || m.w.Over != nil {
		return
	}
	switch m.upgradeState(u) {
	case "owned":
		m.refuse("Can't buy " + u.Name + ": you already have it.")
		return
	case "locked":
		var names []string
		for _, id := range m.w.Missing(u) {
			names = append(names, m.cfg.Upgrades.Upgrade(id).Name)
		}
		m.refuse("Can't buy " + u.Name + ": it needs " + strings.Join(names, " and ") + " first.")
		return
	}
	if !m.canAfford(u) {
		m.refuse(fmt.Sprintf("Can't buy %s: it costs %s %s, and you have %s.", u.Name, money(u.Cost), pool(u), money(m.poolCash(u))))
		return
	}
	m.upgradeID = u.ID
	m.mode = modeConfirmUpgrade
}

func (m *Model) confirmUpgrade() {
	m.mode = modePlay
	got, err := m.w.BuyUpgrade(m.cfg.Upgrades, m.upgradeID)
	if err != nil {
		m.refuse("Can't buy: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s bought for %s %s. It is yours for the run.", got.Name, money(got.Cost), pool(got)))
}

func pool(u content.UpgradeConfig) string {
	if u.Clean {
		return "clean"
	}
	return "dirty"
}

func (m *Model) poolCash(u content.UpgradeConfig) int {
	if u.Clean {
		return m.w.Player.CleanCash
	}
	return m.w.Player.DirtyCash
}

// effectWords spells a node's effects out the way the tooling reads
// them, one sentence an effect, in the order of the vocabulary (#117):
// the market's, heat's, the crew's, laundering's, the road's and the
// street's.
func effectWords(e content.UpgradeEffects) []string {
	var out []string
	add := func(s string) { out = append(out, s) }
	mul := func(v float64, format string) {
		if v > 0 {
			add(fmt.Sprintf(format, v))
		}
	}
	bonus := func(v int, noun string) {
		if v != 0 {
			add(fmt.Sprintf("%+d %s", v, noun))
		}
	}
	if e.CarryBonus != 0 {
		add(fmt.Sprintf("carry +%d", e.CarryBonus))
	}
	// The market.
	mul(e.SupplierMul, "supplier price ×%.2f")
	mul(e.BuyPressureMul, "buy pressure ×%.1f")
	mul(e.FillMul, "fill ×%.2f every dial")
	mul(e.SaleImpactMul, "price impact ×%.1f")
	mul(e.DemandMul, "demand ×%.2f on your corners")
	mul(e.GlutDecayMul, "gluts clear ×%.1f faster")
	mul(e.BuyerGapMul, "buyers come ×%.1f as often")
	if e.ContractPremiumBonus != 0 {
		add(fmt.Sprintf("contracts pay +%.0f%%", e.ContractPremiumBonus*100))
	}
	// Heat.
	mul(e.SaleHeatMul, "sale heat ×%.2f")
	mul(e.CrewHeatMul, "runners' heat ×%.2f")
	if e.PatrolCap > 0 {
		add(fmt.Sprintf("patrols cap sales at %.0f%%", e.PatrolCap*100))
	}
	if e.CooldownBonus > 0 {
		add("+" + plural(e.CooldownBonus, "day") + " between busts")
	}
	mul(e.StingStockMul, "stings take ×%.1f stock")
	mul(e.RaidLossMul, "raids take ×%.1f")
	mul(e.LieLowMultiplier, "lie low ×%.1f")
	if e.Decay > 0 {
		add(fmt.Sprintf("heat fades %.0f%%/day", e.Decay*100))
	}
	mul(e.DirtyCashThresholdMul, "cash pile ×%.1f before heat")
	if e.EvidenceCut > 0 {
		add(fmt.Sprintf("file −%d per bust", e.EvidenceCut))
	}
	if e.AuditEvidenceCut > 0 {
		add(fmt.Sprintf("file −%d per audit", e.AuditEvidenceCut))
	}
	if e.EvidenceDecayDays > 0 {
		add(fmt.Sprintf("file −1 per %d quiet days", e.EvidenceDecayDays))
	}
	if e.EvidenceArrest > 0 {
		add(fmt.Sprintf("indicted at file %d", e.EvidenceArrest))
	}
	if e.FallGuys > 0 {
		add("survive " + plural(e.FallGuys, "indictment"))
	}
	// The crew.
	mul(e.WageMul, "wages ×%.2f")
	mul(e.LoyaltyLossMul, "loyalty loss ×%.1f")
	mul(e.DangerLoyaltyMul, "danger costs ×%.1f loyalty")
	mul(e.SkimChanceMul, "skimming ×%.1f")
	mul(e.InformantChanceMul, "turning ×%.1f")
	bonus(e.CrewSlots, "crew")
	bonus(e.CandidatesBonus, "faces looking for work")
	if e.PoolDaysCut > 0 {
		add(fmt.Sprintf("new faces %d days sooner", e.PoolDaysCut))
	}
	bonus(e.SkillBonus, "skill on new faces")
	mul(e.HireFeeMul, "signing fees ×%.1f")
	bonus(e.StartLoyaltyBonus, "loyalty on new faces")
	// Laundering.
	mul(e.WashMul, "wash ×%.2f every front")
	mul(e.AuditRiskMul, "audit risk ×%.1f")
	mul(e.AuditSeizeMul, "audits seize ×%.1f")
	mul(e.UpkeepMul, "upkeep ×%.1f")
	if e.AuditFreezeCut > 0 {
		add(fmt.Sprintf("audits freeze %d days less", e.AuditFreezeCut))
	}
	mul(e.FloatMul, "float ×%.1f")
	// The road.
	mul(e.RouteRiskMul, "route risk ×%.1f")
	mul(e.RouteCapacityMul, "route capacity ×%.1f")
	mul(e.RouteDaysMul, "road days ×%.2f")
	mul(e.FareMul, "fares ×%.1f")
	mul(e.WholesaleMul, "wholesale price ×%.1f")
	// The street.
	if e.DriftDaysBonus > 0 {
		add(fmt.Sprintf("corners drift %d days later", e.DriftDaysBonus))
	}
	mul(e.RobberyMul, "robberies ×%.1f")
	bonus(e.GuardBonus, "guard on every contested corner")
	mul(e.RivalPushMul, "rival pushes ×%.1f")
	return out
}

// ownedLine is the dashboard's compact list of what the player has bought.
func (m *Model) ownedLine() string {
	var ids []string
	for _, u := range m.upgradeRows() {
		if m.w.Owns(u.ID) {
			ids = append(ids, u.ID)
		}
	}
	if len(ids) == 0 {
		return "no upgrades yet: buy " + screenPointer(screenUpgrades)
	}
	return "upgrades " + strings.Join(ids, ", ")
}

// upgradeAt is the column and the row within it of the cursor.
func (m *Model) upgradeAt() (col, row int) {
	row = m.upgradeCursor
	for col < len(content.Branches)-1 && row >= len(m.cfg.Upgrades.Branch(content.Branches[col])) {
		row -= len(m.cfg.Upgrades.Branch(content.Branches[col]))
		col++
	}
	return col, row
}

// upgradePage is the window of rows the columns show: the tree is
// taller than MAIN at 80x24 since #117, so the columns page together
// (they walk at the same row) by as many nodes as fit under the title,
// the pools, the column heads and over the legend, and the page is the
// cursor's. The screen for seven branches is #120's.
func (m *Model) upgradePage() (top, per int) {
	per = max(1, (m.mainHeight()-7)/2)
	_, row := m.upgradeAt()
	return row / per * per, per
}

// upgradeColMin is the narrowest a branch's column is drawn.
const upgradeColMin = 20

// upgradeColumns is the window of branches MAIN shows: since #118 the
// tree is wider than MAIN too, so it shows as many columns of
// upgradeColMin as the width holds, a space apart, the leftmost window
// that has the cursor's column in it (stateless, like the row page);
// left and right still cross every branch, and the title says which
// are shown.
func (m *Model) upgradeColumns() (first, per int) {
	per = max(1, min(len(content.Branches), (m.mainWidth()+1)/(upgradeColMin+1)))
	col, _ := m.upgradeAt()
	return max(0, min(col, len(content.Branches)-per)), per
}

// viewUpgrades is the tree's MAIN (#86): the title with the count, the
// two pools, and the branches as columns (as many as fit), each node a
// name-and-cost line over a one-line summary of its effects; the node
// under the cursor is the pane's.
func (m *Model) viewUpgrades() string {
	w := m.w
	width := m.mainWidth()
	var b strings.Builder
	owned := 0
	for _, u := range m.cfg.Upgrades.Nodes {
		if w.Owns(u.ID) {
			owned++
		}
	}
	firstCol, perCol := m.upgradeColumns()
	title := sectionTitle("UPGRADES", theme.Money) + theme.Subtle.Render(fmt.Sprintf(" · %d of %d owned", owned, len(m.cfg.Upgrades.Nodes)))
	if perCol < len(content.Branches) {
		title += theme.Subtle.Render(fmt.Sprintf(" · branches %d–%d of %d", firstCol+1, min(firstCol+perCol, len(content.Branches)), len(content.Branches)))
	}
	b.WriteString(truncate(title, width) + "\n")
	b.WriteString(truncate(theme.Gold.Render("dirty "+cash(w.Player.DirtyCash))+theme.Subtle.Render(" · ")+theme.Good.Render("clean "+cash(w.Player.CleanCash)), width) + "\n\n")

	// The branches as columns a space apart, sharing the width, as
	// many as it holds at once (upgradeColumns). The cursor walks a
	// column with up and down and crosses to the next with left and
	// right; the nodes of a branch off the page still count toward
	// the cursor's index.
	colW := max(upgradeColMin, (width-perCol+1)/perCol)
	top, per := m.upgradePage()
	var cols []string
	idx := 0
	for i, branch := range content.Branches {
		all := m.cfg.Upgrades.Branch(branch)
		if i < firstCol || i >= firstCol+perCol {
			idx += len(all)
			continue
		}
		var c strings.Builder
		title := sectionTitle(strings.ToUpper(branch), theme.Money)
		if len(all) > per {
			title += theme.Subtle.Render(fmt.Sprintf(" · %d–%d of %d", min(top+1, len(all)), min(top+per, len(all)), len(all)))
		}
		c.WriteString(fit(title, colW) + "\n")
		// The page's window of the branch; the cursor is an index into
		// the whole tree, so the nodes before the window still count.
		nodes := all[min(top, len(all)):min(top+per, len(all))]
		idx += min(top, len(all))
		// The mark in the gutter is the node's state; the cost is
		// through cash(), as the pane prints it, and a node paid in
		// clean cash says so on its effects line.
		var rows [][]any
		cursor := -1
		for i, u := range nodes {
			sign, st := "·", theme.Subtle
			switch m.upgradeState(u) {
			case "owned":
				sign, st = "✓", theme.Good
			case "available":
				sign, st = "○", theme.Gold
				if !m.canAfford(u) {
					st = theme.Warning
				}
			}
			rows = append(rows, []any{mark(sign), styled{st, u.Name}, styled{st, u.Cost}})
			if idx == m.upgradeCursor {
				cursor = i
			}
			idx++
		}
		lines := table([]col{{"node", kText, 0}, {"cost", kCash, 0}}, rows, cursor, colW)
		c.WriteString(fit(lines[0], colW) + "\n")
		for i, u := range nodes {
			c.WriteString(fit(lines[i+1], colW) + "\n")
			words := effectWords(u.Effects)
			if u.Clean {
				words = append([]string{theme.Good.Render("clean")}, words...)
			}
			c.WriteString(fit(theme.Subtle.Render("  "+truncate(strings.Join(words, ", "), colW-2)), colW) + "\n")
		}
		idx += len(all) - min(top+per, len(all))
		cols = append(cols, strings.TrimRight(c.String(), "\n"), " ")
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cols[:len(cols)-1]...) + "\n\n")
	b.WriteString(truncate(theme.Good.Render("✓")+theme.Subtle.Render(" owned  ")+theme.Gold.Render("○")+theme.Subtle.Render(" available  · locked"), width) + "\n")
	return b.String()
}

// costLine is a node's cost and pool as the pane's first line prints
// them: `$12K dirty`, the tree's cost string.
func costLine(u content.UpgradeConfig) string {
	return cash(u.Cost) + " " + pool(u)
}

// upgradesDetails is the tree's pane (#86): the node under the cursor,
// its cost and pool with where it stands (owned, available, short, or
// what it needs first), what it does in a sentence and one effect a
// line, the notes that apply, and what u would do.
func (m *Model) upgradesDetails() []section {
	w := m.w
	sel, ok := m.upgradeSelected()
	if !ok {
		return nil
	}
	cost := theme.Gold.Render(costLine(sel))
	var lines []string
	switch m.upgradeState(sel) {
	case "owned":
		lines = append(lines, cost+theme.Subtle.Render(" · ")+theme.Good.Render("owned"))
	case "available":
		if m.canAfford(sel) {
			lines = append(lines, cost+theme.Subtle.Render(" · ")+theme.Gold.Render("available"))
		} else {
			lines = append(lines, cost+theme.Subtle.Render(" · ")+theme.Warning.Render(fmt.Sprintf("%s short", cash(sel.Cost-m.poolCash(sel)))))
		}
	default:
		var names []string
		for _, id := range w.Missing(sel) {
			names = append(names, m.cfg.Upgrades.Upgrade(id).Name)
		}
		needs := "needs " + strings.Join(names, " and ")
		if lipgloss.Width(costLine(sel)+" · "+needs) <= paneTextW {
			lines = append(lines, cost+theme.Subtle.Render(" · "+needs))
		} else {
			lines = append(lines, cost)
			label := "needs"
			for _, l := range wrap(strings.Join(names, " and "), paneTextW-paneLabelW-1) {
				lines = append(lines, row(label, theme.Subtle.Render(l)))
				label = ""
			}
		}
	}
	lines = append(lines, wrapped(theme.Subtle, sel.Desc)...)
	for _, e := range effectWords(sel.Effects) {
		lines = append(lines, "  "+e)
	}
	if sel.Clean && w.Player.CleanCash == 0 {
		lines = append(lines, wrapped(theme.Subtle, "Clean cash only. Nothing you do yet makes any; that comes with the fronts.")...)
	}
	if sel.Effects.FallGuys > 0 && w.Owns(sel.ID) && !w.FallGuyLeft(game.FoldEffects(w, m.cfg.Upgrades)) {
		lines = append(lines, wrapped(theme.Warning, "He already took his fall. There is no second one.")...)
	}
	if m.upgradeState(sel) == "available" && m.canAfford(sel) {
		lines = append(lines, keyRow("u", "buy it for "+costLine(sel)))
	}
	return []section{{strings.ToUpper(sel.Name), lines}}
}

// upgradeConfirm is the modal body for buying the node awaiting yes.
func (m *Model) upgradeConfirm() string {
	u := m.cfg.Upgrades.Upgrade(m.upgradeID)
	if u == nil {
		return m.modal("BUY?", []string{"Nothing selected."}, m.modalFooter())
	}
	body := []string{
		fmt.Sprintf("%s for %s %s cash.", u.Name, money(u.Cost), pool(*u)),
		"It applies at once and stays for the run.",
		theme.Subtle.Render(strings.Join(effectWords(u.Effects), " · ")),
	}
	return m.modal("BUY "+u.Name+"?", body, m.modalFooter())
}
