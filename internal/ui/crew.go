package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/theclifmeister/kingpin/internal/events"
	"github.com/theclifmeister/kingpin/internal/game"
	"github.com/theclifmeister/kingpin/internal/ui/sparkline"
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
		m.refuse("Can't hire: put the cursor on somebody looking for work.")
		return
	}
	got, err := m.w.Hire(c.ID, m.set.Crew.MaxCrew(m.w))
	if err != nil {
		m.refuse("Can't hire: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s hired for %s.", got.Name, money(got.Fee)))
	m.crewCursor = len(m.w.Crew.Members) - 1
}

func (m *Model) askFire() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.refuse("Can't fire: put the cursor on somebody on the payroll.")
		return
	}
	m.fireID = c.ID
	m.mode = modeConfirmFire
}

func (m *Model) confirmFire() {
	m.mode = modePlay
	got, err := m.w.Fire(m.fireID)
	if err != nil {
		m.refuse("Can't fire: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s is gone. The rest noticed.", got.Name))
}

// talking reports whether the report's tell has shown enough for the
// screens to hint at it: the DA's file grew without a bust twice running.
func (m *Model) talking() bool { return m.w.Heat.Leaks >= 2 }

func (m *Model) askInvestigate() {
	if len(m.w.Crew.Members) == 0 {
		m.refuse("Nothing to ask: nobody on the payroll.")
		return
	}
	if m.w.Today.Investigation != nil {
		m.refuse("Can't ask twice: somebody is already asking around tonight.")
		return
	}
	m.mode = modeConfirmInvestigate
}

