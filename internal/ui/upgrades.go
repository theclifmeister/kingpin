package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
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

// upgradeMove walks the tree as three columns: dc moves to the branch
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
	col, row := 0, m.upgradeCursor
	for col < len(lens)-1 && row >= lens[col] {
		row -= lens[col]
		col++
	}
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
		m.status = "You already have " + u.Name + "."
		return
	case "locked":
		var names []string
		for _, id := range m.w.Missing(u) {
			names = append(names, m.cfg.Upgrades.Upgrade(id).Name)
		}
		m.status = u.Name + " needs " + strings.Join(names, " and ") + " first."
		return
	}
	if !m.canAfford(u) {
		m.status = fmt.Sprintf("%s costs %s %s; you have %s.", u.Name, money(u.Cost), pool(u), money(m.poolCash(u)))
		return
	}
	m.upgradeID = u.ID
	m.mode = modeConfirmUpgrade
}

func (m *Model) confirmUpgrade() {
	m.mode = modePlay
	got, err := m.w.BuyUpgrade(m.cfg.Upgrades, m.upgradeID)
	if err != nil {
		m.status = "Can't buy: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s bought for %s %s. It is yours for the run.", got.Name, money(got.Cost), pool(got))
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

// effectWords spells a node's effects out the way the tooling reads them.
func effectWords(e content.UpgradeEffects) []string {
	var out []string
	if e.CarryBonus != 0 {
		out = append(out, fmt.Sprintf("carry +%d", e.CarryBonus))
	}
	if e.SupplierMul > 0 {
		out = append(out, fmt.Sprintf("supplier price ×%.2f", e.SupplierMul))
	}
	if e.BuyPressureMul > 0 {
		out = append(out, fmt.Sprintf("buy pressure ×%.1f", e.BuyPressureMul))
	}
	if e.FillMul > 0 {
		out = append(out, fmt.Sprintf("fill ×%.2f every dial", e.FillMul))
	}
	if e.SaleHeatMul > 0 {
		out = append(out, fmt.Sprintf("sale heat ×%.2f", e.SaleHeatMul))
	}
	if e.CrewHeatMul > 0 {
		out = append(out, fmt.Sprintf("runners' heat ×%.2f", e.CrewHeatMul))
	}
	if e.PatrolCap > 0 {
		out = append(out, fmt.Sprintf("patrols cap sales at %.0f%%", e.PatrolCap*100))
	}
	if e.CooldownBonus > 0 {
		out = append(out, fmt.Sprintf("+%d days between busts", e.CooldownBonus))
	}
	if e.StingStockMul > 0 {
		out = append(out, fmt.Sprintf("stings take ×%.1f stock", e.StingStockMul))
	}
	if e.RaidLossMul > 0 {
		out = append(out, fmt.Sprintf("raids take ×%.1f", e.RaidLossMul))
	}
	if e.LieLowMultiplier > 0 {
		out = append(out, fmt.Sprintf("lie low ×%.1f", e.LieLowMultiplier))
	}
	if e.Decay > 0 {
		out = append(out, fmt.Sprintf("heat fades %.0f%%/day", e.Decay*100))
	}
	if e.EvidenceCut > 0 {
		out = append(out, fmt.Sprintf("file −%d per bust", e.EvidenceCut))
	}
	if e.EvidenceDecayDays > 0 {
		out = append(out, fmt.Sprintf("file −1 per %d quiet days", e.EvidenceDecayDays))
	}
	if e.EvidenceArrest > 0 {
		out = append(out, fmt.Sprintf("indicted at file %d", e.EvidenceArrest))
	}
	if e.FallGuy {
		out = append(out, "survive one indictment")
	}
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
	b.WriteString(truncate(theme.PanelTitle.Render("UPGRADES")+
		theme.Subtle.Render(fmt.Sprintf("  %d of %d owned · ", owned, len(m.cfg.Upgrades.Nodes)))+
		theme.Gold.Render("dirty "+cash(w.Player.DirtyCash))+theme.Subtle.Render(" · clean "+cash(w.Player.CleanCash)), width) + "\n\n")

	// Three columns, one per branch; each node is a name-and-cost line
	// over a one-line effect. The cursor walks a column with up and down
	// and crosses to the next with left and right.
	colW := max(20, (width-1)/len(content.Branches))
	var cols []string
	idx := 0
	for _, branch := range content.Branches {
		var c strings.Builder
		c.WriteString(theme.Bold.Render(fit(strings.ToUpper(branch), colW)) + "\n")
		nodes := m.cfg.Upgrades.Branch(branch)
		// The mark in the gutter is the node's state; the cost is in
		// green when it is clean cash. The tree and the inspector print
		// a cost the same way, through cash().
		var rows [][]any
		cursor := -1
		for i, u := range nodes {
			state := m.upgradeState(u)
			sign, st := "·", theme.Subtle
			switch state {
			case "owned":
				sign, st = "✓", theme.Good
			case "available":
				sign, st = "○", theme.Gold
				if !m.canAfford(u) {
					st = theme.Warning
				}
			}
			var cost any = styled{st, u.Cost}
			if u.Clean {
				cost = styled{theme.Good, u.Cost}
			}
			rows = append(rows, []any{mark(sign), styled{st, u.Name}, cost})
			if idx == m.upgradeCursor {
				cursor = i
			}
			idx++
		}
		lines := table([]col{{"node", kText, 0}, {"cost", kCash, 0}}, rows, cursor, colW-1)
		c.WriteString(lines[0] + "\n")
		for i, u := range nodes {
			c.WriteString(fit(lines[i+1], colW) + "\n")
			c.WriteString(theme.Subtle.Render(fit("  "+strings.Join(effectWords(u.Effects), ", "), colW-2)) + "\n")
		}
		cols = append(cols, strings.TrimRight(c.String(), "\n"))
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cols...) + "\n")
	b.WriteString(truncate(theme.Good.Render("✓")+theme.Subtle.Render(" owned · ")+theme.Gold.Render("○")+theme.Subtle.Render(" available · · locked · ")+theme.Good.Render("green")+theme.Subtle.Render(" clean cash"), width) + "\n")
	return b.String()
}

