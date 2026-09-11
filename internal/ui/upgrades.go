package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/content"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// upgradeRow is one row of the shown branch's table: the node and how
// deep it sits under the branch's roots, one level per prerequisite in
// the branch (#120).
type upgradeRow struct {
	node  content.UpgradeConfig
	depth int
}

// branchRows lays a branch out as a tree: a node's parent is the
// deepest of its prerequisites in the branch (the later in the file
// between two as deep; a prerequisite in another branch is the status
// column's business, not the layout's), the roots are the nodes with
// none, and each node's children follow it in file order, one cell
// deeper, so the branch reads as a tree and the cursor walks it top to
// bottom.
func (m *Model) branchRows(branch string) []upgradeRow {
	nodes := m.cfg.Upgrades.Branch(branch)
	depth := map[string]int{}
	order := map[string]int{}
	children := map[string][]content.UpgradeConfig{}
	var roots []content.UpgradeConfig
	for i, u := range nodes {
		order[u.ID] = i
		parent := ""
		for _, r := range u.Requires {
			if m.cfg.Upgrades.Upgrade(r).Branch != branch {
				continue
			}
			if parent == "" || depth[r] > depth[parent] || depth[r] == depth[parent] && order[r] > order[parent] {
				parent = r
			}
		}
		if parent == "" {
			roots = append(roots, u)
			continue
		}
		depth[u.ID] = depth[parent] + 1
		children[parent] = append(children[parent], u)
	}
	var rows []upgradeRow
	var walk func(u content.UpgradeConfig)
	walk = func(u content.UpgradeConfig) {
		rows = append(rows, upgradeRow{u, depth[u.ID]})
		for _, c := range children[u.ID] {
			walk(c)
		}
	}
	for _, u := range roots {
		walk(u)
	}
	return rows
}

// shownBranch is the branch the upgrades screen is turned to.
func (m *Model) shownBranch() string {
	m.branch = max(0, min(m.branch, len(content.Branches)-1))
	return content.Branches[m.branch]
}

// upgradeRows is the list the upgrades cursor walks: the shown branch's
// nodes in tree order (branchRows).
func (m *Model) upgradeRows() []content.UpgradeConfig {
	var rows []content.UpgradeConfig
	for _, r := range m.branchRows(m.shownBranch()) {
		rows = append(rows, r.node)
	}
	return rows
}

// nodeCursor is the shown branch's cursor: each branch keeps its own,
// so a branch left and come back to is on the node it was on.
func (m *Model) nodeCursor() *int {
	m.shownBranch()
	for len(m.upgradeCursor) < len(content.Branches) {
		m.upgradeCursor = append(m.upgradeCursor, 0)
	}
	return &m.upgradeCursor[m.branch]
}

// upgradeSelected returns the node under the cursor.
func (m *Model) upgradeSelected() (content.UpgradeConfig, bool) {
	rows := m.upgradeRows()
	if len(rows) == 0 {
		return content.UpgradeConfig{}, false
	}
	c := m.nodeCursor()
	*c = max(0, min(*c, len(rows)-1))
	return rows[*c], true
}