func (m *Model) confirmInvestigate() {
	m.mode = modePlay
	if err := m.w.Investigate(m.set.Crew.InvestigateCost()); err != nil {
		m.refuse("Can't investigate: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("Questions get asked tonight. Odds of a name ~%.0f%%.", m.set.Crew.InvestigateOdds(m.w)*100))
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
	body := []string{
		fmt.Sprintf("Somebody goes through the crew tonight for %s.", money(m.set.Crew.InvestigateCost())),
		who,
		fmt.Sprintf("If somebody is talking to the police, ~%.0f%% it names them.", odds*100),
	}
	if w.Crew.Investigated > 0 {
		body = append(body, theme.Subtle.Render(fmt.Sprintf("Every empty night so far (%d) narrows it down.", w.Crew.Investigated)))
	}
	body = append(body, theme.Warning.Render(fmt.Sprintf("Naming nobody costs everyone %.0f loyalty.", m.cfg.Crew.Informant.InvestigateLoyalty)))
	return m.modal("INVESTIGATE?", body, m.modalFooter())
}

func (m *Model) askPayOff() {
	c, onPayroll, ok := m.crewSelected()
	if !ok || !onPayroll {
		m.refuse("Can't pay off: put the cursor on somebody on the payroll.")
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
		m.refuse("Can't pay them off: " + err.Error())
		return
	}
	m.say(fmt.Sprintf("%s pocketed it. Loyalty %.0f.", got.Name, got.Loyalty))
}

func (m *Model) payOffConfirm() string {
	c := m.w.Crew.Member(m.fireID)
	if c == nil {
		return m.modal("PAY OFF", []string{"They are gone."}, m.modalFooter())
	}
	body := []string{
		fmt.Sprintf("%s for %s: loyalty %.0f → %.0f.", money(m.set.Crew.PayoffCost(*c)), c.Name, c.Loyalty, min(100, c.Loyalty+m.set.Crew.PayoffLoyalty())),
		theme.Subtle.Render("It buys loyalty, not silence: somebody already talking keeps talking."),
	}
	return m.modal("PAY OFF "+c.Name+"?", body, m.modalFooter())
}

func (m *Model) cyclePay() {
	p := (m.w.Crew.Pay + 1) % 3
	m.w.SetPay(p)
	m.say(fmt.Sprintf("Pay %s, %s/day. %s", p, money(m.set.Crew.Wages(m.w, p)), payBlurb(p)))
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

// crewWarning is the one warning line the crew screen carries under the
// pay dial, when there is one: the tell that somebody is talking, or a
// skim on record. MAIN truncates it; the pane's CREW section carries the
// whole of it.
func (m *Model) crewWarning() string {
	w := m.w
	tun := m.set.Crew.Tuning()
	switch {
	case m.talking():
		return "▲ Somebody is talking. The file grew without a bust. Ask around or fire your suspect."
	case w.Crew.LastSkim > 0 && w.Day-w.Crew.LastSkim < tun.SuspectDays:
		who := fmt.Sprintf("Somebody's loyalty is under %.0f.", tun.SkimThreshold)
		if w.Crew.Role(game.RoleLieutenant) > 0 {
			who = fmt.Sprintf("Somebody's loyalty is under %.0f, or a lieutenant is greedy.", tun.SkimThreshold)
		}
		return fmt.Sprintf("▲ Skimming suspected. Money went missing on day %d. %s", w.Crew.LastSkim, who)
	}
	return ""
}

// payRow draws the pay dial as `stingy  [fair]  generous`.
func payRow(p events.Pay) string {
	var notches []string
	for d := events.PayStingy; d <= events.PayGenerous; d++ {
		notches = append(notches, d.String())
	}
	return dialCells(notches, int(p-events.PayStingy))
}

// post is the roster's status column: where the member is and, when they
// are nowhere, what that means for their role. An accountant has no
// post, they work every front you own; an enforcer without a corner
// guards nothing; a runner without one is idle.
func (m *Model) post(c game.CrewMember) any {
	w := m.w
	switch {
	case c.Lieutenant():
		if c.City != "" {
			return styled{theme.CrewText, "runs " + w.CityName(c.City)}
		}
		return styled{theme.Warning, "no city"}
	case c.Role == "accountant":
		if n := len(w.Fronts); n > 0 {
			return styled{theme.CrewText, plural(n, "front")}
		}
		return styled{theme.Warning, "no front"}
	}
	if p := w.PostOf(c.ID); p != nil {
		return p.Name
	}
	if h := w.GuardOf(c.ID); h != nil {
		return styled{theme.CrewText, "guards " + h.Name}
	}
	if c.Role == "enforcer" {
		return styled{theme.Warning, "unposted"}
	}
	return styled{theme.Warning, "idle"}
}

// crewLine is the line a member skims (or, for a lieutenant, turns)
// under, which the loyalty bar is coloured against.
func (m *Model) crewLine(c game.CrewMember) float64 {
	if c.Lieutenant() {
		return m.set.Crew.FlipLine()
	}
	return m.set.Crew.Tuning().SkimThreshold
}

// viewCrew is the crew screen's MAIN (#86): the title with the count,
// the pay dial, the warning line when there is one, and the two tables.
// Everything about the person under the cursor and about the crew as a
// whole is the pane's.
func (m *Model) viewCrew() string {
	w := m.w
	tun := m.set.Crew.Tuning()
	pay := w.Crew.Pay
	width := m.mainWidth()
	var b strings.Builder

	b.WriteString(truncate(sectionTitle("CREW", theme.Crew)+theme.Subtle.Render(fmt.Sprintf(" · %d of %d on the payroll", len(w.Crew.Members), m.set.Crew.MaxCrew(w))), width) + "\n")
	b.WriteString(truncate(theme.Subtle.Render("pay  ")+payRow(pay)+theme.Gold.Render(fmt.Sprintf("   %s/day", money(m.set.Crew.Wages(w, pay)))), width) + "\n")
	if warn := m.crewWarning(); warn != "" {
		b.WriteString(truncate(theme.Bad.Render(warn), width) + "\n")
	}

	marks := []float64{tun.SkimThreshold / 100}
	// row is the cells every member and candidate shares: the loyalty
	// bar is coloured against the line they skim (or, for a lieutenant,
	// turn) under, and carry is what a runner adds to the stash.
	row := func(c game.CrewMember) []any {
		var carry any
		if c.Units > 0 {
			carry = c.Units
		}
		var name any = c.Name
		if c.ID == w.Crew.Exposed {
			name = styled{theme.Bad.Bold(true), c.Name}
		}
		return []any{name, c.Role, c.Skill, styled{loyaltyStyle(c.Loyalty, m.crewLine(c)), gauge{c.Loyalty / 100, marks, c.Loyalty}}, m.set.Crew.WageAt(w, c, pay), carry}
	}
	shared := []col{{"name", kText, 0}, {"role", kText, 0}, {"skill", kInt, 0}, {"loyalty", kBar, 10}, {"wage", kMoney, 0}, {"carry", kInt, 0}}

	b.WriteString(sectionTitle("ON THE PAYROLL", theme.Crew) + "\n")
	if len(w.Crew.Members) == 0 {
		b.WriteString(emptyState("Nobody on the payroll. Pick a face below and press ", "h", ".") + "\n")
	} else {
		var rows [][]any
		for _, c := range w.Crew.Members {
			rows = append(rows, append(row(c), m.post(c), day(c.Hired)))
		}
		cols := append(shared, col{"post", kText, 0}, col{"hired", kDays, 0})
		// Where MAIN is too narrow for the post to read whole (64
		// columns beside the pane at 100), the columns the pane carries
		// go first: carry, then the hire day.
		for _, drop := range []int{5, 6} {
			if tableWidth(cols, rows) <= width {
				break
			}
			cols = append(cols[:drop:drop], cols[drop+1:]...)
			for i := range rows {
				rows[i] = append(rows[i][:drop:drop], rows[i][drop+1:]...)
			}
		}
		for _, l := range table(cols, rows, m.crewCursor, width) {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("\n")

	next := max(1, m.set.Crew.PoolDays(w)-(w.Day-w.Crew.PoolDay))
	title := sectionTitle("LOOKING FOR WORK", theme.Crew) + theme.Subtle.Render(" · new faces in "+plural(next, "day"))
	if !m.set.Crew.LieutenantsWanted(w) {
		// The line named before it fires (#148): what the lieutenants
		// wait on, in the form the width has room for.
		title = firstFit(width,
			title+theme.Subtle.Render(" · lieutenants come looking once you hold corners in two cities"),
			title+theme.Subtle.Render(" · lieutenants once you hold two cities"),
			title)
	}
	b.WriteString(truncate(title, width) + "\n")
	if len(w.Crew.Candidates) == 0 {
		b.WriteString(theme.Subtle.Render("Nobody right now.") + "\n")
	} else {
		var rows [][]any
		for _, c := range w.Crew.Candidates {
			var fee any = c.Fee
			if c.Fee > w.Player.DirtyCash {
				fee = styled{theme.Bad, c.Fee}
			}
			rows = append(rows, append(row(c), fee))
		}
		for _, l := range table(append(shared, col{"fee", kMoney, 0}), rows, m.crewCursor-len(w.Crew.Members), width) {
			b.WriteString(l + "\n")
		}
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

// crewDetails is the crew screen's pane (#86): the person under the
// cursor (their role, skill and hire day, loyalty against the lines, the
// wage at each pay dial, post, temper and whether they were named), what
// the keys would do to them with the numbers the confirmations use, then
// CREW, what the crew adds and needs as a whole, and the keys.
func (m *Model) crewDetails() []section {
	c, onPayroll, ok := m.crewSelected()
	if !ok {
		return []section{{"NOBODY", wrapped(theme.Subtle, "Nobody on the payroll and nobody looking for work. New faces come by every few days.")}, m.crewSection()}
	}
	return []section{{strings.ToUpper(c.Name), m.personLines(c, onPayroll)}, m.crewSection()}
}

// personLines is the selection section's body for one member or
// candidate.
func (m *Model) personLines(c game.CrewMember, onPayroll bool) []string {
	w := m.w
	tun := m.set.Crew.Tuning()
	pay := w.Crew.Pay
	sub := theme.Subtle.Render

	// The first line is the person in a breath: the role, the skill and
	// the day they signed (or, for a candidate, their fee); the strip
	// reads it after the name.
	first := fmt.Sprintf("%s · skill %d · fee %s", c.Role, c.Skill, money(c.Fee))
	if onPayroll {
		first = fmt.Sprintf("%s · skill %d · hired d%d", c.Role, c.Skill, c.Hired)
		if lipgloss.Width(first) > paneTextW {
			first = fmt.Sprintf("%s · skill %d · d%d", c.Role, c.Skill, c.Hired)
		}
	}
	line := m.crewLine(c)
	lines := []string{
		first,
		row("loyalty", loyaltyStyle(c.Loyalty, line).Render(sparkline.Bar(c.Loyalty/100, 10, []float64{line / 100})+fmt.Sprintf(" %.0f", c.Loyalty))),
	}
	if c.Lieutenant() {
		lines = append(lines, sub(fmt.Sprintf("  turns under %.0f · walks at %.0f", line, tun.QuitThreshold)))
	} else {
		lines = append(lines, sub(fmt.Sprintf("  skims under %.0f · walks at %.0f", line, tun.QuitThreshold)))
	}
	// The wage at the dial, and at the other two.
	var others []string
	for d := events.PayStingy; d <= events.PayGenerous; d++ {
		if d != pay {
			others = append(others, fmt.Sprintf("%s %s", d, money(m.set.Crew.WageAt(w, c, d))))
		}
	}
	lines = append(lines, row("wage", fmt.Sprintf("%s/day %s", money(m.set.Crew.WageAt(w, c, pay)), pay)), sub("  "+strings.Join(others, " · ")))
	if c.Units > 0 {
		lines = append(lines, row("carries", "+"+plural(c.Units, "unit")))
	}
	if !onPayroll {
		lines = append(lines, row("would", hireBlurb(c.Role)))
		hire := fmt.Sprintf("hire for %s", money(c.Fee))
		if c.Fee > w.Player.DirtyCash {
			hire = theme.Bad.Render(hire + " · can't afford")
		}
		return append(lines, keyRow("h", hire), m.askAroundRow())
	}
	switch {
	case c.Lieutenant() && c.City != "":
		lines = append(lines, row("runs", w.CityName(c.City)))
	case c.Lieutenant():
		lines = append(lines, row("runs", theme.Warning.Render("no city yet")))
	case c.Role == "accountant":
		if len(w.Fronts) == 0 {
			lines = append(lines, row("post", theme.Warning.Render("no front to work")))
		} else {
			lines = append(lines, row("post", "the books"))
		}
	default:
		if p := w.PostOf(c.ID); p != nil {
			lines = append(lines, row("post", p.Name))
		} else if c.Role == "enforcer" {
			lines = append(lines, row("post", theme.Warning.Render("unposted")))
		} else {
			lines = append(lines, row("post", theme.Warning.Render("idle")))
		}
	}
	if t := m.temper(c); t != "" {
		lines = append(lines, row("temper", t))
	}
	if c.ID == w.Crew.Exposed {
		lines = append(lines, theme.Bad.Bold(true).Render("SNITCH")+theme.Bad.Render(": talking to the police"))
	}
	// What the keys would do to them, with the numbers the
	// confirmations use.
	if c.ID == w.Crew.Exposed {
		lines = append(lines, keyRow("f", "fire: the file stops growing"))
	} else {
		lines = append(lines, keyRow("f", fmt.Sprintf("fire: the rest lose %.0f loyalty", tun.FireLoyalty)))
	}
	if c.Lieutenant() && c.City == "" {
		lines = append(lines, keyRow("t", "give them a city"))
	} else if c.Lieutenant() {
		lines = append(lines, keyRow("t", "move them or take the city"))
	}
	lines = append(lines, keyRow("$", fmt.Sprintf("pay off for %s: %.0f → %.0f", money(m.set.Crew.PayoffCost(c)), c.Loyalty, min(100, c.Loyalty+m.set.Crew.PayoffLoyalty()))))
	return append(lines, m.askAroundRow())
}

// askAroundRow is what i would do tonight, with the price and the odds
// the confirmation shows.
func (m *Model) askAroundRow() string {
	if len(m.w.Crew.Members) == 0 {
		return keyRow("i", theme.Subtle.Render("ask around: nobody to ask"))
	}
	if m.w.Today.Investigation != nil {
		return keyRow("i", theme.Subtle.Render("ask around: already asking"))
	}
	return keyRow("i", fmt.Sprintf("ask around %s, names ~%.0f%%", money(m.set.Crew.InvestigateCost()), m.set.Crew.InvestigateOdds(m.w)*100))
}

// temper is a lieutenant's personality as the pane shows it: the word
// once you have seen enough of them, how long that is until then, and
// nothing for anyone else.
func (m *Model) temper(c game.CrewMember) string {
	switch {
	case !c.Lieutenant():
		return ""
	case c.Observed:
		return c.Personality
	case c.City == "":
		return theme.Subtle.Render("shows on the job")
	}
	left := max(1, m.set.Crew.RevealDays()-(m.w.Day-c.Assigned))
	return theme.Subtle.Render("shows in " + plural(left, "day"))
}

// hireBlurb is what a candidate would do on the payroll, for the pane.
func hireBlurb(role string) string {
	switch role {
	case "accountant":
		return "work the fronts"
	case "enforcer":
		return "guard a corner"
	case game.RoleLieutenant:
		return "run a city"
	default:
		return "hold a corner"
	}
}

// crewSection is the pane's CREW section: the warning in full, what the
// crew adds where you stand, who is off a corner, what the accountants
// do, who runs what, who is talking and what the sloppy ones cost.
func (m *Model) crewSection() section {
	w := m.w
	here := w.Here()
	sub := theme.Subtle.Render
	var lines []string
	if warn := m.crewWarning(); warn != "" {
		lines = append(lines, wrapped(theme.Bad, warn)...)
	}
	lines = append(lines,
		row("capacity", fmt.Sprintf("%d in %s", w.Capacity(here.ID), here.Name)),
		row("", sub(fmt.Sprintf("%d yours + %d crew", w.Player.CarryLimit, w.Capacity(here.ID)-w.Player.CarryLimit))),
		row("corners", fmt.Sprintf("%d worked", w.Worked())),
	)
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
		lines = append(lines, row("idle", theme.Warning.Render(plural(idle, "runner"))))
	}
	if unposted > 0 {
		lines = append(lines, row("unposted", theme.Warning.Render(plural(unposted, "enforcer"))))
	}
	if idle+unposted > 0 {
		// The pointer is wrapped on its own so it never breaks.
		lines = append(lines, wrapped(theme.Warning, "A runner earns nothing and an enforcer guards nothing off a corner.")...)
		lines = append(lines, wrapped(theme.Warning, "Post them "+screenPointer(screenMap)+".")...)
	}
	if n := w.Crew.Role("accountant"); n > 0 {
		if len(w.Fronts) == 0 {
			lines = append(lines, wrapped(theme.Warning, "An accountant with no front is a wage.")...)
			lines = append(lines, wrapped(theme.Warning, "Buy "+screenPointer(screenLedger)+".")...)
		} else {
			add, cut := m.accountants()
			lines = append(lines, row("accountant", fmt.Sprintf("+%s/day a front", money(add))), row("", sub(fmt.Sprintf("audit risk cut %.0f%%", cut*100))))
		}
	}
	for _, cid := range w.CityOrder {
		if lt := w.Crew.Lieutenant(cid); lt != nil {
			runs := lt.Name
			if lt.Observed {
				runs += " · " + lt.Personality
			}
			lines = append(lines, row(w.CityName(cid), runs))
		}
	}
	for _, c := range w.Crew.Members {
		if c.Lieutenant() && c.City == "" {
			lines = append(lines, row("no city", theme.Warning.Render(c.Name+" is a wage")))
		}
	}
	if c := w.Crew.Member(w.Crew.Exposed); c != nil {
		lines = append(lines, row("snitch", theme.Bad.Render(c.Name+", fire them")))
	} else if w.Today.Investigation != nil {
		lines = append(lines, row("tonight", "questions get asked"))
	}
	if sl := m.set.Heat.Sloppiness(w, here.ID); sl > 0 {
		per := sl * m.cfg.Heat.Heat.SloppyHeat * 100
		lines = append(lines, row("sloppy", theme.Warning.Render(fmt.Sprintf("+%.1f heat/100 units", per))), row("", sub(fmt.Sprintf("runners under skill %d", m.cfg.Heat.Heat.SloppySkill))))
	}
	return section{"CREW", lines}
}