// upgradesDetails is the tree's pane: the inspector for the node under
// the cursor (what it costs and from which pool, where it stands, what
// it does, one effect per line) and the keys.
func (m *Model) upgradesDetails() []section {
	w := m.w
	sel, ok := m.upgradeSelected()
	if !ok {
		return nil
	}
	lines := []string{row("cost", theme.Gold.Render(cash(sel.Cost)+" "+pool(sel)))}
	switch m.upgradeState(sel) {
	case "owned":
		lines = append(lines, row("status", theme.Good.Render("owned")))
	case "available":
		if m.canAfford(sel) {
			lines = append(lines, row("status", theme.Gold.Render("available")))
		} else {
			lines = append(lines, row("status", theme.Warning.Render(fmt.Sprintf("%s short", cash(sel.Cost-m.poolCash(sel))))))
		}
	default:
		var names []string
		for _, id := range w.Missing(sel) {
			names = append(names, m.cfg.Upgrades.Upgrade(id).Name)
		}
		label := "needs"
		for _, l := range wrap(strings.Join(names, " and "), paneTextW-paneLabelW-1) {
			lines = append(lines, row(label, theme.Subtle.Render(l)))
			label = ""
		}
	}
	lines = append(lines, wrapped(theme.Subtle, sel.Desc)...)
	for _, e := range effectWords(sel.Effects) {
		lines = append(lines, "  "+e)
	}
	if sel.Clean && w.Player.CleanCash == 0 {
		lines = append(lines, wrapped(theme.Subtle, "Clean cash only. Nothing you do yet makes any; that comes with the fronts.")...)
	}
	if sel.Effects.FallGuy && w.FallGuyUsed {
		lines = append(lines, wrapped(theme.Warning, "He already took his fall. There is no second one.")...)
	}
	if m.upgradeState(sel) == "available" && m.canAfford(sel) {
		lines = append(lines, keyRow("u", "buy it"))
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