// upgradeMove walks the tree: dc turns it to the branch beside this one
// (round the end, as the market's arrows turn the city), dr up or down
// the shown branch's nodes; the ends of a branch are no-ops.
func (m *Model) upgradeMove(dc, dr int) {
	if dc != 0 {
		m.branch = (m.branch + dc + len(content.Branches)) % len(content.Branches)
		return
	}
	if _, ok := m.upgradeSelected(); !ok {
		return
	}
	c := m.nodeCursor()
	*c = max(0, min(*c+dr, len(m.upgradeRows())-1))
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
			add(fmt.Sprintf(format, times(v)))
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
	mul(e.SupplierMul, "supplier price ×%s")
	mul(e.BuyPressureMul, "buy pressure ×%s")
	mul(e.FillMul, "fill ×%s every dial")
	mul(e.SaleImpactMul, "price impact ×%s")
	mul(e.DemandMul, "demand ×%s on your corners")
	mul(e.GlutDecayMul, "gluts clear ×%s faster")
	mul(e.BuyerGapMul, "buyers come ×%s as often")
	if e.ContractPremiumBonus != 0 {
		add(fmt.Sprintf("contracts pay +%.0f%%", e.ContractPremiumBonus*100))
	}
	// Heat.
	mul(e.SaleHeatMul, "sale heat ×%s")
	mul(e.CrewHeatMul, "runners' heat ×%s")
	if e.PatrolCap > 0 {
		add(fmt.Sprintf("patrols cap sales at %.0f%%", e.PatrolCap*100))
	}
	if e.CooldownBonus > 0 {
		add("+" + plural(e.CooldownBonus, "day") + " between busts")
	}
	mul(e.StingStockMul, "stings take ×%s stock")
	mul(e.RaidLossMul, "raids take ×%s")
	mul(e.LieLowMultiplier, "lie low ×%s")
	if e.Decay > 0 {
		add(fmt.Sprintf("heat fades %.0f%%/day", e.Decay*100))
	}
	mul(e.DirtyCashThresholdMul, "cash pile ×%s before heat")
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
	mul(e.WageMul, "wages ×%s")
	mul(e.LoyaltyLossMul, "loyalty loss ×%s")
	mul(e.DangerLoyaltyMul, "danger costs ×%s loyalty")
	mul(e.SkimChanceMul, "skimming ×%s")
	mul(e.InformantChanceMul, "turning ×%s")
	bonus(e.CrewSlots, "crew")
	bonus(e.CandidatesBonus, "faces looking for work")
	if e.PoolDaysCut > 0 {
		add(fmt.Sprintf("new faces %d days sooner", e.PoolDaysCut))
	}
	bonus(e.SkillBonus, "skill on new faces")
	mul(e.HireFeeMul, "signing fees ×%s")
	bonus(e.StartLoyaltyBonus, "loyalty on new faces")
	// Laundering.
	mul(e.WashMul, "wash ×%s every front")
	mul(e.AuditRiskMul, "audit risk ×%s")
	mul(e.AuditSeizeMul, "audits seize ×%s")
	mul(e.UpkeepMul, "upkeep ×%s")
	if e.AuditFreezeCut > 0 {
		add(fmt.Sprintf("audits freeze %d days less", e.AuditFreezeCut))
	}
	mul(e.FloatMul, "float ×%s")
	// The road.
	mul(e.RouteRiskMul, "route risk ×%s")
	mul(e.RouteCapacityMul, "route capacity ×%s")
	mul(e.RouteDaysMul, "road days ×%s")
	mul(e.FareMul, "fares ×%s")
	mul(e.WholesaleMul, "wholesale price ×%s")
	// The street.
	if e.DriftDaysBonus > 0 {
		add(fmt.Sprintf("corners drift %d days later", e.DriftDaysBonus))
	}
	mul(e.RobberyMul, "robberies ×%s")
	bonus(e.GuardBonus, "guard on contested corners")
	mul(e.RivalPushMul, "rival pushes ×%s")
	return out
}

