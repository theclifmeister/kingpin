package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/theme"
)

// crewRows is the list the crew cursor moves over: the roster first, then
// the people looking for work.
func (m *Model) crewRows() []game.CrewMember {
	c := m.w.Crew
	rows := append([]game.CrewMember(nil), c.Members...)
	return append(rows, c.Candidates...)
}

// crewSelected returns the member under the cursor and whether they are on
// the payroll (as opposed to a candidate).
func (m *Model) crewSelected() (game.CrewMember, bool, bool) {
	rows := m.crewRows()
	if len(rows) == 0 {
		return game.CrewMember{}, false, false
	}
	m.crewCursor = max(0, min(m.crewCursor, len(rows)-1))
	return rows[m.crewCursor], m.crewCursor < len(m.w.Crew.Members), true
}

func (m *Model) hireSelected() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || onPayroll {
		m.status = "Move the cursor to someone looking for work, then press h."
		return
	}
	got, err := m.w.Hire(c.ID, m.set.Crew.MaxCrew(m.w))
	if err != nil {
		m.status = "Can't hire: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s hired for %s.", got.Name, money(got.Fee))
	m.crewCursor = len(m.w.Crew.Members) - 1
}

func (m *Model) askFire() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.status = "Move the cursor to someone on the payroll, then press f."
		return
	}
	m.fireID = c.ID
	m.mode = modeConfirmFire
}

func (m *Model) confirmFire() {
	m.mode = modePlay
	got, err := m.w.Fire(m.fireID)
	if err != nil {
		m.status = "Can't fire: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s is gone. The rest noticed.", got.Name)
}

// talking reports whether the report's tell has shown enough for the
// screens to hint at it: the DA's file grew without a bust twice running.
func (m *Model) talking() bool { return m.w.Heat.Leaks >= 2 }

func (m *Model) askInvestigate() {
	if len(m.w.Crew.Members) == 0 {
		m.status = "Nobody on the payroll to ask."
		return
	}
	if m.w.Investigation != nil {
		m.status = "Somebody is already asking around tonight."
		return
	}
	m.mode = modeConfirmInvestigate
}

func (m *Model) confirmInvestigate() {
	m.mode = modePlay
	if err := m.w.Investigate(m.set.Crew.InvestigateCost()); err != nil {
		m.status = "Can't investigate: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Questions get asked tonight. Odds of a name ~%.0f%%.", m.set.Crew.InvestigateOdds(m.w)*100)
}

func (m *Model) investigateConfirm() string {
	w := m.w
	odds := m.set.Crew.InvestigateOdds(w)
	best := 0
	for _, c := range w.Crew.Members {
		if c.Role == "enforcer" && c.Skill > best {
			best = c.Skill
		}
	}
	who := "With no enforcer on the payroll you are asking yourself."
	if best > 0 {
		who = fmt.Sprintf("Your best enforcer (skill %d) does the asking.", best)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Somebody goes through the crew tonight for %s.\n", money(m.set.Crew.InvestigateCost())))
	b.WriteString(who + "\n")
	b.WriteString(fmt.Sprintf("If somebody is talking to the police, ~%.0f%% it names them.\n", odds*100))
	if w.Crew.Investigated > 0 {
		b.WriteString(theme.Subtle.Render(fmt.Sprintf("Every empty night so far (%d) narrows it down.", w.Crew.Investigated)) + "\n")
	}
	b.WriteString(theme.Warning.Render(fmt.Sprintf("Naming nobody costs everyone %.0f loyalty.", m.cfg.Crew.Informant.InvestigateLoyalty)) + "\n")
	b.WriteString("\n" + theme.Key.Render("y") + " ask   " + theme.Key.Render("any other key") + " back")
	return m.modal("INVESTIGATE THE CREW?", b.String())
}

func (m *Model) askPayOff() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.status = "Move the cursor to someone on the payroll, then press $."
		return
	}
	m.fireID = c.ID
	m.mode = modeConfirmPayOff
}

func (m *Model) confirmPayOff() {
	m.mode = modePlay
	c := m.w.Crew.Member(m.fireID)
	if c == nil {
		return
	}
	got, err := m.w.PayOff(c.ID, m.set.Crew.PayoffCost(*c), m.set.Crew.PayoffLoyalty())
	if err != nil {
		m.status = "Can't pay them off: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s pocketed it. Loyalty %.0f.", got.Name, got.Loyalty)
}