// times writes a multiplier as the effects read it: `0.9`, `0.75`,
// `1.5`, two decimals at most and no trailing zero.
func times(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// ownedLine is the dashboard's fact on the tree: how much of it is
// yours (`upgrades 9 of 56`), or where to buy the first node.
func (m *Model) ownedLine() string {
	owned, total := m.ownedCount()
	if owned == 0 {
		return "no upgrades yet: buy " + screenPointer(screenUpgrades)
	}
	return fmt.Sprintf("upgrades %d of %d", owned, total)
}

// ownedCount is how many nodes of the tree are owned, and how many
// there are; given a branch, of that branch alone.
func (m *Model) ownedCount(branch ...string) (owned, total int) {
	for _, u := range m.cfg.Upgrades.Nodes {
		if len(branch) > 0 && u.Branch != branch[0] {
			continue
		}
		total++
		if m.w.Owns(u.ID) {
			owned++
		}
	}
	return owned, total
}

// branchShort is a branch's name where the tabs have no room for the
// long ones, as the title bar's tabs shorten.
var branchShort = map[string]string{"operations": "Ops", "laundering": "Launder", "logistics": "Road"}

// branchFor is what a branch is for, in one line of the pane
// (paneTextW): the BRANCH section says it under the node.
var branchFor = map[string]string{
	"operations": "Hold more, buy and sell better.",
	"security":   "Take less damage, cool faster.",
	"legal":      "Survive the case the DA builds.",
	"crew":       "Cheaper, steadier, more of them.",
	"laundering": "Wash more, get looked at less.",
	"logistics":  "Move more for less on the road.",
	"street":     "Hold your corners, lose fewer.",
}

// branchName is a branch's name as the tabs print it.
func branchName(branch string) string {
	return strings.ToUpper(branch[:1]) + branch[1:]
}

// branchTabs is the branch selector under the upgrades title, in the
// market's city-tab convention: every branch in order, the shown one in
// brackets and Selected; the names shorten where the width has no room
// for the long ones, as the title bar's tabs do.
func (m *Model) branchTabs(width int) string {
	long := make([]string, len(content.Branches))
	short := make([]string, len(content.Branches))
	for i, b := range content.Branches {
		long[i] = branchName(b)
		short[i] = long[i]
		if s, ok := branchShort[b]; ok {
			short[i] = s
		}
	}
	if line := tabLine(long, m.branch); lipgloss.Width(line) <= width {
		return line
	}
	return truncate(tabLine(short, m.branch), width)
}

// tabLine is the branch tabs in the market's city-tab convention
// (cityTabs, which #113 holds; fold the two once it lands): the labels
// two spaces apart in Subtle, the shown one in brackets and Selected.
func tabLine(labels []string, shown int) string {
	var parts []string
	for i, label := range labels {
		if i == shown {
			parts = append(parts, theme.Selected.Render("[ "+label+" ]"))
		} else {
			parts = append(parts, theme.Subtle.Render(label))
		}
	}
	return strings.Join(parts, "  ")
}

// upgradeStatus is where a node stands as the table's status column
// says it: owned, available, what it needs first, or how short the
// pool is.
func (m *Model) upgradeStatus(u content.UpgradeConfig) (string, lipgloss.Style) {
	switch m.upgradeState(u) {
	case "owned":
		return "owned", theme.Good
	case "available":
		if m.canAfford(u) {
			return "available", theme.Gold
		}
		return cash(u.Cost-m.poolCash(u)) + " short", theme.Warning
	}
	var names []string
	for _, id := range m.w.Missing(u) {
		names = append(names, m.cfg.Upgrades.Upgrade(id).Name)
	}
	return "needs " + strings.Join(names, " and "), theme.Subtle
}

// viewUpgrades is the tree's MAIN (#86, #120): the title with the count
// and the two pools, the branch tabs, the shown branch as one table
// (the mark in the gutter is the node's state, the name indented a cell
// a level under its prerequisite so the branch reads as a tree, the
// cost through cash() as the pane prints it, and where the node stands)
// and the legend; the node under the cursor is the pane's.
func (m *Model) viewUpgrades() string {
	w := m.w
	width := m.mainWidth()
	var b strings.Builder
	owned, total := m.ownedCount()
	sep := theme.Subtle.Render(" · ")
	title := sectionTitle("UPGRADES", theme.Money) + theme.Subtle.Render(fmt.Sprintf(" · %d of %d owned", owned, total)) +
		sep + theme.Gold.Render("dirty "+cash(w.Player.DirtyCash)) + sep + theme.Good.Render("clean "+cash(w.Player.CleanCash))
	b.WriteString(truncate(title, width) + "\n")
	b.WriteString(m.branchTabs(width) + "\n\n")

	var rows [][]any
	for _, r := range m.branchRows(m.shownBranch()) {
		u := r.node
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
		status, sst := m.upgradeStatus(u)
		if u.Clean {
			status = "clean · " + status
		}
		rows = append(rows, []any{mark(sign), styled{st, strings.Repeat(" ", r.depth) + u.Name}, styled{st, u.Cost}, styled{sst, status}})
	}
	cursor := -1
	if _, ok := m.upgradeSelected(); ok {
		cursor = *m.nodeCursor()
	}
	for _, l := range table([]col{{"node", kText, 0}, {"cost", kCash, 0}, {"status", kText, 0}}, rows, cursor, width) {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n" + truncate(theme.Good.Render("✓")+theme.Subtle.Render(" owned  ")+theme.Gold.Render("○")+theme.Subtle.Render(" available  · locked"), width) + "\n")
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
	return []section{{strings.ToUpper(sel.Name), lines}, m.branchSection()}
}

// branchSection is the pane's BRANCH section (#120): what the shown
// branch is for, in a line, and how much of it is owned.
func (m *Model) branchSection() section {
	branch := m.shownBranch()
	owned, total := m.ownedCount(branch)
	lines := wrapped(theme.Subtle, branchFor[branch])
	lines = append(lines, row("owned", fmt.Sprintf("%d of %d", owned, total)))
	return section{strings.ToUpper(branch), lines}
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