func (m *Model) payOffConfirm() string {
	c := m.w.Crew.Member(m.fireID)
	if c == nil {
		return m.modal("PAY OFF", "They are gone.")
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s for %s: loyalty %.0f → %.0f.\n", money(m.set.Crew.PayoffCost(*c)), c.Name, c.Loyalty, min(100, c.Loyalty+m.set.Crew.PayoffLoyalty())))
	b.WriteString(theme.Subtle.Render("It buys loyalty, not silence: somebody already talking keeps talking.") + "\n")
	b.WriteString("\n" + theme.Key.Render("y") + " pay   " + theme.Key.Render("any other key") + " back")
	return m.modal("PAY OFF "+strings.ToUpper(c.Name)+"?", b.String())
}

func (m *Model) cyclePay() {
	p := (m.w.Crew.Pay + 1) % 3
	m.w.SetPay(p)
	m.status = fmt.Sprintf("Pay %s, %s/day. %s", p, money(m.set.Crew.Wages(m.w, p)), payBlurb(p))
}

func payBlurb(p events.Pay) string {
	switch p {
	case events.PayStingy:
		return "Loyalty bleeds."
	case events.PayGenerous:
		return "Loyalty climbs."
	default:
		return "The greedy drift."
	}
}

func loyaltyStyle(l, threshold float64) lipgloss.Style {
	switch {
	case l < threshold:
		return theme.Bad
	case l < threshold+15:
		return theme.Warning
	default:
		return theme.Good
	}
}

func (m *Model) viewCrew() string {
	w := m.w
	tun := m.set.Crew.Tuning()
	pay := w.Crew.Pay
	var b strings.Builder

	title := theme.PanelTitle.Render("CREW") + theme.Subtle.Render(fmt.Sprintf("  %d of %d on the payroll", len(w.Crew.Members), m.set.Crew.MaxCrew(w)))
	var dial []string
	for p := events.PayStingy; p <= events.PayGenerous; p++ {
		if p == pay {
			dial = append(dial, theme.Selected.Render(" "+p.String()+" "))
		} else {
			dial = append(dial, theme.Subtle.Render(" "+p.String()+" "))
		}
	}
	b.WriteString(title + "   pay " + strings.Join(dial, "") + theme.Gold.Render(fmt.Sprintf("  %s/day", money(m.set.Crew.Wages(w, pay)))) + "\n")
	switch {
	case m.talking():
		b.WriteString(truncate(theme.Bad.Bold(true).Render("  ▲ Somebody is talking.")+theme.Bad.Render(" The file grew without a bust. Investigate (i) or fire your suspect."), m.width) + "\n")
	case w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < tun.SuspectDays:
		who := fmt.Sprintf("Somebody's loyalty is under %.0f.", tun.SkimThreshold)
		if w.Crew.Role(game.RoleLieutenant) > 0 {
			who = fmt.Sprintf("Somebody's loyalty is under %.0f, or a lieutenant is greedy.", tun.SkimThreshold)
		}
		b.WriteString(truncate(theme.Bad.Bold(true).Render("  ▲ Skimming suspected.")+theme.Bad.Render(fmt.Sprintf(" Money went missing on day %d. %s", w.Crew.LastSkim, who)), m.width) + "\n")
	case w.Crew.Role(game.RoleLieutenant) > 0:
		b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("  Skim under %.0f; a lieutenant turns under %.0f. Walk at %.0f; a lieutenant takes the city.", tun.SkimThreshold, m.set.Crew.FlipLine(), tun.QuitThreshold)), m.width) + "\n")
	default:
		b.WriteString(theme.Subtle.Render(fmt.Sprintf("  Under %.0f loyalty they skim. At %.0f they walk. Firing costs everyone else %.0f.", tun.SkimThreshold, tun.QuitThreshold, tun.FireLoyalty)) + "\n")
	}
	b.WriteString("\n")

	marks := []float64{tun.SkimThreshold / 100}
	crewStyle := lipgloss.NewStyle().Foreground(theme.Crew)
	// post is the status column: where the member is and, when they are
	// nowhere, what that means for their role. An accountant has no post,
	// they work every front you own; an enforcer without a corner guards
	// nothing; a runner without one is idle.
	post := func(c game.CrewMember) any {
		switch {
		case c.Lieutenant():
			if c.City != "" {
				return styled{crewStyle, "runs " + m.w.CityName(c.City)}
			}
			return styled{theme.Warning, "no city"}
		case c.Role == "accountant":
			switch n := len(w.Fronts); {
			case n == 0:
				return styled{theme.Warning, "no front"}
			case n == 1:
				return styled{crewStyle, "the books"}
			default:
				return styled{crewStyle, fmt.Sprintf("%d fronts", n)}
			}
		}
		if p := m.w.PostOf(c.ID); p != nil {
			return p.Name
		}
		if c.Role == "enforcer" {
			return styled{theme.Warning, "unposted"}
		}
		return styled{theme.Warning, "idle"}
	}
	// row is the cells every member and candidate shares: the loyalty
	// bar is coloured against the line they skim (or, for a lieutenant,
	// turn) under, and carry is what a runner adds to the stash.
	row := func(c game.CrewMember) []any {
		line := tun.SkimThreshold
		if c.Lieutenant() {
			line = m.set.Crew.FlipLine()
		}
		var carry any
		if c.Units > 0 {
			carry = c.Units
		}
		var name any = c.Name
		if c.ID == w.Crew.Exposed {
			name = styled{theme.Bad.Bold(true), c.Name}
		}
		return []any{name, c.Role, c.Skill, styled{loyaltyStyle(c.Loyalty, line), gauge{c.Loyalty / 100, marks, c.Loyalty}}, m.set.Crew.WageAt(c, pay), carry}
	}
	shared := []col{{"name", kText, 0}, {"role", kText, 0}, {"skill", kInt, 0}, {"loyalty", kBar, 10}, {"wage", kMoney, 0}, {"carry", kInt, 0}}

	b.WriteString(theme.Bold.Render("ON THE PAYROLL") + "\n")
	if len(w.Crew.Members) == 0 {
		b.WriteString(theme.Subtle.Render("  Nobody. Runners hold corners you can't stand on yourself; pick one below and press h.") + "\n")
	} else {
		var rows [][]any
		for _, c := range w.Crew.Members {
			rows = append(rows, append(row(c), post(c), day(c.Hired)))
		}
		for _, l := range table(append(shared, col{"post", kText, 0}, col{"hired", kDays, 0}), rows, m.crewCursor, m.width) {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("\n")

	next := tun.PoolDays - (w.Day - w.Crew.PoolDay)
	b.WriteString(theme.Bold.Render("LOOKING FOR WORK") + theme.Subtle.Render(fmt.Sprintf("  new faces in %d day(s)", max(1, next))) + "\n")
	if len(w.Crew.Candidates) == 0 {
		b.WriteString(theme.Subtle.Render("  Nobody right now.") + "\n")
	} else {
		var rows [][]any
		for _, c := range w.Crew.Candidates {
			var fee any = c.Fee
			if c.Fee > w.Player.DirtyCash {
				fee = styled{theme.Bad, c.Fee}
			}
			rows = append(rows, append(row(c), fee))
		}
		for _, l := range table(append(shared, col{"fee", kMoney, 0}), rows, m.crewCursor-len(w.Crew.Members), m.width) {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("\n")

	// What the crew adds, in the same terms the dashboard uses.
	here := w.Here()
	b.WriteString(truncate(theme.Subtle.Render(fmt.Sprintf("  Capacity in %s %d units (%d yours + %d crew) · %d corner(s) worked",
		here.Name, w.Capacity(here.ID), w.Player.CarryLimit, w.Capacity(here.ID)-w.Player.CarryLimit, w.Worked())), m.width) + "\n")
	idle, unposted := 0, 0
	for _, c := range w.Crew.Members {
		if w.PostOf(c.ID) != nil {
			continue
		}
		switch c.Role {
		case "runner":
			idle++
		case "enforcer":
			unposted++
		}
	}
	if idle > 0 {
		b.WriteString(theme.Warning.Render(fmt.Sprintf("  %d idle: a runner earns nothing off a corner. Post them on the map (5).", idle)) + "\n")
	}
	if unposted > 0 {
		b.WriteString(theme.Warning.Render(fmt.Sprintf("  %d unposted: post them on a corner to guard it (map, 5).", unposted)) + "\n")
	}
	if n := w.Crew.Role("accountant"); n > 0 {
		if len(w.Fronts) == 0 {
			b.WriteString(theme.Warning.Render("  An accountant with no front is a wage. Buy one on the ledger (7).") + "\n")
		} else {
			add, cut := m.accountants()
			b.WriteString(truncate(crewStyle.Render(fmt.Sprintf("  %d accountant(s): +%s/day through each front, audit risk cut %.0f%%, no post.", n, money(add), cut*100)), m.width) + "\n")
		}
	}
	if line := m.runsLine(); line != "" {
		b.WriteString(truncate(crewStyle.Render("  "+line+"."), m.width) + "\n")
	} else if n := w.Crew.Role(game.RoleLieutenant); n > 0 {
		b.WriteString(theme.Warning.Render("  A lieutenant with no city is a wage. Press t to give them one.") + "\n")
	}
	if c := w.Crew.Member(w.Crew.Exposed); c != nil {
		b.WriteString(theme.Bad.Render(fmt.Sprintf("  SNITCH: %s has been talking to the police. The file grows until they go (f).", c.Name)) + "\n")
	} else if w.Investigation != nil {
		b.WriteString(theme.Warning.Render("  Questions get asked tonight.") + "\n")
	}
	if sl := m.set.Heat.Sloppiness(w, here.ID); sl > 0 {
		per := sl * m.cfg.Heat.Heat.SloppyHeat * 100
		b.WriteString(theme.Warning.Render(fmt.Sprintf("  Sloppy runners (skill under %d) add +%.1f heat per 100 units moved.", m.cfg.Heat.Heat.SloppySkill, per)) + "\n")
	}
	return b.String()
}

// accountants is what the accountants on the payroll add to every front's
// daily wash at the current dial and the share of its audit risk they
// remove, by the laundering sim's rule: each counts by skill, and each
// takes its cut of the risk the ones before them left.
func (m *Model) accountants() (add int, cut float64) {
	tun := m.cfg.Laundering.Laundering
	through, risk := 0.0, 1.0
	for _, c := range m.w.Crew.Members {
		if c.Role != "accountant" {
			continue
		}
		skill := float64(c.Skill) / 100
		through += tun.AccountantThroughput * skill
		risk *= 1 - tun.AccountantRiskCut*skill
	}
	add = int(math.Round(through * m.set.Laundering.Dial(m.w.Laundering.Dial).Mul))
	return add, 1 - math.Max(0, risk)
}
